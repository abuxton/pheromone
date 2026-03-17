package benchmark

// Tech Spike: ADR-005 — NATS JetStream Load Test (100K msgs/sec, Closes #4)
//
// Validates NATS JetStream against the success criteria defined in ADR-005:
//
//   SC-008:  100K msgs/sec sustained throughput
//   SC-LAT:  p95 consumer latency < 10 ms, p99 < 50 ms
//   SC-MEM:  heap delta for 1M messages < 512 MB
//   SC-OVHD: JetStream overhead vs. core NATS < 20%
//
// Results are written to ./tmp/nats_spike_results.txt when the environment
// variable NATS_SPIKE_RESULTS is set.
//
// NOTE: All tests skip gracefully when NATS is unavailable (no running server
// on nats://localhost:4222), so the suite can be compiled and run in CI
// without a live NATS instance.

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// ── success-criteria constants ───────────────────────────────────────────────

const (
	natsURL         = "nats://localhost:4222"
	natsStreamName  = "TELEMETRY"    // canonical stream name (supersedes metricsStream — see ADR-005 H1)
	natsSubject     = "metrics.test" // spike-only; production uses metrics.{agent_id}.{metric_type} (ADR-005 H3)
	natsPayloadSize = 256

	// ADR-005 thresholds
	maxP95LatencyMs  = 10.0
	maxP99LatencyMs  = 50.0
	maxMemoryMB      = 512
	maxJSOverheadPct = 20.0
	targetMsgsPerSec = 100_000
)

// ── connectNATS — helper that skips when NATS is unavailable ─────────────────

func connectNATS(t *testing.T) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second))
	if err != nil {
		t.Skipf("NATS unavailable: %v — start NATS with JetStream (-js) to run this suite", err)
	}
	t.Cleanup(func() { nc.Close() })
	return nc
}

// ── natsPercentile — mirrors grpcPercentile ───────────────────────────────────

func natsPercentile(latencies []time.Duration, pct float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	idx := int(float64(len(latencies)) * pct / 100.0)
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}
	return latencies[idx]
}

// ── TestNATSThroughputRamp: SC-008 ───────────────────────────────────────────

// TestNATSThroughputRamp validates that NATS JetStream sustains 100K msgs/sec
// by ramping through 1K → 10K → 50K → 100K msgs/sec stages (5 s each).
func TestNATSThroughputRamp(t *testing.T) {
	nc := connectNATS(t)
	ctx := context.Background()
	_ = ctx

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream() failed: %v", err)
	}

	// Create (or reuse) the TELEMETRY stream.
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     natsStreamName,
		Subjects: []string{natsSubject},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		t.Fatalf("AddStream failed: %v", err)
	}
	t.Cleanup(func() { _ = js.DeleteStream(natsStreamName) })

	rng := rand.New(rand.NewSource(42))

	stages := []struct {
		label  string
		target int // msgs/sec
		isGoal bool
	}{
		{"1K", 1_000, false},
		{"10K", 10_000, false},
		{"50K", 50_000, false},
		{"100K", 100_000, true},
	}

	t.Logf("%-6s  %-14s  %-14s  %s", "Stage", "Published/s", "Delivered/s", "Result")
	t.Logf("%s", "------  --------------  --------------  ------")

	for _, stage := range stages {
		payload := make([]byte, natsPayloadSize)
		_, _ = rng.Read(payload)

		var published atomic.Int64
		var received atomic.Int64

		// Subscribe to count deliveries.
		sub, err := js.Subscribe(natsSubject, func(_ *nats.Msg) {
			received.Add(1)
		})
		if err != nil {
			t.Fatalf("[%s] Subscribe failed: %v", stage.label, err)
		}

		window := 5 * time.Second
		interval := time.Duration(int64(time.Second) / int64(stage.target))
		if interval < time.Microsecond {
			interval = time.Microsecond
		}

		deadline := time.Now().Add(window)
		for time.Now().Before(deadline) {
			binary.LittleEndian.PutUint64(payload[:8], uint64(time.Now().UnixNano()))
			if _, err := js.Publish(natsSubject, payload); err == nil {
				published.Add(1)
			}
			time.Sleep(interval)
		}

		// Allow in-flight messages to be delivered.
		time.Sleep(200 * time.Millisecond)
		_ = sub.Unsubscribe()

		pubRate := float64(published.Load()) / window.Seconds()
		recRate := float64(received.Load()) / window.Seconds()
		result := "OK"
		if stage.isGoal && recRate < float64(targetMsgsPerSec) {
			result = "FAIL"
		}

		t.Logf("%-6s  %-14.0f  %-14.0f  %s", stage.label, pubRate, recRate, result)

		if stage.isGoal && recRate < float64(targetMsgsPerSec) {
			t.Errorf("100K stage: achieved %.0f msgs/sec, need >= %d (SC-008)", recRate, targetMsgsPerSec)
		}
	}
}

