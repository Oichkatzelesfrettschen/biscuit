package oommsg

/// OomCh is notified when the system runs out of memory.
var OomCh chan Oommsg_t = make(chan Oommsg_t)

/// Oommsg_t is sent on OomCh when memory is exhausted.
type Oommsg_t struct {
	Need   int
	Resume chan bool
}
