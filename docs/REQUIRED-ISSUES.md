# Required GitHub Issues for ADR Processing

**Generated**: 2026-02-26  
**Purpose**: Track all GitHub issues that must be opened to process outstanding ADRs and continue development.

Each issue below is ready to be created. Copy the title and body into a new GitHub issue at
`https://github.com/abuxton/pheromone/issues/new`.

---

## Background

Current issue state:
- **#3 (Closed)**: Tech Spike: ADR-002 — partial results (etcd tests need live instance)
- **#4 (Open)**: Tech Spike: ADR-005 — NATS-JetStream load test
- **#5 (Open)**: Tech Spike: ADR-006 — Go Agent Scaffold Framework

The following ADRs are in **Proposed** status with no associated tracking issue:
ADR-002, ADR-003, ADR-004, ADR-007, ADR-008, ADR-009, ADR-010, ADR-011, ADR-012

---

## Tech Spike Issues

### Issue A: Tech Spike — Complete ADR-002 etcd Validation (Live etcd)

**Title**: `Tech Spike: ADR-002 - Complete etcd Validation with Live etcd Instance`

**Body**:
```
## Objective

Issue #3 closed with partial results. The etcd write-latency (P99) and server-recovery-time
benchmarks were implemented but not run against a live etcd instance. This issue tracks
completing those validations so ADR-002 can be formally accepted.

## Background

See `docs/adr/adr-002-performance-validation.md` for existing results:
- ✅ Cache hit ratio: 85% (exceeds 80% target)
- ✅ No data loss on write failure (rollback verified)
- ⏳ etcd write latency P99 < 100ms — benchmark code exists, needs live etcd
- ⏳ Server recovery from etcd < 5s — benchmark code exists, needs live etcd

## Tasks

1. Start etcd via `docker-compose up -d` (project root `docker-compose.yml`)
2. Run etcd integration tests:
   ```
   go test -v ./benchmark -run TestEtcdWriteLatency
   go test -v ./benchmark -run TestServerRecoveryTime
   go test -v ./benchmark -run TestHybridRecovery
   ```
3. Record P99 write latency and recovery time results
4. Update `docs/adr/adr-002-performance-validation.md` with actual results
5. If all criteria pass: update `docs/adr/adr-002-server-architecture.md` Status → Accepted

## Success Criteria

- etcd write latency P99 < 100ms
- Server recovery time from etcd < 5 seconds
- No data loss on write failure (already passing)

## Related

- ADR-002: `docs/adr/adr-002-server-architecture.md`
- Benchmark code: `benchmark/etcd_bench_test.go`, `benchmark/hybrid_bench_test.go`
- Closed issue: #3

## Effort

Estimated: 2 hours (benchmarks already implemented)
```

---

### Issue B: Tech Spike — ADR-003 gRPC Bidirectional Stream Prototype

**Title**: `Tech Spike: ADR-003 - gRPC Bidirectional Stream Prototype (1000 Agents)`

**Body**:
```
## Objective

Validate that the three gRPC services defined in ADR-003 (AgentRegistry, TwinControl,
TelemetryStream) can handle 1000 simultaneous agents streaming metrics and state updates
before ADR-003 is formally accepted.

## Requirements

**Success Criteria**:
- 1000 agents stream simultaneously without connection drops
- TwinControl bidirectional stream handles config push latency < 5 seconds (SC-002)
- TelemetryStream handles 100K msgs/sec without message loss (SC-008)
- RegisterRequest/Heartbeat round-trip latency < 100ms (P99)

## Tasks

1. Define proto files in `.proto/pheromone/v1/`:
   - `agent_registry.proto` (AgentRegistry service)
   - `twin_control.proto` (TwinControl service with bidirectional streaming)
   - `telemetry.proto` (TelemetryStream service)
2. Generate Go code with `protoc-gen-go` and `protoc-gen-go-grpc`
3. Implement a minimal mock server (all three services)
4. Build a load generator: spawn 1000 goroutines simulating agents
5. Measure connection latency, streaming throughput, and memory usage under load
6. Document findings in `docs/adr/adr-003-grpc-contracts.md`

## Related

- ADR-003: `docs/adr/adr-003-grpc-contracts.md`
- ADR-001: gRPC as primary protocol
- Spec-001: FR-005, FR-006, FR-007, SC-002, SC-008

## Effort

Estimated: 12 hours
```

