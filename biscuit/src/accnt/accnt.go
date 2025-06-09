package accnt

import "sync"
import "sync/atomic"
import "time"

import "util"

// Accnt_t tracks accumulated user and system time in nanoseconds.
// The embedded mutex allows safe concurrent snapshots.
type Accnt_t struct {
	Userns     int64 /// nanoseconds spent in user mode
	Sysns      int64 /// nanoseconds spent in system mode
	sync.Mutex       /// guards concurrent updates
}

// / Utadd increments the accumulated user time by delta nanoseconds.
func (a *Accnt_t) Utadd(delta int) {
	atomic.AddInt64(&a.Userns, int64(delta))
}

// / Systadd increments the accumulated system time by delta nanoseconds.
func (a *Accnt_t) Systadd(delta int) {
	atomic.AddInt64(&a.Sysns, int64(delta))
}

// / Now returns the current time in nanoseconds.
func (a *Accnt_t) Now() int {
	return int(time.Now().UnixNano())
}

// / Io_time subtracts the elapsed time since from system time.
func (a *Accnt_t) Io_time(since int) {
	d := a.Now() - since
	a.Systadd(-d)
}

// / Sleep_time accounts for time spent sleeping since the provided timestamp.
func (a *Accnt_t) Sleep_time(since int) {
	d := a.Now() - since
	a.Systadd(-d)
}

// / Finish records the system time consumed since the provided start.
func (a *Accnt_t) Finish(inttime int) {
	a.Systadd(a.Now() - inttime)
}

// / Add merges another accounting structure into this one.
func (a *Accnt_t) Add(n *Accnt_t) {
	a.Lock()
	a.Userns += n.Userns
	a.Sysns += n.Sysns
	a.Unlock()
}

// / Fetch returns a usage snapshot encoded as a rusage structure.
func (a *Accnt_t) Fetch() []uint8 {
	a.Lock()
	ru := a.To_rusage()
	a.Unlock()
	return ru
}

// / To_rusage builds a rusage byte slice representing user and system time.
func (a *Accnt_t) To_rusage() []uint8 {
	words := 4
	ret := make([]uint8, words*8)
	totv := func(nano int64) (int, int) {
		secs := int(nano / 1e9)
		usecs := int((nano % 1e9) / 1000)
		return secs, usecs
	}
	off := 0
	// user timeval
	s, us := totv(a.Userns)
	util.Writen(ret, 8, off, s)
	off += 8
	util.Writen(ret, 8, off, us)
	off += 8
	// sys timeval
	s, us = totv(a.Sysns)
	util.Writen(ret, 8, off, s)
	off += 8
	util.Writen(ret, 8, off, us)
	off += 8
	return ret
}
