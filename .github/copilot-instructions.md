# Copilot Instructions for Pheromone

## Project Overview

Pheromone is a digital twin platform implementing a 1:Many model for managing operating system and workload configurations through lightweight agents and efficient protocols. It is inspired by pheromone-based communication used by the Aliens from Ridley Scott's franchise—decentralised, observable, and adaptive.

This project follows the **Pheromone Constitution v2.0.0** (`.specify/memory/constitution.md`). All development MUST align with its five core principles.

## Core Principles (from Constitution v2.0.0)

### I. Layered Digital Twin Architecture
Every component MUST align with the 1:Many digital twin model:
- **OS-Level Twins**: OS configurations, infrastructure metrics, resource allocation
- **Workload-Level Twins**: Application services and dependencies
- **Management Layer**: Real-time orchestration across both twin levels

### II. ADR-Driven Decision Contracts & Observability
- Every architecturally-significant decision MUST be captured in an ADR **before** implementation begins
- ADRs live in `./docs/adr/` using present-tense imperative verb filenames (e.g., `choose-database.md`)
- ADRs MUST include: Status (`Proposed`/`Accepted`/`Superseded`), Context, Decision, Consequences
- ADRs are living documents — append dated updates, never delete old text
- All agents MUST implement autonomous metric reporting; observability instrumentation is NOT optional

### III. Language-Agnostic Protocol Foundation
- **Go**: Rapid prototyping, agent development, server implementation
- **Rust**: Performance-critical components, memory-safe low-level control
- All inter-component communication MUST use gRPC with Protocol Buffers
- Protocol contracts MUST be version-managed and backward-compatible

### IV. Smoke Tests & Quality Gates (NON-NEGOTIABLE)
- Every PR MUST pass automated smoke tests before merge
- Go: `go test ./... -v -race`; Rust: `cargo test --all`
- Coverage targets: ≥70% new code (unit), ≥50% critical paths (integration)
- Pre-commit hooks enforce linting and formatting on every local commit

### V. Git-Driven ADR Workflow & DevOps Toolchain
- Significant decisions MUST be formalized as ADRs **before** feature branch creation
- Branch naming: `feature/ADR-NNN-descriptive-name` or `bugfix/issue-NNN-description`
- Commit message format: `feat(scope): description - ADR-NNN` (conventional commits)
- Squash-merge to `develop`; merge `develop` → `main` only via release branches

## Repository Layout

```
docs/adr/              Architectural Decision Records (primary location)
specs/                 Feature specifications and the project constitution
  constitution.md      Governing principles and development guidelines
.specify/memory/       Speckit workflow artifacts (plans, specs, constitution)
  constitution.md      Constitution v2.0.0 (authoritative)
.github/
  agents/              Custom Copilot coding-agent definitions
  prompts/             Speckit workflow prompt templates
speckit.yml            Speckit workflow configuration
.pre-commit-config.yaml Pre-commit hooks
```

## Branching Strategy (GitFlow — MANDATORY)

| Branch | Purpose |
|---|---|
| `main` | Production; tagged on every release; commits via release/hotfix only |
| `develop` | Integration; base for all feature work |
| `feature/ADR-NNN-*` | Feature branches; merge back to `develop` via PR |
| `release/vX.Y.Z` | Release prep; merges to both `main` and `develop` |
| `hotfix/vX.Y.Z-*` | Emergency fixes from `main`; merges to both `main` and `develop` |

## ADR-Driven Development Workflow

1. **Create ADR** in `./docs/adr/adr-NNN-descriptive-name.md` (Status: Proposed)
2. **ADR Review** — minimum 2 approvals before Status → Accepted
3. **Speckit Plan** — `plan` agent generates `.specify/memory/plan-NNN.md` from accepted ADR
4. **Specify Feature** — `specify` agent generates `.specify/memory/spec-NNN.md`
5. **Create Feature Branch** — named `feature/ADR-NNN-descriptive-name`
6. **Implement** — `tasks` agent creates checklist; commits reference ADR-NNN
7. **PR Submission** — include ADR link, spec link, coverage report; squash-merge with `feat: ADR-NNN description`

## Technology Stack

- **Primary language**: Go (`gofmt`, `golangci-lint`, `go vet`)
- **Performance-critical**: Rust (`rustfmt`, `clippy`, `cargo check`)
- **Agent↔server protocol**: gRPC with Protocol Buffers (versioned packages, e.g., `pheromone.v1`)
- **Event streaming**: Kafka (Phase 2); NATS (MVP)
- **Distributed state store**: etcd or Consul

## Pre-commit Hooks

```bash
pre-commit install          # install hooks once
pre-commit run --all-files  # run manually across the whole repo
```

Hooks enforce: YAML/JSON syntax, end-of-file newlines, large files (>500 KB), merge-conflict markers, private-key detection, ShellCheck, `gofmt`, `golangci-lint`, `rustfmt`, `clippy`.

## Quality Gates (before merging to `main`)

- [ ] All tests pass (Go + Rust smoke tests)
- [ ] Code review approved (minimum 2 reviewers)
- [ ] ADR created and accepted (if architecturally significant)
- [ ] Documentation updated
- [ ] Spec updated (if feature change)
- [ ] No security vulnerabilities
- [ ] Coverage ≥70% new code; ≥50% critical paths

## Key References

- [Constitution v2.0.0](.specify/memory/constitution.md)
- [ADR Index](docs/adr/INDEX.md)
- [Summary](summary.md)
- [Speckit Config](speckit.yml)
