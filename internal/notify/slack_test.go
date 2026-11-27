package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlack_Send(t *testing.T) {
	t.Run("posts escaped text", func(t *testing.T) {
		var got map[string]string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode body: %v", err)
			}
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()

		err := NewSlack(srv.URL).Send(context.Background(), Payload{
			InstanceName: "Prod <EU> & Co",
			EventName:    "RotationSucceeded",
			Status:       StatusSuccess,
			OccurredAt:   testTime,
		})
		if err != nil {
			t.Fatalf("Send() unexpected error: %v", err)
		}
		text := got["text"]
		if !strings.Contains(text, "*Syncret — Prod &lt;EU&gt; &amp; Co*") {
			t.Errorf("text = %q, want escaped instance name in a bold header", text)
		}
		if !strings.Contains(text, "*Event:* RotationSucceeded") {
			t.Errorf("text = %q, want the event line", text)
		}
	})

	t.Run("non-2xx response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("no_service"))
		}))
		defer srv.Close()

		err := NewSlack(srv.URL).Send(context.Background(), Payload{OccurredAt: testTime})
		if err == nil || !strings.HasPrefix(err.Error(), "notify: slack: ") {
			t.Fatalf("Send() error = %v, want a notify: slack: error", err)
		}
	})
}
