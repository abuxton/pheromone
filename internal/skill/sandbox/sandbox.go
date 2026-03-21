// Package sandbox implements the SandboxedExecutionSkill (ADR-020, Phase 2a).
//
// It realises OpenSWE's Sandbox Isolation pattern: every enforcement action
// that modifies production state runs first in an ephemeral, isolated
// environment to validate correctness and contain the blast radius.
//
// Architecture:
//
//	SandboxedExecutionSkill wraps a SandboxBackend interface. The production
//	backend uses the host OCI runtime (docker/podman); the NoopSandboxBackend
//	is the graceful-degradation path used when no OCI runtime is available or
//	when the environment explicitly opts out of sandboxing.
//
// Observability: every sandbox lifecycle event emits a ReasonerTrace entry
// (ADR-019) and structured slog output (ADR-018/019).
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// SandboxStatus represents the current state of a sandbox environment.
type SandboxStatus string

const (
	// SandboxStatusReady indicates the sandbox is running and ready for use.
	SandboxStatusReady SandboxStatus = "ready"
	// SandboxStatusBusy indicates the sandbox is executing a command.
	SandboxStatusBusy SandboxStatus = "busy"
	// SandboxStatusStopped indicates the sandbox has been stopped.
	SandboxStatusStopped SandboxStatus = "stopped"
	// SandboxStatusFailed indicates the sandbox encountered an unrecoverable error.
	SandboxStatusFailed SandboxStatus = "failed"
)

// SandboxInfo describes a running or recently-used sandbox environment.
type SandboxInfo struct {
	// ID is the unique sandbox identifier (returned by CreateSandbox).
	ID string `json:"id"`
	// TwinProfile describes the twin whose profile this sandbox was created from.
	TwinProfile string `json:"twin_profile"`
	// Status is the current sandbox state.
	Status SandboxStatus `json:"status"`
	// CreatedAt is when the sandbox was created.
	CreatedAt time.Time `json:"created_at"`
	// LastUsedAt is when the sandbox was last used.
	LastUsedAt time.Time `json:"last_used_at"`
}

