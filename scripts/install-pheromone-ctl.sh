#!/usr/bin/env bash
# install-pheromone-ctl.sh — Install pheromone-ctl from GitHub Releases
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/abuxton/pheromone/main/scripts/install-pheromone-ctl.sh | bash
#   curl -fsSL ... | bash -s -- --version v1.0.0 --install-dir /usr/local/bin
#
# Options:
#   --version <v>      Specific version to install (default: latest)
#   --install-dir <d>  Directory to install the binary (default: /usr/local/bin)
#   --no-verify        Skip SHA256 checksum verification

set -euo pipefail

REPO="abuxton/pheromone"
BINARY="pheromone-ctl"
VERSION=""
INSTALL_DIR="/usr/local/bin"
VERIFY=true

# -------------------------------------------------------------------------
# Parse arguments
# -------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      shift 2
      ;;
    --no-verify)
      VERIFY=false
      shift
      ;;
    -h|--help)
      echo "Usage: install-pheromone-ctl.sh [--version <v>] [--install-dir <dir>] [--no-verify]"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

# -------------------------------------------------------------------------
# Detect OS and architecture
# -------------------------------------------------------------------------
detect_platform() {
  local os arch

  os="$(uname -s)"
  case "$os" in
    Linux*)  os="linux" ;;
    Darwin*) os="darwin" ;;
    *)
      echo "Unsupported OS: $os" >&2
      exit 1
      ;;
  esac

  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "Unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac

  echo "${os}_${arch}"
}

# -------------------------------------------------------------------------
# Fetch latest release version from GitHub API
# -------------------------------------------------------------------------
latest_version() {
  local url="https://api.github.com/repos/${REPO}/releases"
  local version

  if command -v curl &>/dev/null; then
    version=$(curl -fsSL "${url}" \
      | grep '"tag_name"' \
      | grep '"pheromone-ctl/' \
      | head -1 \
      | sed 's/.*"pheromone-ctl\/\(v[^"]*\)".*/\1/' || true)
  elif command -v wget &>/dev/null; then
    version=$(wget -qO- "${url}" \
      | grep '"tag_name"' \
      | grep '"pheromone-ctl/' \
      | head -1 \
      | sed 's/.*"pheromone-ctl\/\(v[^"]*\)".*/\1/' || true)
  else
    echo "Error: curl or wget is required" >&2
    exit 1
  fi

  if [[ -z "$version" ]]; then
    echo "Error: could not determine latest version" >&2
    exit 1
  fi

  echo "$version"
}

# -------------------------------------------------------------------------
# Download a file using curl or wget
# -------------------------------------------------------------------------
download() {
  local url="$1"
  local dest="$2"

  if command -v curl &>/dev/null; then
    curl -fsSL -o "$dest" "$url"
  elif command -v wget &>/dev/null; then
    wget -qO "$dest" "$url"
  else
    echo "Error: curl or wget is required" >&2
    exit 1
  fi
}

# -------------------------------------------------------------------------
# Verify SHA256 checksum
# -------------------------------------------------------------------------
verify_checksum() {
  local archive="$1"
  local checksums_file="$2"
  local archive_basename
  archive_basename="$(basename "$archive")"

  local expected
  expected=$(grep "${archive_basename}" "$checksums_file" | awk '{print $1}')

  if [[ -z "$expected" ]]; then
    echo "Error: checksum not found for ${archive_basename}" >&2
    exit 1
  fi

  local actual
  if command -v sha256sum &>/dev/null; then
    actual=$(sha256sum "$archive" | awk '{print $1}')
  elif command -v shasum &>/dev/null; then
    actual=$(shasum -a 256 "$archive" | awk '{print $1}')
  else
    echo "Warning: sha256sum/shasum not found; skipping checksum verification" >&2
    return 0
  fi

  if [[ "$actual" != "$expected" ]]; then
    echo "Error: checksum mismatch for ${archive_basename}" >&2
    echo "  expected: ${expected}" >&2
    echo "  actual:   ${actual}" >&2
    exit 1
  fi

  echo "Checksum verified: ${archive_basename}"
}

# -------------------------------------------------------------------------
# Main
# -------------------------------------------------------------------------
main() {
  local platform
  platform="$(detect_platform)"

  if [[ -z "$VERSION" ]]; then
    echo "Fetching latest pheromone-ctl release..."
    VERSION="$(latest_version)"
  fi

  echo "Installing pheromone-ctl ${VERSION} (${platform})..."

  local base_url="https://github.com/${REPO}/releases/download/pheromone-ctl/${VERSION}"
  local archive="${BINARY}_${VERSION}_${platform}.tar.gz"
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading ${archive}..."
  download "${base_url}/${archive}" "${tmpdir}/${archive}"

  if [[ "$VERIFY" == "true" ]]; then
    echo "Downloading checksums..."
    download "${base_url}/checksums.txt" "${tmpdir}/checksums.txt"
    verify_checksum "${tmpdir}/${archive}" "${tmpdir}/checksums.txt"
  fi

  echo "Extracting..."
  tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}"

  echo "Installing to ${INSTALL_DIR}/${BINARY}..."
  if [[ -w "$INSTALL_DIR" ]]; then
    mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"
  else
    sudo mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    sudo chmod +x "${INSTALL_DIR}/${BINARY}"
  fi

  echo ""
  echo "pheromone-ctl ${VERSION} installed successfully!"
  echo ""
  echo "Run: pheromone-ctl --help"
  echo ""
  echo "Quick start:"
  echo "  export PHEROMONE_CTL_SERVER=http://your-server:8081"
  echo "  pheromone-ctl login --username admin --password <password>"
}

main "$@"
