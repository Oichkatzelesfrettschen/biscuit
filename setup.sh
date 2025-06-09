#!/usr/bin/env bash
# Setup script for Biscuit build environment.
# Installs dependencies and configures Go bootstrap.
#
# Biscuit uses a forked Go 1.10.1 runtime. When no system Go is
# available this script downloads a small Go toolchain that is only
# used for bootstrapping the old runtime. The script fetches the latest
# stable release of Go so that the toolchain stays up to date.
PATH="/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
export PATH
set -euo pipefail

# Prefix all logs so output is easy to follow.
log() {
	echo "[setup] $*"
}

# Log errors but do not stop execution.
log_error() {
	echo "[setup] ERROR: $*" >&2
}

# Determine whether we need to install packages required for building Biscuit.
need_install() {
	# Check if a package is installed via dpkg.
	dpkg -s "$1" >/dev/null 2>&1 || return 0
	return 1
}

# Required packages for the build.
packages=(
	qemu-system-x86     # QEMU emulator for running Biscuit
	build-essential     # GCC and related build tools
	git                 # Version control
	gdb                 # Debugger
	clang               # Additional compiler for fuzzing or linting
	clang-format        # Code formatting tool
	lld                 # LLVM linker used by some build scripts
	llvm                # LLVM utilities such as objdump
	valgrind            # Memory debugging
	strace              # System call tracing
	ltrace              # Library call tracing
	cmake               # Build configuration tool
	python3             # Python required for some build scripts
	python3-pip         # Python package installer
	doxygen             # Documentation generator
	python3-sphinx      # Sphinx documentation tool
	nodejs              # Node runtime
	npm                 # Node package manager
	curl                # Preferred tool for downloading Go bootstrap
	wget                # Fallback download tool
	golang-go           # Go compiler
        # golang-go-tools and golangci-lint are unavailable in Ubuntu 24.04
        # A direct 'go install' step later provides similar tooling.
	net-tools           # Networking utilities
	tcpdump             # Packet capture
	graphviz            # Graph visualization for docs
	shellcheck          # Shell script analysis
	jq                  # JSON processing
	htop                # Resource monitoring
	python3-venv        # Python virtual environments
	libpcap-dev         # Packet capture headers
	systemtap           # SystemTap tracing
	linux-tools-generic # Kernel performance events
	pkg-config          # Build config helper
	bpfcc-tools         # BPF compiler collection
	libssl-dev          # Crypto development libraries
	coq                 # Coq proof assistant
)

# Determine the latest stable Go release version number. The go.dev
# service exposes this via the VERSION?m=text endpoint.
latest_go_version() {
        local version_url="https://go.dev/VERSION?m=text"
        local ver
        if command -v curl >/dev/null; then
                ver=$(curl -fsSL "$version_url")
        else
                ver=$(wget -qO- "$version_url")
        fi
        ver=${ver%%$'\n'*}
        echo "${ver#go}"
}

# Bootstrap Go version to download when no system Go is found.
GO_BOOTSTRAP_VERSION="$(latest_go_version)"

# Directory where the bootstrap Go will be installed.
GO_BOOTSTRAP_DIR="$HOME/go-bootstrap"

# Download and unpack the Go bootstrap toolchain into GO_BOOTSTRAP_DIR.
download_go() {
	local url="https://go.dev/dl/go${GO_BOOTSTRAP_VERSION}.linux-amd64.tar.gz"
	local tarball="/tmp/go${GO_BOOTSTRAP_VERSION}.tar.gz"

	mkdir -p "$GO_BOOTSTRAP_DIR"

	if command -v curl >/dev/null; then
		# Use curl if available.
		curl -fsSL "$url" -o "$tarball" >/dev/null 2>&1 ||
			log_error "Failed to download Go with curl"
	elif command -v wget >/dev/null; then
		# Fallback to wget.
		wget -qO "$tarball" "$url" >/dev/null 2>&1 ||
			log_error "Failed to download Go with wget"
	else
		log_error "curl or wget is required to download Go"
		return
	fi

	if tar -C "$GO_BOOTSTRAP_DIR" -xzf "$tarball" --strip-components=1 >/dev/null 2>&1; then
		log "Go bootstrap extracted"
	else
		log_error "Failed to extract Go bootstrap"
	fi
	export PATH="$GO_BOOTSTRAP_DIR/bin:$PATH" # Make 'go' command available.
}

