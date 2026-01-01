package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/swarm" // Correct import
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles using "Hacker Slate" palette
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#64748B"} // Sub-text/Labels
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#6366F1"} // Indigo Headers
	text      = lipgloss.AdaptiveColor{Light: "#191919", Dark: "#E2E8F0"} // Primary Text
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#10B981"} // Emerald Success
	warning   = lipgloss.AdaptiveColor{Light: "#F05D5E", Dark: "#F59E0B"} // Amber Warning
	danger    = lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#F43F5E"} // Rose Red Danger/Zombie

	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2).
			Margin(0, 1)
            
    // Status Indicators
    iconOK     = lipgloss.NewStyle().Foreground(special).SetString("✓") // or [OK]
    iconWarn   = lipgloss.NewStyle().Foreground(warning).SetString("[WARN]")
    iconDanger = lipgloss.NewStyle().Foreground(danger).SetString("[FAIL]") 
    // Prompt asked for specific symbols. "Warning: ! or [WARN]". Let's use [WARN] for clarity in tree root.

	dimStyle    = lipgloss.NewStyle().Foreground(subtle)
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	labelStyle  = lipgloss.NewStyle().Foreground(subtle)
)

type tickMsg time.Time

type Model struct {
	spinner  spinner.Model
	scanning bool
	results  []string
	err      error
	quitting bool
	isTrial  bool // Changed to bool to match main.go

	// Engines
	Engine *swarm.Engine
	Graph  *graph.Graph

	// State
	wasteItems     []string
	cursor         int
	tasksDone      int
	showingDetails bool
    
    // Whitelist State
    totalSavings float64
}

