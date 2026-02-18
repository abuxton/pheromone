# Specifications Directory

This directory contains feature specifications and implementation plans managed using the Speckit framework.

## What is Speckit?

Speckit is a spec-driven development framework that helps build high-quality software faster by focusing on product scenarios and predictable outcomes. It follows a structured workflow from specification to implementation.

## Directory Structure

```
specs/
├── README.md                              # This file
├── {feature-number}-{feature-name}/       # Feature-specific directory
│   ├── spec.md                           # Feature specification
│   ├── plan.md                           # Implementation plan
│   ├── tasks.md                          # Task breakdown
│   ├── checklist.md                      # Quality checklist
│   └── analysis.md                       # Cross-artifact analysis
└── constitution.md                        # Project principles and guidelines
```

## Speckit Workflow

The Speckit workflow consists of the following phases:

### 1. Constitution (Project-wide, one-time)
Create project principles and development guidelines that guide all development:
```
/speckit.constitution
```

### 2. Specify
Define what you want to build, focusing on the "what" and "why":
```
/speckit.specify [Feature description]
```

### 3. Clarify (Optional)
Refine specifications through Q&A to address underspecified areas:
```
/speckit.clarify
```

### 4. Plan
Create technical implementation plans with chosen tech stack:
```
/speckit.plan
```

### 5. Tasks
Break down the plan into actionable tasks:
```
/speckit.tasks
```

### 6. Checklist (Optional)
Generate quality validation checklists:
```
/speckit.checklist
```

### 7. Analyze (Optional)
Perform cross-artifact consistency analysis:
```
/speckit.analyze
```

### 8. Implement
Execute all tasks to build the feature:
```
/speckit.implement
```

## Getting Started with a New Feature

1. **Create the specification**
   ```
   /speckit.specify Build a lightweight agent in Go for collecting and reporting system metrics
   ```

2. **Review and refine** (optional)
   ```
   /speckit.clarify Focus on performance requirements and scalability
   ```

3. **Create implementation plan**
   ```
   /speckit.plan Use gRPC for communication, minimize external dependencies
   ```

4. **Break down into tasks**
   ```
   /speckit.tasks
   ```

5. **Execute implementation**
   ```
   /speckit.implement
   ```

## Integration with ADRs

When a feature requires significant architectural decisions:

1. Create the spec and plan using Speckit workflow
2. Document the architectural decision in `../adrs/` directory
3. Reference the ADR in your feature's `plan.md`
4. Link the feature spec in the ADR's "Related Decisions" section

## Example Feature Structure

For a feature numbered 001 for "lightweight-agent":

```
specs/001-lightweight-agent/
├── spec.md          # What: Collect system metrics, report to server
├── plan.md          # How: Use Go, gRPC protocol, plugin architecture
├── tasks.md         # Tasks: T001-Setup, T002-Core, T003-Plugins, etc.
└── checklist.md     # Validation: Performance tests, error handling, docs
```

## Best Practices

1. **Focus on outcomes** - Describe what you want to achieve, not how to build it (that comes in the plan)
2. **Keep specs technology-agnostic** - Technology choices belong in the plan phase
3. **Be specific** - Include concrete examples and success criteria
4. **Link decisions** - Cross-reference related ADRs and specifications
5. **Iterate** - Use `/speckit.clarify` to refine underspecified areas
6. **Validate** - Use `/speckit.checklist` and `/speckit.analyze` before implementation

## References

- [GitHub Speckit Repository](https://github.com/github/spec-kit)
- [Spec-Driven Development Guide](https://github.github.io/spec-kit/)
- [Pheromone ADRs](../adrs/README.md)
- [Pheromone Project Summary](../summary.md)
