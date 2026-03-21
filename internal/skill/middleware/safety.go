// Package middleware implements the safety-net middleware layer (ADR-020, Phase 2d).
//
// It realises OpenSWE's Middleware Hooks pattern, extending ADR-011's
// post-action hook service with two additional safety nets:
//
//	IaCPRSafetyNet       — If a reasoning cycle completes significant changes
//	                       without opening an IaC PR, this middleware
//	                       automatically opens one via GitOpsSkill.
//
//	ActionRollbackMiddleware — If an action's production execution fails, this
//	                          middleware triggers a rollback and emits an audit
//	                          event via the hook service.
//
// Both middlewares are composable with the AgentFramework through the
// MiddlewareChain helper.
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// ActionOutcome describes whether an action completed successfully.
type ActionOutcome string

const (
	// ActionOutcomeSuccess indicates the action completed without error.
	ActionOutcomeSuccess ActionOutcome = "success"
	// ActionOutcomeFailure indicates the action encountered an error.
	ActionOutcomeFailure ActionOutcome = "failure"
	// ActionOutcomeRolledBack indicates the action was rolled back.
	ActionOutcomeRolledBack ActionOutcome = "rolled-back"
)

// ActionExecution holds the context for a single action that was executed.
type ActionExecution struct {
	// Action is the action that was executed.
	Action skill.Action
	// Outcome is the execution result.
	Outcome ActionOutcome
	// Error is non-nil when Outcome is ActionOutcomeFailure.
	Error error
	// ExecutedAt is when the action was executed.
	ExecutedAt time.Time
	// TwinID is the twin the action targeted.
	TwinID string
}

// RollbackFunc is called by ActionRollbackMiddleware when an action fails.
// It receives the failed action and should attempt to restore the prior state.
type RollbackFunc func(ctx context.Context, failed ActionExecution) error

// PROpener is the interface used by IaCPRSafetyNet to open PRs.
// The GitOpsSkill satisfies this interface.
type PROpener interface {
	Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error)
}

// ---------------------------------------------------------------------------
// IaCPRSafetyNet
// ---------------------------------------------------------------------------

// IaCPRSafetyNet monitors a reasoning cycle and automatically opens an IaC PR
// if significant state changes occurred but no PR was opened by the agent loop.
//
// "Significant change" is defined as: one or more actions with RequiresApproval
// set to true, or actions whose ActionType matches a configured set of
// significant action types (defaults: "apply-config", "commit-sandbox",
// "apply-twin-model").
type IaCPRSafetyNet struct {
	// Opener is used to open PRs when the safety net triggers.
	Opener PROpener
	// WorkspacePath is the IaC workspace to commit from.
	WorkspacePath string
	// SignificantActions is the set of action types that trigger PR creation.
	// Defaults to {"apply-config", "commit-sandbox", "apply-twin-model"}.
	SignificantActions map[string]struct{}
	log               *slog.Logger
}

