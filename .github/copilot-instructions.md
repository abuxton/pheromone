# copilot-instructions.md

This file provides Copilot-specific guidance for working in this repository.
Full agent operating contract is in `./AGENTS.md` (source of truth for workflow, architecture, and quality).

## Repository Overview

**Pheromone** is a distributed digital-twin platform inspired by pheromone-based communication. It implements a 1:Many digital twin model across three architectural layers (OS-Level Twins, Workload-Level Twins, Management Layer). Agents are **agentic AI-capable** — each runs an autonomous AI reasoning loop on the managed instance; digital twin management is a core *skill* of that loop.

Primary languages: **Go 1.24+** (agents, server) and **Rust** (performance-critical paths). All inter-component communication uses **gRPC / Protocol Buffers**. See `./summary.md` and `./docs/adr/INDEX.md` for architecture context.

## Operating Contract

| Rule | Requirement |
|------|-------------|
| Instruction priority | `./AGENTS.md` → scoped `AGENTS.md` → this file → `./.specify/memory/constitution.md` |
| Temporary paths | ALWAYS use `./tmp/<task>/...` for temp files |
| Forbidden temp paths | NEVER use `/tmp/...` for repository work |
| Architecture authority | Align with ADRs in `./docs/adr/` and constitution in `./.specify/memory/constitution.md` |
| Protocol contracts | Keep gRPC/protobuf compatibility and observability requirements |

## Project Map

| Area | Path | Notes |
|------|------|-------|
| ADR index | `docs/adr/INDEX.md` | Architecture decisions and rationale |
| Constitution | `.specify/memory/constitution.md` | Non-negotiable project principles |
| Core Go code | `internal/` | Main implementation |
| Entrypoints | `cmd/` | Binaries (`pheromone-agent`, `pheromone-server`) |
| Speckit agents | `.github/agents/` | Feature lifecycle agent prompts |
| Local skills | `skills/` | Reusable implementation guidance |
| Temp workspace | `tmp/` | Required scratch/output location |

## Build, Test, and Lint Commands

```bash
# Install dependencies
go mod download
make deps

# Run unit tests with race detector
make test
# or directly:
go test $(go list ./... | grep -v '^github.com/abuxton/pheromone/benchmark$') -v -race

# Quick compilation/test-wiring check (no test execution)
go test ./... -run '^$'

# Lint (golangci-lint)
make lint

# Offline validation suite
make validate-offline

# Rust components (when applicable)
cargo test --all -- --nocapture

# Pre-commit checks (run before committing)
pre-commit run --all-files
```

Coverage targets: ≥70% for new Go code, ≥50% for integration paths.

## Branching and Commit Conventions

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/ADR-NNN-short-name` | `feature/ADR-015-grpc-streams` |
| Bugfix | `bugfix/issue-NNN-description` | `bugfix/issue-42-grpc-timeout` |
| Docs | `docs/description` | `docs/update-contributing` |

Commit format: `<type>(<scope>): <description> - ADR-NNN`
Examples: `feat(agent): add gRPC streaming - ADR-015`, `fix(api): handle nil twin state gracefully`

## ADR-Driven Workflow (Mandatory)

1. Identify a significant architectural decision
2. Create an ADR in `./docs/adr/adr-NNN-descriptive-name.md` before implementation
3. Reference the ADR number in the feature branch name, PR description, and squash-commit message
4. Use Speckit agents to flow from decision → specification → implementation

All PRs MUST include: ADR reference, spec link (if generated), test coverage report, and alignment summary.

## Speckit Agent Routing

Prefer agents in `.github/agents/` for artifact-driven work:

| Stage | Agent | Key Output |
|-------|-------|------------|
| Specify | `speckit.specify` | `spec.md` |
| Clarify | `speckit.clarify` | Clarified `spec.md` |
| Plan | `speckit.plan` | `plan.md`, design docs |
| Tasks | `speckit.tasks` | `tasks.md` |
| Analyze | `speckit.analyze` | Consistency report |
| Implement | `speckit.implement` | Code + updated task state |
| Checklist | `speckit.checklist` | Requirement checklists |
| Constitution | `speckit.constitution` | Updated constitution |
| Tasks→Issues | `speckit.taskstoissues` | GitHub issues |

Preferred lifecycle: `speckit.specify` → `speckit.clarify` → `speckit.plan` → `speckit.tasks` → `speckit.analyze` → `speckit.implement`

## Skill-Aware Workflow

Use relevant skills from `skills/` to improve implementation quality:

- Go quality skills (`go-*`) when editing Go packages
- Architecture/spec skills for design and planning work
- OpenSpec skills for lifecycle operations and artifact management

Key Go skills: `go-development`, `go-style-core`, `go-code-review`, `go-concurrency`, `go-error-handling`, `go-testing`

## Code Style and Quality

- Follow standard Go conventions enforced by `gofmt`, `golangci-lint`, `go vet`
- All new code paths MUST include structured logging, telemetry, and AI decision traces
- Observability instrumentation is NON-OPTIONAL — emit metrics, logs, and reasoning traces
- gRPC services MUST include version number in package name (e.g., `pheromone.v1`)
- ADRs MUST be created before implementing any architecturally-significant change

## Clarifications and Research

When clarification is needed, ask focused questions before making changes.

Use subagents and parallel read-only discovery when it improves speed and isolation, then summarize findings concisely before editing.

## Maintaining Guidance

When new stable patterns are discovered:

- Update `./AGENTS.md` with concise, reusable guidance
- Create scoped `AGENTS.md` files in subdirectories only when needed
- Capture practical implementation notes, pitfalls, and dependency constraints

## Primary References

- `./.specify/memory/constitution.md` — non-negotiable architectural principles
- `./docs/adr/INDEX.md` — architecture decision records
- `./AGENTS.md` — full agent operating contract and skill routing
- `./summary.md` — project summary and digital twin model
- `./speckit.yml` — Speckit configuration
