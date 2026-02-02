package forensics

import (
	"context"
	"testing"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

func TestIdentifyOwner_Tags(t *testing.T) {
	detective := NewDetective(nil) // No CloudTrail client needed for tag tests
	ctx := context.Background()

	tests := []struct {
		name     string
		props    map[string]interface{}
		expected string
	}{
		{
			name:     "Direct Owner Tag",
			props:    map[string]interface{}{"Owner": "jane.doe@example.com"},
			expected: "Tag:jane.doe@example.com",
		},
		{
			name:     "Lowercase owner tag",
			props:    map[string]interface{}{"owner": "john.doe@example.com"},
			expected: "Tag:john.doe@example.com",
		},
		{
			name: "Nested Tags Map",
			props: map[string]interface{}{
				"Tags": map[string]string{
					"CreatedBy": "ci-bot",
				},
			},
			expected: "Tag:ci-bot",
		},
		{
			name:     "No Tags",
			props:    map[string]interface{}{},
			expected: "UNCLAIMED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the Graph API to construct the node correctly
			g := graph.NewGraph()
			g.AddNode("test-node", "AWS::EC2::Instance", tt.props)
			g.CloseAndWait()
			
			// Retrieve the node to get the internally consistent ID
			node := g.GetNode("test-node")
			if node == nil {
				t.Fatalf("Failed to retrieve node")
			}

			got := detective.IdentifyOwner(ctx, node)
			if got != tt.expected {
				t.Errorf("IdentifyOwner() = %v, want %v", got, tt.expected)
			}
		})
	}
}
