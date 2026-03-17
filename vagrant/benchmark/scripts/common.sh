#!/usr/bin/env bash
# common.sh — Install Go toolchain on Ubuntu 24.04 (shared with ADR-012 scripts)
set -euo pipefail

GO_VERSION="${GO_VERSION:-1.24.1}"
GO_ARCH="amd64"
GO_TAR="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
GO_URL="https://go.dev/dl/${GO_TAR}"
INSTALL_DIR="/usr/local"

echo "[common] Installing Go ${GO_VERSION}..."

if [ -x "${INSTALL_DIR}/go/bin/go" ]; then
  INSTALLED=$(${INSTALL_DIR}/go/bin/go version | awk '{print $3}' | sed 's/go//')
  if [ "${INSTALLED}" = "${GO_VERSION}" ]; then
    echo "[common] Go ${GO_VERSION} already installed — skipping."
    exit 0
  fi
fi

apt-get update -qq
apt-get install -y -qq curl wget tar jq bc 2>/dev/null

curl -fsSL "${GO_URL}" -o "/tmp/${GO_TAR}"
rm -rf "${INSTALL_DIR}/go"
tar -C "${INSTALL_DIR}" -xzf "/tmp/${GO_TAR}"
rm "/tmp/${GO_TAR}"

# Add Go to PATH for all users
cat > /etc/profile.d/go.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
EOF

export PATH=$PATH:/usr/local/go/bin
echo "[common] Go $(go version) installed."
