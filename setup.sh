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
packages=(qemu-system-x86 build-essential python3)

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

# Use the system Go for bootstrapping.
export GOROOT_BOOTSTRAP=$(go env GOROOT)

# Print Go version for verification.
go version
