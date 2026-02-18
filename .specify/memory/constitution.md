<!-- Sync Impact Report
Version: 1.1.0 → 2.0.0 (MAJOR: ADR-driven workflow integration with Speckit alignment)
Modified principles:
  - Principle II: Added explicit ADR decision contracts and linking
  - Principle V: Integrated ADR filing into Git/DevOps workflow
Added sections:
  - ADR-Driven Workflow (new - decision lifecycle tied to features)
  - Speckit Workflow Integration (new - plan→specify→implement→tasks alignment)
  - Amendment procedure updated to require ADR for all governance changes
Rationale: ADRs become first-class decision artifacts; feature work flows from decision→specification→implementation
Status: Constitutional upgrade to ADR-first with Speckit workflow integration
-->

# Pheromone Constitution

## Core Principles

### I. Layered Digital Twin Architecture
Every component MUST align with the 1:Many digital twin model comprising three architectural layers:
- **OS-Level Twins**: Manage operating system configurations, infrastructure metrics, and resource allocation
- **Workload-Level Twins**: Manage application services and their dependencies
- **Management Layer**: Orchestrate real-time insights and adjustments across both twin levels

Architecture MUST support modular separation of concerns, enabling independent development and testing of each layer. Scalability and observability MUST be achieved through hierarchical digital twin mapping.

### II. ADR-Driven Decision Contracts & Observability
**Architecture Decision Records (ADRs) are non-negotiable decision contracts** that guide all significant architectural work:

- Every architecturally-significant decision MUST be captured in an ADR before implementation begins (see ./ADR/ directory)
- ADR file naming: present-tense imperative verb, lowercase, dashes, markdown (e.g., `choose-database.md`, `implement-grpc-streams.md`)
- ADRs MUST include: Status (Proposed/Accepted/Superseded), Context, Decision, Consequences, follow-up ADRs
- ADRs are LIVING DOCUMENTS; updates MUST include date stamps and rationale (do not delete old text)
- No feature branch MUST reference an ADR; all ADRs MUST be linked in PR description and squash-commit message

All agents MUST implement autonomous metric reporting and enforcement with full observability:
- Real-time bidirectional communication between agents and management layer is MANDATORY
- All agent implementations MUST include structured logging and telemetry collection
- Observability instrumentation is NOT optional—all components MUST emit metrics for monitoring and debugging
- Rationale: ADRs create shared understanding of "why"; observability ensures "what" and "why" remain synchronized at runtime

### III. Language-Agnostic Protocol Foundation
The platform MUST support Go and Rust as primary implementation languages, with clear protocol contracts:
- **Go**: Preferred for rapid prototyping, agent development, and server implementation
- **Rust**: Preferred for performance-critical components and memory-safe low-level control
- All inter-component communication MUST use gRPC with Protocol Buffers for schema definition
- Alternative protocols (Kafka for event streaming, MQTT for pub/sub) MUST be evaluated against gRPC baseline
- Protocol contracts MUST be version-managed and backward-compatible unless explicitly breaking

### IV. Smoke Tests & Quality Gates (NON-NEGOTIABLE)
Every pull request MUST pass automated smoke tests before merge:
- **Language-Specific Tests**: Basic Go (`go test`) and Rust (`cargo test`) test suites validating agent/server functionality
- **Integration Tests**: Verify gRPC communication contracts between agent and management layer
- **Pre-Commit Hooks**: All commits MUST pass linting, formatting, and basic unit tests
- Test coverage targets MUST be documented per component and enforced via CI/CD
- Smoke test failures are blocking; no exceptions granted without explicit ADR amendment

Rationale: Early detection of regressions saves debugging time and maintains architecture integrity across rapid development cycles.

### V. Git-Driven ADR Workflow & DevOps Toolchain
Development workflow MUST be driven by Git, GitHub CLI, and ADRs with integrated tooling:

**ADR Filing (Decision → Implementation)**:
- Significant decisions MUST be formalized as ADRs BEFORE feature branch creation
- ADR created in `./ADR/` directory with Nygard template (simple, status-driven)
- ADR committed to `develop` branch as proposal; team reviews for acceptance
- Feature branch MUST reference ADR in branch name and PR description (e.g., `feature/ADR-015-grpc-streams`)
- Commit squash message MUST include: `ADR-XXX: descriptive change` format

**Git Workflow & GitHub CLI**:
- Feature branches, code review via pull requests, ADR-based decision tracking (squash-merge default)
- GitHub CLI (`gh`) is primary interface for PR management, issue tracking, workflow automation
- ADR acceptance MUST precede feature branch creation; PRs enforce this via branch naming patterns

**Pre-Commit Framework**: Enforce formatting, linting, and type checking before local commits
- Go tools: `gofmt`, `golangci-lint`, `go vet`
- Rust tools: `rustfmt`, `clippy`, `cargo check`
- General: YAML validation, trailing whitespace cleanup, secret scanning

**CI/CD Integration**: All smoke tests, linting, and integration tests MUST run on PR submission

