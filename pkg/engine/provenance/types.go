package provenance

import "time"

// ProvenanceRecord holds resource attribution.
type ProvenanceRecord struct {
	ResourceID string
	TFAddress  string
	FilePath   string
	LineStart  int
	LineEnd    int
	Author     string
	CommitHash string
	CommitDate time.Time
	Message    string
	IsLegacy   bool // True if commit is older than 1 year (Statute of Limitations)
}
