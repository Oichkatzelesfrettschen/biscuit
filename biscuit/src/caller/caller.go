package caller

import (
	"fmt"
	"runtime"
	"sync"
)

// Callerdump prints the call stack starting at the given depth.
//
// Parameters:
//
//	start - stack frame to begin printing.
func Callerdump(start int) {
	i := start
	s := ""
	for {
		_, f, l, ok := runtime.Caller(i)
		if !ok {
			break
		}
		i++
		//li := strings.LastIndex(f, "/")
		//if li != -1 {
		//	f = f[li+1:]
		//}
		if s == "" {
			s = fmt.Sprintf("%s:%d\n", f, l)
		} else {
			s += fmt.Sprintf("\t<-%s:%d\n", f, l)
		}
	}
	fmt.Printf("%s", s)
}

// a type for detecting the first call from each distinct path of ancestor
// callers.
// Distinct_caller_t tracks whether a call chain has been seen before.
// Fields are protected by the embedded mutex.
type Distinct_caller_t struct {
	sync.Mutex
	Enabled bool
	did     map[uintptr]bool
	Whitel  map[string]bool
}

// returns a poor-man's hash of the given RIP values, which is probably unique.
func (dc *Distinct_caller_t) _pchash(pcs []uintptr) uintptr {
	if len(pcs) == 0 {
		panic("d'oh")
	}
	var ret uintptr
	for _, pc := range pcs {
		pc = pc*1103515245 + 12345
		ret ^= pc
	}
	return ret
}

// Len returns the number of unique caller paths recorded.
func (dc *Distinct_caller_t) Len() int {
	dc.Lock()
	ret := len(dc.did)
	dc.Unlock()
	return ret
}

// Distinct reports whether the current call chain is new.
// It returns true along with a formatted stack trace when not seen before.
func (dc *Distinct_caller_t) Distinct() (bool, string) {
	dc.Lock()
	defer dc.Unlock()
	if !dc.Enabled {
		return false, ""
	}

	if dc.did == nil {
		dc.did = make(map[uintptr]bool)
	}

	var pcs []uintptr
	for sz, got := 30, 30; got >= sz; sz *= 2 {
		pcs = make([]uintptr, 30)
		got = runtime.Callers(3, pcs)
		if got == 0 {
			panic("no")
		}
	}
	h := dc._pchash(pcs)
	if ok := dc.did[h]; !ok {
		dc.did[h] = true
		frames := runtime.CallersFrames(pcs)
		fs := ""
		// check for white-listed caller
		for {
			fr, more := frames.Next()
			if ok := dc.Whitel[fr.Function]; ok {
				return false, ""
			}
			if fs == "" {
				fs = fmt.Sprintf("%v (%v:%v)\n", fr.Function,
					fr.File, fr.Line)
			} else {
				fs += fmt.Sprintf("\t%v (%v:%v)\n", fr.Function,
					fr.File, fr.Line)
			}
			if !more || fr.Function == "runtime.goexit" {
				break
			}
		}
		return true, fs
	}
	return false, ""
}