---

### Issue C: Tech Spike — ADR-007 Agentic AI Loop Resource Usage

**Title**: `Tech Spike: ADR-007 - Validate Agentic AI Loop Resource Usage`

**Body**:
```
## Objective

Measure the CPU and memory overhead of the agentic AI reasoning loop (Tier 0 rule-based
reasoner, Phase 1 MVP) running continuously on a typical managed instance to confirm
resource targets are acceptable.

## Background

ADR-007 establishes that every Pheromone agent hosts an AI reasoning loop. The Phase 1
loop uses a deterministic rule-based reasoner (RuleBasedReasoner). This spike validates
the resource overhead of the loop itself (not the LLM — that is ADR-008).

## Requirements

**Success Criteria**:
- RuleBasedReasoner loop overhead: < 50 MB RAM on an instance with 1 GB RAM
- Reasoning cycle CPU: < 5% average on a 2-core instance
- Loop does not degrade metric collection throughput

## Tasks

1. Implement `RuleBasedReasoner` from ADR-008's Go interface:
   ```go
   type AIReasoner interface {
       Plan(ctx, obs Observations, goal GoalState, drift DriftReport) ([]Action, error)
   }
   ```
2. Implement a test harness running 100 reasoning cycles (simulating 5-minute operation)
3. Measure RAM and CPU with `runtime.ReadMemStats` and OS-level profiling
4. Document results in `docs/adr/adr-007-agentic-ai-agent-model.md`
5. Confirm ADR-007 status remains Accepted (or flag if assumptions are violated)

## Related

- ADR-007: `docs/adr/adr-007-agentic-ai-agent-model.md` (Status: Accepted)
- ADR-008: `docs/adr/adr-008-ai-model-selection.md` (depends on this spike)

## Effort

Estimated: 6 hours
```

---

### Issue D: Tech Spike — ADR-008 Ollama + phi3.5:mini Resource Benchmark

**Title**: `Tech Spike: ADR-008 - Ollama + phi3.5:mini Resource Benchmark on 8 GB Instance`

**Body**:
```
## Objective

Validate that the ADR-008 tiered reasoning architecture (Tier 0 rule-based → Tier 1 Ollama
local LLM) operates within the resource envelope of a typical managed instance (8 GB RAM).
Confirm fallback to RuleBasedReasoner when Ollama is unavailable.

## Requirements

**Success Criteria**:
- `phi3.5:3.8b-mini-instruct-q4_K_M` model runs within 3 GB RAM on an 8 GB instance
- Reasoning cycle with Ollama < 5 seconds (acceptable latency for config decisions)
- Fallback to RuleBasedReasoner when Ollama HTTP API is unreachable (< 100ms)
- `qwen2.5:1.5b-instruct-q4_K_M` runs within 1.5 GB RAM (edge node profile)

## Tasks

1. Install Ollama on an Ubuntu 24.04 VM (Vagrant `agent-ubuntu` from ADR-012 or bare metal)
2. Pull and benchmark `phi3.5:3.8b-mini-instruct-q4_K_M`:
   - Measure cold-start time (model load)
   - Measure inference time for a sample system-management prompt
   - Record RAM usage (Ollama + agent process)
3. Pull and benchmark `qwen2.5:1.5b-instruct-q4_K_M` (edge profile)
4. Implement `OllamaReasoner` with fallback:
   ```go
   func (o *OllamaReasoner) Plan(...) ([]Action, error) {
       resp, err := ollamaGenerate(ctx, o.BaseURL, o.Model, prompt)
       if err != nil {
           return (&RuleBasedReasoner{}).Plan(ctx, obs, goal, drift)  // fallback
       }
       return parseActionsFromResponse(resp), nil
   }
   ```
5. Test fallback: kill Ollama mid-cycle; verify agent continues on rule-based
6. Document results in `docs/adr/adr-008-ai-model-selection.md`

## Related

- ADR-008: `docs/adr/adr-008-ai-model-selection.md`
- ADR-007: establishes the AIReasoner interface
- ADR-012: Vagrant environment provides Ubuntu 24.04 test instance

## Effort

Estimated: 6 hours
```

