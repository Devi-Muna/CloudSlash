package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	internalconfig "github.com/DrSkyle/cloudslash/v2/pkg/config"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/oracle"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/pricing"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/provenance"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/solver"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/tetris"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	tf "github.com/DrSkyle/cloudslash/v2/pkg/providers/terraform"
	script "github.com/DrSkyle/cloudslash/v2/pkg/providers/tf"
	ui "github.com/DrSkyle/cloudslash/v2/pkg/tui"
	"github.com/DrSkyle/cloudslash/v2/pkg/version"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
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

		// Pre-flight: Validate AWS Credentials before starting engine.
		// This prevents silent hanging/retrying if keys are missing.
		if !config.MockMode {
			regions := strings.Split(config.Region, ",")
			primaryRegion := "us-east-1"
			if len(regions) > 0 && regions[0] != "" {
				primaryRegion = regions[0]
			}

			fmt.Printf(" -> Verifying AWS Credentials (%s)... ", primaryRegion)
			
			// Create a lightweight client just for verification.
			verifClient, err := aws.NewClient(cmd.Context(), primaryRegion, "", false)
			if err != nil {
				fmt.Printf("\n[FATAL] Failed to initialize AWS client: %v\n", err)
				os.Exit(1)
			}

			accountId, err := verifClient.VerifyIdentity(cmd.Context())
			if err != nil {
				// Graceful exit!
				fmt.Printf("\n\n❌ AWS Authentication Failed.\n")
				fmt.Printf("   Error: %v\n\n", err)
				fmt.Println("   Typical causes:")
				fmt.Println("   1. AWS CLI not configured (run 'aws configure')")
				fmt.Println("   2. Expired SSO/MFA tokens")
				fmt.Println("   3. Invalid environment variables")
				fmt.Println("\n   (Use --mock to run without credentials)")
				os.Exit(1)
			}
			fmt.Printf("OK (Account: %s)\n", accountId)
		}

		// Check for AWS CLI.
		startTime := time.Now()
		var totalNodes int

		if _, err := exec.LookPath("aws"); err != nil {
			fmt.Println("[WARN] The 'aws' CLI is not found in your PATH.")
			fmt.Println("       Generated remediation scripts will require the AWS CLI to run.")
		}

		// Configure logging.
		var handler slog.Handler
		if config.JsonLogs {
			handler = slog.NewJSONHandler(os.Stdout, nil)
		} else if config.Verbose {
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		} else {
			handler = slog.NewTextHandler(io.Discard, nil)
		}
		config.Logger = slog.New(handler)

		// Determine cache directory.
		cacheDir, err := os.UserHomeDir()
		if err != nil {
			cacheDir = ".cloudslash"
		} else {
			cacheDir = filepath.Join(cacheDir, ".cloudslash")
		}
		config.CacheDir = cacheDir

		// Initialize pricing client.
		pricingClient, err := pricing.NewClient(cmd.Context(), config.Logger, config.CacheDir, config.DiscountRate)
		if err != nil {
			config.Logger.Debug("Pre-init pricing client failed", "error", err)
		}

		// Initialize engine.
		eng, err := engine.New(cmd.Context(),
			engine.WithLogger(config.Logger),
			engine.WithConfig(config),
			engine.WithPricing(pricingClient),
			engine.WithConcurrency(config.MaxConcurrency),
		)
		if err != nil {
			config.Logger.Error("Failed to initialize engine", "error", err)
			os.Exit(1)
		}

		success, g, swarmEngine, err := eng.Run(cmd.Context())
		if err != nil {
			config.Logger.Error("Pipeline failed", "error", err)
			os.Exit(1)
		}
		if !success {
			os.Exit(1)
		}

		if !config.Headless {
			model := ui.NewModel(swarmEngine, g, config.MockMode, config.Region)
			startTime := time.Now()
			p := tea.NewProgram(model)
			if _, err := p.Run(); err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}

			// Print summary.
			g.Mu.RLock()
			totalNodes = len(g.GetNodes())
			g.Mu.RUnlock()
			ui.PrintExitSummary(startTime, totalNodes)
		}

		if !config.Headless {
			// ...
			ui.PrintExitSummary(startTime, totalNodes)
		}

		runSolver(g)

		// Generate remediation artifacts.
		fmt.Printf("\n[INFO] Safe Remediation Plan generated at: %s/remediation_plan.json\n", config.OutputDir)
		fmt.Printf("       (Use the JSON plan with the CloudSlash Executor for safe removal)\n")

		// Generate restoration plan.
		restorePath := filepath.Join(config.OutputDir, "restore.tf")
		gen := script.NewGenerator(g, nil)
		if err := gen.GenerateRestorationPlan(restorePath); err != nil {
			fmt.Printf("[WARN] Failed to generate restoration plan: %v\n", err)
		} else {
			fmt.Printf("[SUCCESS] Lazarus Protocol Active: Restoration plan generated: %s\n", restorePath)
		}

		// Initialize Terraform analysis.
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
					for _, n := range g.GetNodes() {
						if n.IsWaste && !n.Ignored {
							unused = append(unused, n)
						}
					}
					g.Mu.RUnlock()

					// Analyze provenance.
					provEngine := provenance.NewEngine(".")
					provMap := make(map[string]*provenance.ProvenanceRecord)

					// Attribute waste to source.
					for _, z := range unused {
						rec, err := provEngine.Attribute(cmd.Context(), z.IDStr(), state)
						if err == nil && rec != nil {
							provMap[rec.TFAddress] = rec
						}
					}

					report := tf.Analyze(state, unused)

					printTerraformReport(report, provMap)
					generateFixScript(report)
				}
			}
			// Check for Partial Failures to signal CI/CD
			if err != nil && errors.Is(err, engine.ErrPartialResult) {
				fmt.Println("\n[WARN] Scan completed with partial failures (Strict Mode).")
				os.Exit(2)
			} else if config.StrictMode {
				// If strict mode is on but engine returned nil, check manual state just in case
				// (Though engine should have returned error)
				isPartial := false
				g.Mu.RLock()
				if g.Metadata.Partial {
					isPartial = true
				}
				g.Mu.RUnlock()
				if isPartial {
					fmt.Println("\n[WARN] Scan completed with partial failures. Check logs for details.")
					os.Exit(2)
				}
			} else {
				// Non-strict mode: check if partial just to warn user, but exit 0
				g.Mu.RLock()
				isPartial := g.Metadata.Partial
				g.Mu.RUnlock()
				if isPartial {
					fmt.Println("\n[WARN] Scan completed with partial failures. (Pass --strict to fail on this)")
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
	scanCmd.Flags().BoolVar(&config.StrictMode, "strict", false, "Exit with code 2 on partial failures (Strict Mode)")
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

		// Print provenance.
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
		fmt.Fprintf(f, "terraform state rm '%s'\n", mod)
	}

	for _, res := range report.ResourcesToDelete {
		fmt.Fprintf(f, "echo \"Removing Unused Resource: %s\"\n", res)
		fmt.Fprintf(f, "terraform state rm '%s'\n", res)
	}

	fmt.Fprintf(f, "\necho \"--------------------------------------------------\"\n")
	fmt.Fprintf(f, "echo \"State update complete. Run 'terraform plan' to verify.\"\n")
	_ = os.Chmod(f.Name(), 0755)
}

