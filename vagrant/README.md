# Pheromone Vagrant Testing Environment

This directory contains the Vagrant configuration for running a local multi-machine testing
environment for Pheromone server and agent development.

See [ADR-012](../docs/adr/adr-012-vagrant-testing-environment.md) for the full decision rationale.

## Prerequisites

- [Vagrant](https://developer.hashicorp.com/vagrant/install) ≥ 2.3.0
- [VirtualBox](https://www.virtualbox.org/wiki/Downloads) ≥ 6.1
- 8 GB RAM recommended (all three VMs consume ~2 GB when running simultaneously)

> **Apple Silicon (M1/M2/M3) note**: VirtualBox does not support ARM natively. Use
> [VMware Fusion](https://www.vmware.com/products/fusion.html) with the Vagrant VMware provider,
> or [UTM](https://mac.getutm.app/) with the `vagrant-utm` plugin as alternatives.

## VM Topology

| VM | Box | IP | Role |
|---|---|---|---|
| `server` | `bento/ubuntu-24.04` | `192.168.56.10` | etcd + future pheromone server |
| `agent-ubuntu` | `bento/ubuntu-24.04` | `192.168.56.11` | Pheromone agent (Ubuntu 24.04 LTS) |
| `agent-debian` | `bento/debian-12` | `192.168.56.12` | Pheromone agent (Debian 12 Bookworm) |

## Quick Start

```bash
# Start all three VMs (first run downloads box images ~1 GB each)
cd vagrant/
vagrant up

# Or start a single VM
vagrant up server
vagrant up agent-ubuntu
vagrant up agent-debian
```

### Verify the Server

```bash
vagrant ssh server

# Inside the VM:
ph-etcd-health           # Check etcd health
ph-test                  # Run offline validation tests
ph-test-etcd             # Run etcd integration tests
docker compose ps        # Show running containers
```

### Verify an Agent VM

```bash
vagrant ssh agent-ubuntu   # or agent-debian

# Inside the VM:
ph-server-health           # Check server etcd reachability
ph-build                   # Build the project
ph-test                    # Run offline unit tests
ph-lint                    # Format and vet code
go version                 # Verify Go installation
```

## Common Commands

```bash
# Reprovisioning (re-run provisioning scripts without destroying VMs)
vagrant provision server
vagrant provision agent-ubuntu

# Stop VMs (preserves disk state)
vagrant halt

# Restart VMs
vagrant reload

# Destroy all VMs and release disk space
vagrant destroy -f

# Check VM status
vagrant status

# Validate Vagrantfile syntax
vagrant validate
```

## Project Files Inside VMs

The repository root is synced into each VM at `/home/vagrant/pheromone`.
Changes made on the host are immediately reflected inside the VM.

```bash
vagrant ssh server
ls ~/pheromone   # same as repository root on host
```

## Running Tests Against Live etcd

With the server VM running, connect from an agent VM to run the full integration suite:

```bash
vagrant ssh agent-ubuntu

# Inside agent-ubuntu VM:
export ETCD_ENDPOINT="http://192.168.56.10:2379"
cd ~/pheromone
go test -v ./benchmark -run TestEtcd
go test -v ./benchmark -run TestHybrid
go test -v ./benchmark -run TestServerRecoveryTime
```

## Troubleshooting

### etcd not healthy on server VM

```bash
vagrant ssh server
docker compose logs etcd    # inspect etcd logs
ph-etcd-down && ph-etcd-up  # restart etcd
```

### Agent cannot reach server

Ensure the server VM is running and the private network interface is up:

```bash
vagrant status              # check VM states
vagrant ssh agent-ubuntu
ping 192.168.56.10          # test network connectivity
ph-server-health            # test etcd endpoint
```

### Go binary not found

The Go installation script writes to `/etc/profile.d/go.sh`. If `go` is not found after SSH:

```bash
source /etc/profile.d/go.sh
go version
```

### Re-provision after code changes to provisioning scripts

```bash
vagrant provision server
vagrant provision agent-ubuntu
vagrant provision agent-debian
```