---

### Issue E: Tech Spike — ADR-011 HTTP Webhook Dispatch at Scale

**Title**: `Tech Spike: ADR-011 - HTTP Webhook Dispatch at Scale (100 Endpoints × 1000 Agents)`

**Body**:
```
## Objective

Validate that the server-side Post-Action Hook Service (ADR-011) can dispatch HTTP webhooks
to 100 registered endpoints when 1000 agents simultaneously fire action events, without
significant latency degradation or dropped hooks.

## Requirements

**Success Criteria**:
- Server dispatches to 100 endpoints with 1000 simultaneous ActionEvents without OOM
- P99 hook dispatch latency < 2 seconds per endpoint
- Zero hooks dropped under load (dead-letter queue captures failures)
- Server gRPC responsiveness unaffected during hook dispatch burst

## Tasks

1. Implement a minimal `PostActionHookService` in Go:
   - In-memory `HookRegistry`
   - Async dispatch worker pool (configurable concurrency)
   - Exponential backoff retry (max 3 attempts)
2. Set up 100 mock HTTP webhook servers (httptest.Server)
3. Register 100 hooks; fire 1000 simultaneous `ActionEvent` messages
4. Measure: dispatch latency, memory consumption, gRPC impact
5. Document results in `docs/adr/adr-011-post-action-hooks.md`

## Related

- ADR-011: `docs/adr/adr-011-post-action-hooks.md`
- ADR-003: ActionEventService extends gRPC contracts
- ADR-005: NATS for dead-letter queue (Phase 2)

## Effort

Estimated: 6 hours
```

---

### Issue F: Tech Spike — ADR-011 Agent Direct Notify Resilience

**Title**: `Tech Spike: ADR-011 - Agent Direct Notify Resilience (Unreachable Destinations)`

**Body**:
```
## Objective

Verify that the agent's NotificationSkill fires direct HTTP webhooks asynchronously
(fire-and-forget) and does NOT block or delay the AI reasoning loop when notification
destinations are unreachable.

## Requirements

**Success Criteria**:
- `NotifyDirect()` returns in < 10ms regardless of destination reachability
- Reasoning loop tick is not delayed by notification failures
- Failed deliveries are logged with trace_id; no panic or crash

## Tasks

1. Implement `NotificationSkillImpl.NotifyDirect()` with goroutine-based dispatch
2. Write a unit test:
   ```go
   func TestNotificationSkillNonBlocking(t *testing.T) {
       skill := NewNotificationSkill(mockGRPCClient, mockHTTPClient)
       start := time.Now()
       err := skill.NotifyDirect(ctx, ActionEvent{Outcome: "success"}, []NotificationDestination{
           {Type: "http-webhook", URL: "http://0.0.0.0:1/unreachable"},
       })
       assert.NoError(t, err)
       assert.Less(t, time.Since(start), 10*time.Millisecond)
   }
   ```
3. Simulate 30-second destination outage; verify reasoning loop continues normally
4. Confirm delivery failures appear in structured logs with trace_id

## Related

- ADR-011: `docs/adr/adr-011-post-action-hooks.md`
- ADR-007: AI reasoning loop must not be blocked by notification

## Effort

Estimated: 4 hours
```

---

### Issue G: Tech Spike — ADR-011 HMAC Webhook Verification Prototype

**Title**: `Tech Spike: ADR-011 - HMAC Webhook Verification Prototype`

