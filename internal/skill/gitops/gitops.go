// Package gitops implements the GitOpsSkill (ADR-020, Phase 2d).
//
// It realises OpenSWE's PR-creation pattern combined with Pheromone's twin
// model GitOps principle: significant infrastructure changes are committed to
// an IaC repository and surfaced as draft pull requests, providing an
// operator review gate before changes reach production.
//
// Architecture:
//
//	GitOpsSkill wraps a GitOpsBackend interface. The production backend uses
//	the local git binary (exec.Command) for repository operations and the VCS
//	HTTP API for PR creation. The NoopGitOpsBackend is the graceful-degradation
//	path used when no IaC repository is configured.
//
// Supported action types:
//   - "clone-repo"         — CloneIaCRepo
//   - "apply-twin-model"   — ApplyTwinModelToIaC
//   - "commit-and-open-pr" — CommitAndOpenPR
//   - "check-pr-status"    — CheckPRStatus
package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// PRStatus describes the current state of an open pull request.
type PRStatus struct {
	// URL is the pull-request web URL.
	URL string `json:"url"`
	// State is "open", "closed", or "merged".
	State string `json:"state"`
	// Mergeable is true when the PR has no conflicts.
	Mergeable bool `json:"mergeable"`
	// Title is the PR title.
	Title string `json:"title"`
}

// Workspace is a local clone of an IaC repository used for mutations.
type Workspace struct {
	// Path is the absolute local filesystem path to the cloned repository.
	Path string `json:"path"`
	// RepoURL is the URL the workspace was cloned from.
	RepoURL string `json:"repo_url"`
}

// GitOpsBackend is the interface every GitOps implementation must satisfy.
type GitOpsBackend interface {
	// Available returns true when the backend can perform GitOps operations.
	Available() bool

	// CloneIaCRepo clones the IaC repository at repoURL into a local workspace.
	CloneIaCRepo(ctx context.Context, repoURL string) (*Workspace, error)

	// ApplyTwinModelToIaC writes the desired twin state into the workspace as
	// an IaC change (e.g. YAML/HCL file update).
	ApplyTwinModelToIaC(ctx context.Context, ws *Workspace, model *skill.TwinModel) ([]string, error)

	// CommitAndOpenPR commits the workspace changes and opens a draft PR.
	// Returns the PR URL.
	CommitAndOpenPR(ctx context.Context, ws *Workspace, description string) (string, error)

	// CheckPRStatus fetches the current state of the PR at prURL.
	CheckPRStatus(ctx context.Context, prURL string) (*PRStatus, error)
}

// GitOpsSkill implements the GitOps/IaC PR workflow (ADR-020 Phase 2d).
type GitOpsSkill struct {
	backend GitOpsBackend
	log     *slog.Logger
}

// NewGitOpsSkill creates a GitOpsSkill with the given backend.
// If backend is nil or reports Available() == false, a NoopGitOpsBackend is
// substituted so the skill never panics when no IaC repository is configured.
func NewGitOpsSkill(backend GitOpsBackend) *GitOpsSkill {
	if backend == nil || !backend.Available() {
		backend = &NoopGitOpsBackend{}
	}
	return &GitOpsSkill{
		backend: backend,
		log:     slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (s *GitOpsSkill) Name() string                  { return "gitops" }
func (s *GitOpsSkill) Version() string               { return "1.0.0" }
func (s *GitOpsSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }

// Execute dispatches the action to the appropriate GitOps operation.
func (s *GitOpsSkill) Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	switch action.ActionType {
	case "clone-repo":
		return s.cloneRepo(ctx, action)
	case "apply-twin-model":
		return s.applyTwinModel(ctx, obs, action)
	case "commit-and-open-pr":
		return s.commitAndOpenPR(ctx, action)
	case "check-pr-status":
		return s.checkPRStatus(ctx, action)
	default:
		return nil, fmt.Errorf("gitops: unknown action type %q", action.ActionType)
	}
}

// IsAvailable reports whether the underlying backend can perform GitOps operations.
func (s *GitOpsSkill) IsAvailable() bool { return s.backend.Available() }

func (s *GitOpsSkill) cloneRepo(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	repoURL := action.Params["repo_url"]
	if repoURL == "" {
		return nil, fmt.Errorf("gitops: clone-repo requires 'repo_url' param")
	}
	start := time.Now()
	ws, err := s.backend.CloneIaCRepo(ctx, repoURL)
	dur := time.Since(start).Milliseconds()
	s.log.Info("gitops clone-repo",
		slog.String("repo_url", repoURL),
		slog.Int64("duration_ms", dur),
		slog.Bool("success", err == nil),
	)
	if err != nil {
		return nil, fmt.Errorf("gitops: clone-repo: %w", err)
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"workspace_path": ws.Path},
		},
		Detail: fmt.Sprintf("gitops: cloned %q → %q (%d ms)", repoURL, ws.Path, dur),
	}, nil
}

