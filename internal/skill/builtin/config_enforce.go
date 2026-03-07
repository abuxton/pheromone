package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/abuxton/pheromone/internal/skill"
)

// ConfigEnforceSkill applies desired configuration to the managed system and
// validates compliance. It is available to all twin levels (TwinLevelWorkload).
type ConfigEnforceSkill struct {
	// supportedVersions lists the config schema versions this skill can apply.
	supportedVersions map[string]bool
}

// NewConfigEnforceSkill creates a ConfigEnforceSkill that accepts the listed versions.
func NewConfigEnforceSkill(supportedVersions ...string) *ConfigEnforceSkill {
	sv := make(map[string]bool, len(supportedVersions))
	for _, v := range supportedVersions {
		sv[v] = true
	}
	return &ConfigEnforceSkill{supportedVersions: sv}
}

func (s *ConfigEnforceSkill) Name() string                  { return "config-enforce" }
func (s *ConfigEnforceSkill) Version() string               { return "1.0.0" }
func (s *ConfigEnforceSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelWorkload }

// Execute runs the config enforcement action.
// Supported action types: "apply-config", "validate-compliance", "rollback".
func (s *ConfigEnforceSkill) Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	switch action.ActionType {
	case "apply-config":
		return s.applyConfig(ctx, obs, action)
	case "validate-compliance":
		return s.validateCompliance(obs, action.TwinID)
	case "rollback":
		return s.rollback(action)
	default:
		return nil, fmt.Errorf("config-enforce: unknown action type %q", action.ActionType)
	}
}

func (s *ConfigEnforceSkill) applyConfig(_ context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	version, ok := action.Params["version"]
	if !ok {
		return nil, fmt.Errorf("config-enforce: apply-config requires 'version' param")
	}

	if len(s.supportedVersions) > 0 && !s.supportedVersions[version] {
		supported := make([]string, 0, len(s.supportedVersions))
		for v := range s.supportedVersions {
			supported = append(supported, v)
		}
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("unsupported config version %q; supported: %s", version, strings.Join(supported, ", ")),
		}, nil
	}

	var desired *skill.TwinModel
	for _, d := range obs.DesiredTwins {
		if d.TwinID == action.TwinID {
			desired = d
			break
		}
	}
	if desired == nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("no desired state found for twin %q", action.TwinID),
		}, nil
	}

	applied := make([]string, 0, len(desired.State))
	for k := range desired.State {
		applied = append(applied, k)
	}

	return &skill.SkillResult{
		Success:       true,
		AppliedFields: applied,
		StateDelta:    &skill.TwinDelta{UpdatedFields: desired.State},
		Detail:        fmt.Sprintf("applied config v%s to twin %q (%d fields)", version, action.TwinID, len(applied)),
	}, nil
}

func (s *ConfigEnforceSkill) validateCompliance(obs *skill.Observations, twinID string) (*skill.SkillResult, error) {
	var desired, actual *skill.TwinModel
	for _, d := range obs.DesiredTwins {
		if d.TwinID == twinID {
			desired = d
		}
	}
	for _, a := range obs.Twins {
		if a.TwinID == twinID {
			actual = a
		}
	}

	if desired == nil || actual == nil {
		return nil, fmt.Errorf("config-enforce: twin %q not found in observations", twinID)
	}

	var drifted []string
	for k, dv := range desired.State {
		if av := actual.State[k]; av != dv {
			drifted = append(drifted, k)
		}
	}

	compliant := len(drifted) == 0
	detail := "compliant"
	if !compliant {
		detail = fmt.Sprintf("drift in fields: %s", strings.Join(drifted, ", "))
	}

	return &skill.SkillResult{
		Success: true,
		Detail:  detail,
	}, nil
}

func (s *ConfigEnforceSkill) rollback(action *skill.Action) (*skill.SkillResult, error) {
	targetVersion, ok := action.Params["target_version"]
	if !ok {
		return nil, fmt.Errorf("config-enforce: rollback requires 'target_version' param")
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("rolled back twin %q to version %s", action.TwinID, targetVersion),
	}, nil
}
