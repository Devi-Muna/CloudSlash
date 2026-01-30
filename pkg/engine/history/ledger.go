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
	TotalMonthlyCost float64           `json:"monthly_cost"`    // Estimated monthly cost.
	ResourceCounts map[string]int    `json:"resource_counts"` // Resource counts by type.
	WasteCount     int               `json:"waste_count"`     // Total number of flagged resources
	Vector         Vector            `json:"-"`               // Derived state vector.
}

// Backend defines the storage interface for snapshots.
type Backend interface {
	Append(s Snapshot) error
	Load(n int) ([]Snapshot, error)
}

// Client manages historical state.
type Client struct {
	backend Backend
}

// NewClient initializes a history client.
// Defaults to FileBackend.
func NewClient(backend Backend) *Client {
	if backend == nil {
		backend = &FileBackend{}
	}
	return &Client{
		backend: backend,
	}
}

// Append records a new snapshot.
func (c *Client) Append(s Snapshot) error {
	return c.backend.Append(s)
}

// LoadWindow retrieves the growing history window.
func (c *Client) LoadWindow(n int) ([]Snapshot, error) {
	return c.backend.Load(n)
}

// FileBackend implements local filesystem storage.
type FileBackend struct{}

func (b *FileBackend) Append(s Snapshot) error {
	path, err := GetLedgerPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (b *FileBackend) Load(n int) ([]Snapshot, error) {
	path, err := GetLedgerPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []Snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var history []Snapshot
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var s Snapshot
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue
		}
		history = append(history, s)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(history) > n {
		return history[len(history)-n:], nil
	}
	return history, nil
}

// GetLedgerPath provides the default local storage path.
func GetLedgerPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cloudslash", "ledger.jsonl"), nil
}
