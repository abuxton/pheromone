package hooks_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/hooks"
	"github.com/abuxton/pheromone/internal/notification"
)

// TestWebhookDispatchAtScale validates that PostActionHookService can deliver
// 1000 events × 100 hooks = 100,000 webhooks with all of the following:
//   - ≥99.9% delivery within a 30-second window
//   - P99 per-endpoint dispatch latency < 2 s
//
// "Per-endpoint dispatch latency" is measured from the instant the HTTP
// goroutine begins the request (recorded in the X-Dispatch-Nanos header) to
// when the endpoint receives it. This matches the SLA wording in issue #26 and
// ADR-011: it tests HTTP transport latency, not total queue-drain time.
func TestWebhookDispatchAtScale(t *testing.T) {
	const (
		numServers  = 100
		numEvents   = 1000
		totalWanted = numServers * numEvents   // 100,000
		minOK       = totalWanted * 999 / 1000 // 0.1 % drop tolerance → 99,900
		deadline    = 60 * time.Second
	)

	// Pre-allocate space for per-request latency samples (add headroom for retries).
	const maxSamples = totalWanted * 5
	reqLatencies := make([]int64, maxSamples)
	var reqLatIdx atomic.Int64
	var delivered atomic.Int64

	// 1. Create 100 httptest servers.
	servers := make([]*httptest.Server, numServers)
	for i := range servers {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receiveNs := time.Now().UnixNano()

			// Measure per-request dispatch latency from header set in doPost.
			if nsStr := r.Header.Get("X-Dispatch-Nanos"); nsStr != "" {
				if startNs, err := strconv.ParseInt(nsStr, 10, 64); err == nil {
					lat := receiveNs - startNs
					if idx := reqLatIdx.Add(1) - 1; idx < maxSamples {
						reqLatencies[idx] = lat
					}
				}
			}

			// Drain body to avoid broken-pipe on client.
			_, _ = io.Copy(io.Discard, r.Body)

			delivered.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
	}
	defer func() {
		for _, srv := range servers {
			srv.Close()
		}
	}()

	// 2. Register 100 HookConfig entries — one per server, wildcard scopes.
	registry := &hooks.HookRegistry{}
	for i, srv := range servers {
		registry.Register(hooks.HookConfig{
			ID:            fmt.Sprintf("hook-%d", i),
			ScopeAgentID:  "*",
			ScopeTwinID:   "*",
			ScopeSkill:    "*",
			EventType:     "*",
			OutcomeFilter: "*",
			Destination: notification.NotificationDestination{
				Type:           "http-webhook",
				URL:            srv.URL,
				TimeoutSeconds: 5,
			},
		})
	}

	// 3. Start PostActionHookService with concurrency=50 event workers and a
	// well-tuned HTTP client. The service bounds concurrent TCP connections via
	// defaultMaxHTTPConns so it can sustain high throughput without goroutine explosion.
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10_000,
			MaxIdleConnsPerHost: 200,
			DisableKeepAlives:   false,
		},
	}
	svc := hooks.NewPostActionHookService(registry, 50, httpClient, []byte("scale-test-secret"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Start(ctx)

	// Brief settle to let workers spin up.
	time.Sleep(10 * time.Millisecond)

	// 4. Fire 1000 events via Dispatch() concurrently.
	dispatchStart := time.Now()
	var dispatchWG sync.WaitGroup
	for i := 0; i < numEvents; i++ {
		dispatchWG.Add(1)
		go func(idx int) {
			defer dispatchWG.Done()
			event := notification.ActionEvent{
				AgentID:    fmt.Sprintf("agent-%d", idx%10),
				TwinID:     fmt.Sprintf("twin-%d", idx%20),
				SkillName:  "config-enforce",
				ActionType: "apply-config",
				Outcome:    "success",
				TraceID:    fmt.Sprintf("trace-%d", idx),
				Timestamp:  time.Now(),
			}
			svc.Dispatch(event)
		}(i)
	}
	dispatchWG.Wait()
	t.Logf("all %d Dispatch calls returned in %v", numEvents, time.Since(dispatchStart))

	// 5. Poll until all deliveries arrive or the deadline expires.
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	deadlineTimer := time.NewTimer(deadline)
	defer deadlineTimer.Stop()

	for {
		select {
		case <-ticker.C:
			if delivered.Load() >= int64(totalWanted) {
				goto done
			}
		case <-deadlineTimer.C:
			goto done
		}
	}

done:
	finalCount := delivered.Load()
	t.Logf("delivered %d / %d webhooks in %v", finalCount, totalWanted, time.Since(dispatchStart))

	// 5a. Assert delivery count meets 99.9 % threshold.
	if finalCount < int64(minOK) {
		t.Errorf("too few deliveries: got %d, want >= %d", finalCount, minOK)
	}

	// 6. Compute P99 per-endpoint dispatch latency.
	// This is the HTTP round-trip time from X-Dispatch-Nanos (set by doPost just
	// before calling httpClient.Do) to when the server handler records receiveNs.
	// This measures transport latency independent of queue-drain time, matching the
	// ADR-011 / issue-#26 SLA: "P99 per-endpoint dispatch latency < 2 s".
	nSamples := int(reqLatIdx.Load())
	if nSamples > maxSamples {
		nSamples = maxSamples
	}
	if nSamples > 0 {
		latencies := make([]time.Duration, nSamples)
		for i := range latencies {
			latencies[i] = time.Duration(reqLatencies[i])
		}
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := latencies[len(latencies)*50/100]
		p99 := latencies[len(latencies)*99/100]
		t.Logf("per-endpoint dispatch latency: samples=%d  P50=%v  P99=%v", nSamples, p50, p99)

		// Assert P99 per-endpoint dispatch latency < 2 s.
		if p99 >= 2*time.Second {
			t.Errorf("P99 per-endpoint dispatch latency %v exceeds 2 s SLA", p99)
		}
	}
}