func runSolver(g *graph.Graph) {
	fmt.Printf("\n[ %s OPTIMIZATION ENGINE ]\n", version.Current)
	fmt.Println("Initializing Solver with Dynamic Intelligence...")

	// Setup Logger.
	var logger *slog.Logger
	if config.Verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Setup cache directory.
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cloudslash", "cache")
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		cacheDir = filepath.Join(xdg, "cloudslash")
	}
	_ = os.MkdirAll(cacheDir, 0755)

	// Initialize pricing client.
	logger.Info("Initializing Solver", "mock_mode", config.MockMode)

	var pc *pricing.Client
	var err error
	ctx := context.Background()

	if config.MockMode {
		fmt.Println(" -> [MOCK] Using static pricing estimation.")
	} else {
		done := make(chan bool)
		fmt.Printf(" -> Connecting to AWS Pricing API... ")
		go func() {
			chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			i := 0
			for {
				select {
				case <-done:
					return
				default:
					fmt.Printf("\r -> Connecting to AWS Pricing API... %s ", chars[i%len(chars)])
					time.Sleep(100 * time.Millisecond)
					i++
				}
			}
		}()

		manualRate := 0.0
		pc, err = pricing.NewClient(ctx, logger, cacheDir, manualRate)
		done <- true // Stop spinner
		fmt.Printf("\r -> Connecting to AWS Pricing API... Done.\n")

		if err != nil {
			fmt.Printf("[WARN] Pricing API unavailable: %v. Using static estimation.\n", err)
		} else {
			logger.Info("Pricing Client Initialized")
		}
	}

	// Calculate current spend.
	var workloads []*tetris.Item
	var currentSpend float64

	g.Mu.RLock()
	nodes := g.GetNodes()
	totalNodes := len(nodes)
	fmt.Printf(" -> Analyzing Current Spend (%d resources)...\n", totalNodes)

	for i, n := range nodes {
		if i%5 == 0 {
			fmt.Printf("\r    [%d/%d] Scanning resource: %s...", i+1, totalNodes, n.IDStr())
		}

		if n.TypeStr() == "AWS::EC2::Instance" {
			instanceType := "m5.large"
			if t, ok := n.Properties["Type"].(string); ok {
				instanceType = t
			}

			specs := aws.GetSpecs(instanceType)

			// Calculate cost.
			var cost float64
			var err error
			if pc != nil {
				cost, err = pc.GetEC2InstancePrice(ctx, internalconfig.DefaultRegion, instanceType)
			}
			if pc == nil || err != nil || cost == 0 {
				estimator := &aws.StaticCostEstimator{}
				cost = estimator.GetEstimatedCost(instanceType, internalconfig.DefaultRegion)
			}

			// Add to monthly spend.
			currentSpend += cost

			workloads = append(workloads, &tetris.Item{
				ID: n.IDStr(),
				Dimensions: tetris.Dimensions{
					CPU: specs.VCPU * 1000,
					RAM: specs.Memory,
				},
			})
		}
	}
	g.Mu.RUnlock()
	fmt.Printf("\r    [%d/%d] Graph Analysis Complete.                             \n", totalNodes, totalNodes)

	if len(workloads) == 0 {
		fmt.Println("No active compute workloads detected. Optimization skipped.")
		return
	}

	// Initialize solver.
	riskEngine := oracle.NewRiskEngine(internalconfig.DefaultRiskConfig())
	safePolicy := policy.DefaultPolicy()
	validator := policy.NewValidator(safePolicy)
	optimizer := solver.NewOptimizer(riskEngine, validator)

	// Build catalog.
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

		fmt.Printf("\r   [%d/%d] Analyzing: %-12s ", i+1, len(aws.CandidateTypes), it)

		if pc != nil {
			cost, err = pc.GetEC2InstancePrice(ctx, internalconfig.DefaultRegion, it)
		}

		if pc == nil || err != nil || cost == 0 {
			estimator := &aws.StaticCostEstimator{}
			cost = estimator.GetEstimatedCost(it, internalconfig.DefaultRegion)
			fallbackCount++
		} else {
			successCount++
		}

		// Monthly cost (730 hours).
		hourlyCost := cost / 730.0

		catalog = append(catalog, solver.InstanceType{
			Name:       it,
			CPU:        specs.VCPU * 1000,
			RAM:        specs.Memory,
			HourlyCost: hourlyCost,
			Zone:       internalconfig.DefaultRegion + "a", // Default zone placement.
		})
		
		if pc == nil {
			time.Sleep(20 * time.Millisecond)
		}
	}
	fmt.Printf("\n > Catalog Complete. Live Prices: %d | Estimates: %d\n", successCount, fallbackCount)

	// Execute optimization.
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

	// Print results.
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
