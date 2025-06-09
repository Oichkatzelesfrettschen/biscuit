#!/usr/bin/env bash
/**
 * @file
 * @brief Setup script for the Biscuit build environment.
 * @details
 *  - Installs system dependencies via APT.
 *  - Bootstraps a lightweight Go toolchain if none is present.
 *  - Builds the legacy Go 1.10.1 runtime, Biscuit kernel/userland, and docs.
 * @author Biscuit Team
 * @date   2025-06-09
 */

# Exit on error, undefined var, or pipe failure :contentReference[oaicite:5]{index=5}
set -euo pipefail

# Prepend standard system bin dirs (bootstrap Go takes precedence).
export PATH="/usr/local/go/bin:/usr/local/sbin:\
/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

## @var packages
#  @brief APT packages required by Biscuit (no duplicates).
packages=(
  aflplusplus           # Fuzzing engine
  bpfcc-tools           # BPF compiler collection
  clang                 # LLVM C/C++ compiler
  clang-format          # Code formatter
  coq                   # Proof assistant
  curl                  # HTTP download tool
  ctags                 # Code tagging
  cmake                 # Build system generator
  doxygen               # API doc generator
  gdb                   # Debugger
  git                   # Version control
  golang-go             # Go compiler (>=1.22)
  graphviz              # Graph visualizer
  htop                  # Process monitor
  jq                    # JSON processor
  lld                   # LLVM linker
  llvm                  # LLVM utilities
  ltrace                # Library call tracer
  net-tools             # Networking utilities
  nodejs                # JS runtime
  npm                   # JS package manager
  pkg-config            # Lib config helper
  python3               # Python interpreter
  python3-pip           # Python package installer
  python3-sphinx        # Sphinx docs
  python3-venv          # Virtual env support
  qemu-system-x86       # QEMU emulator
  shellcheck            # Shell script linter
  ssh                   # Secure shell client
  strace                # Syscall tracer
  systemtap             # Kernel tracing
  tcpdump               # Packet capture
  valgrind              # Memory debugger
  wget                  # HTTP download fallback
  linux-tools-generic   # Kernel perf events
  libpcap-dev           # libpcap headers
)

###############################################################################
# Logging utilities
###############################################################################

/**
 * @brief  Prefix all log messages.
 * @param  args  Message text
 */
log() {
  echo "[setup] $*"
}

/**
 * @brief  Log an error without exiting.
 * @param  args  Error message text
 */
log_error() {
  echo "[setup] ERROR: $*" >&2
}

###############################################################################
# Package check & install
###############################################################################

/**
 * @brief  Check if an APT package is installed.
 * @param  pkg  Package name
 * @return 0 if not installed, 1 if installed (dpkg -s) :contentReference[oaicite:6]{index=6}
 */
need_install() {
  dpkg -s "$1" >/dev/null 2>&1 || return 0
  return 1
}

# Required packages for the build.
packages=(
  qemu-system-x86   # QEMU emulator for running Biscuit
  qemu-utils        # QEMU disk utilities
  qemu-nox          # Headless QEMU binary
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
  doxygen           # Documentation generator
  python3-sphinx    # Sphinx documentation tool
  tmux              # Terminal multiplexer
  cloc              # Source code line counter
  nodejs            # Node runtime
  npm               # Node package manager
  curl              # Preferred tool for downloading Go bootstrap
  wget              # Fallback download tool
  coq               # Coq proof assistant
  tlaplus           # Tools for TLA+ specifications
)
/**
 * @brief  Install all missing APT packages.
 * @details Uses non-interactive, no-install-recommends for minimal installs :contentReference[oaicite:7]{index=7}
 */
