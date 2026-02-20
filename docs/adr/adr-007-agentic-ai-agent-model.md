# ADR 007: Agentic AI Agent Model — Digital Twin as an Agent Skill

## Status
Accepted

## Context

The Pheromone platform was originally described using the term "agent" in a generic sense—lightweight processes that monitor and enforce OS/workload configuration. This ADR clarifies that agents in Pheromone are **agentic AI-capable agents**: autonomous reasoning processes that run on each managed instance and use digital twin management as one of their core *skills*.

This distinction is architecturally significant because:

1. **Original framing**: Agents were conceptualised as lightweight daemons (similar to Telegraf or Fluent Bit) that collect metrics and apply configuration deltas delivered by the server.
2. **Revised framing**: Agents are AI-capable autonomous processes. Each agent can reason about its environment, decide which actions to take, and use the digital twin model as a structured tool (skill) to read state, compute desired changes, and apply them to the system under management.

The motivation for this clarification comes directly from the platform's pheromone metaphor: pheromones encode *intent* and are *interpreted* by receivers. An agentic AI agent interprets the twin model, reasons about the gap between desired and actual state, and acts—without waiting for explicit command-response cycles.

**Impact Areas**:
- Agent internal architecture: must host an AI reasoning loop alongside lifecycle hooks
- Server design: must support agent capability advertisement, skill negotiation, and AI-driven telemetry
- Communication protocol (ADR-003): must convey agent capabilities and allow agents to propose actions
- Discovery: server must understand which AI skills/models each agent exposes
- Twin model (ADR-004): twin becomes a structured data skill available to the agent's reasoning engine

## Decision

**All Pheromone agents are agentic AI-capable agents. Digital twin management is a core skill of each agent.**

### Core Model

```
┌──────────────────────────────────────────────────────────────────┐
│                     Pheromone Agent (per instance)               │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   AI Reasoning Loop                      │   │
│  │  - Observes environment (metrics, logs, events)          │   │
│  │  - Plans actions based on goals / twin desired state     │   │
│  │  - Selects and invokes Skills                            │   │
│  │  - Reflects on outcomes and updates twin model           │   │
│  └───────────────────┬──────────────────────────────────────┘   │
│                      │  invokes                                  │
│          ┌───────────▼────────────────────────────────┐         │
│          │              Agent Skills                  │         │
│          │                                            │         │
│          │  ┌─────────────────────────────────────┐  │         │
│          │  │  Digital Twin Skill                 │  │         │
│          │  │  - ReadTwin(twin_id) → model        │  │         │
│          │  │  - UpdateTwin(twin_id, delta)        │  │         │
│          │  │  - ApplyModel(model) → system        │  │         │
│          │  │  - DiffModel(desired, actual)        │  │         │
│          │  └─────────────────────────────────────┘  │         │
│          │                                            │         │
│          │  ┌─────────────────────────────────────┐  │         │
│          │  │  Metrics Collection Skill           │  │         │
│          │  │  - CollectOSMetrics()               │  │         │
│          │  │  - CollectWorkloadMetrics()         │  │         │
│          │  └─────────────────────────────────────┘  │         │
│          │                                            │         │
│          │  ┌─────────────────────────────────────┐  │         │
│          │  │  Config Enforcement Skill           │  │         │
│          │  │  - EnforceConfig(desired_state)     │  │         │
│          │  │  - ValidateCompliance()             │  │         │
│          │  │  - RollbackConfig(version)          │  │         │
│          │  └─────────────────────────────────────┘  │         │
│          │                                            │         │
│          │  ┌─────────────────────────────────────┐  │         │
│          │  │  Custom / Extensible Skills         │  │         │
│          │  │  - Developer-provided via scaffold  │  │         │
│          │  └─────────────────────────────────────┘  │         │
│          └────────────────────────────────────────────┘         │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │               gRPC Communication Layer                   │   │
│  │  - AgentRegistry  (discovery, capability advertisement)  │   │
│  │  - TwinControl    (twin sync, desired-state delivery)    │   │
│  │  - TelemetryStream (metrics, logs, AI decision traces)   │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

### Digital Twin as an Agent Skill

The digital twin is NOT merely a data structure passively updated by the agent. It is an active **skill** — a structured capability the AI reasoning loop can invoke:

| Skill Method | Description |
|---|---|
| `ReadTwin(twin_id)` | Retrieve the current twin model (desired + actual state) from local cache or server |
| `UpdateTwin(twin_id, delta)` | Merge a state delta into the local twin model and propagate to server |
| `ApplyModel(model)` | Translate a twin model into concrete system operations (package installs, config rewrites, service restarts) |
| `DiffModel(desired, actual)` | Compute the gap between desired and actual twin state; return structured drift report |
| `ProposeAction(rationale, action)` | Agent AI loop proposes an action to the server with a natural-language rationale; server may approve, reject, or modify |

### Agentic AI Loop (Pseudo-Code)

```
loop:
  observations = CollectMetrics() + CollectLogs() + ReadTwin(self.twin_ids)
  goal_state   = GetDesiredState(self.twin_ids)          // from server or local cache
  drift        = TwinSkill.DiffModel(goal_state, observations)

  if drift is significant:
    plan = AIReasoner.Plan(observations, goal_state, drift)
    for action in plan:
      if action.requires_approval:
        approval = Server.ProposeAction(action)        // AI proposes; server decides
        if not approval.granted: continue
      result = ExecuteSkill(action.skill, action.params)
      TwinSkill.UpdateTwin(action.twin_id, result.delta)
      TelemetryStream.Emit(action, result)

  sleep(collection_interval)
