// Package subagent implements the SubagentSkill (ADR-020, Phase 2c).
//
// It realises OpenSWE's Subagent Orchestration pattern: a parent agent can
// spawn child reasoning cycles for independent per-twin or per-subsystem tasks,
// enabling parallel remediation of multiple issues without blocking the parent
// reasoning loop.
//
// Design:
//
//	Each subagent runs as a goroutine with its own context (derived from the
//	parent context). Goroutine lifecycle is managed by the SubagentSkill:
//	spawned subagents are tracked by a unique ID; callers can wait for, cancel,
//	or list all active subagents.
//
//	An orphan-reaper helper is provided (ReapOrphans) to cancel all
//	still-running child contexts when the parent context is cancelled; it is
//	the caller's responsibility to invoke this as part of their shutdown
//	handling to prevent goroutine leaks.
//
// Observability: active subagent count emitted as structured log entries.
package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// SubagentStatus describes the current lifecycle state of a subagent.
type SubagentStatus string

const (
	// SubagentStatusRunning indicates the subagent goroutine is executing.
	SubagentStatusRunning SubagentStatus = "running"
	// SubagentStatusCompleted indicates the subagent finished successfully.
	SubagentStatusCompleted SubagentStatus = "completed"
	// SubagentStatusFailed indicates the subagent finished with an error.
	SubagentStatusFailed SubagentStatus = "failed"
	// SubagentStatusCancelled indicates the subagent was cancelled by the caller.
	SubagentStatusCancelled SubagentStatus = "cancelled"
)

// SubagentResult is the outcome returned when a subagent finishes.
type SubagentResult struct {
	// SubagentID is the unique identifier of the subagent.
	SubagentID string `json:"subagent_id"`
	// Status is the final lifecycle state.
	Status SubagentStatus `json:"status"`
	// Actions lists the action types planned by the subagent's reasoning cycle.
	Actions []string `json:"actions,omitempty"`
	// Error is the error message if Status == SubagentStatusFailed.
	Error string `json:"error,omitempty"`
	// DurationMs is how long the subagent ran.
	DurationMs int64 `json:"duration_ms"`
}

// SubagentInfo describes a currently-active or recently-completed subagent.
type SubagentInfo struct {
	// ID is the unique subagent identifier.
	ID string `json:"id"`
	// TwinIDs lists the twins the subagent was spawned for.
	TwinIDs []string `json:"twin_ids"`
	// TaskSpec is a human-readable description of the subagent's task.
	TaskSpec string `json:"task_spec"`
	// Status is the current state.
	Status SubagentStatus `json:"status"`
	// SpawnedAt is when the subagent was created.
	SpawnedAt time.Time `json:"spawned_at"`
}

// TaskFunc is the function that a subagent executes.
// It receives the subagent's context and returns a list of action types and an error.
type TaskFunc func(ctx context.Context) ([]string, error)

// record is the internal tracking structure for a spawned subagent.
type record struct {
	info      SubagentInfo
	cancel    context.CancelFunc
	resultCh  chan SubagentResult
	result    *SubagentResult // stored after completion so Wait is idempotent
	done      chan struct{}    // closed when the subagent finishes
	spawnedAt time.Time
}

// SubagentSkill manages the lifecycle of child reasoning cycles (ADR-020 Phase 2c).
//
// It satisfies the skill.Skill interface and can be invoked through the standard
// Execute dispatch path.
//
// Supported action types:
//   - "spawn"       — SpawnSubagent (task_spec, twin_ids params)
//   - "wait"        — WaitForSubagent (subagent_id param)
//   - "cancel"      — CancelSubagent (subagent_id param)
//   - "list"        — ListActiveSubagents
type SubagentSkill struct {
	log    *slog.Logger
	mu     sync.RWMutex
	agents map[string]*record
	seq    int
}

// NewSubagentSkill creates a SubagentSkill ready for use.
func NewSubagentSkill() *SubagentSkill {
	return &SubagentSkill{
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		agents: make(map[string]*record),
	}
}

