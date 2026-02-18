# Speckit Quick Reference Guide

This guide provides quick reference for using Speckit in the pheromone repository.

## Installation

```bash
# Install Speckit CLI (one-time)
uv tool install specify-cli --from git+https://github.com/github/spec-kit.git

# Check installation
specify check
```

## Initialization (Already Done)

The repository is already configured with Speckit. The following files have been set up:

- `speckit.yml` - Configuration file
- `specs/` - Specifications directory
- `adrs/` - Architectural Decision Records
- `specs/constitution.md` - Project principles

## Speckit Commands Reference

### Core Workflow Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `/speckit.constitution` | Define project principles | Once per project, or when updating principles |
| `/speckit.specify` | Create feature specification | Start of every new feature |
| `/speckit.clarify` | Refine specifications | When spec needs more detail or has ambiguities |
| `/speckit.plan` | Create implementation plan | After spec is clear, before implementation |
| `/speckit.tasks` | Break down into tasks | After plan is approved |
| `/speckit.checklist` | Generate quality checklist | Before implementation starts |
| `/speckit.analyze` | Cross-artifact analysis | After tasks, before implementation |
| `/speckit.implement` | Execute implementation | Final step - build the feature |

### Typical Feature Development Flow

```
1. /speckit.specify "Build agent health monitoring system"
   → Creates: specs/002-agent-health-monitoring/spec.md

2. /speckit.clarify "Focus on alerting and thresholds"
   → Refines: specs/002-agent-health-monitoring/spec.md

3. /speckit.plan "Use Go with gRPC, integrate with existing agent framework"
   → Creates: specs/002-agent-health-monitoring/plan.md

4. /speckit.tasks
   → Creates: specs/002-agent-health-monitoring/tasks.md

5. /speckit.implement
   → Executes tasks and builds the feature
```

## ADR Workflow

### When to Create an ADR

Create an ADR when you need to document:
- Major architectural decisions
- Technology stack changes
- Protocol or framework selections
- Design pattern choices
- Significant trade-offs

### Creating an ADR

1. **Copy the template**
   ```bash
   cp adrs/templates/adr-template.md adrs/adr-002-your-decision.md
   ```

2. **Fill in the sections**
   - Status (usually "Proposed")
   - Context (what problem are you solving?)
   - Decision (what did you decide and why?)
   - Consequences (what are the trade-offs?)

3. **Update the index**
   - Add entry to `adrs/README.md`

4. **Link to specs**
   - Reference the ADR in related `specs/*/plan.md` files
   - Reference related specs in the ADR

### Example ADR Creation Flow

```
1. Start working on new feature
   /speckit.specify "Add support for multiple message protocols"

2. Create implementation plan
   /speckit.plan "Evaluate MQTT, AMQP, and gRPC"

3. During planning, realize this is a significant decision
   → Create ADR-002-message-protocol-selection.md

4. Document alternatives in ADR:
   - Context: Need flexible message protocol support
   - Considered: MQTT, AMQP, gRPC
   - Decision: Use gRPC as primary with plugin system for others
   - Consequences: List pros/cons

5. Reference ADR in plan.md:
   "Protocol selection documented in ADR-002"

6. Continue with Speckit workflow
   /speckit.tasks
   /speckit.implement
```

## Integration: Specs + ADRs

### Best Practices

1. **Start with Spec**
   - Use `/speckit.specify` to describe what you want
   - Focus on requirements, not implementation

2. **Plan and Evaluate**
   - Use `/speckit.plan` to think through implementation
   - If architecturally significant → Create ADR

3. **Document Decision**
   - Create ADR while information is fresh
   - Include alternatives considered
   - Document trade-offs

4. **Link Everything**
   - Reference ADR in spec's plan.md
   - Reference spec in ADR
   - Update indexes

5. **Implement**
   - Use `/speckit.tasks` and `/speckit.implement`
   - Follow the documented decision

### Directory Structure Example

```
pheromone/
├── speckit.yml
├── adrs/
│   ├── README.md (index)
│   ├── adr-001-digital-twin-architecture.md
│   └── adr-002-message-protocol-selection.md
└── specs/
    ├── constitution.md
    ├── 001-lightweight-agent/
    │   ├── spec.md        # References: ADR-001
    │   ├── plan.md        # References: ADR-001
    │   └── tasks.md
    └── 002-message-protocols/
        ├── spec.md        # What: Support multiple protocols
        ├── plan.md        # How: gRPC + plugins (References: ADR-002)
        └── tasks.md
```

## Common Workflows

### Workflow 1: Simple Feature (No ADR)

```bash
/speckit.specify "Add configuration validation"
/speckit.plan "Use Go validator library"
/speckit.tasks
/speckit.implement
```

### Workflow 2: Feature with Architectural Decision

```bash
/speckit.specify "Add distributed state management"
/speckit.plan "Evaluate Etcd vs Consul vs Redis"

# Realize this is significant → Create ADR
# Create adrs/adr-003-distributed-state-store.md

/speckit.tasks
/speckit.implement
```

### Workflow 3: Updating Project Principles

```bash
/speckit.constitution "Add principle about security-first design"
# Updates specs/constitution.md
```

## Tips and Tricks

### 1. Keep Specs Technology-Agnostic

❌ **Bad**: "Build a React component that displays agent status"
✅ **Good**: "Display agent status with real-time updates"

Technology choice comes in `/speckit.plan`, not `/speckit.specify`.

### 2. Use Clarify Early

If your spec feels incomplete:
```
/speckit.clarify "Focus on error handling and edge cases"
```

### 3. Reference Liberally

In plan.md:
```markdown
## Technology Choices
- Protocol: gRPC (see ADR-001)
- State Store: Etcd (see ADR-003)
```

In ADR:
```markdown
## Related Decisions
- Relates to: specs/001-lightweight-agent/
```

### 4. Keep ADRs Focused

One decision per ADR. If you're documenting multiple decisions, create multiple ADRs.

### 5. Update Status

When an ADR is superseded:
```markdown
## Status
Superseded by ADR-005

## Context
...
```

## Troubleshooting

### "I don't know if this needs an ADR"

Ask yourself:
- Will future developers need to understand why this decision was made?
- Are there significant trade-offs?
- Will this be hard to change later?
- Does it affect multiple components?

If yes to any → Create an ADR

### "My spec is too vague"

Use `/speckit.clarify` to refine it, or create a more detailed spec with examples:
```
/speckit.specify "Build agent health monitoring with:
- Heartbeat every 30 seconds
- Alert on 3 missed heartbeats
- Dashboard showing agent status
- Historical uptime tracking"
```

## Resources

- [Speckit Documentation](https://github.github.io/spec-kit/)
- [ADR Guidelines](https://adr.github.io/)
- [Pheromone ADR Index](adrs/README.md)
- [Pheromone Specs Guide](specs/README.md)

---

**Need Help?**
- Check the examples in `specs/` and `adrs/` directories
- Review existing ADRs for patterns
- Consult the project constitution: `specs/constitution.md`
