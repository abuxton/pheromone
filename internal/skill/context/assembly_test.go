package context_test

import (
	"context"
	"testing"
	"time"

	scontext "github.com/abuxton/pheromone/internal/skill/context"

	"github.com/abuxton/pheromone/internal/skill"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newAssembler() *scontext.ContextAssemblySkill {
	return scontext.NewContextAssemblySkill(nil, 500*time.Millisecond)
}

func obsFor(twinID string) *skill.Observations {
	return &skill.Observations{
		DesiredTwins: []*skill.TwinModel{
			{TwinID: twinID, State: map[string]string{"nginx_version": "1.24"}},
		},
		Twins: []*skill.TwinModel{
			{TwinID: twinID, State: map[string]string{"nginx_version": "1.20"}},
		},
	}
}

// ---------------------------------------------------------------------------
// ContextAssemblySkill metadata
// ---------------------------------------------------------------------------

func TestContextAssemblySkill_Metadata(t *testing.T) {
	s := newAssembler()
	if s.Name() != "context-assembly" {
		t.Errorf("Name() = %q, want context-assembly", s.Name())
	}
	if s.Version() == "" {
		t.Error("Version() should not be empty")
	}
	if s.MinTwinLevel() != skill.TwinLevelOS {
		t.Errorf("MinTwinLevel() = %v, want TwinLevelOS", s.MinTwinLevel())
	}
}

// ---------------------------------------------------------------------------
// Assemble happy path
// ---------------------------------------------------------------------------

func TestContextAssemblySkill_Assemble_Success(t *testing.T) {
	s := newAssembler()
	obs := obsFor("twin-1")

	result, err := s.Execute(context.Background(), obs, &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-1",
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
}

func TestContextAssemblySkill_Assemble_BundleStored(t *testing.T) {
	s := newAssembler()
	obs := obsFor("twin-1")

	_, err := s.Execute(context.Background(), obs, &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-1",
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	bundle := s.GetBundle("twin-1")
	if bundle == nil {
		t.Fatal("expected bundle to be stored after assembly")
	}
	if bundle.TwinID != "twin-1" {
		t.Errorf("bundle.TwinID = %q, want twin-1", bundle.TwinID)
	}
	if bundle.TwinDesiredState == nil {
		t.Error("expected TwinDesiredState to be populated")
	}
	if bundle.TwinActualState == nil {
		t.Error("expected TwinActualState to be populated")
	}
}

func TestContextAssemblySkill_Assemble_DriftDetected(t *testing.T) {
	s := newAssembler()
	_, _ = s.Execute(context.Background(), obsFor("twin-1"), &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-1",
	})

	bundle := s.GetBundle("twin-1")
	if bundle.DriftReport == nil {
		t.Fatal("expected DriftReport to be computed")
	}
	if !bundle.DriftReport.HasDrift {
		t.Error("expected drift to be detected (1.20 vs 1.24)")
	}
}

func TestContextAssemblySkill_Assemble_NoDrift(t *testing.T) {
	s := newAssembler()
	obs := &skill.Observations{
		DesiredTwins: []*skill.TwinModel{
			{TwinID: "twin-2", State: map[string]string{"nginx_version": "1.24"}},
		},
		Twins: []*skill.TwinModel{
			{TwinID: "twin-2", State: map[string]string{"nginx_version": "1.24"}},
		},
	}
	_, _ = s.Execute(context.Background(), obs, &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-2",
	})

	bundle := s.GetBundle("twin-2")
	if bundle.DriftReport.HasDrift {
		t.Error("expected no drift when states are equal")
	}
}

func TestContextAssemblySkill_Assemble_UnknownAction(t *testing.T) {
	s := newAssembler()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "explode",
		TwinID:     "twin-1",
	})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestContextAssemblySkill_Assemble_EmptyTwinID(t *testing.T) {
	s := newAssembler()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "assemble",
		TwinID:     "",
	})
	if err == nil {
		t.Error("expected error for empty TwinID")
	}
}

func TestContextAssemblySkill_GetBundleNilForUnknownTwin(t *testing.T) {
	s := newAssembler()
	if b := s.GetBundle("ghost"); b != nil {
		t.Error("expected nil bundle for unknown twin")
	}
}

