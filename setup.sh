#!/usr/bin/env bash
# Setup script for Biscuit build environment.
# Installs dependencies and configures Go bootstrap.
#
# Biscuit uses a forked Go 1.10.1 runtime. When no system Go is
# available this script downloads a small Go toolchain that is only
# used for bootstrapping the old runtime. The script fetches the latest
# stable release of Go so that the toolchain stays up to date.
set -uo pipefail

# Prefix all logs so output is easy to follow.
##
# log - Print a setup-prefixed log message.
#
# Parameters:
#   $*: Message to log.
# Returns:
#   None
##
log() {
	echo "[setup] $*"
}

# Log errors but do not stop execution.
##
# log_error - Print an error message without aborting.
#
# Parameters:
#   $*: Error message to log.
# Returns:
#   None
##
log_error() {
	echo "[setup] ERROR: $*" >&2
}

# Determine whether we need to install packages required for building Biscuit.
##
# need_install - Determine whether a package is missing.
#
# Parameters:
#   $1: Package name to check.
# Returns:
#   0 if package needs to be installed, 1 otherwise.
##
need_install() {
	dpkg -s "$1" >/dev/null 2>&1 || return 0
	return 1
}

# Required packages for the build.
packages=(
	qemu-system-x86 # QEMU emulator for running Biscuit
	build-essential # GCC and related build tools
	git             # Version control
	gdb             # Debugger
	clang           # Additional compiler for fuzzing or linting
	clang-format    # Code formatting tool
	lld             # LLVM linker used by some build scripts
	llvm            # LLVM utilities such as objdump
	valgrind        # Memory debugging
	strace          # System call tracing
	ltrace          # Library call tracing
	cmake           # Build configuration tool
	python3         # Python required for some build scripts
	python3-pip     # Python package installer
	doxygen         # Documentation generator
	python3-sphinx  # Sphinx documentation tool
	nodejs          # Node runtime
	npm             # Node package manager
	curl            # Preferred tool for downloading Go bootstrap
	wget            # Fallback download tool
	coq             # Coq proof assistant
	cloc            # Code line counter for statistics
	qemu-utils      # Additional QEMU utilities
	tmux            # Terminal multiplexer
)

# Determine the latest stable Go release version number. The go.dev
# service exposes this via the VERSION?m=text endpoint.
##
# latest_go_version - Retrieve the latest stable Go version.
#
# Globals:
#   None
# Returns:
#   Prints version string to stdout.
##
latest_go_version() {
	local version_url="https://go.dev/VERSION?m=text"
	if command -v curl >/dev/null; then
		curl -fsSL "$version_url"
	else
		wget -qO- "$version_url"
	fi | sed 's/^go//'
}

# Bootstrap Go version to download when no system Go is found.
GO_BOOTSTRAP_VERSION="$(latest_go_version)"

# Directory where the bootstrap Go will be installed.
GO_BOOTSTRAP_DIR="$HOME/go-bootstrap"

# Download and unpack the Go bootstrap toolchain into GO_BOOTSTRAP_DIR.
##
# download_go - Fetch and extract a small Go toolchain for bootstrapping.
#
# Globals:
#   GO_BOOTSTRAP_VERSION
#   GO_BOOTSTRAP_DIR
# Returns:
#   None
##
download_go() {
	local url="https://go.dev/dl/go${GO_BOOTSTRAP_VERSION}.linux-amd64.tar.gz"
	local tarball="/tmp/go${GO_BOOTSTRAP_VERSION}.tar.gz"

	mkdir -p "$GO_BOOTSTRAP_DIR"

	if command -v curl >/dev/null; then
		curl -fsSL "$url" -o "$tarball" >/dev/null 2>&1 ||
			log_error "Failed to download Go with curl"
	elif command -v wget >/dev/null; then
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
	export PATH="$GO_BOOTSTRAP_DIR/bin:$PATH"
}

