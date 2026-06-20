package notify

import (
	"context"
	"errors"
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
			name: "success with all actions",
			payload: Payload{
				EventName:  "RotationSucceeded",
				SecretARN:  "arn:aws:secretsmanager:us-east-1:123:secret/db",
				Actions:    []string{"Target secret updated", "ECS redeployment: backend, worker"},
				OccurredAt: testTime,
			},
			contains: []string{
				"RotationSucceeded",
				"2026-06-20 18:06:17 UTC",
				"arn:aws:secretsmanager:us-east-1:123:secret/db",
				"Target secret updated",
				"ECS redeployment: backend, worker",
				"✅ Success",
			},
		},
		{
			name: "failure includes error message",
			payload: Payload{
				EventName:  "RotationSucceeded",
				SecretARN:  "arn:aws:secretsmanager:us-east-1:123:secret/db",
				Err:        errors.New("ecs: service not found"),
				OccurredAt: testTime,
			},
			contains: []string{"❌ Failed", "ecs: service not found"},
			absent:   []string{"✅ Success"},
		},
		{
			name: "no actions section when actions empty",
			payload: Payload{
				EventName:  "RotationFailed",
				SecretARN:  "arn:aws:secretsmanager:us-east-1:123:secret/db",
				OccurredAt: testTime,
			},
			contains: []string{"RotationFailed", "✅ Success"},
			absent:   []string{"*Actions:*"},
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
			SecretARN:  "arn:x",
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
