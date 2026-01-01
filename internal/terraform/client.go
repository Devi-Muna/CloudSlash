package terraform

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Client wraps the terraform CLI execution.
type Client struct{}

// NewClient creates a new Terraform client.
func NewClient() *Client {
	return &Client{}
}

// IsInstalled checks if the 'terraform' binary is available in the user's PATH.
func (c *Client) IsInstalled() bool {
	_, err := exec.LookPath("terraform")
	return err == nil
}

// PullState executes 'terraform state pull' and returns the raw JSON stdout.
// It relies on the user's existing shell authentication context.
// Safe ingestion: No file reading, no asking for credentials.
func (c *Client) PullState(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "terraform", "state", "pull")
	// We intentionally do not set cmd.Dir, letting it run in the CWD.
	// This assumes the user is running cloudslash from the root of their infra repo.
	
	output, err := cmd.Output()
	if err != nil {
		// If exit code is non-zero, capture stderr in the error if possible
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("terraform state pull failed: %s", string(exitErr.Stderr)) 
		}
		return nil, err
	}

	return output, nil
}
