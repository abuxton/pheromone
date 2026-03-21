// Package devops implements the DevOpsToolSkill (ADR-020, Phase 2a).
//
// It realises OpenSWE's Tool Curation pattern: a small, focused set of
// ~13 typed DevOps tools that an agentic AI loop may invoke. Quantity is
// avoided in favour of quality — each tool has a clear contract and
// structured input/output.
//
// Curated tool set:
//
//	ShellExec         — execute shell commands
//	FileRead          — read a file from the instance
//	FileWrite         — write/create a file on the instance
//	FileEdit          — apply a targeted string substitution to a file
//	GitClone          — clone a git repository
//	GitCommit         — stage all changes and commit in a local repository
//	GitPush           — push commits to a remote branch
//	OpenPR            — open a draft pull request via VCS API
//	APICall           — make an authenticated HTTP request
//	PackageInstall    — install OS packages
//	PackageRemove     — remove OS packages
//	ServiceRestart    — restart a systemd/container service
//	ServiceStatus     — query the status of a service
//
// All tools are OS-level (TwinLevelOS) to prevent workload agents from
// performing arbitrary shell execution.
package devops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// DevOpsToolSkill is the curated DevOps tool set (ADR-020 Tool Curation pattern).
// Each public Execute dispatch corresponds to one of the ~13 curated tools.
//
// Concurrency: all tool methods are stateless; concurrent invocations are safe.
type DevOpsToolSkill struct {
	// httpClient is used for APICall. Override in tests to avoid real HTTP.
	httpClient httpDoer
	log        *slog.Logger
}

