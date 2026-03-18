# Plan 003: SWE/DevOps Expert Agent Skills — Implementation Plan

**Version**: 1.0.0
**Status**: Draft
**Phase**: Phase 2 — AI Reasoning Enhancement
**Spec Reference**: `.specify/memory/spec-003-swe-devops-agent-skills.md`
**ADR Reference**: ADR-020 (OpenSWE Pattern Evaluation)
**Date**: 2026-03-18

---

## Executive Summary

This plan details the implementation roadmap for Pheromone's Phase 2 SWE/DevOps Expert Agent
Skills, derived from ADR-020's positive evaluation of OpenSWE architectural patterns.

The implementation is divided into four sequential sub-phases across an estimated **80–102 hours**
of engineering effort, plus **24 hours** of prerequisite tech spikes. All work targets the
`internal/skill/` framework and existing hook infrastructure; no breaking changes to gRPC
contracts (ADR-003) or the agentic AI loop (ADR-007) are required.

---

## Prerequisite Tech Spikes

Tech spikes MUST be completed and results documented in the ADR-020 appendix before
implementation of the corresponding phase begins.

| Spike ID | Description | Owner | Effort | Blocks |
|---|---|---|---|---|
| SP-020-01 | OCI sandbox lifecycle benchmark (create/exec/destroy P95 latency on Ubuntu 24.04) | TBD | 6h | Phase 2a |
| SP-020-02 | testcontainers-go `SandboxedExecution` prototype (validate OCI runtime availability) | TBD | 6h | Phase 2a |
| SP-020-03 | go-git `OpenPR` prototype (GitHub + GitLab; validate authentication flow) | TBD | 4h | Phase 2d |
| SP-020-04 | SubagentSkill goroutine lifecycle (leak detection + context propagation) | TBD | 4h | Phase 2c |
| SP-020-05 | ContextBundle assembly benchmark (P95 latency on target instances) | TBD | 4h | Phase 2b |

---

## Phase 2a: Tool Curation + Sandboxed Execution

**Goal**: Every enforcement action executes first in an isolated OCI container; the agent gains a
curated, typed DevOps tool set.

**Estimated Effort**: 28–36 hours
**Prerequisites**: SP-020-01, SP-020-02

### Tasks

| Task | File | Description | Effort |
|---|---|---|---|
| T-2a-01 | `internal/skill/sandbox/sandbox.go` | Define `SandboxedExecutionSkill` interface; implement OCI backend using testcontainers-go; handle graceful degradation when OCI unavailable | 8–10h |
| T-2a-02 | `internal/skill/sandbox/mock_sandbox.go` | Implement deterministic mock for unit tests; support configurable failure injection | 2h |
| T-2a-03 | `internal/skill/sandbox/sandbox_test.go` | Unit tests for all interface methods; mock-based; ≥70% coverage | 4h |
| T-2a-04 | `internal/skill/devops/tools.go` | Implement `DevOpsToolSkill` with all 13 tools; typed input/output structs; structured logging | 8–10h |
| T-2a-05 | `internal/skill/devops/tools_test.go` | Unit tests for all 13 tools using mocks; edge cases (package manager detection, no VCS token error) | 4h |
| T-2a-06 | `internal/skill/framework.go` | Register `SandboxedExecutionSkill` and `DevOpsToolSkill` in skill registry; update `AgentCapabilities` proto advertisement | 2h |
| T-2a-07 | `docs/metrics/phase2a-metrics.md` | Document new Prometheus metrics: `pheromone_sandbox_lifecycle_duration_seconds`, `pheromone_sandbox_unavailable_total`, `pheromone_tool_invocation_duration_seconds` | 1h |
| T-2a-08 | Update `AGENTS.md` | Document Phase 2a skill usage patterns for AI coding agents working on this codebase | 1h |

### Acceptance Criteria

