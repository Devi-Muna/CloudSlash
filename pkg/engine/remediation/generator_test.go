package remediation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

func TestGenerateSafeDeleteScript_Coverage(t *testing.T) {
	g := graph.NewGraph()

	// 1. EC2 Instance
	g.AddNode("i-inst1", "AWS::EC2::Instance", map[string]interface{}{})
	// 2. EC2 Volume (Standard)
	g.AddNode("vol-del", "AWS::EC2::Volume", map[string]interface{}{})
	// 3. EC2 Volume (GP2 Modernization)
	g.AddNode("vol-gp2", "AWS::EC2::Volume", map[string]interface{}{"IsGP2": true})
	// 4. RDS
	g.AddNode("db-main", "AWS::RDS::DBInstance", map[string]interface{}{})
	// 5. NAT Gateway
	g.AddNode("nat-123", "AWS::EC2::NatGateway", map[string]interface{}{})
	// 6. EIP
	g.AddNode("eipalloc-1", "AWS::EC2::EIP", map[string]interface{}{})
	// 7. AMI
	g.AddNode("ami-old", "AWS::EC2::AMI", map[string]interface{}{})
	// 8. ELB
	lbARN := "arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/app/my-lb/123"
	g.AddNode(lbARN, "AWS::ElasticLoadBalancingV2::LoadBalancer", map[string]interface{}{})
	// 9. ECS Cluster
	clusterARN := "arn:aws:ecs:us-east-1:123:cluster/MyCluster"
	g.AddNode(clusterARN, "AWS::ECS::Cluster", map[string]interface{}{})
	// 10. ECS Service
	svcARN := "arn:aws:ecs:us-east-1:123:service/MyCluster/MyService"
	g.AddNode(svcARN, "AWS::ECS::Service", map[string]interface{}{})
	// 11. EKS Cluster
	g.AddNode("MyEKSCluster", "AWS::EKS::Cluster", map[string]interface{}{})
	// 12. EKS NodeGroup
	g.AddNode("ng-1", "AWS::EKS::NodeGroup", map[string]interface{}{"ClusterName": "MyEKSCluster"})
	// 13. ECR Repo
	g.AddNode("my-repo", "AWS::ECR::Repository", map[string]interface{}{})
	// 14. Lambda
	g.AddNode("my-func", "AWS::Lambda::Function", map[string]interface{}{})
	// 15. Log Group
	g.AddNode("/aws/lambda/logs", "AWS::Logs::LogGroup", map[string]interface{}{})
	
	// 16. Non-Waste (Should be skipped)
	g.AddNode("i-good", "AWS::EC2::Instance", map[string]interface{}{})

	// 17. Malformed ID (Security Check)
	g.AddNode("i-bad; rm -rf /", "AWS::EC2::Instance", map[string]interface{}{})

	g.CloseAndWait()

	// Mark waste (since AddNode doesn't set IsWaste via props easily in the ingestion flow)
	g.MarkWaste("i-inst1", 90)
	g.MarkWaste("vol-del", 90)
	g.MarkWaste("vol-gp2", 90)
	g.MarkWaste("db-main", 90)
	g.MarkWaste("nat-123", 90)
	g.MarkWaste("eipalloc-1", 90)
	g.MarkWaste("ami-old", 90)
	g.MarkWaste(lbARN, 90)
	g.MarkWaste(clusterARN, 90)
	g.MarkWaste(svcARN, 90)
	g.MarkWaste("MyEKSCluster", 90)
	g.MarkWaste("ng-1", 90)
	g.MarkWaste("my-repo", 90)
	g.MarkWaste("my-func", 90)
	g.MarkWaste("/aws/lambda/logs", 90)
	g.MarkWaste("i-bad; rm -rf /", 90)

	// 17. Malformed ID (Security Check)
	g.Nodes = append(g.Nodes, &graph.Node{
		ID: "i-bad; rm -rf /", Type: "AWS::EC2::Instance", IsWaste: true,
	})

	// Run Generator
	tmpDir, err := os.MkdirTemp("", "cloudslash_cov_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "safe_cleanup.sh")
	gen := NewGenerator(g)
	if err := gen.GenerateSafeDeleteScript(scriptPath); err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	contentBytes, _ := os.ReadFile(scriptPath)
	content := string(contentBytes)

	// Define expectations (with Quoting!)
	checks := []string{
		"aws ec2 stop-instances --instance-ids 'i-inst1'",
		"aws ec2 delete-volume --volume-id 'vol-del'",
		"aws ec2 modify-volume --volume-id 'vol-gp2' --volume-type gp3",
		"aws rds stop-db-instance --db-instance-identifier 'db-main'",
		"aws ec2 delete-nat-gateway --nat-gateway-id 'nat-123'",
		"aws ec2 release-address --allocation-id 'eipalloc-1'",
		"aws ec2 deregister-image --image-id 'ami-old'",
		"aws elbv2 delete-load-balancer --load-balancer-arn 'arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/app/my-lb/123'",
		"aws ecs delete-cluster --cluster 'MyCluster'",
		"aws ecs delete-service --cluster 'MyCluster' --service 'MyService' --force",
		"aws eks delete-cluster --name 'MyEKSCluster'",
		"aws eks delete-nodegroup --cluster-name 'MyEKSCluster' --nodegroup-name 'ng-1'",
		"aws ecr delete-repository --repository-name 'my-repo' --force",
		"aws lambda delete-function --function-name 'my-func'",
		"aws logs delete-log-group --log-group-name '/aws/lambda/logs'",
		"# SKIPPING MALFORMED ID",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("Missing expected command: %s", check)
		}
	}

	if strings.Contains(content, "i-good") {
		t.Error("Non-waste node was processed!")
	}
}

func TestShellEscape(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"has space", "'has space'"},
		{"has'quote", "'has'\\''quote'"},
		{"", "''"},
		{"danger; rm -rf /", "'danger; rm -rf /'"},
	}

	for _, tc := range cases {
		got := shellEscape(tc.input)
		if got != tc.expected {
			t.Errorf("shellEscape(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
