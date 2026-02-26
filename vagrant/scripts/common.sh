#!/usr/bin/env bash
# common.sh — Shared provisioning for all Pheromone Vagrant VMs
# ADR-012: Vagrant Testing Environment
#
# Installs: Go (upstream tarball), Docker, Docker Compose, essential tools.
# Called by server.sh and agent.sh provisioners.
#
# Environment variables (set by Vagrantfile):
#   GO_VERSION — Go toolchain version to install (e.g., "1.24.1")

set -euo pipefail

GO_VERSION="${GO_VERSION:-1.24.1}"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
GO_INSTALL_DIR="/usr/local"

log() { echo "[common.sh] $*"; }

# ── Essential packages ──────────────────────────────────────────────────────
log "Installing essential packages..."
apt-get update -qq
apt-get install -y --no-install-recommends \
  curl \
  git \
  make \
  ca-certificates \
  gnupg \
  lsb-release \
  2>/dev/null

# ── Docker ──────────────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
  log "Installing Docker..."
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/$(. /etc/os-release; echo "$ID") \
    $(lsb_release -cs) stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null
  apt-get update -qq
  apt-get install -y --no-install-recommends \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin
  usermod -aG docker vagrant
  systemctl enable docker
  systemctl start docker
  log "Docker installed: $(docker --version)"
else
  log "Docker already installed: $(docker --version)"
fi

# ── Go toolchain ─────────────────────────────────────────────────────────────
INSTALLED_GO=""
if command -v go &>/dev/null; then
  INSTALLED_GO=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
fi

if [[ "${INSTALLED_GO}" != "${GO_VERSION}" ]]; then
  log "Installing Go ${GO_VERSION}..."
  curl -fsSL "${GO_URL}" -o "/tmp/${GO_TARBALL}"
  rm -rf "${GO_INSTALL_DIR}/go"
  tar -C "${GO_INSTALL_DIR}" -xzf "/tmp/${GO_TARBALL}"
  rm "/tmp/${GO_TARBALL}"

  # Add Go to system PATH
  cat > /etc/profile.d/go.sh <<'GOPATH'
export PATH="/usr/local/go/bin:${PATH}"
GOPATH
  chmod +x /etc/profile.d/go.sh

  # Add to vagrant user's .bashrc for interactive sessions
  if ! grep -q '/usr/local/go/bin' /home/vagrant/.bashrc; then
    echo 'export PATH="/usr/local/go/bin:${PATH}"' >> /home/vagrant/.bashrc
  fi

  export PATH="/usr/local/go/bin:${PATH}"
  log "Go installed: $(go version)"
else
  log "Go ${GO_VERSION} already installed."
fi

log "Common provisioning complete."
