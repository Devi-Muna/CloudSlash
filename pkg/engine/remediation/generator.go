package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/lazarus"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/resources"
	"github.com/DrSkyle/cloudslash/v2/pkg/storage"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
)

// Generator creates remediation plans.
type Generator struct {
	Graph  *graph.Graph
	Logger *slog.Logger
}

// NewGenerator initializes the generator.
func NewGenerator(g *graph.Graph, logger *slog.Logger) *Generator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Generator{Graph: g, Logger: logger}
}

var idRegex = regexp.MustCompile("^[a-zA-Z0-9._/-]+$")

// PlanAction is a remediation step.
type PlanAction struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Operation   string                 `json:"operation"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`

	// Transaction Safety
	PreConditions  []Condition `json:"pre_conditions,omitempty"`
	PostConditions []Condition `json:"post_conditions,omitempty"`
	Rollback       *PlanAction `json:"rollback,omitempty"`
}

// Condition is a verification check.
type Condition struct {
	Type   string            `json:"type"` // e.g. "EXISTS", "STATUS", "TAG_MATCH"
	Params map[string]string `json:"params"`
}

// TransactionManifest is the remediation log.
type TransactionManifest struct {
	Version     string       `json:"version"`
	GeneratedAt time.Time    `json:"generated_at"`
	Actions     []PlanAction `json:"actions"`
}

// Deprecated: Use TransactionManifest
type RemediationPlan = TransactionManifest

