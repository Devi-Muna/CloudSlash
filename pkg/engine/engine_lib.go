package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/engine/aws"
	internalconfig "github.com/DrSkyle/cloudslash/pkg/config"
	"github.com/DrSkyle/cloudslash/pkg/engine/forensics"
	"github.com/DrSkyle/cloudslash/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/DrSkyle/cloudslash/pkg/engine/heuristics"
	"github.com/DrSkyle/cloudslash/pkg/engine/history"
	"github.com/DrSkyle/cloudslash/pkg/providers/k8s"
	"github.com/DrSkyle/cloudslash/pkg/engine/notifier"
	"github.com/DrSkyle/cloudslash/pkg/engine/pricing"
	"github.com/DrSkyle/cloudslash/pkg/engine/remediation"
	"github.com/DrSkyle/cloudslash/pkg/engine/report"
	"github.com/DrSkyle/cloudslash/pkg/engine/swarm"
	"github.com/DrSkyle/cloudslash/pkg/providers/tf"
	"github.com/DrSkyle/cloudslash/pkg/telemetry"
	"github.com/DrSkyle/cloudslash/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// Config defines the engine execution parameters.
type Config struct {
	Region           string
	TFStatePath      string
	MockMode         bool
	AllProfiles      bool
	RequiredTags     string
	SlackWebhook     string
	SlackChannel     string
	Headless         bool
	DisableCWMetrics bool
	Verbose          bool
	MaxConcurrency   int
	JsonLogs         bool
	RulesFile        string
	HistoryURL       string // "s3://bucket/key" or empty for local
	OutputDir        string // Directory for generated artifacts
	Heuristics       internalconfig.HeuristicConfig
	
	// Pricing overrides.
	DiscountRate float64 // Manual EDP/RI rate (e.g. 0.82)

	// Telemetry configuration.
	OtelEndpoint  string // "http://localhost:4318" or via env
	SkipTelemetry bool   // Set true if embedding in an app that already has OTEL

	// Dependency injection.
	Logger   *slog.Logger
	CacheDir string
}

func Run(ctx context.Context, cfg Config) (bool, *graph.Graph, *swarm.Engine, error) {
	// Set default OutputDir
	if cfg.OutputDir == "" {
		cfg.OutputDir = "cloudslash-out"
	}

	// Initialize logger.
	if cfg.Logger == nil {
		// Fallback logger for testing.
		var handler slog.Handler
		if cfg.JsonLogs {
			handler = slog.NewJSONHandler(os.Stdout, nil)
		} else {
			handler = slog.NewTextHandler(os.Stdout, nil)
		}
		cfg.Logger = slog.New(handler)
	}
	slog.SetDefault(cfg.Logger)

	// Initialize Telemetry (Golden Signal A).
	if !cfg.SkipTelemetry {
		shutdown, telErr := telemetry.Init(ctx, version.AppName, version.Current, cfg.OtelEndpoint)
		if telErr != nil {
			slog.Warn("Failed to initialize telemetry (Mode A fallback)", "error", telErr)
		} else {
			defer func() {
				slog.Debug("Flushing telemetry...")
				shutdown(context.Background())
			}()
		}
	}

	if !cfg.Headless && !cfg.JsonLogs {
		fmt.Printf("%s %s [%s]\n", version.AppName, version.Current, version.License)
	}

	// Configure panic recovery.
	var err error
	defer func() {
		if r := recover(); r != nil {
			// Crash Handler: Intercept panics.
			tr := otel.Tracer("cloudslash/engine")
			// Use independent context for critical reporting.
			_, span := tr.Start(ctx, "CriticalPanic")
			
			stack := debug.Stack()
			
			// Record Exception in OTEL
			span.RecordError(fmt.Errorf("%v", r), trace.WithStackTrace(true))
			span.SetStatus(codes.Error, "CRITICAL FAILURE")
			span.SetAttributes(
				attribute.String("crash.stack", string(stack)),
				attribute.String("crash.reason", fmt.Sprintf("%v", r)),
			)
			span.End()

			// Return structured error
			err = fmt.Errorf("CRITICAL FAILURE: %v", r)
			
			// Log to Stdout (Container/Serverless friendly)
			slog.Error("CRITICAL FAILURE", "error", r, "stack", string(stack))
		}
	}()

	// Initialize graph and engine.
	var g *graph.Graph
	var engine *swarm.Engine

	g = graph.NewGraph()
	engine = swarm.NewEngine()
	if cfg.MaxConcurrency > 0 {
		engine.MaxWorkers = cfg.MaxConcurrency
	}

	// Initialize history backend.
	// Initialize History Client
	var backend history.Backend
	if strings.HasPrefix(cfg.HistoryURL, "s3://") {
		var err error
		backend, err = history.NewS3Backend(cfg.HistoryURL)
		if err != nil {
			slog.Error("Failed to initialize S3 History Backend", "error", err)
		} else {
			slog.Info("Using S3 History Backend", "url", cfg.HistoryURL)
		}
	}
	historyClient := history.NewClient(backend)

	engine.Start(ctx)

	var doneChan <-chan struct{}

	if cfg.MockMode {
		runMockMode(ctx, cfg, g, engine, historyClient)
	} else {
		doneChan = runRealMode(ctx, cfg, g, engine, historyClient)
	}

	// Wait for scan to complete in non-headless mode (if applicable) or return.
	if doneChan != nil {
		<-doneChan
	}

	return true, g, engine, err
}