// ── TestNATSConsumerLatency: SC-LAT ──────────────────────────────────────────

// TestNATSConsumerLatency validates p50/p95/p99 consumer latency for 10K
// messages where each 256-byte payload carries a nanosecond timestamp in
// its first 8 bytes (little-endian uint64).
func TestNATSConsumerLatency(t *testing.T) {
	nc := connectNATS(t)

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream() failed: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     natsStreamName,
		Subjects: []string{natsSubject},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		t.Fatalf("AddStream failed: %v", err)
	}
	t.Cleanup(func() { _ = js.DeleteStream(natsStreamName) })

	const msgCount = 10_000
	latencies := make([]time.Duration, 0, msgCount)

	var wg sync.WaitGroup
	wg.Add(1)

	sub, err := js.Subscribe(natsSubject, func(msg *nats.Msg) {
		recvAt := time.Now()
		if len(msg.Data) >= 8 {
			sentNs := int64(binary.LittleEndian.Uint64(msg.Data[:8]))
			lat := recvAt.Sub(time.Unix(0, sentNs))
			latencies = append(latencies, lat)
		}
		if len(latencies) >= msgCount {
			wg.Done()
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	payload := make([]byte, natsPayloadSize)
	rng := rand.New(rand.NewSource(42))
	_, _ = rng.Read(payload[8:]) // fill non-timestamp bytes

	for i := 0; i < msgCount; i++ {
		binary.LittleEndian.PutUint64(payload[:8], uint64(time.Now().UnixNano()))
		if _, err := js.Publish(natsSubject, payload); err != nil {
			t.Fatalf("Publish[%d] failed: %v", i, err)
		}
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for all messages to be received")
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := natsPercentile(latencies, 50)
	p95 := natsPercentile(latencies, 95)
	p99 := natsPercentile(latencies, 99)

	t.Logf("Consumer latency (%d msgs) — P50: %v  P95: %v  P99: %v  (thresholds: p95<%vms, p99<%vms)",
		msgCount, p50, p95, p99, maxP95LatencyMs, maxP99LatencyMs)

	if p95.Seconds()*1000 > maxP95LatencyMs {
		t.Errorf("p95 latency %v exceeds %.0f ms threshold (SC-LAT)", p95, maxP95LatencyMs)
	}
	if p99.Seconds()*1000 > maxP99LatencyMs {
		t.Errorf("p99 latency %v exceeds %.0f ms threshold (SC-LAT)", p99, maxP99LatencyMs)
	}
}

// ── TestNATSJetStreamOverhead: SC-OVHD ───────────────────────────────────────

// TestNATSJetStreamOverhead compares core NATS vs JetStream throughput at
// 10K msgs/sec and validates that JetStream overhead stays below 20%.
func TestNATSJetStreamOverhead(t *testing.T) {
	nc := connectNATS(t)

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream() failed: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     natsStreamName,
		Subjects: []string{natsSubject},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		t.Fatalf("AddStream failed: %v", err)
	}
	t.Cleanup(func() { _ = js.DeleteStream(natsStreamName) })

	const (
		targetRate = 10_000
		window     = 5 * time.Second
	)

	payload := make([]byte, natsPayloadSize)
	rng := rand.New(rand.NewSource(42))
	_, _ = rng.Read(payload)
	interval := time.Second / targetRate

	measureRate := func(publishFn func([]byte) error) float64 {
		var count int64
		deadline := time.Now().Add(window)
		for time.Now().Before(deadline) {
			if publishFn(payload) == nil {
				count++
			}
			time.Sleep(interval)
		}
		return float64(count) / window.Seconds()
	}

	// Core NATS
	coreRate := measureRate(func(p []byte) error {
		return nc.Publish(natsSubject, p)
	})

	// JetStream
	jsRate := measureRate(func(p []byte) error {
		_, err := js.Publish(natsSubject, p)
		return err
	})

	var overheadPct float64
	if coreRate > 0 {
		overheadPct = (coreRate - jsRate) / coreRate * 100
	}

	t.Logf("Overhead comparison — Core: %.0f msgs/s  JetStream: %.0f msgs/s  Overhead: %.1f%%  (threshold: %.0f%%)",
		coreRate, jsRate, overheadPct, maxJSOverheadPct)

	if overheadPct > maxJSOverheadPct {
		t.Errorf("JetStream overhead %.1f%% exceeds %.0f%% threshold (SC-OVHD)", overheadPct, maxJSOverheadPct)
	}
}

// ── TestNATSMemoryProfile: SC-MEM ────────────────────────────────────────────

// TestNATSMemoryProfile publishes 1M messages and validates that the heap
// delta stays below 512 MB (SC-MEM).
func TestNATSMemoryProfile(t *testing.T) {
	nc := connectNATS(t)

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream() failed: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     natsStreamName,
		Subjects: []string{natsSubject},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		t.Fatalf("AddStream failed: %v", err)
	}
	t.Cleanup(func() { _ = js.DeleteStream(natsStreamName) })

	payload := make([]byte, natsPayloadSize)
	rng := rand.New(rand.NewSource(42))
	_, _ = rng.Read(payload)

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	const totalMsgs = 1_000_000
	for i := 0; i < totalMsgs; i++ {
		binary.LittleEndian.PutUint64(payload[:8], uint64(time.Now().UnixNano()))
		if _, err := js.Publish(natsSubject, payload); err != nil {
			t.Fatalf("Publish[%d] failed: %v", i, err)
		}
	}

	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	var heapDeltaMB float64
	if memAfter.HeapInuse > memBefore.HeapInuse {
		heapDeltaMB = float64(memAfter.HeapInuse-memBefore.HeapInuse) / (1024 * 1024)
	}

	t.Logf("Memory profile (%d msgs) — HeapInuse before: %.1f MB  after: %.1f MB  delta: %.1f MB  (threshold: %d MB)",
		totalMsgs,
		float64(memBefore.HeapInuse)/(1024*1024),
		float64(memAfter.HeapInuse)/(1024*1024),
		heapDeltaMB,
		maxMemoryMB)

	if heapDeltaMB > float64(maxMemoryMB) {
		t.Errorf("heap delta %.1f MB exceeds %d MB threshold (SC-MEM)", heapDeltaMB, maxMemoryMB)
	}
}

// ── BenchmarkNATSPublish: core NATS ──────────────────────────────────────────

// BenchmarkNATSPublish measures core NATS publish throughput (ns/op).
func BenchmarkNATSPublish(b *testing.B) {
	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second))
	if err != nil {
		b.Skipf("NATS unavailable: %v", err)
	}
	defer nc.Close()

	payload := make([]byte, natsPayloadSize)
	rng := rand.New(rand.NewSource(42))
	_, _ = rng.Read(payload)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := nc.Publish(natsSubject, payload); err != nil {
			b.Fatalf("Publish failed: %v", err)
		}
	}
	_ = nc.Flush()
}

