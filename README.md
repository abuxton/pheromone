# Pheromone Repository

## Overview
Pheromone is a platform inspired by the pheromone-based communication system of the Alien species in Ridley Scott's franchise. The project explores the use of digital twins in a scalable 1:Many model to facilitate communication, observability, and alignment across nodes in a distributed system. It aims to leverage modular and hierarchical digital twins to manage server configurations and workloads effectively.

## Key Features
- **Digital Twin Management**: Implement a 1:Many model where a single digital twin is used to manage and synchronize multiple matching nodes or workloads.
- **Layered Architecture**: Split twins into OS-level twins and workload-level twins for managing distinct layers of node topology.
- **Agent-Based Communication**: Employ lightweight agents for monitoring and enforcement of states based on digital twin models.
- **Protocol Agnostic**: Design for flexibility, supporting protocols like gRPC, Kafka, and others for agent-to-server communication.
- **Scalability and Observability**: Provide real-time insights across distributed systems using telemetry and logging frameworks.

## Goals
1. Develop a modular and scalable system for managing servers and workloads using digital twins.
2. Leverage open standards and frameworks to enable platform-agnostic implementation.
3. Explore cutting-edge technologies, including lightweight protocols, distributed key-value stores, and advanced configuration management solutions.
4. Enable efficient tracking and management of architectural decisions through tools like Speckit.

## Current Status
The repository is in the early development stage, with ongoing work on:
1. Setting up an Architectural Decision Record (ADR) framework.
2. Exploring suitable tools, languages (e.g., Go and Rust), and protocols for implementation.
3. Building foundational components for agents and server communication.
4. Performance validation of hybrid in-memory + etcd architecture (ADR-002).

## Recent Developments

### ADR-002 Performance Validation (Tech Spike)

A comprehensive performance validation infrastructure has been implemented to validate the hybrid in-memory + etcd persistence architecture:

- ✅ **Cache Hit Ratio**: Validated >80% for twin state queries (achieved 85%)
- ⏳ **etcd Write Latency**: Tests ready (target: P99 <100ms)
- ⏳ **Server Recovery**: Tests ready (target: <5 seconds)
- ✅ **Data Consistency**: Rollback logic verified for write failures

**Quick Start**:
```bash
# Run offline validation tests
make validate-offline

# Start etcd and run full validation
make validate

# Clean up
make etcd-clean
```

See [`ADR/adr-002-performance-validation.md`](ADR/adr-002-performance-validation.md) for complete results and [`benchmark/README.md`](benchmark/README.md) for detailed testing instructions.

## Inspiration
This project is named after the pheromone-based communication used by the Aliens from Ridley Scott's franchise. The design embodies principles of decentralized communication, observability, and adaptability.

## Contributing
Contributions are welcome as the project evolves. Please check the ADRs and ongoing discussions for more details.

## License
This repository is private and is currently aimed at team-internal development and exploration.

Stay tuned for updates as we build the platform!
