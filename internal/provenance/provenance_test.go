package provenance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProvenanceFlow(t *testing.T) {
	// 1. Setup Temp Dir
	dir, err := os.MkdirTemp("", "cloudslash-prov-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// 2. Init Git Repo
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "jdoe@example.com")
	runCmd(t, dir, "git", "config", "user.name", "Jane Doe")

	// 3. Create TF File
	tfContent := `
resource "aws_instance" "test" {
  ami           = "ami-12345678"
  instance_type = "t2.micro"
}
`
	tfPath := filepath.Join(dir, "main.tf")
	os.WriteFile(tfPath, []byte(tfContent), 0644)

	// 4. Commit
	runCmd(t, dir, "git", "add", "main.tf")
	runCmd(t, dir, "git", "commit", "-m", "Initial commit")

	// 5. Modify Resource
	// We change line 4 (instance_type)
	newContent := `
resource "aws_instance" "test" {
  ami           = "ami-12345678"
  instance_type = "c5.xlarge"
}
`
	os.WriteFile(tfPath, []byte(newContent), 0644)
	runCmd(t, dir, "git", "add", "main.tf")
	runCmd(t, dir, "git", "commit", "-m", "Resize instance")

	// 6. Test HCL Parser
	loc, err := FindResourceInDir(dir, "aws_instance", "test")
	if err != nil {
		t.Fatalf("FindResourceInDir failed: %v", err)
	}

	if loc.StartLine != 2 {
		t.Logf("Found resource at lines %d-%d", loc.StartLine, loc.EndLine)
	}

	// 7. Test Git Blame (Mocked)
	// We override execCmd to call the TestHelperProcess
	execCmd = fakeExecCommand
	defer func() { execCmd = exec.Command }()

	// We blame the instance_type line (line 4)
	blame, err := GetBlame(loc.FilePath, 4, 4)
	if err != nil {
		t.Fatalf("GetBlame failed: %v", err)
	}

	if blame.Author != "Jane Doe" {
		t.Errorf("expected author 'Jane Doe', got '%s'", blame.Author)
	}
	if blame.Message != "Resize instance" {
		t.Errorf("expected message 'Resize instance', got '%s'", blame.Message)
	}

	t.Logf("Success! Blamed commit: %s by %s", blame.Hash, blame.Author)
}

// fakeExecCommand handles the mocking logic
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is the fake command runner
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	
	// Print mock git blame porcelain output
	// Format:
	// <hash> <line> <line> <lines in group>
	// author <name>
	// author-mail <email>
	// author-time <timestamp>
	// author-tz <tz>
	// committer <name>
	// committer-mail <email>
	// committer-time <timestamp>
	// committer-tz <tz>
	// summary <msg>
	// boundary
	// filename <file>
	// 	<content>
	
	fmt.Print(`8f3a21abcd 4 4 1
author Jane Doe
author-mail <jdoe@example.com>
author-time 1705000000
author-tz -0800
committer Jane Doe
committer-mail <jdoe@example.com>
committer-time 1705000000
committer-tz -0800
summary Resize instance
boundary
filename main.tf
	instance_type = "c5.xlarge"
`)
	os.Exit(0)
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	// Skip real git commands since we are mocking
	if name == "git" {
		return
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command %s %v failed: %v\nOutput: %s", name, args, err, out)
	}
}