func TestContextBundle_IsComplete(t *testing.T) {
	b := &scontext.ContextBundle{}
	if b.IsComplete() {
		t.Error("empty bundle should not be complete")
	}
	b.TwinDesiredState = &skill.TwinModel{TwinID: "t"}
	if b.IsComplete() {
		t.Error("bundle with only desired state should not be complete")
	}
	b.TwinActualState = &skill.TwinModel{TwinID: "t"}
	if !b.IsComplete() {
		t.Error("bundle with both states should be complete")
	}
}

// ---------------------------------------------------------------------------
// AgentsMD injection
// ---------------------------------------------------------------------------

type stubAgentsMD struct{ content string }

func (s *stubAgentsMD) ReadAgentsMD() (string, error) { return s.content, nil }

func TestContextAssemblySkill_Assemble_AgentsMDInjected(t *testing.T) {
	s := scontext.NewContextAssemblySkill(&stubAgentsMD{content: "# AGENTS.md stub"}, 500*time.Millisecond)
	_, _ = s.Execute(context.Background(), obsFor("twin-1"), &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-1",
	})
	bundle := s.GetBundle("twin-1")
	if bundle.AgentsMDContent != "# AGENTS.md stub" {
		t.Errorf("expected AGENTS.md content injected, got %q", bundle.AgentsMDContent)
	}
}

// ---------------------------------------------------------------------------
// InMemoryMessageQueue
// ---------------------------------------------------------------------------

func TestInMemoryMessageQueue_PushPoll(t *testing.T) {
	q := &scontext.InMemoryMessageQueue{}
	q.Push("msg-1")
	q.Push("msg-2")
	msgs := q.Poll()
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	if len(q.Poll()) != 0 {
		t.Error("second poll should return empty slice after drain")
	}
}

// ---------------------------------------------------------------------------
// MessageQueueInjector
// ---------------------------------------------------------------------------

func TestMessageQueueInjector_Inject(t *testing.T) {
	s := newAssembler()
	// Assemble a bundle first.
	_, _ = s.Execute(context.Background(), obsFor("twin-1"), &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-1",
	})

	q := &scontext.InMemoryMessageQueue{}
	q.Push("operator: please also update node version")
	q.Push("operator: check disk space after update")

	injector := scontext.NewMessageQueueInjector(q, s)
	n := injector.Inject("twin-1")
	if n != 2 {
		t.Errorf("expected 2 messages injected, got %d", n)
	}

	bundle := s.GetBundle("twin-1")
	if len(bundle.FollowUpMessages) != 2 {
		t.Errorf("expected 2 FollowUpMessages in bundle, got %d", len(bundle.FollowUpMessages))
	}
}

func TestMessageQueueInjector_NoOpWhenNoBundleExists(t *testing.T) {
	s := newAssembler()
	q := &scontext.InMemoryMessageQueue{}
	q.Push("msg")
	injector := scontext.NewMessageQueueInjector(q, s)
	n := injector.Inject("ghost-twin")
	if n != 0 {
		t.Errorf("expected 0 injected for unknown twin, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// PrePlanContextCheck
// ---------------------------------------------------------------------------

func TestPrePlanContextCheck_RefreshesStaleBundle(t *testing.T) {
	s := newAssembler()
	check := scontext.NewPrePlanContextCheck(s, 30*time.Second)
	obs := obsFor("twin-1")

	// No bundle exists yet; Check should trigger assembly.
	bundle, err := check.Check(context.Background(), "twin-1", obs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if bundle == nil {
		t.Fatal("expected non-nil bundle after Check")
	}
}

func TestPrePlanContextCheck_ReusesRecentBundle(t *testing.T) {
	s := newAssembler()
	obs := obsFor("twin-1")

	// Pre-assemble.
	_, _ = s.Execute(context.Background(), obs, &skill.Action{
		ActionType: "assemble",
		TwinID:     "twin-1",
	})
	v1 := s.GetBundle("twin-1").Version

	check := scontext.NewPrePlanContextCheck(s, 30*time.Second)
	bundle, _ := check.Check(context.Background(), "twin-1", obs)

	if bundle.Version != v1 {
		t.Errorf("expected same bundle version (no reassembly needed), got v%d vs v%d", bundle.Version, v1)
	}
}
