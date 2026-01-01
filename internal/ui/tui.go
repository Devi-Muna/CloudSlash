package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/swarm"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#64748B"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#6366F1"}
	text      = lipgloss.AdaptiveColor{Light: "#191919", Dark: "#E2E8F0"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#10B981"}
	warning   = lipgloss.AdaptiveColor{Light: "#F05D5E", Dark: "#F59E0B"}
	danger    = lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#F43F5E"}

	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2).
			Margin(0, 1)

	iconOK     = lipgloss.NewStyle().Foreground(special).SetString("✓")
	iconWarn   = lipgloss.NewStyle().Foreground(warning).SetString("[WARN]")
	iconDanger = lipgloss.NewStyle().Foreground(danger).SetString("[FAIL]")

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
	isTrial  bool

	Engine *swarm.Engine
	Graph  *graph.Graph

	wasteItems     []string
	cursor         int
	tasksDone      int
	showingDetails bool
	totalSavings   float64
	tfRepairReady  bool
}

func NewModel(e *swarm.Engine, g *graph.Graph, isTrial bool, isMock bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)

	return Model{
		spinner:    s,
		scanning:   !isMock,
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
			m.cursor++
		case "enter", " ":
			m.showingDetails = !m.showingDetails
		case "i":
			idToIgnore := m.getKthWasteNodeID(m.cursor)
			if idToIgnore != "" {
				m.ignoreNode(idToIgnore)
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

		var total float64
		m.Graph.Mu.RLock()
		for _, n := range m.Graph.Nodes {
			if n.IsWaste && !n.Ignored {
				total += n.Cost
			}
		}
		m.Graph.Mu.RUnlock()
		m.totalSavings = total

		// Check for Terraform Fix Script availability
		if _, err := os.Stat("cloudslash-out/fix_terraform.sh"); err == nil {
			m.tfRepairReady = true
		} else {
			m.tfRepairReady = false
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

	if m.tfRepairReady {
		s.WriteString(lipgloss.NewStyle().Foreground(highlight).Bold(true).Render(" [ TF BRIDGE: ACTIVE ]"))
	}
	s.WriteString("\n\n")

	tickerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF00")).
		Background(lipgloss.Color("#1E293B")).
		Padding(0, 2)

	savingsStr := fmt.Sprintf("POTENTIAL SAVINGS: $%.2f", m.totalSavings)
	s.WriteString(" " + tickerStyle.Render("[ "+savingsStr+" ]") + "\n\n")

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

	sort.Slice(roots, func(i, j int) bool {
		return roots[i].ID < roots[j].ID
	})

	if len(roots) == 0 {
		s.WriteString(dimStyle.Render("Infrastructure Clean. No Waste Detected."))
	} else {
		type displayItem struct {
			node   *graph.Node
			depth  int
			isLast bool
			parent *graph.Node
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

		if m.cursor >= len(displayList) {
			m.cursor = len(displayList) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}

		start, end := 0, len(displayList)
		if len(displayList) > 15 {
			if m.cursor > 10 {
				start = m.cursor - 10
			}
			end = start + 15
			if end > len(displayList) {
				end = len(displayList)
				start = end - 15
				if start < 0 {
					start = 0
				}
			}
		}

		for i := start; i < end; i++ {
			dItem := displayList[i]
			node := dItem.node

			params := " "
			if i == m.cursor {
				params = cursorStyle.Render(">")
			}

			treePrefix := ""
			if dItem.depth == 0 {
				// Root has no prefix
			} else {
				if dItem.isLast {
					treePrefix = " └─ "
				} else {
					treePrefix = " ├─ "
				}
			}

			name := node.ID
			if val, ok := node.Properties["Name"].(string); ok && val != "" {
				name = val
			} else {
				parts := strings.Split(node.ID, "/")
				if len(parts) > 1 {
					name = parts[len(parts)-1]
				}
			}
			name = makeHyperlink(node.ID, node.Properties, name, node.Type)

			if dItem.depth == 0 {
				icon := iconWarn
				if node.RiskScore > 80 {
					icon = iconDanger
				}

				line := fmt.Sprintf("%s %s %s: %s", params, icon, node.Type, name)
				s.WriteString(line + "\n")
			} else {
				line := fmt.Sprintf("%s   %s%s", params, treePrefix, name)

				if node.Cost > 0 {
					line += labelStyle.Render(fmt.Sprintf(" ($%.2f)", node.Cost))
				}

				s.WriteString(line + "\n")
			}
		}

		if len(displayList) > end {
			s.WriteString(dimStyle.Render("   ..."))
		}

		if len(displayList) > 0 {
			idx := m.cursor
			if idx >= 0 && idx < len(displayList) {
				node := displayList[idx].node

				details := fmt.Sprintf(
					"%s %s\n%s\n %s Status:   %s\n %s Region:   %v\n %s Cost:     $%.2f/mo\n %s Reason:   %v",
					iconDanger,
					"RESOURCE DETAILS",
					dimStyle.Render(strings.Repeat("-", 40)),
					dimStyle.Render("├─"),
					"Active",
					dimStyle.Render("├─"),
					node.Properties["Region"],
					dimStyle.Render("├─"),
					node.Cost,
					dimStyle.Render("└─"),
					node.Properties["Reason"],
				)

				s.WriteString("\n\n")
				s.WriteString(details)
			}
		}
	}

	s.WriteString("\n\n")
	if m.showingDetails {
		s.WriteString(helpStyle("enter: close details • q: quit"))
	} else {
		s.WriteString(helpStyle("i: ignore resource • enter: view details • q: quit"))
	}
	return s.String()
}

func (m Model) getKthWasteNodeID(k int) string {
	m.Graph.Mu.RLock()
	defer m.Graph.Mu.RUnlock()

	var roots []*graph.Node
	children := make(map[string][]*graph.Node)
	wasteMap := make(map[string]*graph.Node)

	for _, node := range m.Graph.Nodes {
		if node.IsWaste && !node.Ignored {
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
		node.IsWaste = false
	}
	m.Graph.Mu.Unlock()

	type IgnoreFile struct {
		Ignored []string `yaml:"ignored"`
	}

	var data IgnoreFile
	fBytes, err := os.ReadFile(".ignore.yaml")
	if err == nil {
		yaml.Unmarshal(fBytes, &data)
	}

	found := false
	for _, existing := range data.Ignored {
		if existing == id {
			found = true
			break
		}
	}

	if !found {
		data.Ignored = append(data.Ignored, id)
		outBytes, _ := yaml.Marshal(data)
		os.WriteFile(".ignore.yaml", outBytes, 0644)
	}
}

func makeHyperlink(id string, props map[string]interface{}, text, resourceType string) string {
	region := "us-east-1"
	if r, ok := props["Region"].(string); ok && r != "" {
		region = r
	}
	parts := strings.Split(id, ":")
	if len(parts) > 3 {
		region = parts[3]
	}

	url := ""

	switch resourceType {
	case "AWS::ECS::Cluster":
		name := ""
		arnParts := strings.Split(id, "/")
		if len(arnParts) > 1 {
			name = arnParts[len(arnParts)-1]
		}
		url = fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s?region=%s", region, name, region)
	case "AWS::ECS::Service":
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
		name := ""
		arnParts := strings.Split(id, "/")
		if len(arnParts) > 1 {
			name = arnParts[len(arnParts)-1]
		}
		url = fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#InstanceDetails:instanceId=%s", region, region, name)
	default:
		return text
	}

	if url != "" {
		return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
	}
	return text
}

func helpStyle(s string) string {
	return lipgloss.NewStyle().Foreground(subtle).Render(s)
}
