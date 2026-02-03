package remediation

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRemediationPlan_Golden(t *testing.T) {
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

	// Run Generator
	tmpDir, err := os.MkdirTemp("", "cloudslash_cov_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	planPath := filepath.Join(tmpDir, "remediation_plan.json")
	gen := NewGenerator(g, nil)
	if err := gen.GenerateRemediationPlan(planPath); err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	contentBytes, _ := os.ReadFile(planPath)

	// Normalize timestamps for stable Golden File comparison
	content := string(contentBytes)
	re := regexp.MustCompile(`"generated_at": ".*"`)
	content = re.ReplaceAllString(content, `"generated_at": "2026-01-01T00:00:00Z"`)
	
	reExpiry := regexp.MustCompile(`"CloudSlash:ExpiryDate": ".*"`)
	content = reExpiry.ReplaceAllString(content, `"CloudSlash:ExpiryDate": "2026-03-04"`)

	// GOLDIE: Snapshot Testing
	golder := goldie.New(t)
	golder.Assert(t, "remediation_plan", []byte(content))

	// Verify no security bypass (manual check still valuable)
	if strings.Contains(string(contentBytes), "i-good") {
		assert.Fail(t, "Non-waste node was processed!")
	}
}

// TestGenerateBashScript_SecurityInjection ensures malicious IDs are escaped.
func TestGenerateBashScript_SecurityInjection(t *testing.T) {
	g := graph.NewGraph()
	// Malicious ID that attempts to close the quote and inject a command
	maliciousID := "i-123'; rm -rf /"
	
	g.AddNode(maliciousID, "AWS::EC2::Instance", map[string]interface{}{})
	g.CloseAndWait()
	g.MarkWaste(maliciousID, 100)
	
	tmpDir, _ := os.MkdirTemp("", "security_test")
	defer os.RemoveAll(tmpDir)
	
	gen := NewGenerator(g, nil)
	
	// Generate the JSON plan first (in-memory simulation)
	plan := TransactionManifest{
		Version: "1.0", 
		Actions: []PlanAction{
			{
				ID: maliciousID, 
				Type: "AWS::EC2::Instance", 
				Operation: "STOP",
				Description: "Injection Test",
				Parameters: map[string]interface{}{
					"Region": "us-east-1",
					"Tags": map[string]string{
						"CloudSlash:ExpiryDate": "2026-01-01",
					},
				},
			},
		},
	}
	
	shPath := filepath.Join(tmpDir, "attack_test.sh")
	err := gen.GenerateBashScript(shPath, plan)
	assert.NoError(t, err)
	
	bytes, _ := os.ReadFile(shPath)
	scriptContent := string(bytes)
	
	// Verify critical safety: The malicious payload must NOT exist without escaping
	// Raw Injection check:
	if strings.Contains(scriptContent, "--instance-ids i-123'; rm -rf /") {
		t.Fatal("VULNERABILITY DETECTED: Payload injected raw into script.")
	}
	
	// Verify it contains the escaped version.
	// Bash strong quoting: 'i-123'\''...
	expectedSnippet := `'i-123'\''`
	if !strings.Contains(scriptContent, expectedSnippet) {
		t.Errorf("Script did not contain expected escaping. Got:\n%s", scriptContent)
	}
}
