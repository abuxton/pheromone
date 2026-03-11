# copilot-instructions.md

This file provides Copilot-specific guidance for working in this repository.

## Primary Reference

Use `./AGENTS.md` as the source of truth for project workflow, architecture, and quality expectations.

@/workspace/AGENTS.md

## Required Temporary Path Policy

Use `./tmp/` for all temporary files created during development, tests, scripts, diagnostics, and generated scratch output.

- Prefer paths such as `./tmp/<feature-or-task>/...`
- Do not use `/tmp/...` for repository work

## Speckit Agent Usage

Prefer the local Speckit agent set in `.github/agents/` when work maps to artifact-driven flow:

- `speckit.specify` -> `speckit.clarify` -> `speckit.plan` -> `speckit.tasks` -> `speckit.analyze` -> `speckit.implement`

Use `speckit.checklist` for requirement-quality checks, `speckit.constitution` for constitution updates, and `speckit.taskstoissues` when turning tasks into GitHub issues.

## Skill-Aware Workflow

Use relevant local skills from `.agents/skills/` to improve implementation quality, especially:

- Go quality skills (`go-*`) when editing Go packages
- Architecture/spec skills for design and planning work
- OpenSpec skills for lifecycle operations and artifact management

## Clarifications And Research

When clarification is needed, use `vscode_askQuestions` with focused questions.

Use subagents and parallel read-only discovery when it improves speed and isolation, then summarize findings concisely before editing.

## Maintaining Guidance

When new stable patterns are discovered:

- Update existing `AGENTS.md` files with concise, reusable guidance
- Create scoped `AGENTS.md` files in subdirectories only when needed
- Capture practical implementation notes, pitfalls, and dependency constraints
