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

## ADR-015: User Access Control & Identity Provider Integration

> **Feature Branch**: `001-user-access-control`
> **Tasks**: `specs/001-user-access-control/tasks.md` · `.specify/memory/tasks-002-user-access-control.md`
> **Status**: ADR Proposed → implementation issues ready to open

---

### Issue Q: Tech Spike — JWT Library Selection and RS256 Key Management (ADR-015)

**Title**: `spike(auth): JWT library selection and RS256 key-management strategy (ADR-015)`

**Body**:
```
## Objective

Validate the JWT library choice (`golang-jwt/jwt` v5) and RS256 key-management approach
for ADR-015 Phase 1 local auth before implementation begins.

## Background

ADR-015 specifies RS256-signed JWTs as the session credential format for human operator
authentication. Before writing production code, we need:
1. Confirmation that `golang-jwt/jwt` v5 has no open CVEs
2. A benchmark validating RS256 sign/verify latency on CI and arm64 target hardware
3. A documented comparison against alternative libraries (`lestrrat-go/jwx` v2, `go-jose/go-jose`)
4. A threat model covering the top 5 attack vectors for local auth (brute-force, token theft, bootstrap abuse, etc.)

## Tasks

1. Run `govulncheck` against `golang-jwt/jwt` v5 — record result
2. Write a throwaway Go test: RS256 key load → sign token → verify → assert claims round-trip
3. Benchmark RS256 sign + verify on:
   - GitHub Actions runner (amd64)
   - Raspberry Pi 4 target (arm64) if available; otherwise estimate from `golang-jwt` benchmarks
4. Compare `golang-jwt/jwt` v5 vs `lestrrat-go/jwx` v2 vs `go-jose/go-jose` v3 on: API ergonomics, maintenance status, dependency footprint, CVE history
5. Document top 5 threat vectors + mitigations in `research.md` §Threat Model
6. Update `specs/001-user-access-control/research.md` §R1–R4 with final decisions and benchmark figures

## Acceptance Criteria

- `govulncheck` clean on `golang-jwt/jwt` v5
- RS256 sign latency < 5 ms p95 on amd64 CI runner
- Threat model documented with mitigations mapped to existing design controls
- Library comparison table in research.md
- Decision recorded: proceed with `golang-jwt/jwt` v5 (or documented alternative)

## Related

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md`
- Research doc: `specs/001-user-access-control/research.md`
- Tasks: T001, T002 in `specs/001-user-access-control/tasks.md`

## Effort

Estimated: 7 hours (T001 4h + T002 3h, parallelisable)

## Labels

`type:spike` `phase:1` `adr:015`
```

---

### Issue R: feat(auth) — Proto Contract + Code Generation for AuthService (ADR-015)

**Title**: `feat(auth): define auth.proto AuthService contract and generate Go stubs (ADR-015)`

**Body**:
```
## Objective

Define the `AuthService` gRPC contract in `proto/pheromone/v1/auth.proto` and generate the
Go stubs required for Phase 1 local auth implementation.

## Background

ADR-015 Phase 1 introduces a new `AuthService` gRPC service with 7 RPCs. The proto file and
generated stubs must exist before any service handler or interceptor can be implemented.
The existing contract in `specs/001-user-access-control/contracts/auth.proto` is the source of truth.

## Tasks

1. Create `proto/pheromone/v1/auth.proto` matching `specs/001-user-access-control/contracts/auth.proto`
   - Service: `AuthService` with RPCs: `Login`, `Logout`, `ChangePassword`, `ListUsers`, `CreateUser`, `UpdateUser`, `DeleteUser`
   - All request/response message types with field numbers and proto comments
2. Update `buf.gen.yaml` if needed; run `buf lint` and `buf breaking` (no regressions to existing services)
3. Run `buf generate`; verify `internal/gen/pheromone/v1/auth_grpc.pb.go` and `auth.pb.go` created
4. Add `AuthConfig` struct to `internal/config/config.go` (all fields from data-model.md §AuthConfig)
5. Verify `go build ./...` succeeds with generated code

## Acceptance Criteria

- `buf lint` exits 0 on `auth.proto`
- `buf breaking` exits 0 (no changes to existing `pheromone.v1` contracts)
- Generated Go stubs compile: `go build ./...` exits 0
- `AuthConfig` struct in `internal/config/config.go` with all required fields and default values