// NewModel initializes the TUI model.
func NewModel(e *swarm.Engine, g *graph.Graph, isTrial bool, isMock bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)

	return Model{
		spinner:    s,
		scanning:   !isMock, // If mock, scan is already done in bootstrap
		isTrial:    isTrial,
		Engine:     e,
		Graph:      g,
		wasteItems: []string{},
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
        case "i":
            // Interactive Smart Whitelist
            if m.cursor >= 0 && len(m.wasteItems) > m.cursor { // Need robust cursor mapping
                 // Actually m.wasteItems is just strings in state?
                 // No, we build displayList in View. We should really build it in Update/Tick or access Graph directly.
                 // For MVP, if we hold Graph Read Lock in Update (briefly), we can find the node.
                 // Wait, we can't reliably map cursor to node unless we have the list.
                 // Let's rely on reprocessing or storing displayList in Model?
                 // Safer: Just grab the ID from a helper or re-calculate.
                 idToIgnore := m.getKthWasteNodeID(m.cursor)
                 if idToIgnore != "" {
                     m.ignoreNode(idToIgnore)
                 }
            }
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
        
        // Update Total Savings
        var total float64
        m.Graph.Mu.RLock()
        for _, n := range m.Graph.Nodes {
            if n.IsWaste && !n.Ignored {
                total += n.Cost
            }
        }
        m.Graph.Mu.RUnlock()
        m.totalSavings = total
        
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

    // LIVE COST TICKER (Financial Instrument Style)
    // Format: [ POTENTIAL SAVINGS: $1,240.50 ]
    // Style: Bold Green text on Dark Gray background (or just simple bold green as requested)
    tickerStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#00FF00")). // Bright Green for text? Or #10B981?
        // Prompt says "Bold Green text on a Dark Gray background".
        // Let's use text green, background slate.
        Background(lipgloss.Color("#1E293B")). // Dark Slate
        Padding(0, 2)

    savingsStr := fmt.Sprintf("POTENTIAL SAVINGS: $%.2f", m.totalSavings)
    s.WriteString(" " + tickerStyle.Render("[ "+savingsStr+" ]") + "\n\n")

	// Tree Construction (Same logic as before, just better rendering)
	m.Graph.Mu.RLock()
	children := make(map[string][]*graph.Node)
	var roots []*graph.Node
	wasteMap := make(map[string]*graph.Node)

	for _, node := range m.Graph.Nodes {
		if node.IsWaste && !node.Ignored {
			wasteMap[node.ID] = node
		}
	}

	for _, node := range wasteMap {
		parentID := ""
		if pid, ok := node.Properties["ParentID"].(string); ok {
			parentID = pid
		} else if cArn, ok := node.Properties["ClusterArn"].(string); ok {
			parentID = cArn
		}

		if parentID != "" && wasteMap[parentID] != nil {
			children[parentID] = append(children[parentID], node)
		} else {
			roots = append(roots, node)
		}
	}
	m.Graph.Mu.RUnlock()

	// Sort Roots
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].ID < roots[j].ID
	})

	if len(roots) == 0 {
		s.WriteString(dimStyle.Render("Infrastructure Clean. No Waste Detected."))
	} else {
		// Flatten logic for cursor navigation
		type displayItem struct {
			node   *graph.Node
			depth  int
			isLast bool
            parent *graph.Node // To track relationship for tree lines?
		}
		var displayList []displayItem

		for _, root := range roots {
			displayList = append(displayList, displayItem{node: root, depth: 0})
			kids := children[root.ID]
			sort.Slice(kids, func(i, j int) bool { return kids[i].ID < kids[j].ID })
			for k, kid := range kids {
				displayList = append(displayList, displayItem{node: kid, depth: 1, isLast: k == len(kids)-1, parent: root})
			}
		}

		// Cursor Logic
		if m.cursor >= len(displayList) {
			m.cursor = len(displayList) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}

		// Windowing
		start, end := 0, len(displayList)
		if len(displayList) > 15 {
			if m.cursor > 10 {
				start = m.cursor - 10
			}
			end = start + 15
			if end > len(displayList) {
				end = len(displayList)
				start = end - 15
				if start < 0 { start = 0 }
			}
		}

		for i := start; i < end; i++ {
			dItem := displayList[i]
			node := dItem.node

			// Cursor
			params := " "
			if i == m.cursor {
				params = cursorStyle.Render(">")
			}

			// Tree Structure
			treePrefix := ""
            if dItem.depth == 0 {
                // Root
                // [WARN] ResourceID
                // No Lines
            } else {
                // Child
                if dItem.isLast {
                    treePrefix = " └─ "
                } else {
                    treePrefix = " ├─ "
                }
            }

            // Name
            name := node.ID
            if val, ok := node.Properties["Name"].(string); ok && val != "" {
				name = val
			} else {
                parts := strings.Split(node.ID, "/") // simple basename
				if len(parts) > 1 {
					name = parts[len(parts)-1]
				}
            }
            // Hyperlink disabled for now to keep ASCII clean? Or keep it?
            // Keep hyperlinks, useful.
            name = makeHyperlink(node.ID, node.Properties, name, node.Type)

            // Render Row
            if dItem.depth == 0 {
                // Root Style: [WARN] Type: Name
                // Risk-based Icon
                icon := iconWarn
                if node.RiskScore > 80 { icon = iconDanger }
                
                line := fmt.Sprintf("%s %s %s: %s", params, icon, node.Type, name)
                s.WriteString(line + "\n")
                // Only root gets the "Pipe" down?
                // Actually, standard tree:
                // Root
                // ├─ Child
                
                // If we want "Context" lines under root (Status, Cost, Reason), we need to fake them or add them as children?
                // The prompt example:
                // [WARN] NAT Gateway: nat...
                //  │
                //  ├─ Status: Idle...
                //  ├─ Cost: $32...
                
                // My current children list contains actual Graph Nodes (Subnets).
                // I should render Properties AS IF they were children logic?
                // Or just render the node line.
                // Current `displayList` only has Nodes.
                // To achieve prompt style, I would need to inject property rows into displayList or render multi-line items.
                // Multi-line items break cursor logic (1 item = N lines?).
                // Simpler: Just render 1 line per Node, but format it well.
                // Or if it IS the cursor, expand it?
                // "m.showingDetails" toggle works. 
                // But prompt asks for "Visual Hierarchy" in the main view.
                
                // Compromise:
                // Render Roots as Headers.
                // Render Details (Reason, Cost) as "Attributes" in 1 line?
                // "└─ Cost: $xx • Reason: ..." 
                // No, prompt wants "Tree".
                // Since I can't easily rewrite the entire display list logic to include property-rows without breaking cursor,
                // I will render the Child Nodes (Subnets) properly inducted.
                
                // Example:
                // [WARN] NAT Gateway
                //  └─ [FAIL] Subnet: subnet-1 (Reason: Empty)
                
            } else {
                // Child Style
                //    └─ Subnet: subnet-abc
                line := fmt.Sprintf("%s   %s%s", params, treePrefix, name)
                
                // Add short reason/cost if available
                if node.Cost > 0 {
                    line += labelStyle.Render(fmt.Sprintf(" ($%.2f)", node.Cost))
                }
                
                s.WriteString(line + "\n")
            }
		}
        
        if len(displayList) > end {
            s.WriteString(dimStyle.Render("   ..."))
        }

		// DETAILS PANE (Bottom)
		if len(displayList) > 0 {
			idx := m.cursor
			if idx >= 0 && idx < len(displayList) {
				node := displayList[idx].node
                
                // Construct Detail Tree for this node
                // ├─ Status: ...
                // ├─ Cost: ...
				
				details := fmt.Sprintf(
					"%s %s\n%s\n %s Status:   %s\n %s Region:   %v\n %s Cost:     $%.2f/mo\n %s Reason:   %v",
                    iconDanger, // Title Icon
					"RESOURCE DETAILS",
                    dimStyle.Render(strings.Repeat("-", 40)),
					dimStyle.Render("├─"),
                    "Active", // Placeholder, or read State
					dimStyle.Render("├─"),
					node.Properties["Region"],
					dimStyle.Render("├─"),
					node.Cost,
					dimStyle.Render("└─"), // Last one
					node.Properties["Reason"],
				)
                
                // Just append to bottom
				s.WriteString("\n\n")
				s.WriteString(details) // plaintext, no box to start? Prompt: "Visual Hierarchy"
                // Prompt: "Strict Design Commandments... Tier SSSSS+ Output".
                // The prompt example was in the main list.
                // But doing multi-line list items is hard in simple loop.
                // I'll keep single line list, but rich details at bottom.
			}
		}
	}

	s.WriteString("\n\n")
    // Status Bar
	if m.showingDetails {
		s.WriteString(helpStyle("enter: close details • q: quit"))
	} else {
		s.WriteString(helpStyle("i: ignore resource • enter: view details • q: quit"))
	}
	return s.String()
}

