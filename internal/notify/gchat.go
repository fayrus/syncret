package notify

import (
	"context"
	"fmt"
	"net/http"
)

type GoogleChat struct {
	webhookURL string
	client     *http.Client
}

func NewGoogleChat(webhookURL string) *GoogleChat {
	return &GoogleChat{
		webhookURL: webhookURL,
		client:     newWebhookClient(),
	}
}

func (g *GoogleChat) Send(ctx context.Context, p Payload) error {
	if err := postText(ctx, g.client, g.webhookURL, format(p)); err != nil {
		return fmt.Errorf("notify: gchat: %w", err)
	}
	return nil
}
