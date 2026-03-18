// Package context implements the ContextAssemblySkill and its middleware
// companions (ADR-020, Phase 2b).
//
// It realises OpenSWE's Context Engineering pattern: before every planning
// cycle, a rich ContextBundle is assembled from all available sources and
// injected into the AI reasoning loop, giving the model the maximum possible
// situational awareness.
//
// Components:
//
//	ContextBundle       — versioned snapshot of all context for one planning cycle
//	ContextAssemblySkill — assembles a ContextBundle on demand
//	PrePlanContextCheck — middleware that validates bundle freshness before model call
//	MessageQueueInjector — middleware that polls a webhook event queue and injects
//	                       operator follow-up messages before the next model call
package context

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/abuxton/pheromone/internal/skill"
)

// ContextBundle is the versioned context snapshot assembled before each
// planning cycle (ADR-020 Context Engineering pattern).
//
// All fields are optional; a field with a zero value means "not available".
// The assembler sets AssembledAt and Version on every call.
type ContextBundle struct {
	// Version is an incrementing counter that uniquely identifies this snapshot.
	Version int `json:"version"`
	// AssembledAt is when this bundle was assembled (UTC).
	AssembledAt time.Time `json:"assembled_at"`
	// TwinID is the twin this bundle is scoped to.
	TwinID string `json:"twin_id"`
	// TwinDesiredState is the desired TwinModel for TwinID.
	TwinDesiredState *skill.TwinModel `json:"twin_desired_state,omitempty"`
	// TwinActualState is the most recently observed TwinModel for TwinID.
	TwinActualState *skill.TwinModel `json:"twin_actual_state,omitempty"`
	// DriftReport is the pre-computed drift between desired and actual state.
	DriftReport *skill.DriftReport `json:"drift_report,omitempty"`
	// RecentActionHistory lists the last N action types executed for TwinID.
	RecentActionHistory []string `json:"recent_action_history,omitempty"`
	// AgentsMDContent is the content of the AGENTS.md file injected for context.
	AgentsMDContent string `json:"agents_md_content,omitempty"`
	// InstanceProfile captures key instance attributes (OS, packages, services).
	InstanceProfile map[string]string `json:"instance_profile,omitempty"`
	// FollowUpMessages are operator messages injected by MessageQueueInjector.
	FollowUpMessages []string `json:"follow_up_messages,omitempty"`
	// Stale is true when the bundle could not be freshly assembled and a prior
	// version was returned under the stale-OK fallback policy.
	Stale bool `json:"stale"`
}

// IsComplete returns true when all critical fields are populated.
// Used by PrePlanContextCheck to decide whether to retry assembly.
func (b *ContextBundle) IsComplete() bool {
	return b != nil && b.TwinDesiredState != nil && b.TwinActualState != nil
}

// ---------------------------------------------------------------------------
// AgentsMDReader abstracts AGENTS.md loading for testability.
// ---------------------------------------------------------------------------

// AgentsMDReader loads architectural guidance text for injection into bundles.
type AgentsMDReader interface {
	ReadAgentsMD() (string, error)
}

// FileAgentsMDReader reads AGENTS.md from a file path.
type FileAgentsMDReader struct{ Path string }

