package skill_test

import (
	"context"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/builtin"
)

// --- helpers ---

type mockAgent struct {
	twins   []*skill.TwinRef
	metrics []skill.Metric
	actual  map[string]*skill.TwinModel
	desired map[string]*skill.TwinModel
}

func (a *mockAgent) Initialize() ([]*skill.TwinRef, error) { return a.twins, nil }
func (a *mockAgent) CollectMetrics(_ context.Context) ([]skill.Metric, error) {
	return a.metrics, nil
}
func (a *mockAgent) CurrentState(_ context.Context, id string) (*skill.TwinModel, error) {
	if m, ok := a.actual[id]; ok {
		return m, nil
	}
	return nil, nil
}
func (a *mockAgent) DesiredState(_ context.Context, id string) (*skill.TwinModel, error) {
	if m, ok := a.desired[id]; ok {
		return m, nil
	}
	return nil, nil
}
func (a *mockAgent) EnforceConfig(_ context.Context, _ *skill.TwinModel) error { return nil }
func (a *mockAgent) Shutdown() error                                           { return nil }

func newOSTwinRef(id string) *skill.TwinRef {
	return &skill.TwinRef{ID: id, Level: skill.TwinLevelOS}
}

func newWorkloadTwinRef(id string) *skill.TwinRef {
	return &skill.TwinRef{ID: id, Level: skill.TwinLevelWorkload}
}

// --- TwinLevel ---

func TestTwinLevelString(t *testing.T) {
	cases := []struct {
		lvl  skill.TwinLevel
		want string
	}{
		{skill.TwinLevelOS, "os"},
		{skill.TwinLevelWorkload, "workload"},
	}
	for _, c := range cases {
		if got := c.lvl.String(); got != c.want {
			t.Errorf("TwinLevel(%d).String() = %q, want %q", c.lvl, got, c.want)
		}
	}
}

// --- AccessPolicy ---

func TestAccessPolicy_OSAgentAllowsAll(t *testing.T) {
	policy := skill.NewAccessPolicy()
	s := builtin.NewDigitalTwinSkill() // MinTwinLevel = OS
	err := policy.Check("agent-os", []skill.TwinLevel{skill.TwinLevelOS}, s)
	if err != nil {
		t.Errorf("OS agent should be allowed to invoke digital-twin: %v", err)
	}
}

func TestAccessPolicy_WorkloadAgentDeniedOSSkill(t *testing.T) {
	policy := skill.NewAccessPolicy()
	s := builtin.NewDigitalTwinSkill() // MinTwinLevel = OS
	err := policy.Check("agent-workload", []skill.TwinLevel{skill.TwinLevelWorkload}, s)
	if err == nil {
		t.Error("workload agent should be denied access to OS-only skill digital-twin")
	}
}

func TestAccessPolicy_WorkloadAgentAllowedWorkloadSkill(t *testing.T) {
	policy := skill.NewAccessPolicy()
	s := builtin.NewMetricsCollectionSkill() // MinTwinLevel = Workload
	err := policy.Check("agent-workload", []skill.TwinLevel{skill.TwinLevelWorkload}, s)
	if err != nil {
		t.Errorf("workload agent should be allowed to invoke metrics skill: %v", err)
	}
}

func TestAccessPolicy_ExplicitGrantOverridesHierarchy(t *testing.T) {
	policy := skill.NewAccessPolicy()
	policy.Grant("agent-workload", "digital-twin")
	s := builtin.NewDigitalTwinSkill()
	err := policy.Check("agent-workload", []skill.TwinLevel{skill.TwinLevelWorkload}, s)
	if err != nil {
		t.Errorf("explicit grant should allow workload agent to invoke digital-twin: %v", err)
	}
}

func TestAccessPolicy_RevokeRemovesGrant(t *testing.T) {
	policy := skill.NewAccessPolicy()
	policy.Grant("agent-workload", "digital-twin")
	policy.Revoke("agent-workload", "digital-twin")
	s := builtin.NewDigitalTwinSkill()
	err := policy.Check("agent-workload", []skill.TwinLevel{skill.TwinLevelWorkload}, s)
	if err == nil {
		t.Error("revoked grant should deny workload agent access to digital-twin")
	}
}

