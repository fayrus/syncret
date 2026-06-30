package handler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fayrus/syncret/internal/config"
	"github.com/fayrus/syncret/internal/event"
	"github.com/fayrus/syncret/internal/notify"
)

func notificationConfig() *config.Config {
	cfg := withECS(baseConfig(), "backend", "worker")
	cfg.InstanceName = "V4 Production"
	cfg.Timezone = "America/Lima"
	return cfg
}

func TestBuildNotifyPayload(t *testing.T) {
	occurredAt := time.Date(2026, 6, 20, 18, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		result           executionResult
		err              error
		wantEvent        string
		wantEventSecret  string
		wantStatus       notify.Status
		wantActions      []string
		wantErrorMessage string
	}{
		{
			name: "successful target update and ECS request",
			result: executionResult{
				EventName:      event.RotationSucceeded,
				EventSecretARN: sourceARN,
				TargetUpdate:   stageSucceeded,
				ECSForceDeploy: stageSucceeded,
			},
			wantEvent:  "RotationSucceeded",
			wantStatus: notify.StatusSuccess,
			wantActions: []string{
				"✅ Target secret updated",
				"✅ ECS force-deployment request accepted",
			},
		},
		{
			name: "rotation failure reports skipped stages",
			result: executionResult{
				EventName:      event.RotationFailed,
				EventSecretARN: sourceARN,
				TargetUpdate:   stageSkipped,
				ECSForceDeploy: stageSkipped,
			},
			wantEvent:  "RotationFailed",
			wantStatus: notify.StatusSkipped,
			wantActions: []string{
				"⏭ Target secret update skipped",
				"⏭ ECS force-deployment skipped",
			},
		},
		{
			name: "source read failure is sanitized",
			result: executionResult{
				EventName:      event.RotationSucceeded,
				EventSecretARN: sourceARN,
				TargetUpdate:   stageNotAttempted,
				ECSForceDeploy: stageNotAttempted,
				Failure:        failureSourceRead,
			},
			err:        errors.New("AccessDeniedException: internal details"),
			wantEvent:  "RotationSucceeded",
			wantStatus: notify.StatusFailed,
			wantActions: []string{
				"❌ Source secret read failed",
				"⏭ Target secret update not attempted",
				"⏭ ECS force-deployment not attempted",
			},
			wantErrorMessage: "Unable to read source secret; see Lambda logs using the request ID",
		},
		{
			name: "target update failure is sanitized",
			result: executionResult{
				EventName:      event.RotationSucceeded,
				EventSecretARN: sourceARN,
				TargetUpdate:   stageFailed,
				ECSForceDeploy: stageNotAttempted,
				Failure:        failureTargetUpdate,
			},
			err:        errors.New("ThrottlingException: raw target ARN"),
			wantEvent:  "RotationSucceeded",
			wantStatus: notify.StatusFailed,
			wantActions: []string{
				"❌ Target secret update failed",
				"⏭ ECS force-deployment not attempted",
			},
			wantErrorMessage: "Unable to update target secret; see Lambda logs using the request ID",
		},
		{
			name: "ECS request failure preserves successful target stage",
			result: executionResult{
				EventName:      event.RotationSucceeded,
				EventSecretARN: sourceARN,
				TargetUpdate:   stageSucceeded,
				ECSForceDeploy: stageFailed,
				Failure:        failureECSForceDeploy,
			},
			err:        errors.New("service backend unavailable"),
			wantEvent:  "RotationSucceeded",
			wantStatus: notify.StatusFailed,
			wantActions: []string{
				"✅ Target secret updated",
				"❌ ECS force-deployment request failed",
			},
			wantErrorMessage: "One or more ECS force-deployment requests failed; see Lambda logs using the request ID",
		},
		{
			name: "invalid event reports parse failure",
			result: executionResult{
				TargetUpdate:   stageNotAttempted,
				ECSForceDeploy: stageNotAttempted,
				Failure:        failureEventParse,
			},
			err:        errors.New("invalid character"),
			wantEvent:  "unknown",
			wantStatus: notify.StatusFailed,
			wantActions: []string{
				"❌ Event parsing failed",
				"⏭ Target secret update not attempted",
				"⏭ ECS force-deployment not attempted",
			},
			wantErrorMessage: "Event payload could not be parsed; see Lambda logs using the request ID",
		},
		{
			name: "ARN mismatch includes unexpected event secret name",
			result: executionResult{
				EventName:      event.PutSecretValue,
				EventSecretARN: "arn:aws:secretsmanager:us-east-1:123:secret/unexpected",
				TargetUpdate:   stageNotAttempted,
				ECSForceDeploy: stageNotAttempted,
				Failure:        failureARNGuard,
			},
			err:             errors.New("raw ARN mismatch details"),
			wantEvent:       "PutSecretValue",
			wantEventSecret: "unexpected",
			wantStatus:      notify.StatusFailed,
			wantActions: []string{
				"❌ Event rejected by source secret guard",
				"⏭ Target secret update not attempted",
				"⏭ ECS force-deployment not attempted",
			},
			wantErrorMessage: "Event secret does not match the configured source secret; see Lambda logs using the request ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := buildNotifyPayload(notificationConfig(), tt.result, occurredAt, "request-123", tt.err)

			if p.InstanceName != "V4 Production" {
				t.Errorf("InstanceName = %q, want V4 Production", p.InstanceName)
			}
			if p.EventName != tt.wantEvent {
				t.Errorf("EventName = %q, want %q", p.EventName, tt.wantEvent)
			}
			if p.AccountID != "123" || p.Region != "us-east-1" {
				t.Errorf("AWS identity = %s/%s, want 123/us-east-1", p.AccountID, p.Region)
			}
			if p.SourceSecret != "source" || p.TargetSecret != "target" {
				t.Errorf("secrets = %q/%q, want source/target", p.SourceSecret, p.TargetSecret)
			}
			if p.EventSecret != tt.wantEventSecret {
				t.Errorf("EventSecret = %q, want %q", p.EventSecret, tt.wantEventSecret)
			}
			if p.ECSCluster != "my-cluster" || strings.Join(p.ECSServices, ",") != "backend,worker" {
				t.Errorf("ECS resources = %q/%v", p.ECSCluster, p.ECSServices)
			}
			if p.RequestID != "request-123" {
				t.Errorf("RequestID = %q, want request-123", p.RequestID)
			}
			if p.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", p.Status, tt.wantStatus)
			}
			if strings.Join(p.Actions, "|") != strings.Join(tt.wantActions, "|") {
				t.Errorf("Actions = %v, want %v", p.Actions, tt.wantActions)
			}
			if p.ErrorMessage != tt.wantErrorMessage {
				t.Errorf("ErrorMessage = %q, want %q", p.ErrorMessage, tt.wantErrorMessage)
			}
			if strings.Contains(p.ErrorMessage, "internal details") || strings.Contains(p.ErrorMessage, "raw ARN") {
				t.Errorf("ErrorMessage leaked raw handler error: %q", p.ErrorMessage)
			}
			if !p.OccurredAt.Equal(occurredAt) || p.TimezoneName != "America/Lima" {
				t.Errorf("time context = %v/%q", p.OccurredAt, p.TimezoneName)
			}
		})
	}
}
