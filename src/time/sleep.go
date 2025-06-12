// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// / Sleep pauses the current goroutine for at least the provided duration.
// / \param d Minimum time to sleep.
// / A negative or zero duration causes Sleep to return immediately.
func Sleep(d Duration)

// runtimeNano returns the current value of the runtime clock in nanoseconds.
func runtimeNano() int64

// Interface to timers implemented in package runtime.
// Must be in sync with ../runtime/time.go:/^type timer
type runtimeTimer struct {
	tb uintptr
	i  int

	when   int64
	period int64
	f      func(interface{}, uintptr) // NOTE: must not be closure
	arg    interface{}
	seq    uintptr
}

// when is a helper function for setting the 'when' field of a runtimeTimer.
// It returns what the time will be, in nanoseconds, Duration d in the future.
// If d is negative, it is ignored. If the returned value would be less than
// zero because of an overflow, MaxInt64 is returned.
func when(d Duration) int64 {
	if d <= 0 {
		return runtimeNano()
	}
	t := runtimeNano() + int64(d)
	if t < 0 {
		t = 1<<63 - 1 // math.MaxInt64
	}
	return t
}

func startTimer(*runtimeTimer)
func stopTimer(*runtimeTimer) bool

// / Timer represents a single event that fires in the future.
// / When the Timer expires, the current time will be sent on C
// / unless the Timer was created by AfterFunc. A Timer must be
// / created with NewTimer or AfterFunc.
type Timer struct {
	C <-chan Time
	r runtimeTimer
}

/**
 * Stop prevents the Timer from firing.
 * \return true if the call stops the timer, false if the timer has already
 * expired or been stopped. Stop does not close the channel to avoid
 * incorrectly succeeding reads.
 *
 * To prevent a timer created with NewTimer from firing after a call to Stop,
 * check the return value and drain the channel. Example:
 * \code
 * if !t.Stop() {
 *     <-t.C
 * }
 * \endcode
 * This cannot be done concurrently with other receives from the Timer's channel.
 *
 * For a timer created with AfterFunc(d, f), if Stop returns false then the
 * timer has already expired and the function has been started in its own
 * goroutine; Stop does not wait for f to complete. If the caller needs to know
 * whether f is completed, it must coordinate with f explicitly.
 */
func (t *Timer) Stop() bool {
	if t.r.f == nil {
		panic("time: Stop called on uninitialized Timer")
	}
	return stopTimer(&t.r)
}

// / NewTimer creates a new Timer that will send the current time on its
// / channel after at least duration d.
// / \param d Time delay before the timer fires.
// / \return Pointer to the newly created Timer.
func NewTimer(d Duration) *Timer {
	c := make(chan Time, 1)
	t := &Timer{
		C: c,
		r: runtimeTimer{
			when: when(d),
			f:    sendTime,
			arg:  c,
		},
	}
	startTimer(&t.r)
	return t
}

/**
 * Reset changes the timer to expire after duration d.
 * \param d New duration for the timer.
 * \return true if the timer had been active, false if it had expired or been stopped.
 *
 * Resetting a timer must take care not to race with the send into t.C that
 * happens when the current timer expires. If a program has already received
 * from t.C, the timer is known to have expired and Reset can be used directly.
 * Otherwise the timer must be stopped and, if Stop reports that the timer
 * expired before being stopped, the channel must be drained:
 * \code
 * if !t.Stop() {
 *     <-t.C
 * }
 * t.Reset(d)
 * \endcode
 * This should not be done concurrently with other receives from the Timer's channel.
 *
 * Note that it is not possible to use Reset's return value correctly because
 * there is a race condition between draining the channel and the new timer
 * expiring. Reset should always be invoked on stopped or expired timers as described above.
 * The return value exists to preserve compatibility with existing programs.
 */
func (t *Timer) Reset(d Duration) bool {
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Timer")
	}
	w := when(d)
	active := stopTimer(&t.r)
	t.r.when = w
	startTimer(&t.r)
	return active
}

func sendTime(c interface{}, seq uintptr) {
	// Non-blocking send of time on c.
	// Used in NewTimer, it cannot block anyway (buffer).
	// Used in NewTicker, dropping sends on the floor is
	// the desired behavior when the reader gets behind,
	// because the sends are periodic.
	select {
	case c.(chan Time) <- Now():
	default:
	}
}

/**
 * After waits for the duration to elapse and then sends the current time on
 * the returned channel. It is equivalent to NewTimer(d).C. The underlying
 * Timer is not recovered by the garbage collector until the timer fires. If
 * efficiency is a concern, use NewTimer instead and call Timer.Stop if the timer
 * is no longer needed.
 */
func After(d Duration) <-chan Time {
	return NewTimer(d).C
}

/**
 * AfterFunc waits for the duration to elapse and then calls f in its own
 * goroutine.
 * \param d Duration to wait before calling f.
 * \param f Function to invoke.
 * \return Timer that can be used to cancel the call using its Stop method.
 */
func AfterFunc(d Duration, f func()) *Timer {
	t := &Timer{
		r: runtimeTimer{
			when: when(d),
			f:    goFunc,
			arg:  f,
		},
	}
	startTimer(&t.r)
	return t
}

func goFunc(arg interface{}, seq uintptr) {
	go arg.(func())()
}