func (s *GitOpsSkill) applyTwinModel(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	wsPath := action.Params["workspace_path"]
	if wsPath == "" {
		return nil, fmt.Errorf("gitops: apply-twin-model requires 'workspace_path' param")
	}

	// Find the desired twin model from observations.
	var model *skill.TwinModel
	for _, d := range obs.DesiredTwins {
		if d.TwinID == action.TwinID {
			model = d
			break
		}
	}
	if model == nil {
		return nil, fmt.Errorf("gitops: apply-twin-model: desired state for twin %q not found in observations", action.TwinID)
	}

	ws := &Workspace{Path: wsPath, RepoURL: action.Params["repo_url"]}
	changed, err := s.backend.ApplyTwinModelToIaC(ctx, ws, model)
	if err != nil {
		return nil, fmt.Errorf("gitops: apply-twin-model: %w", err)
	}
	return &skill.SkillResult{
		Success:       true,
		AppliedFields: changed,
		Detail:        fmt.Sprintf("gitops: applied %d fields to IaC workspace %q", len(changed), wsPath),
	}, nil
}

func (s *GitOpsSkill) commitAndOpenPR(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	wsPath := action.Params["workspace_path"]
	description := action.Params["description"]
	if wsPath == "" {
		return nil, fmt.Errorf("gitops: commit-and-open-pr requires 'workspace_path' param")
	}
	if description == "" {
		description = fmt.Sprintf("Pheromone: automated IaC update for twin %q", action.TwinID)
	}

	ws := &Workspace{Path: wsPath, RepoURL: action.Params["repo_url"]}
	start := time.Now()
	prURL, err := s.backend.CommitAndOpenPR(ctx, ws, description)
	dur := time.Since(start).Milliseconds()
	s.log.Info("gitops commit-and-open-pr",
		slog.String("twin_id", action.TwinID),
		slog.String("workspace", wsPath),
		slog.Int64("duration_ms", dur),
		slog.Bool("success", err == nil),
	)
	if err != nil {
		return nil, fmt.Errorf("gitops: commit-and-open-pr: %w", err)
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"pr_url": prURL},
		},
		Detail: fmt.Sprintf("gitops: PR created at %q (%d ms)", prURL, dur),
	}, nil
}

func (s *GitOpsSkill) checkPRStatus(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	prURL := action.Params["pr_url"]
	if prURL == "" {
		return nil, fmt.Errorf("gitops: check-pr-status requires 'pr_url' param")
	}
	status, err := s.backend.CheckPRStatus(ctx, prURL)
	if err != nil {
		return nil, fmt.Errorf("gitops: check-pr-status: %w", err)
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{
				"pr_state":     status.State,
				"pr_mergeable": fmt.Sprintf("%v", status.Mergeable),
			},
		},
		Detail: fmt.Sprintf("gitops: PR %q is %s (mergeable=%v)", prURL, status.State, status.Mergeable),
	}, nil
}

// ---------------------------------------------------------------------------
// NoopGitOpsBackend — graceful degradation when no IaC repo configured
// ---------------------------------------------------------------------------

// NoopGitOpsBackend is the no-op GitOps backend used when no IaC repository
// is configured. It records operations without performing real git or HTTP
// calls so the skill can be registered and invoked without side effects.
type NoopGitOpsBackend struct{}

