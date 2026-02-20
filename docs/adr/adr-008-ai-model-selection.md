# ADR 008: AI Model Selection — Local vs. Remote Reasoning Engine

## Status

Proposed

## Context

ADR-007 establishes that every Pheromone agent hosts an **agentic AI reasoning loop** and that digital twin
management is a discrete agent skill. ADR-007 explicitly defers the choice of AI model and reasoning
engine to this ADR (ADR-007 § Follow-Up ADRs).

The core question is: **what AI reasoning engine should each Pheromone agent embed or invoke, and
should that engine run locally on the managed instance or call a remote API?**

This matters because:

1. **Resource diversity**: Managed instances range from cloud VMs (64 GB RAM) to edge devices (512 MB RAM).
   A single solution may not fit all.
2. **Network isolation**: Air-gapped environments cannot call remote APIs. Local inference must be available.
3. **Language alignment**: Pheromone agents are primarily written in Go (ADR-001). Most mature AI frameworks
   are Python-first. Bridging or wrapping strategies carry trade-offs.
4. **Phase dependency**: ADR-007 mandates a **rule-based fallback** for Phase 1 MVP; LLM-backed reasoning
   is Phase 2. The chosen architecture must support both phases on the same skill interface.
5. **Open source preference**: The project constitution and ADR-001 favour open-source, community-supported
   tooling with no vendor lock-in.

### What "AI Model Selection" Means Here

This ADR covers three tightly related sub-decisions:

| Sub-Decision | Question |
|---|---|
| **Reasoning engine type** | Rule engine, local LLM, remote LLM API, or hybrid? |
| **AI agent framework** | Which open-source framework, if any, wraps the reasoning loop? |
| **Host OS recommendation** | Which Linux distribution best supports agent deployment with optional AI tooling? |

---

## Candidates Evaluated

### A. Open-Source AI Agent Frameworks

The following frameworks were evaluated for their suitability as the AI reasoning loop harness:

#### A1. LangChain (Python) + langchaingo (Go)

- **Repository**: <https://github.com/langchain-ai/langchain> / <https://github.com/tmc/langchaingo>
- **Language**: Python (primary); `langchaingo` provides a Go port
- **Model support**: Ollama, OpenAI, Anthropic, Bedrock, Vertex, Hugging Face, and >30 more via adapters
- **Agent support**: ReAct, OpenAI functions, tool-calling agents
- **Strengths**: Largest ecosystem, extensive documentation, active community, `langchaingo` gives Go-native bindings
- **Weaknesses**: Python library is heavyweight (>150 dependencies); `langchaingo` lags behind main Python releases by several months; no built-in skill interface matching ADR-007's design; abstraction can obscure reasoning traces needed for ADR-007 observability
- **Resource footprint**: Python agent ~150–300 MB RAM at idle; Go agent (langchaingo) ~20–40 MB

#### A2. AutoGen (Microsoft, Python)

- **Repository**: <https://github.com/microsoft/autogen>
- **Language**: Python
- **Model support**: OpenAI, Azure OpenAI, Ollama, Anthropic via adapters
- **Agent support**: Multi-agent conversation flows, code-execution agents
- **Strengths**: Strong multi-agent orchestration (useful for future Action Approval in ADR-009); well-maintained by Microsoft; large community
- **Weaknesses**: Designed for multi-agent *conversations*, not single-node system-management agents; Python-only limits Go alignment; resource overhead similar to LangChain; not a natural fit for the observe-plan-act loop in ADR-007
- **Resource footprint**: ~200–350 MB RAM at idle (includes Python runtime)

#### A3. CrewAI (Python)

- **Repository**: <https://github.com/crewAIInc/crewAI>
- **Language**: Python
- **Model support**: OpenAI, Ollama, Anthropic, Groq, and others via LangChain adapters
- **Agent support**: Role-based multi-agent crews with defined tasks and goals
- **Strengths**: Intuitive task/role model; active development; integrates with LangChain tools
- **Weaknesses**: Multi-crew orchestration is overengineered for single-host system management; Python-only; not lightweight enough for edge devices
- **Resource footprint**: ~200–400 MB RAM at idle

