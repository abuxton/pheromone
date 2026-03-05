# ADR 012: Vagrant Testing Environment for Server and Agent Validation

## Status

Accepted

**Decision Date**: 2026-02-22
**Accepted Date**: 2026-03-05

## Context

Pheromone agents and the management server must be validated on real Linux distributions before
production deployment. The ADR-008 host OS recommendation designates:

- **Ubuntu Server 24.04 LTS** — primary target for standard cloud/on-premises instances
- **Debian 12 (Bookworm)** — viable alternative for Debian-ecosystem operators

Current development is performed on developer workstations (macOS/Linux) using Docker Compose
for etcd only (ADR-002 tech spike). There is no mechanism to:

1. Boot a representative Linux VM and exercise the server + agent end-to-end
2. Validate provisioning scripts across distributions before writing configuration-management code
3. Replicate the agent's actual OS environment (systemd, package managers, file-system layout) locally

**Requirements**:
- Reproducible multi-machine environment usable without cloud accounts
- Supports the two OS targets from ADR-008 (Ubuntu 24.04 LTS, Debian 12)
- Allows a developer to run `vagrant up` and get a working server ↔ agent topology
- Must not require GPU or large model downloads; AI reasoning (ADR-008 Tier 1) is optional
- Lightweight enough to run on a developer laptop (8 GB RAM minimum)

**Options Considered**:

| Option | Notes |
|---|---|
| Docker Compose only | Already used for etcd; containers share kernel, not a real OS environment |
| Vagrant + VirtualBox | Cross-platform, free, widely used; good multi-machine support |
| Vagrant + libvirt/KVM | Better performance on Linux; less portable for macOS/Windows developers |
| Multipass (Canonical) | Ubuntu-only; not suitable for Debian target |
| Lima (macOS) | macOS-only; not portable |
| Dev containers | Code environment only; not OS-level provisioning testing |

## Decision

**Use Vagrant with VirtualBox as the default provider to create a multi-machine local testing
environment with Ubuntu 24.04 LTS and Debian 12.**

### Environment Topology

```
┌──────────────────────────────────────────────────────────────────┐
│                  Vagrant Private Network (192.168.56.0/24)       │
│                                                                  │
│  ┌────────────────────────────┐                                  │
│  │  server (192.168.56.10)    │                                  │
│  │  Ubuntu 24.04 LTS          │                                  │
│  │  - etcd (via Docker)       │                                  │
│  │  - pheromone server (TBD)  │                                  │
│  │  - Go 1.24 toolchain       │                                  │
│  │  2 vCPU / 1024 MB RAM      │                                  │
│  └────────────┬───────────────┘                                  │
│               │ gRPC (future)                                    │
│  ┌────────────▼───────────────┐  ┌──────────────────────────┐   │
│  │ agent-ubuntu (192.168.56.11)│  │agent-debian (192.168.56.12)│ │
│  │ Ubuntu 24.04 LTS           │  │ Debian 12 Bookworm        │  │
│  │ - pheromone agent (TBD)    │  │ - pheromone agent (TBD)   │  │
│  │ - Go 1.24 toolchain        │  │ - Go 1.24 toolchain       │  │
│  │ 1 vCPU / 512 MB RAM        │  │ 1 vCPU / 512 MB RAM       │  │
│  └────────────────────────────┘  └──────────────────────────────┘
└──────────────────────────────────────────────────────────────────┘
```

### Directory Layout

```
vagrant/
  Vagrantfile          Multi-machine Vagrant configuration
  scripts/
    common.sh          Shared provisioning: Go, Docker, project sync
    server.sh          Server provisioning: etcd, server setup
    agent.sh           Agent provisioning: agent setup
  README.md            Usage instructions and quick-start guide
```

### Vagrant Box Selection

| Machine | Box | Rationale |
|---|---|---|
| `server` | `bento/ubuntu-24.04` | Official Bento Ubuntu 24.04 LTS; well-maintained |
| `agent-ubuntu` | `bento/ubuntu-24.04` | Same OS as server; validates same-distro agent operation |
| `agent-debian` | `bento/debian-12` | Debian 12 Bookworm; validates cross-distro agent operation |

### Go Installation

Go `1.24` is installed via the official upstream tarball on all VMs (not via `apt`/`snap`) to:
- Ensure the same Go version as `go.mod` (`go 1.24`) across all distributions
- Avoid distribution-packaged Go versions that lag upstream

### etcd

etcd is deployed on the `server` VM via Docker Compose (matching the existing
`docker-compose.yml` in the repository root), reusing the validated ADR-002 configuration.

## Consequences

### Positive
- **Reproducibility**: `vagrant up` creates a clean, identical environment on every developer's machine
- **Cross-distro validation**: Ubuntu and Debian coverage satisfies ADR-008 OS recommendation
- **Low barrier to entry**: VirtualBox is available on macOS, Windows, and Linux; no cloud account needed
- **Aligned with ADR-008**: Boxes map directly to ADR-008's recommended OS targets
- **Iterative provisioning**: `vagrant provision` re-runs scripts without destroying VMs
- **Isolation**: VMs are fully isolated from the host; no port conflicts or dependency pollution

### Negative
- **Resource usage**: Three VMs consume ~2–3 GB RAM when all running simultaneously; single-agent mode available
- **VirtualBox dependency**: VirtualBox not available on Apple Silicon Macs without Rosetta; libvirt can be used as alternative provider on Linux
- **Boot time**: Initial `vagrant up --provision` takes 5–10 minutes (box download + Go install)
- **Not CI-ready as-is**: VMs are for local developer use; CI uses Docker Compose (existing)

### Migration / Compatibility
- The `vagrant/` directory is self-contained; existing Docker Compose workflows are unaffected
- The `.gitignore` already excludes `.vagrant/` and `*.box`
- A future ADR may introduce a libvirt provider profile for Linux-only CI environments

## Testing Strategy

- `vagrant up server` → etcd starts; `vagrant ssh server` → `curl http://192.168.56.10:2379/health` returns healthy
- `vagrant up agent-ubuntu` → Go toolchain available; `vagrant ssh agent-ubuntu` → `go version` outputs `go1.24`
- `vagrant up agent-debian` → same validation on Debian 12
- `vagrant validate` (Vagrant built-in) → Vagrantfile syntax is valid

## References

- ADR-008 (AI Model Selection — Host OS Recommendation: Ubuntu 24.04 LTS primary, Debian 12 alternative)
- ADR-002 (Server Architecture — etcd Docker Compose configuration reused)
- ADR-001 (Go as primary implementation language)
- Constitution Principle IV (Smoke tests — VMs provide the OS environment for integration testing)
- [Bento boxes](https://app.vagrantup.com/bento)
- [Vagrant multi-machine documentation](https://developer.hashicorp.com/vagrant/docs/multi-machine)
- [Go official downloads](https://go.dev/dl/)

---

**Decision Date**: 2026-02-22
**Accepted Date**: 2026-03-05
