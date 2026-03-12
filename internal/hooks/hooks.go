// Package hooks implements the PostActionHookService defined in ADR-011.
//
// Architecture (two-tier FIFO):
//   - concurrency event-worker goroutines pull ActionEvents from the event queue.
//   - Each event worker resolves matching hooks and enqueues one hookTask per
//     destination into a shared task queue — this is non-blocking and fast.
//   - concurrency×httpWorkerMultiplier HTTP workers drain the task queue,
//     each making exactly one HTTP POST per dequeued task (no retries while
//     holding a worker slot to prevent throughput collapse under errors).
//
// This design keeps goroutine count predictable and avoids head-of-line
// blocking across independent events.
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/abuxton/pheromone/internal/notification"
)

const (
	defaultConcurrency   = 50
	defaultQueueCap      = 10_000
	defaultTaskQueueCap  = 200_000
	httpWorkerMultiplier = 20 // HTTP workers = concurrency × this; default 1000
	defaultMaxRetries    = 3
	defaultTimeout       = 10 * time.Second
)

// hookTask is an enqueued delivery work item.
type hookTask struct {
	hookID  string
	dest    notification.NotificationDestination
	payload []byte
	traceID string
}

// HookConfig describes a single hook registration entry.
//
// Scope fields accept an exact value or the wildcard "*" which matches any field.
type HookConfig struct {
	// ID is a unique identifier for this hook.
	ID string
	// ScopeAgentID matches against ActionEvent.AgentID ("*" = any).
	ScopeAgentID string
	// ScopeTwinID matches against ActionEvent.TwinID ("*" = any).
	ScopeTwinID string
	// ScopeSkill matches against ActionEvent.SkillName ("*" = any).
	ScopeSkill string
	// EventType is "action", "delivery", or "*".
	EventType string
	// OutcomeFilter is "success", "failure", "partial", or "*".
	OutcomeFilter string
	// Destination is where notifications are delivered.
	Destination notification.NotificationDestination
}

// HookRegistry is an in-memory, goroutine-safe store of HookConfig entries.
type HookRegistry struct {
	mu    sync.RWMutex
	hooks []HookConfig
}

// Register appends h to the registry.
func (r *HookRegistry) Register(h HookConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, h)
}

// List returns a snapshot of all registered hooks.
func (r *HookRegistry) List() []HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]HookConfig, len(r.hooks))
	copy(out, r.hooks)
	return out
}

// MatchingHooks returns all hooks whose scope filters match event.
func (r *HookRegistry) MatchingHooks(event notification.ActionEvent) []HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []HookConfig
	for _, h := range r.hooks {
		if matchesField(h.ScopeAgentID, event.AgentID) &&
			matchesField(h.ScopeTwinID, event.TwinID) &&
			matchesField(h.ScopeSkill, event.SkillName) &&
			matchesField(h.OutcomeFilter, event.Outcome) {
			out = append(out, h)
		}
	}
	return out
}

// matchesField returns true when filter is "*" or exactly equals value.
func matchesField(filter, value string) bool {
	return filter == "*" || filter == value
}

// PostActionHookService processes ActionEvents and fans out webhook deliveries.
type PostActionHookService struct {
	registry    *HookRegistry
	concurrency int
	httpClient  notification.HTTPClient
	secret      []byte
	queue       chan notification.ActionEvent
	taskQueue   chan hookTask
	log         *slog.Logger
}

// NewPostActionHookService creates a PostActionHookService.
//
// concurrency controls event-worker goroutine count (default 50 when ≤ 0).
// HTTP workers = concurrency × httpWorkerMultiplier (default 1000 total).
func NewPostActionHookService(
	registry *HookRegistry,
	concurrency int,
	httpClient notification.HTTPClient,
	secret []byte,
) *PostActionHookService {
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	return &PostActionHookService{
		registry:    registry,
		concurrency: concurrency,
		httpClient:  httpClient,
		secret:      secret,
		queue:       make(chan notification.ActionEvent, defaultQueueCap),
		taskQueue:   make(chan hookTask, defaultTaskQueueCap),
		log:         slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// Start launches event workers and HTTP workers. Blocks until ctx is cancelled.
// Callers should run Start in a dedicated goroutine.
func (s *PostActionHookService) Start(ctx context.Context) {
	var wg sync.WaitGroup

	// HTTP workers: drain the task queue and make HTTP POSTs.
	httpWorkers := s.concurrency * httpWorkerMultiplier
	for i := 0; i < httpWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runHTTPWorker(ctx)
		}()
	}

	// Event workers: resolve hooks and enqueue tasks.
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runEventWorker(ctx)
		}()
	}

	wg.Wait()
}

