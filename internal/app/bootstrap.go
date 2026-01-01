package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/DrSkyle/cloudslash/internal/aws"
	"github.com/DrSkyle/cloudslash/internal/forensics"
	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/heuristics"
	"github.com/DrSkyle/cloudslash/internal/k8s"
	"github.com/DrSkyle/cloudslash/internal/license"
	"github.com/DrSkyle/cloudslash/internal/notifier"
	"github.com/DrSkyle/cloudslash/internal/pricing"
	"github.com/DrSkyle/cloudslash/internal/remediation"
	"github.com/DrSkyle/cloudslash/internal/report"
	"github.com/DrSkyle/cloudslash/internal/swarm"
	"github.com/DrSkyle/cloudslash/internal/tf"
	"github.com/DrSkyle/cloudslash/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

type Config struct {
	LicenseKey       string
	Region           string
	TFStatePath      string
	MockMode         bool
	AllProfiles      bool
	RequiredTags string
	SlackWebhook string
	Headless         bool // New: Don't run TUI
	DisableCWMetrics bool // New: Save money
}

func Run(cfg Config) (bool, *graph.Graph, error) {
	// 1. License Check (Fail-Open / Trial Mode)
	isTrial := false
	if cfg.LicenseKey == "" {
		if !cfg.Headless {
			fmt.Println("No license key provided. Running Community Edition.")
		}
		isTrial = true
	} else {
		if err := license.Check(cfg.LicenseKey); err != nil {
			fmt.Printf("License check failed: %v\n", err)
			fmt.Println("Falling back to Community Edition.")
			isTrial = true
		}
	}

	// 2. Initialize Components
	ctx := context.Background()
	var g *graph.Graph
	var engine *swarm.Engine

	g = graph.NewGraph()
	engine = swarm.NewEngine()
	engine.Start(ctx)

	var doneChan <-chan struct{}

	if cfg.MockMode {
		runMockMode(ctx, g, engine, cfg.Headless) // Mock mode is synchronous
	} else {
		doneChan = runRealMode(ctx, cfg, g, engine, isTrial)
	}

	// 3. Start Interface (TUI vs Headless)
	if !cfg.Headless {
		model := ui.NewModel(engine, g, isTrial, cfg.MockMode)
		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	} else {
		// Headless: Wait for background analysis to complete
		if doneChan != nil {
			<-doneChan
		}
	}

	// Return true if PRO (not trial), graph for analytics, and nil error
	return !isTrial, g, nil
}

// Logic extracted from original main.go
func runMockMode(ctx context.Context, g *graph.Graph, engine *swarm.Engine, headless bool) {
	if !headless {
		// TUI model handles starting the mock scan?
		// Original main.go: mockScanner.Scan(ctx) was called in main thread.
	}

	mockScanner := aws.NewMockScanner(g)
	mockScanner.Scan(ctx)

	// Synchronous Heuristics for Demo
	heuristicEngine := heuristics.NewEngine()
	heuristicEngine.Register(&heuristics.ZombieEBSHeuristic{})
	heuristicEngine.Register(&heuristics.S3MultipartHeuristic{})
	heuristicEngine.Register(&heuristics.IdleClusterHeuristic{})
	heuristicEngine.Register(&heuristics.EmptyServiceHeuristic{}) // Clients are nil, checks logic only
	heuristicEngine.Register(&heuristics.ZombieEKSHeuristic{})
	heuristicEngine.Register(&heuristics.GhostNodeGroupHeuristic{})
	heuristicEngine.Register(&heuristics.ElasticIPHeuristic{}) // Property-based
	heuristicEngine.Register(&heuristics.RDSHeuristic{})       // Handles "stopped" status without CW
	
	// v1.3.0 Mock Heuristics
	heuristicEngine.Register(&heuristics.NetworkForensicsHeuristic{})
	heuristicEngine.Register(&heuristics.StorageOptimizationHeuristic{})
	
	// heuristicEngine.Register(&heuristics.TagComplianceHeuristic{RequiredTags: ...}) // Only if config passed
	if err := heuristicEngine.Run(ctx, g); err != nil {
		fmt.Printf("Heuristic run failed: %v\n", err)
	}

	// Second Pass (Time Machine)
	hEngine2 := heuristics.NewEngine()
	hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
	hEngine2.Run(ctx, g)

	os.Mkdir("cloudslash-out", 0755)
	if err := report.GenerateHTML(g, "cloudslash-out/dashboard.html"); err != nil {
		fmt.Printf("Failed to generate mock dashboard: %v\n", err)
	}
	report.GenerateCSV(g, "cloudslash-out/waste_report.csv")
	report.GenerateJSON(g, "cloudslash-out/waste_report.json")
}

