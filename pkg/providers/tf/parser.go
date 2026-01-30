package tf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// SourceLocation represents a file and line number.
type SourceLocation struct {
	File string
	Line int
}

// CodeAuditor locates resource definitions using AST analysis.
type CodeAuditor struct {
	State   *State
	Mapping map[string]string // ID -> Address
	
	// Cache for AST analysis
	mu        sync.RWMutex
	index     map[string]SourceLocation // "type.name" -> Location
	indexed   bool
}

// NewCodeAuditor creates a new auditor.
func NewCodeAuditor(state *State) *CodeAuditor {
	if state == nil {
		return &CodeAuditor{
			index: make(map[string]SourceLocation),
		}
	}
	return &CodeAuditor{
		State:   state,
		Mapping: state.GetResourceMapping(),
		index:   make(map[string]SourceLocation),
	}
}

// FindSource locates resource definition in files.
// Uses on-demand indexing of the rootDir.
func (a *CodeAuditor) FindSource(resourceID string, rootDir string) (string, int, error) {
	if a.Mapping == nil {
		return "", 0, fmt.Errorf("no state mapping available")
	}

	address, ok := a.Mapping[resourceID]
	if !ok {
		return "", 0, fmt.Errorf("resource ID %s not found in state", resourceID)
	}

	// Parse address components.
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("invalid address format: %s", address)
	}

	resourceName := parts[len(parts)-1]
	resourceType := parts[len(parts)-2]
	key := fmt.Sprintf("%s.%s", resourceType, resourceName)

	// Check Index
	a.mu.RLock()
	if loc, exists := a.index[key]; exists {
		a.mu.RUnlock()
		return loc.File, loc.Line, nil
	}
	isIndexed := a.indexed
	a.mu.RUnlock()

	if isIndexed {
		// If already indexed and not found, it's missing.
		return "", 0, fmt.Errorf("definition not found in %s", rootDir)
	}

	// Index the directory (One-time cost)
	if err := a.indexDirectory(rootDir); err != nil {
		return "", 0, err
	}

	// Retry Lookup
	a.mu.RLock()
	defer a.mu.RUnlock()
	if loc, exists := a.index[key]; exists {
		return loc.File, loc.Line, nil
	}

	return "", 0, fmt.Errorf("definition not found in %s", rootDir)
}

// indexDirectory walks the directory and uses HCL AST to find all resources.
func (a *CodeAuditor) indexDirectory(rootDir string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.indexed {
		return nil
	}

	parser := hclparse.NewParser()

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".terraform" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".tf") {
			return nil
		}

		// Parse HCL (AST Analysis)
		f, parseDiags := parser.ParseHCLFile(path)
		if parseDiags != nil && parseDiags.HasErrors() {
			// Start with best-effort, ignore parse errors in individual files
			return nil 
		}

		// Calculate relative path for reporting
		relPath, _ := filepath.Rel(rootDir, path)

		// Analyze Blocks
		// We cast to hclsyntax.Body to get raw blocks/attributes
		if body, ok := f.Body.(*hclsyntax.Body); ok {
			for _, block := range body.Blocks {
				if block.Type == "resource" && len(block.Labels) == 2 {
					resType := block.Labels[0]
					resName := block.Labels[1]
					
					key := fmt.Sprintf("%s.%s", resType, resName)
					a.index[key] = SourceLocation{
						File: relPath,
						Line: block.Range().Start.Line,
					}
				}
			}
		}
		return nil
	})

	a.indexed = true
	return err
}
