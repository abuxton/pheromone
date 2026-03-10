package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// NotificationSkill is the ADR-011 skill interface for post-action hook delivery.
// Implementations emit ActionEvents to one or more NotificationDestinations.
type NotificationSkill interface {
	skill.Skill

	// NotifyDirect fans out event to each destination in a fire-and-forget goroutine.
	// It returns immediately without waiting for delivery outcomes.
	NotifyDirect(ctx context.Context, event ActionEvent, dests []NotificationDestination)

	// NotifyServer forwards event to the Pheromone server via gRPC (Phase 2 stub).
	NotifyServer(ctx context.Context, event ActionEvent) error
}

// NotificationSkillImpl is the concrete implementation of NotificationSkill.
type NotificationSkillImpl struct {
	secret     []byte
	httpClient HTTPClient
	log        *slog.Logger
}

// NewNotificationSkill creates a NotificationSkillImpl that signs payloads with
// secret and executes HTTP requests via httpClient.
func NewNotificationSkill(secret []byte, httpClient HTTPClient) *NotificationSkillImpl {
	return &NotificationSkillImpl{
		secret:     secret,
		httpClient: httpClient,
		log:        slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// Name returns the skill identifier.
func (n *NotificationSkillImpl) Name() string { return "notification" }

// Version returns the skill contract version.
func (n *NotificationSkillImpl) Version() string { return "v0.1.0" }

// MinTwinLevel returns the minimum twin level required to invoke this skill.
// Notification is available to all agent tiers.
func (n *NotificationSkillImpl) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelWorkload }

// Execute dispatches the notification action encoded in action.Params.
//
// Supported action types:
//   - "notify-direct": fire-and-forget fan-out to destinations encoded in
//     action.Params["destinations"] (JSON array of NotificationDestination).
//   - "notify-server": stub gRPC delivery; always succeeds.
//
// The ActionEvent is built from action fields and action.Params.
func (n *NotificationSkillImpl) Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	event := ActionEvent{
		AgentID:    action.Params["agent_id"],
		TwinID:     action.TwinID,
		SkillName:  action.Params["source_skill"],
		ActionType: action.ActionType,
		Outcome:    action.Params["outcome"],
		Detail:     action.Rationale,
		TraceID:    action.Params["trace_id"],
		Timestamp:  time.Now(),
	}

	switch action.ActionType {
	case "notify-direct":
		var dests []NotificationDestination
		if raw := action.Params["destinations"]; raw != "" {
			if err := json.Unmarshal([]byte(raw), &dests); err != nil {
				return &skill.SkillResult{
					Success: false,
					Detail:  fmt.Sprintf("invalid destinations JSON: %v", err),
				}, nil
			}
		}
		n.NotifyDirect(ctx, event, dests)
		return &skill.SkillResult{
			Success: true,
			Detail:  fmt.Sprintf("notify-direct dispatched to %d destinations", len(dests)),
		}, nil

	case "notify-server":
		if err := n.NotifyServer(ctx, event); err != nil {
			return &skill.SkillResult{Success: false, Detail: err.Error()}, nil
		}
		return &skill.SkillResult{Success: true, Detail: "notify-server stub: queued"}, nil

	default:
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("unknown action type: %q", action.ActionType),
		}, nil
	}
}

// NotifyDirect fans out event to each destination in its own goroutine.
// It returns immediately; failures are logged and never surfaced to the caller.
func (n *NotificationSkillImpl) NotifyDirect(ctx context.Context, event ActionEvent, dests []NotificationDestination) {
	payload, err := json.Marshal(event)
	if err != nil {
		n.log.Error("notification: failed to marshal event",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, dest := range dests {
		dest := dest // capture loop variable
		go func() {
			n.dispatch(ctx, dest, payload)
		}()
	}
}

// NotifyServer is a Phase 2 stub that will forward events to the Pheromone server
// via gRPC. Currently a no-op.
func (n *NotificationSkillImpl) NotifyServer(_ context.Context, _ ActionEvent) error {
	return nil
}

// dispatch sends payload as an HTTP POST to dest, attaching the HMAC signature
// header. Errors are logged and discarded; the function never blocks the caller.
func (n *NotificationSkillImpl) dispatch(ctx context.Context, dest NotificationDestination, payload []byte) {
	timeout := dest.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, dest.URL, bytes.NewReader(payload))
	if err != nil {
		n.log.Error("notification: failed to build request",
			slog.String("url", dest.URL),
			slog.String("error", err.Error()),
		)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pheromone-Signature", ComputeHMAC(payload, n.secret))
	for k, v := range dest.Headers {
		req.Header.Set(k, v)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		n.log.Error("notification: dispatch failed",
			slog.String("url", dest.URL),
			slog.String("error", err.Error()),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		n.log.Warn("notification: non-2xx response",
			slog.String("url", dest.URL),
			slog.Int("status", resp.StatusCode),
		)
	}
}
