package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSHandler is a slog.Handler that publishes structured JSON log records to
// a NATS subject of the form "logs.{agentID}". It is safe for concurrent use.
//
// If the NATS connection is nil or becomes disconnected, records are silently
// discarded to avoid blocking the caller.
type NATSHandler struct {
	conn    *nats.Conn
	agentID string
	opts    *slog.HandlerOptions
	attrs   []slog.Attr
	groups  []string
}

// NewNATSHandler returns a NATSHandler that publishes log records to NATS.
// If conn is nil, all Handle calls are no-ops.
func NewNATSHandler(conn *nats.Conn, agentID string, opts *slog.HandlerOptions) *NATSHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &NATSHandler{
		conn:    conn,
		agentID: agentID,
		opts:    opts,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *NATSHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle publishes the log record as a JSON message to "logs.{agentID}".
func (h *NATSHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.conn == nil || !h.conn.IsConnected() {
		return nil
	}

	entry := make(map[string]any, 6+len(h.attrs)+r.NumAttrs())
	entry["time"] = r.Time.UTC().Format(time.RFC3339Nano)
	entry["level"] = r.Level.String()
	entry["msg"] = r.Message
	if h.agentID != "" {
		entry["agent_id"] = h.agentID
	}

	// Add pre-set attributes.
	for _, a := range h.attrs {
		applyAttr(entry, a, h.groups)
	}

	// Add per-record attributes.
	r.Attrs(func(a slog.Attr) bool {
		applyAttr(entry, a, h.groups)
		return true
	})

	// Propagate request_id from context if present.
	if rid, _ := ctx.Value(requestIDCtxKey{}).(string); rid != "" {
		entry["request_id"] = rid
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("nats handler: marshal: %w", err)
	}

	subject := "logs." + h.agentID
	if err := h.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("nats handler: publish %s: %w", subject, err)
	}
	return nil
}

// WithAttrs returns a new handler with the given attributes pre-set on every record.
func (h *NATSHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := h.clone()
	newH.attrs = append(newH.attrs, attrs...)
	return newH
}

// WithGroup returns a new handler that scopes subsequent attributes under the named group.
func (h *NATSHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newH := h.clone()
	newH.groups = append(newH.groups, name)
	return newH
}

func (h *NATSHandler) clone() *NATSHandler {
	newH := *h
	newH.attrs = make([]slog.Attr, len(h.attrs))
	copy(newH.attrs, h.attrs)
	newH.groups = make([]string, len(h.groups))
	copy(newH.groups, h.groups)
	return &newH
}

// applyAttr inserts a slog.Attr into entry, prefixing with any active group names.
func applyAttr(entry map[string]any, a slog.Attr, groups []string) {
	key := a.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}
	entry[key] = a.Value.Any()
}

// requestIDCtxKey is the context key type used by this package to propagate request IDs.
// HTTP middleware in the api package uses the same unexported key pattern; if the
// request ID must be visible here, callers should attach it using WithRequestID.
type requestIDCtxKey struct{}

// WithRequestID stores a request ID in ctx so NATSHandler can include it in records.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// RequestIDFromContext retrieves the request ID from ctx, or "" if not set.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDCtxKey{}).(string)
	return v
}
