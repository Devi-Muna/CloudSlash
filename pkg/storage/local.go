package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LocalStore implements BlobStore for local filesystem.
type LocalStore struct {
	Root string
}

func NewLocalStore(root string) *LocalStore {
	return &LocalStore{Root: root}
}

func (s *LocalStore) Put(ctx context.Context, key string, data []byte) error {
	path := filepath.Join(s.Root, key)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

func (s *LocalStore) Get(ctx context.Context, key string) ([]byte, error) {
	path := filepath.Join(s.Root, key)
	return os.ReadFile(path)
}

func (s *LocalStore) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	root := filepath.Join(s.Root, prefix)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(s.Root, path)
			keys = append(keys, rel)
		}
		return nil
	})

	return keys, err
}
