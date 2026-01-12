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
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run a headless scan (no TUI)",
	Long: `Run CloudSlash in headless mode. Useful for CI/CD pipelines or cron jobs.
    
Example:
  cloudslash scan --region us-west-2`,
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

		// Resource Deletion Script
		nukePath := "cloudslash-out/resource_deletion.sh"
		// Ensure output dir exists
		if _, err := os.Stat("cloudslash-out"); os.IsNotExist(err) {
			_ = os.Mkdir("cloudslash-out", 0755)
		}
		gen := script.NewGenerator(g, nil)
		if err := gen.GenerateDeletionScript(nukePath); err != nil {
			fmt.Printf("[WARN] Failed to generate deletion script: %v\n", err)
		} else {
			fmt.Printf("\n[SUCCESS] Resource deletion script generated: %s\n", nukePath)
			_ = os.Chmod(nukePath, 0755)
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
					var zombies []*graph.Node
					g.Mu.RLock()
					for _, n := range g.Nodes {
						if n.IsWaste && !n.Ignored {
							zombies = append(zombies, n)
						}
					}
					g.Mu.RUnlock()

					// Provenance Analysis
					provEngine := provenance.NewEngine(".")
					provMap := make(map[string]*provenance.ProvenanceRecord)

					// Attribute all unused resources
					for _, z := range zombies {
						rec, err := provEngine.Attribute(z.ID, state)
						if err == nil && rec != nil {
							provMap[rec.TFAddress] = rec
						}
					}

					report := tf.Analyze(zombies, state)

					printTerraformReport(report, provMap)
					generateFixScript(report)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().Bool("no-metrics", false, "Disable CloudWatch Metric calls (Saves money)")
	scanCmd.Flags().Bool("fast", false, "Alias for --no-metrics (Fast scan)")
	scanCmd.Flags().Bool("headless", false, "Run without TUI (for CI/CD)")
	scanCmd.Flags().StringVar(&config.SlackWebhook, "slack-webhook", "", "Slack Webhook URL for Reporting")
	scanCmd.Flags().StringVar(&config.SlackChannel, "slack-channel", "", "Override Slack Channel")
}

func printTerraformReport(report *tf.AnalysisReport, provMap map[string]*provenance.ProvenanceRecord) {
	if report.TotalZombiesFound == 0 {
		fmt.Println("\n[Success] No Terraform-managed unused resources found.")
		return
	}

	fmt.Printf("\n[ Analysis Complete ]\nFound %d unused resources managed by Terraform.\n", report.TotalZombiesFound)
	fmt.Println("\n-------------------------------------------------------------")
	fmt.Println("RECOMMENDED ACTION:")
	fmt.Println("-------------------------------------------------------------")

	for _, mod := range report.ModulesToDelete {
		fmt.Printf("# 1. Remove the '%s' module entirely\n", mod)
		fmt.Println("#    (Logic: All resources in this module are dead)")
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
		fmt.Println("  │ Status:  [LEGACY DEBT] (> 1 year old)")
	} else {
		fmt.Println("  │ Status:  [ACTIVE COMMIT] (Recent change)")
	}
	fmt.Println("  └──────────────────────────────────────────────────────────────")
}

func generateFixScript(report *tf.AnalysisReport) {
	if report.TotalZombiesFound == 0 {
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
	fmt.Fprintf(f, "# CloudSlash %s - Terraform Surgical Fix Script\n", version.Current)
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
