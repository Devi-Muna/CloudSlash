package provenance

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/providers/terraform"
)

// Engine orchestrates the provenance lookup.
type Engine struct {
	RepoRoot string
}

func NewEngine(root string) *Engine {
	return &Engine{RepoRoot: root}
}

// Attribute attempts to find the author and commit for a given AWS resource ID.
// It uses the Terraform State to bridge the gap between AWS ID and Source Code.
func (e *Engine) Attribute(resourceID string, state *terraform.TerraformState) (*ProvenanceRecord, error) {
	// 1. State Bridge
	// Search TF State for the resource ID
	addr, err := terraform.FindAddressByID(state, resourceID)
	if err != nil {
		return nil, fmt.Errorf("state lookup failed: %w", err)
	}
	if addr == "" {
		return nil, fmt.Errorf("resource ID %s not found in state", resourceID)
	}

	// Extract the resource type and name from the address string.
	// Standard format: type.name (e.g. aws_instance.worker)
	
	var resType, resName string
	
	// Clean off index if present
	cleanAddr := addr
	if idx := strings.Index(cleanAddr, "["); idx != -1 {
		cleanAddr = cleanAddr[:idx]
	}
	
	addrParts := strings.Split(cleanAddr, ".")
	if len(addrParts) >= 2 {
		resName = addrParts[len(addrParts)-1]
		resType = addrParts[len(addrParts)-2]
	} else {
		return nil, fmt.Errorf("could not parse type/name from address: %s", cleanAddr)
	}

	// 2. Source Anchor
	// Search RepoRoot for the resource definition.
	loc, err := FindResourceInDir(e.RepoRoot, resType, resName)
	if err != nil {
	// Fallback to error if not found.
		return nil, fmt.Errorf("AST lookup failed: %w", err)
	}

	// 3. Temporal Link
	blame, err := GetBlame(loc.FilePath, loc.StartLine, loc.EndLine)
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	// 4. Record
	rec := &ProvenanceRecord{
		ResourceID: resourceID,
		TFAddress:  addr,
		FilePath:   filepath.Base(loc.FilePath), // relative path better?
		LineStart:  loc.StartLine,
		LineEnd:    loc.EndLine,
		Author:     blame.Author,
		CommitHash: blame.Hash,
		CommitDate: blame.Date,
		Message:    blame.Message,
	}
	
	// Statute of Limitations
	if time.Since(blame.Date) > 365*24*time.Hour {
		rec.IsLegacy = true
	}

	return rec, nil
}
