package lazarus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Tombstone represents the serialized "Soul" (Configuration) of a resource before deletion.
type Tombstone struct {
	ResourceID   string                 `json:"resource_id"`
	ResourceType string                 `json:"resource_type"`
	Timestamp    int64                  `json:"timestamp"`
	Region       string                 `json:"region"`
	Soul         map[string]interface{} `json:"soul"` // The preservation of configuration
}

// NewTombstone creates a new preservation record.
func NewTombstone(id, kind, region string, metadata map[string]interface{}) *Tombstone {
	return &Tombstone{
		ResourceID:   id,
		ResourceType: kind,
		Timestamp:    time.Now().Unix(),
		Region:       region,
		Soul:         metadata,
	}
}

// Save writes the tombstone to disk (e.g., .cloudslash/tombstones/{id}.json).
func (t *Tombstone) Save(dir string) error {
	path := filepath.Join(dir, fmt.Sprintf("%s.json", t.ResourceID))
	
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tombstone directory: %w", err)
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize tombstone: %w", err)
	}

	return os.WriteFile(path, data, 0600) // Secure permissions
}

// LoadTombstone reads a tombstone from disk.
func LoadTombstone(path string) (*Tombstone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Tombstone
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to parse tombstone: %w", err)
	}
	return &t, nil
}
