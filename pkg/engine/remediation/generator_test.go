package remediation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

func TestGenerateSafeDeleteScript_Security(t *testing.T) {
	// Setup Graph with Malicious Node
	g := graph.NewGraph()
	maliciousID := "i-1234567890abcdef0; rm -rf /" // Shell injection attempt
	validID := "i-0a1b2c3d4e5f6g7h8"

	g.Nodes = append(g.Nodes, &graph.Node{
		ID:      maliciousID,
		Type:    "AWS::EC2::Instance",
		IsWaste: true,
	})
	g.Nodes = append(g.Nodes, &graph.Node{
		ID:      validID,
		Type:    "AWS::EC2::Instance",
		IsWaste: true,
	})

	// Run Generator
	tmpDir, err := os.MkdirTemp("", "cloudslash_security_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "safe_cleanup.sh")
	gen := NewGenerator(g)
	if err := gen.GenerateSafeDeleteScript(scriptPath); err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	// Verify Output
	contentBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(contentBytes)

	// Check if malicious ID was skipped/commented
	if strings.Contains(content, "aws ec2 stop-instances --instance-ids i-1234567890abcdef0; rm -rf /") {
		t.Fatal("SECURITY FAILURE: Malicious ID was injected into command!")
	}

	if !strings.Contains(content, "# SKIPPING MALFORMED ID (Potential Injection): i-1234567890abcdef0; rm -rf /") {
		t.Error("Expected warning comment for malicious ID, found none")
	}

	// Check if valid ID was processed
	if !strings.Contains(content, "aws ec2 stop-instances --instance-ids i-0a1b2c3d4e5f6g7h8") {
		t.Error("Valid ID was incorrectly blocked")
	}
}