// NewIaCPRSafetyNet creates an IaCPRSafetyNet with default significant action types.
// opener must be non-nil; passing nil will cause a panic at construction time rather
// than silently at check time.
func NewIaCPRSafetyNet(opener PROpener, workspacePath string) *IaCPRSafetyNet {
	if opener == nil {
		panic("IaCPRSafetyNet: opener must not be nil")
	}
	return &IaCPRSafetyNet{
		Opener:        opener,
		WorkspacePath: workspacePath,
		SignificantActions: map[string]struct{}{
			"apply-config":     {},
			"commit-sandbox":   {},
			"apply-twin-model": {},
		},
		log: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// CheckAndEnsurePR inspects the executed actions. If any are significant and
// none have already opened a PR (detected by the presence of a "open-pr" or
// "commit-and-open-pr" action), it automatically opens a draft PR.
//
// Returns the PR URL if a PR was opened, or empty string if none was needed.
func (m *IaCPRSafetyNet) CheckAndEnsurePR(ctx context.Context, twinID string, executed []ActionExecution) (string, error) {
	hasSignificant := false
	hasPR := false
	for _, e := range executed {
		if e.Outcome != ActionOutcomeSuccess {
			continue
		}
		if _, ok := m.SignificantActions[e.Action.ActionType]; ok {
			hasSignificant = true
		}
		if e.Action.ActionType == "open-pr" || e.Action.ActionType == "commit-and-open-pr" {
			hasPR = true
		}
		if e.Action.RequiresApproval {
			hasSignificant = true
		}
	}

	if !hasSignificant || hasPR {
		return "", nil // nothing to do
	}

	// Safety net fires: open a PR automatically.
	description := fmt.Sprintf(
		"IaC PR safety net: significant changes were applied to twin %q without an explicit PR.\n\nActions: %s",
		twinID, describeActions(executed),
	)
	m.log.Warn("IaCPRSafetyNet: no PR opened after significant changes; opening automatically",
		slog.String("twin_id", twinID),
		slog.Int("significant_actions", len(executed)),
	)

	result, err := m.Opener.Execute(ctx, &skill.Observations{}, &skill.Action{
		ActionType: "commit-and-open-pr",
		TwinID:     twinID,
		Params: map[string]string{
			"workspace_path": m.WorkspacePath,
			"description":    description,
		},
	})
	if err != nil {
		return "", fmt.Errorf("IaCPRSafetyNet: open PR: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("IaCPRSafetyNet: open PR failed: %s", result.Detail)
	}

	prURL := ""
	if result.StateDelta != nil {
		prURL = result.StateDelta.UpdatedFields["pr_url"]
	}
	m.log.Info("IaCPRSafetyNet: draft PR opened",
		slog.String("twin_id", twinID),
		slog.String("pr_url", prURL),
	)
	return prURL, nil
}

// describeActions returns a short comma-separated list of action types.
func describeActions(executed []ActionExecution) string {
	seen := make(map[string]struct{})
	var parts []string
	for _, e := range executed {
		if _, ok := seen[e.Action.ActionType]; !ok {
			parts = append(parts, e.Action.ActionType)
			seen[e.Action.ActionType] = struct{}{}
		}
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

// ---------------------------------------------------------------------------
// ActionRollbackMiddleware
// ---------------------------------------------------------------------------

// ActionRollbackMiddleware intercepts failed action executions and calls a
// RollbackFunc to restore prior state, then emits an audit event.
type ActionRollbackMiddleware struct {
	rollbackFn RollbackFunc
	log        *slog.Logger
	// RollbackAttempts counts how many rollbacks were triggered (for metrics).
	RollbackAttempts int
	// RollbackSuccesses counts successful rollbacks.
	RollbackSuccesses int
}

// NewActionRollbackMiddleware creates an ActionRollbackMiddleware with the given rollback function.
// If rollbackFn is nil, a no-op rollback is used.
func NewActionRollbackMiddleware(rollbackFn RollbackFunc) *ActionRollbackMiddleware {
	if rollbackFn == nil {
		rollbackFn = func(_ context.Context, _ ActionExecution) error { return nil }
	}
	return &ActionRollbackMiddleware{
		rollbackFn: rollbackFn,
		log:        slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// OnActionFailed is called when an action's execution returns an error.
// It invokes the rollback function and logs the outcome as an audit event.
// Returns an ActionExecution record with Outcome set to ActionOutcomeRolledBack or ActionOutcomeFailure.
func (m *ActionRollbackMiddleware) OnActionFailed(ctx context.Context, failed ActionExecution) ActionExecution {
	m.RollbackAttempts++
	m.log.Warn("action-rollback: action failed; attempting rollback",
		slog.String("skill", failed.Action.SkillName),
		slog.String("action_type", failed.Action.ActionType),
		slog.String("twin_id", failed.TwinID),
		slog.String("error", fmt.Sprintf("%v", failed.Error)),
	)

	if err := m.rollbackFn(ctx, failed); err != nil {
		m.log.Error("action-rollback: rollback failed",
			slog.String("action_type", failed.Action.ActionType),
			slog.String("rollback_error", err.Error()),
		)
		return ActionExecution{
			Action:     failed.Action,
			Outcome:    ActionOutcomeFailure,
			Error:      fmt.Errorf("rollback also failed: rollback=%w; original=%v", err, failed.Error),
			ExecutedAt: failed.ExecutedAt,
			TwinID:     failed.TwinID,
		}
	}

	m.RollbackSuccesses++
	m.log.Info("action-rollback: rollback succeeded",
		slog.String("action_type", failed.Action.ActionType),
		slog.String("twin_id", failed.TwinID),
	)
	return ActionExecution{
		Action:     failed.Action,
		Outcome:    ActionOutcomeRolledBack,
		ExecutedAt: time.Now(),
		TwinID:     failed.TwinID,
	}
}

// WrapExecute wraps a skill.Execute call with automatic rollback on failure.
// Returns the SkillResult on success; calls OnActionFailed and returns the
// rollback record on failure.
func (m *ActionRollbackMiddleware) WrapExecute(
	ctx context.Context,
	s skill.Skill,
	obs *skill.Observations,
	action *skill.Action,
) (*skill.SkillResult, ActionExecution) {
	exec := ActionExecution{
		Action:     *action,
		TwinID:     action.TwinID,
		ExecutedAt: time.Now(),
	}

	result, err := s.Execute(ctx, obs, action)
	if err != nil || result == nil || !result.Success {
		if err == nil && result == nil {
			err = fmt.Errorf("skill returned nil result")
		} else if err == nil {
			err = fmt.Errorf("skill reported failure: %s", result.Detail)
		}
		exec.Outcome = ActionOutcomeFailure
		exec.Error = err
		rolled := m.OnActionFailed(ctx, exec)
		return result, rolled
	}

	exec.Outcome = ActionOutcomeSuccess
	return result, exec
}