install_packages() {
  local missing=()
  for pkg in "${packages[@]}"; do
    need_install "$pkg" && missing+=("$pkg")
  done

  if [ ${#missing[@]} -eq 0 ]; then
    log "All packages installed"
    return
  fi

  sudo apt-get update -qq || log_error "apt-get update failed"
  for pkg in "${missing[@]}"; do
    sudo apt-get install -y --no-install-recommends "$pkg" \
      && log "Installed $pkg" \
      || log_error "Failed to install $pkg"
  done
}

###############################################################################
# Go bootstrap: version detection, download, and extraction
###############################################################################

/**
 * @brief  Fetch the latest Go version string (e.g., "1.22.4").
 * @return Version without leading "go" (uses curl or wget) :contentReference[oaicite:8]{index=8}
 */
latest_go_version() {
  local endpoint="https://go.dev/VERSION?m=text"
  local ver
  if command -v curl >/dev/null; then
    ver=$(curl -fsSL "$endpoint")
  else
    ver=$(wget -qO- "$endpoint")
  fi
  echo "${ver#go}"
}

# Bootstrap parameters
GO_BOOTSTRAP_VERSION="$(latest_go_version)"
GO_BOOTSTRAP_DIR="$HOME/go-bootstrap"

/**
 * @brief  Download & extract Go to GO_BOOTSTRAP_DIR.
 * @details Falls back to wget if curl is missing :contentReference[oaicite:9]{index=9}
 */
download_go() {
  local url="https://go.dev/dl/go${GO_BOOTSTRAP_VERSION}.linux-amd64.tar.gz"
  local tarball="/tmp/go${GO_BOOTSTRAP_VERSION}.tar.gz"
  mkdir -p "$GO_BOOTSTRAP_DIR"

  if command -v curl >/dev/null; then
    curl -fsSL "$url" -o "$tarball" \
      || log_error "curl download failed"
  else
    wget -qO "$tarball" "$url" \
      || log_error "wget download failed"
  fi

  tar -xzf "$tarball" -C "$GO_BOOTSTRAP_DIR" --strip-components=1 \
    && log "Go bootstrap extracted" \
    || log_error "Extraction failed"

  export PATH="$GO_BOOTSTRAP_DIR/bin:$PATH"
}

###############################################################################
# Go tooling & project build
###############################################################################

/**
 * @brief  Install Go-based developer tools via `go install ...@latest`.
 * @see   Go Modules Reference :contentReference[oaicite:10]{index=10}
 */
install_go_tools() {
  log "Installing Go tools..."
  local tools=(
    golang.org/x/tools/cmd/goimports
    honnef.co/go/tools/cmd/staticcheck
    github.com/golangci/golangci-lint/cmd/golangci-lint
  )
  for mod in "${tools[@]}"; do
    go install "${mod}@latest" \
      && log "Installed ${mod##*/}" \
      || log_error "Failed to install $mod"
  done
}

/**
 * @brief  Download module dependencies for main & test.
 */
update_go_modules() {
  log "Downloading Go modules..."
  if go mod download all >/dev/null 2>&1 && \
     cd test && go mod download >/dev/null 2>&1; then
    log "Go modules downloaded"
  else
    log_error "Failed to download Go modules"
  fi
}

/**
 * @brief  Build the legacy Go runtime (forked 1.10.1).
 */
build_runtime() {
  log "Building Go runtime..."
  (cd src && ./make.bash >/dev/null 2>&1) \
    && log "Runtime build complete" \
    || log_error "Runtime build failed"
}

# Install Python and Node utilities used for testing and linting.
if command -v pip3 >/dev/null; then
  pip3 install --user --upgrade mypy flake8 pytest breathe sphinx-rtd-theme >/dev/null 2>&1 || true
fi
/**
 * @brief  Build Biscuit kernel & userland.
 */
build_biscuit() {
  log "Building Biscuit..."
  GOPATH="$(pwd)" make -C biscuit >/dev/null 2>&1 \
    && log "Biscuit build complete" \
    || log_error "Biscuit build failed"
}

/**
 * @brief  Generate documentation via Doxygen & Sphinx.
 */
build_docs() {
  log "Building docs..."
  doxygen docs/Doxyfile >/dev/null 2>&1 && \
  python3 -m sphinx -b html docs docs/_build/html >/dev/null 2>&1 \
    && log "Docs build complete" \
    || log_error "Docs build failed"
}

###############################################################################
# Main execution
###############################################################################
install_packages

if command -v go >/dev/null; then
  export GOROOT_BOOTSTRAP="$(go env GOROOT)"
else
  download_go
  export GOROOT_BOOTSTRAP="$GO_BOOTSTRAP_DIR"
fi

log "Using Go: $(go version)"
install_go_tools
update_go_modules
build_runtime
build_biscuit
build_docs
