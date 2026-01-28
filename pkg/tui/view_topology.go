package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/graph"

)

// viewTopology renders the ECS hierarchy tree
func (m Model) viewTopology() string {
	s := strings.Builder{}

	// Header
	headerTxt := fmt.Sprintf("   %-60s | %-10s | %s", "TOPOLOGY HIERARCHY (Cluster -> Service -> Task)", "STATUS", "INFO")
	s.WriteString(dimStyle.Render(headerTxt) + "\n")
	s.WriteString(dimStyle.Render("   "+strings.Repeat("─", 60)) + "\n")

	if len(m.topologyLines) == 0 {
		if m.scanning {
			return fmt.Sprintf("\n\n   %s Building Topology Map...", m.spinner.View())
		}
		return "\n\n   " + subtle.Render("No Clusters Detected.")
	}

	// Pagination window logic for topology
	start, end := m.calculateTopologyWindow(len(m.topologyLines))

	for i := start; i < end; i++ {
		// Safety check
		if i >= len(m.topologyLines) {
			break
		}

		line := m.topologyLines[i]
		isSelected := (i == m.topologyCursor)

		// Render the tree line
		treePart := line.Text
		
		// Status / Info columns
		status := "Unknown"
		info := ""
		
		if line.Node != nil {
			if s, ok := line.Node.Properties["Status"].(string); ok {
				status = s
			}
			// Add specific info based on type
			switch line.Node.Type {
			case "AWS::ECS::Service":
				if rc, ok := line.Node.Properties["RunningCount"].(int); ok {
					info = fmt.Sprintf("Running: %d", rc)
				}
			case "AWS::ECS::Cluster":
				if as, ok := line.Node.Properties["ActiveServicesCount"].(int); ok {
					info = fmt.Sprintf("Services: %d", as)
				}
			}

			// Highlight Waste
			if line.Node.IsWaste {
				status += " [WASTE]"
			}
		}

		// Truncate
		if len(treePart) > 60 {
			treePart = treePart[:57] + "..."
		}

		displayLine := fmt.Sprintf(" %-60s | %-10s | %s", treePart, status, info)

		if isSelected {
			s.WriteString(listSelectedStyle.Render(displayLine) + "\n")
		} else {
			// Apply tree styles to the prefix if possible, for now just normal
			s.WriteString(listNormalStyle.Render(displayLine) + "\n")
		}
	}

	return s.String()
}

// buildTopology regenerates the flattened topology lines
// This should be called when data refreshes or filter changes
func (m *Model) buildTopology() {
	var lines []TopologyLine

	// 1. Find Roots (Clusters)
	var clusters []*graph.Node
	m.Graph.Mu.RLock()
	for _, n := range m.Graph.Nodes {
		if n.Type == "AWS::ECS::Cluster" {
			clusters = append(clusters, n)
		}
	}
	m.Graph.Mu.RUnlock()

	// Sort clusters for stability
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})


	// 2. Traverse
	for i, cluster := range clusters {
		isLastCluster := (i == len(clusters)-1)
		prefix := "├── "
		if isLastCluster {
			prefix = "└── "
		}

		// Add Cluster Line
		clusterName := cluster.ID
		if name, ok := cluster.Properties["Name"].(string); ok {
			clusterName = name
		}
		
		// Professional Indicator: [Cluster]
		lines = append(lines, TopologyLine{
			ID:    cluster.ID,
			Text:  prefix + "[C] " + clusterName,
			Level: 0,
			Node:  cluster,
		})

		children := m.Graph.GetDownstream(cluster.ID)
		
		// Sort children
		sort.Strings(children)

		for j, childID := range children {
			isLastChild := (j == len(children)-1)
			childPrefix := "│   ├── "
			if isLastCluster {
				childPrefix = "    ├── "
			}
			if isLastChild {
				childPrefix = "│   └── "
				if isLastCluster {
					childPrefix = "    └── "
				}
			}

			// Resolve Node
			childNode := m.Graph.GetNode(childID)
			
			if childNode != nil {
				childName := childNode.ID
				if name, ok := childNode.Properties["Name"].(string); ok {
					childName = name
				}
				
				// Professional Indicator: [Service] or [Inst]
				typeIndicator := "[S]" // Service
				if childNode.Type == "AWS::ECS::ContainerInstance" {
					typeIndicator = "[I]"
				} else if childNode.Type == "AWS::ECS::Task" {
					typeIndicator = "[T]"
				}

				lines = append(lines, TopologyLine{
					ID:    childID,
					Text:  childPrefix + " " + typeIndicator + " " + childName,
					Level: 1,
					Node:  childNode,
				})
			}
		}
	}


	m.topologyLines = lines
}

func (m Model) calculateTopologyWindow(total int) (int, int) {
	windowSize := m.height - 8
	if windowSize < 5 {
		windowSize = 5
	}

	start := m.topologyCursor - (windowSize / 2)
	if start < 0 {
		start = 0
	}

	end := start + windowSize
	if end > total {
		end = total
		start = end - windowSize
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
