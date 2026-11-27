package notify

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPostText_errorOmitsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	webhookURL := srv.URL + "/v1/spaces/AAA/messages?key=k&token=secret-token"
	srv.Close() // connection refused

	err := postText(context.Background(), newWebhookClient(), webhookURL, "hello")
	if err == nil {
		t.Fatal("postText() expected error for a closed server, got nil")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Errorf("error %q exposes the webhook URL", err)
	}
}

func TestPostText_reusesConnection(t *testing.T) {
	var conns atomic.Int32
	body := strings.Repeat("x", 32<<10)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	client := newWebhookClient()
	for range 3 {
		if err := postText(context.Background(), client, srv.URL, "hello"); err != nil {
			t.Fatalf("postText() unexpected error: %v", err)
		}
	}
	if got := conns.Load(); got != 1 {
		t.Errorf("connections = %d, want 1", got)
	}
}