// runHTTPWorker pulls hookTasks and executes a single HTTP POST per task.
func (s *PostActionHookService) runHTTPWorker(ctx context.Context) {
	for {
		select {
		case task, ok := <-s.taskQueue:
			if !ok {
				return
			}
			if err := s.doPost(ctx, task.dest, task.payload); err != nil {
				s.log.Warn("hooks: dispatch failed",
					slog.String("hook_id", task.hookID),
					slog.String("url", task.dest.URL),
					slog.String("trace_id", task.traceID),
					slog.String("error", err.Error()),
				)
			}
		case <-ctx.Done():
			// Drain remaining tasks before exiting.
			for {
				select {
				case task := <-s.taskQueue:
					_ = s.doPost(ctx, task.dest, task.payload)
				default:
					return
				}
			}
		}
	}
}

// runEventWorker pulls events from the event queue and enqueues hook tasks.
func (s *PostActionHookService) runEventWorker(ctx context.Context) {
	for {
		select {
		case event, ok := <-s.queue:
			if !ok {
				return
			}
			s.fanOut(event)
		case <-ctx.Done():
			for {
				select {
				case event := <-s.queue:
					s.fanOut(event)
				default:
					return
				}
			}
		}
	}
}

// fanOut resolves matching hooks for event and enqueues one hookTask per match.
// Non-blocking: drops tasks and logs when the task queue is full.
func (s *PostActionHookService) fanOut(event notification.ActionEvent) {
	matched := s.registry.MatchingHooks(event)
	if len(matched) == 0 {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.Error("hooks: marshal event failed",
			slog.String("trace_id", event.TraceID),
			slog.String("error", err.Error()),
		)
		return
	}

	for _, h := range matched {
		task := hookTask{
			hookID:  h.ID,
			dest:    h.Destination,
			payload: payload,
			traceID: event.TraceID,
		}
		select {
		case s.taskQueue <- task:
		default:
			s.log.Warn("hooks: task queue full, dropping hook",
				slog.String("hook_id", h.ID),
				slog.String("trace_id", event.TraceID),
			)
		}
	}
}

// Dispatch enqueues event for asynchronous processing.
// Non-blocking: drops and logs a warning when the queue is full.
func (s *PostActionHookService) Dispatch(event notification.ActionEvent) {
	select {
	case s.queue <- event:
	default:
		s.log.Warn("hooks: queue full, dropping event",
			slog.String("agent_id", event.AgentID),
			slog.String("trace_id", event.TraceID),
		)
	}
}

// dispatchWithRetry attempts to POST payload to dest with exponential backoff.
// Intended for callers that can tolerate retries outside the hot path.
//
//nolint:unused // reserved for future callers that need retry-capable dispatch
func (s *PostActionHookService) dispatchWithRetry(ctx context.Context, dest notification.NotificationDestination, payload []byte) error {
	maxRetries := dest.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
		if err := s.doPost(ctx, dest, payload); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("all %d attempts failed: %w", maxRetries+1, lastErr)
}

// doPost performs a single HTTP POST of payload to dest with HMAC signature header.
func (s *PostActionHookService) doPost(ctx context.Context, dest notification.NotificationDestination, payload []byte) error {
	timeout := defaultTimeout
	if dest.TimeoutSeconds > 0 {
		timeout = time.Duration(dest.TimeoutSeconds) * time.Second
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, dest.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pheromone-Signature", notification.ComputeHMAC(payload, s.secret))
	req.Header.Set("X-Dispatch-Nanos", strconv.FormatInt(time.Now().UnixNano(), 10))
	for k, v := range dest.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("non-2xx status: %d", resp.StatusCode)
	}
	return nil
}
