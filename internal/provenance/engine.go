package provenance

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/internal/terraform"
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

	// addr is like "module.vpc.aws_nat_gateway.main"
	// We need to parse this. For v1, let's assume flat structure or simple module support.
	// Parsing the address to get Type and Name.
	// Last two parts are usually Type and Name.
	// e.g. aws_instance.worker -> Type=aws_instance, Name=worker
	
	parts := strings.Split(addr, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid address format: %s", addr)
	}
	
	// Heuristic: The last two parts that are not array indices are Type and Name.
	// But TF addresses can be tricky: module.x.aws_instance.y["key"]
	// Let's simplify: look for the resource type part.
	
	// A robust parser would be ideal, but for now let's try to extract from the end.
	// The standard format ends with type.name (or type.name[key])
	
	// Let's try to find the resource type and name from the address string.
	// We can cheat: we already know the resource type from the Graph Node usually? 
	// But here we only pass ID.
	
	// Actually, `tf.FindAddressByID` returns the full address.
	// Let's assume standard 'aws_type.name' at the end.
	
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
	// Find the file defining this resource.
	// In a real generic engine, we'd traverse modules. 
	// For v1 POC, we search the entire repo or specific dirs?
	// Let's assume we search the RepoRoot recursively? No, that's slow.
	// Let's assume standard layout?
	// Or just search root?
	
	// Let's search the RepoRoot.
	loc, err := FindResourceInDir(e.RepoRoot, resType, resName)
	if err != nil {
		// Try recursive search?
		// For now, fail fast.
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
