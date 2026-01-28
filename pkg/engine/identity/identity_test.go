package identity

import (
	"fmt"
	"os"
	"testing"
)

// MockProvider simulates a Slack Directory
type MockProvider struct {
	validEmails map[string]*DirectoryUser
}

func (m *MockProvider) LookupByEmail(email string) (*DirectoryUser, error) {
	if u, ok := m.validEmails[email]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockProvider) LookupByName(name string) ([]DirectoryUser, error) {
	return nil, fmt.Errorf("fuzzy matching disabled for test")
}

func TestIdentityResolution(t *testing.T) {
	// 1. Setup
	tempDir, _ := os.MkdirTemp("", "cloudslash-identity-test")
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	mock := &MockProvider{
		validEmails: map[string]*DirectoryUser{
			"jdoe@acme.corp": {ID: "U12345", Name: "Jane Doe", Email: "jdoe@acme.corp"},
		},
	}

	resolver := NewResolver(store, mock)

	// Case 1: The Golden Key (Email Match)
	// Git says: "poppie" <jdoe@acme.corp>
	// Expected: Match because email matches mock
	mapping, status := resolver.Resolve("poppie", "jdoe@acme.corp")
	
	if status != StatusVerified {
		t.Errorf("expected VERIFIED, got %s", status)
	}
	if mapping.SlackUserID != "U12345" {
		t.Errorf("expected U12345, got %s", mapping.SlackUserID)
	}
	if mapping.NiceName != "Jane Doe" {
		t.Errorf("expected Jane Doe, got %s", mapping.NiceName)
	}

	// Case 2: Unknown Identity (Different Email)
	// Git says: "hacker" <evil@gmail.com>
	// Expected: UNKNOWN (Confidence 0)
	mapping2, status2 := resolver.Resolve("hacker", "evil@gmail.com")
	
	if status2 != StatusUnknown {
		t.Errorf("expected UNKNOWN, got %s", status2)
	}
	if mapping2.SlackUserID != "" {
		t.Errorf("expected empty SlackUserID, got %s", mapping2.SlackUserID)
	}

	// Case 3: Persistence Check
	// Reload store and check if Jane Doe is saved
	store2, _ := NewStore(tempDir)
	val, ok := store2.Get("poppie")
	if !ok {
		t.Fatal("failed to persist mapping for 'poppie'")
	}
	if val.SlackUserID != "U12345" {
		t.Errorf("persisted value mismatch: wanted U12345, got %s", val.SlackUserID)
	}
}
