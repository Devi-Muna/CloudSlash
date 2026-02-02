package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/history"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/notifier"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/scanner"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/swarm"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/providers/k8s"
	"gopkg.in/yaml.v3"
)

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
	eLog := slog.Default() // Use default which is set in Engine.Run
	eLog.Info("Connected to AWS", "profile", profile, "account", identity)

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

	// Initialize Registry
	reg := scanner.NewRegistry()

	// Register Scanners
	reg.Register(&aws.EC2InstanceScanner{Scanner: ec2Scanner})
	reg.Register(&aws.EC2VolumeScanner{Scanner: ec2Scanner})
	reg.Register(&aws.NATScannerWrapper{Scanner: natScanner})
	reg.Register(&aws.EIPScannerWrapper{Scanner: eipScanner})
	reg.Register(&aws.ALBScannerWrapper{Scanner: albScanner})
	reg.Register(&aws.VPCEndpointScannerWrapper{Scanner: vpcepScanner})
	reg.Register(&aws.S3ScannerWrapper{Scanner: s3Scanner})
	reg.Register(&aws.RDSScannerWrapper{Scanner: rdsScanner})
	reg.Register(&aws.EC2SnapshotScanner{Scanner: ec2Scanner, OwnerID: "self"})
	reg.Register(&aws.EC2ImageScanner{Scanner: ec2Scanner})
	reg.Register(&aws.EKSScannerWrapper{Scanner: eksScanner})
	reg.Register(&aws.ECSScannerWrapper{Scanner: ecsScanner})
	reg.Register(&aws.ElasticacheScannerWrapper{Scanner: elasticacheScanner})
	reg.Register(&aws.RedshiftScannerWrapper{Scanner: redshiftScanner})
	reg.Register(&aws.DynamoDBScannerWrapper{Scanner: dynamoScanner})
	reg.Register(&aws.LambdaScannerWrapper{Scanner: lambdaScanner})

	if k8sClient, err := k8s.NewClient(); err == nil {
		k8sScanner := k8s.NewScanner(k8sClient, g)
		reg.Register(k8sScanner)
	}

	// Execute All Scanners
	reg.RunAll(ctx, g, engine, scanWg, region, profile)


	return awsClient, nil
}

// performSignalAnalysis detects cost anomalies.
func performSignalAnalysis(g *graph.Graph, slack *notifier.SlackClient, hClient *history.Client) {
	// Snapshot state.
	s := history.Snapshot{
		Timestamp:      time.Now().Unix(),
		ResourceCounts: make(map[string]int),
	}

	g.Mu.RLock()
	var wasteVector history.Vector
	for _, n := range g.GetNodes() {
		s.TotalMonthlyCost += n.Cost
		s.ResourceCounts[n.TypeStr()]++
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

	// Analyze window.
	window, err := hClient.LoadWindow(10)
	if err == nil {
		// Analyze with zero budget.
		res := history.Analyze(window, 0)

		// Alert critical signals.
		if len(res.Alerts) > 0 {
			fmt.Println("\n[ COST ANOMALY DETECTED ]")
			for _, alert := range res.Alerts {
				fmt.Printf(" %s\n", alert)
			}
			fmt.Printf(" Current Velocity: %+.2f $/mo per hour\n", res.Velocity)
			if res.Acceleration > 0 {
				fmt.Printf(" Acceleration:     %+.2f $/mo/h^2 (SPEND ACCELERATING)\n", res.Acceleration)

				// Budget alert.
				if slack != nil && res.Acceleration > 20.0 {
					slack.SendBudgetAlert(res.Velocity, res.Acceleration)
				}
			}
			fmt.Println("-----------------------------------------------------------------")
		} else if res.Velocity != 0 {

		}
	}
}

// runPolicyEngine executes CEL policies.
func runPolicyEngine(ctx context.Context, rulesFile string, g *graph.Graph) error {
	// Read rules.
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

	// Initialize CEL.
	engine, err := policy.NewCELEngine()
	if err != nil {
		return err
	}

	// Compile rules.
	slog.Info("Compiling Rules", "count", len(config.Rules))
	if err := engine.Compile(config.Rules); err != nil {
		return err
	}

	// Evaluate nodes.
	// Phase 1: Read-only analysis.
	g.Mu.RLock()

	type violation struct {
		Node    *graph.Node
		Matches []policy.DynamicRule
	}
	var pendingUpdates []violation
	violations := 0

	for _, node := range g.GetNodes() {
		// Create evaluation context.
		evalCtx := policy.EvaluationContext{
			ID:       node.IDStr(),
			Kind:     node.TypeStr(),
			Cost:     node.Cost,
			Tags:     make(map[string]string),
			Resource: node.TypedData,
		}

		// Safely extract tags.
		if tags, ok := node.Properties["Tags"].(map[string]string); ok {
			evalCtx.Tags = tags
		}

		// Evaluate rules.
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

	// Phase 2: Apply changes.
	if len(pendingUpdates) > 0 {
		g.Mu.Lock()
		defer g.Mu.Unlock()

		violations = len(pendingUpdates)
		for _, v := range pendingUpdates {
			// Mark as waste.
			v.Node.IsWaste = true

			// Append reason.
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