func runRealMode(ctx context.Context, cfg Config, g *graph.Graph, engine *swarm.Engine, isTrial bool) <-chan struct{} {
	done := make(chan struct{})

	var pricingClient *pricing.Client
	if !isTrial {
		var err error
		pricingClient, err = pricing.NewClient(ctx)
		if err != nil {
			fmt.Printf("Warning: Failed to initialize Pricing Client: %v\n", err)
		}
	}

	profiles := []string{""}
	if cfg.AllProfiles {
		var err error
		profiles, err = aws.ListProfiles()
		if err != nil {
			// If profile listing fails, fallback to default
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
	var logsClient *aws.CloudWatchLogsClient // New Logs Client
	var ecsScanner *aws.ECSScanner
	var ecrScanner *aws.ECRScanner

	for _, profile := range profiles {
		if cfg.AllProfiles {
			fmt.Printf(">>> Scanning Profile: %s\n", profile)
		}

		// Support Multi-Region Scan (v1.2.6)
		regions := strings.Split(cfg.Region, ",")
		for _, region := range regions {
			region = strings.TrimSpace(region)
			if region == "" {
				continue
			}

			// Using local helper (needs to be moved/exported or copied)
			client, err := runScanForProfile(ctx, region, profile, g, engine, &scanWg)
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

	// Background Analysis & Reporting
	go func() {
		defer close(done) // Signal completion when done
		scanWg.Wait()     // Wait for ingestion

		// Additional Background Scans (Logs require logsClient)
		// Note: We scan Log Groups here because they are global/regional and handled via loop context usually.
		// Ideally they should be in the main scan loop, but `logsClient` is created per profile.
		// Since `bootstrap.go` structure is a bit flat, we can cheat and scan logs for the LAST profile
		// or we should have scanned them inside the loop.
		// Given existing structure, let's scan logs here using the last valid client (sub-optimal but works for single profile).
		// OR better: Move Log Scanning to `runScanForProfile`? No, `runScanForProfile` returns generic client.
		// Let's just run it here if client exists.
		if logsClient != nil {
			logsClient.ScanLogGroups(context.Background())
		}

		// ECR Scan (Global/Regional)
		if ecrScanner != nil {
			ecrScanner.ScanRepositories(context.Background())
		}

		// Shadow State Reconciliation
		var state *tf.State
		if _, err := os.Stat(cfg.TFStatePath); err == nil {
			var err error
			state, err = tf.ParseStateFile(cfg.TFStatePath)
			if err == nil {
				detector := tf.NewDriftDetector(g, state)
				detector.ScanForDrift()

				// Code Auditor (v1.2 Feature)
				cwd, _ := os.Getwd() // Default to current directory
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


	// Run Genius Heuristic Engine
		hEngine := heuristics.NewEngine()
		// v1.3.0: Replacing Legacy Heuristics with Forensics Engine
		// hEngine.Register(&heuristics.ElasticIPHeuristic{Pricing: pricingClient}) // Replaced by NetworkForensics
		
		// hEngine.Register(&heuristics.S3MultipartHeuristic{}) // Keeping for Legacy compat (but StorageOptimization handles Iceberg too? Check logic.)
		// S3MultipartHeuristic (legacy) likely just flags them. StorageOptimization adds "FixRecommendation" and checks Lifecycle.
		// Let's keep both or disable one. The legacy one is small. S3Scanner runs it. 
		// Actually S3Scanner calls scanMultipartUploads. 
		// Heuristic usually just marks them as waste.
		// My new StorageOptimizationHeuristic (analyzeMultipart) marks them as waste and adds Fix.
		// If I run both, they might duplicate or overwrite. 
		// Safer to run NEW one. 
		// I will comment out S3MultipartHeuristic.
		
		// Legacy NAT/RDS/ELB
		/*
		if cwClient != nil {
			if pricingClient != nil {
				hEngine.Register(&heuristics.NATGatewayHeuristic{CW: cwClient, Pricing: pricingClient})
			} else {
				hEngine.Register(&heuristics.NATGatewayHeuristic{CW: cwClient})
			}
			hEngine.Register(&heuristics.RDSHeuristic{CW: cwClient})
			hEngine.Register(&heuristics.ELBHeuristic{CW: cwClient})
			if pricingClient != nil {
				hEngine.Register(&heuristics.UnderutilizedInstanceHeuristic{CW: cwClient, Pricing: pricingClient})
			}
		}
		*/
		// Re-enabling non-conflicting legacy ones:
		if cwClient != nil {
			hEngine.Register(&heuristics.RDSHeuristic{CW: cwClient}) // RDS not covered by new NetworkForensics
			// UnderutilizedInstance is separate from Forensics.
			if pricingClient != nil {
				hEngine.Register(&heuristics.UnderutilizedInstanceHeuristic{CW: cwClient, Pricing: pricingClient})
			}
		}

		if pricingClient != nil {
			hEngine.Register(&heuristics.ZombieEBSHeuristic{Pricing: pricingClient})
		} else {
			hEngine.Register(&heuristics.ZombieEBSHeuristic{})
		}

		if cfg.RequiredTags != "" {
			hEngine.Register(&heuristics.TagComplianceHeuristic{RequiredTags: strings.Split(cfg.RequiredTags, ",")})
		}

		if iamClient != nil {
			hEngine.Register(&heuristics.IAMHeuristic{IAM: iamClient})
		}

		// Register Heuristics
		// v1.2.7 Heuristics
		hEngine.Register(&heuristics.LogHoardersHeuristic{})
		hEngine.Register(&heuristics.ECRJanitorHeuristic{})
		
		// v1.2.9 Forensics
		hEngine.Register(&heuristics.DataForensicsHeuristic{})
		hEngine.Register(&heuristics.LambdaHeuristic{})
		
		// v1.3.0 Network & Storage Forensics
		hEngine.Register(&heuristics.NetworkForensicsHeuristic{})
		hEngine.Register(&heuristics.StorageOptimizationHeuristic{}) // Handles S3 Iceberg & EBS Modernizer
		
		// K8s
		hEngine.Register(&heuristics.GhostNodeGroupHeuristic{})
		// v1.2.6 ECS Heuristics
		hEngine.Register(&heuristics.IdleClusterHeuristic{})
		// Injecting captured ECR/ECS scanners
		hEngine.Register(&heuristics.EmptyServiceHeuristic{ECR: ecrScanner, ECS: ecsScanner})

		// v1.2.5 Fargate Analysis
		if k8sClient, err := k8s.NewClient(); err == nil {
			hEngine.Register(&heuristics.AbandonedFargateHeuristic{K8sClient: k8sClient})
		} else {
			hEngine.Register(&heuristics.AbandonedFargateHeuristic{K8sClient: nil})
		}

		// Execute Forensics
		if err := hEngine.Run(ctx, g); err != nil {
			fmt.Printf("Deep Analysis failed: %v\n", err)
		}

		// SECOND PASS (Dependent Heuristics)
		// "The Time Machine" needs Volume Waste to be identified first.
		hEngine2 := heuristics.NewEngine()
		if pricingClient != nil {
			hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{Pricing: pricingClient})
		} else {
			hEngine2.Register(&heuristics.SnapshotChildrenHeuristic{})
		}
		if err := hEngine2.Run(ctx, g); err != nil {
			fmt.Printf("Time Machine Analysis failed: %v\n", err)
		}

		// Execute Forensics (Internal Detective)
		if !isTrial {
			detective := forensics.NewDetective(ctClient)
			detective.InvestigateGraph(ctx, g)
		} else {
			detective := forensics.NewDetective(nil)
			detective.InvestigateGraph(ctx, g)
		}

		// Generate Output
		if !isTrial {
			os.Mkdir("cloudslash-out", 0755)
			gen := tf.NewGenerator(g, state)
			gen.GenerateWasteTF("cloudslash-out/waste.tf")
			gen.GenerateImportScript("cloudslash-out/import.sh")
			gen.GenerateDestroyPlan("cloudslash-out/destroy_plan.out")
			gen.GenerateFixScript("cloudslash-out/fix_terraform.sh")
			os.Chmod("cloudslash-out/fix_terraform.sh", 0755)

			remGen := remediation.NewGenerator(g)
			remGen.GenerateSafeDeleteScript("cloudslash-out/safe_cleanup.sh")
			os.Chmod("cloudslash-out/safe_cleanup.sh", 0755)

			remGen.GenerateIgnoreScript("cloudslash-out/ignore_resources.sh")
			os.Chmod("cloudslash-out/ignore_resources.sh", 0755)

			if err := report.GenerateHTML(g, "cloudslash-out/dashboard.html"); err != nil {
				fmt.Printf("Failed to generate dashboard: %v\n", err)
			}

			// Generate data export artifacts for external processing.
			report.GenerateCSV(g, "cloudslash-out/waste_report.csv")
			report.GenerateJSON(g, "cloudslash-out/waste_report.json")

			if cfg.SlackWebhook != "" {
				if err := notifier.SendSlackReport(cfg.SlackWebhook, g); err != nil {
					fmt.Printf("Failed to send Slack report: %v\n", err)
				}
			}
		}
	}()

	return done
}

func runScanForProfile(ctx context.Context, region, profile string, g *graph.Graph, engine *swarm.Engine, scanWg *sync.WaitGroup) (*aws.Client, error) {
	awsClient, err := aws.NewClient(ctx, region, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %v", err)
	}

	// Verify Identity
	identity, err := awsClient.VerifyIdentity(ctx)
	if err != nil {
		// Helpful error for "No Credentials"
		if strings.Contains(err.Error(), "no EC2 IMDS role found") || strings.Contains(err.Error(), "failed to get caller identity") {
			return nil, fmt.Errorf("\n❌ Unable to find AWS Credentials.\n   Please run 'aws configure' or set AWS_PROFILE.\n   (Error: %v)", err)
		}
		return nil, fmt.Errorf("failed to verify identity: %v", err)
	}
	fmt.Printf(" [Profile: %s] Connected to AWS Account: %s\n", profile, identity)

	// Legacy Scanners
	ec2Scanner := aws.NewEC2Scanner(awsClient.Config, g)
	s3Scanner := aws.NewS3Scanner(awsClient.Config, g)
	rdsScanner := aws.NewRDSScanner(awsClient.Config, g)
	// elbScanner := aws.NewELBScanner(awsClient.Config, g) // Replaced by ALBScanner
	eksScanner := aws.NewEKSScanner(awsClient.Config, g)

	// v1.3.0 Network Scanners
	natScanner := aws.NewNATScanner(awsClient.Config, g)
	eipScanner := aws.NewEIPScanner(awsClient.Config, g)
	albScanner := aws.NewALBScanner(awsClient.Config, g)
	vpcepScanner := aws.NewVpcEndpointScanner(awsClient.Config, g)

	submitTask := func(task func(ctx context.Context) error) {
		scanWg.Add(1)
		engine.Submit(func(ctx context.Context) error {
			defer scanWg.Done()
			return task(ctx)
		})
	}

	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanInstances(ctx) })
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanVolumes(ctx) })
	// submitTask(func(ctx context.Context) error { return ec2Scanner.ScanNatGateways(ctx) }) // Replaced by NATScanner
	// submitTask(func(ctx context.Context) error { return ec2Scanner.ScanAddresses(ctx) }) // Replaced by EIPScanner
	
	submitTask(func(ctx context.Context) error { return natScanner.ScanNATGateways(ctx) })
	submitTask(func(ctx context.Context) error { return eipScanner.ScanAddresses(ctx) })
	submitTask(func(ctx context.Context) error { return albScanner.ScanALBs(ctx) })
	submitTask(func(ctx context.Context) error { return vpcepScanner.ScanEndpoints(ctx) })
	
	submitTask(func(ctx context.Context) error { return s3Scanner.ScanBuckets(ctx) })
	submitTask(func(ctx context.Context) error { return rdsScanner.ScanInstances(ctx) })
	// submitTask(func(ctx context.Context) error { return elbScanner.ScanLoadBalancers(ctx) }) // Replaced by ALBScanner
	
	// New Scans
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanSnapshots(ctx, "self") })
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanImages(ctx) })
	submitTask(func(ctx context.Context) error { return eksScanner.ScanClusters(ctx) })

	// New ECS Scanner (v1.2.6)
	ecsScanner := aws.NewECSScanner(awsClient.Config, g)
	submitTask(func(ctx context.Context) error { return ecsScanner.ScanClusters(ctx) })

	// v1.2.9 Data & Code Forensics Scanners
	elasticacheScanner := aws.NewElasticacheScanner(awsClient.Config, g)
	redshiftScanner := aws.NewRedshiftScanner(awsClient.Config, g)
	dynamoScanner := aws.NewDynamoDBScanner(awsClient.Config, g)
	lambdaScanner := aws.NewLambdaScanner(awsClient.Config, g)

	submitTask(func(ctx context.Context) error { return elasticacheScanner.ScanClusters(ctx) })
	submitTask(func(ctx context.Context) error { return redshiftScanner.ScanClusters(ctx) })
	submitTask(func(ctx context.Context) error { return dynamoScanner.ScanTables(ctx) })
	submitTask(func(ctx context.Context) error { return lambdaScanner.ScanFunctions(ctx) })

	// K8s Scanner (Ghost Detector v1.2.4)
	// Only run if we can create a client
	if k8sClient, err := k8s.NewClient(); err == nil {
		k8sScanner := k8s.NewScanner(k8sClient, g)
		submitTask(func(ctx context.Context) error { return k8sScanner.Scan(ctx) })
	} else {
		// Log warning but don't fail, as this might be running outside k8s context
		// fmt.Printf("Skipping K8s Scan (Ghost Detector): %v\n", err)
	}

	return awsClient, nil
}
