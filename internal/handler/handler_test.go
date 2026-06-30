package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fayrus/syncret/internal/config"
)

// ── mock clients ─────────────────────────────────────────────────────────────

type mockSecrets struct {
	values  map[string]string
	getErr  error
	putErr  error
	putArns []string
}

func (m *mockSecrets) GetSecretString(_ context.Context, arn string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.values[arn], nil
}

func (m *mockSecrets) MergeAndPutSecret(_ context.Context, targetARN string, _ map[string]any, _ []string) error {
	m.putArns = append(m.putArns, targetARN)
	return m.putErr
}

type mockECS struct {
	updateCalls []string
	updateErr   error
}

func (m *mockECS) ForceNewDeployment(_ context.Context, _ string, services []string) error {
	m.updateCalls = append(m.updateCalls, services...)
	return m.updateErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

const (
	sourceARN = "arn:aws:secretsmanager:us-east-1:123:secret/source"
	targetARN = "arn:aws:secretsmanager:us-east-1:123:secret/target"
)

func baseConfig() *config.Config {
	return &config.Config{
		SecretARN:        sourceARN,
		AWSRegion:        "us-east-1",
		TargetSecretARN:  targetARN,
		TargetSecretKeys: []string{"password"},
	}
}

func withECS(c *config.Config, services ...string) *config.Config {
	c.ECSForceDeploy = true
	c.ECSCluster = "my-cluster"
	c.ECSServices = services
	return c
}

func ecsOnlyConfig(services ...string) *config.Config {
	return withECS(&config.Config{
		SecretARN: sourceARN,
		AWSRegion: "us-east-1",
	}, services...)
}

func rotationEvent(eventName, secretARN string) []byte {
	evt := map[string]any{
		"source":      "aws.secretsmanager",
		"detail-type": "AWS API Call via CloudTrail",
		"time":        "2026-01-01T00:00:00Z",
		"detail": map[string]any{
			"eventSource": "secretsmanager.amazonaws.com",
			"eventName":   eventName,
			"requestParameters": map[string]any{
				"secretId": secretARN,
			},
		},
	}
	b, _ := json.Marshal(evt)
	return b
}

func serviceEvent(eventName, secretARN string) []byte {
	evt := map[string]any{
		"source":      "aws.secretsmanager",
		"detail-type": "AWS Service Event via CloudTrail",
		"time":        "2026-01-01T00:00:00Z",
		"detail": map[string]any{
			"eventSource":       "secretsmanager.amazonaws.com",
			"eventName":         eventName,
			"requestParameters": nil,
			"additionalEventData": map[string]any{
				"SecretId": secretARN,
			},
		},
	}
	b, _ := json.Marshal(evt)
	return b
}

func secretJSON(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func makeDeps(sm *mockSecrets, ecsClient *mockECS) Dependencies {
	return Dependencies{
		Secrets: sm,
		ECS:     ecsClient,
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestHandle(t *testing.T) {
	sourceSecret := secretJSON(map[string]any{"username": "app", "password": "newpass"})
	targetSecret := secretJSON(map[string]any{
		"host": "db.example.com", "port": "5432", "dbname": "mydb",
		"username": "app", "password": "old",
	})

	tests := []struct {
		name         string
		cfg          *config.Config
		payload      []byte
		secretValues map[string]string
		getErr       error
		putErr       error
		ecsUpdateErr error
		wantErr      bool
		wantECSCalls int
		wantPutCalls int
	}{
		{
			name:         "RotationSucceeded updates target secret",
			cfg:          baseConfig(),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			wantPutCalls: 1,
		},
		{
			name:         "PutSecretValue updates target secret",
			cfg:          baseConfig(),
			payload:      rotationEvent("PutSecretValue", sourceARN),
			wantPutCalls: 1,
		},
		{
			name:         "RotationFailed skips target update and redeploy",
			cfg:          baseConfig(),
			payload:      serviceEvent("RotationFailed", sourceARN),
			wantECSCalls: 0,
			wantPutCalls: 0,
		},
		{
			name:         "ECS force deploy enabled updates target secret and redeploys",
			cfg:          withECS(baseConfig(), "syncret-db"),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			secretValues: map[string]string{sourceARN: sourceSecret, targetARN: targetSecret},
			wantECSCalls: 1,
			wantPutCalls: 1,
		},
		{
			name:         "ECS force deploy disabled skips redeploy",
			cfg:          baseConfig(),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			wantECSCalls: 0,
			wantPutCalls: 1,
		},
		{
			name:         "get source secret error stops pipeline",
			cfg:          withECS(baseConfig(), "syncret-db"),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			getErr:       errors.New("access denied"),
			wantErr:      true,
			wantECSCalls: 0,
			wantPutCalls: 0,
		},
		{
			name:         "target secret update error stops before ECS",
			cfg:          withECS(baseConfig(), "syncret-db"),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			putErr:       errors.New("throttled"),
			wantErr:      true,
			wantECSCalls: 0,
			wantPutCalls: 1,
		},
		{
			name:         "RotationSucceeded ARN mismatch returns error",
			cfg:          baseConfig(),
			payload:      serviceEvent("RotationSucceeded", "arn:aws:secretsmanager:us-east-1:123:secret/other"),
			wantErr:      true,
			wantECSCalls: 0,
			wantPutCalls: 0,
		},
		{
			name:         "PutSecretValue ARN mismatch returns error",
			cfg:          baseConfig(),
			payload:      rotationEvent("PutSecretValue", "arn:aws:secretsmanager:us-east-1:123:secret/other"),
			wantErr:      true,
			wantECSCalls: 0,
			wantPutCalls: 0,
		},
		{
			name:         "invalid payload returns error",
			cfg:          baseConfig(),
			payload:      []byte(`{bad json}`),
			wantErr:      true,
			wantECSCalls: 0,
			wantPutCalls: 0,
		},
		{
			name:         "ecs error is returned",
			cfg:          withECS(baseConfig(), "syncret-db"),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			ecsUpdateErr: errors.New("throttled"),
			wantErr:      true,
			wantECSCalls: 1,
			wantPutCalls: 1,
		},
		{
			name:         "multiple ECS services all redeployed",
			cfg:          withECS(baseConfig(), "api", "worker"),
			payload:      serviceEvent("RotationSucceeded", sourceARN),
			wantECSCalls: 2,
			wantPutCalls: 1,
		},
		{
			name:         "ECS-only config skips secret update",
			cfg:          ecsOnlyConfig("app"),
			payload:      rotationEvent("PutSecretValue", sourceARN),
			wantECSCalls: 1,
			wantPutCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.secretValues
			if values == nil {
				values = map[string]string{sourceARN: sourceSecret, targetARN: targetSecret}
			}

			sm := &mockSecrets{values: values, getErr: tt.getErr, putErr: tt.putErr}
			ecsClient := &mockECS{updateErr: tt.ecsUpdateErr}
			deps := makeDeps(sm, ecsClient)

			err := handle(context.Background(), tt.cfg, tt.payload, deps)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(ecsClient.updateCalls) != tt.wantECSCalls {
				t.Errorf("ECS UpdateService calls = %d, want %d", len(ecsClient.updateCalls), tt.wantECSCalls)
			}
			if len(sm.putArns) != tt.wantPutCalls {
				t.Errorf("PutSecretValue calls = %d, want %d", len(sm.putArns), tt.wantPutCalls)
			}
		})
	}
}

func TestExecute_resultTracksPipelineStages(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		getErr      error
		putErr      error
		ecsErr      error
		wantErr     bool
		wantTarget  stageStatus
		wantECS     stageStatus
		wantFailure failureKind
	}{
		{
			name:       "all configured stages succeed",
			payload:    serviceEvent("RotationSucceeded", sourceARN),
			wantTarget: stageSucceeded,
			wantECS:    stageSucceeded,
		},
		{
			name:        "source read failure prevents later stages",
			payload:     serviceEvent("RotationSucceeded", sourceARN),
			getErr:      errors.New("access denied"),
			wantErr:     true,
			wantTarget:  stageNotAttempted,
			wantECS:     stageNotAttempted,
			wantFailure: failureSourceRead,
		},
		{
			name:        "target update failure prevents ECS request",
			payload:     serviceEvent("RotationSucceeded", sourceARN),
			putErr:      errors.New("throttled"),
			wantErr:     true,
			wantTarget:  stageFailed,
			wantECS:     stageNotAttempted,
			wantFailure: failureTargetUpdate,
		},
		{
			name:        "ECS request failure preserves completed target update",
			payload:     serviceEvent("RotationSucceeded", sourceARN),
			ecsErr:      errors.New("unavailable"),
			wantErr:     true,
			wantTarget:  stageSucceeded,
			wantECS:     stageFailed,
			wantFailure: failureECSForceDeploy,
		},
		{
			name:       "rotation failure skips configured stages",
			payload:    serviceEvent("RotationFailed", sourceARN),
			wantTarget: stageSkipped,
			wantECS:    stageSkipped,
		},
		{
			name:        "invalid event leaves configured stages unattempted",
			payload:     []byte(`{bad json}`),
			wantErr:     true,
			wantTarget:  stageNotAttempted,
			wantECS:     stageNotAttempted,
			wantFailure: failureEventParse,
		},
		{
			name:        "ARN mismatch leaves configured stages unattempted",
			payload:     serviceEvent("RotationSucceeded", "arn:aws:secretsmanager:us-east-1:123:secret/unexpected"),
			wantErr:     true,
			wantTarget:  stageNotAttempted,
			wantECS:     stageNotAttempted,
			wantFailure: failureARNGuard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := withECS(baseConfig(), "backend", "worker")
			sm := &mockSecrets{
				values: map[string]string{sourceARN: `{"password":"new"}`},
				getErr: tt.getErr,
				putErr: tt.putErr,
			}
			ecsClient := &mockECS{updateErr: tt.ecsErr}

			result, err := execute(context.Background(), cfg, tt.payload, makeDeps(sm, ecsClient))

			if (err != nil) != tt.wantErr {
				t.Fatalf("execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if result.TargetUpdate != tt.wantTarget {
				t.Errorf("TargetUpdate = %q, want %q", result.TargetUpdate, tt.wantTarget)
			}
			if result.ECSForceDeploy != tt.wantECS {
				t.Errorf("ECSForceDeploy = %q, want %q", result.ECSForceDeploy, tt.wantECS)
			}
			if result.Failure != tt.wantFailure {
				t.Errorf("Failure = %q, want %q", result.Failure, tt.wantFailure)
			}
		})
	}
}
