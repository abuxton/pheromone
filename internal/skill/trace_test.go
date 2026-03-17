package skill_test

import (
	"context"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// -----------------------------------------------------------------------------
// Unit tests for TraceStore (ADR-019)
// -----------------------------------------------------------------------------

func TestTraceStore_EmitAndGet(t *testing.T) {
	ts := skill.NewTraceStore(10)

	trace := skill.ReasonerTrace{
		Timestamp:  time.Now().UTC(),
		AgentID:    "agent-1",
		Reasoner:   "rule-based",
		Input:      skill.TraceInput{TwinID: "twin-1", HasDrift: true, DriftedFields: []string{"nginx_version"}},
		Actions:    []string{"apply-config"},
		Outcome:    "actions-planned",
		DurationMs: 0,
	}
	ts.Emit(trace)

	got := ts.Get("agent-1", 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(got))
	}
	if got[0].AgentID != "agent-1" {
		t.Errorf("expected agent-id 'agent-1', got %q", got[0].AgentID)
	}
	if got[0].Outcome != "actions-planned" {
		t.Errorf("expected outcome 'actions-planned', got %q", got[0].Outcome)
	}
}

func TestTraceStore_RingBufferEvictsOldest(t *testing.T) {
	const capacity = 5
	ts := skill.NewTraceStore(capacity)

	for i := 0; i < capacity+3; i++ {
		ts.Emit(skill.ReasonerTrace{
			AgentID: "agent-1",
			Actions: []string{},
			Outcome: "no-drift",
		})
	}

	all := ts.Get("agent-1", 0)
	if len(all) != capacity {
		t.Errorf("expected ring buffer to cap at %d entries, got %d", capacity, len(all))
	}
}

func TestTraceStore_GetNReturnsLastN(t *testing.T) {
	ts := skill.NewTraceStore(20)
	for i := 0; i < 15; i++ {
		ts.Emit(skill.ReasonerTrace{AgentID: "agent-1", Outcome: "no-drift"})
	}
	got := ts.Get("agent-1", 5)
	if len(got) != 5 {
		t.Errorf("expected 5 traces when n=5, got %d", len(got))
	}
}

func TestTraceStore_UnknownAgentReturnsNil(t *testing.T) {
	ts := skill.NewTraceStore(10)
	got := ts.Get("agent-unknown", 0)
	if got != nil {
		t.Errorf("expected nil for unknown agent, got %v", got)
	}
}

func TestTraceStore_IsolatesPerAgent(t *testing.T) {
	ts := skill.NewTraceStore(10)
	ts.Emit(skill.ReasonerTrace{AgentID: "agent-a", Outcome: "no-drift"})
	ts.Emit(skill.ReasonerTrace{AgentID: "agent-b", Outcome: "actions-planned"})

	a := ts.Get("agent-a", 0)
	b := ts.Get("agent-b", 0)
	if len(a) != 1 {
		t.Errorf("expected 1 trace for agent-a, got %d", len(a))
	}
	if len(b) != 1 {
		t.Errorf("expected 1 trace for agent-b, got %d", len(b))
	}
	if a[0].Outcome != "no-drift" {
		t.Errorf("agent-a outcome mismatch: %q", a[0].Outcome)
	}
	if b[0].Outcome != "actions-planned" {
		t.Errorf("agent-b outcome mismatch: %q", b[0].Outcome)
	}
}

// -----------------------------------------------------------------------------
// Unit tests for RuleBasedReasoner trace emission (ADR-019)
// -----------------------------------------------------------------------------

func TestRuleBasedReasoner_EmitsTraceOnNoDrift(t *testing.T) {
	store := skill.NewTraceStore(10)
	r := skill.NewRuleBasedReasoner()
	r.AgentID = "agent-1"
	r.Emitter = store

	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{
		"twin-1": {TwinID: "twin-1", Version: "1.0.0"},
	}}
	drift := skill.DriftReport{TwinID: "twin-1", HasDrift: false}

	_, err := r.Plan(context.Background(), obs, goal, drift)
	if err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}

	traces := store.Get("agent-1", 0)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace emitted, got %d", len(traces))
	}
	tr := traces[0]
	if tr.Outcome != "no-drift" {
		t.Errorf("expected outcome 'no-drift', got %q", tr.Outcome)
	}
	if tr.Reasoner != "rule-based" {
		t.Errorf("expected reasoner 'rule-based', got %q", tr.Reasoner)
	}
	if tr.Input.TwinID != "twin-1" {
		t.Errorf("expected input TwinID 'twin-1', got %q", tr.Input.TwinID)
	}
	if len(tr.Actions) != 0 {
		t.Errorf("expected no actions on no-drift trace, got %v", tr.Actions)
	}
}

func TestRuleBasedReasoner_EmitsTraceOnDrift(t *testing.T) {
	store := skill.NewTraceStore(10)
	r := skill.NewRuleBasedReasoner()
	r.AgentID = "agent-2"
	r.Emitter = store

	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{
		"twin-2": {TwinID: "twin-2", Version: "2.0.0"},
	}}
	drift := skill.DriftReport{
		TwinID:   "twin-2",
		HasDrift: true,
		DriftedFields: []skill.FieldDrift{
			{Field: "nginx_version", Desired: "1.24", Actual: "1.20"},
		},
	}

	actions, err := r.Plan(context.Background(), obs, goal, drift)
	if err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	traces := store.Get("agent-2", 0)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace emitted, got %d", len(traces))
	}
	tr := traces[0]
	if tr.Outcome != "actions-planned" {
		t.Errorf("expected outcome 'actions-planned', got %q", tr.Outcome)
	}
	if len(tr.Actions) != 1 || tr.Actions[0] != "apply-config" {
		t.Errorf("expected actions ['apply-config'], got %v", tr.Actions)
	}
	if len(tr.Input.DriftedFields) != 1 || tr.Input.DriftedFields[0] != "nginx_version" {
		t.Errorf("expected drifted field 'nginx_version', got %v", tr.Input.DriftedFields)
	}
}

func TestRuleBasedReasoner_NoEmitterDoesNotPanic(t *testing.T) {
	r := skill.NewRuleBasedReasoner()
	// No emitter set — should not panic
	obs := &skill.Observations{}
	goal := skill.GoalState{Twins: map[string]*skill.TwinModel{}}
	drift := skill.DriftReport{TwinID: "twin-1", HasDrift: false}

	if _, err := r.Plan(context.Background(), obs, goal, drift); err != nil {
		t.Fatalf("Plan returned unexpected error: %v", err)
	}
}

func TestRuleBasedReasoner_TraceTimestamp(t *testing.T) {
	store := skill.NewTraceStore(10)
	r := skill.NewRuleBasedReasoner()
	r.AgentID = "agent-3"
	r.Emitter = store

	before := time.Now().UTC()
	_, _ = r.Plan(context.Background(), &skill.Observations{}, skill.GoalState{Twins: map[string]*skill.TwinModel{}}, skill.DriftReport{TwinID: "twin-3"})
	after := time.Now().UTC()

	traces := store.Get("agent-3", 0)
	if len(traces) == 0 {
		t.Fatal("no trace emitted")
	}
	ts := traces[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("trace timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}
