package stat

import "unsafe"

/// Stat_t mirrors a file's stat information.
type Stat_t struct {
	_dev    uint
	_ino    uint
	_mode   uint
	_size   uint
	_rdev   uint
	_uid    uint
	_blocks uint
	_m_sec  uint
	_m_nsec uint
}

/// Wdev stores the device ID.
func (st *Stat_t) Wdev(v uint) {
	st._dev = v
}

/// Wino stores the inode number.
func (st *Stat_t) Wino(v uint) {
	st._ino = v
}

/// Wmode records the file mode.
func (st *Stat_t) Wmode(v uint) {
	st._mode = v
}

/// Wsize records the file size.
func (st *Stat_t) Wsize(v uint) {
	st._size = v
}

/// Wrdev stores the rdev field.
func (st *Stat_t) Wrdev(v uint) {
	st._rdev = v
}

/// Mode returns the stored mode value.
func (st *Stat_t) Mode() uint {
	return st._mode
}

/// Size returns the stored size.
func (st *Stat_t) Size() uint {
	return st._size
}

/// Rdev returns the stored rdev.
func (st *Stat_t) Rdev() uint {
	return st._rdev
}

/// Rino returns the stored inode number.
func (st *Stat_t) Rino() uint {
	return st._ino
}

/// Bytes exposes the raw bytes of the structure.
func (st *Stat_t) Bytes() []uint8 {
	const sz = unsafe.Sizeof(*st)
	sl := (*[sz]uint8)(unsafe.Pointer(&st._dev))
	return sl[:]
}
