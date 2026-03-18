package devops_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuxton/pheromone/internal/skill"
	"github.com/abuxton/pheromone/internal/skill/devops"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newSkill() *devops.DevOpsToolSkill {
	return devops.NewDevOpsToolSkill()
}

func execAction(actionType string, params map[string]string) *skill.Action {
	return &skill.Action{
		ActionType: actionType,
		TwinID:     "twin-1",
		Params:     params,
	}
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func TestDevOpsToolSkill_Metadata(t *testing.T) {
	s := newSkill()
	if s.Name() != "devops-tools" {
		t.Errorf("Name() = %q, want devops-tools", s.Name())
	}
	if s.Version() == "" {
		t.Error("Version() should not be empty")
	}
	if s.MinTwinLevel() != skill.TwinLevelOS {
		t.Errorf("MinTwinLevel() = %v, want TwinLevelOS", s.MinTwinLevel())
	}
}

// ---------------------------------------------------------------------------
// ShellExec
// ---------------------------------------------------------------------------

func TestDevOpsToolSkill_ShellExec_Success(t *testing.T) {
	s := newSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, execAction("shell-exec", map[string]string{
		"command": "echo hello",
	}))
	if err != nil {
		t.Fatalf("shell-exec: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success; detail: %s", result.Detail)
	}
	if !strings.Contains(result.Detail, "hello") {
		t.Errorf("expected 'hello' in detail, got %q", result.Detail)
	}
}

func TestDevOpsToolSkill_ShellExec_MissingCommand(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, execAction("shell-exec", map[string]string{}))
	if err == nil {
		t.Error("expected error when command param missing")
	}
}

func TestDevOpsToolSkill_ShellExec_NonZeroExit(t *testing.T) {
	s := newSkill()
	result, err := s.Execute(context.Background(), &skill.Observations{}, execAction("shell-exec", map[string]string{
		"command": "exit 1",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.Success {
		t.Error("expected failure result for exit 1")
	}
}

// ---------------------------------------------------------------------------
// FileRead / FileWrite / FileEdit
// ---------------------------------------------------------------------------

func TestDevOpsToolSkill_FileWrite_And_Read(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	s := newSkill()

	// Write.
	wRes, err := s.Execute(context.Background(), &skill.Observations{}, execAction("file-write", map[string]string{
		"path":    path,
		"content": "hello world",
	}))
	if err != nil || !wRes.Success {
		t.Fatalf("file-write: err=%v result=%v", err, wRes)
	}

	// Read.
	rRes, err := s.Execute(context.Background(), &skill.Observations{}, execAction("file-read", map[string]string{
		"path": path,
	}))
	if err != nil || !rRes.Success {
		t.Fatalf("file-read: err=%v result=%v", err, rRes)
	}
	if rRes.StateDelta.UpdatedFields["file_content"] != "hello world" {
		t.Errorf("unexpected content: %q", rRes.StateDelta.UpdatedFields["file_content"])
	}
}

func TestDevOpsToolSkill_FileRead_NotFound(t *testing.T) {
	s := newSkill()
	res, err := s.Execute(context.Background(), &skill.Observations{}, execAction("file-read", map[string]string{
		"path": "/tmp/pheromone-does-not-exist-xyz",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if res.Success {
		t.Error("expected failure for non-existent file")
	}
}

func TestDevOpsToolSkill_FileEdit_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("nginx_version=1.20"), 0644); err != nil {
		t.Fatal(err)
	}
	s := newSkill()
	res, err := s.Execute(context.Background(), &skill.Observations{}, execAction("file-edit", map[string]string{
		"path": path,
		"old":  "1.20",
		"new":  "1.24",
	}))
	if err != nil || !res.Success {
		t.Fatalf("file-edit: err=%v result=%v", err, res)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "1.24") {
		t.Errorf("expected 1.24 in file after edit, got %q", string(data))
	}
}

func TestDevOpsToolSkill_FileEdit_PatternNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("nginx_version=1.20"), 0644); err != nil {
		t.Fatal(err)
	}
	s := newSkill()
	res, err := s.Execute(context.Background(), &skill.Observations{}, execAction("file-edit", map[string]string{
		"path": path,
		"old":  "notpresent",
		"new":  "x",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if res.Success {
		t.Error("expected failure when pattern not found")
	}
}

func TestDevOpsToolSkill_FileEdit_MissingParams(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, execAction("file-edit", map[string]string{}))
	if err == nil {
		t.Error("expected error when 'path' and 'old' missing")
	}
}

// ---------------------------------------------------------------------------
// APICall (uses httptest server)
// ---------------------------------------------------------------------------

func TestDevOpsToolSkill_APICall_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	s := devops.NewDevOpsToolSkillWithClient(srv.Client())
	res, err := s.Execute(context.Background(), &skill.Observations{}, execAction("api-call", map[string]string{
		"url":    srv.URL + "/test",
		"method": "GET",
	}))
	if err != nil || !res.Success {
		t.Fatalf("api-call GET: err=%v result=%v", err, res)
	}
}

func TestDevOpsToolSkill_APICall_MissingParams(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, execAction("api-call", map[string]string{
		"url": "http://example.com",
		// missing "method"
	}))
	if err == nil {
		t.Error("expected error when method missing")
	}
}

// ---------------------------------------------------------------------------
// OpenPR (uses httptest server)
// ---------------------------------------------------------------------------

func TestDevOpsToolSkill_OpenPR_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"html_url": "https://github.com/owner/repo/pull/42",
			"number":   42,
		})
	}))
	defer srv.Close()

	s := devops.NewDevOpsToolSkillWithClient(srv.Client())
	res, err := s.Execute(context.Background(), &skill.Observations{}, execAction("open-pr", map[string]string{
		"api_url": srv.URL + "/repos/owner/repo/pulls",
		"token":   "test-token",
		"title":   "IaC update for twin-1",
		"head":    "feature/twin-1-update",
		"base":    "main",
		"body":    "Auto-generated by Pheromone agent",
	}))
	if err != nil || !res.Success {
		t.Fatalf("open-pr: err=%v result=%v", err, res)
	}
	if res.StateDelta.UpdatedFields["pr_url"] == "" {
		t.Error("expected pr_url in StateDelta")
	}
}

func TestDevOpsToolSkill_OpenPR_MissingToken(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, execAction("open-pr", map[string]string{
		"api_url": "http://example.com",
		"title":   "title",
		"head":    "branch",
		"base":    "main",
		// missing "token"
	}))
	if err == nil {
		t.Error("expected error when token missing")
	}
}

// ---------------------------------------------------------------------------
// UnknownActionType
// ---------------------------------------------------------------------------

func TestDevOpsToolSkill_UnknownAction(t *testing.T) {
	s := newSkill()
	_, err := s.Execute(context.Background(), &skill.Observations{}, execAction("teleport", nil))
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}
