#!/usr/bin/env bash
#
# @file
# @brief Setup script for the Biscuit build environment.
# @details
# - Installs system dependencies via the detected package manager.
# - Bootstraps a lightweight Go toolchain if none is present.
# - Builds the legacy Go 1.10.1 runtime, Biscuit kernel/userland, and docs.
# @author Biscuit Team
# @date   2025-06-09
#

# Exit on error, undefined var, or pipe failure :contentReference[oaicite:5]{index=5}
set -euo pipefail

# Prepend standard system bin dirs (bootstrap Go takes precedence).
export PATH="/usr/local/go/bin:/usr/local/sbin:\
/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

# Use GOPATH mode to build legacy components
# GOPATH and GO111MODULE were previously exported to emulate
# Biscuit's legacy build environment. Modern Go no longer
# requires these overrides, so they are disabled.
# export GOPATH="$(pwd)/biscuit"
# export GO111MODULE=off

## @var packages
#  @brief APT packages required by Biscuit (no duplicates).
packages=(
	afl++                    # Fuzzing engine
	bpfcc-tools              # BPF compiler collection
	build-essential          # GCC and related build tools
	clang                    # LLVM C/C++ compiler
	clang-format             # Code formatter
	coq                      # Proof assistant
	curl                     # HTTP download tool
	universal-ctags          # Code tagging
	cmake                    # Build system generator
	doxygen                  # API doc generator
	doxygen-doc              # HTML manuals
	doxygen-gui              # GUI frontend
	doxygen-latex            # LaTeX/PDF output
	gdb                      # Debugger
	git                      # Version control
	golang-go                # Go compiler (>=1.22)
	graphviz                 # Graph visualizer
	graphviz-doc             # Graphviz manuals
	libgraphviz-dev          # Development libraries
	linux-tools-generic      # Kernel perf events
	libpcap-dev              # libpcap headers
	htop                     # Process monitor
	jq                       # JSON processor
	tree                     # Directory tree visualizer
	lld                      # LLVM linker
	llvm                     # LLVM utilities
	ltrace                   # Library call tracer
	net-tools                # Networking utilities
	nodejs                   # JS runtime
	npm                      # JS package manager
	pkg-config               # Lib config helper
	python3                  # Python interpreter
	python3-pip              # Python package installer
	python3-sphinx           # Sphinx docs
	sphinx-doc               # Extra HTML manuals
	python3-sphinx-rtd-theme # ReadTheDocs theme
	python3-venv             # Virtual env support
	qemu-system-x86          # QEMU emulator
	qemu-utils               # QEMU disk utilities
	qemu-nox                 # Headless QEMU binary
	shellcheck               # Shell script linter
	ssh                      # Secure shell client
	strace                   # Syscall tracer
	systemtap                # Kernel tracing
	tcpdump                  # Packet capture
	tmux                     # Terminal multiplexer
	cloc                     # Source code line counter
	valgrind                 # Memory debugger
	wget                     # HTTP download fallback
	tlaplus                  # Tools for TLA+ specifications
)

## @var pip_packages
#  @brief Python packages installed via pip.
pip_packages=(
	mypy             # Static typing
	flake8           # Linting
	black            # Code formatter
	isort            # Import sorting
	pytest           # Test runner
	coverage         # Test coverage reports
	breathe          # Doxygen bridge for Sphinx
	sphinx-rtd-theme # Read the Docs theme
)

## @var npm_packages
#  @brief Node packages installed globally with npm.
npm_packages=(
	eslint     # JS linter
	prettier   # Code formatter
	typescript # TypeScript compiler
)

###############################################################################
# Package manager detection
###############################################################################

# @brief Detect the active package manager.
# @details Sets PM_UPDATE, PM_INSTALL, and PM_QUERY for later use.
detect_package_manager() {
	if command -v apt-get >/dev/null; then
		PM_UPDATE='sudo apt-get update -qq'
		PM_INSTALL='sudo apt-get install -y --no-install-recommends'
		PM_QUERY='dpkg -s'
	elif command -v dnf >/dev/null; then
		PM_UPDATE='sudo dnf -y check-update'
		PM_INSTALL='sudo dnf -y install'
		PM_QUERY='rpm -q'
	elif command -v yum >/dev/null; then
		PM_UPDATE='sudo yum -y check-update'
		PM_INSTALL='sudo yum -y install'
		PM_QUERY='rpm -q'
	elif command -v pacman >/dev/null; then
		PM_UPDATE='sudo pacman -Sy --noconfirm'
		PM_INSTALL='sudo pacman -S --noconfirm'
		PM_QUERY='pacman -Q'
	elif command -v brew >/dev/null; then
		PM_UPDATE='brew update'
		PM_INSTALL='brew install'
		PM_QUERY='brew list'
	else
		echo "[setup] ERROR: No supported package manager found" >&2
		exit 1
	fi
}

