// Package skill defines the Pheromone skill framework (ADR-006).
//
// In the Pheromone agentic model (ADR-007), agents are AI-capable autonomous processes
// whose capabilities are expressed as discrete *skills*. Skills are the unit of
// deployment and distribution: the server holds a registry of available skills and
// distributes appropriate skill bundles to each agent based on its twin-level and
// granted access policy.
//
// Hierarchy (highest → lowest priority):
//
//	TwinLevelOS  (0) — OS-level twin; may access all skills including restricted ones
//	TwinLevelWorkload (1) — Workload-level twin; access limited by policy
//
// A skill declares a MinTwinLevel. The access policy enforces that only agents managing
// twins at or above that level may invoke the skill, providing hierarchical segmentation
// of responsibilities and access control across the layered twin architecture.
package skill

import (
	"context"
	"fmt"
	"time"
)

// TwinLevel represents the priority tier of a digital twin in the layered architecture.
// Lower numeric values indicate higher priority / broader access.
type TwinLevel int

const (
	// TwinLevelOS is the highest-priority tier: OS-level twins that manage the
	// underlying operating system. Agents at this level may access all skills.
	TwinLevelOS TwinLevel = 0

	// TwinLevelWorkload is a lower-priority tier: workload twins managing application
	// services. Access to restricted skills requires an explicit grant from OS-level.
	TwinLevelWorkload TwinLevel = 1
)

// String returns a human-readable name for the TwinLevel.
func (l TwinLevel) String() string {
	switch l {
	case TwinLevelOS:
		return "os"
	case TwinLevelWorkload:
		return "workload"
	default:
		return fmt.Sprintf("unknown(%d)", int(l))
	}
}

// TwinRef identifies a twin managed by an agent.
type TwinRef struct {
	// ID is the unique twin identifier.
	ID string
	// Level is the twin's position in the hierarchy.
	Level TwinLevel
}

// Metric is a single named measurement with labels.
type Metric struct {
	// Name is the OpenMetrics-style metric name, e.g. "nginx_requests_total".
	Name string
	// Value is the numeric measurement.
	Value float64
	// Labels contains key-value context, e.g. {"agent_id": "agent-1"}.
	Labels map[string]string
	// Timestamp is when the metric was observed.
	Timestamp time.Time
}

// TwinModel represents the desired or actual state of a twin.
type TwinModel struct {
	// TwinID is the unique identifier of the twin.
	TwinID string
	// Level is the twin's position in the hierarchy.
	Level TwinLevel
	// Version is the model version string.
	Version string
	// State contains key-value state fields, e.g. {"nginx_version": "1.24"}.
	State map[string]string
}

// TwinDelta is a partial state update applied to a twin model.
type TwinDelta struct {
	// UpdatedFields contains only the fields that changed.
	UpdatedFields map[string]string
}

// DriftReport summarises the gap between desired and actual twin state.
type DriftReport struct {
	// TwinID identifies the twin being compared.
	TwinID string
	// HasDrift is true when desired and actual states differ.
	HasDrift bool
	// DriftedFields lists the fields where desired ≠ actual.
	DriftedFields []FieldDrift
}

// FieldDrift captures a single field-level discrepancy.
type FieldDrift struct {
	// Field is the state key that differs.
	Field string
	// Desired is the desired value.
	Desired string
	// Actual is the currently observed value.
	Actual string
}

// ApplyResult summarises the outcome of applying a twin model to the system.
type ApplyResult struct {
	// Success is true when all state changes were applied without error.
	Success bool
	// AppliedFields lists the fields that were successfully applied.
	AppliedFields []string
	// FailedFields lists the fields that could not be applied.
	FailedFields []string
	// Detail is a human-readable summary.
	Detail string
}

// Observations aggregates all inputs to a single reasoning cycle.
type Observations struct {
	// Metrics are the raw metric observations collected this cycle.
	Metrics []Metric
	// Twins contains the current actual state for each managed twin.
	Twins []*TwinModel
	// DesiredTwins contains the desired state for each managed twin.
	DesiredTwins []*TwinModel
}

// Action is a unit of work planned by the reasoning loop.
type Action struct {
	// SkillName identifies which skill should execute this action.
	SkillName string
	// ActionType is the specific operation, e.g. "apply-config", "rollback".
	ActionType string
	// TwinID is the target twin for this action.
	TwinID string
	// Params contains action-specific key-value parameters.
	Params map[string]string
	// RequiresApproval is true when the action must be confirmed by the server.
	RequiresApproval bool
	// Rationale is a human-readable explanation of why this action was planned.
	Rationale string
}

// SkillResult is returned by a Skill after execution.
type SkillResult struct {
	// Success is true when the skill completed without error.
	Success bool
	// Metrics are any additional metrics produced by this skill execution.
	Metrics []Metric
	// StateDelta is the twin state change resulting from this execution.
	StateDelta *TwinDelta
	// AppliedFields lists state fields that were successfully applied.
	AppliedFields []string
	// FailedFields lists state fields that could not be applied.
	FailedFields []string
	// Detail is a human-readable outcome summary.
	Detail string
}

// Skill is the interface every Pheromone skill must satisfy.
//
// Skills are the unit of deployment: the server holds a SkillRegistry and distributes
// skill bundles to agents. Each skill declares its name, version, and the minimum
// TwinLevel required to invoke it, enabling the hierarchical access policy.
type Skill interface {
	// Name returns the skill's unique identifier, e.g. "digital-twin".
	Name() string

	// Version returns the skill contract version (semver).
	Version() string

	// MinTwinLevel returns the minimum TwinLevel an agent must manage to be allowed
	// to invoke this skill. Skills restricted to OS-level twins return TwinLevelOS;
	// skills available to all twins return TwinLevelWorkload.
	MinTwinLevel() TwinLevel

	// Execute runs the skill for the given action and observations.
	// ctx carries cancellation; obs provides the current environment snapshot;
	// action supplies the parameters for this invocation.
	Execute(ctx context.Context, obs *Observations, action *Action) (*SkillResult, error)
}
