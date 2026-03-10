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
| `server` | `bento/ubuntu-24.04` | `192.168.56.10` | etcd + pheromone-server + Management UI |
| `agent-ubuntu` | `bento/ubuntu-24.04` | `192.168.56.11` | Pheromone agent (Ubuntu 24.04 LTS) |
| `agent-debian` | `bento/debian-12` | `192.168.56.12` | Pheromone agent (Debian 12 Bookworm) |

## Port Forwarding

The Vagrantfile forwards these ports from the server VM to your host machine:

| Service | Guest Port | Host Port | URL |
|---|---|---|---|
| Management UI | 8081 | 8081 | http://localhost:8081 |
| etcd | 2379 | 2379 | http://localhost:2379/health |

## Quick Start

```bash
# Start all three VMs (first run downloads box images ~1 GB each)
cd vagrant/
vagrant up

# Or start the server VM only (includes the management UI)
vagrant up server
```

Once the server VM is up, open the management UI in your host browser:

```
http://localhost:8081
```

Default credentials: **admin / admin** (also available: **viewer / viewer**).

## Management UI

The `pheromone-server serve` process runs as a systemd service (`pheromone-ui`) on the server VM.
It provides a browser-based console for managing agents, digital twins, skills, groups, change
sets, and connections.

### Accessing the UI

| Path | Description |
|---|---|
| `http://localhost:8081` | Management console (login page) |
| `http://localhost:8081/api/v1/health` | Health check (no auth required) |
| `http://localhost:8081/api/v1/stats` | Runtime stats (requires Bearer token) |

### UI Helper Commands (server VM)

```bash
vagrant ssh server

ph-ui-health     # GET /api/v1/health and print JSON summary
ph-ui-status     # Show systemd service status
ph-ui-logs       # Tail live UI logs (Ctrl-C to stop)
ph-ui-restart    # Restart the pheromone-ui service
```

### UI Configuration

The UI configuration is written during provisioning to `/etc/pheromone/server/ui.yaml`.
A random secret key and hashed passwords are generated at provision time.

To change the admin password after provisioning:

```bash
vagrant ssh server

# Generate a new password hash
NEW_HASH=$(pheromone-server ui hash-password mynewpassword)

# Edit the config
sudo sed -i "s|password_hash: .*admin.*|password_hash: \"${NEW_HASH}\"|" \
  /etc/pheromone/server/ui.yaml

# Restart the service to pick up the change
ph-ui-restart
```

### Verifying the UI via API

You can exercise the API directly from the server VM or any agent VM:

```bash
# Login and capture token
TOKEN=$(curl -sf -X POST http://192.168.56.10:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')

# List agents
curl -sf -H "Authorization: Bearer ${TOKEN}" \
  http://192.168.56.10:8081/api/v1/agents | python3 -m json.tool

# List twins
curl -sf -H "Authorization: Bearer ${TOKEN}" \
  http://192.168.56.10:8081/api/v1/twins | python3 -m json.tool
```

### Verifying the Selector

```bash
vagrant ssh server

# Verify etcd and UI are both healthy
ph-etcd-health
ph-ui-health
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

## Running API Tests Against the Live UI

With the server VM running, run the API package tests against the live server:

```bash
vagrant ssh server

# Run all UI/API unit tests
cd ~/pheromone
go test -v -race ./internal/api/...

# Or run a single test
go test -v ./internal/api/... -run TestHandleLogin_Success
```

## Troubleshooting

### Management UI not reachable on http://localhost:8081

```bash
vagrant ssh server
ph-ui-status     # check systemd service
ph-ui-logs       # inspect logs
ph-ui-restart    # restart the service
```

If the binary is missing (e.g., first provision before source code was synced):

```bash
vagrant ssh server
cd ~/pheromone
go build -o /usr/local/bin/pheromone-server ./cmd/pheromone-server/
ph-ui-restart
```

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
ph-ui-health                # test management UI endpoint
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

