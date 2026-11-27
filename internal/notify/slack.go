package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// slackEscaper escapes the characters Slack treats as control sequences in message text.
var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

type Slack struct {
	webhookURL string
	client     *http.Client
}

func NewSlack(webhookURL string) *Slack {
	return &Slack{
		webhookURL: webhookURL,
		client:     newWebhookClient(),
	}
}

func (s *Slack) Send(ctx context.Context, p Payload) error {
	if err := postText(ctx, s.client, s.webhookURL, slackEscaper.Replace(format(p))); err != nil {
		return fmt.Errorf("notify: slack: %w", err)
	}
	return nil
}
