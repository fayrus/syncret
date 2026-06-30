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

func TestBuildNotifyPayload_context(t *testing.T) {
	occurredAt := time.Date(2026, 6, 20, 18, 0, 0, 0, time.UTC)
	result := executionResult{
		EventName:      event.RotationSucceeded,
		EventSecretARN: sourceARN,
		TargetUpdate:   stageSucceeded,
		ECSForceDeploy: stageSucceeded,
	}
	p := buildNotifyPayload(notificationConfig(), result, occurredAt, "request-123", nil)

	if p.InstanceName != "V4 Production" {
		t.Errorf("InstanceName = %q, want V4 Production", p.InstanceName)
	}
	if p.AccountID != "123" {
		t.Errorf("AccountID = %q, want 123", p.AccountID)
	}
	if p.Region != "us-east-1" {
		t.Errorf("Region = %q, want us-east-1", p.Region)
	}
	if p.SourceSecret != "source" {
		t.Errorf("SourceSecret = %q, want source", p.SourceSecret)
	}
	if p.TargetSecret != "target" {
		t.Errorf("TargetSecret = %q, want target", p.TargetSecret)
	}
	if p.ECSCluster != "my-cluster" {
		t.Errorf("ECSCluster = %q, want my-cluster", p.ECSCluster)
	}
	if strings.Join(p.ECSServices, ",") != "backend,worker" {
		t.Errorf("ECSServices = %v, want [backend worker]", p.ECSServices)
	}
	if p.RequestID != "request-123" {
		t.Errorf("RequestID = %q, want request-123", p.RequestID)
	}
	if !p.OccurredAt.Equal(occurredAt) {
		t.Errorf("OccurredAt = %v, want %v", p.OccurredAt, occurredAt)
	}
	if p.TimezoneName != "America/Lima" {
		t.Errorf("TimezoneName = %q, want America/Lima", p.TimezoneName)
	}
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
			assertBuildNotifyPayload(t, p, tt.wantEvent, tt.wantEventSecret, tt.wantStatus, tt.wantActions, tt.wantErrorMessage)
		})
	}
}

func assertBuildNotifyPayload(t *testing.T, p notify.Payload, wantEvent, wantEventSecret string, wantStatus notify.Status, wantActions []string, wantErrorMessage string) {
	t.Helper()
	if p.EventName != wantEvent {
		t.Errorf("EventName = %q, want %q", p.EventName, wantEvent)
	}
	if p.EventSecret != wantEventSecret {
		t.Errorf("EventSecret = %q, want %q", p.EventSecret, wantEventSecret)
	}
	if p.Status != wantStatus {
		t.Errorf("Status = %q, want %q", p.Status, wantStatus)
	}
	if strings.Join(p.Actions, "|") != strings.Join(wantActions, "|") {
		t.Errorf("Actions = %v, want %v", p.Actions, wantActions)
	}
	if p.ErrorMessage != wantErrorMessage {
		t.Errorf("ErrorMessage = %q, want %q", p.ErrorMessage, wantErrorMessage)
	}
	if strings.Contains(p.ErrorMessage, "internal details") || strings.Contains(p.ErrorMessage, "raw ARN") {
		t.Errorf("ErrorMessage leaked raw handler error: %q", p.ErrorMessage)
	}
}
