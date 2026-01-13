package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Future-Glass Palette
	colorNeonGreen  = lipgloss.Color("#00FF99") // Main Action / Success
	colorNeonPurple = lipgloss.Color("#874BFD") // Header / Border
	colorDarkGray   = lipgloss.Color("#1E293B") // Background Elements
	colorTextMain   = lipgloss.Color("#E2E8F0") // Main Text
	colorTextSub    = lipgloss.Color("#64748B") // Subtext
	colorDanger     = lipgloss.Color("#FF0055") // Critical / Delete
	colorWarning    = lipgloss.Color("#F59E0B") // Warning

	// Shared Styles
	subtle    = lipgloss.NewStyle().Foreground(colorTextSub)
	dimStyle  = lipgloss.NewStyle().Foreground(colorTextSub) // Alias for subtle text
	highlight = lipgloss.NewStyle().Foreground(colorNeonPurple).Bold(true)
	special   = lipgloss.NewStyle().Foreground(colorNeonGreen).Bold(true)
	danger    = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	warning   = lipgloss.NewStyle().Foreground(colorWarning)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorNeonPurple).
			Bold(true).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorTextSub). // Fixed: Use color, not style
			Padding(1, 2).
			Margin(0, 1)

	// HUD Styles
	hudStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorNeonPurple).
			Padding(0, 1).
			Foreground(colorTextMain)

	hudLabelStyle = lipgloss.NewStyle().
			Foreground(colorTextSub).
			Bold(true).
			MarginRight(1)
	
	hudValueStyle = lipgloss.NewStyle().
			Foreground(colorNeonGreen).
			Bold(true)

	// List Styles
	listSelectedStyle = lipgloss.NewStyle().
			Foreground(colorTextMain).
			Background(lipgloss.Color("#331832")). // Very subtle purple bg
			Bold(true).
			PaddingLeft(0) // Controlled manually in view (`> `)

	listNormalStyle = lipgloss.NewStyle().
			Foreground(colorTextSub).
			PaddingLeft(0) // Controlled manually in view (`  `)

	// Icon Styles (Text Based - No Emojis)
	iconCritical = lipgloss.NewStyle().Foreground(colorDanger).SetString("[CRITICAL]")
	iconWarn     = lipgloss.NewStyle().Foreground(colorWarning).SetString("[WARN]")
	iconSafe     = lipgloss.NewStyle().Foreground(colorNeonGreen).SetString("[SAFE]")
	iconInfo     = lipgloss.NewStyle().Foreground(colorNeonPurple).SetString("[INFO]")

	// Details Pane
	detailsBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorNeonGreen).
			Padding(1, 2).
			MarginTop(1)

	detailsHeaderStyle = lipgloss.NewStyle().
				Foreground(colorNeonPurple).
				Bold(true).
				Underline(true).
				MarginBottom(1)
)
