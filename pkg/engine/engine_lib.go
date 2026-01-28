package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/engine/aws"
	internalconfig "github.com/DrSkyle/cloudslash/pkg/config"
	"github.com/DrSkyle/cloudslash/pkg/engine/forensics"
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
	"github.com/DrSkyle/cloudslash/pkg/version"
)

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
	Heuristics       internalconfig.HeuristicConfig
}

func Run(cfg Config) (bool, *graph.Graph, *swarm.Engine, error) {

	if !cfg.Headless {
		fmt.Printf("%s %s [%s]\n", version.AppName, version.Current, version.License)
	}

	// Recover from panics.
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("\n[CRITICAL FAILURE]")
			fmt.Printf("Error: %v\n", r)
			
			// Log stack trace
			crashFile := fmt.Sprintf("crash_log_%d.txt", time.Now().Unix())
			f, _ := os.Create(crashFile)
			defer f.Close()
			fmt.Fprintf(f, "Crash Time: %s\nError: %v\n", time.Now(), r)
			
			fmt.Printf("Details saved to %s\n", crashFile)
			fmt.Println("Please report this issue.")
			os.Exit(1)
		}
	}()

	// Initialize headless mode context.

	ctx := context.Background()
	var g *graph.Graph
	var engine *swarm.Engine

	g = graph.NewGraph()
	engine = swarm.NewEngine()
	engine.Start(ctx)

	var doneChan <-chan struct{}

	if cfg.MockMode {
		runMockMode(ctx, cfg, g, engine)
	} else {
		doneChan = runRealMode(ctx, cfg, g, engine)
	}

	// Wait for scan to complete in non-headless mode (if applicable) or return.
	if doneChan != nil {
		<-doneChan
	}

	return true, g, engine, nil
}

func runMockMode(ctx context.Context, cfg Config, g *graph.Graph, engine *swarm.Engine) {
	mockScanner := aws.NewMockScanner(g)

	// Seed mock history data.
	history.SeedMockData()

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

	hEngine2 := heuristics.NewEngine()
	hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
	hEngine2.Run(ctx, g)

	os.Mkdir("cloudslash-out", 0755)

	// Output Generation
	report.GenerateCSV(g, "cloudslash-out/waste_report.csv")
	report.GenerateJSON(g, "cloudslash-out/waste_report.json")

	// Dashboard Generation
	if err := report.GenerateDashboard(g, "cloudslash-out/dashboard.html"); err != nil {
		fmt.Printf("Failed to generate dashboard: %v\n", err)
	}

	// Remediation Scripts
	gen := tf.NewGenerator(g, nil)
	gen.GenerateFixScript("cloudslash-out/fix_terraform.sh")
	os.Chmod("cloudslash-out/fix_terraform.sh", 0755)

	// Generate mock artifacts.
	gen.GenerateWasteTF("cloudslash-out/waste.tf")
	gen.GenerateImportScript("cloudslash-out/import.sh")
	gen.GenerateDestroyPlan("cloudslash-out/destroy_plan.out")

	remGen := remediation.NewGenerator(g)
	remGen.GenerateSafeDeleteScript("cloudslash-out/safe_cleanup.sh")
	os.Chmod("cloudslash-out/safe_cleanup.sh", 0755)

	remGen.GenerateIgnoreScript("cloudslash-out/ignore_resources.sh")
	os.Chmod("cloudslash-out/ignore_resources.sh", 0755)

	// Executive Summary
	report.GenerateExecutiveSummary(g, "cloudslash-out/executive_summary.md", fmt.Sprintf("cs-mock-%d", time.Now().Unix()), "MOCK-ACCOUNT-123")

	var slackClient *notifier.SlackClient
	if cfg.SlackWebhook != "" && cfg.Headless {
		fmt.Println(" -> Transmitting Cost Report to Slack (MOCK)...")
		slackClient = notifier.NewSlackClient(cfg.SlackWebhook, cfg.SlackChannel)
		
		// Recalculate summary for Slack
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
		
		slackClient.SendAnalysisReport(summary)
	}

	// Historical Analysis
	performSignalAnalysis(g, slackClient)
}

func runRealMode(ctx context.Context, cfg Config, g *graph.Graph, engine *swarm.Engine) <-chan struct{} {
	done := make(chan struct{})

	var pricingClient *pricing.Client
	var err error
	pricingClient, err = pricing.NewClient(ctx)
	if err != nil {
		// Log pricing client initialization error.
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

		if logsClient != nil {
			logsClient.ScanLogGroups(context.Background())
		}

		if ecrScanner != nil {
			ecrScanner.ScanRepositories(context.Background())
		}

		// Reconcile with Terraform state.
		var state *tf.State
		if _, err := os.Stat(cfg.TFStatePath); err == nil {
			var err error
			state, err = tf.ParseStateFile(cfg.TFStatePath)
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

		detective := forensics.NewDetective(ctClient)
		detective.InvestigateGraph(ctx, g)

		// Generate output files.
		os.Mkdir("cloudslash-out", 0755)

		// 1. Report Generation
		report.GenerateCSV(g, "cloudslash-out/waste_report.csv")
		report.GenerateJSON(g, "cloudslash-out/waste_report.json")

		// 2. Remediation & Planning
		gen := tf.NewGenerator(g, state)
		gen.GenerateWasteTF("cloudslash-out/waste.tf")
		gen.GenerateImportScript("cloudslash-out/import.sh")
		gen.GenerateDestroyPlan("cloudslash-out/destroy_plan.out")

		// Remediation Scripts
		gen.GenerateFixScript("cloudslash-out/fix_terraform.sh")
		os.Chmod("cloudslash-out/fix_terraform.sh", 0755)

		remGen := remediation.NewGenerator(g)
		remGen.GenerateSafeDeleteScript("cloudslash-out/safe_cleanup.sh")
		os.Chmod("cloudslash-out/safe_cleanup.sh", 0755)

		remGen.GenerateIgnoreScript("cloudslash-out/ignore_resources.sh")
		os.Chmod("cloudslash-out/ignore_resources.sh", 0755)

		if err := report.GenerateDashboard(g, "cloudslash-out/dashboard.html"); err != nil {
			fmt.Printf("Failed to generate dashboard: %v\n", err)
		}

		// Executive Summary
		report.GenerateExecutiveSummary(g, "cloudslash-out/executive_summary.md", fmt.Sprintf("cs-scan-%d", time.Now().Unix()), "AWS-ACCOUNT")


		if cfg.SlackWebhook != "" && cfg.Headless {
			fmt.Println(" -> Transmitting Cost Report to Slack...")
			client := notifier.NewSlackClient(cfg.SlackWebhook, cfg.SlackChannel)

			// Recalculate summary for Slack
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
		performSignalAnalysis(g, slackClient)

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
	fmt.Printf(" [Profile: %s] Connected to AWS Account: %s\n", profile, identity)

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
			err := task(ctx)
			if err != nil {
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
func performSignalAnalysis(g *graph.Graph, slack *notifier.SlackClient) {
	// 1. Create state snapshot.
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

	// 2. Persist
	if err := history.Append(s); err != nil {
		// Non-critical failure, just log to debug if needed
	}

	// 3. Analyze history window.
	window, err := history.LoadWindow(10)
	if err == nil {
		// Analyze with zero budget baseline.
		res := history.Analyze(window, 0)

		// 4. Log alerts.
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