func runMockMode(ctx context.Context, cfg Config, g *graph.Graph, engine *swarm.Engine, hClient *history.Client) {
	mockScanner := aws.NewMockScanner(g)

	// Seed mock history data.
	hClient.SeedMockData()

	mockScanner.Scan(ctx)

	// Register mock heuristics.
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

	if err := heuristicEngine.Run(ctx, g); err != nil {
		fmt.Printf("Heuristic run failed: %v\n", err)
	}

	// ---------------------------------------------------------
	// Initialize Enterprise Policy Engine (CEL).
	// ---------------------------------------------------------
	if cfg.RulesFile != "" {
		slog.Info("Initializing Policy Engine", "rules_file", cfg.RulesFile)
		if err := runPolicyEngine(ctx, cfg.RulesFile, g); err != nil {
			slog.Error("Policy Engine failed", "error", err)
		}
	}

	hEngine2 := heuristics.NewEngine()
	hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
	hEngine2.Run(ctx, g)

	os.Mkdir(cfg.OutputDir, 0755)

	// Generate outputs.
	report.GenerateCSV(g, cfg.OutputDir+"/waste_report.csv")
	report.GenerateJSON(g, cfg.OutputDir+"/waste_report.json")

	// Generate dashboard.
	if err := report.GenerateDashboard(g, cfg.OutputDir+"/dashboard.html"); err != nil {
		fmt.Printf("Failed to generate dashboard: %v\n", err)
	}

	// Generate remediation scripts.
	gen := tf.NewGenerator(g, nil)
	gen.GenerateFixScript(cfg.OutputDir+"/fix_terraform.sh")
	os.Chmod(cfg.OutputDir+"/fix_terraform.sh", 0755)

	// Generate mock artifacts.
	gen.GenerateWasteTF(cfg.OutputDir+"/waste.tf")
	gen.GenerateImportScript(cfg.OutputDir+"/import.sh")
	gen.GenerateDestroyPlan(cfg.OutputDir+"/destroy_plan.out")

	remGen := remediation.NewGenerator(g)
	remGen.GenerateSafeDeleteScript(cfg.OutputDir+"/safe_cleanup.sh")
	os.Chmod(cfg.OutputDir+"/safe_cleanup.sh", 0755)

	remGen.GenerateIgnoreScript(cfg.OutputDir+"/ignore_resources.sh")
	os.Chmod(cfg.OutputDir+"/ignore_resources.sh", 0755)

	remGen.GenerateUndoScript(cfg.OutputDir+"/undo_cleanup.sh")
	os.Chmod(cfg.OutputDir+"/undo_cleanup.sh", 0755)

	// Generate Executive Summary.
	report.GenerateExecutiveSummary(g, cfg.OutputDir+"/executive_summary.md", fmt.Sprintf("cs-mock-%d", time.Now().Unix()), "MOCK-ACCOUNT-123")

	// Generate Report Summary.
	summary := report.Summary{
		Region:       cfg.Region,
		TotalScanned: len(g.Nodes),
		TotalWaste:   0,
		TotalSavings: 0,
	}

	g.Mu.RLock()
	for _, n := range g.Nodes {
		if n.IsWaste {
			summary.TotalWaste++
			summary.TotalSavings += n.Cost
		}
	}
	g.Mu.RUnlock()

	// Decorate CI/CD environment.
	ci := report.NewCIDecorator(cfg.Logger)
	if err := ci.Run(summary, g); err != nil {
		cfg.Logger.Error("CI Decoration failed", "error", err)
	}

	// Send Slack notification.
	var slackClient *notifier.SlackClient
	if cfg.SlackWebhook != "" && cfg.Headless {
		fmt.Println(" -> Transmitting Cost Report to Slack (MOCK)...")
		slackClient = notifier.NewSlackClient(cfg.SlackWebhook, cfg.SlackChannel)
		slackClient.SendAnalysisReport(summary)
	}
	// Perform historical analysis.
	performSignalAnalysis(g, slackClient, hClient)

	// Execute E2E validation.
	if os.Getenv("CLOUDSLASH_E2E") == "true" {
		fmt.Println("[E2E] Verifying Graph Integrity...")
		g.Mu.RLock()
		nodeCount := len(g.Nodes)
		g.Mu.RUnlock()
		
		// Expect at least 1 mock resource (we seed ~7 in mock.go)
		if nodeCount < 5 {
			fmt.Printf("[E2E] FAILURE: Expected >5 nodes, got %d\n", nodeCount)
			os.Exit(1)
		}
		fmt.Println("[E2E] SUCCESS: Graph state valid.")
	}
}

