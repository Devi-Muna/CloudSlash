package tui

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

	// Header Display: Type and ID.
	header := detailsHeaderStyle.Render(fmt.Sprintf("%s : %s", node.TypeStr(), node.IDStr()))

	// Properties List.
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

	// Resource Intelligence: Cost, Risk, and Reachability.
	cost := fmt.Sprintf("MONTHLY WASTE: $%.2f", node.Cost)
	risk := fmt.Sprintf("RISK SCORE:    %d/100", node.RiskScore)

	reach := "REACHABILITY:  Unknown"
	reachStyle := dimStyle
	if node.Reachability == "Reachable" {
		reach = "REACHABILITY:  connected"
		reachStyle = special // Green
	} else if node.Reachability == "DarkMatter" {
		reach = "REACHABILITY:  DARK MATTER"
		reachStyle = danger // Red
	}

	cpuHistory, _ := node.Properties["MetricsHistoryCPU"].([]float64)
	netHistory, _ := node.Properties["MetricsHistoryNet"].([]float64)

	cpuSpark := renderSparkline(cpuHistory)
	netSpark := renderSparkline(netHistory)

	intelBlock := lipgloss.JoinVertical(lipgloss.Left,
		special.Render(cost),
		danger.Render(risk),
		reachStyle.Render(reach),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(fmt.Sprintf("CPU ACTIVITY:  %s", cpuSpark)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#7D40FF")).Render(fmt.Sprintf("NET ACTIVITY:  %s", netSpark)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F05D5E")).Render("BLAME:         "+fmt.Sprintf("%v", node.Properties["Owner"])),
	)

	// IAC Provenance.
	source := "Source: Unknown (Not managed by Terraform)"
	if node.SourceLocation != "" {
		source = fmt.Sprintf("Source: %s", node.SourceLocation)
	}

	// Contextual Actions.
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
		if v > max {
			max = v
		}
	}

	var s strings.Builder
	s.WriteString("[")
	for _, v := range data {
		if max == 0 {
			s.WriteString(bars[0])
			continue
		}
		idx := int((v / max) * float64(len(bars)-1))
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		s.WriteString(bars[idx])
	}
	s.WriteString("]")
	return s.String()
}