func (s *SubagentSkill) Name() string                  { return "subagent" }
func (s *SubagentSkill) Version() string               { return "1.0.0" }
func (s *SubagentSkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }

// Execute dispatches the action to the matching subagent operation.
func (s *SubagentSkill) Execute(ctx context.Context, _ *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	switch action.ActionType {
	case "spawn":
		return s.execSpawn(ctx, action)
	case "wait":
		return s.execWait(ctx, action)
	case "cancel":
		return s.execCancel(action)
	case "list":
		return s.execList()
	default:
		return nil, fmt.Errorf("subagent: unknown action type %q", action.ActionType)
	}
}

// SpawnSubagent creates a new subagent goroutine that executes taskFn.
// Returns the unique subagent ID.
func (s *SubagentSkill) SpawnSubagent(parentCtx context.Context, taskSpec string, twinIDs []string, taskFn TaskFunc) (string, error) {
	// Copy the caller's slice so mutations after Spawn don't affect stored metadata.
	twinIDsCopy := make([]string, len(twinIDs))
	copy(twinIDsCopy, twinIDs)

	s.mu.Lock()
	s.seq++
	id := fmt.Sprintf("subagent-%d", s.seq)
	childCtx, cancel := context.WithCancel(parentCtx)
	resultCh := make(chan SubagentResult, 1)
	doneCh := make(chan struct{})
	r := &record{
		info: SubagentInfo{
			ID:        id,
			TwinIDs:   twinIDsCopy,
			TaskSpec:  taskSpec,
			Status:    SubagentStatusRunning,
			SpawnedAt: time.Now().UTC(),
		},
		cancel:    cancel,
		resultCh:  resultCh,
		done:      doneCh,
		spawnedAt: time.Now(),
	}
	s.agents[id] = r
	s.mu.Unlock()

	s.log.Info("subagent spawned",
		slog.String("subagent_id", id),
		slog.String("task_spec", taskSpec),
		slog.Int("twin_count", len(twinIDsCopy)),
	)

	go s.run(id, childCtx, taskFn, resultCh, doneCh)
	return id, nil
}

// run is the goroutine that executes a subagent task.
func (s *SubagentSkill) run(id string, ctx context.Context, taskFn TaskFunc, resultCh chan<- SubagentResult, doneCh chan struct{}) {
	start := time.Now()

	var actions []string
	var err error

	// Recover from panics so a crashing task marks the subagent failed rather than
	// bringing down the entire process.
	func() {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("subagent panic: %v", p)
			}
		}()
		actions, err = taskFn(ctx)
	}()

	dur := time.Since(start).Milliseconds()

	result := SubagentResult{
		SubagentID: id,
		Actions:    actions,
		DurationMs: dur,
	}
	status := SubagentStatusCompleted
	if ctx.Err() != nil {
		status = SubagentStatusCancelled
	} else if err != nil {
		status = SubagentStatusFailed
		result.Error = err.Error()
	}
	result.Status = status

	s.mu.Lock()
	if r, ok := s.agents[id]; ok {
		r.info.Status = status
		resultCopy := result
		r.result = &resultCopy
	}
	s.mu.Unlock()

	resultCh <- result
	close(doneCh)

	s.log.Info("subagent finished",
		slog.String("subagent_id", id),
		slog.String("status", string(status)),
		slog.Int64("duration_ms", dur),
	)
}

// WaitForSubagent blocks until the subagent identified by id completes or
// ctx is cancelled. The call is idempotent: subsequent calls return the cached
// result without blocking. The subagent record is removed after a successful wait.
func (s *SubagentSkill) WaitForSubagent(ctx context.Context, id string) (*SubagentResult, error) {
	s.mu.RLock()
	r, ok := s.agents[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("subagent: %q not found", id)
	}

	// If the subagent already finished, return the cached result immediately.
	s.mu.RLock()
	cached := r.result
	s.mu.RUnlock()
	if cached != nil {
		s.mu.Lock()
		delete(s.agents, id)
		s.mu.Unlock()
		return cached, nil
	}

	select {
	case result := <-r.resultCh:
		s.mu.Lock()
		// Remove the record now that the caller has collected the result.
		delete(s.agents, id)
		s.mu.Unlock()
		return &result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("subagent: wait for %q: %w", id, ctx.Err())
	}
}

