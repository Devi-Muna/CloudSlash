//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TestLazarusProtocol implements the Tombstone Recovery verification.
func TestLazarusProtocol(t *testing.T) {
	cfg := GetAWSConfig(t)
	ec2Client := ec2.NewFromConfig(cfg)

	// 0. Build Binary
	binPath := filepath.Join(t.TempDir(), "cloudslash")
	// Go up two levels to root
	rootDir := "../../" 
	cmd := exec.Command("go", "build", "-o", binPath, "cmd/cloudslash-cli/main.go")
	cmd.Dir = rootDir
	// Inherit env to find go? Yes.
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %s", out)
	}

	// 1. Provision Instance (Spin up t3.micro with Tag Test=Lazarus)
	t.Log("Spinning up infrastructure in LocalStack...")
	instanceID := ProvisionEC2Instance(t, ec2Client, map[string]string{
		"Test": "Lazarus",
	})

	// Verify it is running initially
	if state := GetInstanceState(t, ec2Client, instanceID); state != types.InstanceStateNameRunning {
		t.Fatalf("Instance %s started in %s state, expected running", instanceID, state)
	}

	// 2. Scan & Purge
	outputDir := t.TempDir()
	t.Log("Scanning for waste...")
	
	scanCmd := exec.Command(binPath, "scan", "--headless", "--region", "us-east-1", "--required-tags", "Test=Production", "--output-dir", outputDir)
	// We need to ensure the binary sees the LocalStack env vars.
	// TestMain sets them in this process, so they are inherited.
	// But let's be explicit just in case.
	scanCmd.Env = os.Environ() 

	if out, err := scanCmd.CombinedOutput(); err != nil {
		t.Fatalf("Scan failed: %s", out)
	}

	// 2b. Execute Purge (Run deletion script)
	t.Log("Executing Purge...")
	purgeScript := filepath.Join(outputDir, "resource_deletion.sh")
	if _, err := os.Stat(purgeScript); os.IsNotExist(err) {
		t.Fatalf("Purge script not found at %s", purgeScript)
	}

	// Make the script executable before running it directly
	if err := os.Chmod(purgeScript, 0755); err != nil {
		t.Fatalf("Failed to make purge script executable: %v", err)
	}

	purgeCmd := exec.Command(purgeScript) // Execute the script directly
	purgeCmd.Env = os.Environ() // Inherit AWS env
	if out, err := purgeCmd.CombinedOutput(); err != nil {
		t.Fatalf("Purge execution failed: %s", out)
	}

	// 3. Verify Purge (Check if instance stopped)
	t.Log("Verifying purge...")
	// Wait a moment for state propagation
	time.Sleep(2 * time.Second)
	
	state := GetInstanceState(t, ec2Client, instanceID)
	// CloudSlash default behavior for EC2 is usually STOP (or verify termination if script used terminate).
	// The generated command was `terminate-instances`.
	// LocalStack transitions running -> shutting-down -> terminated.
	if state != types.InstanceStateNameTerminated && state != types.InstanceStateNameShuttingDown {
		t.Fatalf("Instance %s is %s, expected terminated", instanceID, state)
	}

	// 4. Resurrect (Lazarus Protocol)
	t.Log("Resurrecting...")
	restoreScript := filepath.Join(outputDir, "undo_cleanup.sh")
	
	// Ensure script exists
	if _, err := os.Stat(restoreScript); os.IsNotExist(err) {
		t.Fatalf("Undo script not found at %s", restoreScript)
	}

	restoreCmd := exec.Command("bash", restoreScript)
	restoreCmd.Env = os.Environ()
	
	if out, err := restoreCmd.CombinedOutput(); err != nil {
		t.Fatalf("Restoration failed: %s", out)
	}

	// 5. Verify Life
	t.Log("Verifying life...")
	time.Sleep(2 * time.Second) // Give it time to start
	state = GetInstanceState(t, ec2Client, instanceID)
	if state != types.InstanceStateNameRunning {
		t.Fatalf("Instance %s is %s, expected running (resurrected)", instanceID, state)
	}

	t.Log("Lazarus Protocol Verified: Death and Rebirth successful.")
}