**Commit Message Format**: Follow conventional commits + ADR reference (e.g., `feat(agent): add gRPC streaming - ADR-015`)

All contributors MUST install and configure pre-commit hooks locally; CI will enforce compliance.

## Agent Skills & Capabilities

All Speckit agents integrated into Pheromone development MUST support:
- **Constitution Compliance**: Verify commits align with architectural principles, language standards, and layering rules
- **ADR Tracking**: Cross-reference ADRs in PR descriptions and enforce decision consistency
- **Protocol Contract Management**: Validate gRPC schema changes against version-compatibility rules
- **Observability Validation**: Ensure new code paths include appropriate logging and telemetry exports
- **Smoke Test Coverage**: Report test coverage and blocking test failures with remediation guidance
- **Pre-Commit Enforcement**: Guide developers on format/lint issues before submission

Agent interactions MUST use Git and GitHub CLI for all state management (no external state stores in agent decisions).

## ADR-Driven Speckit Workflow

The Pheromone development process MUST follow a decision-first workflow integrated with Speckit templating:

### Phase 1: Decision (ADR → Plan)
1. **Identify Requirement**: Business need or architectural problem surfaces
2. **Create ADR**: Document decision context, alternatives, and proposed choice in `./ADR/adr-NNN-descriptive-name.md`
   - Use Michael Nygard template (see `.specify/templates/adr-template.md` for Pheromone-specific variant)
   - Include: Status=Proposed, Context, Decision, Consequences, follow-up ADRs
3. **ADR Review**: Team reviews; minimum 2 approvals before Status=Accepted
4. **Plan Creation**: Speckit `plan` agent uses accepted ADR as input; generates `.specify/memory/plan-NNN.md`
   - Plan MUST reference ADR number and decision context
   - Plan articulates acceptance criteria tied to ADR consequences

### Phase 2: Specification (Plan → Spec)
1. **Specify Feature**: Speckit `specify` agent generates `.specify/memory/spec-NNN.md` from accepted plan
   - Spec MUST break down architecture decision into implementable features
   - Spec MUST verify alignment with Layered Twin Architecture (Principle I)
   - Spec MUST include protocol contract definitions (gRPC/Kafka) if applicable
2. **Specification Review**: Architecture verification that spec implements ADR intent

### Phase 3: Implementation (Spec → Tasks → Code)
1. **Create Feature Branch**: Named `feature/ADR-NNN-descriptive-name` linked to ADR
2. **Generate Tasks**: Speckit `tasks` agent creates implementation checklist from spec
3. **Implement**: Developer follows tasks; commits MUST reference ADR-NNN in message
4. **Smoke Tests**: CI validates all tests pass (Go/Rust coverage ≥70% new code)
5. **Observability Check**: Ensure new code includes logging/metrics per Principle II

### Phase 4: Review & Merge (Code → Main)
1. **PR Submission**: Include ADR reference, spec link, test coverage report
2. **Agent Validation**: Constitutional compliance, ADR consistency, protocol contracts
3. **Human Review**: Confirm implementation matches spec and ADR intent
4. **Merge**: Squash-merge to `develop` with commit message `feat/fix: ADR-NNN description`
5. **Release**: When feature is ready, create `release/vX.Y.Z` branch; ADR becomes part of release notes

### Post-Decision Learning (ADR Lifecycle)
- **1-Month Review**: Team reviews ADR against actual implementation; append dated notes if consequences differ
- **Supersession**: If new ADR replaces prior decision, link via "Superseded by ADR-XXX" comment
- **Metrics Tracking**: Correlate ADR decisions with observability data; feed learnings into future decisions

**Rationale**: ADR-first workflow ensures architecture decisions are explicit, reviewable, and traceable through implementation and operations. Speckit templates operationalize ADR intent into executable tasks.

## Development Workflow

### Code Review & Merge Process
1. Feature branch created from `develop` linked to accepted ADR (branch name: `feature/ADR-NNN-description`)
2. Developer implements feature with full pre-commit hook execution locally
3. PR submitted with:
   - ADR reference (link to `./ADR/adr-NNN-*.md`)
   - Spec reference (link to `.specify/memory/spec-NNN.md` if generated)
   - Test coverage report showing ≥70% coverage on new code
   - Summary of how implementation aligns with ADR decision and consequences
4. Agents validate constitutional compliance, protocol contracts, and observability instrumentation
5. Human review confirms architectural alignment with digital twin layers and ADR intent
6. Upon approval, squash-merge to `develop` with conventional commit message: `feat/fix: ADR-NNN descriptive message`

### Breaking Changes & Versioning
- **Semantic Versioning**: MAJOR.MINOR.PATCH applied to all deliverables
  - MAJOR: Backward-incompatible protocol/API changes, twin layer restructuring, language support removal
  - MINOR: New features (agent types, twin capabilities), expanded layer functionality
  - PATCH: Bug fixes, performance improvements, documentation updates
- **Protocol Versioning**: gRPC services MUST include version number in package (`pheromone.v1`, etc.)
- **ADR for Significant Changes**: Any MAJOR version bump MUST include ratified ADR

