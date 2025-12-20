package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// ExportItem matches the JSON/CSV structure.
type ExportItem struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	Reason         string  `json:"reason"`
	Cost           float64 `json:"monthly_cost"`
	RiskScore      int     `json:"risk_score"`
	SourceLocation string  `json:"source_location,omitempty"`
	Owner          string  `json:"owner,omitempty"`
	Region         string  `json:"region,omitempty"`
}

// GenerateCSV writes waste items to a CSV file.
func GenerateCSV(g *graph.Graph, path string) error {
	items := extractItems(g)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	header := []string{"Resource ID", "Type", "Reason", "Monthly Cost ($)", "Risk Score", "Source Code", "Owner", "Region"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, item := range items {
		record := []string{
			item.ID,
			item.Type,
			item.Reason,
			fmt.Sprintf("%.2f", item.Cost),
			fmt.Sprintf("%d", item.RiskScore),
			item.SourceLocation,
			item.Owner,
			item.Region,
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
			owner, _ := node.Properties["Owner"].(string)
			reason, _ := node.Properties["Reason"].(string)

			items = append(items, ExportItem{
				ID:             node.ID,
				Type:           node.Type,
				Reason:         reason,
				Cost:           node.Cost,
				RiskScore:      node.RiskScore,
				SourceLocation: node.SourceLocation,
				Owner:          owner,
				Region:         region,
			})
		}
	}
	return items
}
