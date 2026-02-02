package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/forensics"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/heuristics"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/notifier"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/pricing"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/remediation"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/report"
	"github.com/DrSkyle/cloudslash/v2/pkg/providers/k8s"
	"github.com/DrSkyle/cloudslash/v2/pkg/providers/tf"
)

func runRealPipeline(ctx context.Context, e *Engine) <-chan struct{} {
	done := make(chan struct{})
	var err error

	// Init pricing.
	if e.Pricing == nil {
		e.Pricing, err = pricing.NewClient(ctx, e.Logger, e.config.CacheDir, e.config.DiscountRate)
		if err != nil {
			e.Logger.Warn("Pricing Client initialization failed", "error", err)
		}
	}

	profiles := []string{""}
	if e.config.AllProfiles {
		var err error
		profiles, err = aws.ListProfiles()
		if err != nil {
			e.Logger.Warn("Failed to list profiles. Using default", "error", err)
			profiles = []string{"default"}
		} else {
			e.Logger.Info("Deep Scanning enabled", "profiles", len(profiles))
		}
	}

	var scanWg sync.WaitGroup
	var cwClient *aws.CloudWatchClient
	var iamClient *aws.IAMClient
	var ctClient *aws.CloudTrailClient
	var logsClient *aws.CloudWatchLogsClient
	var ecsScanner *aws.ECSScanner
	var ecrScanner *aws.ECRScanner

	// Phase 1.
	for _, profile := range profiles {
		if e.config.AllProfiles {
			e.Logger.Info("Scanning Profile", "profile", profile)
		}

		regions := strings.Split(e.config.Region, ",")
		for _, region := range regions {
			region = strings.TrimSpace(region)
			if region == "" {
				continue
			}

			client, err := runScanForProfile(ctx, region, profile, e.config.Verbose, e.Graph, e.Swarm, &scanWg)
			if err != nil {
				e.Logger.Error("Scan failed", "profile", profile, "region", region, "error", err)
				continue
			}

			if client != nil {
				cwClient = aws.NewCloudWatchClient(client.Config)
				iamClient = aws.NewIAMClient(client.Config)
				ctClient = aws.NewCloudTrailClient(client.Config)
				logsClient = aws.NewCloudWatchLogsClient(client.Config, e.Graph, e.config.DisableCWMetrics)
				ecsScanner = aws.NewECSScanner(client.Config, e.Graph)
				ecrScanner = aws.NewECRScanner(client.Config, e.Graph)
			}
		}
	}

	go func() {
		defer close(done)
		scanWg.Wait()

		// Finalize ingestion.
		// NOTE: We do NOT close the graph here as heuristics may need to add edges.
		// e.Graph.CloseAndWait()

		if logsClient != nil {
			logsClient.ScanLogGroups(context.Background())
		}

		if ecrScanner != nil {
			ecrScanner.ScanRepositories(context.Background())
		}

		// Reconcile state.
		var state *tf.State
		state, err = tf.LoadState(context.Background(), e.config.TFStatePath)
		if err == nil {
			detector := tf.NewDriftDetector(e.Graph, state)
			detector.ScanForDrift()

			cwd, _ := os.Getwd()
			auditor := tf.NewCodeAuditor(state)

			e.Graph.Mu.Lock()
			for _, node := range e.Graph.GetNodes() {
				if node.IsWaste {
					file, line, err := auditor.FindSource(node.IDStr(), cwd)
					if err == nil {
						node.SourceLocation = fmt.Sprintf("%s:%d", file, line)
					}
				}
			}
			e.Graph.Mu.Unlock()
		}

		// Phase 2.
		hEngine := heuristics.NewEngine()

		if cwClient != nil {
			hEngine.Register(&heuristics.RDSHeuristic{CW: cwClient})
			if e.Pricing != nil {
				hEngine.Register(&heuristics.UnderutilizedInstanceHeuristic{CW: cwClient, Pricing: e.Pricing})
			}
		}

		if e.Pricing != nil {
			hEngine.Register(&heuristics.UnattachedVolumeHeuristic{Pricing: e.Pricing, Config: e.config.Heuristics.UnattachedVolume})
		} else {
			hEngine.Register(&heuristics.UnattachedVolumeHeuristic{Config: e.config.Heuristics.UnattachedVolume})
		}

		if e.config.RequiredTags != "" {
			hEngine.Register(&heuristics.TagComplianceHeuristic{RequiredTags: strings.Split(e.config.RequiredTags, ",")})
		}

		if iamClient != nil {
			hEngine.Register(&heuristics.IAMHeuristic{IAM: iamClient})
		}

		hEngine.Register(&heuristics.LogHoardersHeuristic{})
		hEngine.Register(&heuristics.ECRJanitorHeuristic{})
		hEngine.Register(&heuristics.DataForensicsHeuristic{})
		hEngine.Register(&heuristics.LambdaHeuristic{})
		hEngine.Register(&heuristics.NetworkForensicsHeuristic{})
		hEngine.Register(&heuristics.StorageOptimizationHeuristic{})
		hEngine.Register(&heuristics.EBSModernizerHeuristic{})
		hEngine.Register(&heuristics.GhostNodeGroupHeuristic{})
		hEngine.Register(&heuristics.AgedAMIHeuristic{})

		// Register ECS heuristics.
		hEngine.Register(&heuristics.IdleClusterHeuristic{Config: e.config.Heuristics.IdleCluster})
		hEngine.Register(&heuristics.EmptyServiceHeuristic{ECR: ecrScanner, ECS: ecsScanner})

		if k8sClient, err := k8s.NewClient(); err == nil {
			hEngine.Register(&heuristics.AbandonedFargateHeuristic{K8sClient: k8sClient})
		} else {
			hEngine.Register(&heuristics.AbandonedFargateHeuristic{K8sClient: nil})
		}

		if err := hEngine.Run(ctx, e.Graph); err != nil {
			e.Logger.Error("Deep Analysis failed", "error", err)
		}

		// Phase 3.
		hEngine2 := heuristics.NewEngine()
		if e.Pricing != nil {
			hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{Pricing: e.Pricing})
		} else {
			hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
		}
		if err := hEngine2.Run(ctx, e.Graph); err != nil {
			e.Logger.Error("Time Machine Analysis failed", "error", err)
		}

		// Phase 4.
		// Safe to close graph now.
		e.Graph.CloseAndWait()

		if e.config.RulesFile != "" {
			e.Logger.Info("Initializing Policy Engine", "rules_file", e.config.RulesFile)
			if err := runPolicyEngine(ctx, e.config.RulesFile, e.Graph); err != nil {
				e.Logger.Error("Policy Engine failed", "error", err)
			}
		}

		// Phase 5.
		detective := forensics.NewDetective(ctClient)
		detective.InvestigateGraph(ctx, e.Graph)

		// Phase 6.
		os.Mkdir(e.outputDir, 0755)

		report.GenerateCSV(e.Graph, e.outputDir+"/waste_report.csv")
		report.GenerateJSON(e.Graph, e.outputDir+"/waste_report.json")

		gen := tf.NewGenerator(e.Graph, state)
		gen.GenerateWasteTF(e.outputDir + "/waste.tf")
		gen.GenerateImportScript(e.outputDir + "/import.sh")
		gen.GenerateDestroyPlan(e.outputDir + "/destroy_plan.out")

		gen.GenerateFixScript(e.outputDir + "/fix_terraform.sh")
		os.Chmod(e.outputDir+"/fix_terraform.sh", 0755)

		// Generate remediation plan.
		remGen := remediation.NewGenerator(e.Graph, e.Logger)
		planPath := filepath.Join(e.outputDir, "remediation_plan.json")
		if err := remGen.GenerateRemediationPlan(planPath); err != nil {
			e.Logger.Error("Failed to generate remediation plan", "error", err)
		} else {
			e.Logger.Info("Remediation Plan Generated", "path", planPath)
		}

		_ = remGen.GenerateIgnorePlan(e.outputDir + "/ignore_plan.json")
		_ = remGen.GenerateRestorationPlan(e.outputDir + "/restoration_plan.json")

		if err := report.GenerateDashboard(e.Graph, e.outputDir+"/dashboard.html"); err != nil {
			e.Logger.Error("Failed to generate dashboard", "error", err)
		}

		report.GenerateExecutiveSummary(e.Graph, e.outputDir+"/executive_summary.md", fmt.Sprintf("cs-scan-%d", time.Now().Unix()), "AWS-ACCOUNT")

		// Report summary.
		summary := report.Summary{
			Region:       e.config.Region,
			TotalScanned: len(e.Graph.GetNodes()),
			TotalWaste:   0,
			TotalSavings: 0,
		}

		e.Graph.Mu.RLock()
		for _, n := range e.Graph.GetNodes() {
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
		if e.config.SlackWebhook != "" && e.config.Headless {
			e.Logger.Info("Transmitting Cost Report to Slack")
			client := notifier.NewSlackClient(e.config.SlackWebhook, e.config.SlackChannel)

			if err := client.SendAnalysisReport(summary); err != nil {
				e.Logger.Warn("Failed to send Slack report", "error", err)
			} else {
				e.Logger.Info("Slack Report delivered")
			}
		}

		// Historical analysis.
		var slackClient *notifier.SlackClient
		if e.config.SlackWebhook != "" {
			slackClient = notifier.NewSlackClient(e.config.SlackWebhook, e.config.SlackChannel)
		}
		performSignalAnalysis(e.Graph, slackClient, e.History)

		// Check partial results.
		e.Graph.Mu.RLock()
		if e.Graph.Metadata.Partial {
			e.Logger.Warn("Partial Graph Results: Graph incomplete due to API errors")
			for _, failure := range e.Graph.Metadata.FailedScopes {
				e.Logger.Warn("Graph Scope Failed", "scope", failure.Scope, "error", failure.Error)
			}
		}
		e.Graph.Mu.RUnlock()

		// 7. Artifact Persistence (S3)
		if e.s3Target != "" {
			if err := e.UploadArtifacts(context.Background()); err != nil {
				e.Logger.Error("Failed to persist artifacts to S3", "target", e.s3Target, "error", err)
			} else {
				e.Logger.Info("Artifacts successfully uploaded to S3", "target", e.s3Target)
			}
		}
	}()

	return done
}
