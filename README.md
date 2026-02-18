# Pheromone

A Digital Twin Platform implementing a 1:Many model for OS-level and workload-level twins.

## Overview

The pheromone platform is designed to implement a 1:Many model for digital twins, where a single model is used to manage and synchronize multiple agents. The architecture separates twins into logical layers:

- **OS-level twins**: Managing operating system configurations
- **Workload-level twins**: Managing applications or services

## Project Structure

```
pheromone/
├── adrs/                   # Architectural Decision Records
│   ├── README.md          # ADR documentation and index
│   ├── templates/         # ADR templates
│   └── adr-*.md          # Individual ADRs
├── specs/                 # Feature specifications (Speckit)
│   ├── README.md          # Speckit usage guide
│   └── {feature}/         # Feature-specific specs
├── docs/                  # Additional documentation
├── speckit.yml           # Speckit configuration
├── summary.md            # Project summary
└── LICENSE               # MIT License
```

## Spec-Driven Development with Speckit

This repository uses the [Speckit framework](https://github.com/github/spec-kit) for managing specifications and architectural decisions. Speckit enables structured, spec-driven development that focuses on product scenarios and predictable outcomes.

### Quick Start with Speckit

1. **Set up Speckit** (if not already configured)
   ```bash
   # Install Speckit CLI
   uv tool install specify-cli --from git+https://github.com/github/spec-kit.git
   
   # Initialize in the repository (if needed)
   specify init . --ai claude --here
   ```

2. **Create project principles** (one-time)
   ```
   /speckit.constitution
   ```

3. **Work on a new feature**
   ```
   /speckit.specify [Describe your feature]
   /speckit.plan [Define tech stack and architecture]
   /speckit.tasks
   /speckit.implement
   ```

### Speckit Configuration

The repository includes a `speckit.yml` configuration file that defines:

- Directory structure for specs and ADRs
- Workflow phases and settings
- ADR templates and validation rules
- Project principles and guidelines

See [`speckit.yml`](speckit.yml) for full configuration details.

## Architectural Decision Records (ADRs)

All significant architectural decisions are documented as ADRs in the `adrs/` directory. This provides:

- Historical context for design decisions
- Rationale behind technology choices
- Trade-offs and consequences of decisions
- Reference for future development

**Current ADRs:**
- [ADR-001: Digital Twin Architecture Design](adrs/adr-001-digital-twin-architecture.md)

See [adrs/README.md](adrs/README.md) for more information on creating and managing ADRs.

## Key Architectural Decisions

### Technology Stack

- **Languages**: Go (preferred for prototyping) and Rust (for performance-critical components)
- **Protocols**: gRPC for agent-server communication, Kafka for telemetry aggregation
- **Agent Framework**: Custom lightweight agents in Go
- **Digital Twin Framework**: Custom implementation using distributed key-value store (Etcd/Consul)

### Design Principles

1. **Modularity and Flexibility**: Clear separation between OS-level and workload-level twins
2. **Lightweight and High-Performance**: Go for fast prototyping, Rust for critical paths
3. **Technology-Agnostic**: Support multiple agent types and protocols where possible
4. **Documentation-First**: Use Speckit for specs and ADRs for architectural decisions

## Development Workflow

### For New Features

1. Create a specification using `/speckit.specify`
2. Clarify requirements if needed using `/speckit.clarify`
3. Create implementation plan using `/speckit.plan`
4. If the feature involves architectural decisions, create an ADR in `adrs/`
5. Break down into tasks using `/speckit.tasks`
6. Implement using `/speckit.implement`

### For Architectural Decisions

1. Use Speckit to explore alternatives (`/speckit.specify` and `/speckit.plan`)
2. Document the decision as an ADR using the template in `adrs/templates/`
3. Update the ADR index in `adrs/README.md`
4. Link the ADR to relevant specs and features

## Documentation

- [Project Summary](summary.md) - Overview of the 1:Many model for digital twins
- [ADR Index](adrs/README.md) - All architectural decision records
- [Specs Guide](specs/README.md) - How to use Speckit for feature development
- [Speckit Configuration](speckit.yml) - Complete Speckit setup

## Contributing

When contributing to this repository:

1. Follow the Spec-Driven Development workflow using Speckit
2. Document architectural decisions as ADRs
3. Update relevant documentation and indexes
4. Ensure specs are clear and implementation-agnostic
5. Link related ADRs and specs for traceability

## Resources

- [GitHub Speckit](https://github.com/github/spec-kit) - Spec-driven development framework
- [Spec-Driven Development Guide](https://github.github.io/spec-kit/) - Complete Speckit documentation
- [ADR Documentation](https://adr.github.io/) - More about Architectural Decision Records

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Maintainer

adam buxton (@abuxton)
