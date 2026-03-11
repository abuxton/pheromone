#!/usr/bin/env bash
# Post-create setup for Pheromone dev container.
# Installs all tooling required for Go + Rust + gRPC development,
# pre-commit hooks, and the GitHub Copilot CLI extension.
set -euo pipefail

echo "==> Installing protoc (Protocol Buffers compiler)..."
PROTOC_VERSION="27.4"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) PROTOC_ARCH="linux-x86_64" ;;
  aarch64|arm64) PROTOC_ARCH="linux-aarch_64" ;;
  *) echo "Unsupported architecture: ${ARCH}"; exit 1 ;;
esac
curl -sSL \
  "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-${PROTOC_ARCH}.zip" \
  -o /tmp/protoc.zip
unzip -q /tmp/protoc.zip -d /tmp/protoc
sudo mv /tmp/protoc/bin/protoc /usr/local/bin/protoc
sudo cp -r /tmp/protoc/include/* /usr/local/include/
rm -rf /tmp/protoc /tmp/protoc.zip

echo "==> Installing Go tools..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "==> Installing pre-commit..."
pip3 install --quiet --user pre-commit
pre-commit install --install-hooks

echo "==> Installing GitHub Copilot CLI extension..."
gh extension install github/gh-copilot 2>/dev/null || gh extension upgrade gh-copilot 2>/dev/null || true

echo "==> Installing Node.js from distro packages..."
sudo apt-get install -y nodejs
echo "==> Installing AI skills from skills-lock.json..."
# -p: use the skills-lock.json in the current (project) directory
# -y: non-interactive, accept all prompts automatically
npx --yes skills experimental_install -p -y

echo "==> Installing Go module dependencies..."
go mod download

echo "==> Dev container setup complete."
echo "    Run 'make help' to see available targets."
echo "    Run 'make etcd-up' to start etcd for integration tests."
