package msi

import "sync"

// Msivec_t represents an MSI interrupt vector.
type Msivec_t uint

// Msivecs_t tracks available MSI vectors.
type Msivecs_t struct {
	sync.Mutex
	avail map[Msivec_t]bool
}

var msivecs = Msivecs_t{
	avail: map[Msivec_t]bool{56: true, 57: true, 58: true, 59: true, 60: true,
		61: true, 62: true, 63: true},
}

// Msi_alloc allocates an available MSI vector.
func Msi_alloc() Msivec_t {
	msivecs.Lock()
	defer msivecs.Unlock()

	for i := range msivecs.avail {
		delete(msivecs.avail, i)
		return i
	}
	panic("no more MSI vecs")
}

// Msi_free releases a previously allocated MSI vector.
func Msi_free(vector Msivec_t) {
	msivecs.Lock()
	defer msivecs.Unlock()

	if msivecs.avail[vector] {
		panic("double free")
	}
	msivecs.avail[vector] = true
}
