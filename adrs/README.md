# Architectural Decision Records (ADRs)

This directory contains all Architectural Decision Records for the pheromone project.

## What are ADRs?

Architectural Decision Records (ADRs) are documents that capture important architectural decisions made along with their context and consequences. They help maintain a historical record of why certain decisions were made and serve as a reference for future development.

## ADR Index

| Number | Title | Status | Date |
|--------|-------|--------|------|
| [001](adr-001-digital-twin-architecture.md) | Digital Twin Architecture Design | Proposed | 2026-02-18 |

## How to Create a New ADR

1. Copy the template from `templates/adr-template.md`
2. Name the file following the convention: `adr-{number}-{title-slug}.md`
3. Fill in all required sections:
   - Status
   - Context
   - Decision
   - Consequences
4. Update this index file with the new ADR
5. Commit and push your changes

## Using Speckit for ADRs

This repository uses the Speckit framework for managing ADRs. You can use the following Speckit commands:

### Speckit Workflow Commands

- `/speckit.constitution` - Define project principles and guidelines
- `/speckit.specify` - Create feature specifications
- `/speckit.plan` - Create implementation plans
- `/speckit.tasks` - Break down into actionable tasks
- `/speckit.implement` - Execute implementation

### Creating ADRs with Speckit

When a significant architectural decision needs to be made:

1. Use `/speckit.specify` to describe the problem and requirements
2. Use `/speckit.plan` to evaluate alternatives and create an implementation plan
3. Document the final decision as an ADR in this directory
4. Link the ADR to relevant specs in the `specs/` directory

## ADR Statuses

- **Proposed**: The ADR is proposed and under discussion
- **Accepted**: The ADR has been accepted and should be implemented
- **Deprecated**: The ADR is no longer recommended but not yet superseded
- **Superseded**: The ADR has been replaced by a newer decision
- **Rejected**: The ADR was proposed but ultimately rejected

## Directory Structure

```
adrs/
├── README.md                              # This file
├── templates/
│   └── adr-template.md                   # Template for new ADRs
└── adr-{number}-{title-slug}.md          # Individual ADR files
```
