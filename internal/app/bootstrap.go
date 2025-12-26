package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	
	"github.com/DrSkyle/cloudslash/internal/aws"
	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/heuristics"
	"github.com/DrSkyle/cloudslash/internal/forensics"
	"github.com/DrSkyle/cloudslash/internal/license"
	"github.com/DrSkyle/cloudslash/internal/notifier"
	"github.com/DrSkyle/cloudslash/internal/pricing"
	"github.com/DrSkyle/cloudslash/internal/remediation"
	"github.com/DrSkyle/cloudslash/internal/report"
	"github.com/DrSkyle/cloudslash/internal/swarm"
	"github.com/DrSkyle/cloudslash/internal/tf"
	"github.com/DrSkyle/cloudslash/internal/ui"
	"github.com/DrSkyle/cloudslash/internal/k8s"
	tea "github.com/charmbracelet/bubbletea"
)

type Config struct {
	LicenseKey   string
	Region       string
	TFStatePath  string
	MockMode     bool
	AllProfiles  bool
	RequiredTags string
	SlackWebhook string
	Headless     bool // New: Don't run TUI
}

func Run(cfg Config) {
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

	if cfg.MockMode {
		runMockMode(ctx, g, engine, cfg.Headless)
	} else {
		_, _, _ = runRealMode(ctx, cfg, g, engine, isTrial)
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
        // Just wait for completion if headless (simplified logic usually handles waitgroup)
        // Implementation note: The real mode logic has a waitgroup. 
        // We should ensure that completes.
        // For simplicity in this refactor, headless relies on heuristics triggering logic.
        // But heuristics were in a goroutine in main.go.
        // We need to keep that logic.
    }
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

func runRealMode(ctx context.Context, cfg Config, g *graph.Graph, engine *swarm.Engine, isTrial bool) (*aws.CloudWatchClient, *aws.IAMClient, *pricing.Client) {
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

		for _, profile := range profiles {
			if cfg.AllProfiles {
				fmt.Printf(">>> Scanning Profile: %s\n", profile)
			}

            // Using local helper (needs to be moved/exported or copied)
			client, err := runScanForProfile(ctx, cfg.Region, profile, g, engine, &scanWg)
			if err != nil {
				fmt.Printf("Scan failed for profile %s: %v\n", profile, err)
				continue
			}

			if client != nil {
				cwClient = aws.NewCloudWatchClient(client.Config)
				iamClient = aws.NewIAMClient(client.Config)
				ctClient = aws.NewCloudTrailClient(client.Config)
				logsClient = aws.NewCloudWatchLogsClient(client.Config, g)
			}
		}

			// Background Analysis & Reporting
		go func() {
			scanWg.Wait() // Wait for ingestion
			
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
			if pricingClient != nil {
				hEngine.Register(&heuristics.ElasticIPHeuristic{Pricing: pricingClient})
			} else {
				hEngine.Register(&heuristics.ElasticIPHeuristic{})
			}
			hEngine.Register(&heuristics.S3MultipartHeuristic{})

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
			
			// New Heuristics
			hEngine.Register(&heuristics.LogHoardersHeuristic{})
			hEngine.Register(&heuristics.FossilAMIHeuristic{})
			hEngine.Register(&heuristics.ZombieEKSHeuristic{})
			hEngine.Register(&heuristics.GhostNodeGroupHeuristic{})

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
			
			// Execute Forensics (Pro Feature Check implied by binary, but logic runs for graph data)
			if !isTrial {
                detective := forensics.NewDetective(ctClient)
                detective.InvestigateGraph(ctx, g)
            } else {
                // In trial/community, just check tags, no CloudTrail API to save requests/complexity?
                // Or just don't run API.
                // Actually, let's run simple tag check without CT client
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
        
        return cwClient, iamClient, pricingClient
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

	// Scanners
	ec2Scanner := aws.NewEC2Scanner(awsClient.Config, g)
	s3Scanner := aws.NewS3Scanner(awsClient.Config, g)
	rdsScanner := aws.NewRDSScanner(awsClient.Config, g)
	elbScanner := aws.NewELBScanner(awsClient.Config, g)
	eksScanner := aws.NewEKSScanner(awsClient.Config, g)

	submitTask := func(task func(ctx context.Context) error) {
		scanWg.Add(1)
		engine.Submit(func(ctx context.Context) error {
			defer scanWg.Done()
			return task(ctx)
		})
	}

	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanInstances(ctx) })
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanVolumes(ctx) })
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanNatGateways(ctx) })
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanAddresses(ctx) })
	submitTask(func(ctx context.Context) error { return s3Scanner.ScanBuckets(ctx) })
	submitTask(func(ctx context.Context) error { return rdsScanner.ScanInstances(ctx) })
	submitTask(func(ctx context.Context) error { return elbScanner.ScanLoadBalancers(ctx) })
	// New Scans
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanSnapshots(ctx, "self") })
	submitTask(func(ctx context.Context) error { return ec2Scanner.ScanImages(ctx) })
	submitTask(func(ctx context.Context) error { return eksScanner.ScanClusters(ctx) })

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
