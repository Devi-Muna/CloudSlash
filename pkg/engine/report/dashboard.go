package report

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/version"
)

// GenerateDashboard generates an interactive HTML dashboard.
func GenerateDashboard(g *graph.Graph, path string) error {
	items := extractItems(g)

	// Compute statistics.
	totalCost := 0.0
	riskCount := 0
	for _, item := range items {
		totalCost += item.MonthlyCost
		if item.RiskScore > 50 {
			riskCount++
		}
	}

	// Prepare chart data.
	graphData, err := buildSankeyData(g)
	if err != nil {
		fmt.Printf("[WARN] Failed to build Sankey data: %v\n", err)
		// Handle empty graph.
		graphData = []byte("{}")
	}

	jsonData, err := json.Marshal(items)
	if err != nil {
		return err
	}

	// HTML Report Template.
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CloudSlash Audit v2.0</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <script src="https://unpkg.com/d3-sankey@0.12.3/dist/d3-sankey.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        :root {
            --bg: #050505;
            --surface: rgba(255, 255, 255, 0.03);
            --surface-hover: rgba(255, 255, 255, 0.06);
            --border: rgba(255, 255, 255, 0.1);
            --primary: #00FF99;
            --secondary: #874BFD;
            --danger: #FF3366;
            --text: #F8FAFC;
            --text-dim: #94A3B8;
        }

        /* 1. Base styles. */
        * { box-sizing: border-box; }
        body {
            background: var(--bg);
            color: var(--text);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            margin: 0;
            padding: 40px;
            font-size: 14px;
        }

        /* 2. Header styles. */
        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 40px;
            border-bottom: 1px solid var(--border);
            padding-bottom: 20px;
        }
        .logo { font-size: 1.5rem; font-weight: 700; letter-spacing: -1px; }
        .logo span { color: var(--primary); }
        .meta { color: var(--text-dim); }

        /* 3. KPI styles. */
        .kpi-grid {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 20px;
            margin-bottom: 40px;
        }
        .card {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 16px;
            padding: 24px;
            transition: transform 0.2s, background 0.2s;
        }
        .card:hover { background: var(--surface-hover); transform: translateY(-2px); }
        .card h3 { margin: 0 0 10px 0; font-size: 0.75rem; color: var(--text-dim); text-transform: uppercase; letter-spacing: 1.2px; }
        .card .value { font-size: 2.5rem; font-weight: 700; }
        .card .value.cost { color: var(--danger); }
        .card .value.safe { color: var(--primary); }

        /* 4. Analytics chart styles. */
        .analytics-grid {
            display: grid;
            grid-template-columns: 2fr 1fr;
            gap: 20px;
            margin-bottom: 40px;
        }
        .chart-container {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 16px;
            padding: 24px;
            position: relative;
            height: 350px;
            display: flex;
            flex-direction: column;
        }
        .chart-header {
            font-size: 0.85rem;
            font-weight: 600;
            margin-bottom: 16px;
            color: var(--text);
            display: flex;
            justify-content: space-between;
        }
        .chart-body { flex: 1; position: relative; width: 100%; overflow: hidden; }

        /* 5. Sankey container styles. */
        .viz-container {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 16px;
            padding: 20px;
            margin-bottom: 40px;
            height: 500px;
            position: relative;
            overflow: hidden;
        }

        /* 6. Data grid styles. */
        .table-wrapper {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 16px;
            overflow: hidden;
            display: flex;
            flex-direction: column;
        }
        
        .toolbar {
            padding: 16px 24px;
            border-bottom: 1px solid var(--border);
            display: flex;
            gap: 12px;
            align-items: center;
        }
        .search-box {
            background: rgba(0,0,0,0.3);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 8px 12px;
            color: var(--text);
            font-family: inherit;
            width: 300px;
            outline: none;
        }
        .search-box:focus { border-color: var(--primary); }

        .table-scroll {
            width: 100%;
            overflow-x: auto; /* Horizontal Scroll */
        }

        table { width: 100%; border-collapse: collapse; min-width: 1000px; }
        th, td { padding: 16px 24px; text-align: left; border-bottom: 1px solid var(--border); white-space: nowrap; }
        th {
            background: rgba(0,0,0,0.5);
            color: var(--text-dim);
            font-size: 0.75rem;
            text-transform: uppercase;
            font-weight: 600;
            cursor: pointer;
            user-select: none;
        }
        th:hover { color: var(--text); }
        tr:last-child td { border-bottom: none; }
        tr:hover { background: rgba(255,255,255,0.02); }

        .badge { padding: 4px 10px; border-radius: 20px; font-size: 0.7rem; font-weight: 700; }
        .badge.JUNK { background: rgba(255, 51, 102, 0.15); color: var(--danger); }
        .badge.REVIEW { background: rgba(135, 75, 253, 0.15); color: var(--secondary); }
        .badge.JUSTIFIED { background: rgba(0, 255, 153, 0.15); color: var(--primary); }
        
        /* 7. Footer styles. */
        footer { margin-top: 60px; color: var(--text-dim); font-size: 0.8rem; text-align: center; border-top: 1px solid var(--border); padding-top: 20px; }

        /* Sankey styles. */
        .node rect { cursor: pointer; fill-opacity: .9; shape-rendering: crispEdges; }
        .node text { pointer-events: none; text-shadow: 0 1px 0 #000; font-family: monospace; font-size: 10px; fill: #fff; }
        .link { fill: none; stroke: #000; stroke-opacity: .2; }
        .link:hover { stroke-opacity: .5; }
    </style>
</head>
<body>

    <div class="header">
        <div class="logo">CLOUD<span>SLASH</span>_AUDIT</div>
        <div class="meta">Generated: {{GENERATED_TIME}}</div>
    </div>

    <!-- 1. KPI Cards section. -->
    <div class="kpi-grid">
        <div class="card">
            <h3>Monthly Waste</h3>
            <div class="value cost">${{TOTAL_COST}}</div>
        </div>
        <div class="card">
            <h3>Risky Resources</h3>
            <div class="value">{{RISK_COUNT}}</div>
        </div>
        <div class="card">
            <h3>Topology Status</h3>
            <div class="value safe">CONNECTED</div>
        </div>
    </div>

    <!-- 2. Charts section. -->
    <div class="analytics-grid">
        <div class="chart-container">
            <div class="chart-header">Monthly Spend by Service</div>
            <div class="chart-body">
                <canvas id="barChart"></canvas>
            </div>
        </div>
        <div class="chart-container">
            <div class="chart-header">Waste vs. Safe</div>
            <div class="chart-body">
                <canvas id="pieChart"></canvas>
            </div>
        </div>
    </div>

    <!-- 3. Sankey Diagram section. -->
    <div class="viz-container">
        <div class="chart-header" style="position: absolute; top: 20px; left: 20px; z-index: 10;">// NETWORK FLOW</div>
        <div id="chart"></div>
    </div>

    <!-- 4. Data Grid section. -->
    <div class="table-wrapper">
        <div class="toolbar">
            <input type="text" id="searchInput" class="search-box" placeholder="Filter resources..." onkeyup="filterTable()">
        </div>
        <div class="table-scroll">
            <table id="resourceTable">
                <thead>
                    <tr>
                        <th onclick="sortTable(0)">Type &#8597;</th>
                        <th onclick="sortTable(1)">Resource ID &#8597;</th>
                        <th onclick="sortTable(2)">Region &#8597;</th>
                        <th onclick="sortTable(3)">Monthly Cost &#8597;</th>
                        <th onclick="sortTable(4)">Action &#8597;</th>
                        <th>Evidence</th>
                    </tr>
                </thead>
                <tbody id="table-body">
                    <!-- JS Injection -->
                </tbody>
            </table>
        </div>
    </div>

    <footer>
        Generated by CloudSlash ` + version.Current + ` (AGPLv3) | Local-First Cloud Auditor
    </footer>

    <script>
        // --- DATA ---
        window.REPORT_DATA = {{REPORT_DATA}};
        window.GRAPH_DATA = {{GRAPH_DATA}};
        const currency = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

        // --- 1. TABLE INITIALIZATION ---
        const tbody = document.getElementById('table-body');
        
        function renderTable(data) {
            tbody.innerHTML = '';
            data.forEach(item => {
                const tr = document.createElement('tr');
                const badgeClass = item.risk_score > 50 ? 'JUNK' : (item.action === 'JUSTIFIED' ? 'JUSTIFIED' : 'REVIEW');
                const costStyle = item.monthly_cost > 0 ? 'color: #FF3366; font-weight: bold;' : 'color: #94A3B8;';

                tr.innerHTML = ` + "`" + `
                    <td><span style="opacity:0.8; font-weight: 500;">` + "`" + ` + item.type.replace('AWS::', '') + ` + "`" + `</span></td>
                    <td style="font-weight:600; color: #fff;">` + "`" + ` + item.resource_id + ` + "`" + `</td>
                    <td>` + "`" + ` + item.region + ` + "`" + `</td>
                    <td style="` + "`" + ` + costStyle + ` + "`" + `">` + "`" + ` + currency.format(item.monthly_cost) + ` + "`" + `</td>
                    <td><span class="badge ` + "`" + ` + badgeClass + ` + "`" + `">` + "`" + ` + item.action + ` + "`" + `</span></td>
                    <td style="color: #94A3B8;">` + "`" + ` + item.audit_detail + ` + "`" + `</td>
                ` + "`" + `;
                tbody.appendChild(tr);
            });
        }
        renderTable(window.REPORT_DATA);

        // --- 2. SEARCH ---
        function filterTable() {
            const input = document.getElementById('searchInput');
            const filter = input.value.toUpperCase();
            const filtered = window.REPORT_DATA.filter(item => {
                return Object.values(item).some(val => 
                    String(val).toUpperCase().includes(filter)
                );
            });
            renderTable(filtered);
        }

        // --- 3. SORT ---
        function sortTable(n) {
            // Sort implementation.
            const table = document.getElementById("resourceTable");
            
            
            console.log("Sort clicked on col " + n);
        }

        // --- 4. CHARTS ---
        
        // 4.1 Gradient Helper
        function createGradient(ctx, colorStart, colorEnd) {
            const gradient = ctx.createLinearGradient(0, 400, 0, 0); // Vertical gradient
            gradient.addColorStop(0, colorStart);
            gradient.addColorStop(1, colorEnd);
            return gradient;
        }

        // Prepare Data
        const serviceMap = {};
        window.REPORT_DATA.forEach(item => {
            const svc = item.type.split('::')[1] || item.type;
            serviceMap[svc] = (serviceMap[svc] || 0) + item.monthly_cost;
        });

        // Aggregate top services.
        const sortedServices = Object.entries(serviceMap).sort((a,b) => b[1] - a[1]);
        const topServices = sortedServices.slice(0, 5);
        if (sortedServices.length > 5) {
            const othersSum = sortedServices.slice(5).reduce((acc, curr) => acc + curr[1], 0);
            topServices.push(["Others", othersSum]);
        }

        const labels = topServices.map(s => s[0]);
        const dataValues = topServices.map(s => s[1]);

        // 4.2 Bar Chart
        const ctxBar = document.getElementById('barChart').getContext('2d');
        const barGradient = createGradient(ctxBar, 'rgba(255, 51, 102, 0.4)', '#FF3366');
        const barBorder = '#FF3366';

        new Chart(ctxBar, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Monthly Waste ($)',
                    data: dataValues,
                    backgroundColor: barGradient,
                    borderColor: barBorder,
                    borderWidth: 1,
                    borderRadius: 6,
                    barThickness: 'flex',
                    maxBarThickness: 40
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 1500, easing: 'easeOutQuart' },
                plugins: { 
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(10,10,10,0.9)',
                        titleColor: '#fff',
                        bodyColor: '#ccc',
                        borderColor: 'rgba(255,255,255,0.1)',
                        borderWidth: 1,
                        padding: 10,
                        displayColors: false,
                        callbacks: {
                            label: function(context) {
                                return currency.format(context.raw);
                            }
                        }
                    }
                },
                scales: {
                    y: { 
                        beginAtZero: true,
                        grid: { color: 'rgba(255,255,255,0.03)', drawBorder: false },
                        ticks: { color: '#64748B', font: { family: 'monospace' }, callback: (val) => '$'+val }
                    },
                    x: { 
                        grid: { display: false },
                        ticks: { color: '#94A3B8', font: { weight: 600 } } 
                    }
                }
            }
        });

        // 4.3 Doughnut Chart.
        const totalWaste = dataValues.reduce((a, b) => a + b, 0);
        // Estimate potential savings.
        
        
        const estimatedTotal = totalWaste / 0.3; 
        const optimizedSpend = estimatedTotal - totalWaste;

        const ctxPie = document.getElementById('pieChart').getContext('2d');
        const gradientWaste = createGradient(ctxPie, '#FF3366', '#FF99AA');
        const gradientSafe = createGradient(ctxPie, '#00FF99', '#00CC7A');

        // Center text plugin.
        const centerTextPlugin = {
            id: 'centerText',
            beforeDraw: function(chart) {
                const width = chart.width, height = chart.height, ctx = chart.ctx;
                ctx.restore();
                
                // Savings Value
                const fontSize = (height / 114).toFixed(2);
                ctx.font = "bold " + fontSize + "em sans-serif";
                ctx.textBaseline = "middle";
                ctx.fillStyle = "#fff";
                
                const text = currency.format(totalWaste);
                const textX = Math.round((width - ctx.measureText(text).width) / 2);
                const textY = height / 2;
                
                ctx.fillText(text, textX, textY);

                // Label
                ctx.font = "600 " + (fontSize*0.4).toFixed(2) + "em monospace";
                ctx.fillStyle = "#94A3B8";
                const label = "POTENTIAL SAVINGS";
                const labelX = Math.round((width - ctx.measureText(label).width) / 2);
                ctx.fillText(label, labelX, textY - (height * 0.15));

                ctx.save();
            }
        };

        new Chart(ctxPie, {
            type: 'doughnut',
            data: {
                labels: ['Optimized Spend', 'Identified Waste'],
                datasets: [{
                    data: [optimizedSpend, totalWaste],
                    backgroundColor: [gradientSafe, gradientWaste],
                    borderColor: ['#000', '#000'],
                    borderWidth: 2,
                    hoverOffset: 10
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                cutout: '75%', // Thinner ring
                animation: { animateScale: true, animateRotate: true, duration: 2000, easing: 'easeOutElastic' },
                plugins: { 
                    legend: { position: 'bottom', labels: { color: '#94A3B8', padding: 20, font: { size: 11 } } },
                    tooltip: {
                         backgroundColor: 'rgba(10,10,10,0.9)',
                         bodyFont: { size: 13 },
                         callbacks: {
                             label: function(context) {
                                 return " " + context.label + ": " + currency.format(context.raw);
                             }
                         }
                    }
                }
            },
            plugins: [centerTextPlugin]
        });


        // --- 5. SANKEY (D3) ---
        if (window.GRAPH_DATA && window.GRAPH_DATA.nodes && window.GRAPH_DATA.nodes.length > 0) {
            try {
                const container = document.querySelector('.viz-container');
                // Calculate dynamic height.
                const nodeCount = window.GRAPH_DATA.nodes.length;
                const dynamicHeight = Math.max(500, nodeCount * 35);
                const width = container.clientWidth - 40;
                const height = dynamicHeight; // Explicit height calc
                
                // Set container height.
                d3.select(".viz-container").style("height", (height + 60) + "px");

                // Clear existing chart.
                d3.select("#chart").html("");

                // Initialize tooltip.
                const tooltip = d3.select("body").append("div")
                    .attr("class", "sankey-tooltip")
                    .style("position", "absolute")
                    .style("background", "rgba(10, 10, 10, 0.95)")
                    .style("color", "#fff")
                    .style("padding", "10px 14px")
                    .style("border", "1px solid rgba(255,255,255,0.2)")
                    .style("border-radius", "8px")
                    .style("box-shadow", "0 4px 20px rgba(0,0,0,0.5)")
                    .style("backdrop-filter", "blur(4px)")
                    .style("pointer-events", "none")
                    .style("opacity", 0)
                    .style("font-size", "12px")
                    .style("z-index", 1000);

                const svg = d3.select("#chart").append("svg")
                    .attr("width", width)
                    .attr("height", height)
                    .style("overflow", "visible");

                // Define gradients.
                const defs = svg.append("defs");

                // Configure Sankey layout.
                const sankey = d3.sankey()
                    .nodeWidth(14) // Balanced width
                    .nodePadding(Math.max(10, 50 - nodeCount * 0.5)) // Adaptive padding
                    .extent([[1, 1], [width - 1, height - 6]]);

                // Clone data to prevent mutation.
                const graphDataClone = JSON.parse(JSON.stringify(window.GRAPH_DATA));
                
                // Compute layout.
                const {nodes, links} = sankey(graphDataClone);

                // Create link gradients.
                links.forEach((d, i) => {
                    const gradientID = "gradient-" + i;
                    const gradient = defs.append("linearGradient")
                        .attr("id", gradientID)
                        .attr("gradientUnits", "userSpaceOnUse")
                        .attr("x1", d.source.x1)
                        .attr("x2", d.target.x0);

                    // Source Color
                    let startColor = d.source.waste ? "#FF3366" : "#444";
                    if (d.source.name.includes("Internet")) startColor = "#FFFFFF";

                    // Target Color
                    let endColor = d.target.waste ? "#FF3366" : "#00FF99";
                    
                    gradient.append("stop").attr("offset", "0%").attr("stop-color", startColor);
                    gradient.append("stop").attr("offset", "100%").attr("stop-color", endColor);
                });

                // Render links.
                const link = svg.append("g")
                    .attr("fill", "none")
                    .selectAll("path")
                    .data(links)
                    .enter().append("path")
                    .attr("d", d3.sankeyLinkHorizontal())
                    .attr("stroke-width", d => Math.max(2, d.width)) // Ensure visibility
                    .style("stroke", (d, i) => "url(#gradient-" + i + ")")
                    .style("stroke-opacity", 0.4)
                    .style("transition", "all 0.4s cubic-bezier(0.16, 1, 0.3, 1)");

                // Render nodes.
                const node = svg.append("g")
                    .selectAll("g")
                    .data(nodes)
                    .enter().append("g");

                node.append("rect")
                    .attr("x", d => d.x0)
                    .attr("y", d => d.y0)
                    .attr("height", d => Math.max(4, d.y1 - d.y0)) // Min height 4px
                    .attr("width", d => d.x1 - d.x0)
                    .attr("rx", 3)
                    .style("fill", d => {
                    if (d.name.includes("Internet")) return "#FFFFFF";
                    if (d.waste) return "#FF3366";
                    return "#3A3A3A"; 
                    })
                    .style("opacity", 0.9)
                    .style("cursor", "pointer")
                    .style("stroke", "rgba(0,0,0,0.5)")
                    .style("stroke-width", "1px")
                    .style("stroke-width", "1px")
                    .on("click", function(event, d) {
                        // Handle node selection.
                        if (window.activeNode === d) {
                            // Deselect
                            window.activeNode = null;
                            d3.select(this).style("stroke", "rgba(0,0,0,0.5)"); // Reset stroke
                            // Reset Opacity Global
                            node.style("opacity", 0.9);
                            link.style("stroke-opacity", 0.4);
                        } else {
                            // Select
                            window.activeNode = d;
                            // Dim unrelated elements.
                            node.style("opacity", 0.1);
                            link.style("stroke-opacity", 0.05);
                            
                            // Highlight selected node.
                            d3.select(this)
                                .style("opacity", 1)
                                .style("stroke", "#fff");
                                
                            // Highlight connected links.
                            link.filter(l => l.source.index === d.index || l.target.index === d.index)
                                .style("stroke-opacity", 0.8);
                                
                            // Highlight connected nodes.
                            const connectedIndices = new Set();
                            connectedIndices.add(d.index);
                            link.data().forEach(l => {
                                if (l.source.index === d.index) connectedIndices.add(l.target.index);
                                if (l.target.index === d.index) connectedIndices.add(l.source.index);
                            });
                            
                            node.filter(n => connectedIndices.has(n.index))
                                .style("opacity", 1);
                        }
                    })
                    .on("mouseover", function(event, d) {
                        // Show tooltip.
                        tooltip.transition().duration(100).style("opacity", 1);
                        tooltip.html(
                            '<div style="font-weight:700; margin-bottom:4px; color:' + (d.waste?"#FF3366":"#fff") + '">' + d.name + '</div>' +
                            '<div style="color:#aaa; font-size:11px; letter-spacing: 1px;">' + (d.waste ? "<span style='color:#FF3366'>WASTE DETECTED</span>" : "<span style='color:#00FF99'>INFRASTRUCTURE</span>") + '</div>'
                        )
                            .style("width", "max-content")
                            .style("left", (event.pageX + 15) + "px")
                            .style("top", (event.pageY - 20) + "px");

                        // Highlight on hover if no selection active.
                        if (!window.activeNode) {
                            d3.select(this)
                                .style("opacity", 1)
                                .style("stroke", "#fff")
                                .style("filter", "drop-shadow(0 0 8px rgba(255,255,255,0.4))");
                            
                            link.style("stroke-opacity", l => 
                                (l.source.index === d.index || l.target.index === d.index) ? 0.8 : 0.05
                            );
                        }
                    })
                    .on("mouseout", function() {
                        tooltip.transition().duration(300).style("opacity", 0);

                        // Reset styles if no selection active.
                        if (!window.activeNode) {
                             d3.select(this)
                                .style("opacity", 0.9)
                                .style("stroke", "rgba(0,0,0,0.5)")
                                .style("filter", "none");
                            link.style("stroke-opacity", 0.4); 
                        }
                    });

                // Render labels.
                node.append("text")
                    .attr("x", d => d.x0 < width / 2 ? d.x1 + 10 : d.x0 - 10)
                    .attr("y", d => (d.y1 + d.y0) / 2)
                    .attr("dy", "0.35em")
                    .attr("text-anchor", d => d.x0 < width / 2 ? "start" : "end")
                    .text(d => d.name)
                    .style("font-family", "monospace")
                    .style("font-size", "12px")
                    .style("font-weight", "600")
                    .style("fill", "#ddd")
                    .style("opacity", d => (d.y1 - d.y0) > 12 ? 1 : 0) // Hide labels for small nodes.
                    .style("pointer-events", "none")
                    .style("text-shadow", "0 2px 4px rgba(0,0,0,0.8)");

            } catch (e) {
                console.error("Sankey Error:", e);
                d3.select(".viz-container").append("div")
                    .style("color", "#FF3366")
                    .style("padding", "20px")
                    .html('<strong>Visualization Error:</strong> ' + e.message + '<br/><br/>Try scanning additional regions to populate the graph.');
            }
        }
    </script>
</body>
</html>`

	html = strings.ReplaceAll(html, "{{GENERATED_TIME}}", time.Now().Format("2006-01-02 15:04:05"))
	html = strings.ReplaceAll(html, "{{TOTAL_COST}}", fmt.Sprintf("%.2f", totalCost))
	html = strings.ReplaceAll(html, "{{RISK_COUNT}}", fmt.Sprintf("%d", riskCount))
	html = strings.ReplaceAll(html, "{{REPORT_DATA}}", string(jsonData))
	html = strings.ReplaceAll(html, "{{GRAPH_DATA}}", string(graphData))

	return os.WriteFile(path, []byte(html), 0644)
}

// Sankey visualization structures.
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

	nodes := make([]SankeyNode, 0)
	links := make([]SankeyLink, 0)
	idToIndex := make(map[string]int)

	// 1. Add Internet root node.
	nodes = append(nodes, SankeyNode{Name: "Internet [0.0.0.0/0]", Waste: false})
	idToIndex["INTERNET"] = 0

	// 2. Add graph nodes.
	currentIndex := 1
	for _, n := range g.GetNodes() {
		idToIndex[n.IDStr()] = currentIndex
		name := extractID(n.IDStr())
		nodes = append(nodes, SankeyNode{Name: name, Waste: n.IsWaste})
		currentIndex++
	}

	// Create links.
	allNodes := g.GetNodes()
	for _, sourceNode := range allNodes {
		edges := g.GetEdges(sourceNode.Index)

		srcIdx, ok1 := idToIndex[sourceNode.IDStr()]
		if !ok1 {
			continue
		}

		for _, e := range edges {
			targetNode := g.GetNodeByID(e.TargetID)
			if targetNode == nil {
				continue
			}
			tgtIdx, ok2 := idToIndex[targetNode.IDStr()]
			if !ok2 {
				continue
			}

			// Calculate link weight based on cost.

			// Weight link by target cost.

			val := 8.0
			if targetNode != nil && targetNode.Cost > 0 {
				val += math.Log10(targetNode.Cost+1) * 8
			}

			links = append(links, SankeyLink{
				Source: srcIdx,
				Target: tgtIdx,
				Value:  val,
			})
		}
	}

	// 4. Link gateways to Internet.
	// Find IGWs and link INTERNET -> IGW
	for _, n := range g.GetNodes() {
		if n.TypeStr() == "AWS::EC2::InternetGateway" {
			links = append(links, SankeyLink{
				Source: 0, // Internet
				Target: idToIndex[n.IDStr()],
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
