package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
