package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewList() string {
	s := strings.Builder{}

	// Sort and filter roots
	var nodes []*graph.Node
	for _, n := range m.Graph.Nodes {
		if n.IsWaste && !n.Ignored {
			// Filtering Logic
			if m.FilterMode == "Easy" {
				// Easy Wins: Unattached EIPs, Snapshots, Low Risk
				isEasy := false
				if n.Type == "AWS::EC2::EIP" || n.Type == "AWS::EC2::Snapshot" {
					isEasy = true
				}
				if n.RiskScore < 20 {
					isEasy = true
				}
				if !isEasy {
					continue
				}
			} else if m.FilterMode != "" {
				// Region Filter
				if r, ok := n.Properties["Region"].(string); !ok || r != m.FilterMode {
					continue
				}
			}
			nodes = append(nodes, n)
		}
	}
	
	// Sorting Logic
	if m.SortMode == "Price" {
		// Sort by Cost Descending
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Cost > nodes[j].Cost
		})
	} else {
		// Default: ID
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
	}
	m.Graph.Mu.RUnlock()

	// Update local cache for cursor mapping
	// (Note: In a real efficient app, we wouldn't re-sort every frame, but for <1000 items it's fine)
	m.wasteItems = nodes 

	if len(nodes) == 0 {
		if m.scanning {
			return fmt.Sprintf("\n\n   %s Initializing Scan Protocol...", m.spinner.View())
		}
		return "\n\n   " + iconSafe.Render() + subtle.Render("  System Clean. No inefficiencies detected.")
	}

	// Pagination / Windowing
	start, end := m.calculateWindow(len(nodes))

	// Header
	headerTxt := fmt.Sprintf("   %-20s | %-15s | %-10s | %s", "RESOURCE ID", "TYPE", "COST", "REASON")
	s.WriteString(dimStyle.Render(headerTxt) + "\n")
	
	filterStatus := ""
	if m.SortMode != "" { filterStatus += fmt.Sprintf(" [SORT: %s]", m.SortMode) }
	if m.FilterMode != "" { filterStatus += fmt.Sprintf(" [FILTER: %s]", m.FilterMode) }
	if filterStatus != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("   " + filterStatus) + "\n")
	} else {
		s.WriteString(dimStyle.Render("   " + strings.Repeat("─", 60) + "\n"))
	}

	for i := start; i < end; i++ {
		node := nodes[i]
		isSelected := (i == m.cursor)
		
		// Icon based on cost/risk
		icon := "[ ]"
		if node.Cost > 50 { icon = "[!]" }
		if node.RiskScore > 80 { icon = "[x]" }

		// Truncate ID if too long
		dispID := node.ID
		if len(dispID) > 20 { dispID = dispID[:17] + "..." }

		// Type shortener
		dispType := node.Type
		dispType = strings.TrimPrefix(dispType, "AWS::")
		if len(dispType) > 15 { dispType = dispType[:15] }

		// Cost
		// Cost & Guilt Trip
		dispCost := fmt.Sprintf("$%.2f", node.Cost)
		if node.Cost > 0 {
			yearly := node.Cost * 12
			dispCost += lipgloss.NewStyle().Foreground(lipgloss.Color("#F05D5E")).Render(fmt.Sprintf(" ($%.0f/yr)", yearly))
		}

		// Reason (cut off rest)
		reason := fmt.Sprintf("%v", node.Properties["Reason"])
		if len(reason) > 40 { reason = reason[:37] + "..." }

		line := fmt.Sprintf("%s %-20s | %-15s | %-10s | %s", icon, dispID, dispType, dispCost, reason)

		if isSelected {
			s.WriteString(listSelectedStyle.Render(line) + "\n")
		} else {
			s.WriteString(listNormalStyle.Render(line) + "\n")
		}
	}

	return s.String()
}

func (m Model) calculateWindow(total int) (int, int) {
	windowSize := m.height - 8 // approx HUD + footer
	if windowSize < 5 { windowSize = 5 }

	start := m.cursor - (windowSize / 2)
	if start < 0 { start = 0 }
	
	end := start + windowSize
	if end > total {
		end = total
		start = end - windowSize
		if start < 0 { start = 0 }
	}
	return start, end
}
