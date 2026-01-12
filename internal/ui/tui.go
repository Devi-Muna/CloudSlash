package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/internal/audit"

	"gopkg.in/yaml.v2"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/version"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		// Global View Switch
		case "q":
			if m.state == ViewStateDetail || m.state == ViewStateTopology {
				m.state = ViewStateList
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		
		// View Switching
		case "t":
			if m.state == ViewStateTopology {
				m.state = ViewStateList
			} else {
				m.state = ViewStateTopology
				m.buildTopology() // Ensure fresh build on switch
			}
		}

		// State-Specific Handling
		if m.state == ViewStateTopology {
			switch msg.String() {
			case "up", "k":
				if m.topologyCursor > 0 {
					m.topologyCursor--
				}
			case "down", "j":
				if m.topologyCursor < len(m.topologyLines)-1 {
					m.topologyCursor++
				}
			case "enter", " ":
				// Toggle expansion? For now, maybe select/copy ID
				// Future: m.toggleExpansion(m.topologyCursor)
			}
		} else if m.state == ViewStateList {
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.wasteItems)-1 {
					m.cursor++
				}
			case "enter", " ":
				if len(m.wasteItems) > 0 {
					m.state = ViewStateDetail
				}
			case "m":
				// Soft Delete (Mark for Death)
				if len(m.wasteItems) > 0 && m.cursor < len(m.wasteItems) {
					id := m.wasteItems[m.cursor].ID
					// In a real app, this would call AWS: TagResource("cloudslash:status", "to-delete")
					// For now, we simulate/log it and visually indicate it (e.g., ignore/hide it or mark it)
					// Let's re-use ignore logic but with a "soft-delete" flag if we had one.
					// For TUI demo, we'll log it to audit and treat it as 'handled' (ignored)
					audit.LogAction("SOFT_DELETE", id, "MARKED", 0, "Marked for later collision")
					m.ignoreNode(id)
				}
			case "y":
				if len(m.wasteItems) > 0 && m.cursor < len(m.wasteItems) {
					copyToClipboard(m.wasteItems[m.cursor].ID)
				}
			case "Y":
				if len(m.wasteItems) > 0 && m.cursor < len(m.wasteItems) {
					copyToClipboard(m.wasteItems[m.cursor].ID) // ID is often ARN in Graph
				}
			case "c":
				if len(m.wasteItems) > 0 && m.cursor < len(m.wasteItems) {
					// Copy JSON properties
					node := m.wasteItems[m.cursor]
					jsonStr := fmt.Sprintf("ID: %s\nType: %s\nCost: $%.2f\nProps: %v", node.ID, node.Type, node.Cost, node.Properties)
					copyToClipboard(jsonStr)
				}
			case "P", "p":
				if m.SortMode == "Price" {
					m.SortMode = ""
				} else {
					m.SortMode = "Price"
				}
				m.refreshData()
			case "E", "e":
				if m.FilterMode == "Easy" {
					m.FilterMode = ""
				} else {
					m.FilterMode = "Easy"
				}
				m.refreshData()
			case "R", "r":
				// Simple toggle for now: All -> us-east-1 (example) -> All
				// Use "Next Region" logic?
				// Let's iterate unique regions in graph
				regions := m.getUniqueRegions()
				if len(regions) > 0 {
					// Find current index
					currIdx := -1
					for i, r := range regions {
						if m.FilterMode == r {
							currIdx = i
							break
						}
					}
					// Next
					if currIdx == len(regions)-1 {
						m.FilterMode = ""
					} else {
						m.FilterMode = regions[currIdx+1]
					}
				}
				m.refreshData()
			}
		} else if m.state == ViewStateDetail {
			switch msg.String() {
			case "b", "esc":
				m.state = ViewStateList
			case "o":
				// Open in Browser
				if len(m.wasteItems) > 0 && m.cursor < len(m.wasteItems) {
					node := m.wasteItems[m.cursor]
					url := getConsoleURL(node)
					openBrowser(url)
				}
			case "i":
				// Ignore
				if len(m.wasteItems) > 0 && m.cursor < len(m.wasteItems) {
					id := m.wasteItems[m.cursor].ID
					m.ignoreNode(id)
					m.state = ViewStateList // return to list after action
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 20 // Padding

	case spinner.TickMsg:
		m.tickCount++
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case tickMsg:
		// Background Stats Update
		m.refreshData()

		// Check if done scanning
		stats := m.Engine.GetStats()
		m.tasksDone = int(stats.TasksCompleted)

		// Heuristic Progress: Completed / (Completed + Active + 1)
		// The +1 prevents division by zero and keeps it < 100% until truly done
		total := float64(stats.TasksCompleted + int64(stats.ActiveWorkers))
		if total == 0 {
			total = 1
		}
		pct := float64(stats.TasksCompleted) / total

		// If scanning is done (Active=0 and >10 tasks), force 100%
		if stats.TasksCompleted > 10 && stats.ActiveWorkers == 0 {
			m.scanning = false
			pct = 1.0
		}

		cmd := m.progress.SetPercent(pct)

		// Check TF Repair
		if _, err := os.Stat("cloudslash-out/fix_terraform.sh"); err == nil {
			m.tfRepairReady = true
		} else {
			m.tfRepairReady = false
		}

		return m, tea.Batch(cmd, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	if m.quitting {
		// Performance Flex
		duration := time.Since(m.startTime).Seconds()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		memMB := mem.Alloc / 1024 / 1024

		nodeCount := 0
		if m.Graph != nil {
			m.Graph.Mu.RLock()
			nodeCount = len(m.Graph.Nodes)
			m.Graph.Mu.RUnlock()
		}

		stats := fmt.Sprintf("\n[SUCCESS] Scan Complete. Audited %d resources in %.2fs. (Memory: %dMB)\n", nodeCount, duration, memMB)
		// Exit View
		s := "\n" +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF99")).
				Bold(true).
				Render(fmt.Sprintf("%s %s [%s]", version.AppName, version.Current, version.License)) + "\n" +
			`Open Source Infrastructure Forensics
Maintained by DrSkyle | github.com/drskyle/cloudslash

License: AGPLv3. For commercial use without open-source obligations, please acquire a Commercial Exemption.
`
		return stats + s
	}

	// 1. Render HUD
	hud := m.viewHUD()

	// 2. Render Main Body
	var body string
	switch m.state {
	case ViewStateList:
		body = m.viewList()
	case ViewStateDetail:
		body = m.viewDetails()
	case ViewStateTopology:
		body = m.viewTopology()
	}

	// 3. Render Footer (Help)
	footer := quickHelp(m.state)

	return fmt.Sprintf("%s\n%s\n\n%s", hud, body, footer)
}

// Helpers

func (m *Model) refreshData() {
	var total float64
	var nodes []*graph.Node

	m.Graph.Mu.RLock()
	defer m.Graph.Mu.RUnlock()

	for _, n := range m.Graph.Nodes {
		if n.IsWaste && !n.Ignored {
			if m.FilterMode == "Easy" {
				isEasy := false
				if n.Type == "AWS::EC2::EIP" || n.Type == "AWS::EC2::Snapshot" {
					isEasy = true
				}
				if n.RiskScore < 20 {
					isEasy = true
				}
				if !isEasy {
					continue
				}
			} else if m.FilterMode != "" {
				// Region Filter (Simple string match on "Region" property)
				// Note: Ensure property exists and is string
				if r, ok := n.Properties["Region"].(string); !ok || r != m.FilterMode {
					continue
				}
			}

			total += n.Cost
			nodes = append(nodes, n)
		}
	}

	if m.SortMode == "Price" {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Cost > nodes[j].Cost
		})
	} else {
		// Default: ID
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
	}

	m.totalSavings = total
	m.wasteItems = nodes
	
	// Refresh topology logic too if in that view or always? 
	// To be safe, let's keep it updated.
	m.buildTopology()
}

func quickHelp(state ViewState) string {
	base := subtle.Render(" [q] Quit/Back ")
	if state == ViewStateList {
		return base + subtle.Render(" [↑/↓] Nav  [Enter] Details  [i] Ign  [m] Mark  [y] Copy  [P]rice  [E]asy")
	}
	if state == ViewStateDetail {
		return base + subtle.Render(" [o] Open Browser  [i] Ign  [m] Mark  [y] Copy")
	}
	if state == ViewStateTopology {
		return base + subtle.Render(" [↑/↓] Nav  [Enter] Action(WIP)")
	}
	return base
}

func (m Model) ignoreNode(id string) {
	node := m.Graph.GetNode(id)
	if node != nil {
		m.Graph.Mu.Lock()
		node.Ignored = true
		node.IsWaste = false
		m.Graph.Mu.Unlock()
	}

	// Persist to .ignore.yaml
	type IgnoreFile struct {
		Ignored []string `yaml:"ignored"`
	}
	var data IgnoreFile
	fBytes, err := os.ReadFile(".ignore.yaml")
	if err == nil {
		yaml.Unmarshal(fBytes, &data)
	}
	data.Ignored = append(data.Ignored, id)
	outBytes, _ := yaml.Marshal(data)
	os.WriteFile(".ignore.yaml", outBytes, 0644)
}

func getConsoleURL(node *graph.Node) string {
	region := fmt.Sprintf("%v", node.Properties["Region"])
	if region == "" || region == "<nil>" {
		region = "us-east-1"
	}

	switch node.Type {
	case "AWS::EC2::Instance":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#InstanceDetails:instanceId=%s", region, region, node.ID)
	case "AWS::S3::Bucket":
		return fmt.Sprintf("https://s3.console.aws.amazon.com/s3/buckets/%s?region=%s", node.ID, region)
	}
	return "https://console.aws.amazon.com"
}

func copyToClipboard(text string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// check for wl-copy (wayland) or xclip (x11)
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return
	}

	cmd.Stdin = strings.NewReader(text)
	cmd.Start()
	// Don't wait, just fire and forget for UI responsiveness
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		// handle error
	}
}

// Needed by other components?
func helpStyle(s string) string {
	return subtle.Render(s)
}

func (m Model) getUniqueRegions() []string {
	m.Graph.Mu.RLock()
	defer m.Graph.Mu.RUnlock()
	unique := make(map[string]bool)
	var list []string
	for _, n := range m.Graph.Nodes {
		if n.IsWaste && !n.Ignored {
			if r, ok := n.Properties["Region"].(string); ok {
				if !unique[r] {
					unique[r] = true
					list = append(list, r)
				}
			}
		}
	}
	sort.Strings(list)
	return list
}

func (m Model) getKthWasteNodeID(k int) string {
	// Not used in new logic, but kept for compatibility if needed?
	// New logic uses m.wasteItems list.
	return ""
}
