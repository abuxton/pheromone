package skill

import (
	"sync"
	"time"
)

// ReasonerTrace captures the complete observability record for a single
// reasoning cycle (ADR-019). It is emitted by every AIReasoner implementation
// on each call to Plan(), regardless of outcome.
type ReasonerTrace struct {
	// Timestamp is when Plan() was called (UTC).
	Timestamp time.Time `json:"timestamp"`
	// AgentID identifies the agent that performed the reasoning cycle.
	AgentID string `json:"agent_id"`
	// Reasoner is the implementation type, e.g. "rule-based", "ollama".
	Reasoner string `json:"reasoner"`
	// Input is the TwinID under consideration and its drift state.
	Input TraceInput `json:"input"`
	// Actions lists the planned action types (may be empty).
	Actions []string `json:"actions"`
	// Outcome is "actions-planned", "no-drift", or "error".
	Outcome string `json:"outcome"`
	// DurationMs is how long Plan() took in milliseconds.
	DurationMs int64 `json:"duration_ms"`
	// FallbackUsed is true when a higher-tier reasoner failed and the
	// rule-based fallback was substituted.
	FallbackUsed bool `json:"fallback_used"`
}

// TraceInput captures the key inputs to a reasoning cycle for observability.
type TraceInput struct {
	// TwinID is the twin being reasoned about.
	TwinID string `json:"twin_id"`
	// HasDrift indicates whether drift was detected.
	HasDrift bool `json:"has_drift"`
	// DriftedFields lists the field names that differed.
	DriftedFields []string `json:"drifted_fields,omitempty"`
}

// TraceEmitter is implemented by anything that receives reasoning traces.
// The server can implement it to buffer traces in memory; a NATS publisher
// can implement it to forward traces to the traces.{agent_id} subject
// (ADR-019).
type TraceEmitter interface {
	Emit(trace ReasonerTrace)
}

// TraceStore is a bounded in-memory ring buffer of the last maxTraces
// ReasonerTrace entries per agent (ADR-019 action audit log).
type TraceStore struct {
	mu        sync.RWMutex
	maxTraces int
	traces    map[string][]ReasonerTrace // keyed by agent_id
}

// NewTraceStore creates a TraceStore that retains the last capacity traces
// per agent. If capacity is <= 0 it defaults to 100.
func NewTraceStore(capacity int) *TraceStore {
	if capacity <= 0 {
		capacity = 100
	}
	return &TraceStore{
		maxTraces: capacity,
		traces:    make(map[string][]ReasonerTrace),
	}
}

// Emit appends a trace to the per-agent ring buffer, evicting the oldest
// entry when the buffer is full. It satisfies TraceEmitter.
func (ts *TraceStore) Emit(trace ReasonerTrace) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	buf := ts.traces[trace.AgentID]
	buf = append(buf, trace)
	if len(buf) > ts.maxTraces {
		buf = buf[len(buf)-ts.maxTraces:]
	}
	ts.traces[trace.AgentID] = buf
}

// Get returns a copy of the most recent n traces for the given agent.
// If n <= 0 or n >= len(buffer), all stored traces are returned.
func (ts *TraceStore) Get(agentID string, n int) []ReasonerTrace {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	buf := ts.traces[agentID]
	if len(buf) == 0 {
		return nil
	}
	if n > 0 && n < len(buf) {
		buf = buf[len(buf)-n:]
	}
	out := make([]ReasonerTrace, len(buf))
	copy(out, buf)
	return out
}
