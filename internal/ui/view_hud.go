package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewHUD() string {
	// 1. Status Indicator
	status := "IDLE"
	statusColor := subtle
	if m.scanning {
		status = "SCANNING"
		statusColor = special
	}
	
	// Animate dots if scanning
	if m.scanning {
		dots := strings.Repeat(".", m.tickCount%4)
		status = fmt.Sprintf("SCANNING%s", dots)
	}

	// 2. Waste Ticker
	savings := fmt.Sprintf("$%.2f/mo", m.totalSavings)
	if m.totalSavings > 0 {
		savings += fmt.Sprintf(" ($%.0f/yr)", m.totalSavings*12)
	}
	
	// 3. Risk Score (Mock logic for now, or derived from graph)
	riskLevel := "LOW"
	riskColor := subtle
	if m.totalSavings > 100 {
		riskLevel = "MODERATE"
		riskColor = warning
	}
	if m.totalSavings > 1000 {
		riskLevel = "CRITICAL"
		riskColor = danger
	}

	// Assemble Segments
	// [ CLOUDSLASH v1.3.2 ] [ TASKS: 12/40 ] [ WASTE: $... ] [ RISK: ... ]
	
	segTitle := highlight.Render("CLOUDSLASH v1.3.2")
	segStatus := statusColor.Render(fmt.Sprintf("[ STATUS: %-10s ]", status))
	segWaste := hudLabelStyle.Render("WASTE:") + hudValueStyle.Render(savings)
	segRisk := hudLabelStyle.Render("RISK:") + riskColor.Render(riskLevel)

	// Spacer
	width := m.width - 4 // border padding
	if width < 0 { width = 0 }
	
	// Simple Layout: Left aligned for now to be safe, or flex
	// Left: Title + Status
	// Right: Waste + Risk
	// For TUI, simpler is often better. Let's do a join.
	
	// Using lipgloss for layout
	left := lipgloss.JoinHorizontal(lipgloss.Center, segTitle, "  ", segStatus)
	right := lipgloss.JoinHorizontal(lipgloss.Center, segWaste, "  |  ", segRisk)
	
	content := lipgloss.JoinHorizontal(lipgloss.Top, 
		left,
		lipgloss.NewStyle().Width(width - lipgloss.Width(left) - lipgloss.Width(right)).Render(""), // Spacer
		right,
	)

	return hudStyle.Width(m.width - 2).Render(content)
}
