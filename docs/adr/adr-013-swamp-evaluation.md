# ADR 013: Swamp Evaluation — System Initiative AI Automation CLI

## Status

Proposed

## Context

This ADR documents a formal review of **Swamp** (<https://github.com/systeminit/swamp>), an
open-source AI-native automation CLI from [System Initiative](https://www.systeminit.com/), for
potential suitability or inclusion in the Pheromone platform as an integration, developer tool,
or architectural reference.

The review was requested as a spike (see linked issue) with the following acceptance criteria:

1. Write-up on Swamp's approach, architecture, capabilities, and coding model
2. ADR entry created and raised for review
3. Updates to related documentation
4. Additional issues raised as required

---

## What Is Swamp?

Swamp is an **AI-native automation CLI** designed to help AI coding agents create operational
workflows that are reviewable, shareable, and accurate. It functions as a structured layer
between AI agents (Claude Code, Cursor, Codex, OpenCode) and real operational targets (cloud
APIs, infrastructure, CLI tools).

### At a Glance

| Attribute | Detail |
|---|---|
| **Name** | Swamp |
| **Origin** | System Initiative (<https://github.com/systeminit/swamp>) |
| **Language** | TypeScript / [Deno](https://deno.land/) (latest) |
| **Deployment** | CLI — runs on the local developer machine; single-machine scope |
| **State location** | `.swamp/` directory inside a Git repository |
| **AI agent integrations** | Claude Code (default), Cursor, OpenCode, Codex — via bundled skill files |
| **Licence** | GNU AGPL v3 with [Swamp Extension and Definition Exception](https://github.com/systeminit/swamp/blob/main/COPYING-EXCEPTION) |
| **Maturity** | Open Alpha — breaking changes expected; active development |
| **Telemetry** | Anonymous usage telemetry collected; opt-out available |
| **Contribution model** | Issue-driven only; external PRs automatically closed |

### Core Concepts

Swamp structures automation around six first-class concepts:

| Concept | Description |
|---|---|
| **Models** | Typed representations of external systems (cloud resources, CLI tools, APIs). Each model type defines metadata, arguments, methods, and inputs. Written in TypeScript. |
| **Definitions** | YAML files that instantiate a model type with specific configuration. Support [CEL (Common Expression Language)](https://cel.dev/) expressions for dynamic values and cross-model references. |
| **Workflows** | Orchestrate model method executions across parallel jobs and steps, with dependency ordering and trigger conditions. |
| **Data** | Versioned, immutable artifacts (resources, logs, files) produced by method runs. Searchable by tags. |
| **Vaults** | Secure storage for secrets and credentials, referenced in definitions via CEL expressions. |
| **Tags** | Key-value labels on definitions, workflows, and data. Flow from definitions to produced data; overridable at runtime. |

Everything lives in a `.swamp/` directory with human-friendly symlink views under
`/models/` and `/workflows/`.

### AI Agent Skills Model

Swamp ships first-class skills for four AI coding tools:

| Tool | Init flag | Skills dir | Instructions file |
|---|---|---|---|
| Claude Code | *(default)* | `.claude/skills/` | `CLAUDE.md` |
| Cursor | `--tool cursor` | `.cursor/skills/` | `.cursor/rules/swamp.mdc` |
| OpenCode | `--tool opencode` | `.agents/skills/` | `AGENTS.md` |
| Codex | `--tool codex` | `.agents/skills/` | `AGENTS.md` |

When `swamp repo init` is run, skill files are written into the appropriate directory for the
chosen tool. The AI agent discovers them automatically and learns how to work with Swamp — how to
search for models, create definitions, run workflows, manage vaults, etc.

### Architecture Overview

```
Developer machine
├── .swamp/                  # All state lives here (in Git)
│   ├── models/              # Model type definitions (TypeScript)
│   ├── definitions/         # YAML instantiations with CEL expressions
│   ├── workflows/           # Workflow YAML files
│   ├── data/                # Versioned immutable artifacts
│   └── vaults/              # Encrypted secrets
├── .claude/skills/          # Swamp skill files (gitignored) — Claude Code
├── .cursor/skills/          # Swamp skill files (gitignored) — Cursor
└── extensions/models/       # Custom TypeScript model extensions
```

The Swamp binary (compiled from TypeScript/Deno) executes locally, using ambient environment
variables (AWS credentials, SSH keys, kubeconfig) to reach external systems. No credentials
leave the local machine unless a model explicitly calls an external API.

---

## Evaluation

### Question 1 — Suitability as Core Pheromone Infrastructure

Pheromone requires server-side infrastructure that manages a **1:Many fleet** of remote agents
(ADR-001, ADR-002, ADR-007). Swamp is explicitly a **single-machine, developer-local CLI**.

| Requirement | Swamp capability | Assessment |
|---|---|---|
| 1:Many fleet management | ❌ Single-machine scope | Fundamental architectural mismatch |
| Agent registration and heartbeat (gRPC `AgentRegistry`) | ❌ No network server | Not a server; has no listener |
| Digital twin state sync (gRPC `TwinControl`) | ❌ No equivalent | Local `.swamp/` state only |
| Bidirectional telemetry streaming (`TelemetryStream`) | ❌ No equivalent | No streaming protocol |
| In-memory + etcd hybrid state (ADR-002) | ❌ Git-based local state | Incompatible persistence model |
| Go implementation (Constitution Principle III) | ❌ TypeScript/Deno | Language misalignment |
| gRPC service contracts (ADR-003) | ❌ No gRPC | CLI tool; no RPC surface |
| Runs on managed instances (agent-side) | ❌ Developer tooling only | Requires Deno runtime (~50 MB) |

**Conclusion**: Swamp is not suitable as Pheromone's server or agent runtime. It is a
developer tool, not an infrastructure platform.

---

### Question 2 — Suitability as a Pheromone Agent Implementation

The Pheromone agentic AI agent model (ADR-007) requires each managed instance to run an
autonomous AI reasoning loop with digital twin skills.

| Requirement (ADR-007) | Swamp capability | Assessment |
|---|---|---|
| Autonomous reasoning loop (observe → plan → act → reflect) | ❌ Passive CLI | Swamp does not run autonomously; it requires agent/human invocation |
| Digital twin skill (`ReadTwin`, `UpdateTwin`, `ApplyModel`) | ❌ No twin model | Swamp has no awareness of Pheromone twin models |
| Go agent runtime (ADR-006) | ❌ TypeScript/Deno | Runtime incompatible with Go agent scaffold |
| Lightweight footprint for managed instances | ❌ Deno + TypeScript overhead | Requires full Deno runtime; heavier than a compiled Go binary |
| gRPC capability advertisement (ADR-003/007) | ❌ Not supported | No gRPC client |
| Edge/air-gapped deployment | ⚠️ Partial | Binary is self-contained, but models can call external APIs |

**Conclusion**: Swamp cannot serve as a Pheromone agent implementation.

---

### Question 3 — Suitability as Developer Tooling / Workflow Orchestration

This is the most interesting area. Swamp's approach to AI-assisted operational workflows has
direct relevance to Pheromone's development and operational workflows:

| Area | Swamp's approach | Relevance to Pheromone |
|---|---|---|
| **Skill files for AI agents** | `.claude/skills/`, `.cursor/skills/`, `AGENTS.md` | ✅ High — Pheromone already uses `.claude/` and `AGENTS.md` for agent guidance |
| **YAML Definitions with CEL expressions** | `definitions/*.yaml` with CEL | ⚠️ Moderate — mirrors Pheromone twin model YAML schemas (ADR-004); similar patterns, different scope |
| **Versioned data artifacts** | Immutable data objects in `.swamp/data/` | ⚠️ Moderate — aligns with digital twin state versioning concept |
| **Vault-based secret management** | `.swamp/vaults/`; CEL-referenced | ⚠️ Moderate — useful pattern; Pheromone credential management not yet specified |
| **Parallel workflow execution** | Jobs and steps with dependency ordering | ✅ Relevant — similar to Pheromone's twin update propagation graph |
| **Git-committed state** | Everything in `.swamp/` in the repo | ✅ High — aligns with Pheromone's GitOps principle for twin model definitions |
| **Model types as code** | TypeScript models with typed inputs/outputs | ⚠️ Language mismatch — concept is transferable; Go structs + proto definitions achieve the same |

**Observations**:

1. **Skills model**: Swamp's `swamp repo init` writes skill files to `.claude/skills/` or
   `AGENTS.md`, teaching the AI agent how to use the tool. Pheromone already places guidance in
   `AGENTS.md` and `.claude/`. Adopting Swamp's structured skills-directory pattern could
   improve how Pheromone's Speckit workflows and implementation guides are surfaced to AI agents.

2. **YAML definitions + CEL**: Swamp's Definition files (YAML with CEL expressions) closely
   mirror the approach Pheromone is considering for ADR-004 (Twin Model Schema). Swamp's design
   provides a reference data point for this decision.

3. **Workflow orchestration**: Swamp's job/step/dependency model for parallel execution
   is a mature reference implementation of the kind of workflow orchestration Pheromone will
   need for twin reconciliation tasks.

4. **Issue-driven contribution model**: System Initiative's choice to close fork PRs and use
   an issue-driven, internally-implemented model for supply chain security is noteworthy. This
   approach may be worth considering for Pheromone's contribution governance.

---

### Question 4 — Licence Compatibility

Swamp is licensed under **GNU AGPL v3** with a *Swamp Extension and Definition Exception*.

| Consideration | Detail |
|---|---|
| **Core licence** | AGPL v3 — requires source disclosure when the software is used over a network |
| **Extension exception** | Exempts user-defined TypeScript model extensions and YAML definitions from AGPL copyleft |
| **Impact if adopted** | Any Pheromone service that *runs* the Swamp binary as a dependency over a network would trigger AGPL's network use clause |
| **Precedent** | ADR-010 (Signal Protocol) was rejected partly due to AGPL v3; same constraint applies here |
| **Governance** | No current project governance decision to accept AGPL dependencies |

The extension exception reduces the risk for Swamp's *model definitions*, but the core CLI
binary remains AGPL v3. Embedding or distributing Swamp as part of the Pheromone platform
would require explicit governance approval and source disclosure obligations.

**Conclusion**: AGPL v3 creates a licence incompatibility with Pheromone's current dependency
policy. Swamp can be used as an *external development tool* on developer machines without
triggering AGPL (local use does not constitute network distribution), but it cannot be bundled
as a Pheromone platform component without governance acceptance of AGPL.

---

### Evaluation Summary

| Role | Verdict | Key Reason |
|---|---|---|
| Core server infrastructure | ❌ Not suitable | Single-machine CLI; no 1:Many fleet model; no gRPC |
| Pheromone agent implementation | ❌ Not suitable | TypeScript/Deno; no twin model awareness; no gRPC client |
| Direct dependency / bundled component | ❌ Not suitable | AGPL v3 licence incompatibility |
| Developer workflow tool (external) | ✅ Suitable | Can be used by developers on their local machines without licence risk |
| Architectural reference for twin model definitions | ✅ Valuable | YAML + CEL Definition model informs ADR-004 design |
| Architectural reference for AI agent skills pattern | ✅ Valuable | Skills-directory approach is directly relevant to Pheromone's AI coding agent guidance |
| Workflow orchestration reference | ⚠️ Informative | Job/step/dependency model is a useful reference; not adoptable directly |

---

## Decision

**Swamp is not adopted as a Pheromone platform component, dependency, or integration target.**

The AGPL v3 licence, TypeScript/Deno stack, single-machine CLI architecture, and absence of
gRPC or fleet management capability make Swamp incompatible with Pheromone's core requirements.

**Three specific outcomes are captured from this evaluation:**

1. **ADR-004 reference**: Swamp's YAML Definition + CEL expression model is noted as a
   mature reference implementation for Pheromone's twin model schema design. The ADR-004
   authors should review Swamp's definition format when finalising the twin model schema.

2. **Skills directory pattern**: Swamp's approach of placing structured skill files in AI
   agent directories (`.claude/skills/`, `AGENTS.md`) is adopted as an *informal pattern* for
   improving Pheromone's existing AI agent guidance. This does not require any Swamp dependency
   — the pattern is already used natively in Pheromone's `AGENTS.md` and `.claude/` structure.

3. **Issue-driven contribution governance**: System Initiative's issue-driven contribution
   model (no external PRs; plan-then-implement) is noted as a supply chain security measure
   worth considering for Pheromone's contribution governance in a separate discussion.

---

## Consequences

### Positive

- Confirms that no existing external tool meets Pheromone's 1:Many fleet management requirements
  from a server or agent perspective; the custom Go implementation remains the correct path.
- Swamp's Definition/CEL model validates the YAML-based twin schema direction explored in
  ADR-004. The design space is well-trodden.
- The AI agent skills directory pattern is identified as applicable to Pheromone's development
  workflow without adding any dependency.
- Developers on the team can use Swamp as a personal productivity tool on their local machines
  without impacting the platform's dependency posture.

### Negative

- No shortcut to workflow orchestration or twin model management — these must be implemented
  natively in Go as planned.
- The AGPL v3 licence means that if System Initiative builds capabilities Pheromone eventually
  needs, adopting them directly would require governance acceptance of AGPL.

### Follow-on Issues

The following issues should be raised as a result of this evaluation:

| Issue | Type | Priority |
|---|---|---|
| Review Swamp's YAML Definition + CEL schema as reference input for ADR-004 twin model design | Research | Medium |
| Consider issue-driven contribution governance model for Pheromone (supply chain security) | Governance | Low |

---

## References

- Swamp repository: <https://github.com/systeminit/swamp>
- Swamp licence: [AGPL v3 + Swamp Extension and Definition Exception](https://github.com/systeminit/swamp/blob/main/COPYING)
- System Initiative: <https://www.systeminit.com/>
- CEL specification: <https://cel.dev/>
- ADR-004 (Twin Model Schema Format — YAML + JSON Schema; Swamp Definition model is a reference)
- ADR-007 (Agentic AI Agent Model — defines autonomous reasoning loop; Swamp does not meet this)
- ADR-006 (Agent Lifecycle — Go agent scaffold; language incompatibility with TypeScript/Deno)
- ADR-003 (gRPC Service Contracts — Swamp has no gRPC interface)
- ADR-010 (Evaluate Signal Protocol — established AGPL v3 as a concern; same applies here)
- Constitution v2.1.0 — Principle I (Layered Twin Architecture), Principle III
  (Language-Agnostic Protocol Foundation: Go + Rust)

---

**Decision Date**: 2026-03-02
**Status**: Proposed — ready for team review
