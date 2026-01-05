package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

func (m Model) viewList() string {
	s := strings.Builder{}

	// Sort and filter roots
	m.Graph.Mu.RLock()
	var nodes []*graph.Node
	for _, n := range m.Graph.Nodes {
		if n.IsWaste && !n.Ignored {
			nodes = append(nodes, n)
		}
	}
	// Sort by Cost Descending
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Cost > nodes[j].Cost
	})
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
	s.WriteString(dimStyle.Render(fmt.Sprintf("   %-20s | %-15s | %-10s | %s\n", "RESOURCE ID", "TYPE", "COST", "REASON")))
	s.WriteString(dimStyle.Render("   " + strings.Repeat("─", 60) + "\n"))

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
		dispCost := fmt.Sprintf("$%.2f", node.Cost)

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