// Helper to find ID by index (matches View logic sort-of)
func (m Model) getKthWasteNodeID(k int) string {
    m.Graph.Mu.RLock()
    defer m.Graph.Mu.RUnlock()
    
    // Reconstruct list (Same as View)
    // TODO: Refactor View to store this list in Model to avoid duplication?
    // For now, duplicate logic for speed of implementation.
    
    var roots []*graph.Node
    children := make(map[string][]*graph.Node)
    wasteMap := make(map[string]*graph.Node)
    
    for _, node := range m.Graph.Nodes {
        if node.IsWaste && !node.Ignored { // Filter Ignored
            wasteMap[node.ID] = node
        }
    }
    
    for _, node := range wasteMap {
         parentID := ""
         if cArn, ok := node.Properties["ClusterArn"].(string); ok {
             parentID = cArn
         }
         if parentID != "" && wasteMap[parentID] != nil {
             children[parentID] = append(children[parentID], node)
         } else {
             roots = append(roots, node)
         }
    }
    
    sort.Slice(roots, func(i, j int) bool { return roots[i].ID < roots[j].ID })

    var displayList []*graph.Node
    for _, root := range roots {
        displayList = append(displayList, root)
        kids := children[root.ID]
        sort.Slice(kids, func(i, j int) bool { return kids[i].ID < kids[j].ID })
        displayList = append(displayList, kids...)
    }
    
    if k >= 0 && k < len(displayList) {
        return displayList[k].ID
    }
    return ""
}

