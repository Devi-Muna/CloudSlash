package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	internalconfig "github.com/DrSkyle/cloudslash/v2/pkg/config"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/heuristics"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/notifier"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/remediation"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/report"
	"github.com/DrSkyle/cloudslash/v2/pkg/providers/tf"
)

func runMockMode(ctx context.Context, e *Engine) {
	fmt.Println("DEBUG: Starting Mock Mode...")
	mockScanner := aws.NewMockScanner(e.Graph)

	// Seed data.
	fmt.Println("DEBUG: Seeding mock data...")
	e.History.SeedMockData()

	fmt.Println("DEBUG: Running Mock Scanner...")
	mockScanner.Scan(ctx)

	// Register heuristics.
	heuristicEngine := heuristics.NewEngine()
	heuristicEngine.Register(&heuristics.UnattachedVolumeHeuristic{Config: internalconfig.DefaultHeuristicConfig().UnattachedVolume})
	heuristicEngine.Register(&heuristics.S3MultipartHeuristic{Config: internalconfig.DefaultHeuristicConfig().S3Multipart})
	heuristicEngine.Register(&heuristics.IdleClusterHeuristic{Config: internalconfig.DefaultHeuristicConfig().IdleCluster})
	heuristicEngine.Register(&heuristics.EmptyServiceHeuristic{})
	heuristicEngine.Register(&heuristics.IdleEKSClusterHeuristic{})
	heuristicEngine.Register(&heuristics.GhostNodeGroupHeuristic{})
	heuristicEngine.Register(&heuristics.ElasticIPHeuristic{})
	heuristicEngine.Register(&heuristics.RDSHeuristic{})
	heuristicEngine.Register(&heuristics.AgedAMIHeuristic{})

	heuristicEngine.Register(&heuristics.NetworkForensicsHeuristic{})
	heuristicEngine.Register(&heuristics.StorageOptimizationHeuristic{})
	heuristicEngine.Register(&heuristics.EBSModernizerHeuristic{})

	fmt.Println("DEBUG: Running Heuristics...")
	if err := heuristicEngine.Run(ctx, e.Graph); err != nil {
		e.Logger.Warn("Heuristic run failed", "error", err)
	}
	fmt.Println("DEBUG: Heuristics run complete.")

	// Init policies.
	if e.config.RulesFile != "" {
		e.Logger.Info("Initializing Policy Engine", "rules_file", e.config.RulesFile)
		if err := runPolicyEngine(ctx, e.config.RulesFile, e.Graph); err != nil {
			e.Logger.Error("Policy Engine failed", "error", err)
		}
	}

	hEngine2 := heuristics.NewEngine()
	hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
	hEngine2.Run(ctx, e.Graph)

	// Finalize graph.
	e.Graph.CloseAndWait()

	os.Mkdir(e.outputDir, 0755)

	// Generate outputs.
	report.GenerateCSV(e.Graph, e.outputDir+"/waste_report.csv")
	report.GenerateJSON(e.Graph, e.outputDir+"/waste_report.json")

	// Generate dashboard.
	if err := report.GenerateDashboard(e.Graph, e.outputDir+"/dashboard.html"); err != nil {
		fmt.Printf("Failed to generate dashboard: %v\n", err)
	}

	// Generate remediation.
	gen := tf.NewGenerator(e.Graph, nil)
	gen.GenerateFixScript(e.outputDir + "/fix_terraform.sh")
	os.Chmod(e.outputDir+"/fix_terraform.sh", 0755)

	// Generate artifacts.
	gen.GenerateWasteTF(e.outputDir + "/waste.tf")
	gen.GenerateImportScript(e.outputDir + "/import.sh")
	gen.GenerateDestroyPlan(e.outputDir + "/destroy_plan.out")

	// Generate plans.
	remGen := remediation.NewGenerator(e.Graph, e.Logger)
	remGen.GenerateRemediationPlan(e.outputDir + "/remediation_plan.json")
	remGen.GenerateIgnorePlan(e.outputDir + "/ignore_plan.json")
	remGen.GenerateRestorationPlan(e.outputDir + "/restoration_plan.json")

	// Generate summary.
	report.GenerateExecutiveSummary(e.Graph, e.outputDir+"/executive_summary.md", fmt.Sprintf("cs-mock-%d", time.Now().Unix()), "MOCK-ACCOUNT-123")

	// Report summary.
	count := len(e.Graph.GetNodes())
	
	summary := report.Summary{
		Region:       e.config.Region,
		TotalScanned: count,
		TotalWaste:   0,
		TotalSavings: 0,
	}

	e.Graph.Mu.RLock()
	nodes := e.Graph.Store.GetAllNodes()
	for _, n := range nodes {
		if n.IsWaste {
			summary.TotalWaste++
			summary.TotalSavings += n.Cost
		}
	}
	e.Graph.Mu.RUnlock()

	// CI decoration.
	ci := report.NewCIDecorator(e.Logger)
	if err := ci.Run(summary, e.Graph); err != nil {
		e.Logger.Error("CI Decoration failed", "error", err)
	}

	// Slack notification.
	var slackClient *notifier.SlackClient
	if e.config.SlackWebhook != "" && e.config.Headless {
		fmt.Println(" -> Transmitting Cost Report to Slack (MOCK)...")
		slackClient = notifier.NewSlackClient(e.config.SlackWebhook, e.config.SlackChannel)
		slackClient.SendAnalysisReport(summary)
	}
	// Analyze.
	performSignalAnalysis(e.Graph, slackClient, e.History)

	// E2E check.
	if os.Getenv("CLOUDSLASH_E2E") == "true" {
		fmt.Println("[E2E] Verifying Graph Integrity...")
		e.Graph.Mu.RLock()
		nodeCount := len(e.Graph.Store.GetAllNodes())
		e.Graph.Mu.RUnlock()

		// Expect at least 1 mock resource (we seed ~7 in mock.go)
		if nodeCount < 5 {
			fmt.Printf("[E2E] FAILURE: Expected >5 nodes, got %d\n", nodeCount)
			os.Exit(1)
		}
		fmt.Println("[E2E] SUCCESS: Graph state valid.")
	}
}
