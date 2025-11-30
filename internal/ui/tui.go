package ui

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/saujanyayaya/cloudslash/internal/graph"
	"github.com/saujanyayaya/cloudslash/internal/swarm"
)

type tickMsg time.Time

type Model struct {
	Engine *swarm.Engine
	Graph  *graph.Graph

	// State
	activeWorkers int
	concurrency   int
	tasksDone     int64
	wasteItems    []*graph.Node
	totalWaste    int
	cursor        int
	showDetail    bool
	isTrial       bool

	// Styles
	styleTitle    lipgloss.Style
	styleStats    lipgloss.Style
	styleBounty   lipgloss.Style
	styleSelected lipgloss.Style
	styleDetail   lipgloss.Style
}

func NewModel(e *swarm.Engine, g *graph.Graph, isTrial bool) Model {
	return Model{
		Engine:  e,
		Graph:   g,
		isTrial: isTrial,
		styleTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00")).
			Border(lipgloss.DoubleBorder()).
			Padding(0, 1),
		styleStats: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")),
		styleBounty: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")),
		styleSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Bold(true),
		styleDetail: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Foreground(lipgloss.Color("#00FF00")),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.wasteItems)-1 {
				m.cursor++
			}
		case "enter":
			m.showDetail = !m.showDetail
		}
	case tickMsg:
		// If showing details, "lock" the UI. Don't update the list.
		if m.showDetail {
			return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg(t)
			})
		}

		// Update stats
		stats := m.Engine.GetStats()
		m.activeWorkers = stats.ActiveWorkers
		m.concurrency = stats.Concurrency
		m.tasksDone = stats.TasksCompleted

		// Update waste items (simplified, just grabbing top 5 or so)
		m.Graph.Mu.RLock()
		var waste []*graph.Node
		count := 0
		for _, node := range m.Graph.Nodes {
			if node.IsWaste {
				waste = append(waste, node)
				count++
			}
		}
		m.Graph.Mu.RUnlock()

		// Sort waste items to ensure stable display (prevent flickering)
		sort.SliceStable(waste, func(i, j int) bool {
			// Sort by RiskScore (descending)
			if waste[i].RiskScore != waste[j].RiskScore {
				return waste[i].RiskScore > waste[j].RiskScore
			}
			// Then by ID (ascending)
			return waste[i].ID < waste[j].ID
		})

		// Only update if changed (simple check: length or top item)
		// For a perfect check, we'd compare all IDs, but this helps stability
		if len(m.wasteItems) != len(waste) || (len(waste) > 0 && len(m.wasteItems) > 0 && m.wasteItems[0].ID != waste[0].ID) {
			m.wasteItems = waste
			m.totalWaste = count
		} else if len(m.wasteItems) == 0 && len(waste) > 0 {
			m.wasteItems = waste
			m.totalWaste = count
		}

		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return m, nil
}

func (m Model) View() string {
	title := m.styleTitle.Render("CLOUDSLASH v1.0")

	stats := fmt.Sprintf(
		"Active Threads: %d / %d | Tasks Completed: %d",
		m.activeWorkers, m.concurrency, m.tasksDone,
	)

	// Speedometer visual
	barWidth := 50
	filled := 0
	if m.concurrency > 0 {
		filled = (m.activeWorkers * barWidth) / m.concurrency
	}
	if filled > barWidth {
		filled = barWidth
	}

	bar := "["
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "#"
		} else {
			bar += "."
		}
	}
	bar += "]"

	bounties := "BOUNTIES FOUND (Press Enter for Details):\n"
	for i, node := range m.wasteItems {
		// Only show a window of items if list is long (simplified scrolling)
		if i < m.cursor-5 || i > m.cursor+5 {
			continue
		}

		id := node.ID
		if m.isTrial {
			id = redactID(id)
		}

		line := fmt.Sprintf("- [%d] %s (%s)", node.RiskScore, id, node.Type)
		if i == m.cursor {
			line = m.styleSelected.Render("> " + line)
		} else {
			line = "  " + line
		}
		bounties += line + "\n"
	}

	detail := ""
	if m.showDetail && len(m.wasteItems) > 0 {
		selected := m.wasteItems[m.cursor]

		id := selected.ID
		if m.isTrial {
			id = redactID(id)
		}

		content := fmt.Sprintf("ID: %s\nType: %s\nRisk Score: %d\n", id, selected.Type, selected.RiskScore)

		if m.isTrial {
			content += "\n[LOCKED] Buy license to view properties and fix.\n"
		} else {
			for k, v := range selected.Properties {
				content += fmt.Sprintf("%s: %v\n", k, v)
			}
		}
		detail = m.styleDetail.Render(content)
	}

	return fmt.Sprintf(
		"%s\n\n%s\n%s\n\n%s\n%s\n\nTotal Waste Items: %d\n\nPress 'q' to quit.",
		title,
		m.styleStats.Render(stats),
		m.styleStats.Render(bar),
		m.styleBounty.Render(bounties),
		detail,
		m.totalWaste,
	)
}

func redactID(id string) string {
	// Simple redaction: keep first 4 chars, mask rest
	if len(id) > 8 {
		return id[:4] + "..." + id[len(id)-4:]
	}
	return "****"
}