#### A4. Ollama (Local LLM Server)

- **Repository**: <https://github.com/ollama/ollama>
- **Language**: Go (server core) + C/C++ (llama.cpp backend)
- **Model support**: Llama 3, Mistral, Gemma, Phi-3, Qwen, and dozens more via GGUF format
- **Agent support**: Not an agent framework — provides an HTTP API for local model inference; agents built on top
- **Strengths**: **Go-native REST API** (ideal for Go agents); runs fully offline; extensive model library; single-binary deployment; used widely in production; supports quantised models (Q4, Q8) for lower resource usage; active development; compatible with OpenAI API format
- **Weaknesses**: Not an agent framework itself; requires model storage (~1–8 GB per model); no built-in reasoning loop; GPU strongly recommended for large models (CPU-only is slow)
- **Resource footprint (model-dependent)**:
  - `phi3.5:3.8b-mini-instruct-q4_K_M` → ~3 GB RAM, adequate on 8 GB instance
  - `llama3.2:3b-instruct-q4_K_M` → ~2.5 GB RAM
  - `qwen2.5:1.5b-instruct-q4_K_M` → ~1.2 GB RAM (viable on constrained edge nodes)
- **Verdict**: **Best fit as the local inference backend** for Pheromone agents; pairs with a thin Go wrapper

#### A5. llamafile (Mozilla Ocho)

- **Repository**: <https://github.com/Mozilla-Ocho/llamafile>
- **Language**: C/C++ (single-file executable embedding model weights)
- **Model support**: Llama, Mistral, Phi, Gemma families; distributed as self-contained binaries
- **Strengths**: Zero-dependency deployment (single file, no runtime); OpenAI-compatible API; excellent for air-gapped edge devices; small models fit in ~1–2 GB
- **Weaknesses**: Less operationally flexible than Ollama (model swap requires new binary); less active development cadence than Ollama; no native model registry
- **Resource footprint**: ~1–4 GB RAM depending on embedded model; minimal OS dependencies
- **Verdict**: Viable alternative for highly constrained or air-gapped edge deployments where Ollama's package manager is unavailable

#### A6. llama.cpp (Georgi Gerganov)

- **Repository**: <https://github.com/ggerganov/llama.cpp>
- **Language**: C/C++ with Go bindings (`go-llama.cpp`, `llama-go`)
- **Model support**: All GGUF-format models
- **Strengths**: Lowest-level control; most resource-efficient; Go bindings available; enables in-process inference (no separate server)
- **Weaknesses**: Go bindings are community-maintained and lag upstream; in-process inference couples model lifecycle to agent process; complex build toolchain
- **Verdict**: Suitable for embedding inference directly into a Pheromone Go agent binary in future phases; too complex for Phase 2 initial implementation

#### A7. DSPy (Stanford NLP)

- **Repository**: <https://github.com/stanfordnlp/dspy>
- **Language**: Python
- **Agent support**: Signature-based prompt programming and self-optimising pipelines
- **Strengths**: Novel approach to prompt optimisation; well-suited for structured decision-making
- **Weaknesses**: Python-only; academic origin means enterprise support is limited; not designed for real-time system-management loops
- **Verdict**: Not suitable as primary reasoning engine; may be useful for offline prompt optimisation experiments

---

### B. AI-Integrated Linux Distributions

Pheromone agents run directly on managed Linux instances. The following distributions were evaluated for
suitability as the **host OS for managed nodes** running Pheromone agents (with optional local AI inference):

#### B1. Ubuntu Server 24.04 LTS (Noble Numbat)

