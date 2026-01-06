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
    <title>CloudSlash // Infrastructure Forensics</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-deep: #030712;
            --bg-glass: rgba(17, 24, 39, 0.7);
            --border-glass: rgba(255, 255, 255, 0.08);
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --accent-glow: #3b82f6;
            --danger-glow: #ef4444;
            --success-glow: #10b981;
            --font-sans: 'Outfit', sans-serif;
            --font-mono: 'JetBrains Mono', monospace;
        }

        body {
            background-color: var(--bg-deep);
            background-image: 
                radial-gradient(at 0% 0%, rgba(59, 130, 246, 0.15) 0px, transparent 50%),
                radial-gradient(at 100% 0%, rgba(239, 68, 68, 0.1) 0px, transparent 50%);
            color: var(--text-primary);
            font-family: var(--font-sans);
            margin: 0;
            padding: 3rem;
            min-height: 100vh;
            -webkit-font-smoothing: antialiased;
        }

        /* Glassmorphism Logic */
        .glass-panel {
            background: var(--bg-glass);
            backdrop-filter: blur(24px);
            -webkit-backdrop-filter: blur(24px);
            border: 1px solid var(--border-glass);
            border-radius: 1.5rem;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
            transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
        }

        .glass-panel:hover {
            border-color: rgba(255, 255, 255, 0.15);
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
        }

        /* Header Architecture */
        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 4rem;
            padding-bottom: 2rem;
            border-bottom: 1px solid var(--border-glass);
            position: relative;
        }

        header::after {
            content: '';
            position: absolute;
            bottom: -1px;
            left: 0;
            width: 200px;
            height: 1px;
            background: linear-gradient(90deg, var(--accent-glow), transparent);
        }

        h1 {
            font-size: 3.5rem;
            font-weight: 700;
            letter-spacing: -0.05em;
            margin: 0;
            background: linear-gradient(135deg, #fff 0%, #94a3b8 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .meta-tag {
            font-family: var(--font-mono);
            font-size: 0.875rem;
            color: var(--accent-glow);
            text-transform: uppercase;
            letter-spacing: 0.1em;
            background: rgba(59, 130, 246, 0.1);
            padding: 0.5rem 1rem;
            border-radius: 99px;
            border: 1px solid rgba(59, 130, 246, 0.2);
        }

        /* KPI Grid */
        .kpi-grid {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 1.5rem;
            margin-bottom: 3rem;
        }

        .kpi-card {
            padding: 2rem;
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            height: 160px;
        }

        .kpi-label {
            font-size: 0.875rem;
            color: var(--text-secondary);
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .kpi-value {
            font-size: 3rem;
            font-weight: 700;
            font-family: var(--font-mono);
            letter-spacing: -0.05em;
        }

        .text-gradient-success {
            background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .text-gradient-danger {
            background: linear-gradient(135deg, #f87171 0%, #ef4444 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        /* Charts Section */
        .analytics-grid {
            display: grid;
            grid-template-columns: 2fr 1fr;
            gap: 1.5rem;
            margin-bottom: 3rem;
        }

        .chart-card {
            padding: 2rem;
            min-height: 400px;
        }

        .card-header {
            margin-bottom: 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .card-title {
            font-size: 1.25rem;
            font-weight: 600;
            margin: 0;
        }

        /* Data Table */
        .table-container {
            overflow: hidden;
        }

        table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 0;
        }

        th {
            text-align: left;
            padding: 1.5rem;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.1em;
            color: var(--text-secondary);
            border-bottom: 1px solid var(--border-glass);
            background: rgba(17, 24, 39, 0.5);
        }

        td {
            padding: 1.5rem;
            border-bottom: 1px solid var(--border-glass);
            vertical-align: middle;
            transition: background 0.1s;
        }

        tr:last-child td { border-bottom: none; }
        
        tr:hover td {
            background: rgba(255, 255, 255, 0.02);
        }

        .resource-id {
            font-family: var(--font-mono);
            color: var(--text-primary);
            font-size: 0.9rem;
        }

        .type-pill {
            display: inline-flex;
            align-items: center;
            padding: 0.25rem 0.75rem;
            border-radius: 99px;
            font-size: 0.75rem;
            font-weight: 600;
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .risk-score {
            font-family: var(--font-mono);
            font-weight: 700;
        }

        .risk-high { color: #f87171; text-shadow: 0 0 20px rgba(239, 68, 68, 0.3); }
        .risk-mid { color: #fbbf24; }
        .risk-low { color: #94a3b8; }

        /* Action Footer */
        .actions-panel {
            margin-top: 3rem;
            padding: 3rem;
            display: grid;
            grid-template-columns: 1fr 1fr 1fr;
            gap: 2rem;
            background: linear-gradient(180deg, var(--bg-glass) 0%, rgba(17, 24, 39, 0.95) 100%);
        }

        .action-column h3 {
            font-size: 1rem;
            text-transform: uppercase;
            letter-spacing: 0.1em;
            margin-top: 0;
            margin-bottom: 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .action-column h3::before {
            content: '';
            display: block;
            width: 8px;
            height: 8px;
            border-radius: 50%;
        }

        .action-remediate h3::before { background: var(--danger-glow); box-shadow: 0 0 10px var(--danger-glow); }
        .action-suppress h3::before { background: var(--accent-glow); box-shadow: 0 0 10px var(--accent-glow); }
        .action-terraform h3::before { background: var(--success-glow); box-shadow: 0 0 10px var(--success-glow); }

        code {
            display: block;
            background: #000;
            padding: 1rem;
            border-radius: 0.5rem;
            font-family: var(--font-mono);
            font-size: 0.8rem;
            color: #d1d5db;
            border: 1px solid rgba(255, 255, 255, 0.1);
            margin-top: 0.5rem;
        }

    </style>
</head>
<body>
    <div class="container">
        <header>
            <div>
                <h1>CloudSlash Protocol</h1>
                <div style="margin-top: 0.5rem; color: var(--text-secondary); display: flex; align-items: center; gap: 1rem;">
                    <span>Target: AWS Infrastructure</span>
                    <span>//</span>
                    <span>Scan ID: {{.GeneratedAt}}</span>
                </div>
            </div>
            <div>
                <span class="meta-tag">v1.3.3 [AGPLv3]</span>
            </div>
        </header>

        <div class="kpi-grid">
            <div class="glass-panel kpi-card">
                <span class="kpi-label">Monthly Waste</span>
                <div class="kpi-value text-gradient-danger">${{printf "%.2f" .TotalWasteCost}}</div>
            </div>
            <div class="glass-panel kpi-card">
                <span class="kpi-label">Annual Projection</span>
                <div class="kpi-value text-gradient-success">${{printf "%.2f" .ProjectedSavings}}</div>
            </div>
            <div class="glass-panel kpi-card">
                <span class="kpi-label">Efficiency Score</span>
                <!-- Inverse of waste ratio roughly -->
                <div class="kpi-value" style="color: #fff;">
                    {{if eq .TotalResources 0}}100%{{else}}
                    {{printf "%.0f" (sub 100 (mul (div .TotalWaste .TotalResources) 100))}}%
                    {{end}}
                </div>
                <small style="color: var(--text-secondary); font-size: 0.75rem; margin-top: auto;">Based on {{.TotalResources}} Resources</small>
            </div>
             <div class="glass-panel kpi-card">
                <span class="kpi-label">Action Items</span>
                <div class="kpi-value" style="color: #fff;">{{.TotalWaste}}</div>
                <small style="color: var(--text-secondary); font-size: 0.75rem; margin-top: auto;">Immediate Attention</small>
            </div>
        </div>

        <div class="analytics-grid">
            <div class="glass-panel chart-card">
                <div class="card-header">
                    <h3 class="card-title">Cost Velocity by Service</h3>
                </div>
                <div style="height: 300px; width: 100%;">
                    <canvas id="costChart"></canvas>
                </div>
            </div>
            <div class="glass-panel chart-card">
                 <div class="card-header">
                    <h3 class="card-title">Waste Distribution</h3>
                </div>
                <div style="height: 300px; display: flex; justify-content: center; align-items: center;">
                    <canvas id="utilChart"></canvas>
                </div>
            </div>
        </div>

        <div class="glass-panel table-container">
            <div style="padding: 2rem; border-bottom: 1px solid var(--border-glass);">
                <h2 style="margin: 0; font-size: 1.5rem;">Forensic Evidence Log</h2>
            </div>
            <table>
                <thead>
                    <tr>
                        <th width="30%">Resource Identifier</th>
                        <th width="15%">Service Type</th>
                        <th width="10%">Risk</th>
                        <th width="15%">Cost / Mo</th>
                        <th width="30%">Diagnostic Root Cause</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .WasteItems}}
                    <tr>
                        <td>
                            <div class="resource-id">{{.ID}}</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary); margin-top: 4px;">{{.SrcLoc}}</div>
                        </td>
                        <td><span class="type-pill">{{.Type}}</span></td>
                        <td>
                             {{if ge .RiskScore 80}}
                                <span class="risk-score risk-high">CRITICAL</span>
                            {{else}}
                                <span class="risk-score risk-mid">Medium</span>
                            {{end}}
                        </td>
                        <td style="font-family: var(--font-mono); color: var(--text-primary);">$ {{printf "%.2f" .Cost}}</td>
                        <td style="color: var(--text-secondary);">{{.Reason}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="5" style="text-align: center; padding: 4rem; color: var(--text-secondary);">
                            SYSTEM CLEAN. NO ANOMALIES DETECTED.
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <div class="glass-panel actions-panel">
            <div class="action-column action-remediate">
                <h3>Level 1: Cleanup</h3>
                <code>bash cloudslash-out/safe_cleanup.sh</code>
            </div>
             <div class="action-column action-suppress">
                <h3>Level 2: Suppression</h3>
                <code>bash cloudslash-out/ignore_resources.sh</code>
            </div>
             <div class="action-column action-terraform">
                <h3>Level 3: Reconcile</h3>
                <code>bash cloudslash-out/fix_terraform.sh</code>
            </div>
        </div>

        <footer style="margin-top: 4rem; text-align: center; opacity: 0.5; padding-bottom: 2rem;">
            <p style="font-size: 0.8rem; letter-spacing: 0.2em; text-transform: uppercase;">CloudSlash Forensic Engine // v1.3.3 [AGPLv3]</p>
        </footer>
    </div>

    <script>
        // Data Injection
        const labels = {{.ChartLabelsJSON}};
        const values = {{.ChartValuesJSON}};

        // Modern Chart Config
        Chart.defaults.font.family = "'JetBrains Mono', monospace";
        Chart.defaults.color = 'rgba(148, 163, 184, 0.8)';
        
        // Bar Chart (Cost)
        const ctxCost = document.getElementById('costChart').getContext('2d');
        const gradientCost = ctxCost.createLinearGradient(0, 0, 0, 400);
        gradientCost.addColorStop(0, 'rgba(59, 130, 246, 0.5)');
        gradientCost.addColorStop(1, 'rgba(59, 130, 246, 0.0)');

        new Chart(ctxCost, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Cost Impact',
                    data: values,
                    backgroundColor: gradientCost,
                    borderColor: '#3b82f6',
                    borderWidth: 2,
                    borderRadius: 4,
                    hoverBackgroundColor: '#60a5fa'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(17, 24, 39, 0.9)',
                        titleColor: '#fff',
                        bodyColor: '#94a3b8',
                        borderColor: 'rgba(255,255,255,0.1)',
                        borderWidth: 1,
                        padding: 12,
                        displayColors: false,
                        callbacks: {
                            label: function(context) {
                                return '$ ' + context.raw.toFixed(2);
                            }
                        }
                    }
                },
                scales: {
                    y: {
                        grid: { color: 'rgba(255, 255, 255, 0.05)' },
                        border: { display: false }
                    },
                    x: {
                        grid: { display: false },
                        border: { display: false }
                    }
                }
            }
        });

        // Doughnut Chart (Distribution)
        const ctxUtil = document.getElementById('utilChart').getContext('2d');
        new Chart(ctxUtil, {
            type: 'doughnut',
            data: {
                labels: labels,
                datasets: [{
                    data: values,
                    backgroundColor: [
                        '#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899'
                    ],
                    borderWidth: 0,
                    hoverOffset: 10
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                cutout: '75%',
                plugins: {
                    legend: {
                        position: 'right',
                        labels: {
                            usePointStyle: true,
                            padding: 20
                        }
                    }
                }
            }
        });
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

	// Helper for numeric conversion
	toFloat := func(v interface{}) float64 {
		switch i := v.(type) {
		case int:
			return float64(i)
		case int64:
			return float64(i)
		case float64:
			return i
		default:
			return 0
		}
	}

	// Register Math Functions
	funcMap := template.FuncMap{
		"sub": func(a, b interface{}) float64 { return toFloat(a) - toFloat(b) },
		"mul": func(a, b interface{}) float64 { return toFloat(a) * toFloat(b) },
		"div": func(a, b interface{}) float64 {
			num := toFloat(a)
			den := toFloat(b)
			if den == 0 {
				return 0
			}
			return num / den
		},
	}

	t, err := template.New("report").Funcs(funcMap).Parse(htmlTemplate)
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