**Body**:
```
## Objective

Prototype HMAC-SHA256 webhook signature generation in the Go agent and signature
verification in a Python receiver, confirming the cross-language signature scheme is
correct before ADR-011 is accepted.

## Requirements

**Success Criteria**:
- Go agent generates `X-Pheromone-Signature: sha256=<hex>` header
- Python receiver verifies signature using `hmac` stdlib
- Verification fails correctly on tampered payload
- Same shared secret works across Go → Python

## Tasks

1. Implement Go signature generation:
   ```go
   func computeHMAC(payload, secret []byte) string {
       mac := hmac.New(sha256.New, secret)
       mac.Write(payload)
       return "sha256=" + hex.EncodeToString(mac.Sum(nil))
   }
   ```
2. Implement Python verification script (for receiver simulation)
3. Write Go unit tests for sign + verify roundtrip
4. Document the signature scheme in `docs/adr/adr-011-post-action-hooks.md`

## Related

- ADR-011: Security Considerations — webhook HMAC requirement
- ADR-010: mTLS for credential transport

## Effort

Estimated: 2 hours
```

---

## ADR Review and Acceptance Issues

### Issue H: Review and Accept ADR-002 — Server Architecture

**Title**: `Review and Accept ADR-002 - Server Architecture (In-Memory + etcd Hybrid)`

**Body**:
```
## Objective

Formally review ADR-002 and change status from Proposed → Accepted once the etcd
validation tech spike (see issue for ADR-002 etcd spike) passes all success criteria.

## Checklist

- [ ] etcd write latency P99 < 100ms confirmed (live etcd test)
- [ ] Server recovery time from etcd < 5 seconds confirmed
- [ ] Cache hit ratio > 80% confirmed (already passing: 85%)
- [ ] No data loss on write failure confirmed (already passing)
- [ ] ADR-002 reviewed by minimum 2 team members
- [ ] ADR-002 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-002 ⏳ → ✅

## Acceptance Criteria

All four success criteria from `docs/adr/adr-002-performance-validation.md` must show PASS.
Minimum 2 approvers required (per Constitution Amendment procedure).

## Related

- ADR-002: `docs/adr/adr-002-server-architecture.md`
- Performance validation: `docs/adr/adr-002-performance-validation.md`
- Tech spike: issue for ADR-002 etcd completion
- Closed issue: #3 (partial results)
```

---

### Issue I: Review and Accept ADR-003 — gRPC Service Contracts

**Title**: `Review and Accept ADR-003 - gRPC Service Contracts`

**Body**:
```
## Objective

Formally review ADR-003 (gRPC service contracts for AgentRegistry, TwinControl, and
TelemetryStream) and change status from Proposed → Accepted.

## Prerequisites

- Tech spike for ADR-003 (gRPC bidirectional stream prototype) must complete successfully

## Checklist

- [ ] gRPC bidirectional stream prototype spike passed (1000 agents simultaneous)
- [ ] Proto schema reviewed for backward compatibility and correctness
- [ ] AgentCapabilities extension (from ADR-007 update) included in review
- [ ] ProposeAction RPC extension included in review
- [ ] AIDecisionTrace extension to MetricsRequest included in review
- [ ] ADR-003 reviewed by minimum 2 team members
- [ ] ADR-003 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-003 ⏳ → ✅

## Related

- ADR-003: `docs/adr/adr-003-grpc-contracts.md`
- ADR-007: agentic AI extensions to gRPC contracts
- Tech spike: ADR-003 bidirectional stream issue
```

---

### Issue J: Review and Accept ADR-004 — Twin Model Schema Format

**Title**: `Review and Accept ADR-004 - Twin Model Schema Format (YAML + JSON Schema)`