- **Type**: General-purpose server Linux (Canonical)
- **AI tooling**: Ships with Python 3.12, `snap install ollama` available, official NVIDIA/AMD GPU driver support, Ubuntu Pro for compliance
- **Suitability**: ✅ **Excellent** — widest hardware support, well-understood by operators, Ollama installs in one command (`curl -fsSL https://ollama.com/install.sh | sh`), 5-year LTS support window, Go 1.22+ available via `snap`
- **Resource overhead**: ~300–500 MB RAM (minimal install); AI stack adds 1–4 GB
- **Recommendation**: **Primary OS target** for standard cloud/on-premises instances

#### B2. Talos Linux (Sidero Labs)

- **Type**: Immutable, API-driven OS designed for Kubernetes nodes
- **AI tooling**: Containerised workloads only; Ollama runs as a container (via `ghcr.io/ollama/ollama`)
- **Suitability**: ✅ **Very Good** for containerised Pheromone deployments — immutable OS aligns perfectly with the digital twin model (OS state declared, not drifted); API-first design mirrors Pheromone's intent-driven management; minimal attack surface
- **Resource overhead**: ~100–150 MB RAM (OS only); containerised Ollama adds 2–6 GB
- **Recommendation**: **Preferred OS for Phase 2 Kubernetes-native deployments**; its immutable OS model is architecturally aligned with Pheromone's twin-as-desired-state principle

#### B3. Fedora CoreOS (Red Hat)

- **Type**: Immutable, container-optimised OS
- **AI tooling**: Containerised workloads; Ollama as container; toolbox for ephemeral mutable environments
- **Suitability**: ✅ **Good** — immutable, auto-updating (rpm-ostree/Ignition), well-suited for edge deployments, SELinux enforcing by default; Ollama container available
- **Resource overhead**: ~150–200 MB RAM (OS); containerised AI stack on top
- **Recommendation**: Good choice for Red Hat-ecosystem environments or edge deployments requiring SELinux

#### B4. Alpine Linux

- **Type**: Minimal, security-focused Linux
- **AI tooling**: musl libc may conflict with some AI binaries; Ollama and llamafile support Alpine but require testing; Docker/Podman available for containerised inference
- **Suitability**: ⚠️ **Conditional** — excellent for ultra-constrained edge devices where OS footprint must be <50 MB; AI integration is possible but requires extra configuration
- **Resource overhead**: ~5–15 MB RAM (OS only); smallest possible base
- **Recommendation**: Use only for resource-constrained edge agents where a rule-based reasoning fallback (no local LLM) is acceptable; full AI stack not recommended here

#### B5. NVIDIA AI Enterprise Linux (RHEL-based)

- **Type**: Commercial, NVIDIA-optimised
- **AI tooling**: CUDA, cuDNN, NCCL, Triton Inference Server, pre-integrated GPU drivers
- **Suitability**: ⚠️ **Vendor-specific** — excellent GPU AI performance but commercial licence required; not suitable as a general-purpose Pheromone agent OS; may be used for dedicated Pheromone AI model servers (not per-agent)
- **Recommendation**: Not recommended as a per-agent OS; may be considered for centralised model-serving infrastructure in Phase 3

#### B6. Debian 12 Bookworm

- **Type**: Stable, community-driven server Linux
- **AI tooling**: Ollama via `.deb` package or install script; Python 3.11 available; good hardware support
- **Suitability**: ✅ **Good** — conservative release cycle ensures stability; widely used in embedded/server contexts; Ollama support equivalent to Ubuntu
- **Recommendation**: Viable alternative to Ubuntu Server for operators already running Debian

---

## Decision

### Reasoning Engine Architecture

**Adopt a tiered local-first, remote-optional architecture for the AI reasoning engine:**

