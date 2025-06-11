package vm

import "fmt"
import "runtime"
import "sync"
import "unsafe"

import "bounds"
import "defs"
import "res"

// / Userbuf_t assists reading and writing user memory. Address lookups
// / and accesses are atomic with respect to page faults.
type Userbuf_t struct {
	userva int
	len    int
	// 0 <= off <= len
	off int
	as  *Vm_t
}

// / ub_init initialises the buffer for the given address space.
func (ub *Userbuf_t) ub_init(as *Vm_t, uva, len int) {
	// XXX fix signedness
	if len < 0 {
		panic("negative length")
	}
	if len >= 1<<39 {
		fmt.Printf("suspiciously large user buffer (%v)\n", len)
	}
	ub.userva = uva
	ub.len = len
	ub.off = 0
	ub.as = as
}

// / Remain returns the number of unread bytes left in the buffer.
func (ub *Userbuf_t) Remain() int {
	return ub.len - ub.off
}

// / Totalsz reports the total size of the buffer in bytes.
func (ub *Userbuf_t) Totalsz() int {
	return ub.len
}

// / Uioread copies data from user memory into dst and returns the
// / number of bytes read along with an error code.
func (ub *Userbuf_t) Uioread(dst []uint8) (int, defs.Err_t) {
	ub.as.Lock_pmap()
	a, b := ub._tx(dst, false)
	ub.as.Unlock_pmap()
	return a, b
}

// / Uiowrite copies data from src into user memory and returns the
// / number of bytes written along with an error code.
func (ub *Userbuf_t) Uiowrite(src []uint8) (int, defs.Err_t) {
	ub.as.Lock_pmap()
	a, b := ub._tx(src, true)
	ub.as.Unlock_pmap()
	return a, b
}

// copies the min of either the provided buffer or ub.len. returns number of
// bytes copied and error. if an error occurs in the middle of a read or write,
// the userbuf's state is updated such that the operation can be restarted.
func (ub *Userbuf_t) _tx(buf []uint8, write bool) (int, defs.Err_t) {
	ret := 0
	for len(buf) != 0 && ub.off != ub.len {
		if !res.Resadd_noblock(bounds.Bounds(bounds.B_USERBUF_T__TX)) {
			return ret, -defs.ENOHEAP
		}
		va := ub.userva + ub.off
		ubuf, err := ub.as.Userdmap8_inner(va, write)
		if err != 0 {
			return ret, err
		}
		end := ub.off + len(ubuf)
		if end > ub.len {
			left := ub.len - ub.off
			ubuf = ubuf[:left]
		}
		var c int
		if write {
			c = copy(ubuf, buf)
		} else {
			c = copy(buf, ubuf)
		}
		buf = buf[c:]
		ub.off += c
		ret += c
	}
	return ret, 0
}

type _iove_t struct {
	uva uint
	sz  int
}

// / Useriovec_t represents a sequence of user buffers defined by the
// / iovec array in user memory.
type Useriovec_t struct {
	iovs []_iove_t
	tsz  int
	as   *Vm_t
}

// / Iov_init initializes the iovec array from user memory at iovarn.
// / It returns an error code if the array cannot be read.
func (iov *Useriovec_t) Iov_init(as *Vm_t, iovarn uint, niovs int) defs.Err_t {
	if niovs > 10 {
		fmt.Printf("many iovecs\n")
		return -defs.EINVAL
	}
	iov.tsz = 0
	iov.iovs = make([]_iove_t, niovs)
	iov.as = as

	as.Lock_pmap()
	defer as.Unlock_pmap()
	for i := range iov.iovs {
		gimme := bounds.Bounds(bounds.B_USERIOVEC_T_IOV_INIT)
		if !res.Resadd_noblock(gimme) {
			return -defs.ENOHEAP
		}
		elmsz := uint(16)
		va := iovarn + uint(i)*elmsz
		dstva, err := as.userreadn_inner(int(va), 8)
		if err != 0 {
			return err
		}
		sz, err := as.userreadn_inner(int(va)+8, 8)
		if err != 0 {
			return err
		}
		iov.iovs[i].uva = uint(dstva)
		iov.iovs[i].sz = sz
		iov.tsz += sz
	}
	return 0
}

