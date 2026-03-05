#!/usr/bin/env bash
# server.sh — Provisioning for the Pheromone server VM
# ADR-012: Vagrant Testing Environment
#
# Configures the server VM:
#   - Starts etcd via docker compose (reuses project docker-compose.yml)
#   - Verifies etcd health
#   - Placeholder for future pheromone server binary
#
# Environment variables (set by Vagrantfile):
#   SERVER_IP  — Server VM private network IP (e.g., "192.168.56.10")
#   GO_VERSION — Go toolchain version

set -euo pipefail

SERVER_IP="${SERVER_IP:-192.168.56.10}"
PROJECT_DIR="/home/vagrant/pheromone"
ETCD_HEALTH_URL="http://${SERVER_IP}:2379/health"
ETCD_RETRY_SECS=30

log() { echo "[server.sh] $*"; }

export PATH="/usr/local/go/bin:${PATH}"

# ── Verify Go is available ───────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "[server.sh] ERROR: Go not found. Ensure common.sh ran first." >&2
  exit 1
fi
log "Go version: $(go version)"

# ── etcd via Docker Compose ──────────────────────────────────────────────────
if [[ ! -f "${PROJECT_DIR}/docker-compose.yml" ]]; then
  log "WARNING: ${PROJECT_DIR}/docker-compose.yml not found — skipping etcd start."
  log "Mount the project directory and re-provision to start etcd."
else
  log "Starting etcd via docker compose..."
  cd "${PROJECT_DIR}"
  docker compose up -d etcd 2>&1 || docker-compose up -d etcd 2>&1
  log "Waiting up to ${ETCD_RETRY_SECS}s for etcd to become healthy..."
  for i in $(seq 1 "${ETCD_RETRY_SECS}"); do
    if curl -sf "${ETCD_HEALTH_URL}" > /dev/null 2>&1; then
      log "etcd is healthy at ${ETCD_HEALTH_URL}"
      break
    fi
    if [[ "${i}" -eq "${ETCD_RETRY_SECS}" ]]; then
      log "WARNING: etcd did not become healthy after ${ETCD_RETRY_SECS}s."
      log "Check: docker compose logs etcd"
    fi
    sleep 1
  done
fi

# ── Build pheromone server (when available) ──────────────────────────────────
if [[ -f "${PROJECT_DIR}/cmd/server/main.go" ]]; then
  log "Building pheromone server binary..."
  cd "${PROJECT_DIR}"
  go build -o /usr/local/bin/pheromone-server ./cmd/server/
  log "pheromone server binary installed to /usr/local/bin/pheromone-server"
else
  log "pheromone server (cmd/server/main.go) not yet implemented — skipping build."
fi

# ── Helper commands (server VM) ─────────────────────────────────────────────
log "Installing server helper commands..."

cat > /usr/local/bin/ph-etcd-health <<SCRIPT
#!/usr/bin/env bash
curl -sf "http://${SERVER_IP}:2379/health"
SCRIPT

cat > /usr/local/bin/ph-etcd-up <<SCRIPT
#!/usr/bin/env bash
set -euo pipefail
cd /home/vagrant/pheromone
docker compose up -d etcd
SCRIPT

cat > /usr/local/bin/ph-etcd-down <<SCRIPT
#!/usr/bin/env bash
set -euo pipefail
cd /home/vagrant/pheromone
docker compose stop etcd
SCRIPT

cat > /usr/local/bin/ph-test-etcd <<SCRIPT
#!/usr/bin/env bash
set -euo pipefail
export PATH="/usr/local/go/bin:\${PATH}"
export ETCD_ENDPOINT="http://${SERVER_IP}:2379"
cd /home/vagrant/pheromone
exec go test -v ./benchmark/... -run 'TestEtcd|TestHybrid|TestServerRecoveryTime'
SCRIPT

chmod +x \
  /usr/local/bin/ph-etcd-health \
  /usr/local/bin/ph-etcd-up \
  /usr/local/bin/ph-etcd-down \
  /usr/local/bin/ph-test-etcd

log "Server provisioning complete."
log "  etcd health: curl ${ETCD_HEALTH_URL}"
log "  SSH: vagrant ssh server"
