package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/saujanyayaya/cloudslash/internal/aws"
	"github.com/saujanyayaya/cloudslash/internal/graph"
	"github.com/saujanyayaya/cloudslash/internal/heuristics"
	"github.com/saujanyayaya/cloudslash/internal/license"
	"github.com/saujanyayaya/cloudslash/internal/swarm"
	"github.com/saujanyayaya/cloudslash/internal/tf"
	"github.com/saujanyayaya/cloudslash/internal/ui"
)

func main() {
	licenseKey := flag.String("license", "", "License Key")
	region := flag.String("region", "us-east-1", "AWS Region")
	tfStatePath := flag.String("tfstate", "terraform.tfstate", "Path to terraform.tfstate")
	mockMode := flag.Bool("mock", false, "Run in Mock Mode (Simulated Data)")
	flag.Parse()

	// 1. License Check (Fail-Open / Trial Mode)
	isTrial := false
	if *licenseKey == "" {
		fmt.Println("No license key provided. Running in TRIAL MODE.")
		fmt.Println("Resource IDs will be redacted and no output files will be generated.")
		isTrial = true
	} else {
		if err := license.Check(*licenseKey); err != nil {
			fmt.Printf("License check failed: %v\n", err)
			fmt.Println("Falling back to TRIAL MODE.")
			isTrial = true
		}
	}

	// 2. Initialize Components
	ctx := context.Background()

	// awsClient initialization moved to Real AWS Mode block
	// Verify Identity
	// identity, err := awsClient.VerifyIdentity(ctx) // Moved to real mode block
	var g *graph.Graph
	var engine *swarm.Engine
	var cwClient *aws.CloudWatchClient // Declare here, initialize in else block

	g = graph.NewGraph()
	engine = swarm.NewEngine() // Start with default workers
	engine.Start(ctx)

	if *mockMode {
		fmt.Println("Running in MOCK MODE. Simulating AWS environment...")
		mockScanner := aws.NewMockScanner(g)
		// Run scanner synchronously
		mockScanner.Scan(ctx)

		// Run heuristics synchronously for stable demo
		zombieHeuristic := &heuristics.ZombieEBSHeuristic{}
		zombieHeuristic.Analyze(ctx, g)

		s3Heuristic := &heuristics.S3MultipartHeuristic{}
		s3Heuristic.Analyze(ctx, g)
	} else {
		// Real AWS Mode
		awsClient, err := aws.NewClient(ctx, *region)
		if err != nil {
			fmt.Printf("Failed to create AWS client: %v\n", err)
			os.Exit(1)
		}

		// Verify Identity
		identity, err := awsClient.VerifyIdentity(ctx)
		if err != nil {
			fmt.Printf("Failed to verify identity: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Connected to AWS Account: %s\n", identity)

		// Scanners
		ec2Scanner := aws.NewEC2Scanner(awsClient.Config, g)
		s3Scanner := aws.NewS3Scanner(awsClient.Config, g)
		rdsScanner := aws.NewRDSScanner(awsClient.Config, g)
		elbScanner := aws.NewELBScanner(awsClient.Config, g)
		cwClient = aws.NewCloudWatchClient(awsClient.Config)

		// Submit Scan Tasks
		engine.Submit(func(ctx context.Context) error {
			return ec2Scanner.ScanInstances(ctx)
		})
		engine.Submit(func(ctx context.Context) error {
			return ec2Scanner.ScanVolumes(ctx)
		})
		engine.Submit(func(ctx context.Context) error {
			return ec2Scanner.ScanNatGateways(ctx)
		})
		engine.Submit(func(ctx context.Context) error {
			return ec2Scanner.ScanAddresses(ctx)
		})
		engine.Submit(func(ctx context.Context) error {
			return s3Scanner.ScanBuckets(ctx)
		})
		engine.Submit(func(ctx context.Context) error {
			return rdsScanner.ScanInstances(ctx)
		})
		engine.Submit(func(ctx context.Context) error {
			return elbScanner.ScanLoadBalancers(ctx)
		})
	}

	// 3. Start TUI
	model := ui.NewModel(engine, g, isTrial)
	p := tea.NewProgram(model)

	// 4. Run Logic in Background
	go func() {
		if *mockMode {
			return // No background tasks in Mock Mode
		}

		// Wait for scans to complete (simplified wait)
		time.Sleep(5 * time.Second) // In real app, use WaitGroup or better signaling

		// Shadow State Reconciliation
		if _, err := os.Stat(*tfStatePath); err == nil {
			state, err := tf.ParseStateFile(*tfStatePath)
			if err == nil {
				detector := tf.NewDriftDetector(g, state)
				detector.ScanForDrift()
			}
		}

		// Run Heuristics
		// In Mock Mode, we can just run them or skip if they depend on CW
		// For now, let's skip CW-dependent heuristics in Mock Mode or mock CW too.
		// To keep it simple, we'll just let the heuristics run.
		// If they fail (nil CW client), we should handle it.
		// But wait, we didn't init CW client in Mock Mode.
		// Let's just manually mark waste in MockScanner for now to simulate findings.

		// Run Heuristics
		if !*mockMode {
			natHeuristic := &heuristics.NATGatewayHeuristic{CW: cwClient}
			natHeuristic.Analyze(ctx, g)

			zombieHeuristic := &heuristics.ZombieEBSHeuristic{}
			zombieHeuristic.Analyze(ctx, g)

			eipHeuristic := &heuristics.ElasticIPHeuristic{}
			eipHeuristic.Analyze(ctx, g)

			s3Heuristic := &heuristics.S3MultipartHeuristic{}
			s3Heuristic.Analyze(ctx, g)

			rdsHeuristic := &heuristics.RDSHeuristic{CW: cwClient}
			rdsHeuristic.Analyze(ctx, g)

			elbHeuristic := &heuristics.ELBHeuristic{CW: cwClient}
			elbHeuristic.Analyze(ctx, g)
		}

		// Generate Output (Only if not in trial mode)
		if !isTrial {
			os.Mkdir("cloudslash-out", 0755)
			gen := tf.NewGenerator(g)
			gen.GenerateWasteTF("cloudslash-out/waste.tf")
			gen.GenerateImportScript("cloudslash-out/import.sh")
			gen.GenerateDestroyPlan("cloudslash-out/destroy_plan.out")
		} else {
			// In trial mode, we do NOT generate output.
			// This forces the user to buy a license to get the fix.
		}

		// Signal completion to TUI?
		// For now, just let user quit with 'q'
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
