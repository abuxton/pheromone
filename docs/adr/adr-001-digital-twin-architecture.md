# ADR 001: Digital Twin Architecture Design

## Status
Proposed

## Context
The pheromone platform is designed to implement a 1:Many model for digital twins, where a single model is used to manage and synchronize multiple agents. The architecture involves separating twins into logical layers: OS-level twins for managing operating system configurations and workload-level twins for managing applications or services. This decision will focus on key considerations, including technology choices for protocols, agents, and development languages.

## Decision
The architectural decisions for the system are outlined as follows:

### 1. Coding Languages
- **Preferred Languages**: 
  - **Go**: Lightweight, high-performance, and well-suited for systems programming and networked applications.
  - **Rust**: Memory-safe, high-performance, and provides excellent low-level control for agent development.
- **Decision**: Go is preferred for prototyping due to its faster learning curve and library ecosystem.

### 2. Agent Model — Agentic AI-Capable Agents
- **Clarification**: Agents in Pheromone are **agentic AI-capable agents**, not simple metric daemons. Each agent
  runs an autonomous AI reasoning loop on the managed instance. Digital twin management is a core **skill** of
  the agent—the agent reads, updates, and applies its twin model to the system under management. See ADR-007 for
  the full agentic AI agent model.
- **Candidates**:
  - **Telegraf**: Plugin-driven agent for collecting and reporting metrics.
  - **Fluent Bit**: Lightweight log processor and forwarder.
  - **Custom Agentic Implementation**: Build AI-capable agents using Go or Rust with a skill-based architecture.
- **Decision**: Build a custom agentic AI agent in Go. The agent hosts an AI reasoning loop and exposes digital
  twin management, metrics collection, and config enforcement as discrete skills. Rule-based reasoning is used
  for MVP (Phase 1); an LLM-backed or local model reasoning engine is plugged in behind the same interface in
  Phase 2 (ADR-008).

### 3. Protocols
- **Candidates**:
  - **Google A2A (Application-to-Application)**: Strong service discovery capabilities.
  - **MQTT**: Lightweight publish-subscribe protocol suitable for asynchronous communication.
  - **gRPC**: High-performance, open-source universal RPC framework.
  - **Kafka/RabbitMQ**: Message queue systems for asynchronous communication in distributed environments.
- **Decision**: Use gRPC for efficient communication between the agents and the server due to its performance and ecosystem support. Consider Kafka for telemetry aggregation and queuing.

### 4. Digital Twin Frameworks
- **Candidates**:
  - **Eclipse Ditto**: Open framework for implementing and managing digital twins.
  - **DIY Approach**: Custom design based on distributed databases for versioning and updates.
- **Decision**: Start with a custom lightweight twin framework using a distributed key-value store like Etcd or Consul. Eclipse Ditto can be integrated in the future for scalability.

### 5. Server
- **Languages**: Go or Rust to match agent development.
- **Frameworks**: Leverage existing frameworks such as gRPC servers, or lightweight REST APIs for configuration push/pull.
- **Agentic AI Support**: The server MUST maintain an agent capability registry, an action proposal/approval queue,
  and an AI telemetry ingestion path to support agentic AI agents (see ADR-007).

## Consequences
- **Pros**:
  - Modular design for OS/workload twins improves scalability.
  - Use of Go enables fast prototyping and efficient performance.
  - gRPC provides high-performance bi-directional communication.
  - Agentic AI agents enable autonomous, intent-driven management without tight command-response coupling.
- **Cons**:
  - Custom implementation for agentic AI agents increases initial development time.
  - Adoption of multiple tools/frameworks might increase the complexity of maintenance.
  - AI reasoning loop adds resource overhead on managed instances; graceful degradation required for resource-constrained nodes.

## Update — 2026-02-20
Clarified that agents are **agentic AI-capable agents** (see ADR-007). Digital twin management is a *skill*
of the agent's reasoning loop. Server design must support capability advertisement and action proposal flows.
