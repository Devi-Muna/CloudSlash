package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/engine"
	"github.com/DrSkyle/cloudslash/pkg/engine/aws"
	internalconfig "github.com/DrSkyle/cloudslash/pkg/config"
	"github.com/DrSkyle/cloudslash/pkg/engine/pricing"
	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/DrSkyle/cloudslash/pkg/engine/provenance"
	tf "github.com/DrSkyle/cloudslash/pkg/providers/terraform"
	script "github.com/DrSkyle/cloudslash/pkg/providers/tf"
	ui "github.com/DrSkyle/cloudslash/pkg/tui"
	"github.com/DrSkyle/cloudslash/pkg/version"
	"github.com/DrSkyle/cloudslash/pkg/engine/solver"
	"github.com/DrSkyle/cloudslash/pkg/engine/oracle"
	"github.com/DrSkyle/cloudslash/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/pkg/engine/tetris"
	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Launch interactive infrastructure audit (TUI)",
	Long: `Starts the CloudSlash interactive terminal interface for real-time infrastructure auditing.
    
Use --headless for headless mode.

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


		if headless, _ := cmd.Flags().GetBool("headless"); headless {
			config.Headless = true
		}

		// Initialize the application logger.
		var handler slog.Handler
		if config.JsonLogs {
			handler = slog.NewJSONHandler(os.Stdout, nil)
		} else if config.Verbose {
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		} else {
			handler = slog.NewTextHandler(io.Discard, nil) // Silent by default.
		}
		config.Logger = slog.New(handler)

		// Resolve the cache directory, adhering to XDG standards.
		cacheDir := os.Getenv("XDG_CACHE_HOME")
		if cacheDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				home = os.TempDir()
			}
			cacheDir = filepath.Join(home, ".cloudslash")
		} else {
			cacheDir = filepath.Join(cacheDir, "cloudslash")
		}
		config.CacheDir = cacheDir

		// Execute the core engine scan.
		_, g, swarmEngine, err := engine.Run(cmd.Context(), config)
		if err != nil {
			fmt.Printf("Error running scan: %v\n", err)
			os.Exit(1)
		}

		// Launch the terminal user interface.
		if !config.Headless {
			model := ui.NewModel(swarmEngine, g, config.MockMode, config.Region)
			startTime := time.Now()
			p := tea.NewProgram(model)
			if _, err := p.Run(); err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}
			
			// Display the exit summary.
			g.Mu.RLock()
			count := len(g.Nodes)
			g.Mu.RUnlock()
			ui.PrintExitSummary(startTime, count)
		}

		// Execute the optimization solver.
		runSolver(g)

		// Generate the resource deletion script.
		cleanupPath := filepath.Join(config.OutputDir, "resource_deletion.sh")
		// Ensure output directory exists.
		if _, err := os.Stat(config.OutputDir); os.IsNotExist(err) {
			_ = os.Mkdir(config.OutputDir, 0755)
		}
		gen := script.NewGenerator(g, nil)
		if err := gen.GenerateDeletionScript(cleanupPath); err != nil {
			fmt.Printf("[WARN] Failed to generate deletion script: %v\n", err)
		} else {
			fmt.Printf("\n[SUCCESS] Resource deletion script generated: %s\n", cleanupPath)
			_ = os.Chmod(cleanupPath, 0755)
		}

		// Generate the restoration plan (Lazarus Protocol).
		restorePath := filepath.Join(config.OutputDir, "restore.tf")
		if err := gen.GenerateRestorationPlan(restorePath); err != nil {
			fmt.Printf("[WARN] Failed to generate restoration plan: %v\n", err)
		} else {
			fmt.Printf("[SUCCESS] Lazarus Protocol Active: Restoration plan generated: %s\n", restorePath)
		}

		// Initialize the Terraform client.
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

					// Analyze resource provenance.
					provEngine := provenance.NewEngine(".")
					provMap := make(map[string]*provenance.ProvenanceRecord)

					// Attribute unused resources to their source.
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
	scanCmd.Flags().IntVar(&config.MaxConcurrency, "max-workers", 0, "Limit concurrency (default: auto)")
	scanCmd.Flags().StringVar(&config.RulesFile, "rules", "", "Path to YAML Policy Rules (e.g. dynamic_rules.yaml)")
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
		
		// Print provenance details.
		if rec, ok := provMap[res]; ok {
			printProvenanceBox(rec)
		}
		
		fmt.Printf("terraform state rm %s\n\n", res)
	}

	fmt.Println("-------------------------------------------------------------")
	fmt.Println("  Next Step: Run 'terraform plan' to verify the state is clean.")
	fmt.Printf("  Script generated: %s/fix_terraform.sh\n", config.OutputDir)
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

	dir := config.OutputDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.Mkdir(dir, 0755)
	}

	f, err := os.Create(filepath.Join(dir, "fix_terraform.sh"))
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
	fmt.Println("Initializing Solver with Dynamic Intelligence...")

	// ---------------------------------------------------------
	// Initialize Infrastructure (Logger & Paths).
	// ---------------------------------------------------------
	
	// Setup Logger (Silent by default unless verbose).
	var logger *slog.Logger
	if config.Verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Setup Cache Directory (XDG Compliant).
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cloudslash", "cache")
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		cacheDir = filepath.Join(xdg, "cloudslash")
	}
	_ = os.MkdirAll(cacheDir, 0755)

	// ---------------------------------------------------------
	// Inject Dependencies into Pricing Client.
	// ---------------------------------------------------------
	fmt.Println(" -> Connecting to AWS Pricing API (this may take a moment)...")
	ctx := context.Background()

	// Pass Logger, CacheDir, and Manual Discount Rate from Config
	// Note: You need to ensure config.DiscountRate exists in your Config struct!
	// If not, use 0.0 or add it to defaults.go
	manualRate := 0.0 // or config.DiscountRate
	
	pc, err := pricing.NewClient(ctx, logger, cacheDir, manualRate)
	
	if err != nil {
		fmt.Printf("[WARN] Pricing API unavailable: %v. Using static estimation.\n", err)
	} else {
		// Log the effective rate if verbose
		logger.Info("Pricing Client Initialized")
	}

	// Convert graph nodes to workloads and calculate current spend.
	var workloads []*tetris.Item
	var currentSpend float64

	g.Mu.RLock()
	for _, n := range g.Nodes {
		if n.Type == "AWS::EC2::Instance" {
			// Extract compute resources dynamically.
			instanceType := "m5.large" // Default type.
			if t, ok := n.Properties["Type"].(string); ok {
				instanceType = t
			}

			specs := aws.GetSpecs(instanceType)
			
			// Calculate current cost for this instance.
			var cost float64
			var err error
			if pc != nil {
				cost, err = pc.GetEC2InstancePrice(ctx, internalconfig.DefaultRegion, instanceType)
			}
			if pc == nil || err != nil || cost == 0 {
				estimator := &aws.StaticCostEstimator{}
				cost = estimator.GetEstimatedCost(instanceType, internalconfig.DefaultRegion)
			}
			
			// Add to total monthly spend (cost is per month).
			currentSpend += cost

			workloads = append(workloads, &tetris.Item{
				ID: n.ID,
				Dimensions: tetris.Dimensions{
					CPU: specs.VCPU * 1000, 
					RAM: specs.Memory,
				},
			})
		}
	}
	g.Mu.RUnlock()

	if len(workloads) == 0 {
		fmt.Println("No active compute workloads detected. Optimization skipped.")
		return
	}

	// Initialize solver components.
	riskEngine := oracle.NewRiskEngine(internalconfig.DefaultRiskConfig())
	safePolicy := policy.DefaultPolicy()
	validator := policy.NewValidator(safePolicy)
	optimizer := solver.NewOptimizer(riskEngine, validator)

	// Build Dynamic Catalog.
	var catalog []solver.InstanceType
	
	fmt.Printf("Building Instance Catalog (%d candidates)...\n", len(aws.CandidateTypes))
	if pc != nil {
		fmt.Printf(" > Connected to AWS Pricing API (%s). Fetching live data...\n", internalconfig.DefaultRegion)
	} else {
		fmt.Println(" > AWS Pricing API unavailable. Using static estimates.")
	}
	
	successCount := 0
	fallbackCount := 0

	for i, it := range aws.CandidateTypes {
		specs := aws.GetSpecs(it)
		var cost float64
		var err error

		// Visual progress indicator.
		if pc != nil {
			fmt.Printf("\r   [%d/%d] Querying %-12s ", i+1, len(aws.CandidateTypes), it)
			cost, err = pc.GetEC2InstancePrice(ctx, internalconfig.DefaultRegion, it)
		}
		
		if pc == nil || err != nil || cost == 0 {
			estimator := &aws.StaticCostEstimator{}
			cost = estimator.GetEstimatedCost(it, internalconfig.DefaultRegion)
			fallbackCount++
		} else {
			successCount++
		}

		// Monthly Cost (730 hours).
		hourlyCost := cost / 730.0

		catalog = append(catalog, solver.InstanceType{
			Name:       it,
			CPU:        specs.VCPU * 1000,
			RAM:        specs.Memory,
			HourlyCost: hourlyCost,
			Zone:       internalconfig.DefaultRegion + "a", // Default zone placement.
		})
	}
	fmt.Printf("\n > Catalog Complete. Live Prices: %d | Estimates: %d\n", successCount, fallbackCount)

	// Execute solver.
	req := solver.OptimizationRequest{
		Workloads:    workloads,
		Catalog:      catalog,
		CurrentSpend: currentSpend,
	}

	plan, err := optimizer.Solve(req)
	if err != nil {
		fmt.Printf("[WARN] Solver failed: %v\n", err)
		return
	}

	// Print optimization results.
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
