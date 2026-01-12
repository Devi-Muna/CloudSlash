package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/DrSkyle/cloudslash/internal/report"
)

// SlackClient handles the transmission of reports to a configured Slack Webhook.
type SlackClient struct {
	WebhookURL string
	Channel    string // Optional: Override default channel
}

// NewSlackClient initializes a new client validation.
func NewSlackClient(webhookURL string, channel string) *SlackClient {
	return &SlackClient{
		WebhookURL: webhookURL,
		Channel:    channel,
	}
}

// SendAnalysisReport dispatches a structured Block Kit message summarizing the scan results.
func (s *SlackClient) SendAnalysisReport(summary report.Summary) error {
	if s.WebhookURL == "" {
		return nil
	}

	payload := s.constructPayload(summary)
	
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequest("POST", s.WebhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("received non-200 status from slack: %d", resp.StatusCode)
	}

	return nil
}

// constructPayload builds the Slack Block Kit structure.
// Ref: https://api.slack.com/block-kit/building
func (s *SlackClient) constructPayload(summary report.Summary) map[string]interface{} {
	// Status Indicator
	statusIcon := "🟢" // Healthy
	if summary.TotalSavings > 1000 {
		statusIcon = "🔴" // Critical Savings Found
	} else if summary.TotalSavings > 0 {
		statusIcon = "🟡" // Potential Optimization
	}

	blocks := []map[string]interface{}{
		// Header
		{
			"type": "header",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": fmt.Sprintf("%s Infrastructure Optimization Report", statusIcon),
			},
		},
		// Context: Date & Region
		{
			"type": "context",
			"elements": []map[string]interface{}{
				{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Scan Date:* %s | *Region:* %s", time.Now().Format("2006-01-02"), summary.Region),
				},
			},
		},
		// Divider
		{
			"type": "divider",
		},
		// Section: Quick Stats
		{
			"type": "section",
			"fields": []map[string]interface{}{
				{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Total Potential Savings:*\n$%.2f/mo", summary.TotalSavings),
				},
				{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Resources Analyzed:*\n%d", summary.TotalScanned),
				},
				{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Inefficiencies Identified:*\n%d", summary.TotalWaste),
				},
			},
		},
	}

	// Add High Impact Alert if waste is significant
	if summary.TotalSavings > 500 {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "⚠️ *High Financial Impact Detected*\nSignificant unused infrastructure has been identified. Immediate review is recommended.",
			},
		})
	}

	payload := map[string]interface{}{
		"blocks": blocks,
	}

	if s.Channel != "" {
		payload["channel"] = s.Channel
	}

	return payload
}


// SendBudgetAlert dispatches a high-priority alert for anomaly detection.
func (s *SlackClient) SendBudgetAlert(velocity float64, acceleration float64) error {
	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "🔥 Cost Velocity Alert",
				},
			},
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("Spend is accelerating dangerously.\n*Velocity:* +$%.2f/mo per hour\n*Acceleration:* +%.2f%%", velocity, acceleration),
				},
			},
		},
	}

	if s.Channel != "" {
		payload["channel"] = s.Channel
	}

	return s.send(payload)
}

func (s *SlackClient) send(payload map[string]interface{}) error {
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", s.WebhookURL, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