// ReadAgentsMD reads the file at r.Path. Returns empty string on error (not fatal).
func (r *FileAgentsMDReader) ReadAgentsMD() (string, error) {
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// ContextAssemblySkill
// ---------------------------------------------------------------------------

// ContextAssemblySkill assembles a ContextBundle for a twin before each
// planning cycle (ADR-020 Phase 2b). It satisfies the skill.Skill interface
// and can be invoked by the framework's Execute dispatch path.
//
// Supported action types:
//   - "assemble" — build a fresh ContextBundle for action.TwinID
//
// The skill stores the most recent bundle per twin so that PrePlanContextCheck
// middleware can retrieve it without triggering a new assembly.
type ContextAssemblySkill struct {
	agentsMD AgentsMDReader
	timeout  time.Duration
	log      *slog.Logger

	mu      sync.RWMutex
	bundles map[string]*ContextBundle // keyed by twin ID
	seq     int                       // monotonic version counter
}

// NewContextAssemblySkill creates a ContextAssemblySkill.
//
// agentsMD may be nil (AGENTS.md injection is skipped when nil).
// timeout is the maximum duration for bundle assembly (default: 500 ms).
func NewContextAssemblySkill(agentsMD AgentsMDReader, timeout time.Duration) *ContextAssemblySkill {
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &ContextAssemblySkill{
		agentsMD: agentsMD,
		timeout:  timeout,
		log:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		bundles:  make(map[string]*ContextBundle),
	}
}

func (s *ContextAssemblySkill) Name() string                  { return "context-assembly" }
func (s *ContextAssemblySkill) Version() string               { return "1.0.0" }
func (s *ContextAssemblySkill) MinTwinLevel() skill.TwinLevel { return skill.TwinLevelOS }

// Execute assembles a ContextBundle for action.TwinID.
func (s *ContextAssemblySkill) Execute(ctx context.Context, obs *skill.Observations, action *skill.Action) (*skill.SkillResult, error) {
	if action.ActionType != "assemble" {
		return nil, fmt.Errorf("context-assembly: unknown action type %q", action.ActionType)
	}
	twinID := action.TwinID
	if twinID == "" {
		return nil, fmt.Errorf("context-assembly: assemble requires a non-empty TwinID")
	}

	assembleCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	bundle, stale := s.assemble(assembleCtx, twinID, obs)
	if stale {
		s.log.Warn("context-assembly: returning stale bundle",
			slog.String("twin_id", twinID),
			slog.Int("version", bundle.Version),
		)
	}

	s.mu.Lock()
	s.bundles[twinID] = bundle
	s.mu.Unlock()

	return &skill.SkillResult{
		Success: true,
		Detail: fmt.Sprintf("context-assembly: assembled bundle v%d for twin %q (stale=%v, complete=%v)",
			bundle.Version, twinID, bundle.Stale, bundle.IsComplete()),
	}, nil
}

// GetBundle returns the most recently assembled bundle for twinID, or nil if
// no bundle has been assembled yet.
func (s *ContextAssemblySkill) GetBundle(twinID string) *ContextBundle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bundles[twinID]
}

// assemble builds a ContextBundle from the provided observations.
// If the context expires during assembly, it returns a stale bundle.
func (s *ContextAssemblySkill) assemble(ctx context.Context, twinID string, obs *skill.Observations) (bundle *ContextBundle, stale bool) {
	s.mu.Lock()
	s.seq++
	version := s.seq
	prior := s.bundles[twinID]
	s.mu.Unlock()

	b := &ContextBundle{
		Version:     version,
		AssembledAt: time.Now().UTC(),
		TwinID:      twinID,
	}

	// Populate desired and actual twin state from observations.
	for _, d := range obs.DesiredTwins {
		if d.TwinID == twinID {
			b.TwinDesiredState = d
			break
		}
	}
	for _, a := range obs.Twins {
		if a.TwinID == twinID {
			b.TwinActualState = a
			break
		}
	}

	// Compute drift if both states are available.
	if b.TwinDesiredState != nil && b.TwinActualState != nil {
		b.DriftReport = computeDrift(b.TwinDesiredState, b.TwinActualState)
	}

	// Inject AGENTS.md content (non-fatal if unavailable).
	if s.agentsMD != nil {
		if content, err := s.agentsMD.ReadAgentsMD(); err == nil {
			b.AgentsMDContent = content
		}
	}

	// Check context expiry.
	select {
	case <-ctx.Done():
		// Assembly timed out. Return prior bundle as stale fallback.
		if prior != nil {
			prior.Stale = true
			return prior, true
		}
		b.Stale = true
		return b, true
	default:
	}

	return b, false
}

// computeDrift performs a field-level diff between desired and actual twin models.
func computeDrift(desired, actual *skill.TwinModel) *skill.DriftReport {
	report := &skill.DriftReport{TwinID: desired.TwinID}
	for k, dv := range desired.State {
		av, ok := actual.State[k]
		if !ok || av != dv {
			report.HasDrift = true
			report.DriftedFields = append(report.DriftedFields, skill.FieldDrift{
				Field:   k,
				Desired: dv,
				Actual:  av,
			})
		}
	}
	return report
}

// ---------------------------------------------------------------------------
// MessageQueue — injectable event queue for MessageQueueInjector
// ---------------------------------------------------------------------------

