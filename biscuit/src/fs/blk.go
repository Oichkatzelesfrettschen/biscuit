package fs

import "sync"
import "fmt"
import "container/list"

import "mem"

// If you change this, you must change corresponding constants in litc.c
// (fopendir, BSIZE), usertests.c (BSIZE).
const BSIZE = 4096 /// size of a disk block in bytes

// / Blockmem_i abstracts page allocation for block buffers.
type Blockmem_i interface {
	Alloc() (mem.Pa_t, *mem.Bytepg_t, bool)
	Free(mem.Pa_t)
	Refup(mem.Pa_t)
}

// / Block_cb_i is implemented by callers wanting release callbacks.
type Block_cb_i interface {
	Relse(*Bdev_block_t, string)
}

// / blktype_t enumerates the types of blocks stored on disk.
type blktype_t int

const (
	DataBlk   blktype_t = 0  /// regular data block
	CommitBlk blktype_t = -1 /// log commit record
	RevokeBlk blktype_t = -2 /// log revoke record
)

// / Bdev_block_t represents a cached disk block.
type Bdev_block_t struct {
	sync.Mutex
	Block      int
	Type       blktype_t
	_try_evict bool
	Pa         mem.Pa_t
	Data       *mem.Bytepg_t
	Ref        *Objref_t
	Name       string
	Mem        Blockmem_i
	Disk       Disk_i
	Cb         Block_cb_i
}

// / Bdevcmd_t enumerates disk request types.
type Bdevcmd_t uint

const (
	BDEV_WRITE Bdevcmd_t = 1 /// write a block
	BDEV_READ            = 2 /// read a block
	BDEV_FLUSH           = 3 /// flush outstanding writes
)

// / BlkList_t wraps a list.List of block pointers.
type BlkList_t struct {
	l *list.List
	e *list.Element // iterator
}

// / MkBlkList creates an empty block list.
// / MkBlkList creates an empty block list.
func MkBlkList() *BlkList_t {
	bl := &BlkList_t{}
	bl.l = list.New()
	return bl
}

// / Len returns the number of blocks in the list.
func (bl *BlkList_t) Len() int {
	return bl.l.Len()
}

// / PushBack appends a block to the list.
func (bl *BlkList_t) PushBack(b *Bdev_block_t) {
	bl.l.PushBack(b)
}

// / FrontBlock resets the iterator and returns the first block.
func (bl *BlkList_t) FrontBlock() *Bdev_block_t {
	if bl.l.Front() == nil {
		return nil
	} else {
		bl.e = bl.l.Front()
		return bl.e.Value.(*Bdev_block_t)
	}
}

// / Back returns the last block in the list or nil.
func (bl *BlkList_t) Back() *Bdev_block_t {
	if bl.l.Back() == nil {
		return nil
	} else {
		return bl.l.Back().Value.(*Bdev_block_t)
	}
}

// / BackBlock returns the last block or panics if empty.
func (bl *BlkList_t) BackBlock() *Bdev_block_t {
	if bl.l.Back() == nil {
		panic("bl.Front")
	} else {
		return bl.l.Back().Value.(*Bdev_block_t)
	}
}

// / RemoveBlock removes the block with the given number.
func (bl *BlkList_t) RemoveBlock(block int) {
	var next *list.Element
	for e := bl.l.Front(); e != nil; e = next {
		next = e.Next()
		b := e.Value.(*Bdev_block_t)
		if b.Block == block {
			bl.l.Remove(e)
		}
	}
}

// / NextBlock advances the iterator and returns the next block.
func (bl *BlkList_t) NextBlock() *Bdev_block_t {
	if bl.e == nil {
		return nil
	} else {
		bl.e = bl.e.Next()
		if bl.e == nil {
			return nil
		}
		return bl.e.Value.(*Bdev_block_t)
	}
}

// / Apply calls f for each block in the list.
func (bl *BlkList_t) Apply(f func(*Bdev_block_t)) {
	for b := bl.FrontBlock(); b != nil; b = bl.NextBlock() {
		f(b)
	}
}

// / Print dumps each block number to standard output.
func (bl *BlkList_t) Print() {
	bl.Apply(func(b *Bdev_block_t) {
		fmt.Printf("b %v\n", b)
	})
}

// / Append adds all blocks from l to the end of bl.
func (bl *BlkList_t) Append(l *BlkList_t) {
	for b := l.FrontBlock(); b != nil; b = l.NextBlock() {
		bl.PushBack(b)
	}
}

// / Delete removes all elements from the list.
func (bl *BlkList_t) Delete() {
	var next *list.Element
	for e := bl.l.Front(); e != nil; e = next {
		next = e.Next()
		bl.l.Remove(e)
	}
}

