package builtin

import (
	"context"
	"fmt"
	"sync"

	"github.com/abuxton/pheromone/internal/skill"
)

// DigitalTwinSkill is the canonical skill for reading, updating, and diffing twin models.
// It is restricted to OS-level agents (TwinLevelOS) because OS agents are responsible
// for the authoritative twin model; workload agents receive read-only projections via
// their workload skill bundle.
//
// An OS-level actor may explicitly grant workload agents access via AccessPolicy.Grant.
type DigitalTwinSkill struct {
	mu     sync.RWMutex
	models map[string]*skill.TwinModel
}

// NewDigitalTwinSkill creates a DigitalTwinSkill backed by an in-process model store.
// In production this store is backed by the gRPC TwinControl stream (ADR-003).
func NewDigitalTwinSkill() *DigitalTwinSkill {
	return &DigitalTwinSkill{models: make(map[string]*skill.TwinModel)}
}

func (s *DigitalTwinSkill) Name() string            { return "digital-twin" }
func (s *DigitalTwinSkill) Version() string          { return "1.0.0" }
func (s *DigitalTwinSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }

// Execute dispatches the action to the appropriate twin operation.
// Supported action types: "read-twin", "update-twin", "diff-twin", "apply-model".
func (s *DigitalTwinSkill) Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	switch action.ActionType {
	case "read-twin":
		return s.readTwin(action.TwinID)
	case "update-twin":
		return s.updateTwin(action.TwinID, action.Params)
	case "diff-twin":
		return s.diffTwin(obs, action.TwinID)
	case "apply-model":
		return s.applyModel(action.TwinID, action.Params)
	default:
		return nil, fmt.Errorf("digital-twin: unknown action type %q", action.ActionType)
	}
}

// ReadTwin retrieves the model for twinID (exported for direct use by tests/reasoners).
func (s *DigitalTwinSkill) ReadTwin(twinID string) (*skill.TwinModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.models[twinID]
	if !ok {
		return nil, fmt.Errorf("digital-twin: twin %q not found", twinID)
	}
	return m, nil
}

// StoreTwin stores or replaces a twin model (used during initialization and by tests).
func (s *DigitalTwinSkill) StoreTwin(m *skill.TwinModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[m.TwinID] = m
}

// UpdateTwin merges delta into the stored model for twinID.
func (s *DigitalTwinSkill) UpdateTwin(twinID string, delta *skill.TwinDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.models[twinID]
	if !ok {
		return fmt.Errorf("digital-twin: twin %q not found", twinID)
	}
	for k, v := range delta.UpdatedFields {
		m.State[k] = v
	}
	return nil
}

// DiffModel computes the drift between desired and actual twin models.
func (s *DigitalTwinSkill) DiffModel(desired, actual *skill.TwinModel) *skill.DriftReport {
	report := &skill.DriftReport{TwinID: desired.TwinID}
	for k, dv := range desired.State {
		av, ok := actual.State[k]
		if !ok || av != dv {
			report.HasDrift = true
			report.DriftedFields = append(report.DriftedFields, skill.FieldDrift{
				Field:   k,
				Desired: dv,
				Actual:  av,
			})
		}
	}
	return report
}

// readTwin wraps ReadTwin as a SkillResult.
func (s *DigitalTwinSkill) readTwin(twinID string) (*skill.SkillResult, error) {
	m, err := s.ReadTwin(twinID)
	if err != nil {
		return nil, err
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("read twin %q version %s", m.TwinID, m.Version),
	}, nil
}

// updateTwin applies params as a TwinDelta.
func (s *DigitalTwinSkill) updateTwin(twinID string, params map[string]string) (*skill.SkillResult, error) {
	delta := &skill.TwinDelta{UpdatedFields: params}
	if err := s.UpdateTwin(twinID, delta); err != nil {
		return nil, err
	}
	return &skill.SkillResult{
		Success:    true,
		StateDelta: delta,
		Detail:     fmt.Sprintf("updated %d fields on twin %q", len(params), twinID),
	}, nil
}

// diffTwin computes drift for twinID using observations.
func (s *DigitalTwinSkill) diffTwin(obs *skill.Observations, twinID string) (*skill.SkillResult, error) {
	var desired, actual *skill.TwinModel
	for _, d := range obs.DesiredTwins {
		if d.TwinID == twinID {
			desired = d
			break
		}
	}
	for _, a := range obs.Twins {
		if a.TwinID == twinID {
			actual = a
			break
		}
	}
	if desired == nil || actual == nil {
		return nil, fmt.Errorf("digital-twin: twin %q not found in observations", twinID)
	}
	report := s.DiffModel(desired, actual)
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("drift detected=%v fields=%d", report.HasDrift, len(report.DriftedFields)),
	}, nil
}

// applyModel marks an apply operation as successful (real implementation would
// invoke system operations via the OS-level skill bundle).
func (s *DigitalTwinSkill) applyModel(twinID string, params map[string]string) (*skill.SkillResult, error) {
	applied := make([]string, 0, len(params))
	for k := range params {
		applied = append(applied, k)
	}
	return &skill.SkillResult{
		Success:       true,
		AppliedFields: applied,
		Detail:        fmt.Sprintf("applied model to twin %q", twinID),
	}, nil
}