**Body**:
```
## Objective

Formally review ADR-004 (YAML as primary twin model format with JSON Schema v7
validation) and change status from Proposed → Accepted.

## Checklist

- [ ] Sample twin model YAML reviewed for usability by at least one operator/SRE persona
- [ ] JSON Schema validated against sample YAML file
- [ ] apiVersion pattern `pheromone.io/v1` confirmed
- [ ] Schema evolution strategy reviewed (field additions backward compatible)
- [ ] ADR-004 reviewed by minimum 2 team members
- [ ] ADR-004 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-004 ⏳ → ✅

## No Tech Spike Required

ADR-004 does not require a performance spike. Acceptance is based on usability review
and schema correctness validation.

## Related

- ADR-004: `docs/adr/adr-004-twin-model-schema.md`
- ADR-003: Twin message structure in gRPC contracts
```

---

### Issue K: Review and Accept ADR-008 and ADR-009 — AI Model Selection

**Title**: `Review and Accept ADR-008 and ADR-009 - AI Model Selection and OpenClaw Evaluation`

**Body**:
```
## Objective

Formally review and accept:
1. ADR-008 (AI Model Selection — tiered local-first reasoning architecture)
2. ADR-009 (OpenClaw Evaluation — confirms ADR-008; OpenClaw not adopted as core infra)

These ADRs can be reviewed together as ADR-009 is an extension of ADR-008.

## Prerequisites

- Tech spike for ADR-008 (Ollama + phi3.5:mini benchmark) must complete

## Checklist

- [ ] ADR-008 Ollama benchmark spike passed (RAM/CPU within targets)
- [ ] Tiered architecture (Rule → Ollama → Remote API) reviewed
- [ ] Host OS recommendations reviewed (Ubuntu 24.04 primary, Talos secondary)
- [ ] OpenClaw evaluation (ADR-009) confirmed: not suitable as server or agent
- [ ] AIReasoner Go interface reviewed
- [ ] ADR-008 reviewed by minimum 2 team members
- [ ] ADR-009 reviewed by minimum 2 team members
- [ ] ADR-008 status updated: Proposed → Accepted
- [ ] ADR-009 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-008 ⏳ → ✅, ADR-009 ⏳ → ✅

## Related

- ADR-008: `docs/adr/adr-008-ai-model-selection.md`
- ADR-009: `docs/adr/adr-009-openclaw-evaluation.md`
- ADR-007: establishes AIReasoner interface that ADR-008 implements
```

---

### Issue L: Review and Accept ADR-010 — Signal Protocol Evaluation

**Title**: `Review and Accept ADR-010 - Signal Protocol Not Adopted; gRPC+mTLS Confirmed`

**Body**:
```
## Objective

Formally review ADR-010 (Signal Protocol evaluation — not adopted for Pheromone
server↔agent communication) and change status from Proposed → Accepted.

ADR-010 confirms gRPC with mTLS as the sole server↔agent security architecture.

## Checklist

- [ ] Signal Protocol rejection rationale reviewed (5 reasons documented)
- [ ] mTLS security comparison table reviewed
- [ ] AGPL v3 licence constraint acknowledged by project governance
- [ ] Follow-up actions confirmed: mTLS enablement documented in ADR-003 update
- [ ] ADR-010 reviewed by minimum 2 team members
- [ ] ADR-010 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-010 ⏳ → ✅

## No Tech Spike Required

ADR-010 is an architectural evaluation (not adoption). Acceptance is based on the
written analysis of signalapp repositories, licence review, and protocol comparison.

## Related

- ADR-010: `docs/adr/adr-010-evaluate-signal-protocol.md`
- ADR-003: gRPC contracts (mTLS enablement follow-up)
- ADR-001: gRPC as primary protocol (confirmed by this ADR)
```

---

### Issue M: Review and Accept ADR-011 — Post-Action Hooks

**Title**: `Review and Accept ADR-011 - Post-Action Hooks (Notification Skill + Hook Service)`

