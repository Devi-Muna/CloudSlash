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

    dimStyle    = lipgloss.NewStyle().Foreground(subtle)
    cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
    specStyle   = lipgloss.NewStyle().Foreground(special)
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

    // Tree Construction
    m.Graph.Mu.RLock()
    // var items []*graph.Node
    // Map of ParentID -> [Children]
    children := make(map[string][]*graph.Node)
    // Identify Roots (Nodes that are waste and have no parent, OR Nodes that are waste and their parent is NOT in the waste list?
    // Actually, we want to group by Cluster even if Cluster is NOT waste, if we want full hierarchy.
    // But usually we lists Waste.
    // If Cluster is Waste, it's a Root.
    // If Service is Waste, we check if Cluster is known.
    // To support "Contextual Hierarchy", we might need to fetch parent from properties even if parent isn't in "items".
    // But we only want to render waste items.
    // "Plaintext ▼ Zombie Cluster: production-us-east-1 ... └─ Ghost Node Group ..."
    // This implies the Cluster IS waste.
    
    // Grouping Logic:
    // 1. Collect all Waste Nodes.
    // 2. Sort them into:
    //    - Clusters (Roots)
    //    - Services (Children of Clusters)
    //    - Others (Flat Roots)
    
    var roots []*graph.Node
    // orphanedServices := []*graph.Node{} // Unused
    // Simpler: Just render items. If item has children, render children under it.
    // Since we flat-list waste in graph usually.
    // We need to know dependencies.
    
    wasteMap := make(map[string]*graph.Node)
    
    for _, node := range m.Graph.Nodes {
        if node.IsWaste {
            wasteMap[node.ID] = node
        }
    }
    
    for _, node := range wasteMap {
         // Check if node has a parent in wasteMap
         parentID := ""
         if cArn, ok := node.Properties["ClusterArn"].(string); ok {
             parentID = cArn
         }
         
         if parentID != "" && wasteMap[parentID] != nil {
             // It's a child of a waste node
             children[parentID] = append(children[parentID], node)
         } else {
             // It's a root (or orphan)
             roots = append(roots, node)
         }
    }
    m.Graph.Mu.RUnlock()
    
    // Sort Roots
    sort.Slice(roots, func(i, j int) bool {
        return roots[i].ID < roots[j].ID
    })

    if len(roots) == 0 && len(children) == 0 { // Should match total waste count logic
         s.WriteString(dimStyle.Render("No waste found. System Clean."))
    } else {
        s.WriteString(dimStyle.Render(fmt.Sprintf("%-3s %-40s %-12s %s\n", "", "RESOURCE", "COST", "REASON")))
        s.WriteString(dimStyle.Render(strings.Repeat("-", 80) + "\n"))

        // Flatten logic for cursor navigation
        type displayItem struct {
            node *graph.Node
            depth int
            isLast bool 
        }
        var displayList []displayItem
        
        for _, root := range roots {
            displayList = append(displayList, displayItem{node: root, depth: 0})
            kids := children[root.ID]
            sort.Slice(kids, func(i, j int) bool { return kids[i].ID < kids[j].ID })
            for k, kid := range kids {
                displayList = append(displayList, displayItem{node: kid, depth: 1, isLast: k == len(kids)-1})
            }
        }
        
        // Cursor Logic
        if m.cursor >= len(displayList) { m.cursor = len(displayList) - 1 }
        if m.cursor < 0 { m.cursor = 0 }
        
        // Render Window
        start, end := 0, len(displayList)
        if len(displayList) > 15 {
             // simple scrolling
             if m.cursor > 10 {
                 start = m.cursor - 10
             }
             end = start + 15
             if end > len(displayList) {
                 end = len(displayList)
                 start = end - 15
                 if start < 0 { start = 0 } // safety
             }
        }
        
        for i := start; i < end; i++ {
            dItem := displayList[i]
            node := dItem.node
            
            // Cursor
            gutter := "   "
            if i == m.cursor {
                gutter = cursorStyle.Render(" > ")
            }
            
            // Tree Prefix
            prefix := ""
            if dItem.depth > 0 {
                if dItem.isLast {
                    prefix = " └─ "
                } else {
                    prefix = " ├─ "
                }
            } else {
               // Root level spacer? Or just indentation? 
               // Roots don't have tree lines usually unless we have a single system root.
            }
            
            // Icon
            icon := specStyle.Render("▼") // For roots?
            if dItem.depth > 0 { icon = specStyle.Render(" ") } // simpler
            
            // Name/ID (Deep Link)
            // Use Name property if available, else ID suffix
            displayName := node.ID
            if val, ok := node.Properties["Name"].(string); ok {
                displayName = val
            } else {
                parts := strings.Split(node.ID, "/")
                if len(parts) > 1 { displayName = parts[len(parts)-1] }
            }
            // Add Hyperlink
            displayName = makeHyperlink(node.ID, node.Properties, displayName, node.Type)
            
            // Colorize Name
            if node.IsWaste {
                 displayName = lipgloss.NewStyle().Foreground(warning).Render(displayName)
            }
            
            // Cost
            costStr := "-"
            if node.Cost > 0 {
                costStr = fmt.Sprintf("$%.2f", node.Cost)
            }
            
            // Reason (Truncated)
            reason := ""
            if r, ok := node.Properties["Reason"].(string); ok {
                reason = r
                if len(reason) > 40 { reason = reason[:37] + "..." }
            }

            // Row Render
            // Layout: Gutter | Tree | Name ..... | Cost | Reason
            rowStr := fmt.Sprintf("%s%s %-35s %-12s %s", prefix, icon, displayName, costStr, reason)
            if i == m.cursor {
                 // highlight the whole row? Or just cursor.
                 // re-render logic is minimal here.
            }
            
            s.WriteString(gutter + rowStr + "\n")
        }
        
        if len(displayList) > end {
             s.WriteString(dimStyle.Render("..."))
        }
        
        // DETAIL VIEW OVERLAY
        if m.showingDetails && len(displayList) > 0 {
             idx := m.cursor
             if idx >= 0 && idx < len(displayList) {
                 node := displayList[idx].node
                 
                 details := fmt.Sprintf(
                     "DETAILS FOR %s\n%s\n\nType:   %s\nRegion: %v\nCost:   $%.2f/mo\n\n[DETECTED WASTE]\nReason: %v",
                     node.ID, // ID
                     makeHyperlink(node.ID, node.Properties, "(Open in Console)", node.Type), // Link
                     node.Type,
                     node.Properties["Region"],
                     node.Cost,
                     node.Properties["Reason"],
                 )
                 // Add events if any
                 if events, ok := node.Properties["Events"].([]string); ok && len(events) > 0 {
                     details += fmt.Sprintf("\n\nEvents:\n%s", strings.Join(events, "\n"))
                 }
                 
                 s.WriteString("\n\n")
                 s.WriteString(cardStyle.Render(details))
             }
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
        if len(arnParts) > 1 { name = arnParts[len(arnParts)-1] }
        url = fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s?region=%s", region, name, region)
    case "AWS::ECS::Service":
        // arn:aws:ecs:region:account:service/clusterName/serviceName
        // Note: New ARNs use /cluster/service, old used /service/serviceName (requires cluster param)
        // We stored ClusterArn in properties!
        if cArn, ok := props["ClusterArn"].(string); ok {
            cName := ""
            cParts := strings.Split(cArn, "/")
            if len(cParts) > 1 { cName = cParts[len(cParts)-1] }
            
            sName := ""
            sParts := strings.Split(id, "/")
            if len(sParts) > 1 { sName = sParts[len(sParts)-1] }
             
             url = fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s/services/%s?region=%s", region, cName, sName, region)
        }
    case "AWS::EC2::Instance":
         // https://console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-xxx
         // Extract InstanceID
         name := ""
         // usually suffix of ARN: instance/i-xxx
         arnParts := strings.Split(id, "/")
         if len(arnParts) > 1 { name = arnParts[len(arnParts)-1] }
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
