package sandbox_test

import (
	"context"
	"strings"
	"testing"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/sandbox"
)

// ---------------------------------------------------------------------------
// NoopSandboxBackend tests
// ---------------------------------------------------------------------------

func newNoopSkill() *sandbox.SandboxedExecutionSkill {
	return sandbox.NewSandboxedExecutionSkill(&sandbox.NoopSandboxBackend{})
}

func TestSandboxedExecutionSkill_Metadata(t *testing.T) {
	s := newNoopSkill()
	if s.Name() != "sandboxed-execution" {
		t.Errorf("Name() = %q, want %q", s.Name(), "sandboxed-execution")
	}
	if s.Version() == "" {
		t.Error("Version() should not be empty")
	}
	if s.MinTwinLevel() != skill.TwinLevelOS {
		t.Errorf("MinTwinLevel() = %v, want TwinLevelOS", s.MinTwinLevel())
	}
}

func TestSandboxedExecutionSkill_Available(t *testing.T) {
	s := newNoopSkill()
	if !s.IsAvailable() {
		t.Error("NoopSandboxBackend should always report Available=true")
	}
}

func TestSandboxedExecutionSkill_CreateSandbox(t *testing.T) {
	s := newNoopSkill()
	ctx := context.Background()

	result, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "create-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"twin_profile": "ubuntu-24.04"},
	})
	if err != nil {
		t.Fatalf("create-sandbox: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
	if result.StateDelta == nil || result.StateDelta.UpdatedFields["sandbox_id"] == "" {
		t.Error("expected sandbox_id in StateDelta.UpdatedFields")
	}
}

func TestSandboxedExecutionSkill_ExecInSandbox(t *testing.T) {
	s := newNoopSkill()
	ctx := context.Background()

	// First create a sandbox.
	create, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "create-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"twin_profile": "ubuntu-24.04"},
	})
	if err != nil {
		t.Fatalf("create-sandbox: %v", err)
	}
	sandboxID := create.StateDelta.UpdatedFields["sandbox_id"]

	// Execute a command inside the sandbox.
	result, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "exec-in-sandbox",
		TwinID:     "twin-1",
		Params: map[string]string{
			"sandbox_id": sandboxID,
			"command":    "echo hello",
		},
	})
	if err != nil {
		t.Fatalf("exec-in-sandbox: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
}

func TestSandboxedExecutionSkill_ExecMissingSandboxID(t *testing.T) {
	s := newNoopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "exec-in-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"command": "echo test"},
	})
	if err == nil {
		t.Error("expected error when sandbox_id missing")
	}
}

func TestSandboxedExecutionSkill_ExecMissingCommand(t *testing.T) {
	s := newNoopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "exec-in-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"sandbox_id": "some-id"},
	})
	if err == nil {
		t.Error("expected error when command missing")
	}
}

func TestSandboxedExecutionSkill_CommitSandbox(t *testing.T) {
	s := newNoopSkill()
	ctx := context.Background()

	create, _ := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "create-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"twin_profile": "debian-12"},
	})
	sandboxID := create.StateDelta.UpdatedFields["sandbox_id"]

	result, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "commit-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"sandbox_id": sandboxID},
	})
	if err != nil {
		t.Fatalf("commit-sandbox: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
}

func TestSandboxedExecutionSkill_DiscardSandbox(t *testing.T) {
	s := newNoopSkill()
	ctx := context.Background()

	create, _ := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "create-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"twin_profile": "alpine-3"},
	})
	sandboxID := create.StateDelta.UpdatedFields["sandbox_id"]

	result, err := s.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "discard-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"sandbox_id": sandboxID},
	})
	if err != nil {
		t.Fatalf("discard-sandbox: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
}

func TestSandboxedExecutionSkill_UnknownActionType(t *testing.T) {
	s := newNoopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "explode",
		TwinID:     "twin-1",
	})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestSandboxedExecutionSkill_DiscardUnknownSandbox(t *testing.T) {
	s := newNoopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "discard-sandbox",
		TwinID:     "twin-1",
		Params:     map[string]string{"sandbox_id": "ghost"},
	})
	if err == nil {
		t.Error("expected error when discarding unknown sandbox")
	}
}

func TestSandboxedExecutionSkill_NilBackendFallsBackToNoop(t *testing.T) {
	s := sandbox.NewSandboxedExecutionSkill(nil)
	if !s.IsAvailable() {
		t.Error("nil backend should fall back to noop which is always available")
	}
}

func TestSandboxedExecutionSkill_CreateUsesDefaultProfile(t *testing.T) {
	s := newNoopSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "create-sandbox",
		TwinID:     "twin-from-id",
		Params:     map[string]string{}, // no twin_profile param
	})
	if err != nil {
		t.Fatalf("create-sandbox without twin_profile: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
	if !strings.Contains(result.Detail, "twin-from-id") {
		t.Errorf("expected twin_id in detail, got %q", result.Detail)
	}
}
