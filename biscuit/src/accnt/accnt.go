package accnt

import "sync"
import "sync/atomic"
import "time"

import "util"

/// Accnt_t tracks accumulated CPU usage for a thread.
///
/// Fields:
///   Userns - nanoseconds spent executing in user mode
///   Sysns  - nanoseconds spent executing in system mode
///   Mutex  - protects concurrent updates
///
/// No package level globals are referenced by this type.
type Accnt_t struct {
	Userns     int64 /// nanoseconds spent in user mode
	Sysns      int64 /// nanoseconds spent in system mode
	sync.Mutex       /// guards concurrent updates
}

/// Utadd increments the accumulated user time.
///
/// Parameters:
///   delta - nanoseconds added to Userns.
///
/// No global variables are referenced.
func (a *Accnt_t) Utadd(delta int) {
	atomic.AddInt64(&a.Userns, int64(delta))
}

/// Systadd increments the accumulated system time.
///
/// Parameters:
///   delta - nanoseconds added to Sysns.
///
/// No global variables are referenced.
func (a *Accnt_t) Systadd(delta int) {
	atomic.AddInt64(&a.Sysns, int64(delta))
}

/// Now returns the current wall time.
///
/// Return value:
///   int - nanoseconds since the Unix epoch.
///
/// No globals are referenced.
func (a *Accnt_t) Now() int {
	return int(time.Now().UnixNano())
}

/// Io_time removes elapsed I/O time from the system total.
///
/// Parameters:
///   since - timestamp previously returned by Now.
///
/// No global variables are used.
func (a *Accnt_t) Io_time(since int) {
	d := a.Now() - since
	a.Systadd(-d)
}

/// Sleep_time subtracts time spent sleeping from system usage.
///
/// Parameters:
///   since - timestamp when sleeping began.
///
/// No global variables are referenced.
func (a *Accnt_t) Sleep_time(since int) {
	d := a.Now() - since
	a.Systadd(-d)
}

/// Finish records system time consumed since a start time.
///
/// Parameters:
///   inttime - timestamp returned by Now marking the start.
///
/// No global variables are referenced.
func (a *Accnt_t) Finish(inttime int) {
	a.Systadd(a.Now() - inttime)
}

/// Add merges another accounting structure into the receiver.
///
/// Parameters:
///   n - pointer to another Accnt_t whose values are accumulated.
///
/// No global variables are referenced.
func (a *Accnt_t) Add(n *Accnt_t) {
	a.Lock()
	a.Userns += n.Userns
	a.Sysns += n.Sysns
	a.Unlock()
}

/// Fetch returns a usage snapshot encoded as a rusage structure.
///
/// Return value:
///   []uint8 - rusage-encoded user and system times.
///
/// No global variables are referenced.
func (a *Accnt_t) Fetch() []uint8 {
	a.Lock()
	ru := a.To_rusage()
	a.Unlock()
	return ru
}

/// To_rusage encodes accumulated times into a rusage structure.
///
/// Return value:
///   []uint8 - serialized rusage contents.
///
/// No global variables are referenced.
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
