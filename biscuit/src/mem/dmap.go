package mem

//import "sync/atomic"
import "unsafe"
import "runtime"
import "fmt"

// lowest userspace address

/// VREC is the recursive mapping slot used by the kernel.
const VREC int = 0x42

/// VDIRECT is the direct-map slot.
const VDIRECT int = 0x44

/// VEND marks the end of kernel virtual space.
const VEND int = 0x50

/// VUSER is the first user-space slot.
const VUSER int = 0x59

/// USERMIN is the lowest user virtual address.
const USERMIN int = VUSER << 39

/// DMAPLEN is the length of the direct map in bytes.
const DMAPLEN int = 1 << 39

/// Vdirect holds the virtual address of the direct map region.
var Vdirect = uintptr(VDIRECT << 39)

/// Dmaplen returns a slice over the direct map starting at p for l bytes.
func Dmaplen(p Pa_t, l int) []uint8 {
	_dmap := (*[DMAPLEN]uint8)(unsafe.Pointer(Vdirect))
	return _dmap[p : p+Pa_t(l)]
}

/// Dmaplen32 is like Dmaplen but operates on 32-bit units.
/// p and l must be multiples of 4.
func Dmaplen32(p uintptr, l int) []uint32 {
	if p%4 != 0 || l%4 != 0 {
		panic("not 32bit aligned")
	}
	_dmap := (*[DMAPLEN / 4]uint32)(unsafe.Pointer(Vdirect))
	p /= 4
	l /= 4
	return _dmap[p : p+uintptr(l)]
}

func shl(c uint) uint {
	return 12 + 9*c
}

func pgbits(v uint) (uint, uint, uint, uint) {
	lb := func(c uint) uint {
		return (v >> shl(c)) & 0x1ff
	}
	return lb(3), lb(2), lb(1), lb(0)
}

func mkpg(l4 int, l3 int, l2 int, l1 int) int {
	lb := func(c uint) uint {
		var ret uint
		switch c {
		case 3:
			ret = uint(l4) & 0x1ff
		case 2:
			ret = uint(l3) & 0x1ff
		case 1:
			ret = uint(l2) & 0x1ff
		case 0:
			ret = uint(l1) & 0x1ff
		}
		return ret << shl(c)
	}

	return int(lb(3) | lb(2) | lb(1) | lb(0))
}

func caddr(l4 int, ppd int, pd int, pt int, off int) *int {
	ret := mkpg(l4, ppd, pd, pt)
	ret += off * 8

	return (*int)(unsafe.Pointer(uintptr(ret)))
}

/// Kent_t records a kernel page-map entry.
type Kent_t struct {
	Pml4slot int
	Entry    Pa_t
}

/// Zerobpg is a byte representation of the zero page.
var Zerobpg *Bytepg_t

/// P_zeropg is the physical address of Zerobpg.
var P_zeropg Pa_t

/// Kents contains all kernel PML4 entries.
var Kents = make([]Kent_t, 0, 5)

/// Dmap_init installs the direct map covering all physical memory.
func Dmap_init() {
	//kpmpages.pminit()

	// the default cpu qemu uses for x86_64 supports 1GB pages, but
	// doesn't report it in cpuid 0x80000001... i wonder why.
	_, _, _, edx := runtime.Cpuid(0x80000001, 0)
	gbpages := edx&(1<<26) != 0

	_, _, _, edx = runtime.Cpuid(0x1, 0)
	gse := edx&(1<<13) != 0
	if !gse {
		panic("no global pages")
	}
	if runtime.Rcr4()&(1<<7) == 0 {
		panic("global disabled")
	}

	_dpte := caddr(VREC, VREC, VREC, VREC, VDIRECT)
	dpte := (*Pa_t)(unsafe.Pointer(_dpte))
	if *dpte&PTE_P != 0 {
		panic("dmap slot taken")
	}

	pdpt := new(Pmap_t)
	ptn := Pa_t(unsafe.Pointer(pdpt))
	if ptn&PGOFFSET != 0 {
		panic("page table not aligned")
	}
	p_pdpt, ok := runtime.Vtop(unsafe.Pointer(pdpt))
	if !ok {
		panic("must succeed")
	}
	kpgadd(pdpt)

	*dpte = Pa_t(p_pdpt) | PTE_P | PTE_W

	size := Pa_t(1 << 30)

	// make qemu use 2MB pages, like my hardware, to help expose bugs that
	// the hardware may encounter.
	if gbpages {
		fmt.Printf("dmap via 1GB pages\n")
		for i := range pdpt {
			pdpt[i] = Pa_t(i)*size | PTE_P | PTE_W | PTE_PS
		}
	} else {
		fmt.Printf("1GB pages not supported\n")

		size = 1 << 21
		pdptsz := Pa_t(1 << 30)
		for i := range pdpt {
			pd := new(Pmap_t)
			p_pd, ok := runtime.Vtop(unsafe.Pointer(pd))
			if !ok {
				panic("must succeed")
			}
			kpgadd(pd)
			for j := range pd {
				pd[j] = Pa_t(i)*pdptsz +
					Pa_t(j)*size | PTE_P | PTE_W | PTE_PS
			}
			pdpt[i] = Pa_t(p_pd) | PTE_P | PTE_W
		}
	}

	// fill in kent, the list of kernel pml4 entries. make sure we will
	// panic if the runtime adds new mappings that we don't record here.
	runtime.Pml4freeze()
	for i, e := range Kpmap() {
		if e&PTE_U == 0 && e&PTE_P != 0 {
			ent := Kent_t{i, e}
			Kents = append(Kents, ent)
		}
	}
	Physmem.Dmapinit = true

	// PhysRefpg_new() uses the Zeropg to zero the page
	Zeropg, P_zeropg, ok = Physmem._refpg_new()
	if !ok {
		panic("oom in dmap init")
	}
	for i := range Zeropg {
		Zeropg[i] = 0
	}
	Physmem.Refup(P_zeropg)
	Zerobpg = Pg2bytes(Zeropg)
}

/// Kpmapp caches the kernel's top-level page map.
var Kpmapp *Pmap_t

/// Kpmap returns the kernel's pmap pointer.
func Kpmap() *Pmap_t {
	if Kpmapp == nil {
		dur := caddr(VREC, VREC, VREC, VREC, 0)
		Kpmapp = (*Pmap_t)(unsafe.Pointer(dur))
	}
	return Kpmapp
}

// tracks all pages allocated by go internally by the kernel such as pmap pages
// allocated by the kernel (not the bootloader/runtime)
var kpages = pgtracker_t{}

func kpgadd(pg *Pmap_t) {
	va := uintptr(unsafe.Pointer(pg))
	pgn := int(va >> 12)
	if _, ok := kpages[pgn]; ok {
		panic("page already in kpages")
	}
	kpages[pgn] = pg
}

// tracks pages
type pgtracker_t map[int]*Pmap_t
