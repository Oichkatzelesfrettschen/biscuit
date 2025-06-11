package stats

import "reflect"
import "runtime"
import "sync/atomic"
import "strconv"
import "strings"
import "unsafe"

const Stats = false
const Timing = false

var Nirqs [100]int
var Irqs int

/// Rdtsc returns the current cycle count when enabled.
func Rdtsc() uint64 {
	if Stats {
		return runtime.Rdtsc()
	} else {
		return 0
	}
}

/// Counter_t is a statistical counter.
type Counter_t int64

/// Cycles_t holds a cycle count.
type Cycles_t int64

/// Inc increments the counter.
func (c *Counter_t) Inc() {
	if Stats {
		n := (*int64)(unsafe.Pointer(c))
		atomic.AddInt64(n, 1)
	}
}

/// Add adds elapsed cycles to the counter.
func (c *Cycles_t) Add(m uint64) {
	if Timing {
		n := (*int64)(unsafe.Pointer(c))
		atomic.AddInt64(n, int64(Rdtsc()-m))
	}
}

/// Stats2String converts a struct of counters to a printable string.
func Stats2String(st interface{}) string {
	if !Stats {
		return ""
	}
	v := reflect.ValueOf(st)
	s := ""
	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i).Type().String()
		if strings.HasSuffix(t, "Counter_t") {
			n := v.Field(i).Interface().(Counter_t)
			s += "\n\t#" + v.Type().Field(i).Name + ": " + strconv.FormatInt(int64(n), 10)
		}
		if strings.HasSuffix(t, "Cycles_t") {
			n := v.Field(i).Interface().(Cycles_t)
			s += "\n\t#" + v.Type().Field(i).Name + ": " + strconv.FormatInt(int64(n), 10)
		}

	}
	return s + "\n"
}
