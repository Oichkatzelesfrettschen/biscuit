#!/usr/bin/env bash
# Setup script for Biscuit build environment.
# Installs dependencies and configures Go bootstrap.
set -euo pipefail

# Install qemu and build tools if missing.
if ! command -v qemu-system-x86_64 >/dev/null; then
  sudo apt-get update
  sudo apt-get install -y qemu-system-x86 build-essential python3
fi

# Use the system Go for bootstrapping.
export GOROOT_BOOTSTRAP=$(go env GOROOT)

# Print Go version for verification.
go version
