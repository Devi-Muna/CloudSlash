package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// Snapshot represents the state of the infrastructure at a point in time.
type Snapshot struct {
	Timestamp      int64             `json:"timestamp"`       // Unix Epoch
	TotalMonthlyCost float64           `json:"monthly_cost"`    // Total Estimated Burn
	ResourceCounts map[string]int    `json:"resource_counts"` // e.g. "EC2": 10, "NAT": 2
	WasteCount     int               `json:"waste_count"`     // Total number of flagged resources
	Vector         Vector            `json:"-"`               // Computed in memory, not typically stored raw (can be derived)
}

// Append writes a new snapshot to the local ledger.
func Append(s Snapshot) error {
	path, err := GetLedgerPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Serialize to JSON
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	// Append newline
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	return nil
}

// LoadWindow retrieves the last N snapshots from the ledger.
func LoadWindow(n int) ([]Snapshot, error) {
	path, err := GetLedgerPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []Snapshot{}, nil // Empty history is fine
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var history []Snapshot
	scanner := bufio.NewScanner(f)
	
	// Read all (inefficient for years of data, fine for CLI tool v1)
	// Optimization: Seek to end and read backwards? For now, read all is robust.
	for scanner.Scan() {
		var s Snapshot
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue // Skip corrupt lines
		}
		history = append(history, s)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return up to N
	if len(history) > n {
		return history[len(history)-n:], nil
	}
	return history, nil
}

// GetLedgerPath resolves the ~/.cloudslash/ledger.jsonl path.
func GetLedgerPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cloudslash", "ledger.jsonl"), nil
}
