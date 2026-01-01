package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/app"
	"github.com/DrSkyle/cloudslash/internal/graph"
	tf "github.com/DrSkyle/cloudslash/internal/terraform"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run a headless scan (no TUI)",
	Long: `Run CloudSlash in headless mode. Useful for CI/CD pipelines or cron jobs.
    
Example:
  cloudslash scan --region us-west-2`,
	Run: func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("region") {
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

		// The State Doctor (Terraform Integration)
		tfClient := tf.NewClient()
		if tfClient.IsInstalled() {
			fmt.Println("\n[INFO] Terraform detected. Initializing 'The State Doctor'...")
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

					report := tf.Analyze(zombies, state)

					printTerraformReport(report)
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
}

func printTerraformReport(report *tf.AnalysisReport) {
	if report.TotalZombiesFound == 0 {
		fmt.Println("\n[Success] No Terraform-managed zombies found.")
		return
	}

	fmt.Printf("\n[ Analysis Complete ]\nFound %d Zombie resources managed by Terraform.\n", report.TotalZombiesFound)
	fmt.Println("\n-------------------------------------------------------------")
	fmt.Println("RECOMMENDED ACTION (The State Doctor):")
	fmt.Println("-------------------------------------------------------------")

	for _, mod := range report.ModulesToDelete {
		fmt.Printf("# 1. Remove the '%s' module entirely\n", mod)
		fmt.Println("#    (Logic: All resources in this module are dead)")
		fmt.Printf("terraform state rm %s\n\n", mod)
	}

	for _, res := range report.ResourcesToDelete {
		fmt.Printf("# Remove orphaned resource\n")
		fmt.Printf("terraform state rm %s\n", res)
	}

	fmt.Println("\n-------------------------------------------------------------")
	fmt.Println("  Next Step: Run 'terraform plan' to verify the state is clean.")
	fmt.Println("  Script generated: cloudslash-out/fix_terraform.sh")
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
	fmt.Fprintf(f, "# CloudSlash v1.2.8 - Terraform Surgical Fix Script\n")
	fmt.Fprintf(f, "# Generated based on AWS Scan vs Terraform State Analysis\n\n")

	for _, mod := range report.ModulesToDelete {
		fmt.Fprintf(f, "echo \"Removing Zombie Module: %s\"\n", mod)
		fmt.Fprintf(f, "terraform state rm %s\n", mod)
	}

	for _, res := range report.ResourcesToDelete {
		fmt.Fprintf(f, "echo \"Removing Zombie Resource: %s\"\n", res)
		fmt.Fprintf(f, "terraform state rm %s\n", res)
	}

	fmt.Fprintf(f, "\necho \"--------------------------------------------------\"\n")
	fmt.Fprintf(f, "echo \"State update complete. Run 'terraform plan' to verify.\"\n")
	_ = os.Chmod(f.Name(), 0755)
}
