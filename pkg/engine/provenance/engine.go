package provenance

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/providers/terraform"
)

// Engine manages provenance lookup.
type Engine struct {
	RepoRoot string
}

func NewEngine(root string) *Engine {
	return &Engine{RepoRoot: root}
}

// Attribute resolves resource authorship.
func (e *Engine) Attribute(resourceID string, state *terraform.TerraformState) (*ProvenanceRecord, error) {
	// Bridge AWS ID to source via Terraform state.
	// Lookup ID in state.
	addr, err := terraform.FindAddressByID(state, resourceID)
	if err != nil {
		return nil, fmt.Errorf("state lookup failed: %w", err)
	}
	if addr == "" {
		return nil, fmt.Errorf("resource ID %s not found in state", resourceID)
	}

	// Parse resource address.
	
	var resType, resName string
	
	// Strip index.
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

	// Locate resource definition.
	loc, err := FindResourceInDir(e.RepoRoot, resType, resName)
	if err != nil {
		return nil, fmt.Errorf("AST lookup failed: %w", err)
	}

	// Resolve Git blame.
	blame, err := GetBlame(loc.FilePath, loc.StartLine, loc.EndLine)
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	// Build record.
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
	
	// Check statute of limitations (1 year).
	if time.Since(blame.Date) > 365*24*time.Hour {
		rec.IsLegacy = true
	}

	return rec, nil
}