```
┌──────────────────────────────────────────────────────────────────┐
│                   Pheromone Agent Reasoning Stack                │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Tier 0 — Rule-Based Reasoner (Phase 1 MVP, always on)  │   │
│  │  - Deterministic if/then rules derived from twin diff    │   │
│  │  - Zero external dependencies                            │   │
│  │  - Always available as fallback                          │   │
│  └──────────────────────────────┬───────────────────────────┘   │
│                                 │ optional upgrade               │
│  ┌──────────────────────────────▼───────────────────────────┐   │
│  │  Tier 1 — Local LLM via Ollama HTTP API (Phase 2+)       │   │
│  │  - Ollama server (Go binary) on same node or LAN         │   │
│  │  - Models: phi3.5:mini (3.8B, Q4) or qwen2.5:1.5b        │   │
│  │  - Go agent calls Ollama REST API (OpenAI-compatible)    │   │
│  │  - Fully offline; no egress required                     │   │
│  └──────────────────────────────┬───────────────────────────┘   │
│                                 │ optional upgrade               │
│  ┌──────────────────────────────▼───────────────────────────┐   │
│  │  Tier 2 — Remote LLM API (Phase 2+, cloud deployments)  │   │
│  │  - OpenAI / Anthropic / Azure OpenAI / self-hosted vLLM  │   │
│  │  - Same Go interface as Tier 1 (OpenAI-compatible API)   │   │
│  │  - Rate limiting, cost tracking, audit logging required  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Interface (ADR-007 AIReasoner):                                 │
│    Plan(observations, goal_state, drift) → []Action              │
│    (implementation switches between tiers via config)            │
└──────────────────────────────────────────────────────────────────┘
```

### AI Agent Framework Selection

**Select Ollama as the recommended local AI inference backend, with a thin custom Go wrapper
implementing the ADR-007 `AIReasoner` interface.** No third-party agent framework (LangChain,
AutoGen, CrewAI) is adopted at this time.

**Rationale**:

1. **Go-native**: Ollama is written in Go and exposes an OpenAI-compatible REST API; Go agents call it
   with the standard `net/http` client — no Python dependency required.
2. **Lightweight models**: Quantised small models (phi3.5:mini Q4, qwen2.5:1.5b Q4) run within the
   resource budget of typical managed instances without a GPU.
3. **Offline-first**: Ollama operates entirely without internet once models are pulled; critical for
   air-gapped deployments.
4. **Extensible**: The `AIReasoner` interface (defined in ADR-007) abstracts the reasoning engine;
   langchaingo or other frameworks can be plugged in later without changing agent skill interfaces.
5. **llamafile** is designated as the **edge-device fallback** where Ollama cannot be installed
   (ultra-constrained nodes ≤1 GB RAM), using the smallest available GGUF model (~500 MB Q2).

**Framework selection summary**:

| Framework | Decision | Rationale |
|---|---|---|
| Ollama | ✅ **Selected** (Tier 1 local) | Go-native API, offline, quantised models, active community |
| llamafile | ✅ **Selected** (edge fallback) | Zero-dependency single binary, air-gapped edge devices |
| langchaingo | 🔄 **Deferred to Phase 3** | Adds ecosystem if multi-model routing needed; adopt when complexity justifies |
| AutoGen | ❌ Not selected | Multi-agent conversation focus; Python-only; over-engineered for single-node management |
| CrewAI | ❌ Not selected | Python-only; multi-crew model not aligned with single-instance agent design |
| llama.cpp (Go bindings) | 🔄 **Deferred to Phase 3** | In-process inference; consider when minimising sidecar processes is critical |

### Host OS Recommendation

**Ubuntu Server 24.04 LTS** is the **primary recommended OS** for standard deployments.
**Talos Linux** is the **preferred OS for Kubernetes-native and immutable-infrastructure deployments**.

| Deployment Context | Recommended OS | Rationale |
|---|---|---|
| Cloud VM / bare-metal server | Ubuntu Server 24.04 LTS | Widest support, Ollama one-liner install, LTS lifecycle |
| Kubernetes node / immutable infra | Talos Linux | Immutable OS aligns with digital twin desired-state model |
| Edge device (constrained) | Alpine Linux + llamafile binary | Minimal footprint; rule-based reasoning fallback preferred |
| Red Hat ecosystem | Fedora CoreOS or RHEL 9 | SELinux, enterprise support, Ollama via container |
| GPU-accelerated inference node | Ubuntu 24.04 LTS + NVIDIA CUDA | Best GPU driver ecosystem for Ollama GPU mode |

