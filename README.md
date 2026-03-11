# Pheromone Repository

<!-- Row 1: CI/Quality -->
[![CI](https://github.com/abuxton/pheromone/actions/workflows/ci.yml/badge.svg)](https://github.com/abuxton/pheromone/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/abuxton/pheromone/graph/badge.svg)](https://codecov.io/gh/abuxton/pheromone)
[![License](https://img.shields.io/github/license/abuxton/pheromone)](LICENSE)

<!-- Row 2: Security -->
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/abuxton/pheromone/badge)](https://securityscorecards.dev/viewer/?uri=github.com/abuxton/pheromone)
[![Security Scan](https://github.com/abuxton/pheromone/actions/workflows/security-scan.yml/badge.svg)](https://github.com/abuxton/pheromone/actions/workflows/security-scan.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/REPLACE_WITH_PROJECT_ID/badge)](https://www.bestpractices.dev/projects/REPLACE_WITH_PROJECT_ID)

## Overview
Pheromone is a platform inspired by the pheromone-based communication system of the Alien species in Ridley Scott's franchise. The project explores the use of digital twins in a scalable 1:Many model to facilitate communication, observability, and alignment across nodes in a distributed system. It aims to leverage modular and hierarchical digital twins to manage server configurations and workloads effectively.

**Agents in Pheromone are agentic AI-capable agents** — autonomous processes running on each managed instance that use an AI reasoning loop to observe, plan, and act on their environment. Digital twin management is a core *skill* of each agent: the agent reads, updates, and applies its twin model to the system under management (see [ADR-007](docs/adr/adr-007-agentic-ai-agent-model.md)).

## Key Features
- **Agentic AI Agents**: Each managed instance runs an autonomous AI-capable agent with an AI reasoning loop. Digital twin management is a first-class skill—agents read, update, and apply their twin model without waiting for explicit command-response cycles.
- **Digital Twin Management**: Implement a 1:Many model where a single digital twin is used to manage and synchronize multiple matching nodes or workloads.
- **Layered Architecture**: Split twins into OS-level twins and workload-level twins for managing distinct layers of node topology.
- **Intent-Driven Communication**: Agents interpret the twin desired state as *intent* and reason autonomously about how to achieve it, consistent with the pheromone metaphor.
- **Protocol Agnostic**: Design for flexibility, supporting protocols like gRPC, Kafka, and others for agent-to-server communication.
- **Scalability and Observability**: Provide real-time insights across distributed systems using telemetry, AI decision traces, and logging frameworks.

## Goals
1. Develop a modular and scalable system for managing servers and workloads using digital twins driven by agentic AI agents on each instance.
2. Leverage open standards and frameworks to enable platform-agnostic implementation.
3. Explore cutting-edge technologies, including AI reasoning frameworks, lightweight protocols, distributed key-value stores, and advanced configuration management solutions.
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

## Local Testing Environment (Vagrant)

A multi-machine Vagrant environment is available for end-to-end validation on real Linux VMs
(Ubuntu 24.04 LTS and Debian 12 Bookworm), as described in [ADR-012](docs/adr/adr-012-vagrant-testing-environment.md).

| VM | IP | Role |
|---|---|---|
| `server` | `192.168.56.10` | etcd + future pheromone server |
| `agent-ubuntu` | `192.168.56.11` | Pheromone agent (Ubuntu 24.04 LTS) |
| `agent-debian` | `192.168.56.12` | Pheromone agent (Debian 12 Bookworm) |

**Prerequisites**: [Vagrant ≥ 2.3.0](https://developer.hashicorp.com/vagrant/install) and [VirtualBox ≥ 6.1](https://www.virtualbox.org/wiki/Downloads).

**Quick Start**:
```bash
# Start all VMs
make vagrant-up

# Start server only (etcd)
make vagrant-up-server

# Validate Vagrantfile syntax
make vagrant-validate

# Stop all VMs
make vagrant-halt

# Destroy all VMs
make vagrant-destroy
```

See [`vagrant/README.md`](vagrant/README.md) for full usage, VM helper commands, troubleshooting, and integration test instructions.

## Inspiration
This project is named after the pheromone-based communication used by the Aliens from Ridley Scott's franchise. The design embodies principles of decentralized communication, observability, and adaptability.

## Contributing
Contributions are welcome as the project evolves. Please check the ADRs and ongoing discussions for more details.

## License
This repository is private and is currently aimed at team-internal development and exploration.

Stay tuned for updates as we build the platform!
