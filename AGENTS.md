# AGENTS.md

Agent operating guide for `pheromone`. Keep this file concise, executable, and pointer-based.

## Operating Contract

| Rule | Requirement |
|------|-------------|
| Instruction order | Read: `./AGENTS.md` -> scoped `AGENTS.md` -> `./.github/copilot-instructions.md` -> `./.specify/memory/constitution.md` |
| Temporary paths | ALWAYS use `./tmp/<task>/...` for temp files |
| Forbidden temp paths | NEVER use `/tmp/...` for repository work |
| Architecture authority | Align with ADRs in `./docs/adr/` and constitution in `./.specify/memory/constitution.md` |
| Contracts | Keep gRPC/protobuf contract compatibility and observability requirements |

## Project Map

| Area | Path | Notes |
|------|------|-------|
| ADR index | `docs/adr/INDEX.md` | Architecture decisions and rationale |
| Constitution | `.specify/memory/constitution.md` | Non-negotiable project principles |
| Core Go code | `internal/` | Main implementation |
| Entrypoints | `cmd/` | Binaries (`pheromone-agent`, `pheromone-server`) |
| Speckit agents | `.github/agents/` | Feature lifecycle agent prompts |
| Local skills | `.agents/skills/` | Reusable implementation guidance |
| Temp workspace | `tmp/` | Required scratch/output location |

## Speckit Agent Routing (`.github/agents`)

| Stage | Agent | Use For | Key Output |
|------|-------|---------|------------|
| Specify | `speckit.specify` | Create/update feature specification | `spec.md` |
| Clarify | `speckit.clarify` | Resolve ambiguity in active spec | Clarified `spec.md` |
| Plan | `speckit.plan` | Technical planning and design artifacts | `plan.md`, design docs |
| Tasks | `speckit.tasks` | Dependency-ordered implementation tasks | `tasks.md` |
| Analyze | `speckit.analyze` | Read-only cross-artifact consistency check | Analysis report |
| Implement | `speckit.implement` | Execute planned implementation tasks | Code and updated task state |
| Checklist | `speckit.checklist` | Requirement-quality checklists | Checklist docs |
| Constitution | `speckit.constitution` | Update constitution and template alignment | Updated constitution |
| Tasks to issues | `speckit.taskstoissues` | Convert tasks into GitHub issues | GitHub issues |

Preferred lifecycle:

1. `speckit.specify`
2. `speckit.clarify`
3. `speckit.plan`
4. `speckit.tasks`
5. `speckit.analyze`
6. `speckit.implement`

## Skill Routing (`.agents/skills`)

| Category | Skills |
|----------|--------|
| Agent rules | `agent-rules` |
| Planning and architecture | `context-map`, `architecture-blueprint-generator`, `create-architectural-decision-record`, `create-specification`, `create-technical-spike`, `devops-rollout-plan` |
| Go development and quality | `go-development`, `go-style-core`, `go-code-review`, `go-concurrency`, `go-context`, `go-control-flow`, `go-data-structures`, `go-defensive`, `go-documentation`, `go-error-handling`, `go-functional-options`, `go-interfaces`, `go-linting`, `go-naming`, `go-packages`, `go-performance`, `go-testing` |
| OpenSpec workflow | `openspec-new`, `openspec-continue`, `openspec-ff`, `openspec-apply`, `openspec-verify`, `openspec-sync`, `openspec-archive`, `openspec-bulk-archive`, `openspec-config`, `openspec-schema`, `openspec-install`, `openspec-initial`, `openspec-onboard`, `openspec-update`, `openspec-explore` |
| Documentation and delivery | `create-readme`, `create-agentsmd`, `create-github-action-workflow-specification`, `create-github-pull-request-from-specification`, `create-github-issue-feature-from-specification`, `create-github-issues-feature-from-implementation-plan`, `create-github-issues-for-unmet-specification-requirements`, `create-tldr-page`, `ai-prompt-engineering-safety-review` |
| Data and enterprise checks | `data-tools`, `enterprise-readiness` |
| Utility | `gh-cli`, `first-ask`, `skill-creator` |
| File formats | `docx`, `pdf`, `pptx`, `xlsx` |

Placeholder-only skills (do not use as authoritative guidance):

- `openspec-template`
- `template-skill`

## Verified Command Matrix (2026-03-11)

| Command | Status | Notes |
|--------|--------|-------|
| `go test ./... -run '^$'` | Verified | Executes quickly and validates package compilation/test wiring |
| `go test $(go list ./... | grep -v '^github.com/abuxton/pheromone/benchmark$') -v -race` | Verified | Practical race smoke-test excluding heavy benchmark package |
| `pre-commit --version` | Verified | `pre-commit 4.5.1` present |
| `cargo --version` | Verified | `cargo 1.94.0` present |
| `pre-commit install` | Not run in this update | Use before local commits |
| `pre-commit run --all-files` | Not run in this update | Use for full lint/format sweep |

## Development Workflow Rules

1. Map relevant code/docs before edits (`context-map` preferred for broad changes).
2. Create or update ADRs before implementing material architecture or contract changes.
3. Keep spec/plan/tasks synchronized with implementation for larger changes.
4. Update tests and docs in the same change set.
5. Write all temporary outputs to `./tmp` and keep them deterministic.

## Branching And Commits

| Topic | Convention |
|-------|------------|
| Feature branch | `feature/ADR-NNN-descriptive-name` |
| Bugfix branch | `bugfix/issue-NNN-description` |
| Commit format | `feat(scope): description - ADR-NNN` |

## Primary References

- `./.specify/memory/constitution.md`
- `./docs/adr/INDEX.md`
- `./summary.md`
- `./speckit.yml`