## Related

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md`
- Contract source: `specs/001-user-access-control/contracts/auth.proto`
- Data model: `specs/001-user-access-control/data-model.md`
- Tasks: T003, T004, T005 in `specs/001-user-access-control/tasks.md`
- Depends on: Issue Q (spike) complete

## Effort

Estimated: 7 hours (T003 4h + T004 1h + T005 2h)

## Labels

`type:feature` `phase:1` `adr:015`
```

---

### Issue S: feat(auth) — Foundational Data Layer (UserRecord, UserStore, AuthProvider, AuditLogger) (ADR-015)

**Title**: `feat(auth): implement UserRecord data layer, UserStore CRUD, AuthProvider interface, and AuditLogger (ADR-015)`

**Body**:
```
## Objective

Implement the foundational data structures and shared interfaces that all user-story phases
depend on. This issue is a hard prerequisite for all auth implementation work.

## Background

ADR-015 Phase 1 requires:
- `UserRecord` Go struct stored in etcd at `/pheromone/users/<username>`
- `UserStore` interface with etcd (primary) and bbolt (fallback) backends
- `AuthProvider` plugin interface for local, LDAP, OIDC, and SAML adapters
- `AuditLogger` backed by logrus that emits structured JSON events

All subsequent auth issues depend on these foundations being in place.

## Tasks

1. **T006** — `UserRecord` struct, `Role` type, `ValidateUsername()`, `ValidatePassword()` in `internal/auth/store.go`
2. **T007** — etcd CRUD: `CreateUser`, `GetUser`, `UpdateUser`, `DeleteUser`, `ListUsers` in `internal/auth/store.go`
3. **T008** — bbolt fallback `UserStore` (same interface, embedded DB) in `internal/auth/store.go`
4. **T009** — `AuthProvider` interface + `UserAttributes` struct in `internal/auth/provider.go`
5. **T010** — `AuditEvent` struct + `AuditLogger` + per-event-type helpers in `internal/auth/audit.go`

## Acceptance Criteria

- `UserRecord` exactly matches `specs/001-user-access-control/data-model.md` §UserRecord
- `Role` hierarchy: `viewer < operator < admin < owner`
- etcd store: atomic creates (compare-and-swap), correct `NotFound` / `AlreadyExists` gRPC codes
- bbolt store: same interface, no live etcd required for `go test`
- `AuditLogger.Emit()` invariant: never logs raw passwords, tokens, or key material
- `go test ./internal/auth/... -run TestUserRecord` and `TestBboltStore` pass
- `go vet ./internal/auth/...` clean

## Related

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md`
- Data model: `specs/001-user-access-control/data-model.md`
- Tasks: T006–T010 in `specs/001-user-access-control/tasks.md`
- Depends on: Issue R (proto + config) complete

## Effort

Estimated: 16 hours (T006 3h + T007 5h + T008 3h + T009 2h + T010 3h; T008/T009/T010 parallelisable after T007)

## Labels

`type:feature` `phase:1` `adr:015`
```

---

### Issue T: feat(auth) — Phase 1 MVP: Bootstrap, RBAC, JWT, Local Auth, Interceptors (ADR-015)

**Title**: `feat(auth): Phase 1 MVP — bootstrap, RBAC policy, JWT service, local bcrypt auth, and gRPC interceptor chain (ADR-015)`

**Body**:
```
## Objective

Implement the complete Phase 1 local authentication MVP covering User Stories 1–3:
- US1: First-run bootstrap (owner account seeding)
- US2: User lifecycle management (CRUD + RBAC enforcement)
- US3: JWT-based session authentication with gRPC interceptor chain

## Background

This is the core implementation issue for ADR-015 Phase 1. It depends on Issue S (data layer)
being complete, and delivers a fully functional auth system: operators log in with username/password,
receive an RS256-signed JWT, and present it on every gRPC call. The interceptor validates the token
and enforces the four-role RBAC model before any handler runs.

## User Stories

### US1 — Bootstrap (T011–T014)

1. **T011** — `Bootstrap()` function: detect zero users → seed owner account → print one-time password to stderr → emit audit event (`internal/auth/store.go`)
2. **T012** — Bootstrap seal guard: reject re-run when users already exist (`internal/auth/store.go`)
3. **T013** — `pheromone user set-password <username>` interactive CLI (`cmd/cli/user.go`)
4. **T014** — Unit tests for bootstrap flow (`tests/unit/auth/bootstrap_test.go`)

### US2 — User Management (T015–T022)

