# Specification 003: SWE/DevOps Expert Agent Skills

**Version**: 1.0.0
**Status**: Draft
**Phase**: Phase 2 — AI Reasoning Enhancement
**ADR Reference**: ADR-020 (OpenSWE Pattern Evaluation — Accepted 2026-03-18)
**Author**: Copilot (via Speckit)
**Date**: 2026-03-18

---

## Overview

This specification defines the **SWE/DevOps Expert Agent Skills** for Pheromone Phase 2.
Building on the agentic AI agent model (ADR-007) and the OpenSWE pattern evaluation (ADR-020),
it describes the six new capabilities that transform each Pheromone digital twin into an
autonomous DevOps expert capable of sandboxed execution, curated tool use, structured context
reasoning, subagent coordination, and GitOps workflow integration.

All skills are implemented in Go within the existing `internal/skill/` framework, with no new
external language runtimes required.

---

## Goals

| # | Goal |
|---|------|
| G-01 | Each digital twin operates as an autonomous SWE/DevOps expert that can plan, validate (sandboxed), execute, and commit infrastructure changes with minimal human intervention |
| G-02 | Enforcement actions are validated in an isolated OCI container before being applied to the production instance, containing blast radius |
| G-03 | The agent's reasoning loop uses a structured, rich context bundle assembled from twin state, drift report, action history, and architectural guidance |
| G-04 | Parent agents can coordinate parallel subtasks by spawning child reasoning cycles for independent twin operations |
| G-05 | Significant infrastructure changes result in a GitOps IaC PR for review and audit, not silent in-place mutations |
| G-06 | All new capabilities emit Prometheus metrics (ADR-018) and reasoning traces (ADR-019) |
| G-07 | New skills degrade gracefully on resource-constrained instances that cannot run OCI sandboxes |

---

## Non-Goals

| # | Non-Goal |
|---|----------|
| NG-01 | Adopting OpenSWE's Python/LangGraph runtime — all implementation is in Go |
| NG-02 | Integrating with OpenSWE's cloud sandbox providers (Modal, Daytona, Runloop) |
| NG-03 | Adding Slack or Linear invocation surfaces in this spec (addressed by ADR-015/017) |
| NG-04 | Replacing or modifying the existing Phase 1 rule-based reasoner (backward-compatible) |
| NG-05 | Requiring operators to maintain an IaC repository — GitOps skills are optional |

---

## User Stories

| ID | As a... | I want... | So that... |
|---|---|---|---|
| US-01 | Platform operator | Each managed twin to autonomously remediate configuration drift | I don't have to manually intervene for routine drift events |
| US-02 | Platform operator | Enforcement actions to be validated in a sandbox before touching production | I can trust that agent actions won't break production instances |
| US-03 | Platform engineer | Infrastructure changes proposed by agents to appear as IaC PRs | I have an audit trail and change review process |
| US-04 | Platform engineer | An agent to coordinate parallel remediation across multiple twins | Fleet-wide remediations complete faster without serial blocking |
| US-05 | AI model integrator | The reasoning loop to receive a rich, structured context bundle | The AI model makes better planning decisions with full situational awareness |
| US-06 | Operator | Agent actions to be fully observable (metrics + reasoning traces) | I can diagnose and audit all agent decisions post-hoc |
| US-07 | DevOps engineer | Agent to detect that a config change requires a package install, runs it safely, and confirms success | Drift is resolved end-to-end without manual steps |

---

## Functional Requirements

### FR-030: Sandboxed Execution Skill

| ID | Requirement |
|---|---|
| FR-030.1 | The `SandboxedExecutionSkill` MUST create an ephemeral OCI container that mirrors the target twin's system profile (OS, installed packages, running services) |
| FR-030.2 | All enforcement actions that modify system state MUST be executed inside a sandbox first; only on successful validation MAY they be applied to the production instance |
| FR-030.3 | The sandbox lifecycle (create, exec, validate, commit or discard) MUST complete within 30 seconds for typical enforcement actions |
| FR-030.4 | The skill MUST support OCI runtimes available on the managed instance (Docker, containerd, Podman); it MUST NOT require a specific runtime |
| FR-030.5 | On instances where no OCI runtime is available, the skill MUST degrade gracefully to direct execution with enhanced pre/post validation checks and emit a `sandbox_unavailable` metric |
| FR-030.6 | Sandbox results (exit code, stdout, stderr, changeset diff) MUST be captured and included in the action audit log (ADR-019) |

