package provenance

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BlameInfo contains the raw git blame data for a specific line.
type BlameInfo struct {
	Author  string
	Email   string
	Date    time.Time
	Hash    string
	Message string
}

// execCmd allows mocking exec.Command for testing
var execCmd = exec.Command

// GetBlame returns the provenance for a given file and line range.
// It attributes the resource to the commit that introduced/modified the key lines.
func GetBlame(filePath string, startLine, endLine int) (*BlameInfo, error) {
	// Execute git blame via porcelain format for parsing reliability.
	// Targets the specific start line of the resource block to identify the author.
	// Assumes the start line represents the definition origin.
	
	args := []string{"blame", "-L", fmt.Sprintf("%d,%d", startLine, startLine), "--porcelain", filePath}
	cmd := execCmd("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	return parsePorcelain(string(output))
}

func parsePorcelain(output string) (*BlameInfo, error) {
	lines := strings.Split(output, "\n")
	info := &BlameInfo{}
	
	// The first line is the hash
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) >= 1 {
			info.Hash = parts[0]
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "author ") {
			info.Author = strings.TrimPrefix(line, "author ")
		}
		if strings.HasPrefix(line, "author-mail ") {
			info.Email = strings.TrimPrefix(line, "author-mail ")
		}
		if strings.HasPrefix(line, "author-time ") {
			tsStr := strings.TrimPrefix(line, "author-time ")
			var ts int64
			fmt.Sscanf(tsStr, "%d", &ts)
			info.Date = time.Unix(ts, 0)
		}
		if strings.HasPrefix(line, "summary ") {
			info.Message = strings.TrimPrefix(line, "summary ")
		}
	}

	return info, nil
}
