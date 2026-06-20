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
	EventName  string
	SecretARN  string
	Actions    []string
	Err        error
	OccurredAt time.Time
}

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
	status := "✅ Success"
	if p.Err != nil {
		status = "❌ Failed"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "*Syncret* — %s\n\n", p.EventName)
	fmt.Fprintf(&sb, "*Date:*    %s\n", p.OccurredAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&sb, "*Secret:*  %s\n", p.SecretARN)
	if len(p.Actions) > 0 {
		sb.WriteString("*Actions:*\n")
		for _, a := range p.Actions {
			fmt.Fprintf(&sb, "  • %s\n", a)
		}
	}
	fmt.Fprintf(&sb, "*Status:*  %s", status)
	if p.Err != nil {
		fmt.Fprintf(&sb, "\n*Error:*   %s", p.Err.Error())
	}
	return sb.String()
}