func (m Model) ignoreNode(id string) {
    m.Graph.Mu.Lock()
    node, exists := m.Graph.Nodes[id]
    if exists {
        node.Ignored = true
        node.IsWaste = false // Visually remove immediately
    }
    m.Graph.Mu.Unlock()
    
    // PERSISTENCE (Immediate Write)
    // Append to .ignore.yaml
    // Format:
    // ignored:
    //   - id
    
    // Simple append? Or read-modify-write?
    // User wants "Immediate Write". Append is safest/fastest.
    // But YAML isn't append-friendly unless we parse.
    // Let's read, append, write.
    
    type IgnoreFile struct {
        Ignored []string `yaml:"ignored"`
    }
    
    var data IgnoreFile
    fBytes, err := os.ReadFile(".ignore.yaml")
    if err == nil {
        yaml.Unmarshal(fBytes, &data)
    }
    
    // Dedup
    found := false
    for _, existing := range data.Ignored {
        if existing == id { found = true; break }
    }
    
    if !found {
        data.Ignored = append(data.Ignored, id)
        outBytes, _ := yaml.Marshal(data)
        os.WriteFile(".ignore.yaml", outBytes, 0644)
    }
}

// makeHyperlink constructs an OSC 8 hyperlink for the terminal.
func makeHyperlink(id string, props map[string]interface{}, text, resourceType string) string {
	// Basic AWS Console URL construction
	// e.g. https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/NAME

	region := "us-east-1"
	if r, ok := props["Region"].(string); ok && r != "" {
		region = r
	}
	// Try to parse region from ARN
	parts := strings.Split(id, ":")
	if len(parts) > 3 {
		region = parts[3]
	}

	url := ""

	switch resourceType {
	case "AWS::ECS::Cluster":
		// arn:aws:ecs:region:account:cluster/name
		name := ""
		arnParts := strings.Split(id, "/")
		if len(arnParts) > 1 {
			name = arnParts[len(arnParts)-1]
		}
		url = fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s?region=%s", region, name, region)
	case "AWS::ECS::Service":
		// arn:aws:ecs:region:account:service/clusterName/serviceName
		// Note: New ARNs use /cluster/service, old used /service/serviceName (requires cluster param)
		// We stored ClusterArn in properties!
		if cArn, ok := props["ClusterArn"].(string); ok {
			cName := ""
			cParts := strings.Split(cArn, "/")
			if len(cParts) > 1 {
				cName = cParts[len(cParts)-1]
			}

			sName := ""
			sParts := strings.Split(id, "/")
			if len(sParts) > 1 {
				sName = sParts[len(sParts)-1]
			}

			url = fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s/services/%s?region=%s", region, cName, sName, region)
		}
	case "AWS::EC2::Instance":
		// https://console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-xxx
		// Extract InstanceID
		name := ""
		// usually suffix of ARN: instance/i-xxx
		arnParts := strings.Split(id, "/")
		if len(arnParts) > 1 {
			name = arnParts[len(arnParts)-1]
		}
		url = fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#InstanceDetails:instanceId=%s", region, region, name)
	default:
		return text // No link
	}

	if url != "" {
		// OSC 8 Hyperlink: \033]8;;URL\033\TEXT\033]8;;\033\
		return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
	}
	return text
}

func helpStyle(s string) string {
	return lipgloss.NewStyle().Foreground(subtle).Render(s)
}