func runRealMode(ctx context.Context, cfg Config, g *graph.Graph, engine *swarm.Engine, hClient *history.Client) <-chan struct{} {
	done := make(chan struct{})

	var pricingClient *pricing.Client
	var err error
	// Configure pricing client with discount rate.
	pricingClient, err = pricing.NewClient(ctx, cfg.Logger, cfg.CacheDir, cfg.DiscountRate)
	if err != nil {
		// Log pricing initialization failures.
		// fmt.Printf("Warning: Failed to initialize Pricing Client: %v\n", err)
	}

	profiles := []string{""}
	if cfg.AllProfiles {
		var err error
		profiles, err = aws.ListProfiles()
		if err != nil {
			fmt.Printf("Failed to list profiles: %v. Using default.\n", err)
			profiles = []string{"default"}
		} else {
			fmt.Printf("Deep Scanning enabled. Found %d profiles.\n", len(profiles))
		}
	}

	var scanWg sync.WaitGroup
	var cwClient *aws.CloudWatchClient
	var iamClient *aws.IAMClient
	var ctClient *aws.CloudTrailClient
	var logsClient *aws.CloudWatchLogsClient
	var ecsScanner *aws.ECSScanner
	var ecrScanner *aws.ECRScanner

	for _, profile := range profiles {
		if cfg.AllProfiles {
			fmt.Printf(">>> Scanning Profile: %s\n", profile)
		}

		regions := strings.Split(cfg.Region, ",")
		for _, region := range regions {
			region = strings.TrimSpace(region)
			if region == "" {
				continue
			}

			client, err := runScanForProfile(ctx, region, profile, cfg.Verbose, g, engine, &scanWg)
			if err != nil {
				fmt.Printf("Scan failed for profile %s region %s: %v\n", profile, region, err)
				continue
			}

			if client != nil {
				cwClient = aws.NewCloudWatchClient(client.Config)
				iamClient = aws.NewIAMClient(client.Config)
				ctClient = aws.NewCloudTrailClient(client.Config)
				logsClient = aws.NewCloudWatchLogsClient(client.Config, g, cfg.DisableCWMetrics)
				ecsScanner = aws.NewECSScanner(client.Config, g)
				ecrScanner = aws.NewECRScanner(client.Config, g)
			}
		}
	}

	go func() {
		defer close(done)
		scanWg.Wait()
		
		// Finalize ingestion.
		g.CloseAndWait()

		if logsClient != nil {
			logsClient.ScanLogGroups(context.Background())
		}

		if ecrScanner != nil {
			ecrScanner.ScanRepositories(context.Background())
		}

// Reconcile with Terraform state.
		var state *tf.State
		var err error
		// Load Terraform state.
		state, err = tf.LoadState(context.Background(), cfg.TFStatePath)

		if err == nil {
			if err == nil {
				detector := tf.NewDriftDetector(g, state)
				detector.ScanForDrift()

				cwd, _ := os.Getwd()
				auditor := tf.NewCodeAuditor(state)

				g.Mu.Lock()
				for _, node := range g.Nodes {
					if node.IsWaste {
						file, line, err := auditor.FindSource(node.ID, cwd)
						if err == nil {
							node.SourceLocation = fmt.Sprintf("%s:%d", file, line)
						}
					}
				}
				g.Mu.Unlock()
			}
		}

		hEngine := heuristics.NewEngine()

		if cwClient != nil {
			hEngine.Register(&heuristics.RDSHeuristic{CW: cwClient})
			if pricingClient != nil {
				hEngine.Register(&heuristics.UnderutilizedInstanceHeuristic{CW: cwClient, Pricing: pricingClient})
			}
		}

		if pricingClient != nil {
			hEngine.Register(&heuristics.UnattachedVolumeHeuristic{Pricing: pricingClient, Config: cfg.Heuristics.UnattachedVolume})
		} else {
			hEngine.Register(&heuristics.UnattachedVolumeHeuristic{Config: cfg.Heuristics.UnattachedVolume})
		}

		if cfg.RequiredTags != "" {
			hEngine.Register(&heuristics.TagComplianceHeuristic{RequiredTags: strings.Split(cfg.RequiredTags, ",")})
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
		hEngine.Register(&heuristics.IdleClusterHeuristic{Config: cfg.Heuristics.IdleCluster})
		hEngine.Register(&heuristics.EmptyServiceHeuristic{ECR: ecrScanner, ECS: ecsScanner})

		if k8sClient, err := k8s.NewClient(); err == nil {
			hEngine.Register(&heuristics.AbandonedFargateHeuristic{K8sClient: k8sClient})
		} else {
			hEngine.Register(&heuristics.AbandonedFargateHeuristic{K8sClient: nil})
		}

		if err := hEngine.Run(ctx, g); err != nil {
			fmt.Printf("Deep Analysis failed: %v\n", err)
		}

		// Analyze snapshot history.
		hEngine2 := heuristics.NewEngine()
		if pricingClient != nil {
			hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{Pricing: pricingClient})
		} else {
			hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
		}
		if err := hEngine2.Run(ctx, g); err != nil {
			fmt.Printf("Time Machine Analysis failed: %v\n", err)
		}

		// ---------------------------------------------------------
		// Initialize Enterprise Policy Engine (CEL).
		// ---------------------------------------------------------
		if cfg.RulesFile != "" {
			slog.Info("Initializing Policy Engine", "rules_file", cfg.RulesFile)
			if err := runPolicyEngine(ctx, cfg.RulesFile, g); err != nil {
				slog.Error("Policy Engine failed", "error", err)
			}
		}

		detective := forensics.NewDetective(ctClient)
		detective.InvestigateGraph(ctx, g)

		// Generate output files.
		os.Mkdir(cfg.OutputDir, 0755)

		// 1. Report Generation
		report.GenerateCSV(g, cfg.OutputDir+"/waste_report.csv")
		report.GenerateJSON(g, cfg.OutputDir+"/waste_report.json")

		// 2. Remediation & Planning
		gen := tf.NewGenerator(g, state)
		gen.GenerateWasteTF(cfg.OutputDir+"/waste.tf")
		gen.GenerateImportScript(cfg.OutputDir+"/import.sh")
		gen.GenerateDestroyPlan(cfg.OutputDir+"/destroy_plan.out")

		// Remediation Scripts
		gen.GenerateFixScript(cfg.OutputDir+"/fix_terraform.sh")
		os.Chmod(cfg.OutputDir+"/fix_terraform.sh", 0755)

		remGen := remediation.NewGenerator(g)
		remGen.GenerateSafeDeleteScript(cfg.OutputDir+"/safe_cleanup.sh")
		os.Chmod(cfg.OutputDir+"/safe_cleanup.sh", 0755)

		remGen.GenerateIgnoreScript(cfg.OutputDir+"/ignore_resources.sh")
		os.Chmod(cfg.OutputDir+"/ignore_resources.sh", 0755)

		remGen.GenerateUndoScript(cfg.OutputDir+"/undo_cleanup.sh")
		os.Chmod(cfg.OutputDir+"/undo_cleanup.sh", 0755)

		if err := report.GenerateDashboard(g, cfg.OutputDir+"/dashboard.html"); err != nil {
			fmt.Printf("Failed to generate dashboard: %v\n", err)
		}

		// Executive Summary
		report.GenerateExecutiveSummary(g, cfg.OutputDir+"/executive_summary.md", fmt.Sprintf("cs-scan-%d", time.Now().Unix()), "AWS-ACCOUNT")

		// Generate Report Summary (Shared by CI and Slack)
		summary := report.Summary{
			Region:       cfg.Region,
			TotalScanned: len(g.Nodes), // Approximate
			TotalWaste:   0,
			TotalSavings: 0,
		}

		g.Mu.RLock()
		for _, n := range g.Nodes {
			if n.IsWaste {
				summary.TotalWaste++
				summary.TotalSavings += n.Cost
			}
		}
		g.Mu.RUnlock()

		// 1. CI/CD Decoration (Native PR Comments)
		// Detects GitHub/GitLab and posts Report directly to PR.
		ci := report.NewCIDecorator(cfg.Logger)
		if err := ci.Run(summary, g); err != nil {
			cfg.Logger.Error("CI Decoration failed", "error", err)
		}

		// 2. Slack Notification
		if cfg.SlackWebhook != "" && cfg.Headless {
			fmt.Println(" -> Transmitting Cost Report to Slack...")
			client := notifier.NewSlackClient(cfg.SlackWebhook, cfg.SlackChannel)

			if err := client.SendAnalysisReport(summary); err != nil {
				fmt.Printf(" [WARN] Failed to send Slack report: %v\n", err)
			} else {
				fmt.Println(" [SUCCESS] Report delivered.")
			}
		}



		// Analyze cost signals.
		var slackClient *notifier.SlackClient
		if cfg.SlackWebhook != "" {
			slackClient = notifier.NewSlackClient(cfg.SlackWebhook, cfg.SlackChannel)
		}
		performSignalAnalysis(g, slackClient, hClient)

		// Check for partial graph results.
		g.Mu.RLock()
		if g.Metadata.Partial {
			fmt.Println("\n[ WARNING: PARTIAL GRAPH ]")
			fmt.Println(" The graph is incomplete due to missing permissions or API errors.")
			fmt.Println(" Reachability analysis may be inaccurate.")
			for _, failure := range g.Metadata.FailedScopes {
				fmt.Printf(" - %s: %s\n", failure.Scope, failure.Error)
			}
			fmt.Println("----------------------------------------------------------")
		}
		g.Mu.RUnlock()
	}()

	return done
}

