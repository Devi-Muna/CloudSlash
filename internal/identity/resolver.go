package identity

import (
)

// UserProvider defines the interface for looking up users in an external directory (Slack, LDAP).
type UserProvider interface {
	LookupByEmail(email string) (*DirectoryUser, error)
	LookupByName(name string) ([]DirectoryUser, error)
}

type DirectoryUser struct {
	ID    string
	Name  string
	Email string
}

// Resolver handles the logic of matching git authors to directory users.
type Resolver struct {
	store    *Store
	provider UserProvider
}

func NewResolver(store *Store, provider UserProvider) *Resolver {
	return &Resolver{
		store:    store,
		provider: provider,
	}
}

// Resolve attempts to find the corporate identity for a git author.
func (r *Resolver) Resolve(gitName, gitEmail string) (Mapping, ResolutionStatus) {
	// 1. Check Local Cache (Persistence)
	// We check against the git name first, as that is our primary key for "unknowns" like 'poppie'.
	if m, ok := r.store.Get(gitName); ok && m.Verified {
		return m, StatusVerified
	}

	// 2. The Golden Key (Email Match)
	if gitEmail != "" {
		if user, err := r.provider.LookupByEmail(gitEmail); err == nil && user != nil {
			// 100% Match
			m := Mapping{
				GitAuthor:   gitName,
				SlackUserID: user.ID,
				NiceName:    user.Name,
				Confidence:  1.0,
				Verified:    true,
			}
			r.store.Put(gitName, m)
			_ = r.store.Save() // Auto-save verified matches
			return m, StatusVerified
		}
	}

	// 3. Fallback: Fuzzy Name Match (Optional, strict threshold)
	// Enterprise mode configuration disables fuzzy matching by default for safety.
	// Requires standard fuzzy matching library integration.
	// For this release, strict matching is enforced.
	
	// 4. Unknown - Require Human Intervention
	return Mapping{
		GitAuthor: gitName,
	}, StatusUnknown
}

type ResolutionStatus string

const (
	StatusVerified ResolutionStatus = "VERIFIED"
	StatusUnknown  ResolutionStatus = "UNKNOWN"
)
