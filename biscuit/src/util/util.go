// Package util contains helper functions used across the kernel.
package util

import "unsafe"

// Int is satisfied by all built-in integer types.
type Int interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Min returns the smaller of a and b.
func Min[T Int](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Rounddown aligns v down to the nearest multiple of b.
func Rounddown[T Int](v, b T) T {
	return v - (v % b)
}

// Roundup aligns v up to the nearest multiple of b.
func Roundup[T Int](v, b T) T {
	return Rounddown(v+b-1, b)
}

// Readn reads n bytes from a starting at off and returns the value.
// It panics if the requested region is out of bounds or the size is unsupported.
func Readn(a []uint8, n int, off int) int {
	if off < 0 || off+n > len(a) {
		panic("Readn out of bounds")
	}
	p := unsafe.Pointer(&a[off])
	var ret int
	switch n {
	case 8:
		ret = *(*int)(p)
	case 4:
		ret = int(*(*uint32)(p))
	case 2:
		ret = int(*(*uint16)(p))
	case 1:
		ret = int(*(*uint8)(p))
	default:
		panic("unsupported size")
	}
	return ret
}

// Writen writes val using sz bytes into a starting at off.
// It panics if the destination is out of bounds or the size is unsupported.
func Writen(a []uint8, sz int, off int, val int) {
	if off < 0 || off+sz > len(a) {
		panic("Writen out of bounds")
	}
	p := unsafe.Pointer(&a[off])
	switch sz {
	case 8:
		*(*int)(p) = val
	case 4:
		*(*uint32)(p) = uint32(val)
	case 2:
		*(*uint16)(p) = uint16(val)
	case 1:
		*(*uint8)(p) = uint8(val)
	default:
		panic("unsupported size")
	}
}