- [ ] `SandboxedExecutionSkill` creates, execs, and destroys a container within 30 s P95 on Ubuntu 24.04
- [ ] Graceful degradation path tested with no-OCI-runtime environment
- [ ] All 13 `DevOpsToolSkill` tools have unit tests
- [ ] Package manager auto-detection works for apt, yum, dnf, apk
- [ ] `pheromone_sandbox_lifecycle_duration_seconds` histogram visible in `/metrics`
- [ ] No regression in Phase 1 skill tests

---

## Phase 2b: Context Engineering

**Goal**: The AI reasoning loop receives a rich, structured context bundle assembled from all
available twin state, drift, history, and architectural guidance before every planning cycle.

**Estimated Effort**: 16–20 hours
**Prerequisites**: SP-020-05

### Tasks

| Task | File | Description | Effort |
|---|---|---|---|
| T-2b-01 | `internal/skill/context/assembly.go` | Define `ContextBundle` struct (versioned); implement `ContextAssemblySkill.Assemble()` with 500 ms timeout and stale-OK fallback | 6–8h |
| T-2b-02 | `internal/skill/context/middleware.go` | Implement `PrePlanContextCheck` middleware (validates freshness) and `MessageQueueInjector` middleware (polls webhook event queue, injects follow-up messages) | 4–5h |
| T-2b-03 | `internal/skill/context/assembly_test.go` | Unit tests for assembly, timeout handling, sentinel values for unavailable fields, bundle versioning | 4h |
| T-2b-04 | `internal/skill/reasoner.go` | Update `RuleBasedReasoner.Plan()` and `AIReasoner` interface to accept `ContextBundle` instead of raw metrics slice; backward-compatible | 2h |
| T-2b-05 | `docs/metrics/phase2b-metrics.md` | Document `pheromone_context_assembly_duration_seconds` histogram | 1h |

### Acceptance Criteria

- [ ] `ContextBundle` assembled within 500 ms P95 including live twin state + drift report
- [ ] Stale-context path tested; `PrePlanContextCheck` triggers reassembly
- [ ] `MessageQueueInjector` injects a follow-up message from mock event queue
- [ ] `AIReasoner` interface change is backward-compatible with existing `RuleBasedReasoner`
- [ ] All existing reasoner tests pass without modification

---

## Phase 2c: Subagent Coordination

**Goal**: Parent agents can fan out independent twin remediation tasks to child agents running
in parallel goroutines, with lifecycle management and result aggregation.

**Estimated Effort**: 16–20 hours
**Prerequisites**: SP-020-04, Phase 2b complete

### Tasks

| Task | File | Description | Effort |
|---|---|---|---|
| T-2c-01 | `internal/skill/subagent/subagent.go` | Implement `SubagentSkill` with `SpawnSubagent`, `WaitForSubagent`, `CancelSubagent`, `ListActiveSubagents`; goroutine lifecycle with context cancellation; orphan reaper goroutine | 8–10h |
| T-2c-02 | `internal/skill/subagent/subagent_test.go` | Tests: parallel spawn, wait, cancel, orphan reaping (60 s deadline), result propagation to parent audit log | 6h |
| T-2c-03 | `internal/skill/framework.go` | Wire SubagentSkill into skill registry; add `subagent_support` capability flag to `AgentCapabilities` | 1h |
| T-2c-04 | `docs/metrics/phase2c-metrics.md` | Document `pheromone_subagent_active_count` gauge, `pheromone_subagent_completed_total`, `pheromone_subagent_orphan_reaped_total` | 1h |

### Acceptance Criteria

- [ ] 5 parallel subagents spawn, complete, and results aggregate to parent audit log
- [ ] Orphan reaper fires within 60 s when parent context is cancelled
- [ ] `pheromone_subagent_active_count` gauge reflects live count correctly
- [ ] No goroutine leaks detected under `-race` flag
- [ ] `SubagentSkill` listed in agent capability advertisement

---

## Phase 2d: GitOps Integration

**Goal**: Significant infrastructure changes produce draft IaC PRs on the configured Git host;
middleware safety nets ensure PRs are created even if the agent forgets.

**Estimated Effort**: 20–24 hours
**Prerequisites**: SP-020-03, Phase 2a complete

