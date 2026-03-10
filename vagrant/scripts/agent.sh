#!/usr/bin/env bash
# agent.sh — Provisioning for Pheromone agent VMs (Ubuntu and Debian)
# ADR-012: Vagrant Testing Environment
#
# Configures an agent VM:
#   - Verifies Go toolchain
#   - Builds pheromone-agent binary
#   - Logs connectivity information for server gRPC and UI endpoints
#
# Environment variables (set by Vagrantfile):
#   SERVER_IP  — Server VM IP for gRPC connectivity
#   UI_PORT    — Management UI HTTP port on the server
#   AGENT_IP   — This agent VM's private network IP
#   GO_VERSION — Go toolchain version

set -euo pipefail

SERVER_IP="${SERVER_IP:-192.168.56.10}"
UI_PORT="${UI_PORT:-8081}"
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

if curl -sf "http://${SERVER_IP}:${UI_PORT}/api/v1/health" > /dev/null 2>&1; then
  log "Management UI on server (${SERVER_IP}:${UI_PORT}) is reachable."
else
  log "Note: Management UI on ${SERVER_IP}:${UI_PORT} is not yet reachable."
  log "This is expected if the server VM has not finished provisioning."
fi

# ── Build pheromone-agent binary ─────────────────────────────────────────────
AGENT_CMD_PATH="${PROJECT_DIR}/cmd/pheromone-agent/main.go"
if [[ -f "${AGENT_CMD_PATH}" ]]; then
  log "Building pheromone-agent binary..."
  cd "${PROJECT_DIR}"
  go build -o /usr/local/bin/pheromone-agent ./cmd/pheromone-agent/
  log "pheromone-agent installed to /usr/local/bin/pheromone-agent"
else
  log "pheromone-agent (${AGENT_CMD_PATH}) not found — skipping build."
fi

# ── Helper commands (agent VM) ──────────────────────────────────────────────
log "Installing agent helper commands..."

cat > /usr/local/bin/ph-server-health <<SCRIPT
#!/usr/bin/env bash
curl -sf "http://${SERVER_IP}:2379/health"
SCRIPT

cat > /usr/local/bin/ph-ui-health <<SCRIPT
#!/usr/bin/env bash
# Check the management UI health endpoint on the server from this agent VM
URL="http://${SERVER_IP}:${UI_PORT}/api/v1/health"
echo "Checking UI health at \${URL} ..."
if curl -sf "\${URL}" 2>/dev/null | python3 -m json.tool 2>/dev/null; then
  echo ""
  echo "Management UI is reachable. Open http://localhost:${UI_PORT} in your host browser."
else
  echo "Management UI health check failed."
  echo "Ensure the server VM is up: vagrant up server"
fi
SCRIPT

chmod +x \
  /usr/local/bin/ph-server-health \
  /usr/local/bin/ph-ui-health

log "Agent provisioning complete."
log "  Agent IP:       ${AGENT_IP}"
log "  Server IP:      ${SERVER_IP}"
log "  Management UI:  http://${SERVER_IP}:${UI_PORT}  (admin / admin)"
log "  SSH: vagrant ssh agent-ubuntu  (or agent-debian)"

