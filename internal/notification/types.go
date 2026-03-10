package notification

import (
	"net/http"
	"time"
)

// ActionEvent is the structured outcome of a skill execution, emitted by the
// NotificationSkill after each agent reasoning-loop action (ADR-011).
type ActionEvent struct {
	// AgentID identifies the agent that produced this event.
	AgentID string
	// TwinID identifies the twin the action was applied to.
	TwinID string
	// SkillName is the skill that executed the action.
	SkillName string
	// ActionType is the specific operation that was performed.
	ActionType string
	// Outcome is one of "success", "failure", or "partial".
	Outcome string
	// Detail is a human-readable description of the outcome.
	Detail string
	// TraceID is an optional distributed-trace correlation identifier.
	TraceID string
	// Timestamp is when the event was created.
	Timestamp time.Time
	// Labels contains arbitrary key-value metadata for routing and filtering.
	Labels map[string]string
}

// NotificationDestination is a single delivery target for a notification.
type NotificationDestination struct {
	// Type is "http-webhook" or "log".
	Type string
	// URL is the HTTP endpoint for "http-webhook" destinations.
	URL string
	// Headers contains additional HTTP request headers.
	Headers map[string]string
	// TimeoutSeconds is the per-request HTTP timeout (default 10).
	TimeoutSeconds int
	// MaxRetries is the maximum number of delivery retries (default 3).
	MaxRetries int
}

// HTTPClient is a minimal interface satisfied by *http.Client and test doubles.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
