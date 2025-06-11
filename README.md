# Biscuit research OS

Biscuit is a monolithic, POSIX-subset operating system kernel in Go for x86-64
CPUs. It was written to study the performance trade-offs of using a high-level
language with garbage collection to implement a kernel with a common style of
architecture. You can find research papers about Biscuit here:
https://pdos.csail.mit.edu/projects/biscuit.html

Biscuit has some important features for getting good application performance:
- Multicore
- Kernel-supported threads
- Journaled FS with concurrent, deferred, and group commit
- Virtual memory for copy-on-write and lazily mapped anonymous/file pages
- TCP/IP stack
- AHCI SATA disk driver
- Intel 10Gb NIC driver

Biscuit also includes a bootloader, a partial libc ("litc"), and some user
space programs, though we could have used GRUB or existing libc
implementations, like musl.

This repo is a fork of the Go repo (https://github.com/golang/go).  Nearly all
of Biscuit's code is in biscuit/.

## Install

The repository includes a complete Go **1.24.x** toolchain. The kernel builds entirely with this toolchain as defined in `go.mod`.

The `setup.sh` script installs a comprehensive FixIt Toolkit using APT,
pip, and npm packages, automatically searching for and installing any
`tmux`-related utilities. It refreshes the package cache and discovers Go
packages for building, debugging, testing, and fuzzing via `apt-cache`,
installing any found modules. The script then builds the kernel,
documentation, and tests automatically using the toolchain declared in
`go.mod`.

Biscuit used to build on Linux and OpenBSD, but probably only builds on Linux
currently. Clone the repository and launch it with QEMU:
```
$ git clone https://github.com/mit-pdos/biscuit.git
$ cd biscuit
$ make qemu CPUS=2
```

Biscuit should boot, then you can type a command:
```
# ls
```

## Troubleshooting

* You need `qemu-system-x86_64` and `python3` in your environment.  If your distribution does not name them that way, you have to fix the naming, path, etc.

* If the GOPATH environment variable doesn't contain biscuit/, the build will fail with something like:
```
src/ahci/ahci.go:8:8: cannot find package "container/list" in any of:
...
```

Either unset GOPATH or set it explicitly, for example (assuming that your working directory is where the `GNUMakefile` is):
```
$ GOPATH=$(pwd) make qemu CPUS=2
```

The project uses Go modules exclusively, so leaving `GOPATH` unset is usually
the safest option. When using editor tooling that relies on `GOPATH`, point it
at the repository root as above.

Run `go mod graph` to inspect module dependencies. The `misc/depgraph` helper
converts this output to Graphviz for easier visualization.

## Contributing

Please feel free to hack on Biscuit! We're happy to accept contributions.