### Branching Strategy (GitFlow Model - MANDATORY)
The project MUST follow GitFlow branching model strictly:

- **`main`**: Production branch, always stable and deployable
  - Only receives commits via release or hotfix merges
  - Tagged with semantic version on every release
  - All commits MUST pass full CI/CD test suite before merge

- **`develop`**: Integration and staging branch
  - Base branch for all feature development
  - Always in deployable state; candidate for next release
  - Receives feature, bugfix, and release branch merges only

- **`feature/*`**: Individual feature or bugfix branches
  - Named format: `feature/ADR-XXX-descriptive-name` or `bugfix/issue-XXX-descriptive-name`
  - Branched from: `develop`
  - Merge back to: `develop` via PR with smoke tests passing
  - Must include ADR or issue reference in branch name

- **`release/*`**: Release preparation branches
  - Named format: `release/vX.Y.Z`
  - Branched from: `develop`
  - Merge to: both `main` AND `develop` after release
  - Used for final testing, version bumping, release notes preparation
  - No new features allowed; bug fixes only

- **`hotfix/*`**: Emergency production fixes
  - Named format: `hotfix/vX.Y.Z-issue-XXX`
  - Branched from: `main`
  - Merge to: both `main` AND `develop` immediately after fix
  - MUST include version bump and inline ADR rationale
  - Bypass normal review process in emergencies (but document post-facto)

**Enforcement**:
- GitHub branch protection rules MUST enforce GitFlow naming conventions
- Commits directly to `main` or `develop` are FORBIDDEN (except automated merge commits from PR)
- All merges MUST be squash-merge with conventional commit messages

## Quality Assurance & Testing Standards

### Smoke Test Suites
**Go Components** (minimal required for agent/server):
```bash
go test ./... -v -race -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Rust Components** (minimal required for performance-critical paths):
```bash
cargo test --all -- --nocapture
cargo tarpaulin --out Html --output-dir coverage
```

**Integration Tests**:
- gRPC contract validation (bidirectional streaming, deadlines, error handling)
- Agent lifecycle (startup, metric reporting, graceful shutdown)
- Management layer twin synchronization across multiple agents

### Pre-Commit Tool Configuration
Pre-commit configuration MUST include (minimum):
```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: detect-private-key

  - repo: https://github.com/golang/lint
    hooks:
      - id: golangci-lint
      - id: go-fmt

  - repo: https://github.com/rust-lang/rust-clippy
    hooks:
      - id: clippy
      - id: rustfmt
```

### Coverage & Reporting
- Unit test coverage target: ≥70% per module (agents, twin models, protocols)
- Integration test coverage: ≥50% of critical paths (agent-server communication, twin updates)
- Coverage reports MUST be generated on every PR and tracked over time
- Declining coverage MUST be justified and addressed before merge

## Governance

### Constitution Authority
This constitution supersedes all informal practices, Slack discussions, and prior conventions. All architectural decisions MUST align with these five core principles and operational guidelines. Deviations require explicit constitutional amendment via ADR.

### Amendment Procedure
1. **Issue a Proposal**: GitHub issue proposes amendment with rationale and impact analysis
2. **Create ADR**: Formal ADR captures decision context, alternatives, and decision (file: `adr-NNN-amend-constitution.md`)
3. **Review**: Minimum 2 maintainer approvals required; 1-week community feedback period minimum
4. **Version Bump**: MAJOR for principle removals/redefinitions, MINOR for principle additions/expansions, PATCH for editorial changes
5. **ADR Acceptance**: Constitution amendment becomes effective only after ADR Status=Accepted
6. **Enforcement**: Amended constitution MUST be distributed to all contributors; CI/CD updated to enforce new rules
7. **Retroactive Application**: Historical PRs grandfathered only if pre-dating ADR acceptance; all new PRs immediately subject to amended rules

### Compliance Review
- Every PR submission triggers constitutional compliance check via agents
- Monthly review of violation patterns; repeated violations indicate training need
- Quarterly architectural review ensures active principles remain aligned with project goals

### Dependent Artifacts Requiring Sync
After constitutional amendments, these files MUST be reviewed for consistency:
- `./ADR/README.md`: Reference document on ADR filing procedures (already complete; maintainer for ADR standards)
- `.specify/templates/adr-template.md`: Pheromone-specific ADR template with decision/consequence guidance (CREATE if missing)
- `.specify/templates/plan-template.md`: Verify planning guidance includes ADR reference section
- `.specify/templates/spec-template.md`: Verify spec template references corresponding ADR and decision rationale
- `.specify/templates/tasks-template.md`: Verify task categorization includes observability, versioning, testing discipline aligned to ADR
- `.github/workflows/`: Verify CI smoke test and pre-commit enforcement configs match testing standards
- `README.md`: Verify contribution guidelines link to current constitution version and ADR filing procedures

---

**Version**: 2.0.0 | **Ratified**: 2026-02-18 | **Last Amended**: 2026-02-18

