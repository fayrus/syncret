package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/fayrus/syncret/internal/config"
	"github.com/fayrus/syncret/internal/event"
	"github.com/fayrus/syncret/internal/logctx"
	"github.com/fayrus/syncret/internal/notify"
)

func StartLambda(cfg *config.Config, deps Dependencies) {
	var gchat *notify.GoogleChat
	if cfg.GChatWebhook != "" {
		gchat = notify.NewGoogleChat(cfg.GChatWebhook)
	}

	lambda.Start(func(ctx context.Context, payload json.RawMessage) error {
		start := time.Now().UTC()

		requestID := "unknown"
		if lc, ok := lambdacontext.FromContext(ctx); ok {
			requestID = lc.AwsRequestID
		}
		ctx = logctx.WithLogger(ctx, slog.Default().With("request_id", requestID))

		err := handle(ctx, cfg, []byte(payload), deps)

		if gchat != nil {
			p := buildNotifyPayload(cfg, []byte(payload), start, err)
			if notifyErr := gchat.Send(ctx, p); notifyErr != nil {
				logctx.From(ctx).Warn("gchat notification failed", "error", notifyErr)
			}
		}

		return err
	})
}

func buildNotifyPayload(cfg *config.Config, raw []byte, occurredAt time.Time, err error) notify.Payload {
	p := notify.Payload{
		EventName:  "unknown",
		SecretARN:  cfg.SecretARN,
		Err:        err,
		OccurredAt: occurredAt,
	}

	evt, parseErr := event.Parse(raw)
	if parseErr != nil {
		return p
	}

	p.EventName = string(evt.Name)
	p.SecretARN = evt.SecretARN

	if evt.ShouldRedeploy() {
		if cfg.TargetSecretARN != "" {
			p.Actions = append(p.Actions, "Target secret updated")
		}
		if cfg.ECSForceDeploy {
			p.Actions = append(p.Actions, "ECS redeployment: "+strings.Join(cfg.ECSServices, ", "))
		}
	}

	return p
}
