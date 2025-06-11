package fd

import "sync"

import "bpath"
import "defs"
import "fdops"
import "ustr"

/// File descriptor permission bits.
const (
	FD_READ    = 0x1 /// read permission
	FD_WRITE   = 0x2 /// write permission
	FD_CLOEXEC = 0x4 /// close-on-exec flag
)

/// Fd_t represents an open file descriptor.
type Fd_t struct {
       // fops is an interface implemented via a "pointer receiver", thus fops
       // is a reference, not a value
       Fops  fdops.Fdops_i /// descriptor operations
       Perms int           /// permission bits
}

/// Copyfd duplicates an open file descriptor by reopening it.
func Copyfd(fd *Fd_t) (*Fd_t, defs.Err_t) {
	nfd := &Fd_t{}
	*nfd = *fd
	err := nfd.Fops.Reopen()
	if err != 0 {
		return nil, err
	}
	return nfd, 0
}

/// Close_panic closes the descriptor and panics on failure.
func Close_panic(f *Fd_t) {
	if f.Fops.Close() != 0 {
		panic("must succeed")
	}
}

/// Cwd_t tracks the current working directory for a process.
type Cwd_t struct {
       sync.Mutex // to serialize chdirs
       Fd   *Fd_t    /// current directory fd
       Path ustr.Ustr /// canonical path
}

/// Fullpath joins cwd with p if p is not already absolute.
func (cwd *Cwd_t) Fullpath(p ustr.Ustr) ustr.Ustr {
	if p.IsAbsolute() {
		return p
	} else {
		full := append(cwd.Path, '/')
		return append(full, p...)
	}
}

/// Canonicalpath resolves path components relative to cwd.
func (cwd *Cwd_t) Canonicalpath(p ustr.Ustr) ustr.Ustr {
	p1 := cwd.Fullpath(p)
	return bpath.Canonicalize(p1)
}

/// MkRootCwd constructs a Cwd_t rooted at "/".
func MkRootCwd(fd *Fd_t) *Cwd_t {
	c := &Cwd_t{}
	c.Fd = fd
	c.Path = ustr.MkUstrRoot()
	return c
}