### FR-031: Curated DevOps Tool Set

| ID | Requirement |
|---|---|
| FR-031.1 | The `DevOpsToolSkill` MUST provide exactly the following tools, no more: `ShellExec`, `FileRead`, `FileWrite`, `FileEdit`, `GitClone`, `GitCommit`, `GitPush`, `OpenPR`, `APICall`, `PackageInstall`, `PackageRemove`, `ServiceRestart`, `ServiceStatus` |
| FR-031.2 | Each tool MUST accept typed, validated input structs and return typed result structs with error wrapping |
| FR-031.3 | All tool invocations MUST be logged as structured events with tool name, inputs (sanitised), duration, and outcome |
| FR-031.4 | `APICall` MUST support configurable authentication (API key, Bearer token, mTLS) with credentials sourced from the secure credential store, NOT from environment variables |
| FR-031.5 | `PackageInstall` / `PackageRemove` MUST detect the OS package manager (apt, yum, dnf, apk) automatically from the twin profile |
| FR-031.6 | `OpenPR` MUST use the credential provided via the agent's configured VCS integration (GitHub token, GitLab token); if no credential is configured, the tool MUST return a structured error (not panic) |

### FR-032: Structured Context Assembly

| ID | Requirement |
|---|---|
| FR-032.1 | The `ContextAssemblySkill` MUST assemble a `ContextBundle` before each planning cycle containing: twin desired state, twin actual state, drift report, recent action history (last 20 actions), `AGENTS.md` content, and instance profile |
| FR-032.2 | The assembly MUST complete within 500 ms; fields not available within 300 ms MUST be populated with a structured `unavailable` sentinel (not omitted) |
| FR-032.3 | The `MessageQueueInjector` middleware MUST poll for operator follow-up messages (from the webhook event queue, ADR-011) and inject them into the context bundle before the next model call |
| FR-032.4 | The context bundle MUST be versioned (include schema version) so that reasoning models can validate they are receiving the expected structure |
| FR-032.5 | Context assembly latency MUST be emitted as a Prometheus histogram metric `pheromone_context_assembly_duration_seconds` |

### FR-033: Subagent Coordination

| ID | Requirement |
|---|---|
| FR-033.1 | The `SubagentSkill` MUST allow a parent agent to spawn a child reasoning cycle for a specified task description and set of target twin IDs |
| FR-033.2 | Each child agent MUST run in its own goroutine with isolated context, tool access, and middleware stack |
| FR-033.3 | The parent agent MUST be able to wait for a subagent (`WaitForSubagent`), cancel it (`CancelSubagent`), or fire-and-forget |
| FR-033.4 | Orphaned subagents (parent cancelled or crashed) MUST be automatically reaped by the skill framework within 60 seconds |
| FR-033.5 | Active subagent count MUST be emitted as a Prometheus gauge metric `pheromone_subagent_active_count` |
| FR-033.6 | Subagent results (success, failure, actions taken) MUST be propagated back to the parent's action audit log |

### FR-034: Middleware Layer Enhancements

| ID | Requirement |
|---|---|
| FR-034.1 | `IaCPRSafetyNet` middleware MUST inspect the agent's completed action set; if significant state changes occurred without a corresponding `OpenPR` tool call, the middleware MUST automatically call `GitOpsSkill.CommitAndOpenPR` |
| FR-034.2 | `ActionRollbackMiddleware` MUST be invoked when a post-sandbox production execution fails; it MUST attempt a rollback using the last known-good twin state snapshot and emit a `pheromone_action_rollback_total` metric |
| FR-034.3 | `PrePlanContextCheck` MUST validate that the `ContextBundle` is fresh (assembled within the last collection interval) before passing it to the reasoning model; stale contexts MUST trigger a reassembly |
| FR-034.4 | All middleware MUST be composable and individually disableable via agent configuration |

### FR-035: GitOps / IaC Workflow Skill

