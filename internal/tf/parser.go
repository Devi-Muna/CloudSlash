package tf

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CodeAuditor finds the source code definition of a resource.
type CodeAuditor struct {
	State   *State
	Mapping map[string]string // ID -> Address
}

// NewCodeAuditor creates a new auditor.
func NewCodeAuditor(state *State) *CodeAuditor {
	if state == nil {
		return &CodeAuditor{}
	}
	return &CodeAuditor{
		State:   state,
		Mapping: state.GetResourceMapping(),
	}
}

// FindSource attempts to locate the file and line number where the resource is defined.
func (a *CodeAuditor) FindSource(resourceID string, rootDir string) (string, int, error) {
	if a.Mapping == nil {
		return "", 0, fmt.Errorf("no state mapping available")
	}

	address, ok := a.Mapping[resourceID]
	if !ok {
		return "", 0, fmt.Errorf("resource ID %s not found in state", resourceID)
	}

	// Address format: module.foo.type.name OR type.name
	// We need to extract Type and Name.
	// This is a naive parser for now.
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("invalid address format: %s", address)
	}

	resourceName := parts[len(parts)-1]
	resourceType := parts[len(parts)-2]

	// Regex to match: resource "type" "name"
	// Allow flexible whitespace.
	// resourceType and resourceName are safe strings from TF state (mostly).
	pattern := fmt.Sprintf(`resource\s+"%s"\s+"%s"`, regexp.QuoteMeta(resourceType), regexp.QuoteMeta(resourceName))
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", 0, err
	}

	var foundFile string
	var foundLine int

	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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

		lineNum, found := scanFile(path, re)
		if found {
			foundFile = path
			foundLine = lineNum
			return fmt.Errorf("found") // Abort walk
		}
		return nil
	})

	if foundFile != "" {
		// Return relative path if possible for cleaner UI
		rel, err := filepath.Rel(rootDir, foundFile)
		if err == nil {
			foundFile = rel
		}
		return foundFile, foundLine, nil
	}

	return "", 0, fmt.Errorf("definition not found in %s", rootDir)
}

func scanFile(path string, re *regexp.Regexp) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if re.MatchString(scanner.Text()) {
			return lineNum, true
		}
	}
	return 0, false
}
