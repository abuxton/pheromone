package logging

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func TestWithAgentAttrs_allFields(t *testing.T) {
	base := slog.Default()
	enriched := WithAgentAttrs(base, "agent-01", "nginx-monitor", "default-reasoner")
	if enriched == base {
		t.Fatal("expected a new logger, got the same instance")
	}
}

func TestWithAgentAttrs_emptyFields(t *testing.T) {
	base := slog.Default()
	result := WithAgentAttrs(base, "", "", "")
	if result != base {
		t.Fatal("expected the original logger to be returned when all fields are empty")
	}
}

func TestWithAgentAttrs_partialFields(t *testing.T) {
	base := slog.Default()
	enriched := WithAgentAttrs(base, "agent-02", "", "")
	if enriched == base {
		t.Fatal("expected a new logger when agentID is set")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := WithRequestID(t.Context(), "req-abc")
	got := RequestIDFromContext(ctx)
	if got != "req-abc" {
		t.Errorf("RequestIDFromContext = %q, want %q", got, "req-abc")
	}
}

func TestRequestIDFromContext_missing(t *testing.T) {
	got := RequestIDFromContext(t.Context())
	if got != "" {
		t.Errorf("RequestIDFromContext on empty ctx = %q, want empty", got)
	}
}

func TestApplyAttr_noGroups(t *testing.T) {
	entry := make(map[string]any)
	a := slog.String("component", "api")
	applyAttr(entry, a, nil)
	if entry["component"] != "api" {
		t.Errorf("expected entry[component]=api, got %v", entry["component"])
	}
}

func TestApplyAttr_withGroups(t *testing.T) {
	entry := make(map[string]any)
	a := slog.String("id", "twin-01")
	applyAttr(entry, a, []string{"twin"})
	key := "twin.id"
	if entry[key] != "twin-01" {
		t.Errorf("expected entry[%q]=%q, got %v", key, "twin-01", entry[key])
	}
}

func TestApplyAttr_nestedGroups(t *testing.T) {
	entry := make(map[string]any)
	a := slog.String("name", "web-server-01")
	applyAttr(entry, a, []string{"agent", "meta"})
	key := "agent.meta.name"
	if entry[key] != "web-server-01" {
		t.Errorf("expected entry[%q]=%q, got %v", key, "web-server-01", entry[key])
	}
}

func TestNATSHandler_EnabledNilConn(t *testing.T) {
	h := NewNATSHandler(nil, "agent-01", nil)
	if !h.Enabled(t.Context(), slog.LevelInfo) {
		t.Error("Enabled should return true for LevelInfo with default opts")
	}
	if h.Enabled(t.Context(), slog.LevelDebug) {
		t.Error("Enabled should return false for LevelDebug with default opts (min=Info)")
	}
}

func TestNATSHandler_HandleNilConn(t *testing.T) {
	h := NewNATSHandler(nil, "agent-01", nil)
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	// nil conn — must return nil (no-op)
	if err := h.Handle(t.Context(), r); err != nil {
		t.Errorf("Handle with nil conn returned error: %v", err)
	}
}

func TestNATSHandler_WithAttrs(t *testing.T) {
	h := NewNATSHandler(nil, "agent-01", nil)
	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "api")})
	nh, ok := h2.(*NATSHandler)
	if !ok {
		t.Fatal("WithAttrs should return *NATSHandler")
	}
	if len(nh.attrs) != 1 {
		t.Errorf("expected 1 attr, got %d", len(nh.attrs))
	}
	if nh.attrs[0].Key != "component" {
		t.Errorf("expected attr key=component, got %q", nh.attrs[0].Key)
	}
}

func TestNATSHandler_WithGroup(t *testing.T) {
	h := NewNATSHandler(nil, "agent-01", nil)
	h2 := h.WithGroup("twin")
	nh, ok := h2.(*NATSHandler)
	if !ok {
		t.Fatal("WithGroup should return *NATSHandler")
	}
	if len(nh.groups) != 1 || nh.groups[0] != "twin" {
		t.Errorf("expected groups=[twin], got %v", nh.groups)
	}
}

func TestNATSHandler_WithGroupEmpty(t *testing.T) {
	h := NewNATSHandler(nil, "agent-01", nil)
	h2 := h.WithGroup("")
	if h2 != h {
		t.Error("WithGroup(\"\") should return the same handler")
	}
}

