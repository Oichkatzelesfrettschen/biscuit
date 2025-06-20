package ufs

import "os"
import "sync"

import "defs"
import "fdops"
import "fs"
import "mem"

//
// The "driver"
//

/// ahci_disk_t simulates a disk backed by a file.
type ahci_disk_t struct {
	sync.Mutex
	f *os.File
	t *tracef_t
}

/// StartTrace enables tracing of write operations.
func (ahci *ahci_disk_t) StartTrace() {
	ahci.t = mkTrace()
}

/// Seek moves the underlying file offset to o.
func (ahci *ahci_disk_t) Seek(o int) {
	_, err := ahci.f.Seek(int64(o), 0)
	if err != nil {
		panic(err)
	}
}

/// Start services a block device request.
func (ahci *ahci_disk_t) Start(req *fs.Bdev_req_t) bool {
	ahci.Lock() // lock to ensure that seek folllowed by read/write is atomic
	defer ahci.Unlock()

	switch req.Cmd {
	case fs.BDEV_READ:
		if req.Blks.Len() != 1 {
			panic("read: too many blocks")
		}
		blk := req.Blks.FrontBlock()
		ahci.Seek(blk.Block * fs.BSIZE)
		b := make([]byte, fs.BSIZE)
		n, err := ahci.f.Read(b)
		if n != fs.BSIZE || err != nil {
			panic(err)
		}
		blk.Data = &mem.Bytepg_t{}
		for i, _ := range b {
			blk.Data[i] = uint8(b[i])
		}
	case fs.BDEV_WRITE:
		for b := req.Blks.FrontBlock(); b != nil; b = req.Blks.NextBlock() {
			ahci.Seek(b.Block * fs.BSIZE)
			buf := make([]byte, fs.BSIZE)
			for i, _ := range buf {
				buf[i] = byte(b.Data[i])
			}
			n, err := ahci.f.Write(buf)
			if n != fs.BSIZE || err != nil {
				panic(err)
			}
			if ahci.t != nil {
				ahci.t.write(b.Block, b.Data)
			}
			b.Done("Start")
		}
	case fs.BDEV_FLUSH:
		ahci.f.Sync()
		if ahci.t != nil {
			ahci.t.sync()
		}
	}
	return false
}

/// Stats returns statistics for the disk.
func (ahci *ahci_disk_t) Stats() string {
	return ""
}

func (ahci *ahci_disk_t) close() {
	if ahci.t != nil {
		ahci.t.close()
	}
	// ahci.f.Sync()
	err := ahci.f.Close()
	if err != nil {
		panic(err)
	}
}

//
// Glue
//

/// blockmem_t provides memory for disk blocks during tests.
type blockmem_t struct {
}

var blockmem = &blockmem_t{}

/// Alloc returns a zeroed memory page for block operations.
func (bm *blockmem_t) Alloc() (mem.Pa_t, *mem.Bytepg_t, bool) {
	d := &mem.Bytepg_t{}
	return mem.Pa_t(0), d, true
}

/// Free releases a previously allocated page.
func (bm *blockmem_t) Free(pa mem.Pa_t) {
}

/// Refup increments the reference count of a page.
func (bm *blockmem_t) Refup(pa mem.Pa_t) {
}

/// console_t is a stub console driver used in tests.
type console_t struct {
}

var c console_t

/// Cons_poll implements fdops. It always reports not ready.
func (c console_t) Cons_poll(pm fdops.Pollmsg_t) (fdops.Ready_t, defs.Err_t) {
	return 0, 0
}

/// Cons_read is a stub read that returns an error.
func (c console_t) Cons_read(ub fdops.Userio_i, offset int) (int, defs.Err_t) {
	return -1, 0
}

/// Cons_write discards the provided data.
func (c console_t) Cons_write(src fdops.Userio_i, off int) (int, defs.Err_t) {
	return 0, 0
}