```

### Agent Capability Advertisement

When an agent registers (ADR-003 `RegisterRequest`), it MUST advertise its skills and AI model details:

```proto
message AgentCapabilities {
  bool  ai_reasoning_enabled  = 1;   // true for agentic AI agents
  string ai_model_id          = 2;   // e.g., "llm-local-v1", "rule-engine-v2"
  repeated string skills      = 3;   // e.g., ["digital-twin", "metrics", "config-enforce"]
  string skill_contract_version = 4; // version of skill interface supported
}
```

Servers MUST track agent capabilities to:
- Route skill-specific commands only to capable agents
- Support gradual rollout of AI-capable agents alongside legacy agents
- Provide capability-aware observability dashboards

### Server Design Requirements

The management server must evolve to support agentic AI agents:

1. **Capability Registry**: Store and query agent capabilities (AI model, skill versions)
2. **Action Proposal Queue**: Accept `ProposeAction` RPC calls; route to human/automated approval workflows
3. **AI Telemetry Ingestion**: Accept structured AI decision traces (not just metrics/logs), including reasoning steps, tool calls, and outcomes
4. **Skill Contract Management**: Version-manage skill interfaces separately from gRPC transport contracts
5. **Autonomous Action Audit Log**: Persist all agent-proposed and server-approved actions for compliance/debugging

### Discovery Protocol Enhancement

Agent discovery (ADR-003 `AgentRegistry`) MUST be extended to include:

- Capability negotiation on first registration (server selects appropriate twin model version for agent's skill level)
- Health signals for the AI reasoning loop (distinct from process health)
- Skill availability events (agent reports when a skill becomes unavailable, e.g., AI model OOM)

## Consequences

### Positive
- **Autonomous Operations**: Agents can self-heal without waiting for round-trip server commands, reducing latency
- **Intent-Driven Management**: Operators express *intent* (twin desired state); agents reason about *how* to achieve it
- **Extensibility**: New capabilities added as skills without changing agent lifecycle or server protocol
- **Richer Observability**: AI decision traces enable post-hoc analysis of agent reasoning
- **Human-in-the-Loop**: `ProposeAction` allows AI agents to escalate uncertain decisions to operators

### Negative
- **Complexity**: AI reasoning loop adds operational complexity (model versioning, prompt management if LLM-based)
- **Resource Overhead**: AI reasoning requires more CPU/memory than a pure metrics daemon
- **Trust & Safety**: Autonomous AI actions on production systems require robust approval gates and audit trails
- **Skill Interface Versioning**: Skill contracts must be managed alongside gRPC contracts (additional versioning surface)
- **Non-AI Fallback**: Agents that cannot host an AI reasoning loop (resource-constrained edge devices) must degrade gracefully to rule-based operation

### Migration / Compatibility
- **Phase 1 (MVP)**: Agents implement rule-based reasoning loop (no LLM required); digital twin skill interface is established
- **Phase 2**: AI reasoning loop (local model or remote API) plugged in behind the same skill interface
- **Backward Compatibility**: Non-AI agents MUST still function; server MUST detect `ai_reasoning_enabled=false` and use legacy command-response patterns

## Testing Strategy

- **Unit Test (Skill Interface)**: Each skill method testable in isolation with mock twin models
  ```go
  func TestDigitalTwinSkillDiff(t *testing.T) {
      skill := NewDigitalTwinSkill(mockTwinStore)
      desired := &TwinModel{Packages: []string{"nginx=1.24"}}
      actual  := &TwinModel{Packages: []string{"nginx=1.20"}}
      drift   := skill.DiffModel(desired, actual)
      assert.Len(t, drift.PackageDrift, 1)
      assert.Equal(t, "nginx", drift.PackageDrift[0].Name)
  }
  ```
- **Integration Test (Agentic Loop)**: Mock AI reasoner + real skill implementations; verify loop converges on desired state
- **Server Capability Test**: Verify server tracks agent capabilities and routes commands appropriately
- **Action Proposal Test**: Verify `ProposeAction` roundtrip (agent proposes → server approves → agent executes → telemetry emitted)

## Follow-Up ADRs

- **ADR-008** (Proposed): AI model selection and local vs. remote reasoning engine trade-offs
- **ADR-009** (Proposed): Action approval workflow and human-in-the-loop gate design

## References

- Spec-001, User Stories 1-4, FR-001, FR-005, FR-006, FR-008, FR-020
- ADR-001 (Foundation: agent framework decision)
- ADR-003 (gRPC contracts: discovery and streaming)
- ADR-004 (Twin schema: twin as data model)
- ADR-006 (Agent lifecycle: hooks updated by this ADR)
- Constitution Principle I (Layered Twin Architecture), Principle II (Observability), Principle III (Protocol)

---

**Decision Date**: 2026-02-20
**Status Update**: Accepted — establishes agentic AI agent as the canonical agent model for Pheromone
