# ADR 009: OpenClaw Evaluation — Central Server and Agent Role Assessment

## Status

Proposed

## Context

This ADR extends ADR-008 (AI Model Selection — Local vs. Remote Reasoning Engine) following a
review request to evaluate **OpenClaw** (<https://github.com/openclaw/openclaw>) as both an agent
solution and a central server at the core of the Pheromone platform.

The proposition: *Could an OpenClaw-based server be placed at the centre of the implementation,
with all Pheromone agents communicating with it over a supported communication channel?*

Three specific questions must be answered:

1. **Communication channel**: Which channel is feasible in a non-UI, headless agent-to-agent
   infrastructure?
2. **LLM suitability**: Which LLM would be appropriate, or is OpenClaw itself the LLM in this
   model?
3. **Agent evaluation impact**: Does this alter the ADR-007/ADR-008 agent evaluation? Can
   OpenClaw serve as both agent and server?

---

## What Is OpenClaw?

OpenClaw is a **personal AI assistant gateway** written in TypeScript/Node.js.

| Attribute | Detail |
|---|---|
| **Language** | TypeScript / Node.js (requires Node ≥ 22) |
| **Role** | Personal AI assistant orchestration gateway |
| **Deployment** | Single-user daemon (`openclaw gateway`) on local device |
| **Consumer channels** | WhatsApp, Telegram, Slack, Discord, Signal, iMessage, Microsoft Teams, Google Chat, Matrix, WebChat, Zalo |
| **LLM backends** | Anthropic (Claude — recommended), OpenAI (GPT); community adapters for Ollama and others |
| **Skills/plugins** | Bundled skills + npm plugin ecosystem (ClawHub); MCP support via `mcporter` |
| **Agent capability** | Computer use, tool calling, agentic task execution |
| **Architecture** | Request-response assistant loop; human in the loop by design |
| **Licence** | MIT |

OpenClaw is **not** an LLM itself; it is an orchestration layer that routes prompts to external
model providers (Anthropic, OpenAI, local Ollama, etc.).

---

## Evaluation

### Question 1 — Communication Channel for Headless Agent-to-Agent Infrastructure

OpenClaw's native communication channels are all **consumer-facing messaging platforms** designed
for human interaction:

| Channel | Headless suitability | Notes |
|---|---|---|
| WhatsApp / Telegram / Signal / iMessage | ❌ Not suitable | Require human accounts, phone numbers, and UI-oriented flows |
| Slack / Discord / Teams / Google Chat | ❌ Not suitable | Require workspace/bot setup; rate-limited; not designed for machine-to-machine telemetry |
| Matrix | ⚠️ Marginal | Open protocol; machine accounts possible; but latency, federation overhead, and message-size limits make it unsuitable for high-frequency twin sync |
| WebChat | ⚠️ Marginal | HTTP-based; programmatic access possible but requires browser-context session management |
| **HTTP REST API (gateway port)** | ✅ Technically feasible | `openclaw gateway --port 18789` exposes an HTTP API; Go clients can POST requests programmatically |
| **MCP via mcporter** | ⚠️ Indirect | Model Context Protocol tool-call bridge; designed for tool integration not fleet management |

**Assessment for Pheromone agents:**

The only feasible headless channel is the **HTTP REST gateway API**. However:

- This is a generic HTTP/JSON endpoint — functionally equivalent to calling Ollama or any other
  REST-based inference backend directly.
- Pheromone agents already use **gRPC** (ADR-003) for all agent↔server communication. OpenClaw
  has no gRPC interface and cannot replace the gRPC services defined in ADR-003
  (`AgentRegistry`, `TwinControl`, `TelemetryStream`).
- Using the OpenClaw HTTP API as a communication layer would require each agent to maintain an
  HTTP client to OpenClaw *in addition to* its existing gRPC connections — adding complexity
  with no architectural benefit.
- OpenClaw does **not** support high-frequency, bidirectional streaming (as required by ADR-003
  `TwinControl`/`TelemetryStream`); it is optimised for low-frequency conversational turns.

**Conclusion**: No OpenClaw channel is suitable as a replacement for Pheromone's gRPC-based
agent-to-server communication. The HTTP gateway API is the only programmatic option, and it is
inferior to the established gRPC contracts for fleet management.

---

### Question 2 — LLM Suitability / Is OpenClaw Its Own LLM?

OpenClaw is **not** an LLM. It is an orchestration gateway that delegates all AI reasoning to
external providers:

| Provider | Status in OpenClaw | Pheromone compatibility |
|---|---|---|
| Anthropic (Claude Pro/Max — **recommended**) | Primary | ❌ Remote API; requires internet egress; conflicts with ADR-008 offline-first requirement |
| OpenAI (GPT-4o, Codex) | Supported | ❌ Remote API; same egress conflict |
| Ollama (local) | Community adapter | ⚠️ Supported but adds an unnecessary routing hop (Agent → OpenClaw → Ollama) vs. the ADR-008 decision (Agent → Ollama directly) |
| llamafile | Not natively supported | ❌ No adapter available |
| vLLM / self-hosted | Not natively supported | ❌ No adapter available |

**The LLM routing chain when OpenClaw is used as an intermediary:**

```
Pheromone Agent  →  OpenClaw HTTP Gateway  →  LLM Provider (Anthropic/OpenAI/Ollama)
```

Versus the ADR-008-approved architecture:

```
Pheromone Agent  →  Ollama HTTP API (local)  →  Local model (phi3.5:mini / qwen2.5:1.5b)
```

Routing through OpenClaw adds an extra network hop, a Node.js process, and external API
dependencies — without adding any capability that the ADR-008 `OllamaReasoner` does not already
provide natively in Go.

**Conclusion**: OpenClaw is not itself an LLM. Using it as an intermediary for model access
introduces unnecessary complexity and breaks the local-first, offline-capable architecture
established in ADR-008. The ADR-008 tiered reasoning stack (Tier 0 rule-based → Tier 1 Ollama
→ Tier 2 remote API) remains the correct choice.

---

### Question 3 — Can OpenClaw Act as Both Agent and Server? Does This Alter the Agent Evaluation?

#### OpenClaw as Server

| Requirement (from ADR-002/003) | OpenClaw capability | Assessment |
|---|---|---|
| Agent registration and heartbeat (gRPC `AgentRegistry`) | ❌ No equivalent | Not implemented |
| Digital twin state sync and desired-state push (`TwinControl`) | ❌ No equivalent | Not implemented |
| Bidirectional metric and telemetry streaming (`TelemetryStream`) | ❌ No equivalent | Not implemented |
| In-memory + etcd hybrid state management (ADR-002) | ❌ No state management | Stateless gateway |
| Agent capability registry (ADR-007/ADR-002 update) | ❌ Not supported | No fleet topology concept |
| Action proposal queue (ADR-007 `ProposeAction`) | ❌ Not supported | Assistant-to-human, not server-to-fleet |
| 1:Many fleet management model | ❌ Designed for 1:1 (personal assistant) | Core architectural conflict |
| Go implementation (Constitution Principle III) | ❌ TypeScript/Node.js | Language misalignment |

OpenClaw cannot serve as Pheromone's central management server without effectively rebuilding
Pheromone inside OpenClaw — which defeats the purpose.

#### OpenClaw as Agent

| Requirement (from ADR-007) | OpenClaw capability | Assessment |
|---|---|---|
| Autonomous AI reasoning loop (observe → plan → act → reflect) | ⚠️ Partial | OpenClaw can execute agentic tasks via skills/computer-use, but operates in a request-response model driven by human or channel input, not an autonomous timed loop |
| Digital twin skill (`ReadTwin`, `UpdateTwin`, `ApplyModel`, `DiffModel`) | ❌ Not supported | No twin model awareness |
| Go implementation for agent runtime | ❌ TypeScript/Node.js | Go agent scaffold (ADR-006) not applicable |
| Lightweight resource footprint for managed instances | ❌ Node.js runtime overhead | Node ≥ 22 + npm package: ~50–200 MB RAM vs. ~10–30 MB for a Go Pheromone agent |
| gRPC capability advertisement and skill contract (ADR-007/003) | ❌ Not supported | No gRPC client |
| Edge/air-gapped deployment | ❌ npm-dependent; online model APIs preferred | Conflicts with air-gapped requirement |

OpenClaw cannot be used as a Pheromone agent implementation in place of the ADR-006 Go agent scaffold.

#### Partial Complementary Role

There is **one valid use case** for OpenClaw within the Pheromone ecosystem, distinct from
the server or agent roles:

> **Operator-Interface Gateway**: OpenClaw can act as a conversational bridge between human
> operators and the Pheromone management server's HTTP/gRPC control plane. Operators using
> Slack, Discord, or Teams could query fleet state, trigger configuration pushes, or review
> action proposals through an OpenClaw skill/plugin that calls Pheromone's API.

This is an **optional operator UX layer**, not a core infrastructure component. It does not
alter the fundamental agent or server architecture established in ADR-007, ADR-008, or ADR-003.

---

## Decision

**OpenClaw is not adopted as Pheromone's central server or agent implementation.**

The ADR-008 architecture (tiered local-first reasoning: Tier 0 rule-based → Tier 1 Ollama →
Tier 2 remote API) and ADR-007 agentic AI agent model remain unchanged.

### Rationale Summary

| Question | Answer |
|---|---|
| **Feasible communication channel** | HTTP REST gateway is the only programmatic option; unsuitable for high-frequency gRPC-based fleet management. No viable channel exists for headless agent-to-agent infrastructure. |
| **LLM suitability** | OpenClaw is not an LLM; it routes to Anthropic/OpenAI (remote, egress-required). Using it as an intermediary adds hops without benefit. ADR-008's Ollama-direct approach is preferred. |
| **Agent and server dual role** | OpenClaw cannot act as Pheromone's server (no gRPC, no twin management, no fleet topology, 1:1 model). It cannot act as a Pheromone agent (Node.js runtime, no twin skill, no gRPC capability advertisement). Its skills/computer-use model does not align with ADR-007's autonomous systems management loop. |

### Framework Selection Update

The ADR-008 framework selection table is extended:

| Framework | Decision | Rationale |
|---|---|---|
| Ollama | ✅ **Selected** (Tier 1 local) | Go-native API, offline, quantised models |
| llamafile | ✅ **Selected** (edge fallback) | Zero-dependency, air-gapped edge devices |
| OpenClaw | ❌ **Not selected** (core infra) | TypeScript/Node.js; 1:1 personal assistant model; no gRPC; remote API dependency; no twin skill; not suitable as server or agent |
| OpenClaw | ⚠️ **Optional** (operator UX layer) | May be used as conversational interface for human operators over Slack/Discord/Teams — out of scope for core platform |
| langchaingo | 🔄 **Deferred to Phase 3** | Multi-model routing; adopt when complexity justifies |
| AutoGen | ❌ Not selected | Python-only; multi-agent conversation model |
| CrewAI | ❌ Not selected | Python-only; multi-crew model not aligned |
| llama.cpp (Go bindings) | 🔄 **Deferred to Phase 3** | In-process inference; future consideration |

---

## Consequences

### Positive

- ADR-008's Ollama-based, Go-native, offline-first reasoning architecture is confirmed as the
  correct choice — OpenClaw evaluation provides additional validation that no available
  personal-assistant gateway meets Pheromone's infrastructure management requirements.
- The operator UX layer opportunity (OpenClaw ↔ Pheromone API bridge) is identified as a
  low-risk, optional enhancement for future phases without impacting core architecture.

### Negative

- No new AI frameworks are added; the evaluation confirms the existing decision rather than
  expanding options.
- Teams interested in natural-language operator interfaces must build a separate integration
  layer (outside ADR-008/009 scope).

### Migration / Compatibility

- No changes to existing ADRs (001–008) are required.
- ADR-007 `AIReasoner` interface and ADR-008 tiered reasoning stack remain the canonical
  architecture.
- Any future OpenClaw-as-operator-interface integration should be captured in a separate ADR
  (ADR-010 or later) as an optional platform extension.

---

## References

- ADR-008 (AI Model Selection — establishes tiered reasoning architecture this ADR extends)
- ADR-007 (Agentic AI Agent Model — defines `AIReasoner` interface and agent skill model)
- ADR-006 (Agent Lifecycle — Go agent scaffold; language constraints)
- ADR-003 (gRPC Service Contracts — communication protocol that OpenClaw cannot satisfy)
- ADR-002 (Server Architecture — in-memory + etcd state management; no OpenClaw equivalent)
- OpenClaw project: <https://github.com/openclaw/openclaw>
- OpenClaw vision: <https://raw.githubusercontent.com/openclaw/openclaw/main/VISION.md>
- Constitution v2.1.0 — Principle I (Layered Twin Architecture), Principle III
  (Language-Agnostic Protocol Foundation)

---

**Decision Date**: 2026-02-21
**Status**: Proposed — ready for team review
