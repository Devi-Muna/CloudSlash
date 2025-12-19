package remediation

import (
	"os"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

func TestGenerateIgnoreScript(t *testing.T) {
	// Setup Graph
	g := graph.NewGraph()
	g.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0deadbeef", "AWS::EC2::Volume", nil)
	g.AddNode("arn:aws:ec2:us-east-1:123456789012:instance/i-0cafebabe", "AWS::EC2::Instance", nil)
	
	// Mark as waste
	g.MarkWaste("arn:aws:ec2:us-east-1:123456789012:volume/vol-0deadbeef", 80) // Standard Waste
	
	// Create Justified Node (Simulate MarkWaste logic manually or use Helper if easier, but helper requires tags)
	// Let's use MarkWaste with tags to simulate full flow
	g.AddNode("arn:justified", "AWS::EC2::Instance", map[string]interface{}{
		"Tags": map[string]string{"cloudslash:ignore": "justified:test"},
	})
	g.MarkWaste("arn:justified", 50)
	
	g.MarkWaste("arn:aws:ec2:us-east-1:123456789012:instance/i-0cafebabe", 50)

	// Generate Script
	gen := NewGenerator(g)
	tmpFile := "test_ignore_script.sh"
	defer os.Remove(tmpFile)

	err := gen.GenerateIgnoreScript(tmpFile)
	if err != nil {
		t.Fatalf("GenerateIgnoreScript failed: %v", err)
	}

	// Read and Verify Content
	contentBytes, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read generated script: %v", err)
	}
	content := string(contentBytes)

	// Check Header
	if !strings.Contains(content, "# CloudSlash Ignore Tagging Script") {
		t.Errorf("Script missing header")
	}

	// Check Logic (Deterministic Sort check implicitly via content check)
	expectedVol := "aws resourcegroupstaggingapi tag-resources --resource-arn-list arn:aws:ec2:us-east-1:123456789012:volume/vol-0deadbeef --tags cloudslash:ignore=true"
	expectedInst := "aws resourcegroupstaggingapi tag-resources --resource-arn-list arn:aws:ec2:us-east-1:123456789012:instance/i-0cafebabe --tags cloudslash:ignore=true"

	if !strings.Contains(content, expectedVol) {
		t.Errorf("Script missing volume tagging command")
	}
	if !strings.Contains(content, expectedInst) {
		t.Errorf("Script missing instance tagging command")
	}
	
	// Check Exclusion
	if strings.Contains(content, "arn:justified") {
		t.Errorf("Script SHOULD NOT include justified node")
	}
}
