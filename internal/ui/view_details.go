package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewDetails() string {
	if m.cursor < 0 || m.cursor >= len(m.wasteItems) {
		return "No Item Selected"
	}
	node := m.wasteItems[m.cursor]

	// 1. Header
	header := detailsHeaderStyle.Render(fmt.Sprintf("%s : %s", node.Type, node.ID))
	
	// 2. Properties List
	var props []string
	// Sort keys for deterministic display
	var keys []string
	for k := range node.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := fmt.Sprintf("%v", node.Properties[k])
		// Basic formatting
		line := fmt.Sprintf("%-20s : %s", k, val)
		props = append(props, line)
	}
	
	// 3. Cost & Risk Section (Simulated "Intel")
	cost := fmt.Sprintf("MONTHLY WASTE: $%.2f", node.Cost)
	risk := fmt.Sprintf("RISK SCORE:    %d/100", node.RiskScore)
	
	intelBlock := lipgloss.JoinVertical(lipgloss.Left, 
		special.Render(cost),
		danger.Render(risk),
	)

	// 4. Source Location (if available)
	source := "Source: Unknown (Not managed by Terraform)"
	if node.SourceLocation != "" {
		source = fmt.Sprintf("Source: %s", node.SourceLocation)
	}

	// 5. Actions Footer
	actions := []string{
		"[I]gnore Resource",
		"[O]pen in Console",
		"[B]ack to List",
	}
	actionLine := strings.Join(actions, "  ")

	// Assemble
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		intelBlock,
		"",
		dimStyle.Render(strings.Join(props, "\n")),
		"",
		subtle.Render(source),
		"",
		strings.Repeat("─", 50),
		highlight.Render("ACTIONS:"), 
		actionLine,
	)

	return detailsBoxStyle.Render(content)
}
