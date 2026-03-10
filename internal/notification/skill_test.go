package notification_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/notification"
)

// slowClient is a mock HTTPClient that sleeps for the given duration before
// returning an error, simulating an unresponsive webhook endpoint.
type slowClient struct {
	delay time.Duration
}

func (c *slowClient) Do(_ *http.Request) (*http.Response, error) {
	time.Sleep(c.delay)
	return nil, io.EOF
}

// TestNotifyDirectNonBlocking verifies that NotifyDirect returns in < 10 ms even
// when every destination would take 5 seconds to respond.
func TestNotifyDirectNonBlocking(t *testing.T) {
	skill := notification.NewNotificationSkill(
		[]byte("test-secret"),
		&slowClient{delay: 5 * time.Second},
	)

	dests := []notification.NotificationDestination{
		{Type: "http-webhook", URL: "http://0.0.0.0:1/unreachable", TimeoutSeconds: 10},
		{Type: "http-webhook", URL: "http://0.0.0.0:1/unreachable", TimeoutSeconds: 10},
		{Type: "http-webhook", URL: "http://0.0.0.0:1/unreachable", TimeoutSeconds: 10},
	}

	event := notification.ActionEvent{
		AgentID:    "agent-1",
		TwinID:     "twin-1",
		SkillName:  "config-enforce",
		ActionType: "notify-direct",
		Outcome:    "success",
		Timestamp:  time.Now(),
	}

	start := time.Now()
	skill.NotifyDirect(context.Background(), event, dests)
	elapsed := time.Since(start)

	if elapsed >= 10*time.Millisecond {
		t.Fatalf("NotifyDirect blocked for %v; expected < 10 ms", elapsed)
	}
}

// TestNotifyDirectLogsFailure confirms that NotifyDirect does not crash and returns
// quickly even when all destinations return HTTP 500.
func TestNotifyDirectLogsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	skill := notification.NewNotificationSkill([]byte("test-secret"), srv.Client())

	dests := []notification.NotificationDestination{
		{Type: "http-webhook", URL: srv.URL, TimeoutSeconds: 5},
	}

	event := notification.ActionEvent{
		AgentID:   "agent-1",
		TwinID:    "twin-1",
		Outcome:   "failure",
		Timestamp: time.Now(),
	}

	start := time.Now()
	skill.NotifyDirect(context.Background(), event, dests)
	elapsed := time.Since(start)

	if elapsed >= 10*time.Millisecond {
		t.Fatalf("NotifyDirect blocked for %v; expected < 10 ms", elapsed)
	}

	// Allow background goroutine time to complete its request without crashing.
	time.Sleep(200 * time.Millisecond)
}