// httpDoer abstracts the http.Client for testing.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// NewDevOpsToolSkill creates a DevOpsToolSkill with the default http.Client.
func NewDevOpsToolSkill() *DevOpsToolSkill {
	return &DevOpsToolSkill{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// NewDevOpsToolSkillWithClient creates a DevOpsToolSkill with a custom HTTP client (for tests).
func NewDevOpsToolSkillWithClient(client httpDoer) *DevOpsToolSkill {
	return &DevOpsToolSkill{
		httpClient: client,
		log:        slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

func (t *DevOpsToolSkill) Name() string                  { return "devops-tools" }
func (t *DevOpsToolSkill) Version() string               { return "1.0.0" }
func (t *DevOpsToolSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }

// Execute dispatches the action to the matching tool handler.
// The action.ActionType must be one of the 13 curated tool names.
func (t *DevOpsToolSkill) Execute(ctx context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	switch action.ActionType {
	case "shell-exec":
		return t.shellExec(ctx, action)
	case "file-read":
		return t.fileRead(action)
	case "file-write":
		return t.fileWrite(action)
	case "file-edit":
		return t.fileEdit(action)
	case "git-clone":
		return t.gitClone(ctx, action)
	case "git-commit":
		return t.gitCommit(ctx, action)
	case "git-push":
		return t.gitPush(ctx, action)
	case "open-pr":
		return t.openPR(ctx, action)
	case "api-call":
		return t.apiCall(ctx, action)
	case "package-install":
		return t.packageInstall(ctx, action)
	case "package-remove":
		return t.packageRemove(ctx, action)
	case "service-restart":
		return t.serviceRestart(ctx, action)
	case "service-status":
		return t.serviceStatus(ctx, action)
	default:
		return nil, fmt.Errorf("devops-tools: unknown action type %q", action.ActionType)
	}
}

// ---------------------------------------------------------------------------
// Tool: ShellExec
// ---------------------------------------------------------------------------

// shellExec runs an arbitrary shell command via /bin/sh -c.
//
// Required params: "command" (the shell command string).
// Optional params: "workdir" (working directory).
func (t *DevOpsToolSkill) shellExec(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	command := action.Params["command"]
	if command == "" {
		return nil, fmt.Errorf("devops-tools: shell-exec requires 'command' param")
	}
	workdir := action.Params["workdir"]

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}

	start := time.Now()
	out, err := cmd.CombinedOutput()
	dur := time.Since(start).Milliseconds()

	t.log.Info("shell-exec",
		slog.String("twin_id", action.TwinID),
		slog.Int64("duration_ms", dur),
		slog.Bool("success", err == nil),
	)

	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("shell-exec failed (%d ms): %v; output: %s", dur, err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("shell-exec ok (%d ms): %s", dur, strings.TrimSpace(string(out))),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: FileRead
// ---------------------------------------------------------------------------

// fileRead reads the contents of a file on the managed instance.
//
// Required params: "path" (absolute or relative path to the file).
func (t *DevOpsToolSkill) fileRead(action *skill.Action) (*skill.SkillResult, error) {
	path := action.Params["path"]
	if path == "" {
		return nil, fmt.Errorf("devops-tools: file-read requires 'path' param")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("file-read %q: %v", path, err),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"file_content": string(data)},
		},
		Detail: fmt.Sprintf("file-read %q: %d bytes", path, len(data)),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: FileWrite
// ---------------------------------------------------------------------------

// fileWrite writes content to a file on the managed instance, creating it if
// it does not exist and truncating if it does.
//
// Required params: "path", "content".
func (t *DevOpsToolSkill) fileWrite(action *skill.Action) (*skill.SkillResult, error) {
	path := action.Params["path"]
	content := action.Params["content"]
	if path == "" {
		return nil, fmt.Errorf("devops-tools: file-write requires 'path' param")
	}
	perm := os.FileMode(0644)
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("file-write %q: %v", path, err),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("file-write %q: %d bytes written", path, len(content)),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: FileEdit
// ---------------------------------------------------------------------------

// fileEdit applies a targeted string substitution to an existing file.
//
// Required params: "path", "old" (string to replace), "new" (replacement).
// The first occurrence of "old" is replaced; use "replace_all=true" to replace all.
func (t *DevOpsToolSkill) fileEdit(action *skill.Action) (*skill.SkillResult, error) {
	path := action.Params["path"]
	oldStr := action.Params["old"]
	newStr := action.Params["new"]
	if path == "" || oldStr == "" {
		return nil, fmt.Errorf("devops-tools: file-edit requires 'path' and 'old' params")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("file-edit read %q: %v", path, err),
		}, nil
	}
	original := string(data)
	var updated string
	if action.Params["replace_all"] == "true" {
		updated = strings.ReplaceAll(original, oldStr, newStr)
	} else {
		updated = strings.Replace(original, oldStr, newStr, 1)
	}
	if updated == original {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("file-edit %q: pattern %q not found", path, oldStr),
		}, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("file-edit write %q: %v", path, err),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("file-edit %q: replaced %q → %q", path, oldStr, newStr),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: GitClone
// ---------------------------------------------------------------------------

// gitClone clones a git repository into a local directory.
//
// Required params: "url" (repository URL).
// Optional params: "dest" (destination path; defaults to a temp dir).
func (t *DevOpsToolSkill) gitClone(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	repoURL := action.Params["url"]
	if repoURL == "" {
		return nil, fmt.Errorf("devops-tools: git-clone requires 'url' param")
	}
	dest := action.Params["dest"]
	if dest == "" {
		var err error
		dest, err = os.MkdirTemp("./tmp", "pheromone-gitclone-*")
		if err != nil {
			// Fall back to os temp dir if ./tmp does not exist in this environment.
			dest, err = os.MkdirTemp("", "pheromone-gitclone-*")
			if err != nil {
				return nil, fmt.Errorf("devops-tools: git-clone temp dir: %w", err)
			}
		}
	}

	var args []string
	if branch := action.Params["branch"]; branch != "" {
		args = []string{"clone", "-b", branch, repoURL, dest}
	} else {
		args = []string{"clone", repoURL, dest}
	}

	out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("git-clone %q: %v; output: %s", repoURL, err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"repo_path": dest},
		},
		Detail: fmt.Sprintf("git-clone %q → %q", repoURL, dest),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: GitCommit
// ---------------------------------------------------------------------------

// gitCommit stages all changes in a local repository and creates a commit.
//
// Required params: "repo_path" (local repo directory), "message" (commit message).
// Optional params: "author_name", "author_email".
func (t *DevOpsToolSkill) gitCommit(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	repoPath := action.Params["repo_path"]
	message := action.Params["message"]
	if repoPath == "" || message == "" {
		return nil, fmt.Errorf("devops-tools: git-commit requires 'repo_path' and 'message' params")
	}

	// Stage all changes.
	addCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("git-commit add: %v; output: %s", err, strings.TrimSpace(string(out))),
		}, nil
	}

	// Commit.
	commitArgs := []string{"-C", repoPath, "commit", "-m", message}
	env := os.Environ()
	if name := action.Params["author_name"]; name != "" {
		env = append(env, "GIT_AUTHOR_NAME="+name, "GIT_COMMITTER_NAME="+name)
	}
	if email := action.Params["author_email"]; email != "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_EMAIL="+email)
	}
	commitCmd := exec.CommandContext(ctx, "git", commitArgs...)
	commitCmd.Env = env
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("git-commit: %v; output: %s", err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("git-commit in %q: %s", repoPath, strings.TrimSpace(string(out))),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: GitPush
// ---------------------------------------------------------------------------

// gitPush pushes local commits to a remote.
//
// Required params: "repo_path", "remote" (default: "origin"), "branch".
func (t *DevOpsToolSkill) gitPush(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	repoPath := action.Params["repo_path"]
	branch := action.Params["branch"]
	if repoPath == "" || branch == "" {
		return nil, fmt.Errorf("devops-tools: git-push requires 'repo_path' and 'branch' params")
	}
	remote := action.Params["remote"]
	if remote == "" {
		remote = "origin"
	}

	out, err := exec.CommandContext(ctx, "git", "-C", repoPath, "push", remote, branch).CombinedOutput()
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("git-push %s/%s: %v; output: %s", remote, branch, err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("git-push %s/%s ok", remote, branch),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: OpenPR
// ---------------------------------------------------------------------------

// openPR opens a draft pull request via the VCS HTTP API.
//
// Required params: "api_url" (e.g. https://api.github.com/repos/owner/repo/pulls),
//
//	"token" (Bearer token), "title", "head" (source branch), "base" (target branch).
//
// Optional params: "body" (PR description).
func (t *DevOpsToolSkill) openPR(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	apiURL := action.Params["api_url"]
	token := action.Params["token"]
	title := action.Params["title"]
	head := action.Params["head"]
	base := action.Params["base"]
	if apiURL == "" || token == "" || title == "" || head == "" || base == "" {
		return nil, fmt.Errorf("devops-tools: open-pr requires 'api_url', 'token', 'title', 'head', 'base' params")
	}

	body := map[string]interface{}{
		"title": title,
		"head":  head,
		"base":  base,
		"body":  action.Params["body"],
		"draft": true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("devops-tools: open-pr marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("devops-tools: open-pr request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("open-pr HTTP error: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("open-pr: HTTP %d from %s", resp.StatusCode, apiURL),
		}, nil
	}

	var prResp struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prResp); err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("open-pr: failed to decode PR response: %v", err),
		}, nil
	}
	if prResp.HTMLURL == "" {
		return &skill.SkillResult{
			Success: false,
			Detail:  "open-pr: PR URL missing from API response",
		}, nil
	}

	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"pr_url": prResp.HTMLURL},
		},
		Detail: fmt.Sprintf("open-pr #%d: %s", prResp.Number, prResp.HTMLURL),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: APICall
// ---------------------------------------------------------------------------

// apiCall performs an authenticated HTTP request.
//
// Required params: "url", "method" (GET/POST/PUT/PATCH/DELETE).
// Optional params: "token" (Bearer), "body" (request body string),
//
//	"content_type" (default: application/json).
func (t *DevOpsToolSkill) apiCall(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	url := action.Params["url"]
	method := strings.ToUpper(action.Params["method"])
	if url == "" || method == "" {
		return nil, fmt.Errorf("devops-tools: api-call requires 'url' and 'method' params")
	}

	var bodyReader io.Reader
	if bodyStr := action.Params["body"]; bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("devops-tools: api-call request: %w", err)
	}
	ct := action.Params["content_type"]
	if ct == "" {
		ct = "application/json"
	}
	req.Header.Set("Content-Type", ct)
	if token := action.Params["token"]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("api-call %s %s: %v", method, url, err),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	return &skill.SkillResult{
		Success: success,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"response_body": string(respBody)},
		},
		Detail: fmt.Sprintf("api-call %s %s → HTTP %d", method, url, resp.StatusCode),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: PackageInstall
// ---------------------------------------------------------------------------

// packageInstall installs OS packages using the auto-detected package manager.
//
// Required params: "packages" (space-separated package names).
// Optional params: "package_manager" override ("apt", "yum", "dnf", "apk").
func (t *DevOpsToolSkill) packageInstall(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	packages := action.Params["packages"]
	if packages == "" {
		return nil, fmt.Errorf("devops-tools: package-install requires 'packages' param")
	}
	pm := detectPackageManager(action.Params["package_manager"])
	args := packageInstallArgs(pm, strings.Fields(packages))

	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	t.log.Info("package-install",
		slog.String("pm", pm),
		slog.String("packages", packages),
		slog.Bool("success", err == nil),
	)
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("package-install (%s) %q: %v; output: %s", pm, packages, err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("package-install (%s) %q ok", pm, packages),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: PackageRemove
// ---------------------------------------------------------------------------

// packageRemove removes OS packages using the auto-detected package manager.
//
// Required params: "packages" (space-separated).
// Optional params: "package_manager" override.
func (t *DevOpsToolSkill) packageRemove(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	packages := action.Params["packages"]
	if packages == "" {
		return nil, fmt.Errorf("devops-tools: package-remove requires 'packages' param")
	}
	pm := detectPackageManager(action.Params["package_manager"])
	args := packageRemoveArgs(pm, strings.Fields(packages))

	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("package-remove (%s) %q: %v; output: %s", pm, packages, err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("package-remove (%s) %q ok", pm, packages),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: ServiceRestart
// ---------------------------------------------------------------------------

// serviceRestart restarts a systemd service (or docker container by name).
//
// Required params: "service" (service name).
// Optional params: "manager" ("systemd" or "docker"; auto-detected if omitted).
func (t *DevOpsToolSkill) serviceRestart(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	service := action.Params["service"]
	if service == "" {
		return nil, fmt.Errorf("devops-tools: service-restart requires 'service' param")
	}
	manager := detectServiceManager(action.Params["manager"])
	var out []byte
	var err error
	switch manager {
	case "docker":
		out, err = exec.CommandContext(ctx, "docker", "restart", service).CombinedOutput()
	default: // systemd
		out, err = exec.CommandContext(ctx, "systemctl", "restart", service).CombinedOutput()
	}
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			Detail:  fmt.Sprintf("service-restart %q (%s): %v; output: %s", service, manager, err, strings.TrimSpace(string(out))),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("service-restart %q (%s) ok", service, manager),
	}, nil
}

// ---------------------------------------------------------------------------
// Tool: ServiceStatus
// ---------------------------------------------------------------------------

// serviceStatus queries the operational status of a service.
//
// Required params: "service" (service name).
// Optional params: "manager".
func (t *DevOpsToolSkill) serviceStatus(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	service := action.Params["service"]
	if service == "" {
		return nil, fmt.Errorf("devops-tools: service-status requires 'service' param")
	}
	manager := detectServiceManager(action.Params["manager"])
	var out []byte
	var err error
	switch manager {
	case "docker":
		out, err = exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", service).CombinedOutput()
	default: // systemd
		out, err = exec.CommandContext(ctx, "systemctl", "is-active", service).CombinedOutput()
	}
	status := strings.TrimSpace(string(out))
	if err != nil {
		return &skill.SkillResult{
			Success: false,
			StateDelta: &skill.TwinDelta{
				UpdatedFields: map[string]string{"service_status": status},
			},
			Detail: fmt.Sprintf("service-status %q (%s): not active (%v)", service, manager, err),
		}, nil
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"service_status": status},
		},
		Detail: fmt.Sprintf("service-status %q (%s): %s", service, manager, status),
	}, nil
}

// ---------------------------------------------------------------------------
// Package manager helpers
// ---------------------------------------------------------------------------

// detectPackageManager returns the active package manager. It honours an
// explicit override, then probes PATH for known package managers.
func detectPackageManager(override string) string {
	if override != "" {
		return override
	}
	for _, pm := range []string{"apt-get", "dnf", "yum", "apk"} {
		if p, err := exec.LookPath(pm); err == nil && p != "" {
			// Normalise apt-get → apt for display, but use apt-get for execution.
			return pm
		}
	}
	return "apt-get" // fallback
}

// packageInstallArgs builds the install command for the detected package manager.
func packageInstallArgs(pm string, packages []string) []string {
	switch pm {
	case "apk":
		return append([]string{"apk", "add", "--no-cache"}, packages...)
	case "yum":
		return append([]string{"yum", "install", "-y"}, packages...)
	case "dnf":
		return append([]string{"dnf", "install", "-y"}, packages...)
	default: // apt-get
		return append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, packages...)
	}
}

// packageRemoveArgs builds the remove command for the detected package manager.
func packageRemoveArgs(pm string, packages []string) []string {
	switch pm {
	case "apk":
		return append([]string{"apk", "del"}, packages...)
	case "yum":
		return append([]string{"yum", "remove", "-y"}, packages...)
	case "dnf":
		return append([]string{"dnf", "remove", "-y"}, packages...)
	default: // apt-get
		return append([]string{"apt-get", "remove", "-y"}, packages...)
	}
}

// ---------------------------------------------------------------------------
// Service manager helpers
// ---------------------------------------------------------------------------

// detectServiceManager returns the active service manager. Honours an explicit
// override; falls back to systemd if no container runtime is found.
func detectServiceManager(override string) string {
	if override != "" {
		return override
	}
	if p, err := exec.LookPath("systemctl"); err == nil && p != "" {
		return "systemd"
	}
	if p, err := exec.LookPath("docker"); err == nil && p != "" {
		return "docker"
	}
	return "systemd" // fallback
}
