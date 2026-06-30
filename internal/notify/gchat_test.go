package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 6, 20, 18, 6, 17, 0, time.UTC)

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		payload  Payload
		contains []string
		absent   []string
	}{
		{
			name: "success includes instance resources and localized date",
			payload: Payload{
				InstanceName: "V4 Production",
				EventName:    "RotationSucceeded",
				AccountID:    "123456789012",
				Region:       "us-east-1",
				SourceSecret: "example-source",
				TargetSecret: "example-target",
				ECSCluster:   "my-cluster",
				ECSServices:  []string{"backend", "worker"},
				RequestID:    "request-123",
				Actions: []string{
					"✅ Target secret updated",
					"✅ ECS force-deployment request accepted",
				},
				Status:       StatusSuccess,
				OccurredAt:   testTime,
				TimezoneName: "America/Lima",
			},
			contains: []string{
				"Syncret — V4 Production",
				"RotationSucceeded",
				"123456789012",
				"us-east-1",
				"example-source",
				"example-target",
				"my-cluster",
				"backend, worker",
				"2026-06-20 13:06:17 -05:00 (America/Lima)",
				"request-123",
				"✅ Target secret updated",
				"✅ ECS force-deployment request accepted",
				"✅ Success",
			},
			absent: []string{"arn:aws:"},
		},
		{
			name: "failure includes sanitized error and event secret mismatch",
			payload: Payload{
				EventName:    "PutSecretValue",
				SourceSecret: "configured-source",
				EventSecret:  "unexpected-source",
				RequestID:    "request-456",
				Actions:      []string{"❌ Event rejected by source secret guard"},
				Status:       StatusFailed,
				ErrorMessage: "Event secret does not match the configured source secret; see Lambda logs using the request ID",
				OccurredAt:   testTime,
				TimezoneName: "UTC",
			},
			contains: []string{
				"configured-source",
				"unexpected-source",
				"request-456",
				"❌ Failed",
				"Event secret does not match",
			},
			absent: []string{"✅ Success"},
		},
		{
			name: "skipped status omits optional resources",
			payload: Payload{
				EventName:    "RotationFailed",
				SourceSecret: "example-source",
				Status:       StatusSkipped,
				OccurredAt:   testTime,
				TimezoneName: "UTC",
			},
			contains: []string{"RotationFailed", "⚠️ Skipped"},
			absent:   []string{"*Actions:*", "*Target secret:*", "*ECS cluster:*", "*ECS services:*"},
		},
		{
			name: "New York timezone applies daylight saving offset",
			payload: Payload{
				EventName:    "PutSecretValue",
				SourceSecret: "example-source",
				Status:       StatusSuccess,
				OccurredAt:   testTime,
				TimezoneName: "America/New_York",
			},
			contains: []string{"2026-06-20 14:06:17 -04:00 (America/New_York)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := format(tt.payload)
			for _, want := range tt.contains {
				if !strings.Contains(msg, want) {
					t.Errorf("format() missing %q\ngot:\n%s", want, msg)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(msg, absent) {
					t.Errorf("format() should not contain %q\ngot:\n%s", absent, msg)
				}
			}
		})
	}
}

func TestGoogleChat_Send(t *testing.T) {
	t.Run("successful send", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		gc := NewGoogleChat(srv.URL)
		err := gc.Send(context.Background(), Payload{
			EventName:  "RotationSucceeded",
			Status:     StatusSuccess,
			OccurredAt: testTime,
		})
		if err != nil {
			t.Fatalf("Send() unexpected error: %v", err)
		}
	})

	t.Run("non-2xx response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		gc := NewGoogleChat(srv.URL)
		err := gc.Send(context.Background(), Payload{OccurredAt: testTime})
		if err == nil {
			t.Fatal("Send() expected error for non-2xx response, got nil")
		}
	})

	t.Run("invalid webhook URL returns error", func(t *testing.T) {
		gc := NewGoogleChat("://invalid-url")
		err := gc.Send(context.Background(), Payload{OccurredAt: testTime})
		if err == nil {
			t.Fatal("Send() expected error for invalid URL, got nil")
		}
	})
}
