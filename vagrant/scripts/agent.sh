#!/usr/bin/env bash
# agent.sh — Provisioning for Pheromone agent VMs (Ubuntu and Debian)
# ADR-012: Vagrant Testing Environment
#
# Configures an agent VM:
#   - Verifies Go toolchain
#   - Builds pheromone agent binary (when available)
#   - Logs connectivity information
#
# Environment variables (set by Vagrantfile):
#   SERVER_IP  — Server VM IP for gRPC connectivity
#   AGENT_IP   — This agent VM's private network IP
#   GO_VERSION — Go toolchain version

set -euo pipefail

SERVER_IP="${SERVER_IP:-192.168.56.10}"
AGENT_IP="${AGENT_IP:-192.168.56.11}"
PROJECT_DIR="/home/vagrant/pheromone"

log() { echo "[agent.sh] $*"; }

export PATH="/usr/local/go/bin:${PATH}"

# ── Verify Go is available ───────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "[agent.sh] ERROR: Go not found. Ensure common.sh ran first." >&2
  exit 1
fi
log "Go version: $(go version)"

# ── Verify server reachability ───────────────────────────────────────────────
if curl -sf "http://${SERVER_IP}:2379/health" > /dev/null 2>&1; then
  log "etcd on server (${SERVER_IP}:2379) is reachable."
else
  log "Note: etcd on ${SERVER_IP}:2379 is not reachable from this VM."
  log "Start the server VM first: vagrant up server"
fi

# ── Build pheromone agent (when available) ───────────────────────────────────
if [[ -f "${PROJECT_DIR}/cmd/agent/main.go" ]]; then
  log "Building pheromone agent binary..."
  cd "${PROJECT_DIR}"
  go build -o /usr/local/bin/pheromone-agent ./cmd/agent/
  log "pheromone agent binary installed to /usr/local/bin/pheromone-agent"
else
  log "pheromone agent (cmd/agent/main.go) not yet implemented — skipping build."
fi

log "Agent provisioning complete."
log "  Agent IP:  ${AGENT_IP}"
log "  Server IP: ${SERVER_IP}"
log "  SSH: vagrant ssh agent-ubuntu  (or agent-debian)"