// Available always returns true; the noop backend is always available.
func (b *NoopGitOpsBackend) Available() bool { return true }

// CloneIaCRepo returns a synthetic workspace without touching the filesystem.
func (b *NoopGitOpsBackend) CloneIaCRepo(_ context.Context, repoURL string) (*Workspace, error) {
	return &Workspace{Path: "./tmp/pheromone-noop-workspace", RepoURL: repoURL}, nil
}

// ApplyTwinModelToIaC returns the twin model's state keys as "applied" fields.
func (b *NoopGitOpsBackend) ApplyTwinModelToIaC(_ context.Context, _ *Workspace, model *skill.TwinModel) ([]string, error) {
	changed := make([]string, 0, len(model.State))
	for k := range model.State {
		changed = append(changed, k)
	}
	return changed, nil
}

// CommitAndOpenPR returns a synthetic PR URL.
func (b *NoopGitOpsBackend) CommitAndOpenPR(_ context.Context, ws *Workspace, _ string) (string, error) {
	return fmt.Sprintf("https://github.com/noop/repo/pull/%d", time.Now().UnixNano()%1000), nil
}

// CheckPRStatus returns a synthetic open status.
func (b *NoopGitOpsBackend) CheckPRStatus(_ context.Context, prURL string) (*PRStatus, error) {
	return &PRStatus{
		URL:       prURL,
		State:     "open",
		Mergeable: true,
		Title:     "noop PR",
	}, nil
}

// ---------------------------------------------------------------------------
// GitBackend — production git+HTTP backend
// ---------------------------------------------------------------------------

// GitBackend implements GitOpsBackend using the local git binary and a VCS
// HTTP API for PR creation. This is the Phase 2d production path; full
// validation requires the go-git tech spike (SP-020-03) to be completed first.
type GitBackend struct {
	// VCSAPIBase is the base URL of the VCS API (e.g. https://api.github.com).
	VCSAPIBase string
	// Token is the VCS API token for authentication.
	Token string
	// PROwner is the repository owner for PR creation.
	PROwner string
	// PRRepo is the repository name for PR creation.
	PRRepo string
	// BaseBranch is the PR base branch (default: "main").
	BaseBranch string
	// httpClient is used for API calls; override in tests.
	httpClient *http.Client
}

// NewGitBackend creates a GitBackend.
func NewGitBackend(vcsAPIBase, token, owner, repo string) *GitBackend {
	return &GitBackend{
		VCSAPIBase: vcsAPIBase,
		Token:      token,
		PROwner:    owner,
		PRRepo:     repo,
		BaseBranch: "main",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetHTTPClient replaces the HTTP client used for API calls.
// Intended for test use to inject a test server's client.
func (b *GitBackend) SetHTTPClient(c *http.Client) { b.httpClient = c }

// Available returns true when git is on PATH and a VCS token is configured.
func (b *GitBackend) Available() bool {
	if b.Token == "" {
		return false
	}
	_, err := exec.LookPath("git")
	return err == nil
}

// CloneIaCRepo clones the repository into a temp directory.
func (b *GitBackend) CloneIaCRepo(ctx context.Context, repoURL string) (*Workspace, error) {
	dest, err := os.MkdirTemp("./tmp", "pheromone-iac-*")
	if err != nil {
		// Fall back to os temp dir if ./tmp does not exist in this environment.
		dest, err = os.MkdirTemp("", "pheromone-iac-*")
		if err != nil {
			return nil, fmt.Errorf("git-backend: create temp dir: %w", err)
		}
	}
	out, err := exec.CommandContext(ctx, "git", "clone", repoURL, dest).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git-backend: clone %q: %w (output: %s)", repoURL, err, strings.TrimSpace(string(out)))
	}
	return &Workspace{Path: dest, RepoURL: repoURL}, nil
}

// ApplyTwinModelToIaC writes the desired twin state as a YAML file in the workspace.
func (b *GitBackend) ApplyTwinModelToIaC(_ context.Context, ws *Workspace, model *skill.TwinModel) ([]string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Pheromone twin model: %s (v%s)\n", model.TwinID, model.Version))
	sb.WriteString("state:\n")
	changed := make([]string, 0, len(model.State))
	for k, v := range model.State {
		sb.WriteString(fmt.Sprintf("  %s: %q\n", k, v))
		changed = append(changed, k)
	}

	filePath := ws.Path + "/pheromone-twin-" + sanitizeID(model.TwinID) + ".yaml"
	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		return nil, fmt.Errorf("git-backend: write IaC file: %w", err)
	}
	return changed, nil
}