// --- SkillRegistry ---

func TestSkillRegistry_RegisterAndGet(t *testing.T) {
	reg := skill.NewSkillRegistry(skill.NewAccessPolicy())
	s := builtin.NewMetricsCollectionSkill()
	if err := reg.Register(s); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := reg.Get("metrics")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "metrics" {
		t.Errorf("expected 'metrics', got %q", got.Name())
	}
}

func TestSkillRegistry_DuplicateRegisterFails(t *testing.T) {
	reg := skill.NewSkillRegistry(skill.NewAccessPolicy())
	s := builtin.NewMetricsCollectionSkill()
	_ = reg.Register(s)
	if err := reg.Register(s); err == nil {
		t.Error("second Register should return an error")
	}
}

func TestSkillRegistry_DeregisterWorks(t *testing.T) {
	reg := skill.NewSkillRegistry(skill.NewAccessPolicy())
	s := builtin.NewMetricsCollectionSkill()
	_ = reg.Register(s)
	if err := reg.Deregister("metrics"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if _, err := reg.Get("metrics"); err == nil {
		t.Error("skill should not exist after Deregister")
	}
}

func TestSkillRegistry_BundleForOSAgent(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewDigitalTwinSkill())
	_ = reg.Register(builtin.NewMetricsCollectionSkill())
	_ = reg.Register(builtin.NewConfigEnforceSkill("1.0.0"))

	bundle := reg.BundleFor("agent-os", []skill.TwinLevel{skill.TwinLevelOS})
	if len(bundle) != 3 {
		t.Errorf("OS agent should receive all 3 skills, got %d", len(bundle))
	}
}

func TestSkillRegistry_BundleForWorkloadAgent(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewDigitalTwinSkill())          // OS-only
	_ = reg.Register(builtin.NewMetricsCollectionSkill())    // workload
	_ = reg.Register(builtin.NewConfigEnforceSkill("1.0.0")) // workload

	bundle := reg.BundleFor("agent-workload", []skill.TwinLevel{skill.TwinLevelWorkload})
	if len(bundle) != 2 {
		t.Errorf("workload agent should receive 2 skills (metrics + config-enforce), got %d", len(bundle))
	}
}

// --- DigitalTwinSkill ---

func TestDigitalTwinSkill_DiffModel(t *testing.T) {
	s := builtin.NewDigitalTwinSkill()
	desired := &skill.TwinModel{TwinID: "twin-1", State: map[string]string{"nginx_version": "1.24"}}
	actual := &skill.TwinModel{TwinID: "twin-1", State: map[string]string{"nginx_version": "1.20"}}

	report := s.DiffModel(desired, actual)
	if !report.HasDrift {
		t.Error("expected drift to be detected")
	}
	if len(report.DriftedFields) != 1 {
		t.Errorf("expected 1 drifted field, got %d", len(report.DriftedFields))
	}
	if report.DriftedFields[0].Field != "nginx_version" {
		t.Errorf("expected 'nginx_version' drift, got %q", report.DriftedFields[0].Field)
	}
}

func TestDigitalTwinSkill_NoDrift(t *testing.T) {
	s := builtin.NewDigitalTwinSkill()
	state := map[string]string{"nginx_version": "1.24"}
	desired := &skill.TwinModel{TwinID: "twin-1", State: state}
	actual := &skill.TwinModel{TwinID: "twin-1", State: map[string]string{"nginx_version": "1.24"}}

	report := s.DiffModel(desired, actual)
	if report.HasDrift {
		t.Error("no drift expected when states are equal")
	}
}

func TestDigitalTwinSkill_UpdateTwin(t *testing.T) {
	s := builtin.NewDigitalTwinSkill()
	s.StoreTwin(&skill.TwinModel{
		TwinID: "twin-1",
		State:  map[string]string{"nginx_version": "1.20"},
	})

	err := s.UpdateTwin("twin-1", &skill.TwinDelta{UpdatedFields: map[string]string{"nginx_version": "1.24"}})
	if err != nil {
		t.Fatalf("UpdateTwin: %v", err)
	}

	m, err := s.ReadTwin("twin-1")
	if err != nil {
		t.Fatalf("ReadTwin: %v", err)
	}
	if m.State["nginx_version"] != "1.24" {
		t.Errorf("expected nginx_version=1.24, got %s", m.State["nginx_version"])
	}
}