func TestNATSHandler_Clone_isolation(t *testing.T) {
	h := NewNATSHandler(nil, "agent-01", nil)
	h.attrs = append(h.attrs, slog.String("existing", "val"))

	h2, ok := h.WithAttrs([]slog.Attr{slog.String("new", "v2")}).(*NATSHandler)
	if !ok {
		t.Fatal("WithAttrs should return *NATSHandler")
	}
	// Modifying original attrs slice should not affect clone.
	h.attrs[0] = slog.String("existing", "modified")
	if h2.attrs[0].Value.String() != "val" {
		t.Error("clone should be isolated from mutations to original attrs")
	}
	if len(h2.attrs) != 2 {
		t.Errorf("clone should have 2 attrs, got %d", len(h2.attrs))
	}
}

func TestNATSHandler_LevelFiltering(t *testing.T) {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	h := NewNATSHandler(nil, "agent-01", opts)
	if h.Enabled(t.Context(), slog.LevelInfo) {
		t.Error("Info should be disabled when min level is Warn")
	}
	if !h.Enabled(t.Context(), slog.LevelWarn) {
		t.Error("Warn should be enabled when min level is Warn")
	}
	if !h.Enabled(t.Context(), slog.LevelError) {
		t.Error("Error should be enabled when min level is Warn")
	}
}

// TestNATSHandler_RequestIDFromContext verifies that the context helpers round-trip.
func TestNATSHandler_RequestIDFromContext(t *testing.T) {
	ctx := WithRequestID(t.Context(), "test-req-id")
	id := RequestIDFromContext(ctx)
	if id != "test-req-id" {
		t.Errorf("expected test-req-id, got %q", id)
	}
}

// TestNATSHandler_ImplementsSlogHandler verifies the interface is satisfied at compile time.
func TestNATSHandler_ImplementsSlogHandler(t *testing.T) {
	var _ slog.Handler = (*NATSHandler)(nil)
}

// TestNewNATSHandler_EmptyAgentIDFallback verifies that an empty agentID is
// replaced with "unknown" to avoid publishing to a bare "logs." subject.
func TestNewNATSHandler_EmptyAgentIDFallback(t *testing.T) {
	h := NewNATSHandler(nil, "", nil)
	if h.agentID != "unknown" {
		t.Errorf("expected agentID=unknown for empty input, got %q", h.agentID)
	}
}

// TestNewNATSHandler_AgentIDPreserved verifies that a non-empty agentID is kept.
func TestNewNATSHandler_AgentIDPreserved(t *testing.T) {
	h := NewNATSHandler(nil, "agent-42", nil)
	if h.agentID != "agent-42" {
		t.Errorf("expected agentID=agent-42, got %q", h.agentID)
	}
}

// TestValueToJSON_Kinds verifies that valueToJSON produces JSON-safe values for
// the common slog.Value kinds.
func TestValueToJSON_Kinds(t *testing.T) {
	tests := []struct {
		name string
		val  slog.Value
		want any
	}{
		{"bool true", slog.BoolValue(true), true},
		{"bool false", slog.BoolValue(false), false},
		{"int64", slog.Int64Value(42), int64(42)},
		{"uint64", slog.Uint64Value(99), uint64(99)},
		{"float64", slog.Float64Value(3.14), float64(3.14)},
		{"string", slog.StringValue("hello"), "hello"},
		{"duration", slog.DurationValue(time.Second), "1s"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := valueToJSON(tc.val)
			if got != tc.want {
				t.Errorf("valueToJSON(%v) = %v (%T), want %v (%T)", tc.val, got, got, tc.want, tc.want)
			}
		})
	}
}

// TestValueToJSON_Error verifies that error values are serialized as strings.
func TestValueToJSON_Error(t *testing.T) {
	err := fmt.Errorf("something went wrong")
	v := slog.AnyValue(err)
	got := valueToJSON(v)
	if got != "something went wrong" {
		t.Errorf("expected error string, got %v", got)
	}
}

// TestValueToJSON_Group verifies that KindGroup is serialized as map[string]any.
func TestValueToJSON_Group(t *testing.T) {
	attrs := []slog.Attr{slog.String("k", "v")}
	v := slog.GroupValue(attrs...)
	got := valueToJSON(v)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for group, got %T", got)
	}
	if m["k"] != "v" {
		t.Errorf("expected group[k]=v, got %v", m["k"])
	}
}

// TestApplyAttr_ResolvesValue verifies that applyAttr resolves LogValuer values.
func TestApplyAttr_ResolvesValue(t *testing.T) {
	entry := make(map[string]any)
	// slog.IntValue wraps an int64; valueToJSON should return int64.
	a := slog.Attr{Key: "count", Value: slog.IntValue(7)}
	applyAttr(entry, a, nil)
	if entry["count"] != int64(7) {
		t.Errorf("expected int64(7), got %v (%T)", entry["count"], entry["count"])
	}
}