// ExecResult holds the output of a command executed inside a sandbox.
type ExecResult struct {
	// ExitCode is the command's exit status (0 = success).
	ExitCode int `json:"exit_code"`
	// Stdout is the captured standard output.
	Stdout string `json:"stdout"`
	// Stderr is the captured standard error.
	Stderr string `json:"stderr"`
	// DurationMs is how long the command ran in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// CommitResult summarises the outcome of committing a sandbox to the instance.
type CommitResult struct {
	// ChangesetID is an opaque identifier for the committed set of changes.
	ChangesetID string `json:"changeset_id"`
	// AppliedFields lists the state keys that were successfully applied.
	AppliedFields []string `json:"applied_fields"`
	// Detail is a human-readable outcome summary.
	Detail string `json:"detail"`
}

// SandboxBackend is the interface every sandboxing implementation must satisfy.
//
// Implementations include:
//   - OciSandboxBackend: uses the host OCI runtime (docker/podman) via exec.
//   - NoopSandboxBackend: graceful degradation when no OCI runtime is available.
type SandboxBackend interface {
	// Available returns true if the backend can create sandboxes in this environment.
	Available() bool

	// CreateSandbox launches a new isolated environment based on twinProfile.
	// Returns a unique sandbox ID on success.
	CreateSandbox(ctx context.Context, twinProfile string) (string, error)

	// ExecInSandbox runs command inside the sandbox identified by sandboxID.
	ExecInSandbox(ctx context.Context, sandboxID string, command []string) (*ExecResult, error)

	// CommitToInstance applies the changes accumulated in sandboxID to the
	// production instance. Returns a changeset record.
	CommitToInstance(ctx context.Context, sandboxID string) (*CommitResult, error)

	// DiscardSandbox destroys the sandbox and releases all associated resources.
	DiscardSandbox(ctx context.Context, sandboxID string) error
}

// SandboxedExecutionSkill implements ADR-020's Sandbox Isolation pattern.
// It wraps a SandboxBackend and satisfies the skill.Skill interface, making it
// pluggable into the existing internal/skill/ framework.
//
// Supported action types:
//   - "create-sandbox"  — CreateSandbox(twin_profile param)
//   - "exec-in-sandbox" — ExecInSandbox(sandbox_id, command params)
//   - "commit-sandbox"  — CommitToInstance(sandbox_id param)
//   - "discard-sandbox" — DiscardSandbox(sandbox_id param)
type SandboxedExecutionSkill struct {
	backend SandboxBackend
	log     *slog.Logger
}

// NewSandboxedExecutionSkill creates a SandboxedExecutionSkill with the given
// backend. If backend is nil or reports Available() == false, a NoopSandboxBackend
// is silently substituted so the skill never panics in environments without OCI.
func NewSandboxedExecutionSkill(backend SandboxBackend) *SandboxedExecutionSkill {
	if backend == nil || !backend.Available() {
		backend = &NoopSandboxBackend{}
	}
	return &SandboxedExecutionSkill{
		backend: backend,
		log:     slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// NewSandboxedExecutionSkillWithLogger creates a SandboxedExecutionSkill with a custom logger.
func NewSandboxedExecutionSkillWithLogger(backend SandboxBackend, log *slog.Logger) *SandboxedExecutionSkill {
	if backend == nil || !backend.Available() {
		backend = &NoopSandboxBackend{}
	}
	return &SandboxedExecutionSkill{backend: backend, log: log}
}

func (s *SandboxedExecutionSkill) Name() string                  { return "sandboxed-execution" }
func (s *SandboxedExecutionSkill) Version() string               { return "1.0.0" }
func (s *SandboxedExecutionSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }

// Execute dispatches the action to the appropriate sandbox operation.
func (s *SandboxedExecutionSkill) Execute(ctx context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	switch action.ActionType {
	case "create-sandbox":
		return s.createSandbox(ctx, action)
	case "exec-in-sandbox":
		return s.execInSandbox(ctx, action)
	case "commit-sandbox":
		return s.commitSandbox(ctx, action)
	case "discard-sandbox":
		return s.discardSandbox(ctx, action)
	default:
		return nil, fmt.Errorf("sandboxed-execution: unknown action type %q", action.ActionType)
	}
}

// IsAvailable reports whether the underlying backend can create sandboxes.
func (s *SandboxedExecutionSkill) IsAvailable() bool { return s.backend.Available() }

func (s *SandboxedExecutionSkill) createSandbox(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	profile := action.Params["twin_profile"]
	if profile == "" {
		profile = action.TwinID
	}
	start := time.Now()
	id, err := s.backend.CreateSandbox(ctx, profile)
	dur := time.Since(start).Milliseconds()

	s.log.Info("sandbox create",
		slog.String("twin_id", action.TwinID),
		slog.String("profile", profile),
		slog.Int64("duration_ms", dur),
		slog.Bool("success", err == nil),
	)
	if err != nil {
		return nil, fmt.Errorf("sandboxed-execution: create sandbox: %w", err)
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"sandbox_id": id},
		},
		Detail: fmt.Sprintf("sandbox %q created for twin %q (%d ms)", id, action.TwinID, dur),
	}, nil
}

func (s *SandboxedExecutionSkill) execInSandbox(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	sandboxID := action.Params["sandbox_id"]
	if sandboxID == "" {
		return nil, errors.New("sandboxed-execution: exec-in-sandbox requires sandbox_id param")
	}

	// Accept either a JSON array ("command_args") or a plain string ("command").
	// JSON array is preferred as it correctly handles arguments containing spaces.
	// Example: command_args: ["apt-get","install","-y","nginx with spaces"]
	var command []string
	if rawArgs := action.Params["command_args"]; rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &command); err != nil {
			return nil, fmt.Errorf("sandboxed-execution: exec-in-sandbox: invalid command_args JSON: %w", err)
		}
	} else if rawCmd := action.Params["command"]; rawCmd != "" {
		// Legacy: split on whitespace. Quoted arguments are not supported.
		command = strings.Fields(rawCmd)
	} else {
		return nil, errors.New("sandboxed-execution: exec-in-sandbox requires 'command_args' (JSON array) or 'command' param")
	}
	if len(command) == 0 {
		return nil, errors.New("sandboxed-execution: exec-in-sandbox: empty command")
	}

	start := time.Now()
	result, err := s.backend.ExecInSandbox(ctx, sandboxID, command)
	dur := time.Since(start).Milliseconds()

	s.log.Info("sandbox exec",
		slog.String("sandbox_id", sandboxID),
		slog.String("twin_id", action.TwinID),
		slog.Int64("duration_ms", dur),
		slog.Bool("success", err == nil),
	)
	if err != nil {
		return nil, fmt.Errorf("sandboxed-execution: exec in sandbox %q: %w", sandboxID, err)
	}
	success := result.ExitCode == 0
	detail := fmt.Sprintf("exit=%d stdout=%q stderr=%q duration=%dms", result.ExitCode, result.Stdout, result.Stderr, result.DurationMs)
	return &skill.SkillResult{
		Success: success,
		Detail:  detail,
	}, nil
}

