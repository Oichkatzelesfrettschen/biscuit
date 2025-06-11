package pci

const BSIZE = 4096

// XXX delete and the disks that use it?
/// Idebuf_t describes an IDE request buffer.
type Idebuf_t struct {
	Disk  int
	Block int
	Data  *[BSIZE]uint8
}

/// Disk_i abstracts disk operations used by the PCI layer.
type Disk_i interface {
	Start(*Idebuf_t, bool)
	Complete([]uint8, bool)
	Intr() bool
	Int_clear()
}
