# ADR 010: Evaluate Signal Protocol (signalapp) for Server↔Agent Communication

## Status

Accepted

## Context

The Pheromone platform uses gRPC with Protocol Buffers as the primary server↔agent communication protocol (ADR-001, ADR-003). This ADR captures a formal review of the Signal app open-source organisation (<https://github.com/signalapp>) to determine whether the Signal Protocol or any of its constituent components should be adopted as the communication protocol — or as a security layer — for server↔agent channels.

### What Was Reviewed

The following public repositories under <https://github.com/signalapp> were examined:

| Repository | Language | Purpose |
|---|---|---|
| `Signal-Server` | Java / Spring Boot | Signal Messenger server; FoundationDB + Redis backend |
| `libsignal` | Rust core; Java/Swift/TypeScript bindings | Signal Protocol implementation (Double Ratchet, X3DH, zkgroup, AEAD primitives) |
| `SparsePostQuantumRatchet` | F* | Post-quantum ratchet extension to Double Ratchet |
| `ringrtc` | Rust | WebRTC-based voice/video calling library |
| `Signal-Android`, `Signal-iOS`, `Signal-Desktop` | Kotlin/Swift/TypeScript | End-user client applications |
| `boring` | Rust | BoringSSL Rust bindings (used internally) |
| `key-transparency-auditor` | Java | Key transparency auditing service |

### Signal Protocol Overview

The Signal Protocol is an end-to-end encryption (E2E) framework built for **asynchronous human-to-human secure messaging**. Its core components are:

- **X3DH (Extended Triple Diffie-Hellman)**: Key agreement for session establishment between two parties who may be offline simultaneously; requires a pre-key bundle server.
- **Double Ratchet Algorithm**: Provides forward secrecy and break-in recovery by advancing both a symmetric-key ratchet and a Diffie-Hellman ratchet on every message.
- **Sealed Sender**: Hides the sender identity from the server, providing sender anonymity at the application layer.
- **zkgroup / zkcredential**: Zero-knowledge proofs for private group membership.
- **libsignal-protocol (Rust crate)**: The canonical implementation; Java, Swift, and TypeScript bindings are generated from this Rust core.

### Licensing

All primary Signal repositories are licensed under **GNU AGPL v3**. This licence requires that any software incorporating Signal components and making them available over a network must release its own source code under the same terms. This creates a significant constraint for Pheromone.

### Language and Ecosystem Fit

- `libsignal` exposes official bindings for **Java, Swift, and TypeScript/Node.js only**.
- There are **no official Go bindings**. Integration with Pheromone's Go agent codebase (ADR-001) would require FFI via CGO, introducing build complexity and a C interop surface.
- `Signal-Server` is Java/Spring Boot. Pheromone's server is Go (ADR-001, ADR-002). Adopting Signal-Server would require either rewriting in Java or maintaining a Java service alongside the Go server.

### Signal-Server Operational Dependencies

Running Signal-Server in its current form requires:
- FoundationDB (distributed key-value store)
- Redis (caching, session storage)
- AWS DynamoDB (persistent message storage in the reference deployment)
- Amazon S3 (media storage)
- Google Cloud Messaging / APNs (push notification infrastructure)

These dependencies are appropriate for a global consumer messaging service but are prohibitively complex for a lightweight infrastructure management platform targeting single-server MVP deployments (ADR-002).

---

## Decision

**Do NOT adopt Signal Protocol, Signal-Server, or any signalapp component as the server↔agent communication protocol or security layer for Pheromone.**

**Rationale (point by point)**:

### 1. Architectural Mismatch

Signal Protocol is designed for **asynchronous, store-and-forward human messaging** between parties who may be offline simultaneously. Pheromone server↔agent channels require:

- Synchronous RPC for registration, heartbeat, and configuration push (AgentRegistry, TwinControl — ADR-003)
- High-frequency streaming telemetry at 100K msgs/sec target (SC-008, ADR-005)
- Long-lived bidirectional gRPC streams (not episodic message exchange)

The Double Ratchet Algorithm performs a key-ratcheting operation on **every message**. At 100K msgs/sec, this adds an unacceptable cryptographic overhead per message. The protocol was optimised for low-frequency, high-sensitivity human messages — not for high-throughput binary telemetry.

### 2. Transport Layer Incompatibility

Signal uses **HTTP/WebSocket** as its transport. Pheromone has committed to **gRPC with Protocol Buffers** (ADR-001 Principle III, ADR-003). Bridging these transports would require a translation proxy that adds latency, a failure domain, and operational overhead with no net benefit.

### 3. No Official Go Bindings

`libsignal`'s official language targets are Java, Swift, and TypeScript. Integration into Pheromone's Go codebase requires CGO/FFI, which:
- Breaks cross-compilation (CGO disables `GOARCH`/`GOOS` cross-compile)
- Adds a C interop layer that is harder to lint and profile
- Violates the project's preference for pure-Go dependencies (ADR-001)

### 4. AGPL v3 Licensing Constraint

The GNU AGPL v3 licence requires any software that uses and distributes Signal components over a network to release its own source code under the same terms. While Pheromone is currently open-source, adopting AGPL-licensed components restricts future commercial deployment options and must be explicitly accepted by project governance before integration. There is no current governance decision to accept AGPL dependencies.

### 5. Operational Complexity Exceeds Requirements

Signal-Server's dependency tree (FoundationDB, Redis, DynamoDB, S3) is designed for billions of messages from millions of concurrent human users. Pheromone's Phase 1 MVP targets single-server deployment (ADR-002). Introducing this dependency stack would violate the Constitution's minimal-dependency, rapid-deployment principle.

### 6. mTLS on gRPC Provides Equivalent Security

For server↔agent channels where **both endpoints are known, provisioned, and mutually authenticated**:

- **mTLS (mutual TLS)** provides mutual authentication, encryption in transit, and certificate-based identity — equivalent to what Signal Protocol adds for this use case.
- gRPC has **native mTLS support** via standard Go `crypto/tls` and `google.golang.org/grpc/credentials`.
- mTLS certificates can be rotated automatically (cert-manager, Vault PKI) with zero application code changes.
- No pre-key distribution server, Double Ratchet session state, or sealed-sender infrastructure is required.

**Security comparison for server↔agent use case**:

| Property | Signal Protocol | mTLS on gRPC |
|---|---|---|
| Mutual authentication | ✅ (X3DH identity keys) | ✅ (X.509 client + server certs) |
| Encryption in transit | ✅ (Double Ratchet AEAD) | ✅ (TLS 1.3 with AEAD ciphers) |
| Forward secrecy | ✅ (per-message ratchet) | ✅ (TLS 1.3 ephemeral DH) |
| Sender anonymity | ✅ (Sealed Sender) | ❌ (not needed: both ends known) |
| High-throughput streaming | ❌ (ratchet overhead) | ✅ (native gRPC streams) |
| Go ecosystem fit | ❌ (no official bindings) | ✅ (first-class support) |
| Operational complexity | ❌ (pre-key server required) | ✅ (cert management only) |
| Licence compatibility | ⚠️ AGPL v3 | ✅ Apache 2.0 / BSD |
| Pheromone transport fit | ❌ (HTTP/WebSocket) | ✅ (gRPC/HTTP2) |

---

## What Parts of signalapp Are Potentially Relevant (Future Consideration)

While the full Signal Protocol stack is not suitable, two narrowly scoped components warrant future consideration if specific requirements arise:

### A. `libsignal` AEAD Primitives (AES-GCM-SIV)

`signal-crypto` in libsignal implements AES-GCM-SIV, a nonce-misuse-resistant AEAD cipher. If Pheromone ever requires application-layer payload encryption beyond TLS (e.g., encrypting twin model payloads at rest in etcd), this primitive could be considered — but would need AGPL licence governance approval first. RustCrypto's `aes-gcm-siv` crate (MIT/Apache-licensed) provides the same primitive without the AGPL constraint and is the preferred alternative.

### B. Sealed Sender Concept (Multi-Hop Future Topology)

If Pheromone evolves to support multi-hop agent topologies (agents routing through relay nodes), the concept of sealed sender — hiding which agent originated a message from intermediate relays — could be architecturally valuable. This would be addressed in a future ADR (not ADR-010) as a dedicated anonymity-routing mechanism, likely using a different implementation that does not depend on the Signal stack.

### C. SparsePostQuantumRatchet (Post-Quantum Future)

`SparsePostQuantumRatchet` (SPQR) extends Double Ratchet with post-quantum key encapsulation. If Pheromone requires post-quantum secure agent↔server channels in the future (e.g., for regulated/classified environments), the techniques documented here are relevant research material. Post-quantum TLS (via X25519Kyber768 or similar) integrated into gRPC would be the preferred implementation path, not Signal's messaging stack.

---

## Consequences

### Positive

- **Decision is simple**: gRPC + mTLS continues as the sole server↔agent protocol (no additional protocol to operate or debug)
- **No AGPL licensing risk**: All current dependencies remain MIT/Apache/BSD
- **No Go ecosystem friction**: mTLS is supported natively by `crypto/tls` and `google.golang.org/grpc/credentials`
- **Operational simplicity preserved**: No pre-key server, no FoundationDB, no Redis (beyond ADR-002/005 decisions)
- **Throughput targets met**: gRPC bidirectional streaming handles 100K msgs/sec without per-message cryptographic ratcheting

### Negative

- **No application-layer forward secrecy beyond TLS 1.3**: Messages are protected by the TLS session; if a TLS session key is compromised, past messages in that session are exposed (mitigated by short TLS session lifetimes and frequent cert rotation)
- **Agent identity is visible to server**: Unlike Sealed Sender, server always knows which agent sent which message (acceptable — server must know agent identity for twin management)
- **Post-quantum readiness deferred**: Future post-quantum TLS integration is needed once NIST PQC standards stabilise in gRPC library support

### Follow-Up Actions

- **mTLS Enablement** (ADR-003 update): Document mTLS configuration requirements for AgentRegistry, TwinControl, and TelemetryStream gRPC services; include cert rotation policy
- **Cert Management ADR** (ADR-TBD, Phase 2): Define PKI strategy for fleet-wide agent certificate provisioning (Vault PKI recommended)
- **Post-Quantum TLS Assessment** (ADR-TBD, Phase 3): Evaluate X25519Kyber768 hybrid key exchange in gRPC once library support matures

---

## Update — 2026-02-21

ADR-010 records the outcome of a deliberate review of the `signalapp` GitHub organisation requested as a communication protocol evaluation. Signal Protocol is found unsuitable due to transport mismatch, AGPL licensing, absent Go bindings, and throughput constraints. mTLS on gRPC remains the correct security architecture for Pheromone server↔agent channels.

---

## References

- signalapp GitHub organisation: <https://github.com/signalapp>
- libsignal repository: <https://github.com/signalapp/libsignal>
- Signal-Server repository: <https://github.com/signalapp/Signal-Server>
- Signal Protocol documentation: <https://signal.org/docs/>
- Double Ratchet Algorithm: <https://signal.org/docs/specifications/doubleratchet/>
- X3DH Key Agreement: <https://signal.org/docs/specifications/x3dh/>
- SparsePostQuantumRatchet: <https://github.com/signalapp/SparsePostQuantumRatchet>
- RustCrypto AES-GCM-SIV: <https://github.com/RustCrypto/AEADs/tree/master/aes-gcm-siv>
- gRPC TLS documentation: <https://grpc.io/docs/guides/auth/>
- ADR-001 (Digital Twin Architecture — gRPC as primary protocol)
- ADR-003 (gRPC Service Contracts — AgentRegistry, TwinControl, TelemetryStream)
- ADR-005 (Message Queue Selection — NATS for telemetry)
- Constitution Principle III (Language-Agnostic Protocol Foundation)

---

**Decision Date**: 2026-02-21
**Status**: Accepted — 2026-03-09

## Update — 2026-03-09

ADR-010 formally accepted. Signal Protocol rejection rationale reviewed (5 reasons: architectural mismatch, transport incompatibility, no Go bindings, AGPL v3 licence constraint, operational complexity). mTLS security comparison table reviewed and confirmed adequate for server↔agent use case. AGPL v3 licence constraint acknowledged by project governance — no AGPL dependencies to be introduced without governance decision. Follow-up action confirmed: mTLS enablement requirements documented in ADR-003 update (see ADR-003 Update — 2026-03-09). gRPC + mTLS remains the sole server↔agent security architecture for Pheromone.