### ADR-007 `AIReasoner` Interface (Go)

```go
// reasoner.go — thin wrapper over Ollama or rule engine; selected via config

// AIReasoner is the interface the agent framework invokes each reasoning cycle.
type AIReasoner interface {
    Plan(ctx context.Context, obs Observations, goal GoalState, drift DriftReport) ([]Action, error)
}

// RuleBasedReasoner — Tier 0, Phase 1 MVP; no external dependencies.
type RuleBasedReasoner struct{}

func (r *RuleBasedReasoner) Plan(_ context.Context, _ Observations, _ GoalState, drift DriftReport) ([]Action, error) {
    return drift.ToActions(), nil  // deterministic rule: fix every detected drift item
}

// OllamaReasoner — Tier 1, Phase 2; calls local Ollama HTTP API.
type OllamaReasoner struct {
    BaseURL string  // e.g., "http://localhost:11434"
    Model   string  // e.g., "phi3.5:3.8b-mini-instruct-q4_K_M"
}

func (o *OllamaReasoner) Plan(ctx context.Context, obs Observations, goal GoalState, drift DriftReport) ([]Action, error) {
    prompt := buildSystemManagementPrompt(obs, goal, drift)
    resp, err := ollamaGenerate(ctx, o.BaseURL, o.Model, prompt)
    if err != nil {
        // Fall back to rule-based on Ollama failure (ADR-007 §Migration/Compatibility)
        return (&RuleBasedReasoner{}).Plan(ctx, obs, goal, drift)
    }
    return parseActionsFromResponse(resp), nil
}

// RemoteAPIReasoner — Tier 2, Phase 2+; calls OpenAI-compatible remote API.
type RemoteAPIReasoner struct {
    BaseURL string  // OpenAI, Anthropic, or self-hosted vLLM endpoint
    Model   string
    APIKey  string
}
```

---

## Consequences

### Positive

- **No Python runtime required** on managed instances for Phase 1 or Phase 2 (Go + Ollama binary only)
- **Air-gap compatibility**: Tier 1 (Ollama) runs fully offline after initial model pull
- **Graceful degradation**: Tier 0 rule-based fallback ensures agent functionality even if Ollama is unavailable (ADR-007 §Migration/Compatibility)
- **Vendor-neutral**: Ollama supports >100 open-weight models; no lock-in to OpenAI or any other vendor
- **Unified interface**: `AIReasoner` interface makes tier switching transparent to agent skill code
- **Resource-scaled deployment**: Small models (qwen2.5:1.5b Q4 ~1.2 GB) work on instances with 4 GB+ RAM; rule fallback covers sub-4 GB nodes
- **OS alignment**: Talos Linux's immutable OS model directly validates the digital twin desired-state concept in practice

### Negative

- **Model storage overhead**: Even the smallest useful models require 1–2 GB disk; instances with <4 GB RAM must use rule-based fallback
- **Ollama sidecar process**: Adds an additional process to manage (lifecycle, health monitoring, resource limits)
- **Model quality vs. size trade-off**: Small quantised models (1.5B–3.8B) may produce suboptimal reasoning compared to large models; requires evaluation per use-case
- **Observability of reasoning**: LLM outputs are non-deterministic; AI decision traces (ADR-007) must capture full prompt+response for debugging
- **Model updates**: Pulling updated model versions on production nodes requires operational process (rolling updates, bandwidth management)
- **Talos Linux learning curve**: Operators unfamiliar with immutable OS patterns face a steeper on-boarding curve

### Migration / Compatibility

- **Phase 1**: `RuleBasedReasoner` is the only implemented tier; `AIReasoner` interface established
- **Phase 2**: `OllamaReasoner` added behind feature flag (`ai_reasoning_enabled=true` in agent config); `RuleBasedReasoner` remains default
- **Phase 2+**: `RemoteAPIReasoner` added for cloud deployments with egress; selected via `ai_model_id` field in `AgentCapabilities` proto (ADR-007)
- **Backward Compatibility**: Server MUST continue to support agents with `ai_reasoning_enabled=false` (ADR-007 §Migration/Compatibility)

