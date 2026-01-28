package provenance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// ResourceLocation holds the file path and line numbers for a resource definition.
type ResourceLocation struct {
	FilePath  string
	StartLine int
	EndLine   int
}

// FindResource scans a directory of .tf files to find a specific resource block.
// resourceType: e.g. "aws_instance"
// resourceName: e.g. "worker"
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
			// Log error but continue scanning other files? For now, we just skip broken files.
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
				// HCL ranges are 1-based, perfect for editors and git blame
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

// Schema to basically match any top-level block structure so we can iterate.
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