**Body**:
```
## Objective

Formally review ADR-011 (post-action hooks: Notification Skill on agents + Post-Action
Hook Service on server) and change status from Proposed → Accepted.

## Prerequisites

All three ADR-011 tech spikes must complete:
- Tech Spike: HTTP Webhook Dispatch at Scale
- Tech Spike: Agent Direct Notify Resilience
- Tech Spike: HMAC Webhook Verification Prototype

## Checklist

- [ ] HTTP webhook dispatch at scale spike passed
- [ ] Agent direct notify resilience spike passed
- [ ] HMAC webhook verification spike passed
- [ ] Hybrid model (NotificationSkill + PostActionHookService) design reviewed
- [ ] gRPC ActionEventService proto extension reviewed
- [ ] Security requirements reviewed (credential storage, HMAC, URL allowlist)
- [ ] ADR-011 reviewed by minimum 2 team members
- [ ] ADR-011 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-011 ⏳ → ✅

## Related

- ADR-011: `docs/adr/adr-011-post-action-hooks.md`
- ADR-003: ActionEventService extends gRPC contracts
- ADR-006: NotificationSkill added to agent scaffold
```

---

### Issue N: Review and Accept ADR-012 — Vagrant Testing Environment

**Title**: `Review and Accept ADR-012 - Vagrant Testing Environment`

**Body**:
```
## Objective

Formally review ADR-012 (Vagrant multi-machine testing environment: server on Ubuntu 24.04
+ agent-ubuntu + agent-debian) and change status from Proposed → Accepted.

## Current Status

The `Vagrantfile` has been created (`vagrant/Vagrantfile`). The provisioning scripts
referenced by the Vagrantfile (`vagrant/scripts/common.sh`, `server.sh`, `agent.sh`) are
not yet implemented.

## Checklist

- [ ] Vagrant provisioning scripts implemented:
  - [ ] `vagrant/scripts/common.sh` (Go toolchain install, Docker)
  - [ ] `vagrant/scripts/server.sh` (etcd via Docker Compose, server setup)
  - [ ] `vagrant/scripts/agent.sh` (agent setup)
- [ ] `vagrant up server` → etcd health check passes
- [ ] `vagrant up agent-ubuntu` → `go version` outputs `go1.24.x`
- [ ] `vagrant up agent-debian` → `go version` outputs `go1.24.x`
- [ ] `vagrant validate` passes (Vagrantfile syntax valid)
- [ ] ADR-012 reviewed by minimum 2 team members
- [ ] ADR-012 status updated: Proposed → Accepted
- [ ] INDEX.md updated: ADR-012 ⏳ → ✅

## Related

- ADR-012: `docs/adr/adr-012-vagrant-testing-environment.md`
- ADR-008: Ubuntu 24.04 LTS and Debian 12 as OS targets
- ADR-002: etcd Docker Compose configuration reused
```

---

## Implementation Issues

### Issue O: Implement Vagrant Provisioning Scripts (ADR-012)

**Title**: `feat(vagrant): Implement Vagrant provisioning scripts for ADR-012 testing environment`

**Body**:
```
## Objective

Implement the three provisioning shell scripts referenced in `vagrant/Vagrantfile` that
are currently missing. The Vagrantfile is complete; only the scripts directory is absent.

## Scope

Create `vagrant/scripts/`:

### `common.sh` — Shared setup (all VMs)

- Install Go 1.24 from official upstream tarball (not via apt/snap)
- Install Docker and Docker Compose
- Install essential tools (git, curl, make)
- Add `vagrant` user to `docker` group

### `server.sh` — Server VM setup

- Copy project root to `/home/vagrant/pheromone`
- Start etcd via `docker-compose up -d` (reuse project root `docker-compose.yml`)
- Verify etcd health: `curl http://${SERVER_IP}:2379/health`
- Placeholder for future pheromone server binary

### `agent.sh` — Agent VM setup

- Configure Go path
- Placeholder for future pheromone agent binary
- Log agent IP and server IP for connectivity reference

## Acceptance Criteria

- `vagrant up server` completes and `curl http://192.168.56.10:2379/health` returns OK
- `vagrant up agent-ubuntu` completes and `go version` outputs `go1.24.x`
- `vagrant up agent-debian` completes and `go version` outputs `go1.24.x`
- `vagrant validate` passes

