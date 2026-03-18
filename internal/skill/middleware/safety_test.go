package middleware_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/middleware"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// stubPROpener records calls and returns a canned pr_url.
type stubPROpener struct {
	called      bool
	returnError bool
}

func (s *stubPROpener) Execute(_ context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	s.called = true
	if s.returnError {
		return nil, errors.New("stub PR opener error")
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"pr_url": "https://github.com/owner/repo/pull/99"},
		},
		Detail: "stub PR opened",
	}, nil
}

func significantExecution(actionType string) middleware.ActionExecution {
	return middleware.ActionExecution{
		Action:     skill.Action{ActionType: actionType, SkillName: "devops-tools", TwinID: "twin-1"},
		Outcome:    middleware.ActionOutcomeSuccess,
		ExecutedAt: time.Now(),
		TwinID:     "twin-1",
	}
}

func failedExecution() middleware.ActionExecution {
	return middleware.ActionExecution{
		Action:     skill.Action{ActionType: "apply-config", SkillName: "config-enforce", TwinID: "twin-1"},
		Outcome:    middleware.ActionOutcomeFailure,
		Error:      errors.New("apply failed"),
		ExecutedAt: time.Now(),
		TwinID:     "twin-1",
	}
}

// ---------------------------------------------------------------------------
// IaCPRSafetyNet tests
// ---------------------------------------------------------------------------

func TestIaCPRSafetyNet_OpensWhenSignificantNoExistingPR(t *testing.T) {
	opener := &stubPROpener{}
	sn := middleware.NewIaCPRSafetyNet(opener, "/tmp/ws")

	executed := []middleware.ActionExecution{
		significantExecution("apply-config"),
	}
	prURL, err := sn.CheckAndEnsurePR(context.Background(), "twin-1", executed)
	if err != nil {
		t.Fatalf("CheckAndEnsurePR: %v", err)
	}
	if prURL == "" {
		t.Error("expected non-empty prURL when safety net fires")
	}
	if !opener.called {
		t.Error("expected PR opener to be called")
	}
}