// CancelSubagent cancels the context of the subagent identified by id.
func (s *SubagentSkill) CancelSubagent(id string) error {
	s.mu.Lock()
	r, ok := s.agents[id]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("subagent: %q not found", id)
	}
	r.cancel()
	return nil
}

// ListActiveSubagents returns a snapshot of all tracked subagents.
func (s *SubagentSkill) ListActiveSubagents() []SubagentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SubagentInfo, 0, len(s.agents))
	for _, r := range s.agents {
		out = append(out, r.info)
	}
	return out
}

// ReapOrphans cancels and removes all subagents that are still marked as running.
// Called by the parent agent framework when the parent context is cancelled (ADR-020).
func (s *SubagentSkill) ReapOrphans() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, r := range s.agents {
		if r.info.Status == SubagentStatusRunning {
			r.cancel()
			r.info.Status = SubagentStatusCancelled
			s.log.Warn("subagent orphan reaped", slog.String("subagent_id", id))
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Execute dispatch helpers
// ---------------------------------------------------------------------------

func (s *SubagentSkill) execSpawn(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	taskSpec := action.Params["task_spec"]
	if taskSpec == "" {
		taskSpec = action.Rationale
	}

	// Build the twin ID list: prefer the "twin_ids" param (comma-separated),
	// fall back to a single-element list containing action.TwinID.
	var twinIDs []string
	if raw := action.Params["twin_ids"]; raw != "" {
		for _, id := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				twinIDs = append(twinIDs, trimmed)
			}
		}
	}
	if len(twinIDs) == 0 {
		twinIDs = []string{action.TwinID}
	}

	// Default task: no-op placeholder (real tasks are provided via SpawnSubagent directly).
	id, err := s.SpawnSubagent(ctx, taskSpec, twinIDs, func(_ context.Context) ([]string, error) {
		return nil, nil
	})
	if err != nil {
		return nil, fmt.Errorf("subagent: spawn: %w", err)
	}
	return &skill.SkillResult{
		Success: true,
		StateDelta: &skill.TwinDelta{
			UpdatedFields: map[string]string{"subagent_id": id},
		},
		Detail: fmt.Sprintf("subagent %q spawned for %d twin(s)", id, len(twinIDs)),
	}, nil
}

func (s *SubagentSkill) execWait(ctx context.Context, action *skill.Action) (*skill.SkillResult, error) {
	id := action.Params["subagent_id"]
	if id == "" {
		return nil, fmt.Errorf("subagent: wait requires 'subagent_id' param")
	}
	result, err := s.WaitForSubagent(ctx, id)
	if err != nil {
		return nil, err
	}
	success := result.Status == SubagentStatusCompleted
	return &skill.SkillResult{
		Success: success,
		Detail: fmt.Sprintf("subagent %q %s in %d ms; actions=%v",
			id, result.Status, result.DurationMs, result.Actions),
	}, nil
}

func (s *SubagentSkill) execCancel(action *skill.Action) (*skill.SkillResult, error) {
	id := action.Params["subagent_id"]
	if id == "" {
		return nil, fmt.Errorf("subagent: cancel requires 'subagent_id' param")
	}
	if err := s.CancelSubagent(id); err != nil {
		return nil, err
	}
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("subagent %q cancelled", id),
	}, nil
}

func (s *SubagentSkill) execList() (*skill.SkillResult, error) {
	agents := s.ListActiveSubagents()
	return &skill.SkillResult{
		Success: true,
		Detail:  fmt.Sprintf("%d subagents tracked", len(agents)),
	}, nil
}