### Tasks

| Task | File | Description | Effort |
|---|---|---|---|
| T-2d-01 | `internal/skill/gitops/gitops.go` | Implement `GitOpsSkill` using go-git: `CloneIaCRepo`, `ApplyTwinModelToIaC`, `CommitAndOpenPR`, `CheckPRStatus`; no-op when IaC repo not configured | 8–10h |
| T-2d-02 | `internal/skill/gitops/pr.go` | Implement `OpenPR` for GitHub API (github.com/google/go-github) and GitLab API; include twin ID, drift summary, and trace URL in PR description | 4–5h |
| T-2d-03 | `internal/skill/gitops/gitops_test.go` | Unit tests with mock Git server and mock VCS API; test no-op path; test PR description content | 4h |
| T-2d-04 | `internal/skill/middleware/safety.go` | Implement `IaCPRSafetyNet` middleware and `ActionRollbackMiddleware`; integrate with hook service (ADR-011) | 4–5h |
| T-2d-05 | `internal/skill/middleware/safety_test.go` | Tests: IaCPRSafetyNet fires when no PR created; rollback middleware triggers on failed action | 2h |
| T-2d-06 | `docs/metrics/phase2d-metrics.md` | Document `pheromone_gitops_pr_duration_seconds`, `pheromone_action_rollback_total` | 1h |
| T-2d-07 | Update `docs/adr/adr-020-open-swe-evaluation.md` | Append spike results to ADR-020 as tech spike appendix (as per ADR-007 pattern) | 1h |

### Acceptance Criteria

- [ ] `GitOpsSkill` creates a draft PR with twin metadata and trace URL in description
- [ ] No-op path confirmed when IaC repo not configured (no error, no panic)
- [ ] `IaCPRSafetyNet` automatically opens PR when agent loop completes without `OpenPR` call
- [ ] `ActionRollbackMiddleware` rolls back and emits metric on failed production execution
- [ ] go-git used exclusively (no `git` binary shelled out)

---

## Testing Strategy

### Unit Testing

Each skill package MUST have a `*_test.go` file that:
- Tests the happy path for every public method
- Tests error/degraded paths (no OCI runtime, no VCS token, stale context, etc.)
- Uses mock implementations (`mock_sandbox.go`, mock git server, mock twin store)
- Passes with `-race` flag enabled
- Achieves ≥ 70% coverage (measured with `go test -cover`)

### Integration Testing

Integration tests (tagged `//go:build integration`) run in the Vagrant environment (ADR-012):

| Test | Description |
|---|---|
| `TestSandboxedExecOnRealOCI` | Create real OCI container on Ubuntu 24.04; exec a package install; verify install |
| `TestDevOpsToolShellExec` | Shell exec against real instance; verify output |
| `TestContextAssemblyWithLiveTwin` | Assemble context bundle from live twin in Vagrant; verify all fields populated |
| `TestSubagentParallelTwins` | Spawn 5 subagents targeting 5 Vagrant VMs; verify parallel completion |
| `TestGitOpsCreatePR` | Create IaC PR against a test GitHub repository; verify PR description |

### Regression Testing

The full Phase 1 test suite (`make test`) MUST pass without modification after each phase. The
Phase 2a–2d changes are additive (new packages, new skill registrations) and MUST NOT break any
existing Phase 1 tests.

---

## Dependency Validation

Before implementing Phase 2a and 2d, run security and licence checks on new dependencies:

| Dependency | Version | Check |
|---|---|---|
| `testcontainers-go` (or `github.com/docker/docker`) | Latest stable | Security advisory check; Apache 2.0 licence |
| `go-git` (`github.com/go-git/go-git/v5`) | Latest stable | Security advisory check; Apache 2.0 licence |
| `go-github` (`github.com/google/go-github`) | Latest stable | Security advisory check; BSD-3 licence |

---

## Documentation Updates Required