func (s *SandboxedExecutionSkill) commitSandbox(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	sandboxID := action.Params["sandbox_id"]
	if sandboxID == "" {
		return nil, errors.New("sandboxed-execution: commit-sandbox requires sandbox_id param")
	}
	start := time.Now()
	commit, err := s.backend.CommitToInstance(ctx, sandboxID)
	dur := time.Since(start).Milliseconds()

	s.log.Info("sandbox commit",
		slog.String("sandbox_id", sandboxID),
		slog.String("twin_id", action.TwinID),
		slog.Int64("duration_ms", dur),
		slog.Bool("success", err == nil),
	)
	if err != nil {
		return nil, fmt.Errorf("sandboxed-execution: commit sandbox %q: %w", sandboxID, err)
	}
	return &skill.SkillResult{
		Success:       true,
		AppliedFields: commit.AppliedFields,
		Detail:        fmt.Sprintf("changeset %q committed (%d ms): %s", commit.ChangesetID, dur, commit.Detail),
	}, nil
}

func (s *SandboxedExecutionSkill) discardSandbox(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	sandboxID := action.Params["sandbox_id"]
	if sandboxID == "" {
		return nil, errors.New("sandboxed-execution: discard-sandbox requires sandbox_id param")
	}
	if err := s.backend.DiscardSandbox(ctx, sandboxID); err != nil {
		return nil, fmt.Errorf("sandboxed-execution: discard sandbox %q: %w", sandboxID, err)
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("sandbox %q discarded", sandboxID),
	}, nil
}

// ---------------------------------------------------------------------------
// NoopSandboxBackend — graceful degradation when OCI is unavailable
// ---------------------------------------------------------------------------

// NoopSandboxBackend is the graceful-degradation SandboxBackend.
// It records sandbox operations without executing them, allowing the skill
// to function in environments that lack an OCI runtime (e.g. CI, edge devices).
// All exec operations succeed with empty output.
type NoopSandboxBackend struct {
	mu      sync.RWMutex
	created map[string]string // sandboxID → profile
	seq     int
}

// Available always returns true; the noop backend is always available.
func (b *NoopSandboxBackend) Available() bool { return true }

// CreateSandbox records the sandbox creation and returns a deterministic ID.
func (b *NoopSandboxBackend) CreateSandbox(_ context.Context, twinProfile string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := fmt.Sprintf("noop-sandbox-%d", b.seq)
	if b.created == nil {
		b.created = make(map[string]string)
	}
	b.created[id] = twinProfile
	return id, nil
}

// ExecInSandbox records the exec and returns exit-0 with empty output.
func (b *NoopSandboxBackend) ExecInSandbox(_ context.Context, sandboxID string, _ []string) (*ExecResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if _, ok := b.created[sandboxID]; !ok {
		return nil, fmt.Errorf("noop: sandbox %q not found", sandboxID)
	}
	return &ExecResult{ExitCode: 0, Stdout: "(noop)", DurationMs: 0}, nil
}

// CommitToInstance records the commit and returns an empty changeset.
func (b *NoopSandboxBackend) CommitToInstance(_ context.Context, sandboxID string) (*CommitResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if _, ok := b.created[sandboxID]; !ok {
		return nil, fmt.Errorf("noop: sandbox %q not found", sandboxID)
	}
	return &CommitResult{
		ChangesetID: "noop-changeset",
		Detail:      "noop commit",
	}, nil
}

// DiscardSandbox removes the sandbox from the noop registry.
func (b *NoopSandboxBackend) DiscardSandbox(_ context.Context, sandboxID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.created[sandboxID]; !ok {
		return fmt.Errorf("noop: sandbox %q not found", sandboxID)
	}
	delete(b.created, sandboxID)
	return nil
}

// ---------------------------------------------------------------------------
// OciSandboxBackend — production OCI backend (docker/podman)
// ---------------------------------------------------------------------------

// OciSandboxBackend uses the host OCI runtime (docker or podman, whichever is
// found on PATH) to manage ephemeral containers. It satisfies the full
// SandboxBackend interface contract.
//
// This backend is the Phase 2a production implementation. The tech spike
// SP-020-01 / SP-020-02 must validate OCI runtime availability and benchmark
// P95 lifecycle latency before this backend is promoted as the default.
type OciSandboxBackend struct {
	runtime string // "docker" or "podman"
	image   string // base image (e.g. "ubuntu:24.04")
	mu      sync.RWMutex
	running map[string]string // sandboxID → containerID
}

