package defs

/// Device identifiers used throughout the kernel.
const (
	D_CONSOLE int = 1 /// console device
	// UNIX domain sockets
	D_SUD     = 2         /// datagram socket device
	D_SUS     = 3         /// stream socket device
	D_DEVNULL = 4         /// /dev/null sink
	D_RAWDISK = 5         /// raw disk interface
	D_STAT    = 6         /// statistics device
	D_PROF    = 7         /// profiling device
	D_FIRST   = D_CONSOLE /// lowest device number
	D_LAST    = D_SUS     /// highest device number
)

/// Mkdev encodes a major and minor device number into a 64-bit identifier.
func Mkdev(_maj, _min int) uint {
	maj := uint(_maj)
	min := uint(_min)
	if min > 0xff {
		panic("bad minor")
	}
	m := maj<<8 | min
	return uint(m << 32)
}

/// Unmkdev returns the major and minor components of a device number.
func Unmkdev(d uint) (int, int) {
	return int(d >> 40), int(uint8(d >> 32))
}