| Document | Change |
|---|---|
| `docs/adr/INDEX.md` | Add ADR-020 row; add Phase 2 spike rows to tech spike table; add Phase 2 issues to required issues table |
| `AGENTS.md` | Add Phase 2 skill usage guidance; update command matrix |
| `internal/skill/README.md` (create if not exists) | Document Phase 2 skill packages, interfaces, and configuration |
| `docs/metrics/` | Create `phase2a-metrics.md`, `phase2b-metrics.md`, `phase2c-metrics.md`, `phase2d-metrics.md` |

---

## Issue Filing

The following GitHub issues should be filed to track this work:

| Title | Type | Phase | Labels |
|---|---|---|---|
| ADR-020: OCI sandbox lifecycle benchmark (SP-020-01) | Tech Spike | Phase 2a | `spike`, `adr-020`, `phase-2` |
| ADR-020: testcontainers-go SandboxedExecution prototype (SP-020-02) | Tech Spike | Phase 2a | `spike`, `adr-020`, `phase-2` |
| ADR-020: go-git OpenPR prototype (SP-020-03) | Tech Spike | Phase 2d | `spike`, `adr-020`, `phase-2` |
| ADR-020: SubagentSkill goroutine lifecycle test (SP-020-04) | Tech Spike | Phase 2c | `spike`, `adr-020`, `phase-2` |
| ADR-020: ContextBundle assembly benchmark (SP-020-05) | Tech Spike | Phase 2b | `spike`, `adr-020`, `phase-2` |
| feat(skill): Implement SandboxedExecutionSkill (Phase 2a) | Implementation | Phase 2a | `enhancement`, `adr-020`, `phase-2` |
| feat(skill): Implement DevOpsToolSkill curated tool set (Phase 2a) | Implementation | Phase 2a | `enhancement`, `adr-020`, `phase-2` |
| feat(skill): Implement ContextAssemblySkill + middleware (Phase 2b) | Implementation | Phase 2b | `enhancement`, `adr-020`, `phase-2` |
| feat(skill): Implement SubagentSkill coordination (Phase 2c) | Implementation | Phase 2c | `enhancement`, `adr-020`, `phase-2` |
| feat(skill): Implement GitOpsSkill + IaC PR workflow (Phase 2d) | Implementation | Phase 2d | `enhancement`, `adr-020`, `phase-2` |
| feat(skill): Implement IaCPRSafetyNet + ActionRollbackMiddleware (Phase 2d) | Implementation | Phase 2d | `enhancement`, `adr-020`, `phase-2` |
| docs: Review and Accept ADR-020 | ADR Review | — | `adr-review`, `adr-020` |

---

## Timeline Estimate

| Milestone | Work | Effort | Sequential? |
|---|---|---|---|
| M1: Spikes complete | SP-020-01 through SP-020-05 | 24h (parallel) | No — can all run in parallel |
| M2: Phase 2a complete | T-2a-01 through T-2a-08 | 28–36h | After M1 |
| M3: Phase 2b complete | T-2b-01 through T-2b-05 | 16–20h | After M1 |
| M4: Phase 2c complete | T-2c-01 through T-2c-04 | 16–20h | After M3 |
| M5: Phase 2d complete | T-2d-01 through T-2d-07 | 20–24h | After M2 |
| M6: ADR-020 appendix | Spike results → ADR-020 | 2h | After M1 |

**Total estimated engineering effort**: 80–102 hours (excluding spikes)
**Parallel execution target**: M2 and M3 can run in parallel; M4 after M3; M5 after M2

---

## References

- ADR-020 (`docs/adr/adr-020-open-swe-evaluation.md`)
- Spec-003 (`.specify/memory/spec-003-swe-devops-agent-skills.md`)
- ADR-007 (`docs/adr/adr-007-agentic-ai-agent-model.md`)
- ADR-011 (`docs/adr/adr-011-post-action-hooks.md`)
- ADR-018 (`docs/adr/adr-018-observability-stack.md`)
- ADR-019 (`docs/adr/adr-019-agent-ai-observability.md`)
- Constitution v2.1.0 (`.specify/memory/constitution.md`)
