# Contributing to Pheromone

Thank you for contributing! This guide covers everything you need to know.

## Getting Started

```bash
git clone https://github.com/abuxton/pheromone.git
cd pheromone
go mod download
make deps
```

**Requirements**: Go 1.24+, Docker (for etcd/NATS integration tests), Make

## Branch Naming

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/ADR-NNN-short-name` | `feature/ADR-005-nats-spike` |
| Bugfix | `bugfix/issue-NNN-description` | `bugfix/issue-42-grpc-timeout` |
| Docs | `docs/description` | `docs/update-contributing` |

## Commit Message Format

```
<type>(<scope>): <description> - ADR-NNN

[optional body]

Co-authored-by: ...
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`

Examples:
- `feat(skill): implement OllamaReasoner - ADR-008`
- `fix(api): handle nil twin state gracefully`
- `chore(ci): add CodeQL workflow`

## Development Workflow

1. **Create a feature branch** from `main`
2. **Implement with tests** — new features MUST include unit tests
3. **Run the test suite**:
   ```bash
   make test              # unit tests + race detector
   make lint              # golangci-lint
   make validate-offline  # offline validation suite
   ```
4. **Open a PR** targeting `main`

## ADR Process

Material architecture changes MUST be documented in an ADR before implementation:

1. Copy `docs/adr/adr-NNN-template.md` (or use the next sequential number)
2. Document: Context → Decision → Rationale → Consequences
3. Status: `Proposed` → `Accepted` (after spike/validation) → `Deprecated`
4. Reference the ADR in your commit messages and PR description

See `docs/adr/INDEX.md` for the full ADR inventory.

## Pull Request Requirements

- [ ] Tests pass locally (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] New features have accompanying unit tests
- [ ] ADR created/updated for architectural changes
- [ ] PR description references the issue (`Closes #NNN`)

## Testing

```bash
make test              # Unit tests with race detector
make test-grpc-spike   # gRPC load tests
make validate-offline  # Offline validation (no Docker needed)
make validate          # Full suite (requires Docker for etcd)
```

Integration tests require Docker:
```bash
make etcd-up
go test ./... -tags integration
make etcd-down
```

## Reporting Issues

- **Security vulnerabilities**: Use [GitHub private vulnerability reporting](https://github.com/abuxton/pheromone/security/advisories/new) — do NOT open a public issue
- **Bugs**: Use the bug report template
- **Feature requests**: Use the feature request template

## Code Style

- Follow standard Go idioms and `gofmt` formatting
- Errors must be wrapped with context: `fmt.Errorf("doing X: %w", err)`
- Interfaces should be defined at the consumer, not the producer
- See `docs/adr/` for architectural constraints