// MessageQueue is the interface for the operator follow-up message queue.
// Implementations range from an in-process channel to a NATS JetStream consumer.
type MessageQueue interface {
	// Poll returns all pending messages without blocking.
	// Returns an empty slice when no messages are available.
	Poll() []string
}

// InMemoryMessageQueue is a simple, goroutine-safe in-process queue.
// Used in tests and for environments without NATS.
type InMemoryMessageQueue struct {
	mu   sync.Mutex
	msgs []string
}

// Push enqueues a message.
func (q *InMemoryMessageQueue) Push(msg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.msgs = append(q.msgs, msg)
}

// Poll returns and drains all pending messages.
func (q *InMemoryMessageQueue) Poll() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]string, len(q.msgs))
	copy(out, q.msgs)
	q.msgs = q.msgs[:0]
	return out
}

// ---------------------------------------------------------------------------
// PrePlanContextCheck middleware
// ---------------------------------------------------------------------------

// PrePlanContextCheck validates that a ContextBundle for the target twin exists
// and is complete before allowing the reasoning model to be invoked.
//
// If the bundle is absent or incomplete, it calls assembler.Execute to request
// a fresh assembly before returning.
//
// Usage in the framework: wrap the ReasonFunc.
type PrePlanContextCheck struct {
	Assembler *ContextAssemblySkill
	MaxAge    time.Duration // bundles older than MaxAge are re-assembled
}

// NewPrePlanContextCheck creates a PrePlanContextCheck middleware.
// maxAge 0 defaults to 30 seconds.
func NewPrePlanContextCheck(assembler *ContextAssemblySkill, maxAge time.Duration) *PrePlanContextCheck {
	if maxAge <= 0 {
		maxAge = 30 * time.Second
	}
	return &PrePlanContextCheck{Assembler: assembler, MaxAge: maxAge}
}

// Check validates the bundle for twinID and triggers reassembly if stale or absent.
// Returns the validated (possibly freshly assembled) bundle.
func (m *PrePlanContextCheck) Check(ctx context.Context, twinID string, obs *skill.Observations) (*ContextBundle, error) {
	bundle := m.Assembler.GetBundle(twinID)
	needsRefresh := bundle == nil ||
		bundle.Stale ||
		!bundle.IsComplete() ||
		time.Since(bundle.AssembledAt) > m.MaxAge

	if needsRefresh {
		_, err := m.Assembler.Execute(ctx, obs, &skill.Action{
			ActionType: "assemble",
			TwinID:     twinID,
		})
		if err != nil {
			return nil, fmt.Errorf("pre-plan-context-check: assembly failed for twin %q: %w", twinID, err)
		}
		bundle = m.Assembler.GetBundle(twinID)
	}
	return bundle, nil
}

// ---------------------------------------------------------------------------
// MessageQueueInjector middleware
// ---------------------------------------------------------------------------

// MessageQueueInjector polls the operator message queue before each model call
// and injects any pending follow-up messages into the ContextBundle for twinID.
//
// This implements OpenSWE's "inject queue messages before model call" pattern.
type MessageQueueInjector struct {
	queue     MessageQueue
	assembler *ContextAssemblySkill
	log       *slog.Logger
}

// NewMessageQueueInjector creates a MessageQueueInjector.
func NewMessageQueueInjector(queue MessageQueue, assembler *ContextAssemblySkill) *MessageQueueInjector {
	return &MessageQueueInjector{
		queue:     queue,
		assembler: assembler,
		log:       slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// Inject polls pending messages and appends them to the stored bundle for twinID.
// If no bundle exists for twinID, Inject is a no-op (the messages are dropped).
// Returns the number of messages injected.
func (m *MessageQueueInjector) Inject(twinID string) int {
	msgs := m.queue.Poll()
	if len(msgs) == 0 {
		return 0
	}

	m.assembler.mu.Lock()
	defer m.assembler.mu.Unlock()

	bundle, ok := m.assembler.bundles[twinID]
	if !ok {
		m.log.Warn("message-queue-injector: no bundle for twin; messages dropped",
			slog.String("twin_id", twinID),
			slog.Int("dropped", len(msgs)),
		)
		return 0
	}
	bundle.FollowUpMessages = append(bundle.FollowUpMessages, msgs...)
	m.log.Info("message-queue-injector: injected messages",
		slog.String("twin_id", twinID),
		slog.Int("count", len(msgs)),
	)
	return len(msgs)
}
