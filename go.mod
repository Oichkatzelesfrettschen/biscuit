module biscuit

go 1.24.0

toolchain go1.24.3

require (
	github.com/google/pprof v0.0.0-20250607225305-033d6d78b36a
	golang.org/x/arch v0.18.0
	golang.org/x/text v0.26.0
	golang.org/x/tools v0.34.0
	golang.org/x/tools/go/pointer v0.1.0-deprecated
)

require (
	github.com/ianlancetaylor/demangle v0.0.0-20250417193237-f615e6bd150b // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
)

// Map old-style import paths to their local module directories.
// These replacements allow building the project in module mode.
replace accnt => ./biscuit/src/accnt

replace ahci => ./biscuit/src/ahci

replace apic => ./biscuit/src/apic

replace bnet => ./biscuit/src/bnet

replace bounds => ./biscuit/src/bounds

replace bpath => ./biscuit/src/bpath

replace caller => ./biscuit/src/caller

replace circbuf => ./biscuit/src/circbuf

replace defs => ./biscuit/src/defs

replace fd => ./biscuit/src/fd

replace fdops => ./biscuit/src/fdops

replace fs => ./biscuit/src/fs

replace hashtable => ./biscuit/src/hashtable

replace inet => ./biscuit/src/inet

replace ixgbe => ./biscuit/src/ixgbe

replace kernel => ./biscuit/src/kernel

replace limits => ./biscuit/src/limits

replace mem => ./biscuit/src/mem

replace msi => ./biscuit/src/msi

replace mkfs => ./biscuit/src/mkfs

replace pci => ./biscuit/src/pci

replace proc => ./biscuit/src/proc

replace res => ./biscuit/src/res

replace stat => ./biscuit/src/stat

replace stats => ./biscuit/src/stats

replace tinfo => ./biscuit/src/tinfo

replace ufs => ./biscuit/src/ufs

replace unet => ./biscuit/src/unet

replace ustr => ./biscuit/src/ustr

replace util => ./biscuit/src/util

replace vm => ./biscuit/src/vm