func TestDigitalTwinSkill_Execute_ReadTwin(t *testing.T) {
	s := builtin.NewDigitalTwinSkill()
	s.StoreTwin(&skill.TwinModel{TwinID: "twin-1", Version: "v1", State: map[string]string{}})

	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "read-twin",
		TwinID:     "twin-1",
	})
	if err != nil {
		t.Fatalf("Execute read-twin: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got detail: %s", result.Detail)
	}
}

// --- ConfigEnforceSkill ---

func TestConfigEnforceSkill_ApplyConfig(t *testing.T) {
	s := builtin.NewConfigEnforceSkill("1.0.0")
	obs := &skill.Observations{
		DesiredTwins: []*skill.TwinModel{
			{TwinID: "twin-1", Version: "1.0.0", State: map[string]string{"nginx_version": "1.24"}},
		},
	}
	action := &skill.Action{
		ActionType: "apply-config",
		TwinID:     "twin-1",
		Params:     map[string]string{"version": "1.0.0"},
	}
	result, err := s.Execute(context.Background(), obs, action)
	if err != nil {
		t.Fatalf("Execute apply-config: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success: %s", result.Detail)
	}
}

func TestConfigEnforceSkill_UnsupportedVersion(t *testing.T) {
	s := builtin.NewConfigEnforceSkill("1.0.0")
	obs := &skill.Observations{
		DesiredTwins: []*skill.TwinModel{{TwinID: "twin-1", Version: "2.0.0", State: map[string]string{}}},
	}
	action := &skill.Action{
		ActionType: "apply-config",
		TwinID:     "twin-1",
		Params:     map[string]string{"version": "2.0.0"},
	}
	result, err := s.Execute(context.Background(), obs, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for unsupported version")
	}
}

// --- AgentFramework RunOneTick ---

func TestAgentFrameworkRunOneTick_PlansActionOnDrift(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewConfigEnforceSkill("1.0.0"))

	fw := skill.NewAgentFramework("agent-os",
		reg,
		skill.WithInterval(time.Minute), // prevent background ticking
	)

	agent := &mockAgent{
		twins: []*skill.TwinRef{newOSTwinRef("twin-1")},
		actual: map[string]*skill.TwinModel{
			"twin-1": {TwinID: "twin-1", Level: skill.TwinLevelOS, State: map[string]string{"nginx_version": "1.20"}},
		},
		desired: map[string]*skill.TwinModel{
			"twin-1": {TwinID: "twin-1", Level: skill.TwinLevelOS, Version: "1.0.0", State: map[string]string{"nginx_version": "1.24"}},
		},
	}

	actions, err := fw.RunOneTick(context.Background(), agent)
	if err != nil {
		t.Fatalf("RunOneTick: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action when drift is present")
	}
	if actions[0].SkillName != "config-enforce" {
		t.Errorf("expected config-enforce action, got %q", actions[0].SkillName)
	}
}

func TestAgentFrameworkRunOneTick_NoActionWhenNoDrift(t *testing.T) {
	policy := skill.NewAccessPolicy()
	reg := skill.NewSkillRegistry(policy)
	_ = reg.Register(builtin.NewConfigEnforceSkill("1.0.0"))

	fw := skill.NewAgentFramework("agent-os", reg)

	agent := &mockAgent{
		twins: []*skill.TwinRef{newOSTwinRef("twin-1")},
		actual: map[string]*skill.TwinModel{
			"twin-1": {TwinID: "twin-1", State: map[string]string{"nginx_version": "1.24"}},
		},
		desired: map[string]*skill.TwinModel{
			"twin-1": {TwinID: "twin-1", Version: "1.0.0", State: map[string]string{"nginx_version": "1.24"}},
		},
	}

	actions, err := fw.RunOneTick(context.Background(), agent)
	if err != nil {
		t.Fatalf("RunOneTick: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions when states are identical, got %d", len(actions))
	}
}
