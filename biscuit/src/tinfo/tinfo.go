package tinfo

import "runtime"
import "sync"
import "unsafe"

import "defs"

/// Tnote_t stores per-thread state used by the runtime.
type Tnote_t struct {
	// XXX "alive" should be "terminated"
	State    interface{}
	Alive    bool
	Killed   bool
	Isdoomed bool // XXX maybe don't need doomed, but can use killed?
	// protects killed, Killnaps.Cond and Kerr, and is a leaf lock
	sync.Mutex
	Killnaps struct {
		Killch chan bool
		Cond   *sync.Cond
		Kerr   defs.Err_t
	}
}

/// Doomed reports whether the thread is marked as doomed.
func (t *Tnote_t) Doomed() bool {
	return t.Isdoomed
}

/// Threadinfo_t tracks all thread notes.
type Threadinfo_t struct {
	Notes map[defs.Tid_t]*Tnote_t
	sync.Mutex
}

/// Init initializes the thread info map.
func (t *Threadinfo_t) Init() {
	t.Notes = make(map[defs.Tid_t]*Tnote_t)
}

/// Current returns the current thread note.
func Current() *Tnote_t {
	_p := runtime.Gptr()
	if _p == nil {
		panic("nuts")
	}
	ret := (*Tnote_t)(_p)
	return ret
}

/// SetCurrent installs p as the current thread note.
func SetCurrent(p *Tnote_t) {
	if p == nil {
		panic("nuts")
	}
	if runtime.Gptr() != nil {
		panic("nuts")
	}
	_p := (unsafe.Pointer)(p)
	runtime.Setgptr(_p)
}

/// ClearCurrent removes the current thread note.
func ClearCurrent() {
	if runtime.Gptr() == nil {
		panic("nuts")
	}
	runtime.Setgptr(nil)
}
