package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/fayrus/syncret/internal/config"
	"github.com/fayrus/syncret/internal/event"
	"github.com/fayrus/syncret/internal/logctx"
	"github.com/fayrus/syncret/internal/notify"
)

type SecretsProvider interface {
	GetSecretString(ctx context.Context, arn string) (string, error)
	MergeAndPutSecret(ctx context.Context, targetARN string, sourceData map[string]any, keys []string) (bool, error)
}

type ComputeProvider interface {
	ForceNewDeployment(ctx context.Context, cluster string, services []string) error
}

type Notifier interface {
	Send(ctx context.Context, p notify.Payload) error
}

type Dependencies struct {
	Secrets SecretsProvider
	ECS     ComputeProvider
}

type stageStatus string

const (
	stageNotAttempted stageStatus = "not_attempted"
	stageSucceeded    stageStatus = "succeeded"
	stageUnchanged    stageStatus = "unchanged"
	stageFailed       stageStatus = "failed"
	stageSkipped      stageStatus = "skipped"
)

type failureKind string

const (
	failureEventParse     failureKind = "event_parse"
	failureARNGuard       failureKind = "arn_guard"
	failureSourceRead     failureKind = "source_read"
	failureTargetUpdate   failureKind = "target_update"
	failureECSForceDeploy failureKind = "ecs_force_deploy"
)

type executionResult struct {
	EventName      event.EventName
	EventSecretARN string
	TargetUpdate   stageStatus
	ECSForceDeploy stageStatus
	Failure        failureKind
}

func handle(ctx context.Context, cfg *config.Config, payload []byte, deps Dependencies) error {
	_, err := execute(ctx, cfg, payload, deps)
	return err
}

func execute(ctx context.Context, cfg *config.Config, payload []byte, deps Dependencies) (executionResult, error) {
	log := logctx.From(ctx)
	result := executionResult{}
	if cfg.TargetSecretARN != "" {
		result.TargetUpdate = stageNotAttempted
	}
	if cfg.ECSForceDeploy {
		result.ECSForceDeploy = stageNotAttempted
	}

	evt, err := event.Parse(payload)
	if err != nil {
		result.Failure = failureEventParse
		return result, fmt.Errorf("handler: parse event: %w", err)
	}
	result.EventName = evt.Name
	result.EventSecretARN = evt.SecretARN

	if evt.SecretARN != cfg.SecretARN {
		result.Failure = failureARNGuard
		return result, fmt.Errorf("handler: event secret ARN %q does not match configured %q", evt.SecretARN, cfg.SecretARN)
	}

	log.Info("event received",
		"event", string(evt.Name),
		"secret_arn", cfg.SecretARN,
	)

	if !evt.ShouldRedeploy() {
		log.Warn("rotation failed — skipping target update", "secret_arn", cfg.SecretARN)
		markSkipped(&result, cfg)
		return result, nil
	}

	if cfg.TargetSecretARN != "" {
		secretValue, err := deps.Secrets.GetSecretString(ctx, cfg.SecretARN)
		if err != nil {
			result.Failure = failureSourceRead
			return result, fmt.Errorf("handler: get secret: %w", err)
		}
		changed, err := updateTargetSecret(ctx, cfg, deps.Secrets, secretValue)
		if err != nil {
			result.TargetUpdate = stageFailed
			result.Failure = failureTargetUpdate
			return result, err
		}
		result.TargetUpdate = stageSucceeded
		if !changed {
			result.TargetUpdate = stageUnchanged
		}
	}

	if cfg.ECSForceDeploy {
		if err := deps.ECS.ForceNewDeployment(ctx, cfg.ECSCluster, cfg.ECSServices); err != nil {
			result.ECSForceDeploy = stageFailed
			result.Failure = failureECSForceDeploy
			return result, fmt.Errorf("handler: ecs redeploy: %w", err)
		}
		result.ECSForceDeploy = stageSucceeded
	}

	return result, nil
}

func markSkipped(result *executionResult, cfg *config.Config) {
	if cfg.TargetSecretARN != "" {
		result.TargetUpdate = stageSkipped
	}
	if cfg.ECSForceDeploy {
		result.ECSForceDeploy = stageSkipped
	}
}

func updateTargetSecret(ctx context.Context, cfg *config.Config, sm SecretsProvider, secretValue string) (bool, error) {
	log := logctx.From(ctx)

	// UseNumber keeps numbers as their original text: float64 cannot represent integers above 2^53.
	dec := json.NewDecoder(strings.NewReader(secretValue))
	dec.UseNumber()
	var sourceData map[string]any
	if err := dec.Decode(&sourceData); err != nil {
		return false, fmt.Errorf("handler: parse source secret: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return false, errors.New("handler: parse source secret: unexpected data after JSON object")
	}
	// JSON null decodes without error into a nil map.
	if sourceData == nil {
		return false, errors.New("handler: parse source secret: not a JSON object")
	}

	log.Info("updating target secret",
		"target_arn", cfg.TargetSecretARN,
		"keys", cfg.TargetSecretKeys,
	)

	changed, err := sm.MergeAndPutSecret(ctx, cfg.TargetSecretARN, sourceData, cfg.TargetSecretKeys)
	if err != nil {
		return false, fmt.Errorf("handler: update target secret: %w", err)
	}

	if !changed {
		log.Info("target secret already up to date", "target_arn", cfg.TargetSecretARN)
		return false, nil
	}
	log.Info("target secret updated", "target_arn", cfg.TargetSecretARN)
	return true, nil
}
