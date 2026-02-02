package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Mapping represents a confirmed link between a Git Author and a Messaging User (Slack).
type Mapping struct {
	GitAuthor   string  `json:"git_author"` // e.g., "poppie"
	SlackUserID string  `json:"slack_id"`   // e.g., "U12345"
	NiceName    string  `json:"nice_name"`  // e.g., "Jane Doe"
	Confidence  float64 `json:"confidence"` // 0.0 to 1.0
	Verified    bool    `json:"verified"`   // True if manually confirmed or email match
}

// Store handles the persistence of identity mappings.
type Store struct {
	path string
	mu   sync.RWMutex
	data map[string]Mapping // Key: GitAuthor
}

// NewStore initializes the identity store.
func NewStore(homeDir string) (*Store, error) {
	path := filepath.Join(homeDir, ".cloudslash", "identity_map.json")
	s := &Store{
		path: path,
		data: make(map[string]Mapping),
	}
	if err := s.load(); err != nil {
		// It's okay if file doesn't exist yet
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&s.data)
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("failed to save identity map: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(s.data)
}

// Get returns the mapping for a git author if it exists.
func (s *Store) Get(gitAuthor string) (Mapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.data[gitAuthor]
	return m, ok
}

// Put adds or updates a mapping. call Save() to persist.
func (s *Store) Put(gitAuthor string, list Mapping) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[gitAuthor] = list
}
