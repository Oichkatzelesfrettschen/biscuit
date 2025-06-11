package ustr

/// Ustr represents an immutable path or string used by the kernel.
type Ustr []uint8

/// Isdot reports whether the string equals '.'.
func (us Ustr) Isdot() bool {
	return len(us) == 1 && us[0] == '.'
}

/// Isdotdot reports whether the string equals '..'.
func (us Ustr) Isdotdot() bool {
	return len(us) == 2 && us[0] == '.' && us[1] == '.'
}

/// Eq compares two Ustr values for equality.
/// 
/// \param s other Ustr to compare
/// \return true when both strings contain identical bytes.
func (us Ustr) Eq(s Ustr) bool {
	if len(us) != len(s) {
		return false
	}
	for i, v := range us {
		if v != s[i] {
			return false
		}
	}
	return true
}

/// MkUstr creates an empty Ustr value.
/// \return newly created Ustr.
func MkUstr() Ustr {
	us := Ustr{}
	return us
}

/// MkUstrDot returns a Ustr representing '.'.
/// \return new Ustr for the current directory.
func MkUstrDot() Ustr {
	us := Ustr(".")
	return us
}

/// MkUstrRoot returns a Ustr for the root directory '/'.
/// \return root Ustr value.
func MkUstrRoot() Ustr {
	us := Ustr("/")
	return us
}

/// DotDot is a reusable Ustr containing "..".
var DotDot = Ustr{'.', '.'}

/// MkUstrSlice converts a NUL-terminated byte slice to a Ustr.
/// 
/// \param buf source byte slice
/// \return slice truncated at the first NUL byte.
func MkUstrSlice(buf []uint8) Ustr {
	for i := 0; i < len(buf); i++ {
		if buf[i] == uint8(0) {
			return buf[:i]
		}
	}
	return buf
}

/// Extend appends '/' and p to the current Ustr and returns the result.
/// 
/// \param p path component to add
/// \return new Ustr with p appended.
func (us Ustr) Extend(p Ustr) Ustr {
	tmp := make(Ustr, len(us))
	copy(tmp, us)
	r := append(tmp, '/')
	return append(r, p...)
}

/// ExtendStr appends '/' and the string p to the current Ustr.
/// \param p component as string
/// \return new Ustr with p appended.
func (us Ustr) ExtendStr(p string) Ustr {
	return us.Extend(Ustr(p))
}

/// IsAbsolute reports whether the path begins with '/'.
func (us Ustr) IsAbsolute() bool {
	if len(us) == 0 {
		return false
	}
	return us[0] == '/'
}

/// IndexByte returns the index of b in the string or -1 if not present.
/// \param b byte to search for
/// \return index of b or -1.
func (us Ustr) IndexByte(b uint8) int {
	for i, v := range us {
		if v == b {
			return i
		}
	}
	return -1
}

/// String converts the Ustr to a Go string.
/// \return string representation of the Ustr.
func (us Ustr) String() string {
	return string(us)
}
