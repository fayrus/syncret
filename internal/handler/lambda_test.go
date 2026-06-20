package handler

import (
	"errors"
	"testing"
	"time"

	"github.com/fayrus/syncret/internal/config"
)

func TestBuildNotifyPayload(t *testing.T) {
	occurredAt := time.Date(2026, 6, 20, 18, 0, 0, 0, time.UTC)

	baseCfg := &config.Config{
		SecretARN:       sourceARN,
		TargetSecretARN: targetARN,
	}

	tests := []struct {
		name        string
		cfg         *config.Config
		payload     []byte
		err         error
		wantEvent   string
		wantSecret  string
		wantActions int
		wantErr     bool
	}{
		{
			name:        "RotationSucceeded with target secret and ECS",
			cfg:         withECS(baseConfig(), "backend", "worker"),
			payload:     serviceEvent("RotationSucceeded", sourceARN),
			wantEvent:   "RotationSucceeded",
			wantSecret:  sourceARN,
			wantActions: 2,
		},
		{
			name:        "PutSecretValue ECS only",
			cfg:         ecsOnlyConfig("app"),
			payload:     rotationEvent("PutSecretValue", sourceARN),
			wantEvent:   "PutSecretValue",
			wantSecret:  sourceARN,
			wantActions: 1,
		},
		{
			name:        "RotationFailed has no actions",
			cfg:         baseCfg,
			payload:     serviceEvent("RotationFailed", sourceARN),
			wantEvent:   "RotationFailed",
			wantSecret:  sourceARN,
			wantActions: 0,
		},
		{
			name:        "handler error is propagated to payload",
			cfg:         baseCfg,
			payload:     serviceEvent("RotationSucceeded", sourceARN),
			err:         errors.New("ecs: service not found"),
			wantEvent:   "RotationSucceeded",
			wantSecret:  sourceARN,
			wantActions: 1,
			wantErr:     true,
		},
		{
			name:        "unparseable payload falls back to unknown",
			cfg:         baseCfg,
			payload:     []byte(`{bad json}`),
			wantEvent:   "unknown",
			wantSecret:  sourceARN,
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := buildNotifyPayload(tt.cfg, tt.payload, occurredAt, tt.err)

			if p.EventName != tt.wantEvent {
				t.Errorf("EventName = %q, want %q", p.EventName, tt.wantEvent)
			}
			if p.SecretARN != tt.wantSecret {
				t.Errorf("SecretARN = %q, want %q", p.SecretARN, tt.wantSecret)
			}
			if len(p.Actions) != tt.wantActions {
				t.Errorf("Actions = %v (len %d), want len %d", p.Actions, len(p.Actions), tt.wantActions)
			}
			if (p.Err != nil) != tt.wantErr {
				t.Errorf("Err = %v, wantErr %v", p.Err, tt.wantErr)
			}
			if !p.OccurredAt.Equal(occurredAt) {
				t.Errorf("OccurredAt = %v, want %v", p.OccurredAt, occurredAt)
			}
		})
	}
}
