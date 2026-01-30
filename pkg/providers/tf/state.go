package tf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// State represents Terraform state file.
type State struct {
	Version          int        `json:"version"`
	TerraformVersion string     `json:"terraform_version"`
	Resources        []Resource `json:"resources"`
}

// Resource represents a state resource.
type Resource struct {
	Mode      string     `json:"mode"`
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Provider  string     `json:"provider"`
	Instances []Instance `json:"instances"`
}

// Instance represents a resource instance.
type Instance struct {
	Attributes map[string]interface{} `json:"attributes"`
}

// BackendConfig represents the parsed remote backend configuration.
type BackendConfig struct {
	Type   string
	Bucket string
	Key    string
	Region string
}

// LoadState smart-loads the state from file or remote backend.
func LoadState(ctx context.Context, path string) (*State, error) {
	// 1. Try Local File
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return ParseStateFile(path)
	}

	// 2. Try Remote Backend Detection (if path is a dir or not found)
	// If path is a file that doesn't exist, we assume it might be a dir assumption or usage error, 
	// but if user passed "." or a dir, we search there.
	searchDir := path
	if path == "" {
		searchDir = "."
	} else if info, err := os.Stat(path); err == nil && info.IsDir() {
		searchDir = path
	}

	backend, err := DetectBackend(searchDir)
	if err == nil && backend != nil {
		fmt.Printf("Detected Remote Backend: %s (s3://%s/%s)\n", backend.Type, backend.Bucket, backend.Key)
		return FetchRemoteState(ctx, backend)
	}

	return nil, fmt.Errorf("no state file found at '%s' and no remote backend detected", path)
}

// ParseStateFile reads local state file.
func ParseStateFile(path string) (*State, error) {
	// Safety Check: Is state locked?
	lockPath := fmt.Sprintf("%s.lock.info", path)
	if _, err := os.Stat(lockPath); err == nil {
		return nil, fmt.Errorf("terraform state is locked (lock file found: %s). Aborting to prevent race condition", lockPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %v", err)
	}

	return &state, nil
}

// DetectBackend scans the directory for HCL files defining a backend.
func DetectBackend(rootDir string) (*BackendConfig, error) {
	parser := hclparse.NewParser()
	var backend *BackendConfig

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

		// Parse HCL
		f, diags := parser.ParseHCLFile(path)
		if diags != nil && diags.HasErrors() {
			// Ignore parse errors, best effort
			return nil
		}

		if body, ok := f.Body.(*hclsyntax.Body); ok {
			for _, block := range body.Blocks {
				if block.Type == "terraform" {
					for _, inner := range block.Body.Blocks {
						if inner.Type == "backend" && len(inner.Labels) > 0 {
							bType := inner.Labels[0]
							if bType == "s3" {
								// Found S3 Backend
								backend = &BackendConfig{Type: "s3"}
								attrs := inner.Body.Attributes
								if val, ok := attrs["bucket"]; ok {
									if v, err := val.Expr.Value(nil); err == nil && v.Type() == cty.String {
										backend.Bucket = v.AsString()
									}
								}
								if val, ok := attrs["key"]; ok {
									if v, err := val.Expr.Value(nil); err == nil && v.Type() == cty.String {
										backend.Key = v.AsString()
									}
								}
								if val, ok := attrs["region"]; ok {
									if v, err := val.Expr.Value(nil); err == nil && v.Type() == cty.String {
										backend.Region = v.AsString()
									}
								}
								return io.EOF // Stop walking
							}
						}
					}
				}
			}
		}
		return nil
	})

	if err == io.EOF && backend != nil {
		return backend, nil
	}

	return nil, fmt.Errorf("no supported backend found")
}

// FetchRemoteState downloads the state from the backend.
func FetchRemoteState(ctx context.Context, backend *BackendConfig) (*State, error) {
	if backend.Type != "s3" {
		return nil, fmt.Errorf("unsupported backend type: %s", backend.Type)
	}

	// Load AWS Config (Use environment or allow profile override if we supported it here)
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(backend.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config for backend: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	fmt.Printf("Fetching state from s3://%s/%s...\n", backend.Bucket, backend.Key)

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &backend.Bucket,
		Key:    &backend.Key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage state: %v", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read state body: %v", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		// Try to parse as wrapper if TF Cloud? S3 usually stores raw JSON.
		return nil, fmt.Errorf("failed to parse remote state JSON: %v", err)
	}

	return &state, nil
}

// GetManagedResourceIDs returns managed IDs.
func (s *State) GetManagedResourceIDs() map[string]bool {
	managed := make(map[string]bool)

	for _, res := range s.Resources {
		for _, inst := range res.Instances {
			// Index ID and ARN.
			if id, ok := inst.Attributes["id"].(string); ok {
				managed[id] = true
			}
			if arn, ok := inst.Attributes["arn"].(string); ok {
				managed[arn] = true
			}
		}
	}

	return managed
}

// GetResourceMapping maps IDs to addresses.
func (s *State) GetResourceMapping() map[string]string {
	mapping := make(map[string]string)

	for _, res := range s.Resources {
		// Build address.
		address := fmt.Sprintf("%s.%s", res.Type, res.Name)

		for _, inst := range res.Instances {
			if id, ok := inst.Attributes["id"].(string); ok {
				mapping[id] = address
			}
			if arn, ok := inst.Attributes["arn"].(string); ok {
				mapping[arn] = address
			}
		}
	}
	return mapping
}
