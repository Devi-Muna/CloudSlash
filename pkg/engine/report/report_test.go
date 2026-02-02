package report

import (
	"os"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

func TestGenerateHTML_XSS(t *testing.T) {
	// Setup Graph with malicious node
	g := graph.NewGraph()
	
	// Malicious inputs
	maliciousID := `i-bad"; alert('XSS'); "`
	maliciousType := `AWS::EC2::Instance<script>alert(1)</script>`
	
	props := map[string]interface{}{
		"Cost": 120.50,
	}
	g.AddNode(maliciousID, maliciousType, props)
	
	// Mark as waste
	g.MarkWaste(maliciousID, 100)
	
	// Generate Report
	tmpFile, err := os.CreateTemp("", "report_test_*.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	err = GenerateHTML(g, tmpFile.Name())
	if err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}
	
	// Verify Content
	contentBytes, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	content := string(contentBytes)
	
	// Check 1: Malicious ID should be inside the table, likely HTML escaped by template, 
	// OR inside the JS block securely.
	
	// Vital Check: The detailed JSON arrays at the bottom MUST NOT contain the raw injected quote.
	// We expect the JSON marshaler to escape the quote as \".
	// The bad string was: i-bad"; alert('XSS'); "
	// It should appear as: "i-bad\"; alert('XSS'); \"" inside the JSON array.
	
	// However, note that we don't put IDs in the chart labels currently, only Types.
	// The maliciousType was used in the chart labels loop.
	// Malicious Type: AWS::EC2::Instance<script>alert(1)</script>
	
	// Check for raw injection in the JS block
	if strings.Contains(content, `["AWS::EC2::Instance<script>alert(1)</script>"]`) {
		// If it looks exactly like this, it MIGHT be okay if it's inside a string,
		// but if the quote was unescaped, it would be a problem. 
		// Actually json.Marshal should turn < and > into \u003c \u003e etc to be HTML safe by default?
		// Go's json.Marshal escapes <, >, and & for safety in HTML contexts by default!
		t.Errorf("Found unescaped HTML characters in JSON block, expected safe encoding")
	}
	
	// We expect escaped unicode or at least proper quoting.
	// Let's verify that we CANNOT find the literal script tag in the JS block context.
	// The template variable is {{.ChartLabelsJSON}}.
	
	// Search for the specific XSS payload break-out attempt.
	// If the output contains `alert('XSS')` as executable code (not inside quotes), that's a fail.
	// Since we can't easily parse JS here, we rely on the absence of the raw breakout sequence.
	
	// The generated line looks like: const labels = [...];
	// If vulnerable, it would look like: const labels = ["...", "malicious"]; alert('XSS'); "];
	
	// We verify that the string "alert('XSS')" ONLY appears if it is properly escaped/safe.
	// But `maliciousID` is not in the chart. `maliciousType` is.
	
	// Malicious Type payload: <script>alert(1)</script>
	// Go json.Marshal encodes this as: "AWS::EC2::Instance\u003cscript\u003ealert(1)\u003c/script\u003e"
	expectedSafe := `\u003cscript\u003ealert(1)\u003c/script\u003e`
	if !strings.Contains(content, expectedSafe) {
		t.Logf("Did not find Go-style HTML escaping. Checking for standard JSON escaping...")
		// If standard JSON (e.g. not HTMLEscape), it would be "<script>..."
		// But inside specific context it must certainly be valid string.
	}
	
	// FAIL condition: The string <script> appears literally without escaping.
	if strings.Contains(content, "<script>alert(1)</script>") {
		// This might appear in the HTML BODY (table), which is fine if text content, 
		// but definitely NOT fine if it was raw. The template engine `{{.Type}}` auto-escapes HTML body.
		// So checking strictly for the JS block.
		
		// Let's look for the JS assignment line.
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, "const labels =") {
				if strings.Contains(line, "<script>") {
					t.Fatalf("XSS VULNERABILITY DETECTED in Chart Labels: %s", line)
				}
			}
		}
	}
}
