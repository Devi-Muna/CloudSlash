package forensics

import (
	"context"
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/aws"
	"github.com/DrSkyle/cloudslash/internal/graph"
)

type Detective struct {
	CT *aws.CloudTrailClient
}

func NewDetective(ct *aws.CloudTrailClient) *Detective {
	return &Detective{CT: ct}
}

// IdentifyOwner tries to find the owner of a node.
// Strategy:
// 1. Check Tags (Owner, CreatedBy, Creator, Contact, User).
// 2. Check CloudTrail (if initialized).
// 3. Return "UNCLAIMED".
func (d *Detective) IdentifyOwner(ctx context.Context, node *graph.Node) string {
	// 1. Tag Check
	if node.Properties != nil {
		tags := []string{"Owner", "owner", "CreatedBy", "created_by", "Creator", "creator", "Contact", "contact", "User", "user"}
		for _, t := range tags {
			if val, ok := node.Properties[t].(string); ok {
				return fmt.Sprintf("Tag:%s", val)
			}
			// Sometimes properties are flattened "Tags.Owner" or just "Owner" depending on scanner
			// Assuming Scanner puts tags in Properties map directly or under "Tags" map
			if tagMap, ok := node.Properties["Tags"].(map[string]string); ok {
				if val, ok := tagMap[t]; ok {
					return fmt.Sprintf("Tag:%s", val)
				}
			}
		}
	}

	// 2. CloudTrail Check
	if d.CT != nil {
		// Extract Resource ID (strip ARN if needed)
		resourceID := node.ID
		// Basic ARN stripping logic

		// Robust ARN stripping logic
		// Pattern 1: Slash separated (arn:aws:ec2:region:account:volume/vol-123)
		if strings.Contains(resourceID, "/") {
			parts := strings.Split(resourceID, "/")
			resourceID = parts[len(parts)-1]
		} else if strings.Count(resourceID, ":") >= 5 {
			// Pattern 2: Colon separated (arn:aws:s3:::bucket-name OR arn:aws:sns:region:acc:topic)
			parts := strings.Split(resourceID, ":")
			resourceID = parts[len(parts)-1]
		}

		user, err := d.CT.LookupCreator(ctx, resourceID)
		if err == nil {
			return fmt.Sprintf("IAM:%s", user)
		}
	}

	return "UNCLAIMED"
}

// InvestigateGraph iterates over waste nodes and populates "Owner" property.
func (d *Detective) InvestigateGraph(ctx context.Context, g *graph.Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.IsWaste {
			owner := d.IdentifyOwner(ctx, node)
			if node.Properties == nil {
				node.Properties = make(map[string]interface{})
			}
			node.Properties["Owner"] = owner
		}
	}
}