| ID | Requirement |
|---|---|
| FR-035.1 | The `GitOpsSkill` MUST support cloning a configured IaC repository, applying twin model changes as IaC code (Ansible, Terraform, or shell scripts depending on operator configuration), committing the changeset, pushing to a branch, and opening a draft PR |
| FR-035.2 | The skill MUST include the twin ID, drift summary, and reasoning trace URL in the PR description |
| FR-035.3 | The skill MUST be a no-op (gracefully skip) when no IaC repository is configured |
| FR-035.4 | PR creation latency MUST be emitted as a Prometheus histogram `pheromone_gitops_pr_duration_seconds` |
| FR-035.5 | The skill MUST use `go-git` for Git operations; it MUST NOT shell out to the `git` binary |

---

## Non-Functional Requirements

| ID | Requirement |
|---|---|
| NFR-001 | Sandbox create+exec+destroy lifecycle MUST complete within 30 seconds (P95) on supported instance types |
| NFR-002 | Context bundle assembly MUST complete within 500 ms (P95) |
| NFR-003 | Tool invocations (ShellExec, FileRead/Write, PackageInstall) MUST add < 100 ms overhead over the raw OS operation |
| NFR-004 | All new skills MUST be unit-testable in isolation using mock OCI runtimes, mock git servers, and mock twin stores |
| NFR-005 | All new skills MUST have ≥ 70% unit test coverage |
| NFR-006 | New Prometheus metrics emitted by Phase 2 skills MUST be documented in `docs/metrics/` |
| NFR-007 | All new skill interfaces MUST be versioned using `skill_contract_version` (ADR-007) |

---

## Architecture

### Skill Integration Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      Pheromone Agent (per instance)                      │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │               Middleware Layer (ADR-020 extensions)              │   │
│  │   PrePlanContextCheck → MessageQueueInjector → model call        │   │
│  │   post-loop: IaCPRSafetyNet → ActionRollbackMiddleware           │   │
│  └───────────────────────────┬──────────────────────────────────────┘   │
│                               │                                          │
│  ┌──────────────────────────▼──────────────────────────────────────┐   │
│  │                       AI Reasoning Loop (ADR-007)                │   │
│  │   context = ContextAssemblySkill.Assemble(twin_ids)              │   │
│  │   plan    = AIReasoner.Plan(context)                             │   │
│  │   for action in plan:                                            │   │
│  │     sandbox = SandboxedExecutionSkill.Create(twin_profile)       │   │
│  │     result  = SandboxedExecutionSkill.Exec(sandbox, action)      │   │
│  │     if result.ok → ApplyToInstance(action)                       │   │
│  │     SubagentSkill.SpawnIfParallelNeeded(action, twin_ids)        │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                  Phase 2 Skills (ADR-020)                        │   │
│  │                                                                  │   │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐  │   │
│  │  │ SandboxedExec    │  │ DevOpsToolSkill  │  │ ContextAssem │  │   │
│  │  │ Skill            │  │ ShellExec        │  │ blySkill     │  │   │
│  │  │ CreateSandbox()  │  │ FileRead/Write   │  │ Assemble()   │  │   │
│  │  │ ExecInSandbox()  │  │ GitClone/Commit  │  │ ContextBundle│  │   │
│  │  │ CommitToInstance │  │ OpenPR           │  │              │  │   │
│  │  │ DiscardSandbox() │  │ APICall          │  └──────────────┘  │   │
│  │  └──────────────────┘  │ PackageInstall   │                    │   │
│  │                        │ ServiceRestart   │  ┌──────────────┐  │   │
│  │  ┌──────────────────┐  └──────────────────┘  │ SubagentSkil │  │   │
│  │  │ GitOpsSkill      │                        │ l            │  │   │
│  │  │ CloneIaCRepo()   │                        │ SpawnSubagnt │  │   │
│  │  │ ApplyTwinModel() │                        │ WaitFor()    │  │   │
│  │  │ CommitAndOpenPR()│                        │ Cancel()     │  │   │
│  │  └──────────────────┘                        └──────────────┘  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │              Phase 1 Skills (unchanged)                          │   │
│  │  DigitalTwinSkill  │  MetricsSkill  │  ConfigEnforcementSkill   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │               gRPC Communication Layer (ADR-003)                 │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

### New File Structure