// / Remain returns the number of bytes remaining across all iovecs.
func (iov *Useriovec_t) Remain() int {
	ret := 0
	for i := range iov.iovs {
		ret += iov.iovs[i].sz
	}
	return ret
}

// / Totalsz returns the total number of bytes described by the iovec
// / array.
func (iov *Useriovec_t) Totalsz() int {
	return iov.tsz
}

func (iov *Useriovec_t) _tx(buf []uint8, touser bool) (int, defs.Err_t) {
	ub := &Userbuf_t{}
	did := 0
	for len(buf) > 0 && len(iov.iovs) > 0 {
		if !res.Resadd_noblock(bounds.Bounds(bounds.B_USERIOVEC_T__TX)) {
			return did, -defs.ENOHEAP
		}
		ciov := &iov.iovs[0]
		ub.ub_init(iov.as, int(ciov.uva), ciov.sz)
		var c int
		var err defs.Err_t
		if touser {
			c, err = ub._tx(buf, true)
		} else {
			c, err = ub._tx(buf, false)
		}
		ciov.uva += uint(c)
		ciov.sz -= c
		if ciov.sz == 0 {
			iov.iovs = iov.iovs[1:]
		}
		buf = buf[c:]
		did += c
		if err != 0 {
			return did, err
		}
	}
	return did, 0
}

// / Uioread reads into dst from the set of user buffers and returns the
// / number of bytes copied along with an error code.
func (iov *Useriovec_t) Uioread(dst []uint8) (int, defs.Err_t) {
	iov.as.Lock_pmap()
	a, b := iov._tx(dst, false)
	iov.as.Unlock_pmap()
	return a, b
}

// / Uiowrite writes src to the user buffers and returns the number of
// / bytes copied along with an error code.
func (iov *Useriovec_t) Uiowrite(src []uint8) (int, defs.Err_t) {
	iov.as.Lock_pmap()
	a, b := iov._tx(src, true)
	iov.as.Unlock_pmap()
	return a, b
}

// / Fakeubuf_t implements the same interface as Userbuf_t but
// / operates on a kernel buffer. It is used when the kernel needs to
// / treat internal memory like user memory.
type Fakeubuf_t struct {
	fbuf []uint8
	off  int
	len  int
}

// / Fake_init sets up the fake buffer with the provided slice.
func (fb *Fakeubuf_t) Fake_init(buf []uint8) {
	fb.fbuf = buf
	fb.len = len(fb.fbuf)
}

// / Remain returns the number of bytes left in the fake buffer.
func (fb *Fakeubuf_t) Remain() int {
	return len(fb.fbuf)
}

// / Totalsz returns the total length of the fake buffer.
func (fb *Fakeubuf_t) Totalsz() int {
	return fb.len
}

func (fb *Fakeubuf_t) _tx(buf []uint8, tofbuf bool) (int, defs.Err_t) {
	var c int
	if tofbuf {
		c = copy(fb.fbuf, buf)
	} else {
		c = copy(buf, fb.fbuf)
	}
	fb.fbuf = fb.fbuf[c:]
	return c, 0
}

// / Uioread copies from the fake buffer into dst.
func (fb *Fakeubuf_t) Uioread(dst []uint8) (int, defs.Err_t) {
	return fb._tx(dst, false)
}

// / Uiowrite copies src into the fake buffer.
func (fb *Fakeubuf_t) Uiowrite(src []uint8) (int, defs.Err_t) {
	return fb._tx(src, true)
}

// / Ubpool provides reusable Userbuf_t structures to reduce allocations.
var Ubpool = sync.Pool{New: func() interface{} { return new(Userbuf_t) }}

// / Mkfxbuf allocates a 16-byte aligned buffer initialized for
// / floating-point context storage.
func Mkfxbuf() *[64]uintptr {
	ret := new([64]uintptr)
	n := uintptr(unsafe.Pointer(ret))
	if n&((1<<4)-1) != 0 {
		panic("not 16 byte aligned")
	}
	*ret = runtime.Fxinit
	return ret
}
