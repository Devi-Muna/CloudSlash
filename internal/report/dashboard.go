package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings" // Added import
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// GenerateDashboard creates a high-fidelity HTML report.
func GenerateDashboard(g *graph.Graph, path string) error {
	items := extractItems(g) // Reuse logic from export.go

	// Calculate Stats
	totalCost := 0.0
	riskCount := 0
	for _, item := range items {
		totalCost += item.MonthlyCost
		if item.RiskScore > 50 {
			riskCount++
		}
	}

	// Prepare Graph Data for Visualization
	graphData, err := buildSankeyData(g)
	if err != nil {
		fmt.Printf("[WARN] Failed to build Sankey data: %v\n", err)
		// non-fatal, empty graph
		graphData = []byte("{}")
	}

	jsonData, err := json.Marshal(items)
	if err != nil {
		return err
	}

	// THE TEMPLATE (Embedded for Single-Binary Portability)
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CloudSlash Audit</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <script src="https://unpkg.com/d3-sankey@0.12.3/dist/d3-sankey.min.js"></script>
    <style>
        /* ... existing styles ... */
        :root {
            --bg: #0A0A0A;
            --surface: rgba(255, 255, 255, 0.03);
            --border: rgba(255, 255, 255, 0.1);
            --primary: #00FF99;
            --danger: #FF3366;
            --dark: #333333;
            --text: #F8FAFC;
            --text-dim: #94A3B8;
        }
        body {
            background: var(--bg);
            color: var(--text);
            font-family: 'SF Mono', 'Menlo', monospace;
            margin: 0;
            padding: 40px;
        }
        /* ... header, kpi-grid, table styles reuse ... */
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 40px; border-bottom: 1px solid var(--border); padding-bottom: 20px; }
        .logo { font-size: 1.5rem; font-weight: bold; letter-spacing: -1px; }
        .logo span { color: var(--primary); }
        .meta { color: var(--text-dim); font-size: 0.9rem; }
        
        .kpi-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; margin-bottom: 40px; }
        .card { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; padding: 24px; backdrop-filter: blur(10px); }
        .card h3 { margin: 0 0 10px 0; font-size: 0.8rem; color: var(--text-dim); text-transform: uppercase; letter-spacing: 1px; }
        .card .value { font-size: 2.5rem; font-weight: bold; }
        .card .value.cost { color: var(--danger); }
        .card .value.safe { color: var(--primary); }

        .viz-container {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 40px;
            height: 600px; /* Canvas Height */
            position: relative;
            overflow: hidden;
        }
        .viz-title {
            position: absolute;
            top: 20px;
            left: 20px;
            font-size: 0.8rem;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 1px;
            pointer-events: none;
        }

        .table-container { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; overflow: hidden; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 16px 24px; text-align: left; border-bottom: 1px solid var(--border); }
        th { background: rgba(0,0,0,0.3); color: var(--text-dim); font-size: 0.8rem; text-transform: uppercase; }
        tr:last-child td { border-bottom: none; }
        tr:hover { background: rgba(255,255,255,0.02); }
        .badge { padding: 4px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: bold; }
        .badge.JUNK { background: rgba(255, 51, 102, 0.1); color: var(--danger); border: 1px solid rgba(255, 51, 102, 0.3); }
        .badge.REVIEW { background: rgba(255, 170, 0, 0.1); color: #FFAA00; border: 1px solid rgba(255, 170, 0, 0.3); }
        .evidence { font-size: 0.85rem; color: var(--text-dim); }
        footer { margin-top: 60px; color: var(--text-dim); font-size: 0.8rem; text-align: center; }
        
        /* Sankey Specifics */
        .node rect { cursor: pointer; fill-opacity: .9; shape-rendering: crispEdges; }
        .node text { pointer-events: none; text-shadow: 0 1px 0 #000; font-family: 'SF Mono', sans-serif; font-size: 10px; fill: #fff; }
        .link { fill: none; stroke: #000; stroke-opacity: .2; }
        .link:hover { stroke-opacity: .5; }
    </style>
</head>
<body>

    <div class="header">
        <div class="logo">CLOUD<span>SLASH</span>_AUDIT</div>
        <div class="meta">Generated: {{GENERATED_TIME}}</div>
    </div>

    <div class="kpi-grid">
        <div class="card">
            <h3>Monthly Waste</h3>
            <div class="value cost">${{TOTAL_COST}}</div>
        </div>
        <div class="card">
            <h3>Zombie Resources</h3>
            <div class="value">{{ZOMBIE_COUNT}}</div>
        </div>
        <div class="card">
            <h3>Topology Status</h3>
            <div class="value safe">CONNECTED</div>
        </div>
    </div>

    <!-- THE SANKEY OF DOOM -->
    <div class="viz-container">
        <div class="viz-title">// NETWORK TOPOLOGY & WASTE FLOW</div>
        <div id="chart"></div>
    </div>

    <div class="table-container">
        <table>
            <thead>
                <tr>
                    <th>Type</th>
                    <th>Resource ID</th>
                    <th>Region</th>
                    <th>Monthly Cost</th>
                    <th>Action</th>
                    <th>Evidence</th>
                </tr>
            </thead>
            <tbody id="table-body">
                <!-- JS Injection -->
            </tbody>
        </table>
    </div>

    <footer>
        Generated by CloudSlash " + version.Current + " (AGPLv3) | Local-First Forensic Engine
    </footer>

    <script>
        // DATA INJECTION
        window.REPORT_DATA = {{REPORT_DATA}};
        window.GRAPH_DATA = {{GRAPH_DATA}};

        const tbody = document.getElementById('table-body');
        const currency = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

        window.REPORT_DATA.forEach(item => {
            const tr = document.createElement('tr');
            
            const badgeClass = item.risk_score > 50 ? 'JUNK' : 'REVIEW';
            const costClass = item.monthly_cost > 0 ? 'color: #FF3366;' : 'color: #94A3B8;';

            tr.innerHTML = `+"`"+`
                <td><span style="opacity:0.7">`+"`"+` + item.type + `+"`"+`</span></td>
                <td style="font-weight:bold; color: #fff;">`+"`"+` + item.resource_id + `+"`"+`</td>
                <td>`+"`"+` + item.region + `+"`"+`</td>
                <td style="`+"`"+` + costClass + `+"`"+`">`+"`"+` + currency.format(item.monthly_cost) + `+"`"+`</td>
                <td><span class="badge `+"`"+` + badgeClass + `+"`"+`">`+"`"+` + item.action + `+"`"+`</span></td>
                <td class="evidence">`+"`"+` + item.forensic_evidence + `+"`"+`</td>
            `+"`"+`;
            tbody.appendChild(tr);
        });
        // Sankey Logic
        if (window.GRAPH_DATA && window.GRAPH_DATA.nodes && window.GRAPH_DATA.nodes.length > 0) {
            const width = document.querySelector('.viz-container').clientWidth;
            const height = 600;

            const svg = d3.select("#chart").append("svg")
                .attr("width", width)
                .attr("height", height);

            const sankey = d3.sankey()
                .nodeWidth(15)
                .nodePadding(10)
                .extent([[1, 1], [width - 1, height - 6]]);

            const {nodes, links} = sankey(window.GRAPH_DATA);

            // Links
            svg.append("g")
                .selectAll("rect")
                .data(links)
                .enter().append("path")
                .attr("d", d3.sankeyLinkHorizontal())
                .attr("stroke-width", d => Math.max(1, d.width))
                .style("fill", "none")
                .style("stroke", d => {
                    // Logic: If target is waste, RED. If safe, GREEN.
                    if (d.target.waste) return "#FF3366"; // Danger
                    return "#00FF99"; // Primary
                })
                .style("stroke-opacity", 0.5);

            // Nodes
            const node = svg.append("g")
                .selectAll("rect")
                .data(nodes)
                .enter().append("g");

            node.append("rect")
                .attr("x", d => d.x0)
                .attr("y", d => d.y0)
                .attr("height", d => d.y1 - d.y0)
                .attr("width", d => d.x1 - d.x0)
                .style("fill", d => {
                   if (d.name === "Internet") return "#FFFFFF";
                   if (d.waste) return "#FF3366";
                   return "#444";
                })
                .style("opacity", 0.8);

            node.append("text")
                .attr("x", d => d.x0 - 6)
                .attr("y", d => (d.y1 + d.y0) / 2)
                .attr("dy", "0.35em")
                .attr("text-anchor", "end")
                .text(d => d.name)
                .filter(d => d.x0 < width / 2)
                .attr("x", d => d.x1 + 6)
                .attr("text-anchor", "start");
        }
    </script>
</body>
</html>`

	html = strings.ReplaceAll(html, "{{GENERATED_TIME}}", time.Now().Format("2006-01-02 15:04:05"))
	html = strings.ReplaceAll(html, "{{TOTAL_COST}}", fmt.Sprintf("%.2f", totalCost))
	html = strings.ReplaceAll(html, "{{ZOMBIE_COUNT}}", fmt.Sprintf("%d", len(items)))
	html = strings.ReplaceAll(html, "{{REPORT_DATA}}", string(jsonData))
	html = strings.ReplaceAll(html, "{{GRAPH_DATA}}", string(graphData))

	return os.WriteFile(path, []byte(html), 0644)
}

// Data Structures for D3 Sankey
type SankeyNode struct {
	Name  string `json:"name"`
	Waste bool   `json:"waste"`
}
type SankeyLink struct {
	Source int     `json:"source"`
	Target int     `json:"target"`
	Value  float64 `json:"value"`
}
type SankeyData struct {
	Nodes []SankeyNode `json:"nodes"`
	Links []SankeyLink `json:"links"`
}

func buildSankeyData(g *graph.Graph) ([]byte, error) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	var nodes []SankeyNode
	var links []SankeyLink
	idToIndex := make(map[string]int)

	// 1. Add "The Internet" (Root)
	nodes = append(nodes, SankeyNode{Name: "Internet [0.0.0.0/0]", Waste: false})
	idToIndex["INTERNET"] = 0

	// 2. Add All Graph Nodes
	// 2. Add All Graph Nodes
	currentIndex := 1
	for _, n := range g.Nodes {
		idToIndex[n.ID] = currentIndex
		name := n.Type + " (" + extractID(n.ID) + ")"
		nodes = append(nodes, SankeyNode{Name: name, Waste: n.IsWaste})
		currentIndex++
	}

	// 3. Create Links from Edges
	// If Edge exists, we assume flow.
	// Value = Cost of Target Node (to visualize cash burn flowing)
	// If Target Cost is 0, give minimal width (0.1).
	// 3. Create Links from Edges
	for sourceIdx, edges := range g.Edges {
		if sourceIdx >= len(g.Nodes) { continue }
		sourceNode := g.Nodes[sourceIdx]
		srcIdx, ok1 := idToIndex[sourceNode.ID]
		if !ok1 {
			continue
		}

		for _, e := range edges {
			targetNode := g.GetNodeByID(e.TargetID)
			if targetNode == nil { continue }
			tgtIdx, ok2 := idToIndex[targetNode.ID]
			if !ok2 {
				continue
			}

			// Value Logic: Flow of Money
			// If target depends on source, money flows from source to target?
			// No, dependency is reverse.
			// "Internet Access" flows: IGW -> VPC -> Subnet -> Instance.
			// Visualization: "Flow of Liability".

			// Value = 1.0 (Topology) or n.Cost (Money).
			// Let's mix. Base 1 + Cost.
			val := 1.0
			if targetNode != nil {
				if targetNode.Cost > 0 {
					val += targetNode.Cost
				}
			}

			links = append(links, SankeyLink{
				Source: srcIdx,
				Target: tgtIdx,
				Value:  val,
			})
		}
	}

	// 4. Link Roots to Internet
	// Find IGWs and link INTERNET -> IGW
	for _, n := range g.Nodes {
		if n.Type == "AWS::EC2::InternetGateway" {
			links = append(links, SankeyLink{
				Source: 0, // Internet
				Target: idToIndex[n.ID],
				Value:  10.0, // Fat pipe
			})
		}
	}

	data := SankeyData{
		Nodes: nodes,
		Links: links,
	}

	return json.Marshal(data)
}

func extractID(arn string) string {
	// Simple short ID
	if len(arn) > 15 {
		// Try to find /
		// arn:aws:ec2:region:account:instance/i-1234 -> i-1234
		lastSlash := strings.LastIndex(arn, "/")
		if lastSlash != -1 && lastSlash < len(arn)-1 {
			return arn[lastSlash+1:]
		}
	}
	return arn
}
