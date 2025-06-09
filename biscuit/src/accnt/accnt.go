package accnt

import "sync"
import "sync/atomic"
import "time"

import "util"

/**
 * Accnt_t accumulates per-process accounting information.
 *
 * Both Userns and Sysns store runtime in nanoseconds. The embedded
 * mutex allows callers to take a consistent snapshot of the fields
 * when exporting usage statistics.
 */
type Accnt_t struct {
	/// Nanoseconds of user time consumed.
	Userns int64
	/// Nanoseconds of system time consumed.
	Sysns int64
	/// Protects concurrent access when reporting usage data.
	sync.Mutex
}

/// Utadd adds delta nanoseconds to the user-time counter.
///
/// @param delta Amount to add in nanoseconds.
func (a *Accnt_t) Utadd(delta int) {
	atomic.AddInt64(&a.Userns, int64(delta))
}

/// Systadd adds delta nanoseconds to the system-time counter.
///
/// @param delta Amount to add in nanoseconds.
func (a *Accnt_t) Systadd(delta int) {
	atomic.AddInt64(&a.Sysns, int64(delta))
}

/// Now returns the current time in nanoseconds.
///
/// @return Current time since Unix epoch in nanoseconds.
func (a *Accnt_t) Now() int {
	return int(time.Now().UnixNano())
}

/// Io_time removes time spent waiting for I/O from system time.
///
/// @param since Timestamp when the I/O wait began, in nanoseconds.
func (a *Accnt_t) Io_time(since int) {
	d := a.Now() - since
	a.Systadd(-d)
}

/// Sleep_time removes time spent sleeping from system time.
///
/// @param since Timestamp when the sleep began, in nanoseconds.
func (a *Accnt_t) Sleep_time(since int) {
	d := a.Now() - since
	a.Systadd(-d)
}

/// Finish finalizes accounting by adding time since @p inttime to system time.
///
/// @param inttime Start time for measuring final system usage in nanoseconds.
func (a *Accnt_t) Finish(inttime int) {
	a.Systadd(a.Now() - inttime)
}

/// Add merges another accounting record into this one.
///
/// @param n Record to merge.
func (a *Accnt_t) Add(n *Accnt_t) {
	a.Lock()
	a.Userns += n.Userns
	a.Sysns += n.Sysns
	a.Unlock()
}

/// Fetch returns a snapshot of the accounting information encoded as rusage.
///
/// This method locks the structure to produce a consistent view.
///
/// @return Serialized rusage structure.
func (a *Accnt_t) Fetch() []uint8 {
	a.Lock()
	ru := a.To_rusage()
	a.Unlock()
	return ru
}

/// To_rusage converts the accounting data into a byte slice formatted as an
/// rusage structure.
///
/// @return Byte slice containing user and system usage suitable for copying to
///         userspace.
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
