package subagent_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/subagent"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newSkill() *subagent.SubagentSkill { return subagent.NewSubagentSkill() }

// noop task returns immediately with no actions and no error.
func noopTask(_ context.Context) ([]string, error) { return nil, nil }

// slowTask blocks until ctx is cancelled.
func slowTask(ctx context.Context) ([]string, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func TestSubagentSkill_Metadata(t *testing.T) {
	s := newSkill()
	if s.Name() != "subagent" {
		t.Errorf("Name() = %q, want subagent", s.Name())
	}
	if s.Version() == "" {
		t.Error("Version() should not be empty")
	}
	if s.MinTwinLevel() != skill.TwinLevelOS {
		t.Errorf("MinTwinLevel() = %v, want TwinLevelOS", s.MinTwinLevel())
	}
}

// ---------------------------------------------------------------------------
// Spawn + Wait
// ---------------------------------------------------------------------------

func TestSubagentSkill_SpawnAndWait(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	id, err := s.SpawnSubagent(ctx, "test task", []string{"twin-1"}, noopTask)
	if err != nil {
		t.Fatalf("SpawnSubagent: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty subagent ID")
	}

	result, err := s.WaitForSubagent(ctx, id)
	if err != nil {
		t.Fatalf("WaitForSubagent: %v", err)
	}
	if result.Status != subagent.SubagentStatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
}

func TestSubagentSkill_WaitForUnknownID(t *testing.T) {
	s := newSkill()
	_, err := s.WaitForSubagent(context.Background(), "ghost")
	if err == nil {
		t.Error("expected error for unknown subagent ID")
	}
}

// ---------------------------------------------------------------------------
// Cancel
// ---------------------------------------------------------------------------

func TestSubagentSkill_Cancel(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	id, err := s.SpawnSubagent(ctx, "slow task", []string{"twin-1"}, slowTask)
	if err != nil {
		t.Fatalf("SpawnSubagent: %v", err)
	}

	if err := s.CancelSubagent(id); err != nil {
		t.Fatalf("CancelSubagent: %v", err)
	}

	result, err := s.WaitForSubagent(ctx, id)
	if err != nil {
		t.Fatalf("WaitForSubagent after cancel: %v", err)
	}
	if result.Status != subagent.SubagentStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.Status)
	}
}

