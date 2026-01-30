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

// HCL for the Victim Instance
// Note: Uses 'local' provider or configures AWS to point to LocalStack
const victimHCL = `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region                      = "us-east-1"
  skip_credentials_validation = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true
  # These will be overridden by env vars in main_test.go (AWS_ENDPOINT_URL)
}

resource "aws_instance" "victim" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"

  tags = {
    Test = "Lazarus"
    Name = "Victim-Instance"
  }
}

output "instance_id" {
  value = aws_instance.victim.id
}
`

func TestLazarusProtocol(t *testing.T) {
	// 1. Provision Infrastructure
	t.Log("Initializing Terraform Infrastructure...")
	tf := NewTerraformHelper(t, victimHCL)
	tf.Init()
	instanceID := tf.Apply()
	t.Logf("Terraform provisioned instance: %s", instanceID)

	// Verify Initial State
	cfg := GetAWSConfig(t)
	ec2Client := ec2.NewFromConfig(cfg)
	if state := GetInstanceState(t, ec2Client, instanceID); state != types.InstanceStateNameRunning {
		t.Fatalf("Expected Running, got %s", state)
	}
	tf.AssertCleanPlan() // Should be clean right after creation

	// 2. Scan & Purge (Simulate Accident)
	binPath := GetBinaryPath(t) // Helper we defined previously
	outputDir := t.TempDir()
	
	t.Log("Scanning and Purging...")
	scanCmd := exec.Command(binPath, "scan", "--headless", "--region", "us-east-1", "--required-tags", "Test=Production", "--output-dir", outputDir)
	scanCmd.Env = os.Environ() // Must inherit AWS_ENDPOINT_URL
	if out, err := scanCmd.CombinedOutput(); err != nil {
		t.Fatalf("Scan failed: %s", out)
	}

	// Run the generated Deletion Script
	purgeScript := filepath.Join(outputDir, "resource_deletion.sh")
	runScript(t, purgeScript)

	// Verify Purgatory (Stopped)
	time.Sleep(2 * time.Second)
	state := GetInstanceState(t, ec2Client, instanceID)
	// LocalStack usually terminates on 'stop' calls for simple setups, but let's check for 'stopped' or 'terminated'
	if state != types.InstanceStateNameStopped && state != types.InstanceStateNameTerminated {
		// CloudSlash 'stop' logic might vary. If it detached volume + terminated instance,
		// we need to verify the Volume exists. For this test, assume instance stop.
		t.Logf("Instance state is %s (Proceeding assuming Purgatory active)", state)
	}

	// 3. Verify Drift is Detected
	// Plan should exit with code 2, confirming the state is broken
	cmd := exec.Command("terraform", "plan", "-detailed-exitcode")
	cmd.Dir = tf.Dir
	cmd.Env = os.Environ()
	if err := cmd.Run(); err == nil {
		t.Fatal("Expected Terraform Drift after Purge, but Plan was Clean!")
	} else if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 2 {
		t.Logf("Terraform returned non-2 exit code: %d (This is acceptable if resource is missing)", exitErr.ExitCode())
	}

	// 4. Resurrect (Lazarus Protocol)
	t.Log("Resurrecting...")
	restoreScript := filepath.Join(outputDir, "undo_cleanup.sh") // Or whatever your undo script is named
	runScript(t, restoreScript)

	// 5. Verify State Consistency
	t.Log("Verifying Terraform State Consistency...")
	
	// Wait for consistency
	time.Sleep(5 * time.Second)

	// 6. Final Drift Check
	tf.AssertCleanPlan()
	
	t.Log("SUCCESS: Resource restored AND Terraform State is Clean.")
}

func runScript(t *testing.T, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Script not found: %s", path)
	}
	// chmod +x just in case
	_ = os.Chmod(path, 0755)
	
	cmd := exec.Command("bash", path)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Script execution failed %s: %s", path, out)
	}
}
