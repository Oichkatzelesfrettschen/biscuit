#!/usr/bin/env bash
# Setup script for Biscuit build environment.
# Installs dependencies and configures Go bootstrap.
set -euo pipefail

# Determine whether we need to install packages required for building Biscuit.
need_install() {
  # Check if a package is installed via dpkg.
  dpkg -s "$1" >/dev/null 2>&1 || return 0
  return 1
}

# Required packages for the build.
packages=(
  qemu-system-x86   # QEMU emulator for running Biscuit
  build-essential   # GCC and related build tools
  git               # Version control
  gdb               # Debugger
  clang             # Additional compiler for fuzzing or linting
  clang-format      # Code formatting tool
  lld               # LLVM linker used by some build scripts
  llvm              # LLVM utilities such as objdump
  valgrind          # Memory debugging
  strace            # System call tracing
  ltrace            # Library call tracing
  cmake             # Build configuration tool
  python3           # Python required for some build scripts
  python3-pip       # Python package installer
  nodejs            # Node runtime
  npm               # Node package manager
  curl              # Preferred tool for downloading Go bootstrap
  wget              # Fallback download tool
  coq               # Coq proof assistant
  tlaplus           # Tools for TLA+ specifications
)

# Bootstrap Go version to download when no system Go is found.
GO_BOOTSTRAP_VERSION=1.20.7

# Directory where the bootstrap Go will be installed.
GO_BOOTSTRAP_DIR="$HOME/go-bootstrap"

# Download and unpack the Go bootstrap toolchain into GO_BOOTSTRAP_DIR.
download_go() {
  local url="https://dl.google.com/go/go${GO_BOOTSTRAP_VERSION}.linux-amd64.tar.gz"
  local tarball="/tmp/go${GO_BOOTSTRAP_VERSION}.tar.gz"

  mkdir -p "$GO_BOOTSTRAP_DIR"

  if command -v curl >/dev/null; then
    # Use curl if available.
    curl -fsSL "$url" -o "$tarball"
  elif command -v wget >/dev/null; then
    # Fallback to wget.
    wget -qO "$tarball" "$url"
  else
    echo "Error: curl or wget is required to download Go" >&2
    return 1
  fi

  tar -C "$GO_BOOTSTRAP_DIR" -xzf "$tarball" --strip-components=1
}

# Gather all packages that are missing.
missing=()
for pkg in "${packages[@]}"; do
  need_install "$pkg" && missing+=("$pkg")
done

# Install missing packages, if any.
if [ ${#missing[@]} -ne 0 ]; then
  sudo apt-get update
  sudo apt-get install -y "${missing[@]}"
fi

# Install Python and Node utilities used for testing and linting.
if command -v pip3 >/dev/null; then
  pip3 install --user --upgrade mypy flake8 pytest >/dev/null 2>&1 || true
fi

if command -v npm >/dev/null; then
  npm install --global eslint prettier >/dev/null 2>&1 || true
fi

# Run any custom setup scripts provided by the repository.
if [ -d "scripts/setup.d" ]; then
  for script in scripts/setup.d/*; do
    [ -f "$script" ] && bash "$script"
  done
fi

# Set up the Go bootstrap toolchain.
if command -v go >/dev/null; then
  # If Go exists, use its GOROOT for bootstrapping.
  export GOROOT_BOOTSTRAP="$(go env GOROOT)"
else
  # Otherwise download a lightweight bootstrap toolchain.
  download_go
  export GOROOT_BOOTSTRAP="$GO_BOOTSTRAP_DIR"
fi

# Print Go version for verification if available.
if command -v go >/dev/null; then
  go version
else
  "$GOROOT_BOOTSTRAP/bin/go" version
fi