func TestSubagentSkill_CancelUnknown(t *testing.T) {
	s := newSkill()
	if err := s.CancelSubagent("ghost"); err == nil {
		t.Error("expected error cancelling unknown subagent")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestSubagentSkill_List(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	_, _ = s.SpawnSubagent(ctx, "task-a", []string{"twin-1"}, noopTask)
	_, _ = s.SpawnSubagent(ctx, "task-b", []string{"twin-2"}, noopTask)

	list := s.ListActiveSubagents()
	if len(list) < 2 {
		t.Errorf("expected at least 2 subagents listed, got %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// Parallel subagents
// ---------------------------------------------------------------------------

func TestSubagentSkill_Parallel(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	const n = 5
	ids := make([]string, n)
	var counter atomic.Int32

	for i := range n {
		taskFn := func(_ context.Context) ([]string, error) {
			counter.Add(1)
			return []string{"apply-config"}, nil
		}
		id, err := s.SpawnSubagent(ctx, "parallel task", []string{"twin-1"}, taskFn)
		if err != nil {
			t.Fatalf("SpawnSubagent %d: %v", i, err)
		}
		ids[i] = id
	}

	for _, id := range ids {
		result, err := s.WaitForSubagent(ctx, id)
		if err != nil {
			t.Fatalf("WaitForSubagent %s: %v", id, err)
		}
		if result.Status != subagent.SubagentStatusCompleted {
			t.Errorf("subagent %s: expected completed, got %s", id, result.Status)
		}
	}

	if int(counter.Load()) != n {
		t.Errorf("expected %d tasks to execute, got %d", n, counter.Load())
	}
}

// ---------------------------------------------------------------------------
// Failed subagent
// ---------------------------------------------------------------------------

func TestSubagentSkill_TaskError(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	errTask := func(_ context.Context) ([]string, error) {
		return nil, errors.New("deliberate failure")
	}
	id, _ := s.SpawnSubagent(ctx, "failing task", []string{"twin-1"}, errTask)
	result, err := s.WaitForSubagent(ctx, id)
	if err != nil {
		t.Fatalf("WaitForSubagent: %v", err)
	}
	if result.Status != subagent.SubagentStatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty Error in result")
	}
}

// ---------------------------------------------------------------------------
// Panic recovery
// ---------------------------------------------------------------------------

func TestSubagentSkill_PanicRecovery(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	panicTask := func(_ context.Context) ([]string, error) {
		panic("deliberate panic for testing")
	}
	id, err := s.SpawnSubagent(ctx, "panicking task", []string{"twin-1"}, panicTask)
	if err != nil {
		t.Fatalf("SpawnSubagent: %v", err)
	}
	result, err := s.WaitForSubagent(ctx, id)
	if err != nil {
		t.Fatalf("WaitForSubagent: %v", err)
	}
	if result.Status != subagent.SubagentStatusFailed {
		t.Errorf("expected failed status after panic, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty Error after panic")
	}
}

// ---------------------------------------------------------------------------
// Orphan reaper
// ---------------------------------------------------------------------------

func TestSubagentSkill_ReapOrphans(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	// Spawn several slow tasks.
	for range 3 {
		_, _ = s.SpawnSubagent(ctx, "slow", []string{"twin-1"}, slowTask)
	}

	reaped := s.ReapOrphans()
	if reaped < 1 {
		t.Errorf("expected at least 1 orphan reaped, got %d", reaped)
	}
}

// ---------------------------------------------------------------------------
// Execute dispatch
// ---------------------------------------------------------------------------

func TestSubagentSkill_Execute_Spawn(t *testing.T) {
	s := newSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "spawn",
		TwinID:     "twin-1",
		Params:     map[string]string{"task_spec": "deploy v2"},
	})
	if err != nil || !result.Success {
		t.Fatalf("execute spawn: err=%v result=%v", err, result)
	}
	if result.StateDelta.UpdatedFields["subagent_id"] == "" {
		t.Error("expected subagent_id in StateDelta")
	}
}

func TestSubagentSkill_Execute_Spawn_MultiTwinIDs(t *testing.T) {
	s := newSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "spawn",
		TwinID:     "twin-1",
		Params: map[string]string{
			"task_spec": "multi-twin task",
			"twin_ids":  "twin-1, twin-2, twin-3",
		},
	})
	if err != nil || !result.Success {
		t.Fatalf("execute spawn with twin_ids: err=%v result=%v", err, result)
	}
	// Detail should mention 3 twins.
	if !strings.Contains(result.Detail, "3 twin(s)") {
		t.Errorf("expected detail to mention 3 twins, got %q", result.Detail)
	}
}

func TestSubagentSkill_Execute_Wait(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	// Spawn via direct API.
	id, _ := s.SpawnSubagent(ctx, "test", []string{"twin-1"}, noopTask)

	// Wait via Execute.
	result, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "wait",
		TwinID:     "twin-1",
		Params:     map[string]string{"subagent_id": id},
	})
	if err != nil || !result.Success {
		t.Fatalf("execute wait: err=%v result=%v", err, result)
	}
}

func TestSubagentSkill_Execute_Cancel(t *testing.T) {
	s := newSkill()
	ctx := context.Background()

	id, _ := s.SpawnSubagent(ctx, "slow", []string{"twin-1"}, slowTask)

	result, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "cancel",
		TwinID:     "twin-1",
		Params:     map[string]string{"subagent_id": id},
	})
	if err != nil || !result.Success {
		t.Fatalf("execute cancel: err=%v result=%v", err, result)
	}
}

func TestSubagentSkill_Execute_List(t *testing.T) {
	s := newSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "list",
		TwinID:     "twin-1",
	})
	if err != nil || !result.Success {
		t.Fatalf("execute list: err=%v result=%v", err, result)
	}
}

func TestSubagentSkill_Execute_UnknownAction(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "teleport",
		TwinID:     "twin-1",
	})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestSubagentSkill_Execute_WaitMissingID(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "wait",
		TwinID:     "twin-1",
		Params:     map[string]string{},
	})
	if err == nil {
		t.Error("expected error when subagent_id missing")
	}
}

func TestSubagentSkill_WaitContextCancelled(t *testing.T) {
	s := newSkill()
	parentCtx := context.Background()

	id, _ := s.SpawnSubagent(parentCtx, "slow", []string{"twin-1"}, slowTask)

	// Wait with a very short deadline.
	waitCtx, cancel := context.WithTimeout(parentCtx, 10*time.Millisecond)
	defer cancel()

	_, err := s.WaitForSubagent(waitCtx, id)
	if err == nil {
		t.Error("expected error when wait context is cancelled")
	}
}
