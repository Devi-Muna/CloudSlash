package remediation

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/engine/lazarus"
	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// Generator automates remediation script creation.
type Generator struct {
	Graph *graph.Graph
}

// NewGenerator initializes the remediation engine.
func NewGenerator(g *graph.Graph) *Generator {
	return &Generator{Graph: g}
}

var idRegex = regexp.MustCompile("^[a-zA-Z0-9._/-]+$")

// GenerateSafeDeleteScript creates the purgatory cleanup script.
func (g *Generator) GenerateSafeDeleteScript(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash Safe Remediation Script (Purgatory Mode)\n")
	fmt.Fprintf(f, "# Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "set -e\n\n")

	// Ensure tombstone directory.
	tombstoneDir := ".cloudslash/tombstones"

	wasteCount := 0

	for _, node := range g.Graph.Nodes {
		if !node.IsWaste {
			continue
		}

		// Parse Resource ID.
		resourceID := extractResourceID(node.ID)

		if !idRegex.MatchString(resourceID) {
			fmt.Fprintf(f, "# SKIPPING MALFORMED ID (Potential Injection): %s\n", resourceID)
			continue
		}

		// Phase 1: The Tombstone Engine
		region := "unknown" // Default
		if r, ok := node.Properties["region"].(string); ok {
			region = r
		}
		// Persist state before modification.
		ts := lazarus.NewTombstone(resourceID, node.Type, region, node.Properties)
		if err := ts.Save(tombstoneDir); err != nil {
			fmt.Fprintf(f, "# ERROR: Failed to create tombstone for %s: %v\n", resourceID, err)
		} else {
			fmt.Fprintf(f, "# Tombstone created: %s/%s.json\n", tombstoneDir, resourceID)
		}

		// Phase 2: Enforce Purgatory.
		expiry := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
		
		switch node.Type {
		case "AWS::EC2::Instance":
			fmt.Fprintf(f, "echo \"[Purgatory] Tagging & Stopping Instance: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ec2 create-tags --resources %s --tags Key=CloudSlash:Status,Value=Purgatory Key=CloudSlash:ExpiryDate,Value=%s\n", shellEscape(resourceID), expiry)
			fmt.Fprintf(f, "aws ec2 stop-instances --instance-ids %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::EC2::Volume":
			isGP2 := false
			if v, ok := node.Properties["IsGP2"].(bool); ok && v {
				isGP2 = true
			}

			if isGP2 {
				fmt.Fprintf(f, "echo \"[Modernization] Upgrading Volume to GP3: %s\"\n", resourceID)
				fmt.Fprintf(f, "aws ec2 modify-volume --volume-id %s --volume-type gp3\n\n", shellEscape(resourceID))
			} else {
				fmt.Fprintf(f, "echo \"[Purgatory] Tagging & Deleting Volume: %s\"\n", resourceID)
				fmt.Fprintf(f, "aws ec2 create-tags --resources %s --tags Key=CloudSlash:Status,Value=Purgatory Key=CloudSlash:ExpiryDate,Value=%s\n", shellEscape(resourceID), expiry)
				fmt.Fprintf(f, "aws ec2 delete-volume --volume-id %s\n\n", shellEscape(resourceID))
			}
			wasteCount++

		case "AWS::RDS::DBInstance":
			fmt.Fprintf(f, "echo \"[Purgatory] Tagging & Stopping RDS: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws rds add-tags-to-resource --resource-name %s --tags Key=CloudSlash:Status,Value=Purgatory Key=CloudSlash:ExpiryDate,Value=%s\n", shellEscape(node.ID), expiry) 
			fmt.Fprintf(f, "aws rds stop-db-instance --db-instance-identifier %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::EC2::NatGateway":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting NAT Gateway: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ec2 delete-nat-gateway --nat-gateway-id %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::EC2::EIP":
			fmt.Fprintf(f, "echo \"[Purgatory] Releasing Elastic IP: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ec2 release-address --allocation-id %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::EC2::AMI":
			fmt.Fprintf(f, "echo \"[Purgatory] Deregistering AMI: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ec2 deregister-image --image-id %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::ElasticLoadBalancingV2::LoadBalancer":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting Load Balancer: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws elbv2 delete-load-balancer --load-balancer-arn %s\n\n", shellEscape(node.ID))
			wasteCount++

		case "AWS::ECS::Cluster":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting ECS Cluster: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ecs delete-cluster --cluster %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::ECS::Service":
			cluster := "default"
			if c, ok := node.Properties["ClusterName"].(string); ok {
				cluster = c
			} else {
				// Parse from ARN if possible
				if parsed, err := arn.Parse(node.ID); err == nil {
					parts := strings.Split(parsed.Resource, "/")
					if len(parts) > 1 {
						cluster = parts[1]
					}
				}
			}
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting ECS Service: %s (Cluster: %s)\"\n", resourceID, cluster)
			fmt.Fprintf(f, "aws ecs delete-service --cluster %s --service %s --force\n\n", shellEscape(cluster), shellEscape(resourceID))
			wasteCount++

		case "AWS::EKS::Cluster":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting EKS Cluster: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws eks delete-cluster --name %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::EKS::NodeGroup":
			cluster := "unknown"
			if c, ok := node.Properties["ClusterName"].(string); ok {
				cluster = c
			}
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting EKS NodeGroup: %s (Cluster: %s)\"\n", resourceID, cluster)
			fmt.Fprintf(f, "aws eks delete-nodegroup --cluster-name %s --nodegroup-name %s\n\n", shellEscape(cluster), shellEscape(resourceID))
			wasteCount++

		case "AWS::ECR::Repository":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting ECR Repository: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ecr delete-repository --repository-name %s --force\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::Lambda::Function":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting Lambda Function: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws lambda delete-function --function-name %s\n\n", shellEscape(resourceID))
			wasteCount++

		case "AWS::Logs::LogGroup":
			fmt.Fprintf(f, "echo \"[Purgatory] Deleting Log Group: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws logs delete-log-group --log-group-name %s\n\n", shellEscape(resourceID))
			wasteCount++

		default:
			// Fallback: Log manual review.
			fmt.Fprintf(f, "# [Review] Resource %s (%s) flagged. No Purgatory logic defined.\n\n", resourceID, node.Type)
		}
	}

	if wasteCount == 0 {
		fmt.Fprintf(f, "echo \"No waste found to remediate.\"\n")
	} else {
		fmt.Fprintf(f, "echo \"Purgatory Enforcement Complete. %d resources frozen.\"\n", wasteCount)
	}

	return nil
}

// GenerateUndoScript creates the restoration script.
func (g *Generator) GenerateUndoScript(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash Reanimator Script (Phase 3)\n")
	fmt.Fprintf(f, "# Restores resources from Purgatory.\n")
	fmt.Fprintf(f, "# Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "set -e\n\n")

	// Terraform import helper.
	fmt.Fprintf(f, "function tf_safe_import() {\n")
	fmt.Fprintf(f, "  local addr=$1\n")
	fmt.Fprintf(f, "  local id=$2\n")
	fmt.Fprintf(f, "  echo \"  [TF] Syncing $addr -> $id...\"\n")
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "  # 1. Try to remove old state (ignore 'not found', error on 'lock')\n")
	fmt.Fprintf(f, "  if ! OUTPUT=$(terraform state rm \"$addr\" 2>&1); then\n")
	fmt.Fprintf(f, "    if echo \"$OUTPUT\" | grep -q \"Lock\"; then\n")
	fmt.Fprintf(f, "      echo \"  [CRITICAL] Terraform State is LOCKED. Cannot sync.\"\n")
	fmt.Fprintf(f, "      echo \"  [ACTION] Release the lock and run this script again.\"\n")
	fmt.Fprintf(f, "      exit 1\n")
	fmt.Fprintf(f, "    elif echo \"$OUTPUT\" | grep -qv \"not found\"; then\n")
	fmt.Fprintf(f, "      # Genuine unknown error\n")
	fmt.Fprintf(f, "      echo \"  [ERROR] Terraform state rm failed: $OUTPUT\"\n")
	fmt.Fprintf(f, "      exit 1\n")
	fmt.Fprintf(f, "    fi\n")
	fmt.Fprintf(f, "  fi\n")
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "  # 2. Import new state\n")
	fmt.Fprintf(f, "  if ! terraform import \"$addr\" \"$id\"; then\n")
	fmt.Fprintf(f, "      echo \"  [ERROR] Import failed. Please run manually: terraform import '$addr' '$id'\"\n")
	fmt.Fprintf(f, "      exit 1\n")
	fmt.Fprintf(f, "  fi\n")
	fmt.Fprintf(f, "}\n\n")

	wasteCount := 0

	for _, node := range g.Graph.Nodes {
		if !node.IsWaste {
			continue
		}

		resourceID := extractResourceID(node.ID)
		if !idRegex.MatchString(resourceID) {
			continue
		}

		// Phase 1: The Tombstone Engine
		region := "us-east-1" // Default
		if r, ok := node.Properties["region"].(string); ok {
			region = r
		}

		fmt.Fprintf(f, "echo \"[Reanimator] Processing %s (%s)...\"\n", resourceID, node.Type)

		switch node.Type {
		case "AWS::EC2::Instance":
			// Check if resource exists.
			fmt.Fprintf(f, "if aws ec2 describe-instances --instance-ids %s >/dev/null 2>&1; then\n", shellEscape(resourceID))
			fmt.Fprintf(f, "  echo \"  Resurrecting Instance from Purgatory...\"\n")
			fmt.Fprintf(f, "  aws ec2 start-instances --instance-ids %s\n", shellEscape(resourceID))
			fmt.Fprintf(f, "  aws ec2 delete-tags --resources %s --tags Key=CloudSlash:Status Key=CloudSlash:ExpiryDate\n", shellEscape(resourceID))
			
			// Sync Terraform state.
			if tfAddress, ok := node.Properties["TF_ADDRESS"].(string); ok {
				fmt.Fprintf(f, "  tf_safe_import %s %s\n", shellEscape(tfAddress), shellEscape(resourceID))
			}

			fmt.Fprintf(f, "else\n")
			fmt.Fprintf(f, "  echo \"[FATAL] Instance %s is dead (Terminated).\"\n", resourceID)
			fmt.Fprintf(f, "fi\n\n")
			wasteCount++

		case "AWS::EC2::Volume":
			// Check if resource exists.
			fmt.Fprintf(f, "if aws ec2 describe-volumes --volume-ids %s >/dev/null 2>&1; then\n", shellEscape(resourceID))
			fmt.Fprintf(f, "  echo \"  Resurrecting Volume from Purgatory...\"\n")
			
			// Re-attach volume.
			if atts, ok := node.Properties["Attachments"].([]interface{}); ok && len(atts) > 0 {
				if att, ok := atts[0].(map[string]interface{}); ok {
					instanceID := att["InstanceId"].(string)
					device := att["Device"].(string)
					fmt.Fprintf(f, "  aws ec2 attach-volume --volume-id %s --instance-id %s --device %s\n", shellEscape(resourceID), instanceID, device)
				}
			}
			
			fmt.Fprintf(f, "  aws ec2 delete-tags --resources %s --tags Key=CloudSlash:Status Key=CloudSlash:ExpiryDate\n", shellEscape(resourceID))
			
			// Gap A: Terraform Sync
			if tfAddress, ok := node.Properties["TF_ADDRESS"].(string); ok {
				fmt.Fprintf(f, "  tf_safe_import %s %s\n", shellEscape(tfAddress), shellEscape(resourceID))
			}

			fmt.Fprintf(f, "else\n")
			// Hard recovery via snapshots.
			fmt.Fprintf(f, "  echo \"[Lazarus] Resource missing. Attempting Hard Resurrection from Snapshot...\"\n")
			
			fmt.Fprintf(f, "  SNAP_ID=$(aws ec2 describe-snapshots --filters Name=description,Values='*%s*' --query 'Snapshots[0].SnapshotId' --output text)\n", resourceID)
			
			fmt.Fprintf(f, "  if [ \"$SNAP_ID\" != \"None\" ] && [ \"$SNAP_ID\" != \"\" ]; then\n")
			fmt.Fprintf(f, "    NEW_VOL_ID=$(aws ec2 create-volume --snapshot-id $SNAP_ID --availability-zone %s --query 'VolumeId' --output text)\n", region)
			fmt.Fprintf(f, "    echo \"    Resurrected Volume as $NEW_VOL_ID\"\n")
			
			// Wait for availability.
			fmt.Fprintf(f, "    echo -n \"    Waiting for volume available...\"\n")
			fmt.Fprintf(f, "    count=0; until aws ec2 describe-volumes --volume-ids \"$NEW_VOL_ID\" --query 'Volumes[0].State' --output text | grep -q 'available'; do\n")
			fmt.Fprintf(f, "      if [ $count -ge 60 ]; then echo \" [TIMEOUT]\"; echo \"[ERROR] Timed out waiting for volume.\"; exit 1; fi\n")
			fmt.Fprintf(f, "      echo -n \".\"; sleep 5; ((count++))\n")
			fmt.Fprintf(f, "    done; echo \" [OK]\"\n")

			// Attach volume.
			if atts, ok := node.Properties["Attachments"].([]interface{}); ok && len(atts) > 0 {
				if att, ok := atts[0].(map[string]interface{}); ok {
					instanceID := att["InstanceId"].(string)
					device := att["Device"].(string)
					fmt.Fprintf(f, "    aws ec2 attach-volume --volume-id $NEW_VOL_ID --instance-id %s --device %s\n", instanceID, device)
				}
			}

			// Update Terraform state.
			if tfAddress, ok := node.Properties["TF_ADDRESS"].(string); ok {
				fmt.Fprintf(f, "    tf_safe_import %s \"$NEW_VOL_ID\"\n", shellEscape(tfAddress))
			}

			fmt.Fprintf(f, "  else\n")
			fmt.Fprintf(f, "    echo \"[FATAL] No snapshot found. Data is lost.\"\n")
			fmt.Fprintf(f, "  fi\n")
			fmt.Fprintf(f, "fi\n\n")
			wasteCount++

		case "AWS::RDS::DBInstance":
			fmt.Fprintf(f, "echo \"Resurrecting RDS: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws rds start-db-instance --db-instance-identifier %s\n", shellEscape(resourceID))
			fmt.Fprintf(f, "aws rds remove-tags-from-resource --resource-name %s --tag-keys CloudSlash:Status CloudSlash:ExpiryDate\n\n", shellEscape(node.ID))
			wasteCount++
		}

		// Audit: Archive tombstone.
		fmt.Fprintf(f, "# Audit Trail\n")
		fmt.Fprintf(f, "mkdir -p .cloudslash/history/restored\n")
		// Check if file exists to avoid error spam
		fmt.Fprintf(f, "if [ -f \".cloudslash/tombstones/%s.json\" ]; then\n", resourceID)
		fmt.Fprintf(f, "  mv \".cloudslash/tombstones/%s.json\" \".cloudslash/history/restored/%s_$(date +%%s).json\"\n", resourceID, resourceID)
		fmt.Fprintf(f, "  echo \"  [AUDIT] Tombstone archived.\"\n")
		fmt.Fprintf(f, "fi\n\n")
	}
	
	if wasteCount == 0 {
		fmt.Fprintf(f, "echo \"No resources to resurrect.\"\n")
	}

	return nil
}

// GenerateIgnoreScript tags resources to suppress future reports.
func (g *Generator) GenerateIgnoreScript(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash Ignore Tagging Script\n")
	fmt.Fprintf(f, "# Generated: %s\n\n", time.Now().Format(time.RFC3339))

	fmt.Fprintf(f, "set -e\n\n")

	fmt.Fprintf(f, "set -e\n\n")

	// Sort for deterministic output.
	type wasteItem struct {
		ID   string
		Type string
	}
	var items []wasteItem
 
	for _, node := range g.Graph.Nodes {
		if node.IsWaste && !node.Justified {
			items = append(items, wasteItem{ID: node.ID, Type: node.Type})
		}
	}

	// Sort by ID
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	count := 0
	for _, item := range items {
		resourceID := extractResourceID(item.ID)

		arg := item.ID
		if !strings.HasPrefix(item.ID, "arn:") {
			fmt.Fprintf(f, "# Skipping non-ARN resource: %s\n", item.ID)
			continue
		}

		fmt.Fprintf(f, "echo \"Ignoring: %s\"\n", resourceID)
		fmt.Fprintf(f, "aws resourcegroupstaggingapi tag-resources --resource-arn-list %s --tags cloudslash:ignore=true\n", shellEscape(arg))
		count++
	}

	if count == 0 {
		fmt.Fprintf(f, "echo \"No waste found to ignore.\"\n")
	} else {
		fmt.Fprintf(f, "echo \"Ignore Tagging Complete. %d resources tagged.\"\n", count)
	}

	return nil
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

	// Return original ID if not an ARN.
	return id
}

// shellEscape quotes strings for bash.
func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	// Replace ' with '\''
	val := strings.ReplaceAll(s, "'", "'\\''")
	return fmt.Sprintf("'%s'", val)
}
