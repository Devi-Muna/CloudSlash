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
	
	history := []float64{0, 0, 0, 0, 0, 0, 0} // Default flatline
	if h, ok := node.Properties["MetricsHistory"].([]float64); ok && len(h) > 0 {
		history = h
	}
	// Mock active for demo if missing
	if node.Cost > 50 && len(history) == 7 && history[0] == 0 {
		 history = []float64{0.1, 0.4, 0.2, 0.8, 0.5, 0.3, 0.1} // Mock spiky
	}

	sparkline := renderSparkline(history)
	
	intelBlock := lipgloss.JoinVertical(lipgloss.Left, 
		special.Render(cost),
		danger.Render(risk),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(fmt.Sprintf("ACTIVITY:      %s", sparkline)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F05D5E")).Render("BLAME:         "+fmt.Sprintf("%v", node.Properties["Owner"])),
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

func renderSparkline(data []float64) string {
	if len(data) == 0 {
		return "[NO DATA]"
	}
	bars := []string{" ", " ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	
	// Normalize
	max := 0.0
	for _, v := range data {
		if v > max { max = v }
	}
	
	var s strings.Builder
	s.WriteString("[")
	for _, v := range data {
		if max == 0 {
			s.WriteString(bars[0])
			continue
		}
		idx := int((v / max) * float64(len(bars)-1))
		if idx >= len(bars) { idx = len(bars) - 1 }
		s.WriteString(bars[idx])
	}
	s.WriteString("]")
	return s.String()
}
