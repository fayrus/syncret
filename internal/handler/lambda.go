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

		result, err := execute(ctx, cfg, []byte(payload), deps)
		if err != nil {
			logctx.From(ctx).Error("handler execution failed", "error", err)
		}

		if gchat != nil {
			p := buildNotifyPayload(cfg, result, start, requestID, err)
			if notifyErr := gchat.Send(ctx, p); notifyErr != nil {
				logctx.From(ctx).Warn("gchat notification failed", "error", notifyErr)
			}
		}

		return err
	})
}

func buildNotifyPayload(cfg *config.Config, result executionResult, occurredAt time.Time, requestID string, err error) notify.Payload {
	accountID, region, sourceSecret := secretMetadata(cfg.SecretARN)
	if region == "" {
		region = cfg.AWSRegion
	}
	_, _, targetSecret := secretMetadata(cfg.TargetSecretARN)

	eventName := string(result.EventName)
	if eventName == "" {
		eventName = "unknown"
	}

	p := notify.Payload{
		InstanceName: cfg.InstanceName,
		EventName:    eventName,
		AccountID:    accountID,
		Region:       region,
		SourceSecret: sourceSecret,
		TargetSecret: targetSecret,
		RequestID:    requestID,
		Actions:      notificationActions(result),
		Status:       notify.StatusSuccess,
		OccurredAt:   occurredAt,
		TimezoneName: cfg.Timezone,
	}
	if cfg.ECSForceDeploy {
		p.ECSCluster = cfg.ECSCluster
		p.ECSServices = append([]string(nil), cfg.ECSServices...)
	}
	if result.EventSecretARN != "" && result.EventSecretARN != cfg.SecretARN {
		_, _, p.EventSecret = secretMetadata(result.EventSecretARN)
	}
	if err != nil {
		p.Status = notify.StatusFailed
		p.ErrorMessage = notificationError(result.Failure)
	} else if result.EventName == event.RotationFailed {
		p.Status = notify.StatusSkipped
	}

	return p
}

func secretMetadata(value string) (accountID, region, name string) {
	parts := strings.SplitN(value, ":", 6)
	if len(parts) != 6 {
		return "", "", value
	}
	resource := parts[5]
	resource = strings.TrimPrefix(resource, "secret:")
	resource = strings.TrimPrefix(resource, "secret/")
	return parts[4], parts[3], resource
}

func notificationActions(result executionResult) []string {
	var actions []string
	switch result.Failure {
	case failureEventParse:
		actions = append(actions, "❌ Event parsing failed")
	case failureARNGuard:
		actions = append(actions, "❌ Event rejected by source secret guard")
	case failureSourceRead:
		actions = append(actions, "❌ Source secret read failed")
	}

	switch result.TargetUpdate {
	case stageSucceeded:
		actions = append(actions, "✅ Target secret updated")
	case stageFailed:
		actions = append(actions, "❌ Target secret update failed")
	case stageSkipped:
		actions = append(actions, "⏭ Target secret update skipped")
	case stageNotAttempted:
		actions = append(actions, "⏭ Target secret update not attempted")
	}

	switch result.ECSForceDeploy {
	case stageSucceeded:
		actions = append(actions, "✅ ECS force-deployment request accepted")
	case stageFailed:
		actions = append(actions, "❌ ECS force-deployment request failed")
	case stageSkipped:
		actions = append(actions, "⏭ ECS force-deployment skipped")
	case stageNotAttempted:
		actions = append(actions, "⏭ ECS force-deployment not attempted")
	}
	return actions
}

func notificationError(failure failureKind) string {
	const logHint = "; see Lambda logs using the request ID"
	switch failure {
	case failureEventParse:
		return "Event payload could not be parsed" + logHint
	case failureARNGuard:
		return "Event secret does not match the configured source secret" + logHint
	case failureSourceRead:
		return "Unable to read source secret" + logHint
	case failureTargetUpdate:
		return "Unable to update target secret" + logHint
	case failureECSForceDeploy:
		return "One or more ECS force-deployment requests failed" + logHint
	default:
		return "Execution failed" + logHint
	}
}