func runScanForProfile(ctx context.Context, region, profile string, verbose bool, g *graph.Graph, engine *swarm.Engine, scanWg *sync.WaitGroup) (*aws.Client, error) {
	awsClient, err := aws.NewClient(ctx, region, profile, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %v", err)
	}

	identity, err := awsClient.VerifyIdentity(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "no EC2 IMDS role found") || strings.Contains(err.Error(), "failed to get caller identity") {
			return nil, fmt.Errorf("\n[ERROR] Unable to find AWS Credentials.\n   Please run 'aws configure' or set AWS_PROFILE.\n   (Error: %v)", err)
		}
		return nil, fmt.Errorf("failed to verify identity: %v", err)
	}
	slog.Info("Connected to AWS", "profile", profile, "account", identity)

	// Scanners
	ec2Scanner := aws.NewEC2Scanner(awsClient.Config, g)
	s3Scanner := aws.NewS3Scanner(awsClient.Config, g)
	rdsScanner := aws.NewRDSScanner(awsClient.Config, g)
	eksScanner := aws.NewEKSScanner(awsClient.Config, g)
	natScanner := aws.NewNATScanner(awsClient.Config, g)
	eipScanner := aws.NewEIPScanner(awsClient.Config, g)
	albScanner := aws.NewALBScanner(awsClient.Config, g)
	vpcepScanner := aws.NewVpcEndpointScanner(awsClient.Config, g)
	ecsScanner := aws.NewECSScanner(awsClient.Config, g)
	elasticacheScanner := aws.NewElasticacheScanner(awsClient.Config, g)
	redshiftScanner := aws.NewRedshiftScanner(awsClient.Config, g)
	dynamoScanner := aws.NewDynamoDBScanner(awsClient.Config, g)
	lambdaScanner := aws.NewLambdaScanner(awsClient.Config, g)

	submitTask := func(taskName string, task func(ctx context.Context) error) {
		scanWg.Add(1)
		engine.Submit(func(ctx context.Context) error {
			defer scanWg.Done()
			
			// Golden Signal B: Scanner Swarm (Latency & Failures)
			tr := otel.Tracer("cloudslash/scanner")
			ctx, span := tr.Start(ctx, taskName, trace.WithAttributes(
				attribute.String("provider", "aws"),
				attribute.String("region", region),
				attribute.String("aws.profile", profile),
			))
			defer span.End()

			err := task(ctx)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				
				// Capture partial failure
				scope := fmt.Sprintf("%s:%s [%s]", profile, region, taskName)
				g.AddError(scope, err)
			}
			return err
		})
	}

	submitTask("ScanInstances", func(ctx context.Context) error { return ec2Scanner.ScanInstances(ctx) })
	submitTask("ScanVolumes", func(ctx context.Context) error { return ec2Scanner.ScanVolumes(ctx) })
	submitTask("ScanNATGateways", func(ctx context.Context) error { return natScanner.ScanNATGateways(ctx) })
	submitTask("ScanAddresses", func(ctx context.Context) error { return eipScanner.ScanAddresses(ctx) })
	submitTask("ScanALBs", func(ctx context.Context) error { return albScanner.ScanALBs(ctx) })
	submitTask("ScanEndpoints", func(ctx context.Context) error { return vpcepScanner.ScanEndpoints(ctx) })
	submitTask("ScanBuckets", func(ctx context.Context) error { return s3Scanner.ScanBuckets(ctx) })
	submitTask("ScanInstances", func(ctx context.Context) error { return rdsScanner.ScanInstances(ctx) })
	submitTask("ScanSnapshots", func(ctx context.Context) error { return ec2Scanner.ScanSnapshots(ctx, "self") })
	submitTask("ScanImages", func(ctx context.Context) error { return ec2Scanner.ScanImages(ctx) })
	submitTask("ScanClusters", func(ctx context.Context) error { return eksScanner.ScanClusters(ctx) })
	submitTask("ScanClusters", func(ctx context.Context) error { return ecsScanner.ScanClusters(ctx) })
	submitTask("ScanClusters", func(ctx context.Context) error { return elasticacheScanner.ScanClusters(ctx) })
	submitTask("ScanClusters", func(ctx context.Context) error { return redshiftScanner.ScanClusters(ctx) })
	submitTask("ScanTables", func(ctx context.Context) error { return dynamoScanner.ScanTables(ctx) })
	submitTask("ScanFunctions", func(ctx context.Context) error { return lambdaScanner.ScanFunctions(ctx) })

	if k8sClient, err := k8s.NewClient(); err == nil {
		k8sScanner := k8s.NewScanner(k8sClient, g)
		submitTask("K8sScan", func(ctx context.Context) error { return k8sScanner.Scan(ctx) })
	}

	return awsClient, nil
}