---

## Recommended Model Profiles

| Profile | Model | Quantisation | RAM Required | Use Case |
|---|---|---|---|---|
| **Edge minimal** | `qwen2.5:1.5b-instruct` | Q4_K_M | ~1.2 GB | Resource-constrained nodes ≥4 GB RAM |
| **Standard** | `phi3.5:3.8b-mini-instruct` | Q4_K_M | ~3.0 GB | Standard cloud instances ≥8 GB RAM |
| **Quality** | `llama3.2:8b-instruct` | Q4_K_M | ~5.5 GB | High-capability nodes ≥16 GB RAM |
| **Rule-based** | N/A | N/A | 0 MB (overhead) | Nodes <4 GB RAM or air-gapped with no Ollama |

---

## Testing Strategy

- **Unit Test (RuleBasedReasoner)**: Verify deterministic output for known drift reports; no external dependencies
- **Unit Test (OllamaReasoner)**: Mock Ollama HTTP API; verify prompt construction and response parsing
- **Integration Test (Ollama live)**: Spin up Ollama with `phi3.5:mini` in CI; verify agent reasoning loop produces valid `[]Action`
- **Fallback Test**: Kill Ollama mid-cycle; verify agent falls back to `RuleBasedReasoner` without crashing
- **Resource Test**: Measure agent + Ollama RAM/CPU on `phi3.5:mini` and `qwen2.5:1.5b` to validate profile table above

```go
func TestRuleBasedReasonerPlanConverges(t *testing.T) {
    r := &RuleBasedReasoner{}
    drift := &DriftReport{PackageDrift: []PackageDrift{{Name: "nginx", Desired: "1.24", Actual: "1.20"}}}
    actions, err := r.Plan(context.Background(), Observations{}, GoalState{}, *drift)
    assert.NoError(t, err)
    assert.Len(t, actions, 1)
    assert.Equal(t, "update-package", actions[0].Skill)
}

func TestOllamaReasonerFallsBackOnError(t *testing.T) {
    r := &OllamaReasoner{BaseURL: "http://localhost:99999", Model: "phi3.5:mini"}
    drift := &DriftReport{PackageDrift: []PackageDrift{{Name: "nginx", Desired: "1.24", Actual: "1.20"}}}
    actions, err := r.Plan(context.Background(), Observations{}, GoalState{}, *drift)
    assert.NoError(t, err)   // fallback, not error
    assert.NotEmpty(t, actions)
}
```

---

## Follow-Up ADRs

- **ADR-009** (Proposed): Action approval workflow and human-in-the-loop gate design
- **ADR-010** (Proposed): Ollama model lifecycle management (pull, update, rollback) across managed fleet

---

## References

- ADR-007 (Agentic AI Agent Model — establishes `AIReasoner` interface and `ai_model_id` capability field)
- ADR-006 (Agent Lifecycle — framework `Run()` loop invokes `AIReasoner.Plan()`)
- ADR-003 (gRPC Contracts — `AgentCapabilities.ai_model_id` carries selected reasoner identifier)
- Ollama project: <https://github.com/ollama/ollama>
- llamafile project: <https://github.com/Mozilla-Ocho/llamafile>
- langchaingo: <https://github.com/tmc/langchaingo>
- phi-3.5 model card: <https://ollama.com/library/phi3.5>
- Qwen2.5 model card: <https://ollama.com/library/qwen2.5>
- Talos Linux: <https://www.talos.dev/>
- Ubuntu Server 24.04 LTS: <https://ubuntu.com/server>
- Fedora CoreOS: <https://fedoraproject.org/coreos/>
- Constitution v2.1.0 — Principle I (Layered Twin Architecture), Principle III (Language-Agnostic Protocol)

---

**Decision Date**: 2026-02-20
**Status**: Proposed — ready for team review