// GenerateRemediationPlan creates a JSON plan.
func (g *Generator) GenerateRemediationPlan(path string) error {
	plan := RemediationPlan{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		Actions:     []PlanAction{},
	}

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()


	ctx := context.Background()
	var blobStore storage.BlobStore

	// Configure Storage Backend.
	s3Bucket := os.Getenv("CLOUDSLASH_S3_BUCKET")
	if s3Bucket != "" {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err == nil {
			blobStore = storage.NewS3Store(cfg, s3Bucket)
			g.Logger.Info("Using S3 Backend", "bucket", s3Bucket)
		} else {
			g.Logger.Warn("Failed to load AWS config for S3 backend. Falling back to local.", "error", err)
		}
	}

	if blobStore == nil {
		tombstoneDir := ".cloudslash/tombstones"
		blobStore = storage.NewLocalStore(tombstoneDir)

		// Check for CI environment with ephemeral storage.
		isCI := os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != ""
		if isCI {
			g.Logger.Warn("Tombstones saved to ephemeral storage in CI", 
				"path", ".cloudslash/tombstones",
				"recommendation", "Configure CLOUDSLASH_S3_BUCKET for persistent storage")
		}
	}

	for _, node := range g.Graph.GetNodes() {
		if !node.IsWaste {
			continue
		}

		// Parse Resource ID.
		resourceID := extractResourceID(node.IDStr())
		if !idRegex.MatchString(resourceID) {
			// Skip malformed IDs silently or log.
			continue
		}

		// Phase 1: Snapshot/Tombstone (Side Effect)
		region := "unknown"
		if r, ok := node.Properties["region"].(string); ok {
			region = r
		}

		// Capture state now.
		ts := lazarus.NewTombstone(resourceID, node.TypeStr(), region, node.Properties)
		if err := ts.Save(ctx, blobStore); err != nil {
			g.Logger.Error("Failed to save tombstone. Skipping remediation generation", "resourceID", resourceID, "error", err)
			continue
		}

		// Phase 2: Action Definition
		action := PlanAction{
			ID:   resourceID,
			Type: node.TypeStr(),
		}

		expiry := time.Now().AddDate(0, 0, 30).Format("2006-01-02")

		// Default Params
		params := map[string]interface{}{
			"Region": region,
			"Tags": map[string]string{
				"CloudSlash:Status":     "Purgatory",
				"CloudSlash:ExpiryDate": expiry,
			},
		}

		// Add Tombstone Ref.
		if s3Bucket != "" {
			params["TombstoneURI"] = fmt.Sprintf("s3://%s/%s.json", s3Bucket, resourceID)
		} else {
			params["TombstoneURI"] = fmt.Sprintf("file://.cloudslash/tombstones/%s.json", resourceID)
		}

		// Default Pre-condition: Resource must exist.
		action.PreConditions = append(action.PreConditions, Condition{
			Type:   "EXISTS",
			Params: map[string]string{"ID": resourceID, "Region": region},
		})

		switch node.TypeStr() {
		case resources.EC2Instance:
			action.Operation = "STOP"
			action.Description = "Tag and Stop EC2 Instance"

			// Post-condition: Instance State = stopped
			action.PostConditions = append(action.PostConditions, Condition{
				Type:   "STATUS_MATCH",
				Params: map[string]string{"ID": resourceID, "Region": region, "Value": "stopped"},
			})

			// Rollback: Start Instance
			action.Rollback = &PlanAction{
				ID: resourceID, Type: node.TypeStr(), Operation: "START",
				Description: "Rollback: Start Instance",
				Parameters:  map[string]interface{}{"Region": region},
			}

		case "AWS::EC2::Volume":
			isGP2 := false
			if v, ok := node.Properties["IsGP2"].(bool); ok && v {
				isGP2 = true
			}

			if isGP2 {
				action.Operation = "MODIFY"
				action.Description = "Upgrade Volume to gp3"
				params["VolumeType"] = "gp3"

				action.PostConditions = append(action.PostConditions, Condition{
					Type:   "PROPERTY_MATCH",
					Params: map[string]string{"ID": resourceID, "Region": region, "Property": "VolumeType", "Value": "gp3"},
				})
			} else {
				action.Operation = "SNAPSHOT_AND_DELETE" // Upgraded from DELETE
				action.Description = "Snapshot, Tag and Delete EBS Volume"
			}

		case "AWS::RDS::DBInstance":
			action.Operation = "STOP"
			action.Description = "Tag and Stop RDS Instance"
			action.PostConditions = append(action.PostConditions, Condition{
				Type:   "STATUS_MATCH",
				Params: map[string]string{"ID": resourceID, "Region": region, "Value": "stopped"},
			})
			action.Rollback = &PlanAction{
				ID: resourceID, Type: node.TypeStr(), Operation: "START",
				Description: "Rollback: Start DB Instance",
			}

		case "AWS::EC2::NatGateway":
			action.Operation = "DELETE"
			action.Description = "Delete NAT Gateway"
			action.PostConditions = append(action.PostConditions, Condition{
				Type:   "NOT_EXISTS",
				Params: map[string]string{"ID": resourceID, "Region": region},
			})

		case "AWS::EC2::EIP":
			action.Operation = "RELEASE"
			action.Description = "Release Elastic IP"
			action.PostConditions = append(action.PostConditions, Condition{
				Type:   "NOT_EXISTS",
				Params: map[string]string{"ID": resourceID, "Region": region},
			})

		case "AWS::EC2::AMI":
			action.Operation = "DEREGISTER"
			action.Description = "Deregister AMI"
			action.PostConditions = append(action.PostConditions, Condition{
				Type:   "NOT_EXISTS",
				Params: map[string]string{"ID": resourceID, "Region": region},
			})

		// ... (others keep basic DELETE) ...
		default:
			action.Operation = "DELETE" // Conservative default if known waste
			action.Description = fmt.Sprintf("Delete %s", node.TypeStr())
			action.PostConditions = append(action.PostConditions, Condition{
				Type:   "NOT_EXISTS",
				Params: map[string]string{"ID": resourceID, "Region": region},
			})
		}

		action.Parameters = params
		plan.Actions = append(plan.Actions, action)
	}

	// Generate Sidecar Script
	if err := g.GenerateBashScript(strings.ReplaceAll(path, ".json", ".sh"), plan); err != nil {
		g.Logger.Warn("Failed to generate safe_cleanup.sh", "error", err)
	}

	// Serialize.
	// Use encoding/json, ensuring imports
	return writeJSON(path, plan)
}