5. **T015** — `RBACPolicy.Check(identity, method)` with static permission matrix (`internal/auth/rbac.go`)
6. **T016** — `AuthService.CreateUser` with RBAC pre-check (`internal/auth/service.go`)
7. **T017** — `AuthService.ListUsers` (admin/owner only) (`internal/auth/service.go`)
8. **T018** — `AuthService.UpdateUser` with privilege-escalation guard (`internal/auth/service.go`)
9. **T019** — `AuthService.DeleteUser` with last-owner guard (`internal/auth/service.go`)
10. **T020** — CLI: `pheromone user create/list/delete/set-role` (`cmd/cli/user.go`)
11. **T021** — Unit tests for RBAC permission matrix (`tests/unit/auth/rbac_test.go`)
12. **T022** — Unit tests for user management service handlers (`tests/unit/auth/service_test.go`)

### US3 — Authentication & Sessions (T023–T036)

13. **T023** — JWT `Issuer`: RS256 key load + `IssueToken()` (`internal/auth/jwt.go`)
14. **T024** — JWT `Validator`: `ValidateToken()` RS256 (`internal/auth/jwt.go`)
15. **T025** — HS256 fallback signing/validation (`internal/auth/jwt.go`)
16. **T026** — Local bcrypt `AuthProvider` (`internal/auth/local/local.go`)
17. **T027** — Login rate limiter, token-bucket per source IP (`internal/auth/ratelimit.go`)
18. **T028** — `AuthService.Login` handler (`internal/auth/service.go`)
19. **T029** — `AuthService.Logout` Phase 1 audit-only (`internal/auth/service.go`)
20. **T030** — `AuthService.ChangePassword` (`internal/auth/service.go`)
21. **T031** — `AuthUnaryInterceptor` + `AuthStreamInterceptor` (`internal/auth/interceptor.go`)
22. **T032** — Wire auth into `cmd/server/main.go` + `GET /auth/jwks` endpoint
23. **T033** — `pheromone key generate` CLI (`cmd/cli/key.go`)
24. **T034** — JWT unit tests (`tests/unit/auth/jwt_test.go`)
25. **T035** — Interceptor unit tests (`tests/unit/auth/interceptor_test.go`)
26. **T036** — Integration smoke tests (`tests/integration/auth_interceptor_test.go`)

## Acceptance Criteria

- AC-001: Unauthenticated gRPC client → `codes.Unauthenticated`
- AC-002: Valid JWT bearer token grants access to permitted RPCs
- AC-003: `viewer` role cannot call write RPCs → `codes.PermissionDenied`
- AC-004: Agent mTLS flows unaffected by auth interceptor
- AC-005: `auth.login_success` / `auth.login_failure` logged on each attempt
- AC-006: `auth.access_denied` logged on each RBAC rejection
- AC-007: Passwords never appear in log output
- AC-008: JWT expired after configured TTL → `codes.Unauthenticated`
- AC-009: First-run bootstrap creates `owner` account; re-run rejected
- AC-010: `pheromone user create/list/delete` CLI works end-to-end
- AC-011: `internal/auth/` unit test coverage ≥ 70% (target 80%)
- AC-012: `govulncheck` clean on new dependencies
- AC-013: `gosec` clean on `internal/auth/`

## Related

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md`
- Tasks: T011–T049 in `specs/001-user-access-control/tasks.md`
- Depends on: Issues Q (spike), R (proto), S (data layer)
- Quickstart: `specs/001-user-access-control/quickstart.md`

## Effort

Estimated: ~95 hours total; ~42h critical path; ~53h parallelisable
Recommended: 2 engineers working Phase 4 (US2) and Phase 5 (US3) in parallel after US1 complete

## Labels

`type:feature` `phase:1` `adr:015`
```

---

### Issue U: feat(auth) — Phase 2: External IdP Adapters (LDAP, OIDC, SAML) (ADR-015)

**Title**: `feat(auth): Phase 2 — LDAP, OIDC, and SAML 2.0 identity provider adapters (ADR-015)`

**Body**:
```
## Objective

Implement the three external identity provider adapters defined in ADR-015 D4, enabling
enterprise users to authenticate with corporate credentials (AD/LDAP, OIDC, SAML 2.0).

## Background

Phase 1 delivers local bcrypt authentication. Phase 2 adds pluggable IdP adapters via
the `AuthProvider` interface established in Issue S. Each adapter is a separate sub-package
and can be reviewed/released independently.

## Sub-issues (recommend one issue per adapter)

### Phase 2a — LDAP Adapter (T037)
- Implement `internal/auth/ldap/ldap.go` satisfying `AuthProvider`
- LDAP bind auth, group search, group→role mapping via `LDAPConfig.GroupRoleMap`
- `ldaps://` + STARTTLS; plain `ldap://` blocked by default
- Shadow `UserRecord` created/synced in etcd on first login
- Integration test against containerised OpenLDAP
- Dependency: `go-ldap/ldap` v3 — run `govulncheck` before adding

