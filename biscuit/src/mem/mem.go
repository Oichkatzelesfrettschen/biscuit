package mem

import "runtime"
import "sync"
import "sync/atomic"
import "unsafe"
import "util"
import "fmt"

/// PGSHIFT is the base-2 exponent for the page size.
const PGSHIFT uint = 12

/// PGSIZE is the size of a single page in bytes.
const PGSIZE int = 1 << PGSHIFT

/// PGOFFSET masks offsets within a page.
const PGOFFSET Pa_t = 0xfff

/// PGMASK masks the page number of an address.
const PGMASK Pa_t = ^(PGOFFSET)

/// PTE_P marks a page as present.
const PTE_P Pa_t = 1 << 0

/// PTE_W marks a page writable.
const PTE_W Pa_t = 1 << 1

/// PTE_U marks a page user-accessible.
const PTE_U Pa_t = 1 << 2

/// PTE_G marks a global page.
const PTE_G Pa_t = 1 << 8

/// PTE_PCD disables caching for the page.
const PTE_PCD Pa_t = 1 << 4

/// PTE_PS indicates a large page.
const PTE_PS Pa_t = 1 << 7

/// PTE_ADDR extracts the address bits of a PTE.
const PTE_ADDR Pa_t = PGMASK

/// Pa_t represents a physical address.
type Pa_t uintptr

/// Bytepg_t is a byte addressed page.
type Bytepg_t [PGSIZE]uint8

/// Pg_t is a generic page of ints.
type Pg_t [512]int

/// Pmap_t is a page table page.
type Pmap_t [512]Pa_t

/// Unpin_i allows unpinning of physical pages.
type Unpin_i interface {
	Unpin(Pa_t)
}

/// Mmapinfo_t describes a mapping created by the runtime.
type Mmapinfo_t struct {
	Pg   *Pg_t
	Phys Pa_t
}

/// Page_i abstracts physical page allocation.
type Page_i interface {
	Refpg_new() (*Pg_t, Pa_t, bool)
	Refpg_new_nozero() (*Pg_t, Pa_t, bool)
	Refcnt(Pa_t) int
	Dmap(Pa_t) *Pg_t
	Refup(Pa_t)
	Refdown(Pa_t) bool
}

/// Pg2bytes converts a page of ints to a page of bytes.
func Pg2bytes(pg *Pg_t) *Bytepg_t {
	return (*Bytepg_t)(unsafe.Pointer(pg))
}

/// Bytepg2pg converts a byte page back to a Pg_t.
func Bytepg2pg(pg *Bytepg_t) *Pg_t {
	return (*Pg_t)(unsafe.Pointer(pg))
}

func pg2pmap(pg *Pg_t) *Pmap_t {
	return (*Pmap_t)(unsafe.Pointer(pg))
}

func _pg2pgn(p_pg Pa_t) uint32 {
	return uint32(p_pg >> PGSHIFT)
}

/// Refaddr returns the refcount pointer and index for the given page.
func (phys *Physmem_t) Refaddr(p_pg Pa_t) (*int32, uint32) {
	idx := _pg2pgn(p_pg) - phys.startn
	return &phys.Pgs[idx].Refcnt, idx
}

/// Tlbaddr returns the TLB mask address for a page.
func (phys *Physmem_t) Tlbaddr(p_pg Pa_t) *uint64 {
	idx := _pg2pgn(p_pg) - phys.startn
	return &phys.Pgs[idx].Cpumask
}

/// Physpg_t describes a single physical page and can account for up to 16TB.
type Physpg_t struct {
	Refcnt int32
	// index into pgs of next page on free list
	nexti uint32
	// Bitmask where bit n is set if CPU w/logical ID n loaded this page
	// (which is a pmap) into its cr3 register
	Cpumask uint64
}

/// Physmem_t manages all physical memory for the system.
type Physmem_t struct {
	Pgs    []Physpg_t
	startn uint32
	// index into pgs of first free pg
	freei   uint32
	freelen int32
	pmaps   uint32
	// count of the number of page maps (pml4 pages) in the list, not total
	// pages used in all page maps
	pmaplen int32
	sync.Mutex
	Dmapinit bool
	percpu   [runtime.MAXCPUS]pcpuphys_t
}

type pcpuphys_t struct {
	sync.Mutex
	freei   uint32
	freelen int32
	pmaps   uint32
	// count of the number of page maps (pml4 pages) in the list, not total
	// pages used in all page maps
	pmaplen int32
	//_	[64]uint8
}

func (pc *pcpuphys_t) percpu_init() {
	pc.freei = ^uint32(0)
	pc.pmaps = ^uint32(0)
	pc.freelen, pc.pmaplen = 0, 0
}