func TestIaCPRSafetyNet_DoesNotOpenWhenPRAlreadyPresent(t *testing.T) {
	opener := &stubPROpener{}
	sn := middleware.NewIaCPRSafetyNet(opener, "/tmp/ws")

	executed := []middleware.ActionExecution{
		significantExecution("apply-config"),
		{
			Action:     skill.Action{ActionType: "commit-and-open-pr", TwinID: "twin-1"},
			Outcome:    middleware.ActionOutcomeSuccess,
			ExecutedAt: time.Now(),
			TwinID:     "twin-1",
		},
	}
	prURL, err := sn.CheckAndEnsurePR(context.Background(), "twin-1", executed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prURL != "" {
		t.Errorf("expected empty prURL when PR already opened, got %q", prURL)
	}
	if opener.called {
		t.Error("PR opener should NOT be called when a PR already exists")
	}
}

func TestIaCPRSafetyNet_DoesNotOpenWhenNoSignificantActions(t *testing.T) {
	opener := &stubPROpener{}
	sn := middleware.NewIaCPRSafetyNet(opener, "/tmp/ws")

	executed := []middleware.ActionExecution{
		significantExecution("collect-metrics"), // not significant
	}
	prURL, err := sn.CheckAndEnsurePR(context.Background(), "twin-1", executed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prURL != "" {
		t.Errorf("expected empty prURL for non-significant actions, got %q", prURL)
	}
	if opener.called {
		t.Error("PR opener should NOT be called for non-significant actions")
	}
}

func TestIaCPRSafetyNet_RequiresApprovalTriggersNet(t *testing.T) {
	opener := &stubPROpener{}
	sn := middleware.NewIaCPRSafetyNet(opener, "/tmp/ws")

	executed := []middleware.ActionExecution{
		{
			Action: skill.Action{
				ActionType:       "custom-action",
				TwinID:           "twin-1",
				RequiresApproval: true,
			},
			Outcome:    middleware.ActionOutcomeSuccess,
			ExecutedAt: time.Now(),
			TwinID:     "twin-1",
		},
	}
	prURL, err := sn.CheckAndEnsurePR(context.Background(), "twin-1", executed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prURL == "" {
		t.Error("expected PR to be opened for RequiresApproval action")
	}
}

func TestIaCPRSafetyNet_EmptyExecutions_NoOp(t *testing.T) {
	opener := &stubPROpener{}
	sn := middleware.NewIaCPRSafetyNet(opener, "/tmp/ws")
	prURL, err := sn.CheckAndEnsurePR(context.Background(), "twin-1", nil)
	if err != nil || prURL != "" || opener.called {
		t.Errorf("expected no-op for empty executions: prURL=%q err=%v called=%v", prURL, err, opener.called)
	}
}

func TestIaCPRSafetyNet_OpenerError_Propagated(t *testing.T) {
	opener := &stubPROpener{returnError: true}
	sn := middleware.NewIaCPRSafetyNet(opener, "/tmp/ws")
	_, err := sn.CheckAndEnsurePR(context.Background(), "twin-1", []middleware.ActionExecution{
		significantExecution("apply-config"),
	})
	if err == nil {
		t.Error("expected error when PR opener returns an error")
	}
}

// ---------------------------------------------------------------------------
// ActionRollbackMiddleware tests
// ---------------------------------------------------------------------------

func TestActionRollbackMiddleware_CallsRollbackOnFailure(t *testing.T) {
	rollbackCalled := false
	m := middleware.NewActionRollbackMiddleware(func(_ context.Context, _ middleware.ActionExecution) error {
		rollbackCalled = true
		return nil
	})

	record := m.OnActionFailed(context.Background(), failedExecution())
	if !rollbackCalled {
		t.Error("expected rollback function to be called")
	}
	if record.Outcome != middleware.ActionOutcomeRolledBack {
		t.Errorf("expected ActionOutcomeRolledBack, got %s", record.Outcome)
	}
	if m.RollbackAttempts != 1 {
		t.Errorf("expected RollbackAttempts=1, got %d", m.RollbackAttempts)
	}
	if m.RollbackSuccesses != 1 {
		t.Errorf("expected RollbackSuccesses=1, got %d", m.RollbackSuccesses)
	}
}

func TestActionRollbackMiddleware_RollbackFailure(t *testing.T) {
	m := middleware.NewActionRollbackMiddleware(func(_ context.Context, _ middleware.ActionExecution) error {
		return errors.New("rollback also failed")
	})

	record := m.OnActionFailed(context.Background(), failedExecution())
	if record.Outcome != middleware.ActionOutcomeFailure {
		t.Errorf("expected ActionOutcomeFailure when rollback fails, got %s", record.Outcome)
	}
	if m.RollbackSuccesses != 0 {
		t.Errorf("expected RollbackSuccesses=0, got %d", m.RollbackSuccesses)
	}
}

func TestActionRollbackMiddleware_NilRollbackFn_NoOp(t *testing.T) {
	m := middleware.NewActionRollbackMiddleware(nil)
	record := m.OnActionFailed(context.Background(), failedExecution())
	// Nil rollback = noop rollback which succeeds.
	if record.Outcome != middleware.ActionOutcomeRolledBack {
		t.Errorf("expected ActionOutcomeRolledBack for noop rollback, got %s", record.Outcome)
	}
}

// ---------------------------------------------------------------------------
// WrapExecute tests
// ---------------------------------------------------------------------------

// successSkill always returns Success=true.
type successSkill struct{}

func (s *successSkill) Name() string                  { return "success-skill" }
func (s *successSkill) Version() string               { return "1.0.0" }
func (s *successSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }
func (s *successSkill) Execute(_ context.Context, _ *skill.Observations, _ *skill.Action) (*skill.SkillResult, error) {
	return &skill.SkillResult{Success: true, Detail: "ok"}, nil
}

// failSkill always returns an error.
type failSkill struct{}

func (s *failSkill) Name() string                  { return "fail-skill" }
func (s *failSkill) Version() string               { return "1.0.0" }
func (s *failSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }
func (s *failSkill) Execute(_ context.Context, _ *skill.Observations, _ *skill.Action) (*skill.SkillResult, error) {
	return nil, errors.New("deliberate failure")
}

func TestActionRollbackMiddleware_WrapExecute_Success(t *testing.T) {
	m := middleware.NewActionRollbackMiddleware(nil)
	result, exec := m.WrapExecute(context.Background(), &successSkill{}, &skill.Observations{}, &skill.Action{
		ActionType: "apply-config",
		TwinID:     "twin-1",
	})
	if result == nil || !result.Success {
		t.Errorf("expected success result, got %v", result)
	}
	if exec.Outcome != middleware.ActionOutcomeSuccess {
		t.Errorf("expected ActionOutcomeSuccess, got %s", exec.Outcome)
	}
}

func TestActionRollbackMiddleware_WrapExecute_Failure(t *testing.T) {
	rollbackCalled := false
	m := middleware.NewActionRollbackMiddleware(func(_ context.Context, _ middleware.ActionExecution) error {
		rollbackCalled = true
		return nil
	})

	_, exec := m.WrapExecute(context.Background(), &failSkill{}, &skill.Observations{}, &skill.Action{
		ActionType: "apply-config",
		TwinID:     "twin-1",
	})
	if !rollbackCalled {
		t.Error("expected rollback to be called after skill failure")
	}
	if exec.Outcome != middleware.ActionOutcomeRolledBack {
		t.Errorf("expected ActionOutcomeRolledBack, got %s", exec.Outcome)
	}
}
