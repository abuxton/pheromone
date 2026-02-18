# Example Feature: Agent Health Monitoring

This is a **complete example** demonstrating how to use Speckit with ADRs in the pheromone repository.

## Purpose

This example shows:
1. How to write a feature specification
2. How to create an implementation plan
3. How to reference ADRs in your specs
4. How specs and ADRs work together

## Files in This Example

### spec.md
The feature specification created using `/speckit.specify` (or manually). Contains:
- Problem statement
- User stories
- Requirements (functional and non-functional)
- Success metrics
- Dependencies and assumptions

**Key Points**:
- Technology-agnostic (doesn't specify Go, Prometheus, etc.)
- Focuses on WHAT we want to achieve
- References ADR-001 for architectural context

### plan.md
The implementation plan created using `/speckit.plan`. Contains:
- Architecture diagrams
- Technology stack (with rationale)
- Detailed design
- Performance considerations
- Security considerations
- Testing strategy

**Key Points**:
- References ADR-001 for protocol choice (gRPC)
- Makes specific technology decisions (Go, Prometheus)
- Explains WHY these choices align with existing decisions
- Notes that a new ADR might be needed for metrics storage

## How This Demonstrates Speckit + ADR Integration

### 1. Referencing Existing ADRs

In `plan.md`, we reference ADR-001:
```markdown
### Technology Stack
Per ADR-001 and project constitution:
- **Protocol**: gRPC streaming (existing infrastructure)
```

This shows how implementation plans build on documented architectural decisions.

### 2. Identifying New ADR Needs

In `plan.md`, we note:
```markdown
**Next Steps**:
3. Consider creating ADR-002 for metrics storage choice if deemed significant
```

This shows when to create new ADRs during planning.

### 3. Spec-Plan Separation

**spec.md** says WHAT:
- "Monitor agent health via heartbeats"
- "Detect unhealthy agents within 2 minutes"

**plan.md** says HOW:
- "Use gRPC bidirectional streaming"
- "Prometheus for metrics storage"
- "Check every 30 seconds"

### 4. Constitutional Alignment

Both files reference `constitution.md` to ensure alignment with project principles:
- Use Go (per constitution: "Primary Language: Go")
- Lightweight design (per constitution: "Lightweight Agents")
- Security-first (per constitution: "TLS for all connections")

## Creating Similar Features

### Step 1: Specify
```
/speckit.specify Build [your feature description]
```
Creates: `specs/002-your-feature/spec.md`

### Step 2: Plan
```
/speckit.plan [technology and architecture choices]
```
Creates: `specs/002-your-feature/plan.md`

**In your plan**:
- Reference existing ADRs: "Per ADR-001, we use gRPC..."
- Note if new ADR needed: "This choice warrants ADR-003..."

### Step 3: Create ADR (if needed)

If your plan introduces significant architectural decisions:
```bash
cp adrs/templates/adr-template.md adrs/adr-002-your-decision.md
# Edit the ADR
# Update adrs/README.md index
```

### Step 4: Continue Workflow
```
/speckit.tasks
/speckit.implement
```

## Relationship with ADR-001

This example feature builds on ADR-001:

**ADR-001 decided**:
- ✅ Use Go for agents and server
- ✅ Use gRPC for communication
- ✅ Custom lightweight implementation

**This feature applies those decisions**:
- Health monitor written in Go ✓
- Heartbeats over gRPC streams ✓
- Minimal dependencies (Prometheus standard for Go) ✓

**New decision to potentially document**:
- Choice of Prometheus for metrics storage
- If deemed architecturally significant → Create ADR-002

## Workflow Summary

```
1. Read ADR-001 (understand architecture)
   ↓
2. Create spec.md (define requirements)
   ↓
3. Create plan.md (apply ADR-001 decisions + make new decisions)
   ↓
4. Evaluate: Do new decisions need ADR?
   ↓
   Yes → Create ADR-002
   No  → Continue to tasks
   ↓
5. Create tasks.md (/speckit.tasks)
   ↓
6. Implement (/speckit.implement)
```

## Learning Points

1. **Specs are technology-agnostic** - Don't mention Go/Prometheus in spec.md
2. **Plans apply architecture** - Reference ADRs for decisions
3. **ADRs document "why"** - Explain rationale for future readers
4. **Constitution guides all** - Ensure alignment with project principles
5. **Link everything** - Cross-reference ADRs, specs, and constitution

## Try It Yourself

1. Review this example
2. Create a new feature: `specs/002-your-feature/`
3. Write spec.md (WHAT you want)
4. Write plan.md (HOW to build it, referencing ADR-001)
5. Decide if you need a new ADR
6. Continue with tasks and implementation

---

**Questions?**
- See [Speckit Quick Reference](../../docs/SPECKIT-QUICKREF.md)
- See [ADR README](../../adrs/README.md)
- See [Specs README](../README.md)
