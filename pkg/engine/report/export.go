package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

// ExportItem matches the JSON/CSV structure.
type ExportItem struct {
	ResourceID       string  `json:"resource_id"`
	Type             string  `json:"type"`
	Region           string  `json:"region"`
	NameTag          string  `json:"name_tag"`
	MonthlyCost      float64 `json:"monthly_cost"`
	RiskScore        int     `json:"risk_score"`
	AuditDetail      string  `json:"audit_detail"`
	OwnerARN         string  `json:"owner_arn"`
	Action           string  `json:"action"`
}

// GenerateCSV writes waste items to a CSV file.
func GenerateCSV(g *graph.Graph, path string) error {
	items := extractItems(g)

	// Sort by Cost Descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].MonthlyCost > items[j].MonthlyCost
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// SSSS+ Header
	header := []string{
		"ResourceID",
		"Type",
		"Region",
		"NameTag",
		"MonthlyCost",
		"RiskScore",
		"AuditDetail",
		"OwnerARN",
		"Action",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, item := range items {
		record := []string{
			item.ResourceID,
			item.Type,
			item.Region,
			item.NameTag,
			fmt.Sprintf("$%.2f", item.MonthlyCost),
			fmt.Sprintf("%d", item.RiskScore),
			item.AuditDetail,
			item.OwnerARN,
			item.Action,
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// GenerateJSON writes certain waste items to a JSON file.
func GenerateJSON(g *graph.Graph, path string) error {
	items := extractItems(g)

	// Sort by Cost Descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].MonthlyCost > items[j].MonthlyCost
	})

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func extractItems(g *graph.Graph) []ExportItem {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	var items []ExportItem
	for _, node := range g.Nodes {
		if node.IsWaste {
			region, _ := node.Properties["Region"].(string)
			if region == "" {
				region = "global"
			}

			owner, _ := node.Properties["Owner"].(string)
			if owner == "" {
				owner = "Unknown"
			}

			reason, _ := node.Properties["Reason"].(string)
			
			// Extract Name Tag
			nameTag := ""
			if tags, ok := node.Properties["Tags"].(map[string]string); ok {
				if n, exists := tags["Name"]; exists {
					nameTag = n
				}
			}

			// Determine Action
			action := "DELETE"
			if node.RiskScore < 50 {
				action = "REVIEW"
			}
			if node.Justified {
				action = "JUSTIFIED"
			}

			items = append(items, ExportItem{
				ResourceID:       node.ID,
				Type:             node.Type,
				Region:           region,
				NameTag:          nameTag,
				MonthlyCost:      node.Cost,
				RiskScore:        node.RiskScore,
				AuditDetail:      reason,
				OwnerARN:         owner,
				Action:           action,
			})
		}
	}
	return items
}
