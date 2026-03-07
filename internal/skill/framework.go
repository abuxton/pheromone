package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// ReasonFunc is a pluggable AI reasoning function. It receives the current observations
// and the available skill bundle and returns a list of planned actions.
// The default rule-based reasoner is used when no custom function is provided.
type ReasonFunc func(ctx context.Context, obs *Observations, skills []Skill) ([]Action, error)

// AgentFramework is the ADR-006 skill execution harness for Pheromone agents.
//
// It wires together:
//   - A SkillRegistry (receives the skill bundle distributed by the server)
//   - A pluggable ReasonFunc (rule-based by default; LLM in Phase 2)
//   - A structured logger that auto-emits JSON decision traces (ADR-007)
//
// Developer contract: implement the Agent interface (~50–100 lines) and call
// framework.Run(ctx, agent). The framework handles the reasoning loop, skill dispatch,
// metrics collection, and observability.
type AgentFramework struct {
	agentID  string
	registry *SkillRegistry
	reason   ReasonFunc
	interval time.Duration
	log      *slog.Logger
}

// FrameworkOption is a functional option for AgentFramework.
type FrameworkOption func(*AgentFramework)

// WithReasonFunc replaces the default rule-based reasoner with a custom function.
func WithReasonFunc(fn ReasonFunc) FrameworkOption {
	return func(f *AgentFramework) { f.reason = fn }
}

// WithInterval sets the reasoning loop tick interval (default: 5 s).
func WithInterval(d time.Duration) FrameworkOption {
	return func(f *AgentFramework) { f.interval = d }
}

// WithLogger sets a custom slog.Logger (default: JSON output to stdout).
func WithLogger(l *slog.Logger) FrameworkOption {
	return func(f *AgentFramework) { f.log = l }
}

