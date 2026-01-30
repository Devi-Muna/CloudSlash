package provenance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// ResourceLocation defines a resource block position.
type ResourceLocation struct {
	FilePath  string
	StartLine int
	EndLine   int
}

// FindResourceInDir scans directory for a resource.
func FindResourceInDir(dir string, resourceType, resourceName string) (*ResourceLocation, error) {
	parser := hclparse.NewParser()

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir %s: %w", dir, err)
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".tf") {
			continue
		}

		path := filepath.Join(dir, f.Name())
		hclFile, diags := parser.ParseHCLFile(path)
		if diags.HasErrors() {
			// Skip broken files.
			continue
		}

		if loc := findInFile(hclFile, resourceType, resourceName); loc != nil {
			loc.FilePath = path
			return loc, nil
		}
	}

	return nil, fmt.Errorf("resource %s.%s not found in %s", resourceType, resourceName, dir)
}

func findInFile(f *hcl.File, wantType, wantName string) *ResourceLocation {
	body := f.Body
	content, _, _ := body.PartialContent(schema) // basic schema to just look for blocks

	for _, block := range content.Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 {
			resType := block.Labels[0]
			resName := block.Labels[1]

			if resType == wantType && resName == wantName {
				// HCL ranges are 1-based.
				rng := block.DefRange
				return &ResourceLocation{
					StartLine: rng.Start.Line,
					EndLine:   rng.End.Line,
				}
			}
		}
	}
	return nil
}

// Schema matches top-level blocks.
var schema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "resource", LabelNames: []string{"type", "name"}},
		{Type: "module", LabelNames: []string{"name"}},
		{Type: "data", LabelNames: []string{"type", "name"}},
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "output", LabelNames: []string{"name"}},
		{Type: "locals", LabelNames: nil},
		{Type: "terraform", LabelNames: nil},
		{Type: "provider", LabelNames: []string{"name"}},
	},
}
