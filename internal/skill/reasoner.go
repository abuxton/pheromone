package skill

import (
	"context"
	"fmt"
	"time"
)

// GoalState represents the desired operational objectives for one or more managed twins.
// It is passed to AIReasoner.Plan alongside the current observations and drift report so
// the reasoner can produce a targeted action plan.
type GoalState struct {
	// Twins contains the desired TwinModel for each managed twin, keyed by twin ID.
	Twins map[string]*TwinModel
}

// AIReasoner is the interface every Pheromone AI reasoning engine must satisfy (ADR-008).
//
// Implementations range from the deterministic rule-based reasoner used in Phase 1 (Tier 0,
// no external dependencies) to local LLM-backed engines (Tier 1, Ollama) and remote API
// engines (Tier 2) introduced in Phase 2.
//
// The framework calls Plan once per reasoning cycle; the reasoner returns a (possibly empty)
// list of Actions that the framework then dispatches to the appropriate skills.
type AIReasoner interface {
	// Plan derives a set of actions from the current observations, desired goal state,
	// and the pre-computed drift report for the relevant twin.
	//
	// ctx carries cancellation — implementations must respect it.
	// obs contains the full environment snapshot (metrics + all twin states) for this cycle.
	// goal contains the desired state for each managed twin.
	// drift is the gap between desired and actual state for the twin under consideration.
	Plan(ctx context.Context, obs *Observations, goal GoalState, drift DriftReport) ([]Action, error)
}

// RuleBasedReasoner is the Tier 0 (Phase 1 MVP) implementation of AIReasoner (ADR-008).
//
// It uses deterministic if/then rules derived entirely from the supplied DriftReport, with
// zero external dependencies and negligible resource overhead. When drift is detected it
// emits a single "config-enforce / apply-config" action; when there is no drift it returns
// an empty slice.
//
// This is always available as a fallback even on resource-constrained edge devices.
//
// If Emitter is non-nil, a ReasonerTrace is published after every Plan() call (ADR-019).
type RuleBasedReasoner struct {
	// AgentID is included in emitted traces so the trace can be attributed to
	// a specific agent. Optional: if empty, traces omit the agent identifier.
	AgentID string
	// Emitter receives a ReasonerTrace after each Plan() call (ADR-019).
	// If nil, trace emission is skipped.
	Emitter TraceEmitter
}

// NewRuleBasedReasoner constructs a RuleBasedReasoner ready for use.
func NewRuleBasedReasoner() *RuleBasedReasoner {
	return &RuleBasedReasoner{}
}

// Plan implements AIReasoner for the rule-based Tier 0 engine.
//
// Rules applied (in priority order):
//  1. If DriftReport.HasDrift is false → return no actions.
//  2. Otherwise → plan one "config-enforce / apply-config" action for the drifted twin,
//     including the desired version from GoalState and a structured rationale listing
//     every drifted field.
//
// If r.Emitter is non-nil a ReasonerTrace is published after the decision is
// made (ADR-019).
func (r *RuleBasedReasoner) Plan(_ context.Context, _ *Observations, goal GoalState, drift DriftReport) ([]Action, error) {
	start := time.Now()

	var actions []Action
	var outcome string

	if !drift.HasDrift {
		outcome = "no-drift"
	} else {
		version := ""
		if desired, ok := goal.Twins[drift.TwinID]; ok {
			version = desired.Version
		}

		rationale := fmt.Sprintf(
			"rule-based: drift detected on %d field(s) in twin %q",
			len(drift.DriftedFields), drift.TwinID,
		)

		actions = []Action{
			{
				SkillName:  "config-enforce",
				ActionType: "apply-config",
				TwinID:     drift.TwinID,
				Params:     map[string]string{"version": version},
				Rationale:  rationale,
			},
		}
		outcome = "actions-planned"
	}

	if r.Emitter != nil {
		driftedFields := make([]string, 0, len(drift.DriftedFields))
		for _, f := range drift.DriftedFields {
			driftedFields = append(driftedFields, f.Field)
		}
		actionTypes := make([]string, 0, len(actions))
		for _, a := range actions {
			actionTypes = append(actionTypes, a.ActionType)
		}
		r.Emitter.Emit(ReasonerTrace{
			Timestamp: start.UTC(),
			AgentID:   r.AgentID,
			Reasoner:  "rule-based",
			Input: TraceInput{
				TwinID:        drift.TwinID,
				HasDrift:      drift.HasDrift,
				DriftedFields: driftedFields,
			},
			Actions:    actionTypes,
			Outcome:    outcome,
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	return actions, nil
}
