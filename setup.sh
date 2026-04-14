#!/usr/bin/env bash
# =============================================================================
# setup.sh  –  One-time setup for Docksmith on Linux (or Linux VM on Mac)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="docksmith"
INSTALL_DIR="$HOME/.local/bin"
DOCKSMITH_HOME="$HOME/.docksmith"

# Store base images in home dir (not project dir — project may be read-only in Lima)
BASE_IMAGES_DIR="$HOME/.docksmith-setup"
BASE_IMAGE_TARBALL="$BASE_IMAGES_DIR/python3.11-slim.tar"
BASE_IMAGE_NAME="python:3.11-slim"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

echo ""
echo "=============================================="
echo "  Docksmith – One-Time Setup"
echo "=============================================="
echo ""

# ── 1. Check OS ───────────────────────────────────────────────────────────────
if [[ "$(uname -s)" != "Linux" ]]; then
  echo ""
  warn "You are running on $(uname -s)."
  echo "Docksmith requires Linux namespaces + chroot."
  echo "Please run inside a Linux VM (Lima, Multipass, WSL2, etc.)."
  error "Linux required. Exiting."
fi
success "Running on Linux"

# ── 2. Check Go ───────────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  error "Go is not installed. Run: sudo apt-get install -y golang-go"
fi
GO_VERSION=$(go version | awk '{print $3}')
success "Go found: $GO_VERSION"

# ── 3. Check Docker (needed only for the one-time base-image pull) ────────────
HAVE_DOCKER=true
if ! command -v docker &>/dev/null; then
  HAVE_DOCKER=false
  warn "Docker not found. Will look for a pre-existing base image tarball."
fi

# ── 4. Pull base image if needed ──────────────────────────────────────────────
mkdir -p "$BASE_IMAGES_DIR"

if [[ -f "$BASE_IMAGE_TARBALL" ]]; then
  success "Base image tarball already exists: $BASE_IMAGE_TARBALL"
else
  if [[ "$HAVE_DOCKER" == "false" ]]; then
    error "Docker is required for the one-time base image pull.\nInstall Docker then re-run this script."
  fi
  info "Pulling $BASE_IMAGE_NAME (one-time only, needs internet)..."
  docker pull "$BASE_IMAGE_NAME"
  info "Saving image to $BASE_IMAGE_TARBALL ..."
  docker save "$BASE_IMAGE_NAME" -o "$BASE_IMAGE_TARBALL"
  success "Base image saved."
fi

# ── 5. Build docksmith binary ─────────────────────────────────────────────────
info "Building docksmith binary..."
# Build into a writable temp dir in case SCRIPT_DIR is read-only (Lima mount)
BUILD_DIR="$(mktemp -d)"
cd "$SCRIPT_DIR"
go build -o "$BUILD_DIR/$BINARY_NAME" .
success "Binary built."

# ── 6. Install binary ─────────────────────────────────────────────────────────
mkdir -p "$INSTALL_DIR"
cp "$BUILD_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
rm -rf "$BUILD_DIR"
success "Installed to $INSTALL_DIR/$BINARY_NAME"

# Ensure install dir is on PATH for this session
export PATH="$INSTALL_DIR:$PATH"

# Remind about persistent PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  warn "$INSTALL_DIR is not in your PATH."
fi
echo "  Add this to ~/.bashrc to persist:"
echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""

# ── 7. Import base image into ~/.docksmith/ ───────────────────────────────────
info "Initialising ~/.docksmith/ ..."
mkdir -p "$DOCKSMITH_HOME"/{images,layers,cache}

info "Importing base image into Docksmith store..."
"$INSTALL_DIR/$BINARY_NAME" import "$BASE_IMAGE_TARBALL" "$BASE_IMAGE_NAME"
success "Base image imported as $BASE_IMAGE_NAME"

# ── 8. Verify ─────────────────────────────────────────────────────────────────
echo ""
echo "=============================================="
echo "  Setup complete!"
echo "=============================================="
echo ""
echo "Available images:"
"$INSTALL_DIR/$BINARY_NAME" images
echo ""
echo "Next steps:"
echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
echo "  cd sample-app"
echo "  docksmith build -t myapp:latest ."
echo "  docksmith run myapp:latest"
