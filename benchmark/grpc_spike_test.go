package benchmark

// Tech Spike: ADR-003 — gRPC Bidirectional Stream Prototype (1000 Agents)
//
// Validates the three gRPC services (AgentRegistry, TwinControl, TelemetryStream)
// against the success criteria defined in the ADR-003 spike issue:
//
//   SC-REG:  RegisterRequest/Heartbeat round-trip P99 < 100 ms
//   SC-002:  TwinControl config push latency < 5 seconds
//   SC-008:  TelemetryStream handles 100 K msgs/sec without message loss
//   SC-CONN: 1000 agents stream simultaneously without connection drops

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pbv1 "github.com/abuxton/pheromone/internal/proto/pheromone/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ── success-criteria constants ───────────────────────────────────────────────

const (
	spikeNumAgents      = 1000
	spikeMsgsPerAgent   = 100
	spikeP99Threshold   = 100 * time.Millisecond // SC-REG
	spikePushThreshold  = 5 * time.Second        // SC-002
	spikeThroughputGoal = 100_000                // SC-008: msgs/sec
)

// ── mock AgentRegistry ───────────────────────────────────────────────────────

type mockRegistryServer struct {
	pbv1.UnimplementedAgentRegistryServer
	mu     sync.RWMutex
	agents map[string]bool
}

func (s *mockRegistryServer) Register(_ context.Context, req *pbv1.RegisterRequest) (*pbv1.RegisterResponse, error) {
	s.mu.Lock()
	s.agents[req.AgentId] = true
	s.mu.Unlock()
	return &pbv1.RegisterResponse{
		ServerId:      "mock-server-v1",
		Accepted:      true,
		ConfigVersion: "v1",
	}, nil
}

func (s *mockRegistryServer) Heartbeat(_ context.Context, req *pbv1.HeartbeatRequest) (*pbv1.HeartbeatResponse, error) {
	s.mu.RLock()
	_ = s.agents[req.AgentId]
	s.mu.RUnlock()
	return &pbv1.HeartbeatResponse{
		ServerId:     "mock-server-v1",
		Acknowledged: true,
	}, nil
}

// ── mock TwinControl ─────────────────────────────────────────────────────────

type mockTwinControlServer struct {
	pbv1.UnimplementedTwinControlServer
	configPushDelay time.Duration
}

func (s *mockTwinControlServer) SyncTwinState(
	stream grpc.BidiStreamingServer[pbv1.TwinSyncRequest, pbv1.TwinSyncResponse],
) error {
	if s.configPushDelay > 0 {
		time.Sleep(s.configPushDelay)
	}
	// Push one config action so the client can measure push latency.
	if err := stream.Send(&pbv1.TwinSyncResponse{
		ServerId: "mock-server-v1",
		Actions: []*pbv1.ConfigAction{
			{
				ActionId:    "act-001",
				ActionType:  "apply-config",
				TwinId:      "twin-001",
				PayloadJson: []byte(`{"cpu_limit":8}`),
			},
		},
		DesiredConfigVersion: "v2",
	}); err != nil {
		return err
	}
	// Drain all incoming state reports until the client closes the stream.
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (s *mockTwinControlServer) ProposeAction(_ context.Context, _ *pbv1.ActionProposal) (*pbv1.ActionDecision, error) {
	return &pbv1.ActionDecision{
		Approved:   true,
		DecisionBy: "auto-approve",
	}, nil
}

// ── mock TelemetryStream ─────────────────────────────────────────────────────

type mockTelemetryServer struct {
	pbv1.UnimplementedTelemetryStreamServer
	totalMsgs atomic.Int64
}

func (s *mockTelemetryServer) StreamMetrics(
	stream grpc.BidiStreamingServer[pbv1.MetricsRequest, pbv1.MetricsResponse],
) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		count := int32(len(req.Metrics) + len(req.Logs))
		s.totalMsgs.Add(int64(count))
		if err := stream.Send(&pbv1.MetricsResponse{
			ServerId:     "mock-server-v1",
			AckedCount:   count,
			ReadyForNext: true,
		}); err != nil {
			return err
		}
	}
}

// ── shared test server setup ─────────────────────────────────────────────────

