// Command chentry modifies the entry address of an ELF binary.
//
// The original implementation was written in C and used during the build
// process to update kernel images.  This Go version behaves identically.
package main

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
)

// usage prints a small help message and terminates the program.
func usage(me string) {
	fmt.Printf("%s <filename> <addr>\n\nChange the ELF entry point of <filename> to <addr>\n", me)
	os.Exit(1)
}

// chkELF validates the ELF file header to ensure we are modifying the correct
// type of binary.  It exits the program if any of the checks fail.
func chkELF(eh *elf.FileHeader) {
	// Verify the magic bytes at the start of the file.
	if eh.Ident[0] != 0x7f || string(eh.Ident[1:4]) != "ELF" {
		log.Fatal("not an elf")
	}
	// Only little-endian 64-bit x86 executables are supported.
	if eh.Ident[elf.EI_DATA] != elf.ELFDATA2LSB {
		log.Fatal("not little-endian?")
	}
	if eh.Type != elf.ET_EXEC {
		log.Fatal("not an executable elf")
	}
	if eh.Machine != elf.EM_X86_64 {
		log.Fatal("not a 64 bit elf")
	}
}

// main drives the entry point update.  It expects a filename and an address
// value on the command line and rewrites the ELF header accordingly.
func main() {
	if len(os.Args) != 3 {
		usage(os.Args[0])
	}
	fn := os.Args[1]
	addr, err := parseAddr(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	if addr>>32 != 0 {
		log.Fatal("entry is 64bit pointer; bootloader will perish")
	}
	f, err := os.OpenFile(fn, os.O_RDWR, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	ef, err := elf.NewFile(f)
	if err != nil {
		log.Fatal(err)
	}
	chkELF(&ef.FileHeader)

	fmt.Printf("using address 0x%x\n", addr)
	ef.FileHeader.Entry = addr

	if _, err := f.Seek(0, 0); err != nil {
		log.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, &ef.FileHeader); err != nil {
		log.Fatal(err)
	}
}

// parseAddr converts the supplied string into a uint64 address.  The syntax
// matches that of C's strtoul with a base of 0, allowing both decimal and
// hexadecimal numbers.
func parseAddr(s string) (uint64, error) {
	a, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid address %q", s)
	}
	return a, nil
}
