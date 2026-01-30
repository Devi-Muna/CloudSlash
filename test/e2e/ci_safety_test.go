//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCISafety ensures CloudSlash behaves correctly in hostile CI environments (missing tokens).
func TestCISafety(t *testing.T) {
	// 0. Build Binary (Reuse shared build if possible, but for isolation let's build)
	binPath := filepath.Join(t.TempDir(), "cloudslash")
	rootDir := "../../" 
	buildCmd := exec.Command("go", "build", "-o", binPath, "cmd/cloudslash-cli/main.go")
	buildCmd.Dir = rootDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %s", out)
	}

	// 1. Setup Hostile Environment
	outputDir := t.TempDir()
	
	// Command: ./bin/cloudslash scan --mock --headless --no-metrics --output-dir ...
	scanCmd := exec.Command(binPath, "scan", "--mock", "--headless", "--no-metrics", "--output-dir", outputDir)
	
	// Explicitly construct environment to simulate "Hostile CI"
	// We want GITHUB_ACTIONS=true, but NO GITHUB_TOKEN
	env := os.Environ()
	var newEnv []string
	for _, e := range env {
		// Filter out GITHUB_TOKEN if present
		if len(e) >= 12 && e[:12] == "GITHUB_TOKEN" {
			continue
		}
		newEnv = append(newEnv, e)
	}
	newEnv = append(newEnv, "GITHUB_ACTIONS=true")
	// Ensure mocked CI doesn't try actual AWS calls if we use --mock flag, 
	// but purely relying on flags.
	
	scanCmd.Env = newEnv

	// 2. Execute
	out, err := scanCmd.CombinedOutput()
	
	// 3. Assert 1: Exit Code 0 (Did not crash)
	if err != nil {
		t.Fatalf("CloudSlash crashed in hostile CI environment: %v\nOutput: %s", err, out)
	}

	// 4. Assert 2: Report Generated
	reportPath := filepath.Join(outputDir, "waste_report.json")
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Fatalf("Report not generated in hostile CI environment")
	}

	t.Log("CI Safety Test Passed: Tool survived without AUTH tokens.")
}
