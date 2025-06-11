package fs

import "mem"

// / Superblock_t represents the on-disk super block of a filesystem.
type Superblock_t struct {
	Data *mem.Bytepg_t
}

// / Loglen returns the length of the on-disk log in blocks.
func (sb *Superblock_t) Loglen() int {
	return fieldr(sb.Data, 0)
}

// / Iorphanblock returns the starting block of the orphan inode map.
func (sb *Superblock_t) Iorphanblock() int {
	return fieldr(sb.Data, 1)
}

// / Iorphanlen returns the length of the orphan inode map.
func (sb *Superblock_t) Iorphanlen() int {
	return fieldr(sb.Data, 2)
}

// / Imaplen returns the length of the inode bitmap.
func (sb *Superblock_t) Imaplen() int {
	return fieldr(sb.Data, 3)
}

// / Freeblock gives the starting block of the free block map.
func (sb *Superblock_t) Freeblock() int {
	return fieldr(sb.Data, 4)
}

// / Freeblocklen returns the length of the free block map.
func (sb *Superblock_t) Freeblocklen() int {
	return fieldr(sb.Data, 5)
}

// / Inodelen reports the number of blocks containing inodes.
func (sb *Superblock_t) Inodelen() int {
	return fieldr(sb.Data, 6)
}

// / Lastblock returns the address of the last block on the device.
func (sb *Superblock_t) Lastblock() int {
	return fieldr(sb.Data, 7)
}

// writing

// / SetLoglen updates the log length field.
func (sb *Superblock_t) SetLoglen(ll int) {
	fieldw(sb.Data, 0, ll)
}

// / SetIorphanblock records the starting block of the orphan map.
func (sb *Superblock_t) SetIorphanblock(n int) {
	fieldw(sb.Data, 1, n)
}

// / SetIorphanlen writes the length of the orphan map.
func (sb *Superblock_t) SetIorphanlen(n int) {
	fieldw(sb.Data, 2, n)
}

// / SetImaplen writes the length of the inode bitmap.
func (sb *Superblock_t) SetImaplen(n int) {
	fieldw(sb.Data, 3, n)
}

// / SetFreeblock stores the start block of the free block bitmap.
func (sb *Superblock_t) SetFreeblock(n int) {
	fieldw(sb.Data, 4, n)
}

// / SetFreeblocklen writes the free block bitmap length.
func (sb *Superblock_t) SetFreeblocklen(n int) {
	fieldw(sb.Data, 5, n)
}

// / SetInodelen writes the number of inode blocks.
func (sb *Superblock_t) SetInodelen(n int) {
	fieldw(sb.Data, 6, n)
}

// / SetLastblock stores the address of the last block on the disk.
func (sb *Superblock_t) SetLastblock(n int) {
	fieldw(sb.Data, 7, n)
}
