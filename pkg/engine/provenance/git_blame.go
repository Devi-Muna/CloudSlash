package provenance

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BlameInfo holds line attribution data.
type BlameInfo struct {
	Author  string
	Email   string
	Date    time.Time
	Hash    string
	Message string
}

// execCmd mock injection.
var execCmd = exec.Command

// GetBlame retrieves attribution data.
func GetBlame(filePath string, startLine, endLine int) (*BlameInfo, error) {
	// Execute git blame (porcelain).
	// Target specific line.
	
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
	
	// Parse hash.
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
