package forensics

import (
	"context"
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type Detective struct {
	CT *aws.CloudTrailClient
}

func NewDetective(ct *aws.CloudTrailClient) *Detective {
	return &Detective{CT: ct}
}

// IdentifyOwner resolves resource ownership.
// Strategy: Tags > CloudTrail.
func (d *Detective) IdentifyOwner(ctx context.Context, node *graph.Node) string {
	// Check ownership tags.
	if node.Properties != nil {
		tags := []string{"Owner", "owner", "CreatedBy", "created_by", "Creator", "creator", "Contact", "contact", "User", "user"}
		for _, t := range tags {
			if val, ok := node.Properties[t].(string); ok {
				return fmt.Sprintf("Tag:%s", val)
			}
			if tagMap, ok := node.Properties["Tags"].(map[string]string); ok {
				if val, ok := tagMap[t]; ok {
					return fmt.Sprintf("Tag:%s", val)
				}
			}
		}
	}

	// Lookup CloudTrail creator.
	if d.CT != nil {
		// Extract Resource ID.
		resourceID := node.ID
		
		// Parse ARN patterns.
		if strings.Contains(resourceID, "/") {
			parts := strings.Split(resourceID, "/")
			resourceID = parts[len(parts)-1]
		} else if strings.Count(resourceID, ":") >= 5 {
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

// InvestigateGraph enriches waste nodes with ownership data.
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
