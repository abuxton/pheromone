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
Pheromone is a distributed digital-twin platform inspired by the pheromone-based communication system of the Alien species in Ridley Scott's franchise. The project implements a 1:Many digital twin model across three architectural layers — OS-Level Twins, Workload-Level Twins, and Management Layer — to facilitate communication, observability, and autonomous configuration management across nodes in a distributed system (see [ADR-001](docs/adr/adr-001-digital-twin-architecture.md)).

**Agents in Pheromone are agentic AI-capable agents** — autonomous processes running on each managed instance that use an AI reasoning loop to observe, plan, and act on their environment. Digital twin management is a core *skill* of each agent: the agent reads, updates, and applies its twin model to the system under management (see [ADR-007](docs/adr/adr-007-agentic-ai-agent-model.md)).

## Key Features
- **Agentic AI Agents**: Each managed instance runs an autonomous AI-capable agent with an AI reasoning loop. Digital twin management is a first-class skill—agents read, update, and apply their twin model without waiting for explicit command-response cycles (see [ADR-007](docs/adr/adr-007-agentic-ai-agent-model.md), [ADR-008](docs/adr/adr-008-ai-model-selection.md)).
- **Digital Twin Management**: Implement a 1:Many model where a single digital twin is used to manage and synchronize multiple matching nodes or workloads (see [ADR-004](docs/adr/adr-004-twin-model-schema.md)).
- **Layered Architecture**: Split twins into OS-level twins and workload-level twins for managing distinct layers of node topology (see [ADR-001](docs/adr/adr-001-digital-twin-architecture.md)).
- **Intent-Driven Communication**: Agents interpret the twin desired state as *intent* and reason autonomously about how to achieve it, consistent with the pheromone metaphor.
- **gRPC Communication**: All agent-to-server and server-to-agent communication uses gRPC with Protocol Buffers for efficient, versioned contracts (see [ADR-003](docs/adr/adr-003-grpc-contracts.md), [ADR-011](docs/adr/adr-011-port-assignment.md)).
- **Message Bus**: NATS JetStream provides the asynchronous event backbone between platform components (see [ADR-005](docs/adr/adr-005-message-queue.md)).
- **Scalability and Observability**: Real-time insights across distributed systems using Prometheus metrics, Grafana dashboards, AI decision traces, and structured logging (see [ADR-018](docs/adr/adr-018-observability-stack.md), [ADR-019](docs/adr/adr-019-agent-ai-observability.md)).

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

A comprehensive performance validation infrastructure has been implemented to validate the hybrid in-memory + etcd persistence architecture
(see [ADR-002](docs/adr/adr-002-server-architecture.md) and [performance validation](docs/adr/adr-002-performance-validation.md)):

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

See [`docs/adr/adr-002-performance-validation.md`](docs/adr/adr-002-performance-validation.md) for complete results and [`benchmark/README.md`](benchmark/README.md) for detailed testing instructions.

## Local Development with Docker

The recommended local development environment uses Docker Compose to run all platform services —
etcd, NATS, the Pheromone server, agents, Prometheus, and Grafana — with a single command.

**Prerequisites**: [Docker Desktop ≥ 4.x](https://www.docker.com/products/docker-desktop/) or [Docker Engine ≥ 24](https://docs.docker.com/engine/install/) with the Compose plugin.

### Service topology

| Container | Port(s) | Role |
|---|---|---|
| `pheromone-etcd` | 2379, 2380 | etcd key-value store |
| `pheromone-nats` | 4222 | NATS message bus |
| `pheromone-server` | 8081 | Pheromone management server (gRPC + HTTP) |
| `pheromone-agent-1` | — | Pheromone agent (mirrors ubuntu topology) |
| `pheromone-agent-2` | — | Pheromone agent (mirrors debian topology) |
| `pheromone-prometheus` | 9090 | Prometheus metrics scraper |
| `pheromone-grafana` | 3000 | Grafana observability dashboard |

Port assignments follow [ADR-011](docs/adr/adr-011-port-assignment.md).

### Quick Start

```bash
# 1. Copy the environment template and set required credentials
cp .env.example .env
# Edit .env — at minimum set POSTGRES_PASSWORD if using the postgres profile

# 2. Build container images
make compose-build

# 3. Start core services (etcd, NATS, server, agents, Prometheus, Grafana)
make compose-up

# 4. Check service status
make compose-ps

# 5. Open the management server
open http://localhost:8081

# 6. Open Grafana dashboards  (default login admin/admin)
open http://localhost:3000
```

### Common Docker Compose commands

```bash
# Follow all service logs
make compose-logs

# Follow server logs only
make compose-logs-server

# Follow agent logs
make compose-logs-agents

# Start with PostgreSQL dataplane (ADR-016)
make compose-up-postgres

# Start everything including the interactive pheromone-ctl testing container
make compose-up-full

# Run pheromone-ctl ad-hoc (testing profile must be active)
docker compose run --rm pheromone-ctl health

# Stop all services
make compose-down

# Stop all services and remove persistent volumes
make compose-clean

# Validate docker-compose.yml
make compose-validate
```

### Observability

Prometheus scrapes metrics from `pheromone-server:8081/metrics` every 15 seconds.
Grafana is pre-provisioned with dashboards; open [http://localhost:3000](http://localhost:3000) after `make compose-up`.
The observability stack is described in [ADR-018](docs/adr/adr-018-observability-stack.md) and agent AI traces in [ADR-019](docs/adr/adr-019-agent-ai-observability.md).

## Alternative: Vagrant Local Environment

A multi-machine Vagrant environment is also available for end-to-end validation on real Linux VMs
(Ubuntu 24.04 LTS and Debian 12 Bookworm), as described in [ADR-012](docs/adr/adr-012-vagrant-testing-environment.md).

```bash
make vagrant-up          # Start all VMs
make vagrant-up-server   # Start server VM only
make vagrant-halt        # Stop all VMs
make vagrant-destroy     # Destroy all VMs
```

See [`vagrant/README.md`](vagrant/README.md) for full usage, VM helper commands, and troubleshooting.

## Inspiration
This project is named after the pheromone-based communication used by the Aliens from Ridley Scott's franchise. The design embodies principles of decentralized communication, observability, and adaptability.

## Contributing
Contributions are welcome as the project evolves. Please check the ADRs and ongoing discussions for more details.

## License
This repository is private and is currently aimed at team-internal development and exploration.

Stay tuned for updates as we build the platform!