```
internal/skill/
├── sandbox/
│   ├── sandbox.go            # SandboxedExecutionSkill interface + OCI implementation
│   ├── sandbox_test.go
│   └── mock_sandbox.go       # Mock for unit tests
├── devops/
│   ├── tools.go              # DevOpsToolSkill + all 13 tools
│   ├── tools_test.go
│   └── tools_integration_test.go
├── context/
│   ├── assembly.go           # ContextAssemblySkill + ContextBundle type
│   ├── assembly_test.go
│   └── middleware.go         # PrePlanContextCheck + MessageQueueInjector
├── subagent/
│   ├── subagent.go           # SubagentSkill + goroutine lifecycle
│   └── subagent_test.go
├── gitops/
│   ├── gitops.go             # GitOpsSkill (go-git based)
│   ├── gitops_test.go
│   └── pr.go                 # PR creation (GitHub/GitLab API)
└── middleware/
    ├── safety.go             # IaCPRSafetyNet + ActionRollbackMiddleware
    └── safety_test.go
```

---

## Success Criteria

| ID | Criterion |
|---|---|
| SC-030 | `SandboxedExecutionSkill` creates, executes in, and destroys an OCI container within 30 s on a standard Ubuntu 24.04 instance |
| SC-031 | `DevOpsToolSkill` successfully installs a package, restarts a service, and reads a file within a single reasoning loop cycle |
| SC-032 | `ContextAssemblySkill` assembles a complete `ContextBundle` within 500 ms including twin state + drift report |
| SC-033 | `SubagentSkill` spawns 5 parallel child agents, all complete independently, and orphan reaping fires within 60 s of parent cancellation |
| SC-034 | `GitOpsSkill` creates a draft PR with twin state changes and reasoning trace URL in description |
| SC-035 | All new skills are observable: Prometheus metrics emitted, reasoning traces captured, action audit events written |
| SC-036 | All new skills degrade gracefully when dependencies are unavailable (no OCI runtime, no VCS token, no IaC repo) |
| SC-037 | Phase 1 behaviour (rule-based reasoner + existing skills) is fully preserved with no regression |

---

## Dependencies

| Dependency | Purpose | New? |
|---|---|---|
| `testcontainers-go` or native OCI client | `SandboxedExecutionSkill` — container lifecycle | New (Phase 2a) |
| `go-git` | `GitOpsSkill` — Git operations without shelling out | New (Phase 2d) |
| GitHub API client (existing Go SDK) | `OpenPR` tool — PR creation | New (Phase 2d) |
| `internal/skill/reasoner.go` | AI reasoning loop (ADR-007) | Existing |
| `internal/skill/framework.go` | Skill registration + execution | Existing |
| `internal/hooks/` | Middleware integration (ADR-011) | Existing |
| `internal/metrics/` | Prometheus metrics emission | Existing |

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| OCI runtime not available on some managed instances | Medium | Medium | Graceful degradation to direct execution (FR-030.5); `sandbox_unavailable` metric alerts operator |
| Context bundle assembly exceeds 500 ms on slow instances | Low | Low | Async assembly with stale-OK fallback; timeout metric triggers investigation |
| Subagent goroutine leak | Low | High | Orphan reaper (FR-033.4); `pheromone_subagent_active_count` alert threshold |
| go-git incompatibility with enterprise Git hosting | Medium | Medium | Spike to validate go-git with GitHub, GitLab, Gitea before Phase 2d commitment |
| AI model context window too small for ContextBundle | Low | Medium | ContextBundle trimming: truncate action history to last 5 if full bundle exceeds model limit |

---

## References

- ADR-020 (OpenSWE Pattern Evaluation — this spec implements the accepted decision)
- ADR-007 (Agentic AI Agent Model — Phase 1 foundation extended by this spec)
- ADR-006 (Agent Lifecycle — skill deployment framework)
- ADR-011 (Post-Action Hooks — middleware hook patterns extended)
- ADR-018 (Observability — Prometheus metrics requirements)
- ADR-019 (AI Observability — reasoning traces + audit log)
- Constitution v2.1.0 — Principle I (Layered Twin Architecture), Principle IV (Quality Gates)
- Plan: `.specify/memory/plan-003-swe-devops-agent-skills.md`