// returns true iff the page was added to the per-CPU free list
func (phys *Physmem_t) _pcpu_put(idx uint32, ispmap bool) bool {
	me := runtime.CPUHint()
	mine := &phys.percpu[me]
	var fl *uint32
	var cnt *int32
	if ispmap {
		if mine.pmaplen >= 20 {
			return false
		}
		fl = &mine.pmaps
		cnt = &mine.pmaplen
	} else {
		if mine.freelen >= 100 {
			return false
		}
		fl = &mine.freei
		cnt = &mine.freelen
	}
	phys._phys_insert(fl, idx, mine, cnt)
	return true
}

func (phys *Physmem_t) _pcpu_new(ispmap bool) (*Pg_t, Pa_t, bool) {
	me := runtime.CPUHint()
	mine := &phys.percpu[me]
	fl := &mine.freei
	cnt := &mine.freelen
	if ispmap {
		fl = &mine.pmaps
		cnt = &mine.pmaplen
	}
	return phys._phys_new(fl, mine, cnt)
}

func (phys *Physmem_t) _refpg_new() (*Pg_t, Pa_t, bool) {
	if pg, p_pg, ok := phys._pcpu_new(false); ok {
		return pg, p_pg, ok
	}
	return phys._phys_new(&phys.freei, phys, &phys.freelen)
}

/// Refcnt returns the current reference count of a page.
func (phys *Physmem_t) Refcnt(p_pg Pa_t) int {
	ref, _ := phys.Refaddr(p_pg)
	return int(atomic.LoadInt32(ref))
}

/// Refup increments the reference count of a page.
func (phys *Physmem_t) Refup(p_pg Pa_t) {
	ref, _ := phys.Refaddr(p_pg)
	c := atomic.AddInt32(ref, 1)
	// XXXPANIC
	if c <= 0 {
		panic("wut")
	}
}

// returns true if p_pg should be added to the free list and the index of the
// page in the pgs array
func (phys *Physmem_t) _refdec(p_pg Pa_t) (bool, uint32) {
	ref, idx := phys.Refaddr(p_pg)
	c := atomic.AddInt32(ref, -1)
	// XXXPANIC
	if c < 0 {
		panic("wut")
	}
	return c == 0, idx
}

/// Refdown decrements the reference count of a page.
/// It returns true when the page is freed.
func (phys *Physmem_t) Refdown(p_pg Pa_t) bool {
	return phys._phys_put(p_pg, false)
}

/// Zeropg is a global zero-filled page used for allocations.
var Zeropg *Pg_t

/// Refpg_new allocates a zeroed page and returns its mapping and address.
/// The returned page's refcount is not incremented.
func (phys *Physmem_t) Refpg_new() (*Pg_t, Pa_t, bool) {
	if !phys.Dmapinit {
		panic("refpg_new")
	}
	pg, p_pg, ok := phys._refpg_new()
	if !ok {
		return nil, 0, false
	}
	*pg = *Zeropg
	return pg, p_pg, true
}

/// Refpg_new_nozero allocates an uninitialised page.
func (phys *Physmem_t) Refpg_new_nozero() (*Pg_t, Pa_t, bool) {
	pg, p_pg, ok := phys._refpg_new()
	if !ok {
		return nil, 0, false
	}
	return pg, p_pg, true
}

/// Pmap_new allocates a new page map for the kernel.
func (phys *Physmem_t) Pmap_new() (*Pmap_t, Pa_t, bool) {
	// first try pmap free lists
	a, b, ok := phys._pcpu_new(true)
	if !ok {
		a, b, ok = phys._phys_new(&phys.pmaps, phys, &phys.pmaplen)
	}
	// fall back to page free lists
	if !ok {
		a, b, ok = phys.Refpg_new()
	}
	return pg2pmap(a), b, ok
}

func (phys *Physmem_t) _phys_new(fl *uint32, lock sync.Locker, cnt *int32) (*Pg_t, Pa_t, bool) {
	if !phys.Dmapinit {
		panic("dmap not initted")
	}

	var p_pg Pa_t
	var ok bool
	lock.Lock()
	ff := *fl
	if ff != ^uint32(0) {
		p_pg = Pa_t(ff+phys.startn) << PGSHIFT
		*fl = phys.Pgs[ff].nexti
		ok = true
		if phys.Pgs[ff].Refcnt < 0 {
			panic("negative ref count")
		}
		*cnt--
		if *cnt < 0 {
			panic("no")
		}
	}
	lock.Unlock()
	if ok {
		return phys.Dmap(p_pg), p_pg, true
	}
	return nil, 0, false
}

func (phys *Physmem_t) _phys_insert(fl *uint32, idx uint32, lock sync.Locker, cnt *int32) {
	lock.Lock()
	phys.Pgs[idx].nexti = *fl
	*fl = idx
	*cnt++
	if *cnt < 0 {
		panic("no")
	}
	lock.Unlock()
}

// returns true iff the p_pg was added to the free list
func (phys *Physmem_t) _phys_put(p_pg Pa_t, ispmap bool) bool {
	if add, idx := phys._refdec(p_pg); add {
		if phys._pcpu_put(idx, ispmap) {
			return true
		}
		fl := &phys.freei
		cnt := &phys.freelen
		if ispmap {
			fl = &phys.pmaps
			cnt = &phys.pmaplen
		}
		phys._phys_insert(fl, idx, phys, cnt)
		return true
	}
	return false
}

