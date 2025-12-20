package report

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// ReportData holds data for the HTML template.
type ReportData struct {
	GeneratedAt      string
	TotalWasteCost   float64
	TotalWaste       int
	TotalResources   int
	ProjectedSavings float64 // Annual
	WasteItems       []WasteItem
	JustifiedItems   []WasteItem // New selection for justified waste

	// Chart Data
	ChartLabelsJSON template.JS
	ChartValuesJSON template.JS
}

// WasteItem represents a simplified node for the report.
type WasteItem struct {
	ID        string
	Type      string
	Reason    string
	Cost      float64
	RiskScore int
	SrcLoc    string
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CloudSlash Audit Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        :root {
            --bg-dark: #0f172a;
            --bg-card: #1e293b;
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --accent: #3b82f6;
            --danger: #ef4444;
            --success: #22c55e;
            --border: #334155;
        }
        body {
            font-family: 'Inter', system-ui, -apple-system, sans-serif;
            background-color: var(--bg-dark);
            color: var(--text-primary);
            margin: 0;
            padding: 2rem;
            line-height: 1.5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        header {
            margin-bottom: 3rem;
            border-bottom: 1px solid var(--border);
            padding-bottom: 1rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        h1 { margin: 0; font-size: 2rem; font-weight: 700; letter-spacing: -0.025em; }
        .subtitle { color: var(--text-secondary); font-size: 0.875rem; }
        
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin-bottom: 3rem;
        }
        .card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 0.75rem;
            padding: 1.5rem;
        }
        .metric-label { font-size: 0.875rem; color: var(--text-secondary); display: block; margin-bottom: 0.5rem; }
        .metric-value { font-size: 2rem; font-weight: 700; }
        .metric-value.money { color: var(--success); }
        .metric-value.count { color: var(--danger); }

        .charts-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 1.5rem;
            margin-bottom: 3rem;
        }
        .chart-container {
            position: relative;
            height: 300px;
        }

        table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.875rem;
            text-align: left;
        }
        th {
            background: var(--bg-card);
            padding: 1rem;
            font-weight: 600;
            color: var(--text-secondary);
            border-bottom: 1px solid var(--border);
        }
        td {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            color: var(--text-primary);
        }
        tr:hover td { background: rgba(255,255,255,0.02); }
        .badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
            background: rgba(59, 130, 246, 0.2);
            color: #60a5fa;
        }
        .badge.high-risk { background: rgba(239, 68, 68, 0.2); color: #f87171; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div>
                <h1>CloudSlash Infrastructure Audit</h1>
                <div class="subtitle">Generated on {{.GeneratedAt}}</div>
            </div>
            <div>
                <span class="badge">Enterprise Edition</span>
            </div>
        </header>

        <div class="grid">
            <div class="card">
                <span class="metric-label">Monthly Potential Savings</span>
                <div class="metric-value money">${{printf "%.2f" .TotalWasteCost}}</div>
            </div>
            <div class="card">
                <span class="metric-label">Projected Annual Savings</span>
                <div class="metric-value money">${{printf "%.2f" .ProjectedSavings}}</div>
            </div>
            <div class="card">
                <span class="metric-label">Actionable Waste</span>
                <div class="metric-value count">{{.TotalWaste}} / {{.TotalResources}}</div>
            </div>
            <div class="card" style="opacity: 0.7;">
                <span class="metric-label">Justified / Ignored Risks</span>
                <div class="metric-value">{{len .JustifiedItems}}</div>
            </div>
        </div>

        <div class="charts-grid">
            <div class="card">
                <h3 style="margin-top:0;">Cost by Resource Type</h3>
                <div class="chart-container">
                    <canvas id="costChart"></canvas>
                </div>
            </div>
            <div class="card">
                <h3 style="margin-top:0;">Waste Utilization</h3>
                <div class="chart-container" style="display:flex; justify-content:center;">
                   <div style="width: 250px; height: 250px;">
                        <canvas id="utilChart"></canvas>
                   </div>
                </div>
            </div>
        </div>

        <div class="card">
            <h2 style="margin-top:0; margin-bottom:1.5rem;">Waste Details</h2>
            <table>
                <thead>
                    <tr>
                        <th>Resource ID</th>
                        <th>Type</th>
                        <th>Risk Score</th>
                        <th>Est. Monthly Cost</th>
                        <th>Source</th>
                        <th>Reason</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .WasteItems}}
                    <tr>
                        <td style="font-family: monospace;">{{.ID}}</td>
                        <td><span class="badge">{{.Type}}</span></td>
                        <td>
                            {{if ge .RiskScore 80}}
                                <span class="badge high-risk">{{.RiskScore}}</span>
                            {{else}}
                                {{.RiskScore}}
                            {{end}}
                        </td>
                        <td>${{printf "%.2f" .Cost}}</td>
                        <td style="font-family: monospace; font-size: 0.8em; color: var(--accent);">{{.SrcLoc}}</td>
                        <td>{{.Reason}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="6" style="text-align: center; padding: 2rem; color: var(--text-secondary);">No waste identified. Infrastructure is optimized.</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        {{if .JustifiedItems}}
        <div class="card" style="margin-top: 3rem; opacity: 0.8;">
            <h2 style="margin-top:0; margin-bottom:1.5rem; color: var(--text-secondary);">Justified Risks (Excluded from Remediation)</h2>
            <table>
                <thead>
                    <tr>
                        <th>Resource ID</th>
                        <th>Type</th>
                        <th>Justification</th>
                        <th>Cost</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .JustifiedItems}}
                    <tr>
                        <td style="font-family: monospace;">{{.ID}}</td>
                        <td><span class="badge">{{.Type}}</span></td>
                        <td>{{.Reason}}</td>
                        <td>${{printf "%.2f" .Cost}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}

        <div class="card" style="margin-top: 3rem;">
            <h2 style="margin-top:0; margin-bottom:1rem;">Recommended Actions</h2>
            <div class="grid" style="margin-bottom:0;">
                 <div>
                    <h3 style="margin-top:0; color: var(--accent);">Remediation</h3>
                    <p style="color: var(--text-secondary); font-size: 0.875rem;">Run <code>bash cloudslash-out/safe_cleanup.sh</code> to snapshot and delete identified waste.</p>
                </div>
                 <div>
                    <h3 style="margin-top:0; color: var(--text-primary);">Suppression</h3>
                    <p style="color: var(--text-secondary); font-size: 0.875rem;">Run <code>bash cloudslash-out/ignore_resources.sh</code> to tag resources as ignored (<code>cloudslash:ignore</code>).</p>
                </div>
                 <div>
                    <h3 style="margin-top:0; color: var(--text-primary);">Terraform</h3>
                    <p style="color: var(--text-secondary); font-size: 0.875rem;">Run <code>bash cloudslash-out/fix_terraform.sh</code> to remove waste from your state file (Pro Only).</p>
                </div>
            </div>
        </div>
        
        <footer style="margin-top: 3rem; text-align: center; color: var(--text-secondary); font-size: 0.75rem;">
            Generated by CloudSlash Enterprise. Confidential & Proprietary.
        </footer>
    </div>

    <script>
        // Chart Data
        const chartLabels = {{.ChartLabelsJSON}};
        const chartValues = {{.ChartValuesJSON}};
        
        // 1. Bar Chart: Cost by Type
        const ctxCost = document.getElementById('costChart').getContext('2d');
        new Chart(ctxCost, {
            type: 'bar',
            data: {
                labels: chartLabels,
                datasets: [{
                    label: 'Monthly Cost ($)',
                    data: chartValues,
                    backgroundColor: 'rgba(59, 130, 246, 0.6)',
                    borderColor: 'rgba(59, 130, 246, 1)',
                    borderWidth: 1
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                         grid: { color: '#334155' }
                    },
                     x: {
                         grid: { display: false }
                    }
                },
                plugins: {
                    legend: { display: false }
                }
            }
        });

        // 2. Donut Chart: Waste vs Non-Waste (Simulated for this view, using Type distribution)
        const ctxUtil = document.getElementById('utilChart').getContext('2d');
        new Chart(ctxUtil, {
            type: 'doughnut',
            data: {
                labels: chartLabels,
                datasets: [{
                    data: chartValues,
                    backgroundColor: [
                        '#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#6366f1'
                    ],
                    borderWidth: 0
                }]
            },
             options: {
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'bottom' }
                }
            }
        });
        
        // Dark Mode defaults
        Chart.defaults.color = '#94a3b8';
        Chart.defaults.borderColor = '#334155';
    </script>