# Gather missing packages and attempt installation. Failures are logged but
# do not abort the script.
##
# install_packages - Install required build dependencies.
#
# Globals:
#   packages
# Returns:
#   None
##
install_packages() {
	local missing to_install
	missing=()
	for pkg in "${packages[@]}"; do
		need_install "$pkg" && missing+=("$pkg")
	done

	if [ ${#missing[@]} -eq 0 ]; then
		log "All packages already installed"
		return
	fi

	sudo apt-get update >/dev/null 2>&1 || log_error "apt-get update failed"

	to_install=()
	for pkg in "${missing[@]}"; do
		if apt-cache show "$pkg" >/dev/null 2>&1; then
			to_install+=("$pkg")
		else
			log_error "Package $pkg not found in APT repositories"
		fi
	done

	if [ ${#to_install[@]} -gt 0 ]; then
		if sudo sh -c "apt-get install -y ${to_install[*]} >/tmp/apt-install.log 2>&1"; then
			log "Installed packages: ${to_install[*]}"
		else
			log_error "Failed to install some packages"
			grep -i "unable to locate" /tmp/apt-install.log >&2 || true
		fi
	fi
}

##
# install_tlaplus - Install the latest TLA+ toolbox if missing.
#
# Globals:
#   None
# Returns:
#   None
##
install_tlaplus() {
	if command -v tlc >/dev/null; then
		log "TLA+ already installed"
		return
	fi

	local tmpdir=/tmp/tlaplus
	mkdir -p "$tmpdir"
	local deb_url
	deb_url=$(curl -fsSL https://api.github.com/repos/tlaplus/tlaplus/releases/latest |
		grep -m1 'TLAToolbox-.*\.deb' |
		cut -d '"' -f4)

	if [ -z "$deb_url" ]; then
		log_error "Unable to locate TLA+ release"
		return
	fi

	if curl -L "$deb_url" -o "$tmpdir/tlaplus.deb"; then
		if sudo dpkg -i "$tmpdir/tlaplus.deb" >/dev/null 2>&1; then
			log "Installed TLA+"
		else
			log_error "Failed to install TLA+"
		fi
	else
		log_error "Download of TLA+ failed"
	fi
}

# Begin setup sequence.
install_packages
install_tlaplus

# Install Python and Node utilities used for testing and linting.
if command -v pip3 >/dev/null; then
	pip3 install --user --upgrade mypy flake8 pytest >/dev/null 2>&1 || true
	# Install Breathe for Sphinx-Doxygen integration
	pip3 install --user breathe >/dev/null 2>&1 || true
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
	# Use existing Go installation for bootstrapping.
	GOROOT_BOOTSTRAP="$(go env GOROOT)"
	export GOROOT_BOOTSTRAP
else
	# Download a lightweight bootstrap toolchain.
	download_go
	export GOROOT_BOOTSTRAP="$GO_BOOTSTRAP_DIR"
fi

# Show which Go compiler will be used.
log "Using $(go version)"

# Build Biscuit's modified Go runtime. The build uses the bootstrapped Go
# compiler configured above and must succeed before building the kernel.
##
# build_runtime - Compile Biscuit's forked Go runtime.
#
# Globals:
#   None
# Returns:
#   None
##
build_runtime() {
	log "Building Biscuit Go runtime..."
	if (cd src && ./make.bash >/dev/null 2>&1); then
		log "Runtime build finished"
	else
		log_error "Runtime build failed"
	fi
}

# Build the Biscuit kernel and userland using the freshly built runtime.
##
# build_biscuit - Compile the kernel and userland using the runtime.
#
# Globals:
#   None
# Returns:
#   None
##
build_biscuit() {
	log "Building Biscuit kernel and user programs..."
	if GOPATH="$(pwd)" make -C biscuit >/dev/null 2>&1; then
		log "Biscuit build finished"
	else
		log_error "Biscuit build failed"
	fi
}

##
# run_cloc - Summarize repository statistics using cloc.
#
# Globals:
#   None
# Returns:
#   None
##
run_cloc() {
	log "Collecting source statistics with cloc..."
	if cloc . >/dev/null 2>&1; then
		log "cloc statistics generated"
	else
		log_error "cloc failed"
	fi
}

# Execute the build steps in sequence.
build_runtime
build_biscuit
run_cloc