## Related

- ADR-012: `docs/adr/adr-012-vagrant-testing-environment.md`
- Vagrantfile: `vagrant/Vagrantfile`
- Blocks: ADR-012 review and acceptance
```

---

### Issue P: Implement gRPC Proto Definitions for pheromone.v1 (ADR-003)

**Title**: `feat(proto): Implement gRPC proto definitions for pheromone.v1 (ADR-003)`

**Body**:
```
## Objective

Create the Protocol Buffer definitions for the three gRPC services defined in ADR-003:
- `AgentRegistry` (request/response)
- `TwinControl` (bidirectional streaming)
- `TelemetryStream` (bidirectional streaming)

Including ADR-007 extensions (AgentCapabilities, ProposeAction, AIDecisionTrace).

## Scope

Create `.proto/pheromone/v1/`:

### `agent_registry.proto`
- `AgentRegistry` service (Register, Heartbeat RPCs)
- `RegisterRequest` with `AgentCapabilities` extension (ADR-007)
- `RegisterResponse`, `HeartbeatRequest`, `HeartbeatResponse`

### `twin_control.proto`
- `TwinControl` service (SyncTwinState bidirectional stream, ProposeAction RPC)
- `TwinSyncRequest`, `TwinSyncResponse`
- `ActionProposal`, `ActionDecision` (ADR-007 human-in-the-loop)

### `telemetry.proto`
- `TelemetryStream` service (StreamMetrics bidirectional stream)
- `MetricsRequest` with `AIDecisionTrace` extension (ADR-007)
- `MetricsResponse`

### `action_events.proto`
- `ActionEventService` (ADR-011: ReportActionEvent, ReportDeliveryEvent, StreamHookConfig)
- `ActionEvent`, `ActionEventAck`
- `PackageDeliveryEvent`, `DeliveryEventAck`
- `HookConfig`, `NotificationDestination`

## Acceptance Criteria

- All `.proto` files lint-clean with `buf lint`
- `buf generate` produces valid Go code in `internal/proto/pheromone/v1/`
- ADR-003 and ADR-011 message schemas match proto definitions
- All fields include field numbers consistent with stated ADR proto snippets

## Blocked By

- ADR-003 acceptance (issue I above)

## Related

- ADR-003: `docs/adr/adr-003-grpc-contracts.md`
- ADR-007: gRPC contract extensions for agentic AI
- ADR-011: ActionEventService proto
- `go.mod`: Add `google.golang.org/grpc` and `google.golang.org/protobuf` if not present
```

---

## Summary Table

| Issue | Type | ADR | Effort | Blocking |
|-------|------|-----|--------|----------|
| A | Tech Spike | ADR-002 | 2h | Issue H |
| B | Tech Spike | ADR-003 | 12h | Issue I |
| C | Tech Spike | ADR-007 | 6h | — |
| D | Tech Spike | ADR-008 | 6h | Issue K |
| E | Tech Spike | ADR-011 | 6h | Issue M |
| F | Tech Spike | ADR-011 | 4h | Issue M |
| G | Tech Spike | ADR-011 | 2h | Issue M |
| H | ADR Review | ADR-002 | — | Issue A |
| I | ADR Review | ADR-003 | — | Issue B |
| J | ADR Review | ADR-004 | — | — |
| K | ADR Review | ADR-008/009 | — | Issue D |
| L | ADR Review | ADR-010 | — | — |
| M | ADR Review | ADR-011 | — | Issues E,F,G |
| N | ADR Review | ADR-012 | — | Issue O |
| O | Implementation | ADR-012 | 4h | — |
| P | Implementation | ADR-003 | 8h | Issue I |

**Total estimated effort**: ~56 hours (parallelisable)

**Critical path to MVP implementation start**:
A → H → (server implementation begins)
B → I → P → (gRPC service implementation begins)
J → (twin model implementation begins)
O → N → (Vagrant-based integration testing begins)
