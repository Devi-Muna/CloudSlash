package terraform

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Client executes Terraform commands.
type Client struct{}

// NewClient creates a new Terraform client.
func NewClient() *Client {
	return &Client{}
}

// IsInstalled checks for Terraform binary.
func (c *Client) IsInstalled() bool {
	_, err := exec.LookPath("terraform")
	return err == nil
}

// PullState retrieves Terraform state via CLI.
//
//
func (c *Client) PullState(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "terraform", "state", "pull")
	// Execute in current directory.
	
	
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
