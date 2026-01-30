//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TerraformHelper manages the lifecycle of TF resources for testing
type TerraformHelper struct {
	Dir string
	T   *testing.T
}

// NewTerraformHelper creates a temp directory with your HCL
func NewTerraformHelper(t *testing.T, hcl string) *TerraformHelper {
	dir := t.TempDir()
	
	err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(hcl), 0644)
	if err != nil {
		t.Fatalf("Failed to write main.tf: %v", err)
	}

	return &TerraformHelper{Dir: dir, T: t}
}

// Init runs 'terraform init'
func (h *TerraformHelper) Init() {
	cmd := exec.Command("terraform", "init")
	cmd.Dir = h.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		h.T.Fatalf("Terraform Init failed: %s", out)
	}
}

// Apply runs 'terraform apply' and returns the Instance ID
func (h *TerraformHelper) Apply() string {
	cmd := exec.Command("terraform", "apply", "-auto-approve")
	cmd.Dir = h.Dir
	// Inherit env vars for AWS config
	cmd.Env = os.Environ() 
	
	if out, err := cmd.CombinedOutput(); err != nil {
		h.T.Fatalf("Terraform Apply failed: %s", out)
	}

	// Extract Instance ID
	cmdOut := exec.Command("terraform", "output", "-raw", "instance_id")
	cmdOut.Dir = h.Dir
	cmdOut.Env = os.Environ()
	out, err := cmdOut.CombinedOutput()
	if err != nil {
		h.T.Fatalf("Failed to get instance_id output: %s", out)
	}
	return strings.TrimSpace(string(out))
}

// AssertCleanPlan runs 'terraform plan -detailed-exitcode'
// Returns: true if Clean (Exit 0), false if Drift (Exit 2)
func (h *TerraformHelper) AssertCleanPlan() {
	cmd := exec.Command("terraform", "plan", "-detailed-exitcode")
	cmd.Dir = h.Dir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err == nil {
		// Exit Code 0 = Clean
		h.T.Log("Terraform Plan is Clean (Success)")
		return
	}

	// Check for Exit Code 2 (Drift)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 2 {
			h.T.Fatalf("Terraform Drift Detected!\n%s", out)
		}
	}
	
	h.T.Fatalf("Terraform Plan failed with error: %v\nOutput: %s", err, out)
}