// performSignalAnalysis analyzes cost trends.
func performSignalAnalysis(g *graph.Graph, slack *notifier.SlackClient, hClient *history.Client) {
	// Snapshot current state.
	s := history.Snapshot{
		Timestamp:      time.Now().Unix(),
		ResourceCounts: make(map[string]int),
	}

	g.Mu.RLock()
	var wasteVector history.Vector
	for _, n := range g.Nodes {
		s.TotalMonthlyCost += n.Cost
		s.ResourceCounts[n.Type]++
		if n.IsWaste {
			s.WasteCount++
			wasteVector = append(wasteVector, n.Cost)
		}
	}
	g.Mu.RUnlock()

	// Persist
	if err := hClient.Append(s); err != nil {
		// Non-critical failure, just log to debug if needed
	}

	// Analyze history window.
	window, err := hClient.LoadWindow(10)
	if err == nil {
		// Analyze with zero budget baseline.
		res := history.Analyze(window, 0)

		// Alert on critical signals.
		if len(res.Alerts) > 0 {
			fmt.Println("\n[ COST ANOMALY DETECTED ]")
			for _, alert := range res.Alerts {
				fmt.Printf(" %s\n", alert)
			}
			fmt.Printf(" Current Velocity: %+.2f $/mo per hour\n", res.Velocity)
			if res.Acceleration > 0 {
				fmt.Printf(" Acceleration:     %+.2f $/mo/h^2 (SPEND ACCELERATING)\n", res.Acceleration)

				// Budget Burn Rate Alert
				if slack != nil && res.Acceleration > 20.0 {
					slack.SendBudgetAlert(res.Velocity, res.Acceleration)
				}
			}
			fmt.Println("-----------------------------------------------------------------")
		} else if res.Velocity != 0 {
			
		}
	}
}