# Gather missing packages and attempt installation. Failures are logged but
# do not abort the script.
install_packages() {
        # Install missing apt packages in a non-interactive manner.
        local missing=()
        for pkg in "${packages[@]}"; do
                need_install "$pkg" && missing+=("$pkg")
        done

        if [ ${#missing[@]} -eq 0 ]; then
                log "All packages already installed"
                return
        fi

        local APT_OPTS=("-y" "--no-install-recommends" "-o=Dpkg::Use-Pty=0")
        export DEBIAN_FRONTEND=noninteractive

        if ! apt-get update >/dev/null 2>&1; then
                log_error "apt-get update failed"
        fi

        for pkg in "${missing[@]}"; do
                if apt-get install "${APT_OPTS[@]}" "$pkg" >/dev/null 2>&1; then
                        log "Installed $pkg"
                else
                        log_error "Failed to install $pkg"
                fi
        done
}

# Begin setup sequence.
install_packages

# Install Python and Node utilities used for testing and linting.
if command -v pip3 >/dev/null; then
	pip3 install --user --upgrade mypy flake8 pylint bandit pytest pytest-cov breathe sphinx-rtd-theme black >/dev/null 2>&1 || true
fi

if command -v npm >/dev/null; then
	npm install --global eslint prettier typescript npm-check >/dev/null 2>&1 || true
fi

# Run any custom setup scripts provided by the repository.
if [ -d "scripts/setup.d" ]; then
	for script in scripts/setup.d/*; do
		[ -f "$script" ] && bash "$script"
	done
fi

# Set up the Go bootstrap toolchain.
if command -v go >/dev/null; then
	# Use existing Go installation for bootstrapping.
	GOROOT_BOOTSTRAP="$(go env GOROOT)"
	export GOROOT_BOOTSTRAP
else
	# Download a lightweight bootstrap toolchain.
	download_go
	export GOROOT_BOOTSTRAP="$GO_BOOTSTRAP_DIR"
fi

# Show which Go compiler will be used.
install_go_tools() {
        log "Fetching additional Go tools..."
        local modules=(
                golang.org/x/tools/cmd/goimports
                honnef.co/go/tools/cmd/staticcheck
                github.com/golangci/golangci-lint/cmd/golangci-lint
        )
        for mod in "${modules[@]}"; do
                if go install "${mod}@latest" >/dev/null 2>&1; then
                        log "Installed ${mod##*/}"
                else
                        log_error "Failed to install $mod"
                fi
        done
}

# Fetch Go modules for the project and its test suite.
update_go_modules() {
        log "Downloading Go modules..."
        if go mod download all >/dev/null 2>&1 && (
                cd test && go mod download >/dev/null 2>&1
        ); then
                log "Go modules installed"
        else
                log_error "Failed to download Go modules"
        fi
}

log "Using $(go version)"
install_go_tools
update_go_modules

# Build Biscuit's modified Go runtime. The build uses the bootstrapped Go
# compiler configured above and must succeed before building the kernel.
build_runtime() {
	log "Building Biscuit Go runtime..."
	if (cd src && ./make.bash >/dev/null 2>&1); then
		log "Runtime build finished"
	else
		log_error "Runtime build failed"
	fi
}

# Build the Biscuit kernel and userland using the freshly built runtime.
build_biscuit() {
	log "Building Biscuit kernel and user programs..."
	if GOPATH="$(pwd)" make -C biscuit >/dev/null 2>&1; then
		log "Biscuit build finished"
	else
		log_error "Biscuit build failed"
	fi
}

build_runtime
build_biscuit