</body>
</html>
`

// GenerateHTML creates the report file.
func GenerateHTML(g *graph.Graph, outputPath string) error {
	data := ReportData{
		GeneratedAt: time.Now().Format(time.RFC822),
	}

	// Aggregate for Charts
	costByType := make(map[string]float64)

	g.Mu.RLock()
	data.TotalResources = len(g.Nodes)
	for _, node := range g.Nodes {
		if node.IsWaste {
			// Short Type Name
			parts := strings.Split(node.Type, "::")
			shortType := parts[len(parts)-1]

			// Simple ID formatting
			idShort := node.ID
			if parts := strings.Split(node.ID, "/"); len(parts) > 1 {
				idShort = parts[len(parts)-1] // Just the ID part of ARN
			}

			reason := ""
			if r, ok := node.Properties["Reason"].(string); ok {
				reason = r
			}

			item := WasteItem{
				ID:        idShort,
				Type:      shortType,
				Reason:    reason, // Default reason
				Cost:      node.Cost,
				RiskScore: node.RiskScore,
				SrcLoc:    node.SourceLocation, // Populate Source Location
			}

			if node.Justified {
				item.Reason = node.Justification // Override reason with justification
				data.JustifiedItems = append(data.JustifiedItems, item)
			} else {
				data.TotalWaste++
				data.TotalWasteCost += node.Cost
				costByType[shortType] += node.Cost
				data.WasteItems = append(data.WasteItems, item)
			}
		}
	}
	g.Mu.RUnlock()

	data.ProjectedSavings = data.TotalWasteCost * 12

	// Prepare Chart Data (Sorted by Cost)
	type costEntry struct {
		Type string
		Cost float64
	}
	var costs []costEntry
	for k, v := range costByType {
		costs = append(costs, costEntry{k, v})
	}
	sort.Slice(costs, func(i, j int) bool { return costs[i].Cost > costs[j].Cost })

	var labels []string
	var values []float64
	for _, c := range costs {
		labels = append(labels, c.Type)
		values = append(values, c.Cost)
	}

	// JSON Marshal helper (manual simple string built to avoid import complexities inside template calc)
	// Actually template.JS requires strings.
	// For simplicity, we'll simple json via fmt or just import encoding/json above?
	// Let's import encoding/json to be safe.

	// wait I need to add import encoding/json

	// Quick Fix: manual json construction for array of strings/floats is easy.
	// Labels: ["Item1", "Item2"]
	labelsStr := "["
	for i, l := range labels {
		if i > 0 {
			labelsStr += ","
		}
		labelsStr += fmt.Sprintf("\"%s\"", l)
	}
	labelsStr += "]"

	valuesStr := "["
	for i, v := range values {
		if i > 0 {
			valuesStr += ","
		}
		valuesStr += fmt.Sprintf("%.2f", v)
	}
	valuesStr += "]"

	data.ChartLabelsJSON = template.JS(labelsStr)
	data.ChartValuesJSON = template.JS(valuesStr)

	// Sort Items by Cost descending
	sort.Slice(data.WasteItems, func(i, j int) bool {
		return data.WasteItems[i].Cost > data.WasteItems[j].Cost
	})

	t, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}
