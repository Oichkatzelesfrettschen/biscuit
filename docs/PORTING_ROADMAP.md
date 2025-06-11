# Kernel Porting Roadmap

This document outlines the detailed steps required to migrate the C source files under `biscuit/src/kernel/` and other C components to Go 1.24 while maintaining existing behaviour.

## Overview

Migrating the Biscuit kernel entirely to Go requires staged tasks. Each stage should compile on its own and be testable before proceeding. Use idiomatic Go while mirroring the original C logic.

## Tasks

1. **Inventory C Code**
   - Enumerate all `.c` files within `biscuit/src/kernel/` and subdirectories.
   - Identify supporting C code elsewhere in the repository.
   - Build a tracking spreadsheet mapping each function to its intended Go file.
2. **Prepare Build System**
   - Add a `go.mod` entry for new packages under `biscuit/kernelgo`.
   - Update `GNUMakefile` and scripts to recognise Go replacements alongside C sources.
3. **Bootstrap Go Environment**
   - Ensure Go 1.24.x toolchain from the repository builds successfully.
   - Create placeholder packages mirroring the existing kernel structure (e.g. `kernel/boot`, `kernel/sys`).
4. **Port Bootloader**
   - Translate `bootmain.c` and assembly stubs to Go using `cgo` only where required for inline assembly.
   - Validate boot sequence using QEMU.
5. **Port Core Kernel**
   - Incrementally port each subsystem:
     - Scheduler
     - Memory management
     - Interrupt handling
     - Filesystems
   - After each subsystem, replace the corresponding C object files in the build with the new Go packages.
6. **User Space Interfaces**
   - Reimplement syscalls in Go ensuring ABI compatibility.
   - Update libc (`litc`) bindings.
7. **Device Drivers**
   - Port AHCI, network, and other drivers module by module.
   - Retain hardware-specific headers as needed using `go:linkname` and `cgo` for low-level operations.
8. **Testing and Validation**
   - Write Go unit tests mirroring existing kernel test programs.
   - Use QEMU for integration tests at each stage.
9. **Cleanup**
   - Remove ported C files once their Go equivalents pass tests.
   - Keep assembly stubs minimal and documented.
10. **Continuous Integration**
    - Expand `setup.sh` or new CI scripts to build and test the Go kernel automatically.

## Milestones

- **M1** - Bootable kernel with mixed C and Go code.
- **M2** - Scheduler, memory management, and process model fully in Go.
- **M3** - Filesystem and driver layer converted.
- **M4** - Entire kernel builds from Go sources only.
- **M5** - Performance tuning and code clean-up.

## Long-Term Goals

- Evaluate cross-compilation for additional architectures.
- Investigate using TinyGo or other runtimes for boot components.
- Document new code thoroughly with GoDoc comments.