// Helper to execute the CEL policy engine.
func runPolicyEngine(ctx context.Context, rulesFile string, g *graph.Graph) error {
	// Read Rules YAML
	data, err := os.ReadFile(rulesFile)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	type RuleConfig struct {
		Rules []policy.DynamicRule `yaml:"rules"`
	}
	var config RuleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse rules yaml: %w", err)
	}

	// 2. Initialize CEL Engine
	engine, err := policy.NewCELEngine()
	if err != nil {
		return err
	}

	// 3. Compile Rules
	slog.Info("Compiling Rules", "count", len(config.Rules))
	if err := engine.Compile(config.Rules); err != nil {
		return err
	}

	// 4. Evaluate against Graph Nodes (Two-Phase Locking)
	// Phase 1: Analysis (Read-Only) - Allows concurrent readers
	g.Mu.RLock()
	
	type violation struct {
		Node    *graph.Node
		Matches []policy.DynamicRule
	}
	var pendingUpdates []violation
	violations := 0

	for _, node := range g.Nodes {
		// Convert Node properties to Typed Context.
		evalCtx := policy.EvaluationContext{
			ID:    node.ID,
			Kind:  node.Type,
			Cost:  node.Cost,
			Tags:  make(map[string]string),
			Props: node.Properties,
		}

		// Safely extract tags.
		if tags, ok := node.Properties["Tags"].(map[string]string); ok {
			evalCtx.Tags = tags
		}

		// Evaluate CEL rules (Read-Locked).
		// Rules are sorted by priority.
		matches, err := engine.Evaluate(ctx, evalCtx)
		if err != nil {
			continue 
		}

		if len(matches) > 0 {
			pendingUpdates = append(pendingUpdates, violation{Node: node, Matches: matches})
		}
	}
	g.Mu.RUnlock()

	// Phase 2: Commit findings (Write-Locked).
	if len(pendingUpdates) > 0 {
		g.Mu.Lock()
		defer g.Mu.Unlock()

		violations = len(pendingUpdates)
		for _, v := range pendingUpdates {
			// Mark as Waste or Policy Violation
			v.Node.IsWaste = true
			
			// Append Reason (Top Priority Rule wins)
			// But we log all matching policies for audit trail
			v.Node.WasteReason += fmt.Sprintf("[Policy:%s(P%d)] ", v.Matches[0].ID, v.Matches[0].Priority)
			
			if len(v.Matches) > 1 {
				v.Node.WasteReason += fmt.Sprintf("(+%d others)", len(v.Matches)-1)
			}
		}
	}
	
	slog.Info("Policy Scan Complete", "violations", violations)
	return nil
}
