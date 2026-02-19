# Copilot Instructions for Pheromone

## Project Overview

Pheromone is a digital twin platform implementing a 1:Many model for managing operating system and workload configurations through lightweight agents and efficient protocols. It is inspired by pheromone-based communication used by the Aliens from Ridley Scott's franchise—decentralised, observable, and adaptive.

## Repository Layout

```
ADR/                   Legacy ADR documents (superseded by docs/adr/)
docs/                  Project documentation
  adr/                 Architectural Decision Records (primary location)
specs/                 Feature specifications and the project constitution
  constitution.md      Governing principles and development guidelines
  example-*/           Example feature specifications
.github/
  agents/              Custom Copilot coding-agent definitions
  prompts/             Speckit workflow prompt templates
speckit.yml            Speckit workflow configuration
.pre-commit-config.yaml Pre-commit hooks (YAML lint, large-file checks, etc.)
```

## Technology Stack

- **Primary language**: Go (rapid development, good performance)
- **Performance-critical components**: Rust
- **Agent↔server protocol**: gRPC
- **Telemetry / message queue**: Kafka
- **Distributed state store**: etcd or Consul

## Development Workflow (Speckit)

All feature development follows a specification-driven workflow using Speckit:

1. **specify** – write a feature spec before any code
2. **clarify** – refine requirements through Q&A
3. **plan** – create an implementation plan
4. **tasks** – break the plan into actionable tasks
5. **checklist** – validate quality gates
6. **analyze** – cross-artifact consistency check
7. **implement** – execute the tasks

For architecturally significant decisions, create an ADR in `docs/adr/` using the naming convention `adr-{number:03d}-{title-slug}.md` and include *Context*, *Decision*, and *Consequences* sections. Valid statuses: `Proposed`, `Accepted`, `Deprecated`, `Superseded`, `Rejected`.

## Code Quality Standards

- Write self-documenting, simple code; handle all error cases explicitly.
- Maintain **>80% code coverage** with unit tests.
- Use `golangci-lint` for Go and `rustfmt`/`clippy` for Rust.
- All changes require a code review and must pass CI before merge.
- Keep pull requests small and focused; reference related issues and ADRs.

## Testing Requirements

- **Unit tests**: required for all new code.
- **Integration tests**: test component interactions.
- **Performance benchmarks**: required for critical paths.
- **Documentation tests**: validate code examples in docs.

## Security Guidelines

- TLS for all network communication.
- Authenticate and authorize every agent.
- Validate all external input.
- Run dependency security scans regularly.

## Pre-commit Hooks

The repository uses `pre-commit` to enforce baseline quality on every commit:

```bash
pre-commit install          # install hooks once
pre-commit run --all-files  # run manually across the whole repo
```

Hooks check YAML syntax, JSON syntax, end-of-file newlines, large files (>500 KB), merge-conflict markers, private keys, and shell scripts (ShellCheck).

## Quality Gates (before merging to `main`)

- [ ] All tests pass
- [ ] Code review approved
- [ ] Documentation updated
- [ ] ADR created (if architectural change)
- [ ] Spec updated (if feature change)
- [ ] No security vulnerabilities
- [ ] Performance benchmarks acceptable

## Key References

- [Project Constitution](../specs/constitution.md)
- [Summary](../summary.md)
- [ADR Index](../docs/INDEX.md)
- [Speckit Config](../speckit.yml)
