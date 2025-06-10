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

The root of the repository contains the Go **1.10.1** tools and runtime.
Several Biscuit components modify the runtime directly, primarily in
`src/runtime/os_linux.go`. These changes rely on internal structures from
Go 1.10 and have not been ported to newer releases. As a result the kernel
must be built with Go 1.10.1 instead of a modern Go toolchain.

To compile the old runtime using a recent system Go, the `setup.sh` script
downloads the latest Go release when no local Go installation is found. This
bootstrap compiler only builds the Biscuit runtime and is not used to build
kernel code.  Running `./setup.sh` installs a comprehensive FixIt Toolkit using
APT, pip, and npm packages, automatically searching for and installing any
`tmux`-related utilities. It also refreshes the package cache and discovers Go
packages for building, debugging, testing, and fuzzing via `apt-cache`,
installing any found modules. The script then builds the runtime, kernel,
documentation, and tests automatically.

Biscuit used to build on Linux and OpenBSD, but probably only builds on Linux
currently. You must build Biscuit's modified Go runtime before building
Biscuit:
```
$ git clone https://github.com/mit-pdos/biscuit.git
$ cd biscuit/src
$ ./make.bash
```

then go to Biscuit's main part and launch it:
```
$ cd ../biscuit
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

## Contributing

Please feel free to hack on Biscuit! We're happy to accept contributions.