type spikeServer struct {
	registry  *mockRegistryServer
	twin      *mockTwinControlServer
	telemetry *mockTelemetryServer
	grpcSrv   *grpc.Server
	addr      string
}

func startSpikeServer(t *testing.T) *spikeServer {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := &spikeServer{
		registry:  &mockRegistryServer{agents: make(map[string]bool)},
		twin:      &mockTwinControlServer{},
		telemetry: &mockTelemetryServer{},
		grpcSrv:   grpc.NewServer(),
		addr:      lis.Addr().String(),
	}

	pbv1.RegisterAgentRegistryServer(s.grpcSrv, s.registry)
	pbv1.RegisterTwinControlServer(s.grpcSrv, s.twin)
	pbv1.RegisterTelemetryStreamServer(s.grpcSrv, s.telemetry)

	go func() {
		if err := s.grpcSrv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			// Server stopped; ignore.
		}
	}()

	t.Cleanup(func() { s.grpcSrv.GracefulStop() })
	return s
}

func newClientConn(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// ── calculatePercentile (mirrors etcd_bench_test.go helper) ─────────────────

func grpcPercentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * p / 100.0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// closeSendStream closes the client send-half of a stream.  Errors during
// teardown are intentionally ignored in test code because the stream body has
// already completed successfully; a CloseSend error here only indicates that
// the server already closed the stream first (normal for one-shot test streams).
type sendCloser interface{ CloseSend() error }

func closeSendStream(s sendCloser) {
	_ = s.CloseSend()
}

// ── TestGRPCRegistrationLatency: SC-REG ──────────────────────────────────────

// TestGRPCRegistrationLatency validates that Register and Heartbeat round-trip
// P99 latency stays below 100 ms under sequential load.
func TestGRPCRegistrationLatency(t *testing.T) {
	srv := startSpikeServer(t)
	conn := newClientConn(t, srv.addr)
	rc := pbv1.NewAgentRegistryClient(conn)
	ctx := context.Background()

	const samples = 200
	latencies := make([]time.Duration, 0, samples*2)

	for i := 0; i < samples; i++ {
		agentID := fmt.Sprintf("agent-%04d", i)

		// Register
		start := time.Now()
		resp, err := rc.Register(ctx, &pbv1.RegisterRequest{
			AgentId:   agentID,
			AgentType: "native-go",
			Hostname:  fmt.Sprintf("host-%04d", i),
		})
		latencies = append(latencies, time.Since(start))
		if err != nil {
			t.Fatalf("Register[%d] failed: %v", i, err)
		}
		if !resp.Accepted {
			t.Fatalf("Register[%d] rejected", i)
		}

		// Heartbeat
		start = time.Now()
		hResp, err := rc.Heartbeat(ctx, &pbv1.HeartbeatRequest{
			AgentId:         agentID,
			TimestampUnixMs: time.Now().UnixMilli(),
		})
		latencies = append(latencies, time.Since(start))
		if err != nil {
			t.Fatalf("Heartbeat[%d] failed: %v", i, err)
		}
		if !hResp.Acknowledged {
			t.Fatalf("Heartbeat[%d] not acknowledged", i)
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := grpcPercentile(latencies, 50)
	p99 := grpcPercentile(latencies, 99)

	t.Logf("Register/Heartbeat latency — P50: %v  P99: %v  (threshold: %v)", p50, p99, spikeP99Threshold)

	if p99 > spikeP99Threshold {
		t.Errorf("P99 latency %v exceeds %v threshold (SC-REG)", p99, spikeP99Threshold)
	}
}

// ── TestGRPCTwinControlPushLatency: SC-002 ───────────────────────────────────

// TestGRPCTwinControlPushLatency validates that the server can push a config
// action to a connected agent within 5 seconds (SC-002).
func TestGRPCTwinControlPushLatency(t *testing.T) {
	srv := startSpikeServer(t)
	conn := newClientConn(t, srv.addr)
	tc := pbv1.NewTwinControlClient(conn)
	ctx := context.Background()

	const samples = 50
	latencies := make([]time.Duration, 0, samples)

	for i := 0; i < samples; i++ {
		start := time.Now()
		stream, err := tc.SyncTwinState(ctx)
		if err != nil {
			t.Fatalf("SyncTwinState[%d] open failed: %v", i, err)
		}

		// Send an initial state report so the server can identify the agent.
		if err := stream.Send(&pbv1.TwinSyncRequest{
			AgentId: fmt.Sprintf("agent-%04d", i),
			ActualState: []*pbv1.Twin{
				{TwinId: "twin-001", TwinType: "os-level", ConfigVersion: "v1"},
			},
		}); err != nil {
			t.Fatalf("SyncTwinState[%d] send failed: %v", i, err)
		}

		// Block until the server pushes its first config action.
		resp, err := stream.Recv()
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("SyncTwinState[%d] recv failed: %v", i, err)
		}
		if len(resp.Actions) == 0 {
			t.Errorf("SyncTwinState[%d] expected at least one config action", i)
		}

		latencies = append(latencies, elapsed)
		closeSendStream(stream)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := grpcPercentile(latencies, 50)
	p99 := grpcPercentile(latencies, 99)

	t.Logf("TwinControl config-push latency — P50: %v  P99: %v  (threshold: %v)", p50, p99, spikePushThreshold)

	if p99 > spikePushThreshold {
		t.Errorf("TwinControl push P99 %v exceeds %v threshold (SC-002)", p99, spikePushThreshold)
	}
}

// ── TestGRPCTelemetryThroughput: SC-008 ──────────────────────────────────────

// TestGRPCTelemetryThroughput validates that the TelemetryStream service can
// handle >= 100 K messages/sec in aggregate across concurrent agents (SC-008).
func TestGRPCTelemetryThroughput(t *testing.T) {
	const throughputAgents = 100 // 100 goroutines × spikeMsgsPerAgent batches
	srv := startSpikeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		wg        sync.WaitGroup
		dropCount atomic.Int64
	)

	start := time.Now()

	for i := 0; i < throughputAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			conn, err := grpc.NewClient(srv.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				dropCount.Add(1)
				return
			}
			defer conn.Close()

			tc := pbv1.NewTelemetryStreamClient(conn)
			stream, err := tc.StreamMetrics(ctx)
			if err != nil {
				dropCount.Add(1)
				return
			}

			agentID := fmt.Sprintf("agent-%04d", idx)
			for j := 0; j < spikeMsgsPerAgent; j++ {
				now := time.Now().UnixMilli()
				req := &pbv1.MetricsRequest{
					AgentId: agentID,
					TraceId: fmt.Sprintf("trace-%d-%d", idx, j),
					Metrics: []*pbv1.Metric{
						{Name: "cpu_usage_percent", Value: float64(rand.Intn(100)), TimestampUnixMs: now, Labels: map[string]string{"host": agentID}},
						{Name: "mem_usage_bytes", Value: float64(rand.Intn(1 << 30)), TimestampUnixMs: now, Labels: map[string]string{"host": agentID}},
						{Name: "disk_io_bytes", Value: float64(rand.Intn(1 << 20)), TimestampUnixMs: now, Labels: map[string]string{"host": agentID}},
						{Name: "net_rx_bytes", Value: float64(rand.Intn(1 << 20)), TimestampUnixMs: now, Labels: map[string]string{"host": agentID}},
						{Name: "net_tx_bytes", Value: float64(rand.Intn(1 << 20)), TimestampUnixMs: now, Labels: map[string]string{"host": agentID}},
					},
				}
				if err := stream.Send(req); err != nil {
					dropCount.Add(1)
					return
				}
				// Consume the server acknowledgement to enforce backpressure.
				ack, err := stream.Recv()
				if err != nil {
					dropCount.Add(1)
					return
				}
				if !ack.ReadyForNext {
					// Server is applying backpressure; honour it.
					time.Sleep(time.Millisecond)
				}
			}
			closeSendStream(stream)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalMsgs := srv.telemetry.totalMsgs.Load()
	throughput := float64(totalMsgs) / elapsed.Seconds()
	drops := dropCount.Load()

	t.Logf("TelemetryStream throughput — %d msgs in %v = %.0f msgs/sec  drops=%d  (goal: %d msgs/sec)",
		totalMsgs, elapsed.Round(time.Millisecond), throughput, drops, spikeThroughputGoal)

	if drops > 0 {
		t.Errorf("TelemetryStream dropped %d agent connections (SC-008: no message loss)", drops)
	}
	if throughput < spikeThroughputGoal {
		t.Errorf("TelemetryStream throughput %.0f msgs/sec is below %d msgs/sec goal (SC-008)",
			throughput, spikeThroughputGoal)
	}
}

// ── TestGRPC1000AgentsConcurrent: SC-CONN ────────────────────────────────────

// TestGRPC1000AgentsConcurrent spawns 1000 concurrent simulated agents and
// verifies that every agent can register, stream twin state, and stream
// telemetry without a connection drop.
func TestGRPC1000AgentsConcurrent(t *testing.T) {
	srv := startSpikeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	type result struct {
		agentID string
		err     string
	}

	results := make(chan result, spikeNumAgents)
	var wg sync.WaitGroup

	memBefore := currentMemMB()

	for i := 0; i < spikeNumAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			agentID := fmt.Sprintf("agent-%06d", idx)
			conn, err := grpc.NewClient(srv.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				results <- result{agentID, fmt.Sprintf("dial: %v", err)}
				return
			}
			defer conn.Close()

			// 1. Register
			rc := pbv1.NewAgentRegistryClient(conn)
			regResp, err := rc.Register(ctx, &pbv1.RegisterRequest{
				AgentId:   agentID,
				AgentType: "native-go",
				Hostname:  fmt.Sprintf("host-%06d", idx),
				TwinIds:   []string{fmt.Sprintf("twin-%06d", idx)},
			})
			if err != nil {
				results <- result{agentID, fmt.Sprintf("register: %v", err)}
				return
			}
			if !regResp.Accepted {
				results <- result{agentID, "register: rejected"}
				return
			}

			// 2. TwinControl bidirectional stream (send state, receive config push)
			tc := pbv1.NewTwinControlClient(conn)
			twinStream, err := tc.SyncTwinState(ctx)
			if err != nil {
				results <- result{agentID, fmt.Sprintf("SyncTwinState open: %v", err)}
				return
			}
			if err := twinStream.Send(&pbv1.TwinSyncRequest{
				AgentId: agentID,
				ActualState: []*pbv1.Twin{
					{
						TwinId:        fmt.Sprintf("twin-%06d", idx),
						TwinType:      "os-level",
						ConfigVersion: "v1",
						StateJson:     []byte(`{"uptime":3600}`),
					},
				},
			}); err != nil {
				results <- result{agentID, fmt.Sprintf("SyncTwinState send: %v", err)}
				return
			}
			twinResp, err := twinStream.Recv()
			if err != nil {
				results <- result{agentID, fmt.Sprintf("SyncTwinState recv: %v", err)}
				return
			}
			if twinResp.ServerId == "" {
				results <- result{agentID, "SyncTwinState: empty server_id"}
				return
			}
			closeSendStream(twinStream)

			// 3. TelemetryStream bidirectional stream (send metrics, receive ack)
			tel := pbv1.NewTelemetryStreamClient(conn)
			telStream, err := tel.StreamMetrics(ctx)
			if err != nil {
				results <- result{agentID, fmt.Sprintf("StreamMetrics open: %v", err)}
				return
			}
			if err := telStream.Send(&pbv1.MetricsRequest{
				AgentId: agentID,
				TraceId: fmt.Sprintf("trace-%06d", idx),
				Metrics: []*pbv1.Metric{
					{
						Name:            "cpu_usage_percent",
						Value:           float64(rand.Intn(100)),
						TimestampUnixMs: time.Now().UnixMilli(),
					},
				},
			}); err != nil {
				results <- result{agentID, fmt.Sprintf("StreamMetrics send: %v", err)}
				return
			}
			ack, err := telStream.Recv()
			if err != nil {
				results <- result{agentID, fmt.Sprintf("StreamMetrics recv: %v", err)}
				return
			}
			if ack.AckedCount == 0 {
				results <- result{agentID, "StreamMetrics: acked_count=0"}
				return
			}
			closeSendStream(telStream)

			// 4. Heartbeat
			hResp, err := rc.Heartbeat(ctx, &pbv1.HeartbeatRequest{
				AgentId:         agentID,
				TimestampUnixMs: time.Now().UnixMilli(),
			})
			if err != nil {
				results <- result{agentID, fmt.Sprintf("heartbeat: %v", err)}
				return
			}
			if !hResp.Acknowledged {
				results <- result{agentID, "heartbeat: not acknowledged"}
				return
			}

			results <- result{agentID, ""}
		}(i)
	}

	wg.Wait()
	close(results)

	memAfter := currentMemMB()

	var failures []string
	successful := 0
	for r := range results {
		if r.err != "" {
			failures = append(failures, fmt.Sprintf("%s: %s", r.agentID, r.err))
		} else {
			successful++
		}
	}

	t.Logf("1000-agent concurrent test: %d/%d successful  memory: %.1f MB → %.1f MB (+%.1f MB)",
		successful, spikeNumAgents, memBefore, memAfter, memAfter-memBefore)

	if len(failures) > 0 {
		for _, f := range failures {
			t.Logf("  FAIL: %s", f)
		}
		t.Errorf("%d/%d agents failed (SC-CONN: all 1000 must succeed)", len(failures), spikeNumAgents)
	}
}

