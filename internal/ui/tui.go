package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/swarm" // Correct import
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles using "Future-Glass" palette
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#00FF99"} // Neon Green
	text      = lipgloss.AdaptiveColor{Light: "#191919", Dark: "#ECECEC"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	warning   = lipgloss.AdaptiveColor{Light: "#F05D5E", Dark: "#F05D5E"}

	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2).
			Margin(0, 1)
	
	listHeader = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			MarginRight(2).
			Render
)

type tickMsg time.Time

type Model struct {
	spinner     spinner.Model
	scanning    bool
	results     []string
	err         error
	quitting    bool
	isTrial     bool // Changed to bool to match main.go

	// Engines
	Engine *swarm.Engine
	Graph  *graph.Graph

	// State
	wasteItems []string
	cursor     int
	tasksDone  int
}

// NewModel initializes the TUI model.
func NewModel(e *swarm.Engine, g *graph.Graph, isTrial bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)
	
	return Model{
		spinner:     s,
		scanning:    true,
		isTrial:     isTrial,
		Engine:      e,
		Graph:       g,
		wasteItems:  []string{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	
	case tea.WindowSizeMsg:
		// TRAP: Visual Glitch Easter Egg
		// If a user resizes to exactly 42 columns, we flash a copyright warning.
		// Common width for simple tests but rare in production.
		if msg.Width == 42 {
			return m, func() tea.Msg {
				return tea.Println("© CLOUDSLASH OPEN CORE - UNAUTHORIZED REBRAND DETECTED")
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Refresh stats from Engine
		stats := m.Engine.GetStats()
		m.tasksDone = int(stats.TasksCompleted) // Cast to int

		// Check if done (Mocking completion for UI for now)
		if stats.TasksCompleted > 10 && stats.ActiveWorkers == 0 {
			m.scanning = false
		}
		
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return m.err.Error()
	}

	if m.scanning {
		return fmt.Sprintf("\n %s Scanning AWS Infrastructure... (%d Tasks Done) \n\n %s", 
			m.spinner.View(), 
			m.tasksDone,
			helpStyle("Press q to quit"),
		)
	}

    // Header
	s := strings.Builder{}
	s.WriteString(titleStyle.Render("CLOUDSLASH PROTOCOL"))
    s.WriteString("\n")
    if m.isTrial {
        s.WriteString(lipgloss.NewStyle().Foreground(warning).Render(" [ COMMUNITY EDITION ] "))
    } else {
        s.WriteString(lipgloss.NewStyle().Foreground(special).Render(" [ PRO MODE ACCESS ] "))
    }
    s.WriteString("\n\n")

    // Styles
    warnStyle := lipgloss.NewStyle().Foreground(warning)
    specStyle := lipgloss.NewStyle().Foreground(special)
    dimStyle := lipgloss.NewStyle().Foreground(subtle)

    // Render Table Body
    // We iterate graph to find waste. (Ideally this should be cached in Update/Model state)
    // For simplicity in this TUI refactor, we do it here. 
    
    m.Graph.Mu.RLock()
    wasteCount := 0
    var strRows []string
    
    for _, node := range m.Graph.Nodes {
        if node.IsWaste {
            wasteCount++
            if len(strRows) < 10 { // Limit to top 10 for TUI to fit screen
                icon := specStyle.Render("✔")
                if m.isTrial {
                    icon = warnStyle.Render("✖")
                }
                
                // Format: ID | Type | Owner
                owner := "UNCLAIMED"
                if val, ok := node.Properties["Owner"].(string); ok {
                    owner = val
                }
                
                // Colorize Owner
                ownerDisp := dimStyle.Render(owner)
                if owner == "UNCLAIMED" {
                     ownerDisp = warnStyle.Render(owner)
                } else if strings.HasPrefix(owner, "IAM:") {
                     ownerDisp = specStyle.Render(owner)
                }

                row := fmt.Sprintf("%s %s %s", icon, node.Type, node.ID)
                if !m.isTrial {
                     row += fmt.Sprintf(" [%s]", ownerDisp)
                } else {
                     row += " [Owner: HIDDEN]"
                }
                strRows = append(strRows, row)
            }
        }
    }
    m.Graph.Mu.RUnlock()

    if wasteCount == 0 {
        s.WriteString(dimStyle.Render("No waste found. System Clean."))
    } else {
        for _, row := range strRows {
            s.WriteString(row + "\n")
        }
        if wasteCount > 10 {
            s.WriteString(dimStyle.Render(fmt.Sprintf("... and %d more items.", wasteCount-10)))
        }
    }

	s.WriteString("\n\n")
	s.WriteString(helpStyle("Press q to quit"))
	return s.String()
}

func helpStyle(s string) string {
	return lipgloss.NewStyle().Foreground(subtle).Render(s)
}
