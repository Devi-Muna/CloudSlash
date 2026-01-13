package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/app"
	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/provenance"
	tf "github.com/DrSkyle/cloudslash/internal/terraform"
	script "github.com/DrSkyle/cloudslash/internal/tf"
	"github.com/DrSkyle/cloudslash/internal/version"
	"github.com/DrSkyle/cloudslash/internal/solver"
	"github.com/DrSkyle/cloudslash/internal/oracle"
	"github.com/DrSkyle/cloudslash/internal/policy"
	"github.com/DrSkyle/cloudslash/internal/tetris"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Launch interactive infrastructure audit (TUI)",
	Long: `Starts the CloudSlash interactive terminal interface for real-time infrastructure auditing.
    
Use --headless for CI/CD pipelines or non-interactive environments.

Example:
  cloudslash scan
  cloudslash scan --headless --region us-east-1`,
	Run: func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("region") && !config.MockMode {
			regions, err := PromptForRegions()
			if err == nil {
				config.Region = strings.Join(regions, ",")
			}
		}

		if noMetrics, _ := cmd.Flags().GetBool("no-metrics"); noMetrics {
			config.DisableCWMetrics = true
		}
		if fast, _ := cmd.Flags().GetBool("fast"); fast {
			config.DisableCWMetrics = true
		}

		config.Headless = true
		_, g, err := app.Run(config)
		if err != nil {
			fmt.Printf("Error running scan: %v\n", err)
			os.Exit(1)
		}

		// Optimize (v1.4.0 Optimization Engine)
		runSolver(g)

		// Resource Deletion Script
		cleanupPath := "cloudslash-out/resource_deletion.sh"
		// Ensure output dir exists
		if _, err := os.Stat("cloudslash-out"); os.IsNotExist(err) {
			_ = os.Mkdir("cloudslash-out", 0755)
		}
		gen := script.NewGenerator(g, nil)
		if err := gen.GenerateDeletionScript(cleanupPath); err != nil {
			fmt.Printf("[WARN] Failed to generate deletion script: %v\n", err)
		} else {
			fmt.Printf("\n[SUCCESS] Resource deletion script generated: %s\n", cleanupPath)
			_ = os.Chmod(cleanupPath, 0755)
		}

		// Terraform Integration
		tfClient := tf.NewClient()
		if tfClient.IsInstalled() {
			fmt.Println("\n[INFO] Terraform detected. Initializing State Analysis...")
			fmt.Println("[WARN] Ensure your local 'terraform workspace' matches the target AWS account.")

			stateBytes, err := tfClient.PullState(context.Background())
			if err != nil {
				fmt.Printf("[WARN] Failed to pull Terraform state: %v\n", err)
				fmt.Println("       Skipping Terraform analysis. (Verify authentication/backend access)")
			} else {
				state, err := tf.ParseState(stateBytes)
				if err != nil {
					fmt.Printf("[ERROR] Failed to parse Terraform state: %v\n", err)
				} else {
					var unused []*graph.Node
					g.Mu.RLock()
					for _, n := range g.Nodes {
						if n.IsWaste && !n.Ignored {
							unused = append(unused, n)
						}
					}
					g.Mu.RUnlock()

					// Provenance Analysis
					provEngine := provenance.NewEngine(".")
					provMap := make(map[string]*provenance.ProvenanceRecord)

					// Attribute all unused resources
					for _, z := range unused {
						rec, err := provEngine.Attribute(z.ID, state)
						if err == nil && rec != nil {
							provMap[rec.TFAddress] = rec
						}
					}

					report := tf.Analyze(unused, state)

					printTerraformReport(report, provMap)
					generateFixScript(report)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().Bool("no-metrics", false, "Disable CloudWatch Metrics (Optimizes API costs)")
	scanCmd.Flags().Bool("fast", false, "Alias for --no-metrics (Fast scan)")
	scanCmd.Flags().Bool("headless", false, "Run without TUI (for CI/CD)")
	scanCmd.Flags().StringVar(&config.SlackWebhook, "slack-webhook", "", "Slack Webhook URL for Reporting")
	scanCmd.Flags().StringVar(&config.SlackChannel, "slack-channel", "", "Override Slack Channel")
}

func printTerraformReport(report *tf.AnalysisReport, provMap map[string]*provenance.ProvenanceRecord) {
	if report.TotalUnused == 0 {
		fmt.Println("\n[Success] No Terraform-managed unused resources found.")
		return
	}

	fmt.Printf("\n[ Analysis Complete ]\nFound %d unused resources managed by Terraform.\n", report.TotalUnused)
	fmt.Println("\n-------------------------------------------------------------")
	fmt.Println("RECOMMENDED ACTION:")
	fmt.Println("-------------------------------------------------------------")

	for _, mod := range report.ModulesToDelete {
		fmt.Printf("# 1. Remove the '%s' module entirely\n", mod)
		fmt.Println("#    (Logic: All resources in this module are unused)")
		fmt.Printf("terraform state rm %s\n\n", mod)
	}

	for _, res := range report.ResourcesToDelete {
		fmt.Printf("# Remove orphaned resource: %s\n", res)
		
		// PRINT PROVENANCE AUDIT BOX
		if rec, ok := provMap[res]; ok {
			printProvenanceBox(rec)
		}
		
		fmt.Printf("terraform state rm %s\n\n", res)
	}

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("  Next Step: Run 'terraform plan' to verify the state is clean.")
	fmt.Println("  Script generated: cloudslash-out/fix_terraform.sh")
}

func printProvenanceBox(rec *provenance.ProvenanceRecord) {
	fmt.Println("  ┌── PROVENANCE AUDIT ──────────────────────────────────────────")
	fmt.Printf("  │ Author:  %s\n", rec.Author)
	fmt.Printf("  │ Commit:  %s (%s)\n", rec.CommitHash[:7], rec.CommitDate.Format("2006-01-02"))
	fmt.Printf("  │ Message: \"%s\"\n", strings.TrimSpace(rec.Message))
	fmt.Printf("  │ File:    %s:%d\n", rec.FilePath, rec.LineStart)
	
	if rec.IsLegacy {
		fmt.Println("  │ Status:  [LEGACY] (> 1 year old)")
	} else {
		fmt.Println("  │ Status:  [ACTIVE COMMIT] (Recent change)")
	}
	fmt.Println("  └──────────────────────────────────────────────────────────────")
}

func generateFixScript(report *tf.AnalysisReport) {
	if report.TotalUnused == 0 {
		return
	}

	dir := "cloudslash-out"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.Mkdir(dir, 0755)
	}

	f, err := os.Create(fmt.Sprintf("%s/fix_terraform.sh", dir))
	if err != nil {
		fmt.Printf("[ERROR] Failed to create fix script: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash %s - Terraform Remediation Script\n", version.Current)
	fmt.Fprintf(f, "# Generated based on AWS Scan vs Terraform State Analysis\n\n")

	for _, mod := range report.ModulesToDelete {
		fmt.Fprintf(f, "echo \"Removing Unused Module: %s\"\n", mod)
		fmt.Fprintf(f, "terraform state rm %s\n", mod)
	}

	for _, res := range report.ResourcesToDelete {
		fmt.Fprintf(f, "echo \"Removing Unused Resource: %s\"\n", res)
		fmt.Fprintf(f, "terraform state rm %s\n", res)
	}

	fmt.Fprintf(f, "\necho \"--------------------------------------------------\"\n")
	fmt.Fprintf(f, "echo \"State update complete. Run 'terraform plan' to verify.\"\n")
	_ = os.Chmod(f.Name(), 0755)
}

func runSolver(g *graph.Graph) {
	fmt.Printf("\n[ %s OPTIMIZATION ENGINE ]\n", version.Current)
	fmt.Println("Initializing Solver...")

	// 1. Build Workloads from Graph
	// Extract all EC2/EKS Nodes and convert to Workloads.
	// For MVP: We assume every existing Instance is a "Workload" we want to re-host.
	// In reality, we'd read Pods. But for non-K8s, the "Workload" is the instance itself.
	var workloads []*tetris.Item
	g.Mu.RLock()
	for _, n := range g.Nodes {
		if n.Type == "AWS::EC2::Instance" {
			// Extract CPU/RAM from properties if available, else default (mock)
			// Assuming we enriched this data.
			// Mocking 2 vCPU, 4GB RAM for demo if usage known.
			workloads = append(workloads, &tetris.Item{
				ID: n.ID,
				Dimensions: tetris.Dimensions{
					CPU: 2000, 
					RAM: 4096,
				},
			})
		}
	}
	g.Mu.RUnlock()

	if len(workloads) == 0 {
		fmt.Println("No active compute workloads detected. Optimization skipped (requires running EC2 instances).")
		return
	}

	// 2. Setup Solver Components
	riskEngine := oracle.NewRiskEngine()
	safePolicy := policy.DefaultPolicy()
	validator := policy.NewValidator(safePolicy)
	optimizer := solver.NewOptimizer(riskEngine, validator)

	// 3. Define Catalog (Mock for MVP)
	// In production, this comes from AWS Pricing API
	catalog := []solver.InstanceType{
		{Name: "m5.large", CPU: 2000, RAM: 8192, HourlyCost: 0.096, Zone: "us-east-1a"},
		{Name: "c6g.large", CPU: 2000, RAM: 4096, HourlyCost: 0.068, Zone: "us-east-1a"}, // Cheaper/Better
		{Name: "r5.large", CPU: 2000, RAM: 16384, HourlyCost: 0.126, Zone: "us-east-1a"},
	}

	// 4. Solve
	req := solver.OptimizationRequest{
		Workloads:    workloads,
		Catalog:      catalog,
		CurrentSpend: 1000.0, // Mock current spend
	}

	plan, err := optimizer.Solve(req)
	if err != nil {
		fmt.Printf("[WARN] Solver failed: %v\n", err)
		return
	}

	// 5. Output Results
	fmt.Println("-------------------------------------------------------------")
	fmt.Printf("OPTIMIZATION PLAN (Risk Score: %.2f%%)\n", plan.RiskScore*100)
	fmt.Printf("Current Spend: $%.2f/mo -> Optimized: $%.2f/mo\n", req.CurrentSpend, plan.TotalCost)
	fmt.Printf("POTENTIAL SAVINGS: $%.2f/mo\n", plan.Savings)
	fmt.Println("-------------------------------------------------------------")
	for _, instr := range plan.Instructions {
		fmt.Printf(" > %s\n", instr)
	}
	fmt.Printf(" > Packing Efficiency: %d items packed into %d nodes.\n", len(workloads), len(plan.Nodes))
	fmt.Println("-------------------------------------------------------------")
}
