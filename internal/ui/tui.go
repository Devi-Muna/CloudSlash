package ui

import (
	"fmt"
	"strings"
	"time"
    "sort"

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
	wasteItems     []string
	cursor         int
	tasksDone      int
	showingDetails bool
}

// NewModel initializes the TUI model.
func NewModel(e *swarm.Engine, g *graph.Graph, isTrial bool, isMock bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)
	
	return Model{
		spinner:     s,
		scanning:    !isMock, // If mock, scan is already done in bootstrap
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

// Add sort import above. Note: imports will likely be managed by replace or auto-verify but let's be safe.
// Wait, I cannot change imports easily with partial edit if they are at the top.
// Sort helper required for ordering graph nodes properly.
// Actually, let's stick to the View/Update logic logic first.
// Update handles strict state transitions for the TUI.

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			// We need to know list length. 
			// For now, let's clamp at a reasonable max or safe logic
            // In View we calculate items. Ideally we cache it.
            // visual cursor limit handled in View partial logic or we let it go high and clamp in view
			m.cursor++
		case "enter", " ":
			m.showingDetails = !m.showingDetails
		}
	
	case tea.WindowSizeMsg:
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
		stats := m.Engine.GetStats()
		m.tasksDone = int(stats.TasksCompleted)
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

	s := strings.Builder{}
	s.WriteString(titleStyle.Render("CLOUDSLASH PROTOCOL"))
    s.WriteString("\n")
    if m.isTrial {
        s.WriteString(lipgloss.NewStyle().Foreground(warning).Render(" [ COMMUNITY EDITION ] "))
    } else {
        s.WriteString(lipgloss.NewStyle().Foreground(special).Render(" [ PRO MODE ACCESS ] "))
    }
    s.WriteString("\n\n")

    warnStyle := lipgloss.NewStyle().Foreground(warning)
    specStyle := lipgloss.NewStyle().Foreground(special)
    dimStyle := lipgloss.NewStyle().Foreground(subtle)
    cursorStyle := lipgloss.NewStyle().Foreground(highlight).Bold(true)

    // STABLE SORT & FILTER
    m.Graph.Mu.RLock()
    var items []*graph.Node
    for _, node := range m.Graph.Nodes {
        if node.IsWaste {
            items = append(items, node)
        }
    }
    m.Graph.Mu.RUnlock()
    
    // Sort by ID for stability
    sort.Slice(items, func(i, j int) bool {
        return items[i].ID < items[j].ID
    })

    // Clamp Cursor
    if m.cursor >= len(items) {
        m.cursor = len(items) - 1
    }
    if m.cursor < 0 {
        m.cursor = 0
    }

    if len(items) == 0 {
        s.WriteString(dimStyle.Render("No waste found. System Clean."))
    } else {
        // Pagination window (simple top 15 for now)
        
        // Header
        s.WriteString(dimStyle.Render(fmt.Sprintf("%-3s %-25s %-30s %-12s %s\n", "", "TYPE", "ID", "COST", "OWNER")))
        s.WriteString(dimStyle.Render(strings.Repeat("-", 80) + "\n"))

        for i := 0; i < len(items) && i < 15; i++ { // Show up to 15 items
            node := items[i]
            
            // Cursor
            gutter := "   "
            if i == m.cursor {
                gutter = cursorStyle.Render(" > ")
            }

            // Icon
            icon := specStyle.Render("✔")
            if m.isTrial { icon = warnStyle.Render("✖") }

             // Owner
            owner := "UNCLAIMED"
            if val, ok := node.Properties["Owner"].(string); ok { owner = val }
            
            ownerDisp := dimStyle.Render(owner)
            if owner == "UNCLAIMED" { 
                ownerDisp = warnStyle.Render(owner) 
            } else if strings.HasPrefix(owner, "IAM:") { 
                ownerDisp = specStyle.Render(owner) 
            }
            
            if m.isTrial { ownerDisp = "HIDDEN" }

            // Cost
            costStr := "-"
            if node.Cost > 0 {
                costStr = fmt.Sprintf("$%.2f", node.Cost)
            }
            costDisp := dimStyle.Render(fmt.Sprintf("%-12s", costStr))
            if node.Cost > 50 {
                costDisp = warnStyle.Render(fmt.Sprintf("%-12s", costStr))
            }

            // Row Render
            rowStr := fmt.Sprintf("%s %-25s %-30s %s %s", icon, node.Type, node.ID, costDisp, ownerDisp)
            if i == m.cursor {
                rowStr = cursorStyle.Render(fmt.Sprintf("%s %-25s %-30s %-12s %s", icon, node.Type, node.ID, costStr, ownerDisp))
            }
            
            s.WriteString(gutter + rowStr + "\n")
        }
        
        if len(items) > 15 {
            s.WriteString(dimStyle.Render(fmt.Sprintf("... and %d more items.", len(items)-15)))
        }

        // DETAIL VIEW OVERLAY
        if m.showingDetails && len(items) > 0 {
             // Safe clamp
             idx := m.cursor
             if idx >= len(items) { idx = len(items) - 1 }
             if idx < 0 { idx = 0 }
             
             node := items[idx]
             
             srcLocDisplay := ""
             if node.SourceLocation != "" {
                 srcLocDisplay = fmt.Sprintf("\nDefined in: %s", node.SourceLocation)
             }

             details := fmt.Sprintf(
                 "DETAILS FOR %s\n\nType:   %s\nRegion: %v\nOwner:  %v\nCost:   $%.2f/mo%s\n\n[DETECTED WASTE]\nReason: %v",
                 node.ID,
                 node.Type,
                 node.Properties["Region"],
                 node.Properties["Owner"],
                 node.Cost,
                 srcLocDisplay,
                 node.Properties["Reason"],
             )
             
             s.WriteString("\n\n")
             s.WriteString(cardStyle.Render(details))
        }
    }

	s.WriteString("\n\n")
    if m.showingDetails {
        s.WriteString(helpStyle("Enter: Close Details • q: Quit"))
    } else {
        s.WriteString(helpStyle("↑/↓: Navigate • Enter: Details • q: Quit"))
    }
	return s.String()
}

func helpStyle(s string) string {
	return lipgloss.NewStyle().Foreground(subtle).Render(s)
}