// / Bdev_req_t describes a block device request.
type Bdev_req_t struct {
	Cmd   Bdevcmd_t
	Blks  *BlkList_t
	AckCh chan bool
	Sync  bool
}

// / MkRequest allocates a new block request structure.
func MkRequest(blks *BlkList_t, cmd Bdevcmd_t, sync bool) *Bdev_req_t {
	ret := &Bdev_req_t{}
	ret.Blks = blks
	ret.AckCh = make(chan bool)
	ret.Cmd = cmd
	ret.Sync = sync
	return ret
}

// / Disk_i represents a physical disk interface.
type Disk_i interface {
	Start(*Bdev_req_t) bool
	Stats() string
}

// / Key returns the lookup key for the block cache.
func (blk *Bdev_block_t) Key() int {
	return blk.Block
}

// / EvictFromCache is called before the block leaves the cache.
func (blk *Bdev_block_t) EvictFromCache() {
	// nothing to be done right before being evicted
}

// / EvictDone finalizes eviction by freeing memory.
func (blk *Bdev_block_t) EvictDone() {
	if bdev_debug {
		fmt.Printf("Done: block %v %#x\n", blk.Block, blk.Pa)
	}
	blk.Mem.Free(blk.Pa)
}

// / Tryevict marks the block for eviction on release.
func (blk *Bdev_block_t) Tryevict() {
	blk._try_evict = true
}

// / Evictnow reports whether the block should be evicted.
func (blk *Bdev_block_t) Evictnow() bool {
	return blk._try_evict
}

// / Done releases a reference via the callback.
func (blk *Bdev_block_t) Done(s string) {
	if blk.Cb == nil {
		panic("wtf")
	}
	blk.Cb.Relse(blk, s)
}

// / Write synchronously writes the block to disk.
func (b *Bdev_block_t) Write() {
	if bdev_debug {
		fmt.Printf("bdev_write %v %v\n", b.Block, b.Name)
	}
	//if b.Data[0] == 0xc && b.Data[1] == 0xc { // XXX check
	//	panic("write\n")
	//}
	l := MkBlkList()
	l.PushBack(b)
	req := MkRequest(l, BDEV_WRITE, true)
	if b.Disk.Start(req) {
		<-req.AckCh
	}
}

// / Write_async writes the block to disk without waiting for completion.
func (b *Bdev_block_t) Write_async() {
	if bdev_debug {
		fmt.Printf("bdev_write_async %v %s\n", b.Block, b.Name)
	}
	// if b.data[0] == 0xc && b.data[1] == 0xc {  // XXX check
	//	panic("write_async\n")
	//}
	l := MkBlkList()
	l.PushBack(b)
	ider := MkRequest(l, BDEV_WRITE, false)
	b.Disk.Start(ider)
}

// / Read reads the block from disk synchronously.
func (b *Bdev_block_t) Read() {
	l := MkBlkList()
	l.PushBack(b)
	ider := MkRequest(l, BDEV_READ, true)
	if b.Disk.Start(ider) {
		<-ider.AckCh
	}
	if bdev_debug {
		fmt.Printf("bdev_read %v %v %#x %#x\n", b.Block, b.Name, b.Data[0], b.Data[1])
	}

	// XXX sanity check, but ignore it during recovery
	if b.Data[0] == 0xc && b.Data[1] == 0xc {
		fmt.Printf("WARNING: %v %v\n", b.Name, b.Block)
	}

}

// / New_page allocates backing memory for the block.
func (blk *Bdev_block_t) New_page() {
	pa, d, ok := blk.Mem.Alloc()
	if !ok {
		panic("oom during bdev.new_page")
	}
	blk.Pa = pa
	blk.Data = d
}

// / MkBlock_newpage allocates a block and backing page.
func MkBlock_newpage(block int, s string, mem Blockmem_i, d Disk_i, cb Block_cb_i) *Bdev_block_t {
	b := MkBlock(block, s, mem, d, cb)
	b.New_page()
	return b
}

// / MkBlock constructs a block without allocating memory.
func MkBlock(block int, s string, m Blockmem_i, d Disk_i, cb Block_cb_i) *Bdev_block_t {
	b := &Bdev_block_t{}
	b.Block = block
	b.Pa = mem.Pa_t(0)
	b.Data = nil
	//b.Name = s
	b.Mem = m
	b.Disk = d
	b.Cb = cb
	return b
}

// / Free_page releases the page backing the block.
func (blk *Bdev_block_t) Free_page() {
	blk.Mem.Free(blk.Pa)
}
