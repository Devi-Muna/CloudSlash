package lazarus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/storage"

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

// Save writes the tombstone to the storage backend.
func (t *Tombstone) Save(ctx context.Context, store storage.BlobStore) error {
	key := fmt.Sprintf("%s.json", t.ResourceID)
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize tombstone: %w", err)
	}

	return store.Put(ctx, key, data)
}

// LoadTombstone reads a tombstone from storage.
func LoadTombstone(ctx context.Context, store storage.BlobStore, key string) (*Tombstone, error) {
	data, err := store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var t Tombstone
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to parse tombstone: %w", err)
	}
	return &t, nil
}
