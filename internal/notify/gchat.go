package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Payload struct {
	InstanceName string
	EventName    string
	AccountID    string
	Region       string
	SourceSecret string
	EventSecret  string
	TargetSecret string
	ECSCluster   string
	ECSServices  []string
	RequestID    string
	Actions      []string
	Status       Status
	ErrorMessage string
	OccurredAt   time.Time
	TimezoneName string
}

type Status string

const (
	StatusSuccess Status = "✅ Success"
	StatusFailed  Status = "❌ Failed"
	StatusSkipped Status = "⚠️ Skipped"
)

type GoogleChat struct {
	webhookURL string
	client     *http.Client
}

func NewGoogleChat(webhookURL string) *GoogleChat {
	return &GoogleChat{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (g *GoogleChat) Send(ctx context.Context, p Payload) error {
	body, err := json.Marshal(map[string]string{"text": format(p)})
	if err != nil {
		return fmt.Errorf("notify: gchat: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: gchat: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("notify: gchat: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify: gchat: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func format(p Payload) string {
	status := p.Status
	if status == "" {
		status = StatusSuccess
	}
	timezoneName := p.TimezoneName
	if timezoneName == "" {
		timezoneName = "UTC"
	}
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		timezoneName = "UTC"
		location = time.UTC
	}

	var sb strings.Builder
	if p.InstanceName == "" {
		sb.WriteString("*Syncret*\n\n")
	} else {
		fmt.Fprintf(&sb, "*Syncret — %s*\n\n", p.InstanceName)
	}
	fmt.Fprintf(&sb, "*Event:* %s\n", p.EventName)
	if p.AccountID != "" {
		fmt.Fprintf(&sb, "*Account:* %s\n", p.AccountID)
	}
	if p.Region != "" {
		fmt.Fprintf(&sb, "*Region:* %s\n", p.Region)
	}
	if p.SourceSecret != "" {
		fmt.Fprintf(&sb, "*Source secret:* %s\n", p.SourceSecret)
	}
	if p.EventSecret != "" {
		fmt.Fprintf(&sb, "*Event secret:* %s\n", p.EventSecret)
	}
	if p.TargetSecret != "" {
		fmt.Fprintf(&sb, "*Target secret:* %s\n", p.TargetSecret)
	}
	if p.ECSCluster != "" {
		fmt.Fprintf(&sb, "*ECS cluster:* %s\n", p.ECSCluster)
	}
	if len(p.ECSServices) > 0 {
		fmt.Fprintf(&sb, "*ECS services:* %s\n", strings.Join(p.ECSServices, ", "))
	}
	fmt.Fprintf(&sb, "*Date:* %s (%s)\n",
		p.OccurredAt.In(location).Format("2006-01-02 15:04:05 -07:00"),
		timezoneName,
	)
	if p.RequestID != "" {
		fmt.Fprintf(&sb, "*Request ID:* %s\n", p.RequestID)
	}
	if len(p.Actions) > 0 {
		sb.WriteString("*Actions:*\n")
		for _, a := range p.Actions {
			fmt.Fprintf(&sb, "  • %s\n", a)
		}
	}
	fmt.Fprintf(&sb, "*Status:*  %s", status)
	if p.ErrorMessage != "" {
		fmt.Fprintf(&sb, "\n*Error:*   %s", p.ErrorMessage)
	}
	return sb.String()
}