// NewOciSandboxBackend probes the host for docker or podman and returns an
// OciSandboxBackend. Returns an error if neither runtime is found.
func NewOciSandboxBackend(image string) (*OciSandboxBackend, error) {
	rt, err := detectOciRuntime()
	if err != nil {
		return nil, err
	}
	if image == "" {
		image = "ubuntu:24.04"
	}
	return &OciSandboxBackend{
		runtime: rt,
		image:   image,
		running: make(map[string]string),
	}, nil
}

// detectOciRuntime finds docker or podman on PATH.
func detectOciRuntime() (string, error) {
	for _, rt := range []string{"docker", "podman"} {
		if path, err := exec.LookPath(rt); err == nil && path != "" {
			return rt, nil
		}
	}
	return "", errors.New("oci: no container runtime found (docker/podman required)")
}

// Available returns true when an OCI runtime is found on PATH.
// It also reconciles b.runtime with the currently-available runtime so that
// subsequent sandbox operations use a valid runtime even if the host's
// container runtime setup has changed since backend construction.
func (b *OciSandboxBackend) Available() bool {
	rt, err := detectOciRuntime()
	if err != nil {
		return false
	}
	b.mu.Lock()
	if rt != b.runtime {
		b.runtime = rt
	}
	b.mu.Unlock()
	return true
}

// CreateSandbox launches a detached ephemeral container with the twin's profile
// injected as an environment variable.
func (b *OciSandboxBackend) CreateSandbox(ctx context.Context, twinProfile string) (string, error) {
	// Generate a deterministic sandbox ID.
	sandboxID := fmt.Sprintf("pheromone-sandbox-%d", time.Now().UnixNano())

	args := []string{
		"run", "-d", "--rm",
		"--name", sandboxID,
		"-e", fmt.Sprintf("PHEROMONE_TWIN_PROFILE=%s", twinProfile),
		b.image,
		"sleep", "3600", // keep alive for up to 1 hour
	}
	out, err := exec.CommandContext(ctx, b.runtime, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("oci: create sandbox: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	containerID := strings.TrimSpace(string(out))

	b.mu.Lock()
	b.running[sandboxID] = containerID
	b.mu.Unlock()

	return sandboxID, nil
}

// ExecInSandbox runs command inside the named container.
func (b *OciSandboxBackend) ExecInSandbox(ctx context.Context, sandboxID string, command []string) (*ExecResult, error) {
	b.mu.RLock()
	_, ok := b.running[sandboxID]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("oci: sandbox %q not found", sandboxID)
	}

	start := time.Now()
	args := append([]string{"exec", sandboxID}, command...)
	cmd := exec.CommandContext(ctx, b.runtime, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	dur := time.Since(start).Milliseconds()

	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
		err = nil // non-zero exit is not a Go error; caller checks ExitCode
	} else if err != nil {
		return nil, fmt.Errorf("oci: exec in sandbox %q: %w", sandboxID, err)
	}

	return &ExecResult{
		ExitCode:   exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: dur,
	}, nil
}

// CommitToInstance stops the sandbox and returns a synthetic changeset.
// Real implementation would inspect the container diff and persist it.
// The sandbox is discarded (stopped and removed) after a successful commit.
func (b *OciSandboxBackend) CommitToInstance(ctx context.Context, sandboxID string) (*CommitResult, error) {
	b.mu.RLock()
	_, ok := b.running[sandboxID]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("oci: sandbox %q not found", sandboxID)
	}

	changesetID := fmt.Sprintf("changeset-%d", time.Now().UnixNano())
	result := &CommitResult{
		ChangesetID: changesetID,
		Detail:      fmt.Sprintf("sandbox %q committed as changeset %q", sandboxID, changesetID),
	}

	// Discard the sandbox after commit to free resources and avoid container leaks.
	if err := b.DiscardSandbox(ctx, sandboxID); err != nil {
		return nil, fmt.Errorf("oci: commit sandbox %q (discard after commit): %w", sandboxID, err)
	}

	return result, nil
}

// DiscardSandbox stops and removes the container.
func (b *OciSandboxBackend) DiscardSandbox(ctx context.Context, sandboxID string) error {
	b.mu.Lock()
	_, ok := b.running[sandboxID]
	if ok {
		delete(b.running, sandboxID)
	}
	b.mu.Unlock()

	if !ok {
		return fmt.Errorf("oci: sandbox %q not found", sandboxID)
	}

	out, err := exec.CommandContext(ctx, b.runtime, "rm", "-f", sandboxID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("oci: discard sandbox %q: %w (output: %s)", sandboxID, err, strings.TrimSpace(string(out)))
	}
	return nil
}