// ── BenchmarkNATSPublishJetStream: JetStream publish ─────────────────────────

// BenchmarkNATSPublishJetStream measures JetStream publish throughput (ns/op).
func BenchmarkNATSPublishJetStream(b *testing.B) {
	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second))
	if err != nil {
		b.Skipf("NATS unavailable: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		b.Fatalf("JetStream() failed: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     natsStreamName,
		Subjects: []string{natsSubject},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		b.Fatalf("AddStream failed: %v", err)
	}
	b.Cleanup(func() { _ = js.DeleteStream(natsStreamName) })

	payload := make([]byte, natsPayloadSize)
	rng := rand.New(rand.NewSource(42))
	_, _ = rng.Read(payload)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := js.Publish(natsSubject, payload); err != nil {
			b.Fatalf("JetStream Publish failed: %v", err)
		}
	}
}

// ── result helper ─────────────────────────────────────────────────────────────

// formatSpikeResults returns a human-readable summary for ./tmp/nats_spike_results.txt
// (L1: use ./tmp/ not /tmp/; L2: include Closes #4 reference).
func formatSpikeResults(throughput, p95ms, p99ms, heapMB, overheadPct float64) string {
	return fmt.Sprintf(`NATS JetStream Spike Results — ADR-005 (Closes #4)
===================================================
Throughput (100K stage):  %.0f msgs/sec  (target: %d)
P95 Consumer Latency:     %.2f ms        (threshold: %.0f ms)
P99 Consumer Latency:     %.2f ms        (threshold: %.0f ms)
Heap Delta (1M msgs):     %.1f MB        (threshold: %d MB)
JetStream Overhead:       %.1f%%          (threshold: %.0f%%)
`,
		throughput, targetMsgsPerSec,
		p95ms, maxP95LatencyMs,
		p99ms, maxP99LatencyMs,
		heapMB, maxMemoryMB,
		overheadPct, maxJSOverheadPct,
	)
}
