package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// Snapshot represents a point-in-time state.
type Snapshot struct {
	Timestamp      int64             `json:"timestamp"`       // Unix Epoch
	TotalMonthlyCost float64           `json:"monthly_cost"`    // Monthly cost estimate.
	ResourceCounts map[string]int    `json:"resource_counts"` // Resource counts by type.
	WasteCount     int               `json:"waste_count"`     // Total number of flagged resources
	Vector         Vector            `json:"-"`               // Derived state vector.
}

// Append adds a snapshot to the ledger.
func Append(s Snapshot) error {
	path, err := GetLedgerPath()
	if err != nil {
		return err
	}

	// Create directory.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Marshal to JSON.
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	// Append newline.
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	return nil
}

// LoadWindow returns recent snapshots.
func LoadWindow(n int) ([]Snapshot, error) {
	path, err := GetLedgerPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []Snapshot{}, nil // Return empty if file missing.
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var history []Snapshot
	scanner := bufio.NewScanner(f)
	
	// Read all snapshots.
	
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

	// Return last N items.
	if len(history) > n {
		return history[len(history)-n:], nil
	}
	return history, nil
}

// GetLedgerPath returns the ledger file path.
func GetLedgerPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cloudslash", "ledger.jsonl"), nil
}
