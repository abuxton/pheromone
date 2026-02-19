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

### 2. Open-Source Agent Frameworks
- **Candidates**: 
  - **Telegraf**: Plugin-driven agent for collecting and reporting metrics.
  - **Fluent Bit**: Lightweight log processor and forwarder.
  - **Custom Implementation**: Build lightweight agents using Go or Rust for tighter control and modularity.
- **Decision**: Start with a custom agent in Go for flexibility and optimization.

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

## Consequences
- **Pros**:
  - Modular design for OS/workload twins improves scalability.
  - Use of Go enables fast prototyping and efficient performance.
  - gRPC provides high-performance bi-directional communication.
- **Cons**:
  - Custom implementation for agents increases initial development time.
  - Adoption of multiple tools/frameworks might increase the complexity of maintenance.
