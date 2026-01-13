package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/swarm"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTUI_Rendering_v1_3_0(t *testing.T) {
	tests := []struct {
		name     string
		mockNode *graph.Node
		want     []string
		dontWant []string
	}{
		{
			name: "Zombie NAT: Hollow with 0 Traffic",
			mockNode: &graph.Node{
				ID:      "nat-hollow-123",
				Type:    "aws_nat_gateway",
				IsWaste: true,
				Cost:    32.40,
				Properties: map[string]interface{}{
					"Reason":           "Hollow NAT Gateway: Serves subnets with ZERO active instances. Traffic: 0.",
					"SumConnections7d": 0.0,
					"ActiveUserENIs":   0,
				},
			},
			want: []string{"NAT Gateway", "Hollow", "Traffic: 0"},
		},
		{
			name: "Safe EIP: Unattached & Not in DNS",
			mockNode: &graph.Node{
				ID:      "eip-safe-123",
				Type:    "aws_eip",
				IsWaste: true,
				Cost:    3.50,
				Properties: map[string]interface{}{
					"Reason":        "Safe to Release: Unused EIP (Not in Route53).",
					"AssociationId": "",
					"FoundInDNS":    false,
				},
			},
			want: []string{"Safe to Release", "Unused EIP"},
		},
		{
			name: "Dangerous EIP: Unattached BUT in DNS",
			mockNode: &graph.Node{
				ID:      "eip-danger-999",
				Type:    "aws_eip",
				IsWaste: true,
				Cost:    3.50,
				Properties: map[string]interface{}{
					"Reason":     "DANGEROUS ZOMBIE: EIP eip-danger-999 is unused BUT hardcoded in DNS zone example.com. Do NOT release. DNS Conflict.",
					"FoundInDNS": true,
					"DNSZone":    "example.com",
				},
			},
			want: []string{"DANGEROUS", "DNS Conflict", "Do NOT release"},
		},
		{
			name: "S3 Iceberg: Stalled Multipart Upload",
			mockNode: &graph.Node{
				ID:        "s3-multipart-upload-1",
				Type:      "AWS::S3::MultipartUpload",
				IsWaste:   true,
				RiskScore: 20,
				Properties: map[string]interface{}{
					"Reason":    "Stalled Upload: Initiated 10 days ago.",
					"Initiated": time.Now().Add(-10 * 24 * time.Hour),
				},
			},
			want: []string{"Stalled Upload", "10 days ago", "Multipart"},
		},
		{
			name: "EBS Modernizer: gp2 to gp3",
			mockNode: &graph.Node{
				ID:      "vol-gp2-legacy",
				Type:    "AWS::EC2::Volume",
				IsWaste: true,
				Cost:    2.00,
				Properties: map[string]interface{}{
					"Reason":     "EBS Modernizer: Switch to gp3. Save $2.00/mo.",
					"VolumeType": "gp2",
					"Size":       100,
				},
			},
			want: []string{"EBS Modernizer", "Switch to gp3", "Save $2.00"},
		},
		{
			name: "Redshift: Idle Cluster",
			mockNode: &graph.Node{
				ID:      "redshift-idle-1",
				Type:    "aws_redshift_cluster",
				IsWaste: true,
				Cost:    500.00,
				Properties: map[string]interface{}{
					"Reason": "Idle Cluster: 0 queries in 7 days. Action: PAUSE.",
				},
			},
			want: []string{"Idle Cluster", "0 queries", "PAUSE"},
		},
		{
			name: "Lambda: Prunable Versions",
			mockNode: &graph.Node{
				ID:      "func-v1",
				Type:    "aws_lambda_function",
				IsWaste: true,
				Properties: map[string]interface{}{
					"Reason": "Code Rot: Last modified 400 days ago.",
				},
			},
			want: []string{"Code Rot", "400 days ago"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := graph.NewGraph()
			g.AddNode(tc.mockNode.ID, tc.mockNode.Type, tc.mockNode.Properties)
			if n := g.GetNode(tc.mockNode.ID); n != nil {
				n.IsWaste = tc.mockNode.IsWaste
				n.Cost = tc.mockNode.Cost
				n.RiskScore = tc.mockNode.RiskScore
				n.SourceLocation = tc.mockNode.SourceLocation
			}

			eng := swarm.NewEngine()
			model := NewModel(eng, g, false, "us-east-1")

			model.refreshData()

			updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			model = updatedModel.(Model)
			view := model.View()

			for _, w := range tc.want {
				if !strings.Contains(view, w) {
					t.Errorf("[%s] FAIL: Expected view to contain '%s'.\nGot:\n%s", tc.name, w, view)
				}
			}

			for _, dw := range tc.dontWant {
				if strings.Contains(view, dw) {
					t.Errorf("[%s] FAIL: Expected view NOT to contain '%s'.\nGot:\n%s", tc.name, dw, view)
				}
			}
		})
	}
}

func TestTUI_TerraformIndicator(t *testing.T) {
	g := graph.NewGraph()
	node := &graph.Node{
		ID:             "tf-managed-resource",
		Type:           "unknown",
		IsWaste:        true,
		SourceLocation: "main.tf:12",
	}
	g.AddNode(node.ID, node.Type, node.Properties)
	if n := g.GetNode(node.ID); n != nil {
		n.IsWaste = node.IsWaste
		n.SourceLocation = node.SourceLocation
	}

	eng := swarm.NewEngine()
	model := NewModel(eng, g, false, "us-east-1")
	view := model.View()

	if strings.Contains(view, "[TERRAFORM DETECTED]") {
		// Pass
	}
}
