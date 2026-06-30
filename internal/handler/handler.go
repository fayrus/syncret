package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fayrus/syncret/internal/config"
	"github.com/fayrus/syncret/internal/event"
	"github.com/fayrus/syncret/internal/logctx"
)

type SecretsProvider interface {
	GetSecretString(ctx context.Context, arn string) (string, error)
	MergeAndPutSecret(ctx context.Context, targetARN string, sourceData map[string]any, keys []string) error
}

type ComputeProvider interface {
	ForceNewDeployment(ctx context.Context, cluster string, services []string) error
}

type Dependencies struct {
	Secrets SecretsProvider
	ECS     ComputeProvider
}

type stageStatus string

const (
	stageNotAttempted stageStatus = "not_attempted"
	stageSucceeded    stageStatus = "succeeded"
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
		if err := updateTargetSecret(ctx, cfg, deps.Secrets, secretValue); err != nil {
			result.TargetUpdate = stageFailed
			result.Failure = failureTargetUpdate
			return result, err
		}
		result.TargetUpdate = stageSucceeded
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

func updateTargetSecret(ctx context.Context, cfg *config.Config, sm SecretsProvider, secretValue string) error {
	log := logctx.From(ctx)

	var sourceData map[string]any
	if err := json.Unmarshal([]byte(secretValue), &sourceData); err != nil {
		return fmt.Errorf("handler: parse source secret: %w", err)
	}

	log.Info("updating target secret",
		"target_arn", cfg.TargetSecretARN,
		"keys", cfg.TargetSecretKeys,
	)

	if err := sm.MergeAndPutSecret(ctx, cfg.TargetSecretARN, sourceData, cfg.TargetSecretKeys); err != nil {
		return fmt.Errorf("handler: update target secret: %w", err)
	}

	log.Info("target secret updated", "target_arn", cfg.TargetSecretARN)
	return nil
}