### Phase 2b — OIDC Adapter (T038)
- Implement `internal/auth/oidc/oidc.go`
- OIDC Discovery + JWKS auto-rotation via `coreos/go-oidc` v3
- Authorization Code + PKCE + `state` CSRF
- `GET /auth/oidc/callback` aligned with ADR-011 HTTP listener
- Integration test against containerised Keycloak
- Dependency: `coreos/go-oidc` v3 + `golang.org/x/oauth2`

### Phase 2c — SAML 2.0 Adapter (T039)
- Implement `internal/auth/saml/saml.go`
- SP metadata + ACS endpoint
- Library choice: evaluate `crewjam/saml` vs `russellhaering/gosaml2` (update research.md §R7)
- Configuration examples: Okta, ADFS

## Acceptance Criteria

- Each adapter satisfies `AuthProvider` interface (compiles, passes unit tests)
- Break-glass path: local owner account still works when any IdP is unreachable
- `govulncheck` clean on all new dependencies
- Integration tests green for each adapter

## Related

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md` §D4
- Tasks: T037–T039 in `specs/001-user-access-control/tasks.md`
- Depends on: Issue T (Phase 1 MVP) complete

## Effort

Estimated: 24 hours (T037 8h + T038 8h + T039 8h; all parallelisable)

## Labels

`type:feature` `phase:2` `adr:015`
```

---

### Issue V: docs(auth) — Documentation Updates for ADR-015 Phase 1 (ADR-015)

**Title**: `docs(auth): update SECURITY.md, README, ADR index, and quickstart for Phase 1 auth (ADR-015)`

**Body**:
```
## Objective

Update all documentation to reflect the Phase 1 auth implementation delivered in Issue T.

## Tasks

1. **T044** — Update `SECURITY.md`: close ADR-014 "open items", add sections for Credential Policy,
   JWT Token Lifecycle, RBAC Model, Audit Events, Key Management
2. **T045** — Update `docs/adr/INDEX.md`: add ADR-015 row (status, date, derived-from links)
3. **T046** — Update `README.md`: add §Authentication with key generation, bootstrap, and login steps
4. **T042** — Append audit log `jq` query examples to `specs/001-user-access-control/quickstart.md`
5. **T043** — Commit RSA test key pair to `internal/auth/testdata/` with `DO NOT USE IN PRODUCTION` header

## Acceptance Criteria

- `SECURITY.md` no longer has ADR-014 "open items" referencing user auth
- `docs/adr/INDEX.md` includes ADR-015 with correct status link
- `README.md` §Authentication section links to quickstart.md and ADR-015
- quickstart.md §Audit Logging includes verified `jq` examples
- Test key files: private key mode 0600, clearly labelled as test-only

## Related

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md`
- Tasks: T042–T046 in `specs/001-user-access-control/tasks.md`
- Depends on: Issue T (Phase 1 MVP) complete

## Effort

Estimated: 9 hours (T042 2h + T043 1h + T044 2h + T045 0.5h + T046 1h; all parallelisable)

## Labels

`type:docs` `phase:1` `adr:015`
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
| Q | Tech Spike | ADR-015 | 7h | Issue T |
| R | Implementation | ADR-015 | 7h | Issue Q |
| S | Implementation | ADR-015 | 16h | Issue R |
| T | Implementation | ADR-015 | ~95h | Issue S |
| U | Implementation | ADR-015 | 24h | Issue T |
| V | Documentation | ADR-015 | 9h | Issue T |

**Total estimated effort**: ~212 hours (parallelisable; ~56h pre-ADR-015 + ~156h ADR-015)

**Critical path to MVP implementation start**:
A → H → (server implementation begins)
B → I → P → (gRPC service implementation begins)
J → (twin model implementation begins)
O → N → (Vagrant-based integration testing begins)

**ADR-015 critical path (Phase 1 MVP)**:
Q → R → S → T (US1) → T (US2 ∥ US3) → V → (ADR-015 Status: Accepted)

**ADR-015 Phase 2 critical path**:
T → U (LDAP ∥ OIDC ∥ SAML) → (enterprise IdP available)