// decrease ref count of pml4, freeing it if no CPUs have it loaded into cr3.
/// Dec_pmap decreases the reference count of a pmap and frees it if unused.
func (phys *Physmem_t) Dec_pmap(p_pmap Pa_t) {
	phys._phys_put(p_pmap, true)
}

// returns a page-aligned virtual address for the given physical address using
// the direct mapping
/// Dmap converts a physical address into a direct-mapped virtual address.
func (phys *Physmem_t) Dmap(p Pa_t) *Pg_t {
	pa := uintptr(p)
	if pa >= 1<<39 {
		panic("direct map not large enough")
	}

	v := Vdirect
	v += uintptr(util.Rounddown(int(pa), PGSIZE))
	return (*Pg_t)(unsafe.Pointer(v))
}

/// Dmap_v2p converts a direct-mapped virtual address back to a physical address.
func (phys *Physmem_t) Dmap_v2p(v *Pg_t) Pa_t {
	va := (uintptr)(unsafe.Pointer(v))
	if va <= 1<<39 {
		panic("address isn't in the direct map")
	}

	pa := va - Vdirect
	return Pa_t(pa)
}

// returns a byte aligned virtual address for the physical address as slice of
// uint8s
/// Dmap8 returns a byte slice mapped to the given physical address.
func (phys *Physmem_t) Dmap8(p Pa_t) []uint8 {
	pg := phys.Dmap(p)
	off := p & PGOFFSET
	bpg := Pg2bytes(pg)
	return bpg[off:]
}

/// Pgcount reports free page counts across CPUs.
func (phys *Physmem_t) Pgcount() (int, int, []int, []int) {
	phys.Lock()
	r1 := int(phys.freelen)
	r2 := phys.pmapcount(&phys.pmaps)
	phys.Unlock()

	var pcpg []int
	var pcpm []int
	for i := range phys.percpu {
		pc := &phys.percpu[i]
		pc.Lock()
		if pc.freelen|pc.pmaplen != 0 {
			pcpg = append(pcpg, int(pc.freelen))
			pml := phys.pmapcount(&pc.pmaps)
			pcpm = append(pcpm, pml)
		}
		pc.Unlock()
	}
	return r1, r2, pcpg, pcpm
}

func (phys *Physmem_t) _pmcount(pml4 Pa_t, lev int) int {
	if lev == 0 {
		return 0
	}
	pg := pg2pmap(phys.Dmap(pml4))
	ret := 0
	for _, pte := range pg {
		if pte&PTE_U != 0 && pte&PTE_P != 0 {
			ret += 1 + phys._pmcount(Pa_t(pte&PTE_ADDR), lev-1)
		}
	}
	return ret
}

func (phys *Physmem_t) pmapcount(fl *uint32) int {
	c := 0
	for ni := *fl; ni != ^uint32(0); ni = phys.Pgs[ni].nexti {
		v := phys._pmcount(Pa_t(ni+phys.startn)<<PGSHIFT, 4)
		c += v
	}
	return c
}

/// Physmem is the global physical memory allocator instance.
var Physmem = &Physmem_t{}

/// Phys_init initializes the global physical memory allocator.
func Phys_init() *Physmem_t {
	// reserve 128MB of pages
	//respgs := 1 << 15
	respgs := 1 << 16
	//respgs := 1 << (31 - 12)
	// 7.5 GB
	//respgs := 1835008
	//respgs := 1 << 18 + (1 <<17)
	phys := Physmem
	phys.Pgs = make([]Physpg_t, respgs)
	for i := range phys.Pgs {
		phys.Pgs[i].Refcnt = -10
	}
	first := Pa_t(runtime.Get_phys())
	fpgn := _pg2pgn(first)
	phys.startn = fpgn
	phys.freei = 0
	phys.freelen = 1
	phys.pmaps = ^uint32(0)
	phys.Pgs[0].Refcnt = 0
	phys.Pgs[0].nexti = ^uint32(0)
	last := phys.freei
	for i := 0; i < respgs-1; i++ {
		p_pg := Pa_t(runtime.Get_phys())
		pgn := _pg2pgn(p_pg)
		idx := pgn - phys.startn
		// Get_phys() may skip regions.
		if int(idx) >= len(phys.Pgs) {
			if respgs-i > int(float64(respgs)*0.01) {
				panic("got many bad pages")
			}
			break
		}
		phys.Pgs[idx].Refcnt = 0
		phys.Pgs[last].nexti = idx
		phys.Pgs[idx].nexti = ^uint32(0)
		last = idx
		phys.freelen++
	}
	fmt.Printf("Reserved %v pages (%vMB)\n", respgs, respgs>>8)
	for i := range phys.percpu {
		phys.percpu[i].percpu_init()
	}
	return phys
}