# @brief Run the package database update.
pkg_update() {
	eval "$PM_UPDATE"
}

# @brief Install packages via the detected manager.
# @param pkg Package names
pkg_install() {
	eval "$PM_INSTALL $*"
}

###############################################################################
# Logging utilities
###############################################################################

#
# @brief  Prefix all log messages.
# @param  args  Message text
# /
log() {
	echo "[setup] $*"
}

#
# @brief  Log an error without exiting.
# @param  args  Error message text
# /
log_error() {
	echo "[setup] ERROR: $*" >&2
}

###############################################################################
# Package check & install
###############################################################################

#
# @brief  Check if a package is installed.
# @param  pkg  Package name
# @return 0 if not installed, 1 if installed
need_install() {
	if $PM_QUERY "$1" >/dev/null 2>&1; then
		return 1
	fi
	return 0
}

#
# @brief  Install all missing APT packages.
# @details Uses non-interactive, no-install-recommends for minimal installs :contentReference[oaicite:7]{index=7}
install_packages() {
	detect_package_manager
	local missing=()
	for pkg in "${packages[@]}"; do
		need_install "$pkg" && missing+=("$pkg")
	done

	if [ ${#missing[@]} -eq 0 ]; then
		log "All packages installed"
		return
	fi

	pkg_update || log_error "package database update failed"
	for pkg in "${missing[@]}"; do
		pkg_install "$pkg" && log "Installed $pkg" || log_error "Failed to install $pkg"
	done
}

##
# @brief  Install Python packages with pip.
# @details Ensures the packages in @ref pip_packages are installed for the
#          current user.
install_pip_packages() {
	if ! command -v pip3 >/dev/null; then
		log "pip3 not found; skipping Python packages"
		return
	fi
	log "Installing Python packages..."
	pip3 install --user --upgrade "${pip_packages[@]}" >/dev/null 2>&1 &&
		log "Python packages installed" ||
		log_error "Failed to install Python packages"
}

##
# @brief  Install Node packages globally via npm.
# @details Ensures @ref npm_packages are available for tooling.
install_npm_packages() {
	if ! command -v npm >/dev/null; then
		log "npm not found; skipping Node packages"
		return
	fi
	log "Installing Node packages..."
	npm install -g "${npm_packages[@]}" >/dev/null 2>&1 &&
		log "Node packages installed" ||
		log_error "Failed to install Node packages"
}

##
# @brief Install all tmux-related packages available via apt.
# @details Searches apt-cache for packages prefixed with "tmux" and installs
#          them using @ref pkg_install.
install_tmux_packages() {
	if ! command -v apt-cache >/dev/null; then
		log "apt-cache not available; skipping tmux extras"
		return
	fi
	local tmux_pkgs
	tmux_pkgs=$(apt-cache search '^tmux' | awk '{print $1}')
	local missing=()
	for pkg in $tmux_pkgs; do
		need_install "$pkg" && missing+=("$pkg")
	done
	[ ${#missing[@]} -eq 0 ] && {
		log "All tmux packages installed"
		return
	}
	for pkg in "${missing[@]}"; do
		pkg_install "$pkg" && log "Installed $pkg" || log_error "Failed to install $pkg"
	done
}

##
# @brief Install extra Go packages discovered via apt-cache.
# @details Updates apt cache and installs packages with build, debug,
#          test, or fuzz keywords in their name.
install_go_extras() {
	detect_package_manager
	if ! command -v apt-cache >/dev/null; then
		need_install apt && pkg_install apt || return
	fi
	pkg_update
	log "Searching for Go build/debug/test/fuzz packages..."
	local go_pkgs
	go_pkgs=$(apt-cache search 'go' | grep -Ei '(golang|go-).*\<(build|debug|test|fuzz)\>' | awk '{print $1}')
	local missing=()
	for pkg in $go_pkgs; do
		need_install "$pkg" && missing+=("$pkg")
	done
	[ ${#missing[@]} -eq 0 ] && {
		log "All Go extras installed"
		return
	}
	for pkg in "${missing[@]}"; do
		pkg_install "$pkg" && log "Installed $pkg" || log_error "Failed to install $pkg"
	done
}

###############################################################################
# Go bootstrap: version detection, download, and extraction
###############################################################################

#
# @brief  Fetch the latest Go version string (e.g., "1.22.4").
# @return Version without leading "go" (uses curl or wget) :contentReference[oaicite:8]{index=8}
# /
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

#
# @brief  Download & extract Go to GO_BOOTSTRAP_DIR.
# @details Falls back to wget if curl is missing :contentReference[oaicite:9]{index=9}
# /
download_go() {
	local url="https://go.dev/dl/go${GO_BOOTSTRAP_VERSION}.linux-amd64.tar.gz"
	local tarball="/tmp/go${GO_BOOTSTRAP_VERSION}.tar.gz"
	mkdir -p "$GO_BOOTSTRAP_DIR"

	if command -v curl >/dev/null; then
		curl -fsSL "$url" -o "$tarball" ||
			log_error "curl download failed"
	else
		wget -qO "$tarball" "$url" ||
			log_error "wget download failed"
	fi

	tar -xzf "$tarball" -C "$GO_BOOTSTRAP_DIR" --strip-components=1 &&
		log "Go bootstrap extracted" ||
		log_error "Extraction failed"

	export PATH="$GO_BOOTSTRAP_DIR/bin:$PATH"
}

###############################################################################
# Go tooling & project build
###############################################################################

#
# @brief  Install Go-based developer tools via `go install ...@latest`.
# @see   Go Modules Reference :contentReference[oaicite:10]{index=10}
# /
install_go_tools() {
	log "Installing Go tools..."
	local tools=(
		golang.org/x/tools/cmd/goimports
		honnef.co/go/tools/cmd/staticcheck
		github.com/golangci/golangci-lint/cmd/golangci-lint
	)
	for mod in "${tools[@]}"; do
		go install "${mod}@latest" &&
			log "Installed ${mod##*/}" ||
			log_error "Failed to install $mod"
	done
}

#
# @brief  Download module dependencies for main & test.
# /
update_go_modules() {
	log "Downloading Go modules..."
	if go mod download all >/dev/null 2>&1 &&
		cd test && go mod download >/dev/null 2>&1; then
		log "Go modules downloaded"
	else
		log_error "Failed to download Go modules"
	fi
}

#
# @brief  Build the legacy Go runtime (forked 1.10.1).
# /
build_runtime() {
	log "Building Go runtime..."
	(cd src && ./make.bash >/dev/null 2>&1) &&
		log "Runtime build complete" ||
		log_error "Runtime build failed"
}

##
# @brief Build Biscuit kernel & userland.
build_biscuit() {
	log "Building Biscuit..."
	GOPATH="$(pwd)/biscuit" GO111MODULE=off make -C biscuit >/dev/null 2>&1 &&
		log "Biscuit build complete" ||
		log_error "Biscuit build failed"
}

#
# @brief  Generate documentation via Doxygen & Sphinx.
# /
build_docs() {
	log "Building docs..."
	doxygen docs/Doxyfile >/dev/null 2>&1 &&
		python3 -m sphinx -b html docs docs/_build/html >/dev/null 2>&1 &&
		log "Docs build complete" ||
		log_error "Docs build failed"
}

##
# @brief Run Go tests across all modules.
# @details Executes `go test -v ./...` and logs failures.
run_tests() {
	log "Running Go tests..."
	if go test -v ./...; then
		log "Tests passed"
	else
		log_error "Tests failed"
	fi
}

###############################################################################
# Main execution
###############################################################################
install_packages
install_pip_packages
install_npm_packages
install_tmux_packages
install_go_extras

if command -v go >/dev/null; then
	export GOROOT_BOOTSTRAP="$(go env GOROOT)"
else
	download_go
	export GOROOT_BOOTSTRAP="$GO_BOOTSTRAP_DIR"
fi

log "Using Go: $(go version)"
install_go_tools
update_go_modules
# build_runtime is disabled in the modern setup. The legacy
# Go 1.10 runtime is not compiled by default.
# build_runtime
# build_biscuit
build_docs
run_tests
