# ADR 020: OpenSWE Pattern Evaluation — SWE/DevOps Expert Agent Skills

## Status

Accepted

## Context

AI tooling and autonomous coding agent frameworks are advancing rapidly. This ADR documents a
formal review of **OpenSWE** (<https://github.com/langchain-ai/open-swe>) and the architectural
patterns described in the LangChain announcement blog
(<https://blog.langchain.com/open-swe-an-open-source-framework-for-internal-coding-agents/>),
assessing their relevance to Pheromone's agentic AI agent model (ADR-007).

The review was requested in [Issue: "review SWE as ADR for integration"] with the following
acceptance criteria:

1. ADR created for review, summary, and planning specification
2. Decision reached
3. Positive decision leads to plan, spec, and issues filed for development with review required
4. Negative decision leads to well-formed review and summary

---

## What Is OpenSWE?

OpenSWE is an **open-source framework for building internal coding agents**, built on
[LangGraph](https://github.com/langchain-ai/langgraph) and
[Deep Agents](https://github.com/langchain-ai/deepagents). It models the architectural patterns
observed in elite engineering organisations (Stripe Minions, Ramp Inspect, Coinbase Cloudbot) and
packages them as a composable, customisable framework.

### At a Glance

| Attribute | Detail |
|---|---|
| **Name** | Open SWE |
| **Origin** | LangChain AI (<https://github.com/langchain-ai/open-swe>) |
| **Language** | Python / LangGraph / Deep Agents |
| **Primary domain** | Coding agent (software development tasks) |
| **Licence** | MIT |
| **Maturity** | Active open-source; production patterns proven at Stripe, Ramp, Coinbase |
| **Invocation surfaces** | Slack, Linear, GitHub PR comments |

### Core Architectural Patterns

OpenSWE establishes seven architectural patterns for autonomous coding agents:

| Pattern | Description |
|---|---|
| **Agent Harness** | Compose on an existing agent framework (Deep Agents / LangGraph) rather than fork or build from scratch; enables upstream improvements |
| **Sandbox Isolation** | Every task runs in its own isolated cloud environment (Modal, Daytona, Runloop); full permissions inside the sandbox, zero blast radius outside |
| **Tool Curation** | Small, focused toolset (~15 tools): shell exec, file read/write/edit, grep, glob, git commit + PR, API calls, ticket comments; quantity avoided in favour of quality |
| **Context Engineering** | Inject `AGENTS.md` (repo conventions and architectural rules) + issue/thread context into the agent's system prompt before execution begins |
| **Subagent Orchestration** | Main agent spawns child agents for parallel subtasks; each child gets its own middleware stack, tool list, and execution sandbox |
| **Middleware Hooks** | Deterministic hooks wrap the agent loop: message-queue injection before model call, PR-creation safety net after loop, error handling middleware |
| **Multi-surface Invocation** | Trigger via Slack mention, Linear issue comment, or GitHub PR comment; deterministic thread IDs route follow-up messages to the same running agent |

---

## Evaluation Against Pheromone Requirements

### Question 1 — Direct Adoption of OpenSWE as a Pheromone Component

| Requirement | OpenSWE capability | Assessment |
|---|---|---|
| Go agent runtime (Constitution Principle III) | ❌ Python / LangGraph | Language mismatch; cannot embed in Go agent binary |
| gRPC service contracts (ADR-003) | ❌ No gRPC | HTTP / LangGraph protocol; incompatible transport |
| 1:Many fleet management (ADR-001) | ❌ Single-task focused | Each run is per-issue, not fleet-wide |
| Digital twin skill interface (ADR-007) | ❌ No twin awareness | No concept of twin model, desired state, or drift |
| Lightweight managed-instance footprint | ❌ Python + LangGraph overhead | Heavy runtime; not suitable for resource-constrained instances |
| Edge / air-gapped deployment | ⚠️ Partial | Cloud sandboxes (Modal/Daytona) require external connectivity |
| MIT licence | ✅ Compatible | No licence incompatibility |

**Conclusion**: OpenSWE cannot be adopted as a direct Pheromone component. The Python/LangGraph
stack, absence of gRPC, and cloud-sandbox model are incompatible with Pheromone's Go runtime,
fleet-management architecture, and managed-instance deployment model.

---

### Question 2 — Architectural Pattern Relevance to Pheromone Agents

Despite the implementation incompatibility, every one of OpenSWE's seven patterns is directly
relevant to Pheromone's agentic AI agent vision (ADR-007).

| OpenSWE Pattern | Pheromone Mapping | Relevance |
|---|---|---|
| **Agent Harness (composition)** | Pheromone's pluggable skill framework (`internal/skill/`) already composes skills around the reasoning loop | ✅ High — validates Pheromone's skill composition approach |
| **Sandbox Isolation** | Each twin's enforcement actions should execute in a temporary, isolated environment before committing to the production instance | ✅ High — new `SandboxedExecution` skill needed for Phase 2 |
| **Tool Curation** | Pheromone's skill set should be curated: shell exec, file ops, git ops, API calls — resist scope creep | ✅ High — formalises the principle for Phase 2 skill design |
| **Context Engineering** | ADR-007 already injects twin desired state + drift report into the reasoning loop; `AGENTS.md` is already used | ✅ High — validates existing practice; extend with structured context assembly |
| **Subagent Orchestration** | Multi-twin coordination (parent agent delegates sub-tasks to per-twin child reasoning cycles) | ✅ High — Phase 2 parallel twin management feature |
| **Middleware Hooks** | Pheromone's post-action hook system (ADR-011) is middleware; extend with pre-model safety checks and PR creation safety nets | ✅ High — direct extension of ADR-011 patterns |
| **Multi-surface Invocation** | Webhook triggers, API invocation (ADR-017), and future operator-facing channels (ADR-015) provide the same invocation surface model | ✅ Moderate — ADR-015/017 cover this; OpenSWE validates the approach |

**Conclusion**: All seven patterns are adoptable as architectural guidance for Pheromone's Phase 2
agentic AI agent skills, implemented natively in Go without any OpenSWE dependency.

---

### Question 3 — DevOps Expert Agent Model Fit

The issue requests evaluation of making each digital twin an **individual agentic SWE/DevOps
expert**. OpenSWE's model maps directly onto this vision:

| OpenSWE role | Pheromone equivalent | Fit |
|---|---|---|
| Coding agent: reads codebase, proposes changes, opens PRs | DevOps agent: reads twin state, proposes config changes, opens IaC PRs | ✅ Excellent |
| Sandboxed environment: cloned repo with full permissions | Isolated twin sandbox: ephemeral container with twin's system profile | ✅ Excellent |
| Tool set: shell, file, git, API | DevOps tool set: shell, file, git, cloud API, package manager | ✅ Excellent |
| Context: `AGENTS.md` + issue description | Context: twin desired state + drift report + `AGENTS.md` | ✅ Excellent |
| Subagents: parallel independent subtasks | Multi-twin parallel remediation tasks | ✅ Excellent |
| PR creation: commits changes, opens GitHub draft PR | IaC PR: commits desired-state changes to GitOps repo, opens PR | ✅ Excellent |
| Middleware safety net: commits if agent didn't | Config enforcement safety net: rolls back or alerts if agent action fails | ✅ Excellent |

Each Pheromone digital twin **can and should** operate as an autonomous DevOps expert using the
same architectural patterns OpenSWE demonstrates for software engineering.

---

### Question 4 — Licence Compatibility

OpenSWE is MIT-licensed. The patterns themselves are general architectural knowledge and carry no
licence constraint. Implementing the patterns in Go has zero licence risk.

**Conclusion**: No licence concerns. The decision is entirely based on technical fit.

---

## Decision

**The OpenSWE architectural patterns are adopted as the design basis for Pheromone's Phase 2
SWE/DevOps Agent Skills enhancement, implemented natively in Go.**

OpenSWE as a Python/LangGraph library is **not adopted** as a dependency. The patterns are
implemented from first principles in Go, aligned with the existing `internal/skill/` framework
and the agentic AI agent model established in ADR-007.

### Adopted Patterns

The following new skills and framework enhancements will be designed according to OpenSWE's
architectural patterns:

#### 1. Sandboxed Execution Skill (`SandboxedExecutionSkill`)

Implements OpenSWE's **Sandbox Isolation** pattern. Every enforcement action that modifies
production state runs first in an ephemeral OCI container (testcontainers-go or native OCI
runtime) to validate correctness and contain blast radius.

```
SandboxedExecutionSkill
  - CreateSandbox(twin_profile) → sandbox_id
  - ExecInSandbox(sandbox_id, command) → result
  - CommitToInstance(sandbox_id) → changeset
  - DiscardSandbox(sandbox_id)
```

#### 2. Curated DevOps Tool Set (`DevOpsToolSkill`)

Implements OpenSWE's **Tool Curation** pattern. A focused set of ~13 tools:

| Tool | Purpose |
|---|---|
| `ShellExec` | Execute shell commands in sandbox or managed instance |
| `FileRead` / `FileWrite` / `FileEdit` | File operations on instance or IaC repo |
| `GitClone` / `GitCommit` / `GitPush` | Git operations for GitOps workflows |
| `OpenPR` | Open a GitHub / GitLab draft PR with change description |
| `APICall` | Authenticated HTTP calls to cloud provider APIs |
| `PackageInstall` / `PackageRemove` | OS package management |
| `ServiceRestart` / `ServiceStatus` | Systemd / container service control |

#### 3. Structured Context Assembly (`ContextAssemblySkill`)

Implements OpenSWE's **Context Engineering** pattern. Assembles a rich context bundle for the
AI reasoning loop before each planning cycle:

```
ContextBundle {
  twin_desired_state    // from TwinControl stream
  twin_actual_state     // from CollectMetrics + CollectLogs
  drift_report          // from DiffModel()
  recent_action_history // from ActionAuditLog (ADR-019)
  agents_md_content     // injected from AGENTS.md
  instance_profile      // OS, packages, running services
}
```

#### 4. Subagent Coordination Skill (`SubagentSkill`)

Implements OpenSWE's **Subagent Orchestration** pattern. Parent agent spawns child reasoning
cycles for independent per-twin or per-subsystem tasks:

```
SubagentSkill
  - SpawnSubagent(task_spec, twin_ids[]) → subagent_id
  - WaitForSubagent(subagent_id) → result
  - CancelSubagent(subagent_id)
  - ListActiveSubagents() → []subagent_status
```

#### 5. Middleware Layer Enhancements

Implements OpenSWE's **Middleware Hooks** pattern, extending ADR-011's post-action hooks:

| Middleware | Description |
|---|---|
| `PrePlanContextCheck` | Validate context bundle completeness before model call; retry if twin state stale |
| `IaCPRSafetyNet` | If agent completes without opening an IaC PR for significant changes, middleware opens one automatically |
| `ActionRollbackMiddleware` | If action validation fails, automatically roll back and emit audit event |
| `MessageQueueInjector` | Injects operator follow-up messages (webhook events, ticket comments) before next model call |

#### 6. GitOps / IaC Workflow Skill (`GitOpsSkill`)

New skill combining OpenSWE's PR creation pattern with Pheromone's twin model GitOps principle:

```
GitOpsSkill
  - CloneIaCRepo(repo_url) → workspace
  - ApplyTwinModelToIaC(twin_model, workspace) → changeset
  - CommitAndOpenPR(changeset, description) → pr_url
  - CheckPRStatus(pr_url) → status
```

### Implementation Phasing

| Phase | Skills | ADR Dependencies |
|---|---|---|
| **Phase 2a** — Tool Curation + Sandbox | `SandboxedExecutionSkill`, `DevOpsToolSkill` | ADR-007, ADR-006 |
| **Phase 2b** — Context Engineering | `ContextAssemblySkill`, `MessageQueueInjector` | ADR-007, ADR-019, ADR-011 |
| **Phase 2c** — Subagent Coordination | `SubagentSkill`, `IaCPRSafetyNet` | ADR-007, ADR-003 |
| **Phase 2d** — GitOps Integration | `GitOpsSkill`, `ActionRollbackMiddleware` | ADR-007, ADR-011 |

---

## Consequences

### Positive

- **Each digital twin becomes a DevOps expert**: autonomous reasoning with sandboxed execution,
  curated tools, and GitOps workflow integration — the original pheromone vision fully realised.
- **Validated by industry practice**: Stripe, Ramp, and Coinbase have proven these patterns at
  scale; building on them reduces design risk.
- **No new dependencies**: patterns implemented in Go using existing ecosystem tooling
  (testcontainers-go, go-git); no Python, LangGraph, or OpenSWE runtime required.
- **Composable with existing architecture**: new skills plug directly into the `internal/skill/`
  framework; middleware extends ADR-011's hook service; no breaking changes to ADR-003 or ADR-007.
- **MIT licence**: zero licence risk; patterns are general architectural knowledge.
- **Observability-preserving**: all new skills emit reasoning traces (ADR-019), action audit
  events (ADR-019), and Prometheus metrics (ADR-018).

### Negative

- **Significant implementation effort**: six new skills + middleware layer = ~Phase 2 milestone
  scope; estimated 80–102 hours of implementation effort across phases.
- **Sandbox overhead**: OCI container-per-enforcement-action adds latency (up to 30 s per sandbox
  lifecycle); acceptable for enforcement actions but must be bypassed for read-only observation.
- **Subagent coordination complexity**: managing child reasoning cycles adds state-tracking
  complexity to the parent agent loop; requires careful lifecycle management to prevent orphaned
  subagents.
- **GitOps dependency**: `GitOpsSkill` requires that operators maintain an IaC repository;
  Pheromone cannot require this but must support it optionally.
- **AI model capability requirement**: structured context assembly and subagent orchestration
  require a reasoning model with sufficient context window and instruction-following capability;
  rule-based Phase 1 reasoner cannot leverage Phase 2b/2c skills fully.

### Non-Changes (Explicit Rejections)

- OpenSWE's **Python / LangGraph runtime** is not adopted. Go remains the implementation language.
- OpenSWE's **cloud sandbox providers** (Modal, Daytona, Runloop) are not integrated. Pheromone
  uses its own OCI-based sandbox implementation or delegates to the instance's container runtime.
- OpenSWE's **Linear / Slack invocation** is not adopted in this ADR. The multi-surface invocation
  pattern is already addressed by ADR-015 (IAM / webhook) and ADR-017 (API gateway); extensions
  to specific tools may be addressed in a future ADR.

### Follow-On Artefacts

This positive decision requires:

1. **Specification**: `.specify/memory/spec-003-swe-devops-agent-skills.md`
2. **Plan**: `.specify/memory/plan-003-swe-devops-agent-skills.md`
3. **GitHub Issues**: one per implementation phase + tech spikes (see below)

---

## Tech Spikes Required Before ADR-020 Acceptance

| Spike | Goal | Estimated Effort |
|---|---|---|
| OCI sandbox lifecycle benchmark | Measure container create/exec/destroy P95 latency on target instance types; validate < 30 s overhead acceptable | 6 hours |
| testcontainers-go integration prototype | Prototype `SandboxedExecutionSkill` using testcontainers-go; validate OCI runtime availability on managed instances | 6 hours |
| go-git `OpenPR` prototype | Prototype GitHub PR creation from Go agent using `go-git` + GitHub API; validate authentication flow | 4 hours |
| Subagent goroutine lifecycle test | Prototype `SubagentSkill` using goroutines + channels; measure leak risk and context propagation | 4 hours |
| Context bundle assembly benchmark | Measure time to assemble full `ContextBundle` from live twin state + drift report; validate < 500 ms | 4 hours |

**Total Spike Effort**: ~24 hours (can run in parallel)

---

## GitHub Issues to File

| Issue | Type | Phase | Effort |
|---|---|---|---|
| ADR-020 tech spike: OCI sandbox lifecycle benchmark | Tech Spike | Phase 2a | 6h |
| ADR-020 tech spike: testcontainers-go SandboxedExecution prototype | Tech Spike | Phase 2a | 6h |
| ADR-020 tech spike: go-git OpenPR prototype | Tech Spike | Phase 2b/2d | 4h |
| ADR-020 tech spike: SubagentSkill goroutine lifecycle | Tech Spike | Phase 2c | 4h |
| ADR-020 tech spike: ContextBundle assembly benchmark | Tech Spike | Phase 2b | 4h |
| Implement SandboxedExecutionSkill in internal/skill/ | Implementation | Phase 2a | 16–20h |
| Implement DevOpsToolSkill curated tool set | Implementation | Phase 2a | 12–16h |
| Implement ContextAssemblySkill + MessageQueueInjector | Implementation | Phase 2b | 12h |
| Implement SubagentSkill + coordination lifecycle | Implementation | Phase 2c | 16–20h |
| Implement GitOpsSkill (go-git + GitHub API) | Implementation | Phase 2d | 12–16h |
| Implement IaCPRSafetyNet + ActionRollbackMiddleware | Implementation | Phase 2d | 8h |
| Review and Accept ADR-020 | ADR Review | — | — |

---

## References

- OpenSWE repository: <https://github.com/langchain-ai/open-swe>
- OpenSWE blog announcement: <https://blog.langchain.com/open-swe-an-open-source-framework-for-internal-coding-agents/>
- ADR-007 (Agentic AI Agent Model — digital twin as agent skill; AI reasoning loop)
- ADR-006 (Agent Lifecycle — skill deployment framework; `internal/skill/`)
- ADR-003 (gRPC Service Contracts — ProposeAction, TelemetryStream for subagent coordination)
- ADR-011 (Post-Action Hooks — middleware hook patterns extended by this ADR)
- ADR-017 (UI/API Gateway — multi-surface invocation complements OpenSWE patterns)
- ADR-018 (Observability Stack — new skills must emit Prometheus metrics)
- ADR-019 (Agent & AI Observability — new skills must emit reasoning traces)
- ADR-013 (Swamp Evaluation — prior evaluation; different rejection outcome; establishes AGPL v3 policy)
- Constitution v2.1.0 — Principle I (Layered Twin Architecture), Principle II (ADR-Driven Decisions),
  Principle III (Go + gRPC), Principle IV (Quality Gates)
- Specification: `.specify/memory/spec-003-swe-devops-agent-skills.md`
- Plan: `.specify/memory/plan-003-swe-devops-agent-skills.md`

---

**Decision Date**: 2026-03-18
**Status**: Accepted — OpenSWE architectural patterns adopted for Phase 2 SWE/DevOps Agent Skills;
OpenSWE Python/LangGraph runtime not adopted as a dependency