// CommitAndOpenPR stages, commits, creates a branch, and opens a draft PR.
func (b *GitBackend) CommitAndOpenPR(ctx context.Context, ws *Workspace, description string) (string, error) {
	branch := fmt.Sprintf("pheromone/iac-update-%d", time.Now().UnixNano())

	// Create and checkout branch.
	if out, err := exec.CommandContext(ctx, "git", "-C", ws.Path, "checkout", "-b", branch).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git-backend: checkout branch: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	// Stage all changes.
	if out, err := exec.CommandContext(ctx, "git", "-C", ws.Path, "add", "-A").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git-backend: git add: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	// Commit.
	commitMsg := fmt.Sprintf("feat(twin): Pheromone IaC update\n\n%s", description)
	if out, err := exec.CommandContext(ctx, "git", "-C", ws.Path, "commit", "-m", commitMsg).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git-backend: git commit: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	// Push.
	if out, err := exec.CommandContext(ctx, "git", "-C", ws.Path, "push", "origin", branch).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git-backend: git push: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	// Open PR via VCS API.
	return b.openPR(ctx, branch, description)
}

// openPR creates a draft pull request via the GitHub API.
func (b *GitBackend) openPR(ctx context.Context, headBranch, description string) (string, error) {
	baseBranch := b.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}
	body := map[string]interface{}{
		"title": "Pheromone: automated IaC update",
		"head":  headBranch,
		"base":  baseBranch,
		"body":  description,
		"draft": true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("git-backend: marshal PR body: %w", err)
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls", b.VCSAPIBase, b.PROwner, b.PRRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("git-backend: build PR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("git-backend: PR API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("git-backend: PR API HTTP %d", resp.StatusCode)
	}

	var prResp struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prResp); err != nil {
		return "", fmt.Errorf("git-backend: decode PR response: %w", err)
	}
	return prResp.HTMLURL, nil
}

// CheckPRStatus fetches the current PR state from the GitHub API.
func (b *GitBackend) CheckPRStatus(ctx context.Context, prURL string) (*PRStatus, error) {
	// Convert HTML URL → API URL: https://github.com/owner/repo/pull/N → /repos/owner/repo/pulls/N
	apiURL := convertPRURLToAPIURL(b.VCSAPIBase, prURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("git-backend: check-pr request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("git-backend: check-pr HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("git-backend: check-pr: HTTP %d from %s", resp.StatusCode, apiURL)
	}

	var pr struct {
		Title     string `json:"title"`
		State     string `json:"state"`
		Mergeable *bool  `json:"mergeable"`
		HTMLURL   string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("git-backend: decode PR status: %w", err)
	}
	mergeable := pr.Mergeable != nil && *pr.Mergeable
	return &PRStatus{
		URL:       pr.HTMLURL,
		State:     pr.State,
		Mergeable: mergeable,
		Title:     pr.Title,
	}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// sanitizeID replaces characters not safe for filenames with dashes.
func sanitizeID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, id)
}

// convertPRURLToAPIURL converts a GitHub PR web URL to its API URL.
// e.g. https://github.com/owner/repo/pull/42 → https://api.github.com/repos/owner/repo/pulls/42
func convertPRURLToAPIURL(apiBase, prURL string) string {
	// Simple heuristic: replace the github.com host and /pull/ with /pulls/.
	api := strings.Replace(prURL, "https://github.com/", apiBase+"/repos/", 1)
	api = strings.Replace(api, "/pull/", "/pulls/", 1)
	return api
}
