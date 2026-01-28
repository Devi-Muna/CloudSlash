package tui

import (
	"fmt"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewList() string {
	s := strings.Builder{}

	// cache usage
	nodes := m.wasteItems 

	if len(nodes) == 0 {
		if m.scanning {
			return fmt.Sprintf("\n\n   %s Initializing Scan Protocol...", m.spinner.View())
		}
		return "\n\n   " + iconSafe.Render() + subtle.Render("  System Clean. No inefficiencies detected.")
	}

	// Pagination / Windowing
	start, end := m.calculateWindow(len(nodes))

	// Header
	// Items have PaddingLeft(2).
	// Selection Indicator is 2 chars: "> " or "  ".
	// ID starts after that. 
	// Let's standardise: 
	//  State | ID | Type | Cost | Reason
	headerTxt := fmt.Sprintf("  %-20s | %-15s | %-10s | %s", "RESOURCE ID", "TYPE", "COST", "REASON") 
	s.WriteString(dimStyle.Render(headerTxt) + "\n")
	
	filterStatus := ""
	if m.SortMode != "" { filterStatus += fmt.Sprintf(" [SORT: %s]", m.SortMode) }
	if m.FilterMode != "" { filterStatus += fmt.Sprintf(" [FILTER: %s]", m.FilterMode) }
	if filterStatus != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("   " + filterStatus) + "\n")
	} else {
		s.WriteString(dimStyle.Render("  " + strings.Repeat("─", 60)) + "\n")
	}

	for i := start; i < end; i++ {
		node := nodes[i]
		isSelected := (i == m.cursor)
		
		// Selector
		cursor := "  "
		if isSelected {
			cursor = "> "
		}

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
			// yearly := node.Cost * 12
			// Removed yearly clutter to simplify
		}

		// Reason (cut off rest)
		reason := fmt.Sprintf("%v", node.Properties["Reason"])
		if len(reason) > 40 { reason = reason[:37] + "..." }

		// Severity Coloring
		var line string
		baseLine := fmt.Sprintf("%-20s | %-15s | %-10s | %s", dispID, dispType, dispCost, reason)
		
		if node.RiskScore > 80 {
			// Critical Risk
			baseLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0055")).Render(baseLine)
		} else if node.Cost > 50 {
			// High Cost
			baseLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(baseLine)
		}

		line = cursor + baseLine

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