// GenerateBashScript creates a shell script from the plan.
func (g *Generator) GenerateBashScript(path string, plan TransactionManifest) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash Safe Cleanup Script v%s\n", plan.Version)
	fmt.Fprintf(f, "# Generated: %s\n\n", plan.GeneratedAt)
	fmt.Fprintf(f, "set -e\n\n")

	for _, action := range plan.Actions {
		// FIX: Sanitize inputs before use
		region := shellQuote(action.Parameters["Region"].(string))
		id := shellQuote(action.ID)

		// Note: We use printf with single quotes to prevent any shell interpretation of the ID/Description
		// independent of the regex validation (Defense in Depth).
		fmt.Fprintf(f, "printf \"[Processing] %%s (%%s)...\\n\" %s %s\n", shellQuote(action.ID), shellQuote(action.Description))

		switch action.Operation {
		case "STOP":
			if action.Type == resources.EC2Instance {
				// FIX: Use sanitized variables
				fmt.Fprintf(f, "aws ec2 stop-instances --instance-ids %s --region %s\n", id, region)
				
				// Handle Tags safely
				expiryDate := action.Parameters["Tags"].(map[string]string)["CloudSlash:ExpiryDate"]
				// Even trusted internal values should be quoted to prevent regression if logic changes
				fmt.Fprintf(f, "aws ec2 create-tags --resources %s --tags Key=CloudSlash:Status,Value=Purgatory Key=CloudSlash:ExpiryDate,Value=%s --region %s\n", id, shellQuote(expiryDate), region)
			}
		case "SNAPSHOT_AND_DELETE":
			// FIX: Use sanitized variables for volume-id and tags
			fmt.Fprintf(f, "aws ec2 create-snapshot --volume-id %s --description 'CloudSlash Auto-Backup' --tag-specifications 'ResourceType=snapshot,Tags=[{Key=CreatedBy,Value=CloudSlash},{Key=SourceVolume,Value=%s}]' --region %s\n", id, id, region)
			fmt.Fprintf(f, "aws ec2 delete-volume --volume-id %s --region %s\n", id, region)
		case "DELETE":
			if action.Type == "AWS::EC2::NatGateway" {
				// FIX: Use sanitized variables
				fmt.Fprintf(f, "aws ec2 delete-nat-gateway --nat-gateway-id %s --region %s\n", id, region)
			}
		// Add other cases as needed
		}
		fmt.Fprintf(f, "\n")
	}
	
	return os.Chmod(path, 0755)
}

// shellQuote quotes a string for bash.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
// GenerateRestorationPlan creates a restoration plan.
func (g *Generator) GenerateRestorationPlan(path string) error {
	plan := TransactionManifest{
		Version:     "2.0",
		GeneratedAt: time.Now(),
		Actions:     []PlanAction{},
	}

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	for _, node := range g.Graph.GetNodes() {
		if !node.IsWaste {
			continue
		}

		resourceID := extractResourceID(node.IDStr())
		if !idRegex.MatchString(resourceID) {
			continue
		}

		action := PlanAction{
			ID:   resourceID,
			Type: node.TypeStr(),
		}

		// Common properties
		if tfAddr, ok := node.Properties["TF_ADDRESS"].(string); ok {
			action.Parameters = map[string]interface{}{"TF_ADDRESS": tfAddr}
		} else {
			action.Parameters = make(map[string]interface{})
		}

		switch node.TypeStr() {
		case resources.EC2Instance:
			action.Operation = "RESTORE_INSTANCE"
			action.Description = "Start Instance and Import to State"
			plan.Actions = append(plan.Actions, action)

		case "AWS::EC2::Volume":
			action.Operation = "RESTORE_VOLUME"
			action.Description = "Re-attach Volume or Restore from Snapshot"
			if atts, ok := node.Properties["Attachments"].([]interface{}); ok && len(atts) > 0 {
				action.Parameters["Attachments"] = atts
			}
			plan.Actions = append(plan.Actions, action)

		case "AWS::RDS::DBInstance":
			action.Operation = "RESTORE_RDS"
			action.Description = "Start DB Instance"
			plan.Actions = append(plan.Actions, action)
		}
	}

	return writeJSON(path, plan)
}

// GenerateIgnorePlan creates an ignore plan.
func (g *Generator) GenerateIgnorePlan(path string) error {
	plan := TransactionManifest{
		Version:     "2.0",
		GeneratedAt: time.Now(),
		Actions:     []PlanAction{},
	}

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	// Sort for deterministic output
	type wasteItem struct {
		Node *graph.Node
	}
	var items []wasteItem

	for _, node := range g.Graph.GetNodes() {
		if node.IsWaste && !node.Justified {
			items = append(items, wasteItem{Node: node})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Node.IDStr() < items[j].Node.IDStr()
	})

	for _, item := range items {
		node := item.Node
		resourceID := extractResourceID(node.IDStr())
		if !strings.HasPrefix(node.IDStr(), "arn:") {
			continue
		}

		action := PlanAction{
			ID:          resourceID,
			Type:        node.TypeStr(),
			Operation:   "TAG_IGNORE",
			Description: "Apply cloudslash:ignore tag",
			Parameters: map[string]interface{}{
				"Tags": map[string]string{"cloudslash:ignore": "true"},
				"ARN":  node.IDStr(),
			},
		}
		plan.Actions = append(plan.Actions, action)
	}

	return writeJSON(path, plan)
}

func extractResourceID(id string) string {
	// Parse ARN using official library.
	if parsed, err := arn.Parse(id); err == nil {
		// Use fields function to split by / or : safely
		parts := strings.FieldsFunc(parsed.Resource, func(r rune) bool {
			return r == '/' || r == ':'
		})
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return parsed.Resource
	}
	return id
}

func writeJSON(path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