// ── BenchmarkGRPCRegister ────────────────────────────────────────────────────

// BenchmarkGRPCRegister measures the throughput of sequential Register RPCs.
func BenchmarkGRPCRegister(b *testing.B) {
	srv := startSpikeSrvBench(b)
	conn, err := grpc.NewClient(srv.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	rc := pbv1.NewAgentRegistryClient(conn)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rc.Register(ctx, &pbv1.RegisterRequest{
			AgentId:   fmt.Sprintf("agent-%d", i),
			AgentType: "native-go",
			Hostname:  fmt.Sprintf("host-%d", i),
		})
		if err != nil {
			b.Fatalf("Register[%d]: %v", i, err)
		}
	}
}

// BenchmarkGRPCTelemetryStream measures telemetry streaming throughput
// (messages/sec) with a single persistent bidirectional stream.
func BenchmarkGRPCTelemetryStream(b *testing.B) {
	srv := startSpikeSrvBench(b)
	conn, err := grpc.NewClient(srv.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	tel := pbv1.NewTelemetryStreamClient(conn)
	ctx := context.Background()
	stream, err := tel.StreamMetrics(ctx)
	if err != nil {
		b.Fatalf("StreamMetrics open: %v", err)
	}
	defer closeSendStream(stream)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := stream.Send(&pbv1.MetricsRequest{
			AgentId: "bench-agent",
			Metrics: []*pbv1.Metric{
				{Name: "cpu", Value: float64(i % 100), TimestampUnixMs: time.Now().UnixMilli()},
			},
		}); err != nil {
			b.Fatalf("send[%d]: %v", i, err)
		}
		if _, err := stream.Recv(); err != nil {
			b.Fatalf("recv[%d]: %v", i, err)
		}
	}
}

// ── benchmark helpers ────────────────────────────────────────────────────────

// startSpikeSrvBench is the benchmark variant of startSpikeServer.
func startSpikeSrvBench(b *testing.B) *spikeServer {
	b.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to listen: %v", err)
	}

	s := &spikeServer{
		registry:  &mockRegistryServer{agents: make(map[string]bool)},
		twin:      &mockTwinControlServer{},
		telemetry: &mockTelemetryServer{},
		grpcSrv:   grpc.NewServer(),
		addr:      lis.Addr().String(),
	}

	pbv1.RegisterAgentRegistryServer(s.grpcSrv, s.registry)
	pbv1.RegisterTwinControlServer(s.grpcSrv, s.twin)
	pbv1.RegisterTelemetryStreamServer(s.grpcSrv, s.telemetry)

	go func() {
		if err := s.grpcSrv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			// Server stopped; ignore.
		}
	}()

	b.Cleanup(func() { s.grpcSrv.GracefulStop() })
	return s
}

// currentMemMB returns the current heap allocation in MiB.
func currentMemMB() float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.Alloc) / (1024 * 1024)
}
