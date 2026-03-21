package gitops_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/gitops"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// noopSkill uses the NoopGitOpsBackend (always available, no side effects).
func noopSkill() *gitops.GitOpsSkill {
	return gitops.NewGitOpsSkill(&gitops.NoopGitOpsBackend{})
}

func obsFor(twinID string) *skill.Observations {
	return &skill.Observations{
		DesiredTwins: []*skill.TwinModel{
			{
				TwinID:  twinID,
				Version: "v1",
				State:   map[string]string{"nginx_version": "1.24", "env": "prod"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func TestGitOpsSkill_Metadata(t *testing.T) {
	s := noopSkill()
	if s.Name() != "gitops" {
		t.Errorf("Name() = %q, want gitops", s.Name())
	}
	if s.Version() == "" {
		t.Error("Version() should not be empty")
	}
	if s.MinTwinLevel() != skill.TwinLevelOS {
		t.Errorf("MinTwinLevel() = %v, want TwinLevelOS", s.MinTwinLevel())
	}
}

func TestGitOpsSkill_Available(t *testing.T) {
	s := noopSkill()
	if !s.IsAvailable() {
		t.Error("NoopGitOpsBackend should always be available")
	}
}

func TestGitOpsSkill_NilBackendFallsToNoop(t *testing.T) {
	s := gitops.NewGitOpsSkill(nil)
	if !s.IsAvailable() {
		t.Error("nil backend should fall back to noop which is always available")
	}
}

// ---------------------------------------------------------------------------
// clone-repo
// ---------------------------------------------------------------------------

func TestGitOpsSkill_CloneRepo(t *testing.T) {
	s := noopSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "clone-repo",
		TwinID:     "twin-1",
		Params:     map[string]string{"repo_url": "https://github.com/example/iac"},
	})
	if err != nil || !result.Success {
		t.Fatalf("clone-repo: err=%v result=%v", err, result)
	}
	if result.StateDelta == nil || result.StateDelta.UpdatedFields["workspace_path"] == "" {
		t.Error("expected workspace_path in StateDelta")
	}
}

func TestGitOpsSkill_CloneRepo_MissingURL(t *testing.T) {
	s := noopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "clone-repo",
		TwinID:     "twin-1",
		Params:     map[string]string{},
	})
	if err == nil {
		t.Error("expected error when repo_url missing")
	}
}

// ---------------------------------------------------------------------------
// apply-twin-model
// ---------------------------------------------------------------------------

func TestGitOpsSkill_ApplyTwinModel(t *testing.T) {
	s := noopSkill()
	obs := obsFor("twin-1")
	ws := t.TempDir()

	result, err := s.Execute(context.Background(), obs, &skill.Action{
		ActionType: "apply-twin-model",
		TwinID:     "twin-1",
		Params:     map[string]string{"workspace_path": ws},
	})
	if err != nil || !result.Success {
		t.Fatalf("apply-twin-model: err=%v result=%v", err, result)
	}
	if len(result.AppliedFields) == 0 {
		t.Error("expected AppliedFields to be populated")
	}
}

func TestGitOpsSkill_ApplyTwinModel_MissingWorkspace(t *testing.T) {
	s := noopSkill()
	_, err := s.Execute(context.Background(), obsFor("twin-1"), &skill.Action{
		ActionType: "apply-twin-model",
		TwinID:     "twin-1",
		Params:     map[string]string{},
	})
	if err == nil {
		t.Error("expected error when workspace_path missing")
	}
}

func TestGitOpsSkill_ApplyTwinModel_MissingDesiredState(t *testing.T) {
	s := noopSkill()
	ws := t.TempDir()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "apply-twin-model",
		TwinID:     "twin-1",
		Params:     map[string]string{"workspace_path": ws},
	})
	if err == nil {
		t.Error("expected error when desired twin state not in observations")
	}
}

// ---------------------------------------------------------------------------
// commit-and-open-pr
// ---------------------------------------------------------------------------

func TestGitOpsSkill_CommitAndOpenPR(t *testing.T) {
	s := noopSkill()
	ws := t.TempDir()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "commit-and-open-pr",
		TwinID:     "twin-1",
		Params: map[string]string{
			"workspace_path": ws,
			"description":    "Update nginx to 1.24 for twin-1",
		},
	})
	if err != nil || !result.Success {
		t.Fatalf("commit-and-open-pr: err=%v result=%v", err, result)
	}
	if result.StateDelta == nil || result.StateDelta.UpdatedFields["pr_url"] == "" {
		t.Error("expected pr_url in StateDelta")
	}
}

func TestGitOpsSkill_CommitAndOpenPR_MissingWorkspace(t *testing.T) {
	s := noopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "commit-and-open-pr",
		TwinID:     "twin-1",
		Params:     map[string]string{},
	})
	if err == nil {
		t.Error("expected error when workspace_path missing")
	}
}

func TestGitOpsSkill_CommitAndOpenPR_DefaultDescription(t *testing.T) {
	s := noopSkill()
	ws := t.TempDir()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "commit-and-open-pr",
		TwinID:     "twin-1",
		Params:     map[string]string{"workspace_path": ws},
	})
	if err != nil || !result.Success {
		t.Fatalf("commit-and-open-pr with default description: err=%v result=%v", err, result)
	}
}

// ---------------------------------------------------------------------------
// check-pr-status
// ---------------------------------------------------------------------------

func TestGitOpsSkill_CheckPRStatus(t *testing.T) {
	s := noopSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "check-pr-status",
		TwinID:     "twin-1",
		Params:     map[string]string{"pr_url": "https://github.com/owner/repo/pull/42"},
	})
	if err != nil || !result.Success {
		t.Fatalf("check-pr-status: err=%v result=%v", err, result)
	}
}

func TestGitOpsSkill_CheckPRStatus_MissingURL(t *testing.T) {
	s := noopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "check-pr-status",
		TwinID:     "twin-1",
		Params:     map[string]string{},
	})
	if err == nil {
		t.Error("expected error when pr_url missing")
	}
}

// ---------------------------------------------------------------------------
// UnknownActionType
// ---------------------------------------------------------------------------

func TestGitOpsSkill_UnknownAction(t *testing.T) {
	s := noopSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, &skill.Action{
		ActionType: "teleport",
		TwinID:     "twin-1",
	})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

// ---------------------------------------------------------------------------
// GitBackend with mock HTTP server
// ---------------------------------------------------------------------------

func TestGitBackend_Available_NoToken(t *testing.T) {
	b := gitops.NewGitBackend("https://api.github.com", "", "owner", "repo")
	if b.Available() {
		t.Error("GitBackend without token should not be available")
	}
}

func TestGitBackend_CheckPRStatus_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mergeable := true
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title":     "Pheromone: IaC update",
			"state":     "open",
			"mergeable": mergeable,
			"html_url":  "https://github.com/owner/repo/pull/42",
		})
	}))
	defer srv.Close()

	// Override backend's http client to use the test server.
	b := gitops.NewGitBackend(srv.URL, "test-token", "owner", "repo")
	b.SetHTTPClient(srv.Client())

	status, err := b.CheckPRStatus(context.Background(), "https://github.com/owner/repo/pull/42")
	if err != nil {
		t.Fatalf("CheckPRStatus: %v", err)
	}
	if status.State != "open" {
		t.Errorf("expected state=open, got %s", status.State)
	}
	if !status.Mergeable {
		t.Error("expected mergeable=true")
	}
}