// NewAgentFramework creates a framework for agentID using the provided skill registry.
func NewAgentFramework(agentID string, registry *SkillRegistry, opts ...FrameworkOption) *AgentFramework {
	f := &AgentFramework{
		agentID:  agentID,
		registry: registry,
		interval: 5 * time.Second,
		log:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
	f.reason = f.defaultReason
	for _, o := range opts {
		o(f)
	}
	return f
}

// Agent is the interface developers implement. It contains pure business logic; the
// framework handles gRPC, skill dispatch, and observability.
type Agent interface {
	// Initialize sets up local resources and returns the twins managed by this agent.
	Initialize() ([]*TwinRef, error)

	// CollectMetrics gathers current observations from the managed system.
	CollectMetrics(ctx context.Context) ([]Metric, error)

	// CurrentState returns the current actual state for the given twin.
	CurrentState(ctx context.Context, twinID string) (*TwinModel, error)

	// DesiredState returns the desired state for the given twin (from local cache or server).
	DesiredState(ctx context.Context, twinID string) (*TwinModel, error)

	// EnforceConfig applies a desired configuration to the managed system.
	EnforceConfig(ctx context.Context, desired *TwinModel) error

	// Shutdown flushes pending data and cleans up local resources.
	Shutdown() error
}

// Run starts the agent reasoning loop. It blocks until ctx is cancelled.
//
// Loop per tick:
//  1. Collect metrics via agent.CollectMetrics.
//  2. Read current and desired twin states.
//  3. Call ReasonFunc with observations and skill bundle.
//  4. Execute each planned action via the matching skill.
//  5. Emit a structured AI decision trace log entry.
func (f *AgentFramework) Run(ctx context.Context, agent Agent) error {
	twins, err := agent.Initialize()
	if err != nil {
		return fmt.Errorf("agent initialize: %w", err)
	}

	levels := twinLevels(twins)
	bundle := f.registry.BundleFor(f.agentID, levels)

	f.log.Info("agent started",
		slog.String("agent_id", f.agentID),
		slog.Int("twin_count", len(twins)),
		slog.Int("skill_count", len(bundle)),
	)

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if serr := agent.Shutdown(); serr != nil {
				f.log.Error("shutdown error", slog.String("error", serr.Error()))
			}
			f.log.Info("agent stopped", slog.String("agent_id", f.agentID))
			return ctx.Err()

		case <-ticker.C:
			if err := f.tick(ctx, agent, twins, bundle); err != nil {
				f.log.Error("reasoning tick error",
					slog.String("agent_id", f.agentID),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

// RunOneTick executes a single reasoning cycle and returns the planned actions.
// Useful for testing without starting the full loop.
func (f *AgentFramework) RunOneTick(ctx context.Context, agent Agent) ([]Action, error) {
	twins, err := agent.Initialize()
	if err != nil {
		return nil, fmt.Errorf("agent initialize: %w", err)
	}
	levels := twinLevels(twins)
	bundle := f.registry.BundleFor(f.agentID, levels)
	obs, err := f.observe(ctx, agent, twins)
	if err != nil {
		return nil, err
	}
	return f.reason(ctx, obs, bundle)
}

// tick is a single reasoning iteration.
func (f *AgentFramework) tick(ctx context.Context, agent Agent, twins []*TwinRef, bundle []Skill) error {
	start := time.Now()

	obs, err := f.observe(ctx, agent, twins)
	if err != nil {
		return fmt.Errorf("observe: %w", err)
	}

	actions, err := f.reason(ctx, obs, bundle)
	if err != nil {
		return fmt.Errorf("reason: %w", err)
	}

	executed := 0
	for _, a := range actions {
		if execErr := f.executeAction(ctx, agent, obs, a); execErr != nil {
			f.log.Warn("action failed",
				slog.String("skill", a.SkillName),
				slog.String("action_type", a.ActionType),
				slog.String("error", execErr.Error()),
			)
		} else {
			executed++
		}
	}

	f.emitDecisionTrace(obs, actions, executed, start)
	return nil
}

// observe collects metrics and twin states for the current tick.
func (f *AgentFramework) observe(ctx context.Context, agent Agent, twins []*TwinRef) (*Observations, error) {
	metrics, err := agent.CollectMetrics(ctx)
	if err != nil {
		return nil, err
	}

	var actual, desired []*TwinModel
	for _, tw := range twins {
		cur, curErr := agent.CurrentState(ctx, tw.ID)
		if curErr != nil {
			f.log.Warn("could not retrieve current twin state",
				slog.String("twin_id", tw.ID),
				slog.String("error", curErr.Error()),
			)
		} else if cur != nil {
			actual = append(actual, cur)
		}

		des, desErr := agent.DesiredState(ctx, tw.ID)
		if desErr != nil {
			f.log.Warn("could not retrieve desired twin state",
				slog.String("twin_id", tw.ID),
				slog.String("error", desErr.Error()),
			)
		} else if des != nil {
			desired = append(desired, des)
		}
	}

	return &Observations{
		Metrics:      metrics,
		Twins:        actual,
		DesiredTwins: desired,
	}, nil
}

// executeAction dispatches an action to the appropriate skill.
func (f *AgentFramework) executeAction(ctx context.Context, agent Agent, obs *Observations, a Action) error {
	s, err := f.registry.Get(a.SkillName)
	if err != nil {
		return fmt.Errorf("skill %q not found: %w", a.SkillName, err)
	}

	result, err := s.Execute(ctx, obs, &a)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("skill %q reported failure: %s", a.SkillName, result.Detail)
	}

	return nil
}

// defaultReason is the rule-based reasoner used in Phase 1 (no LLM required).
// It iterates over desired/actual twin pairs and plans a config-enforce action
// for any twin where drift is detected.
func (f *AgentFramework) defaultReason(_ context.Context, obs *Observations, _ []Skill) ([]Action, error) {
	desiredByID := make(map[string]*TwinModel, len(obs.DesiredTwins))
	for _, d := range obs.DesiredTwins {
		desiredByID[d.TwinID] = d
	}

	var actions []Action
	for _, actual := range obs.Twins {
		desired, ok := desiredByID[actual.TwinID]
		if !ok {
			continue
		}
		if hasDrift(desired, actual) {
			actions = append(actions, Action{
				SkillName:  "config-enforce",
				ActionType: "apply-config",
				TwinID:     actual.TwinID,
				Params:     map[string]string{"version": desired.Version},
				Rationale:  "drift detected between desired and actual twin state",
			})
		}
	}
	return actions, nil
}

// hasDrift returns true when any state field differs between desired and actual.
func hasDrift(desired, actual *TwinModel) bool {
	for k, dv := range desired.State {
		if av, ok := actual.State[k]; !ok || av != dv {
			return true
		}
	}
	return false
}

// emitDecisionTrace writes a structured AI decision trace log entry (ADR-007).
func (f *AgentFramework) emitDecisionTrace(obs *Observations, actions []Action, executed int, start time.Time) {
	twinUpdates := make([]string, 0, len(actions))
	for _, a := range actions {
		if a.TwinID != "" {
			twinUpdates = append(twinUpdates, a.TwinID)
		}
	}

	twinUpdatesJSON, _ := json.Marshal(twinUpdates)

	f.log.Info("reasoning cycle complete",
		slog.String("agent_id", f.agentID),
		slog.String("component", "reasoning-loop"),
		slog.Int("observations_count", len(obs.Metrics)+len(obs.Twins)),
		slog.Bool("drift_detected", len(actions) > 0),
		slog.Int("actions_planned", len(actions)),
		slog.Int("actions_executed", executed),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("twin_updates", string(twinUpdatesJSON)),
	)
}

// twinLevels extracts the unique TwinLevel values from a slice of TwinRefs.
func twinLevels(twins []*TwinRef) []TwinLevel {
	seen := make(map[TwinLevel]struct{}, len(twins))
	out := make([]TwinLevel, 0, len(twins))
	for _, t := range twins {
		if _, ok := seen[t.Level]; !ok {
			seen[t.Level] = struct{}{}
			out = append(out, t.Level)
		}
	}
	return out
}
