package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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
			if m.state == ViewStateDetail {
				m.state = ViewStateList
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		}

		// State-Specific Handling
		if m.state == ViewStateList {
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
			case "i":
				// Quick Ignore from List
				if len(m.wasteItems) > 0 {
					// wasteItems might be stale if not updated often, but it's updated in ViewList
					if m.cursor < len(m.wasteItems) {
						id := m.wasteItems[m.cursor].ID
						m.ignoreNode(id)
					}
				}
			case "P", "p":
				if m.SortMode == "Price" {
					m.SortMode = ""
				} else {
					m.SortMode = "Price"
				}
			case "E", "e":
				if m.FilterMode == "Easy" {
					m.FilterMode = ""
				} else {
					m.FilterMode = "Easy"
				}
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

	case spinner.TickMsg:
		m.tickCount++
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Background Stats Update
		m.updateStats()
		
		// Check if done scanning
		stats := m.Engine.GetStats()
		m.tasksDone = int(stats.TasksCompleted)
		if stats.TasksCompleted > 10 && stats.ActiveWorkers == 0 {
			m.scanning = false
		}
		
		// Check TF Repair
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
	if m.quitting {
		return "CloudSlash Session Terminated.\n"
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
	}

	// 3. Render Footer (Help)
	footer := quickHelp(m.state)

	return fmt.Sprintf("%s\n%s\n\n%s", hud, body, footer)
}

// Helpers

func (m *Model) updateStats() {
	var total float64
	m.Graph.Mu.RLock()
	var newItems []*graph.Node
	for _, n := range m.Graph.Nodes {
		if n.IsWaste && !n.Ignored {
			total += n.Cost
			newItems = append(newItems, n)
		}
	}
	m.Graph.Mu.RUnlock()
	
	m.totalSavings = total
}


func quickHelp(state ViewState) string {
	base := subtle.Render(" [q] Quit/Back ")
	if state == ViewStateList {
		return base + subtle.Render(" [↑/↓] Nav  [Enter] Details  [i] Ignore  [P]rice Sort  [E]asy Filter  [R]egion")
	}
	if state == ViewStateDetail {
		return base + subtle.Render(" [o] Open Browser  [i] Ignore")
	}
	return base
}

func (m Model) ignoreNode(id string) {
	m.Graph.Mu.Lock()
	node, exists := m.Graph.Nodes[id]
	if exists {
		node.Ignored = true
		node.IsWaste = false
	}
	m.Graph.Mu.Unlock()

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
	if region == "" || region == "<nil>" { region = "us-east-1" }
	
	switch node.Type {
	case "AWS::EC2::Instance":
		return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#InstanceDetails:instanceId=%s", region, region, node.ID)
	case "AWS::S3::Bucket":
		return fmt.Sprintf("https://s3.console.aws.amazon.com/s3/buckets/%s?region=%s", node.ID, region)
	}
	return "https://console.aws.amazon.com"
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
