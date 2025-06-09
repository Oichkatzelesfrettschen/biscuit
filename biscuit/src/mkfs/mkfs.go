package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"biscuit/biscuit/src/fs"
	"biscuit/biscuit/src/ufs"
	"biscuit/biscuit/src/ustr"
)

// Constants describing the layout of the created filesystem.
const (
	nlogblks   = 1024 // number of log blocks
	ninodeblks = 100 * 50
	ndatablks  = 40000
)

// copydata reads the file at `src` and appends its contents to `dst` in the
// provided filesystem.
//
// \param src path to the source file on the host
// \param f   filesystem handle obtained from ufs.BootFS
// \param dst destination path within the image
func copydata(src string, f *ufs.Ufs_t, dst string) {
	srcFile, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer srcFile.Close()

	buf := make([]byte, fs.BSIZE)
	for {
		n, readErr := srcFile.Read(buf)
		if readErr != nil && readErr != io.EOF {
			panic(readErr)
		}
		if n == 0 {
			break
		}
		chunk := ufs.MkBuf(buf[:n])
		f.Append(ustr.Ustr(dst), chunk)
		if readErr == io.EOF {
			break
		}
	}
}

// addfiles walks `skeldir` on the host and replicates its contents into the
// filesystem `fs`.
//
// \param fs       target filesystem
// \param skeldir  host directory tree to copy
func addfiles(fs *ufs.Ufs_t, skeldir string) {
	err := filepath.WalkDir(skeldir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("failed to access %q: %v\n", path, err)
			return err
		}

		rel := strings.TrimPrefix(path, skeldir)
		if rel == "" {
			return nil
		}

		if d.IsDir() {
			if e := fs.MkDir(ustr.Ustr(rel)); e != 0 {
				fmt.Printf("failed to create dir %v\n", rel)
			}
			return nil
		}

		if e := fs.MkFile(ustr.Ustr(rel), nil); e != 0 {
			fmt.Printf("failed to create file %v\n", rel)
		}
		copydata(path, fs, rel)
		return nil
	})

	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", skeldir, err)
		os.Exit(1)
	}
}

// main is the entry point for the mkfs utility. It creates a bootable disk
// image composed of the bootloader, kernel, and a skeletal filesystem.
func main() {
	if len(os.Args) < 5 {
		fmt.Printf("Usage: mkfs <bootimage> <kernel image> <output image> <skel dir>\n")
		os.Exit(1)
	}

	image := os.Args[3]
	inputs := []string{os.Args[1], os.Args[2]}
	ufs.MkDisk(image, inputs, nlogblks, ninodeblks, ndatablks)

	fs := ufs.BootFS(image)
	if _, err := fs.Stat(ustr.MkUstrRoot()); err != 0 {
		fmt.Printf("not a valid fs: no root inode\n")
		os.Exit(1)
	}

	addfiles(fs, os.Args[4])

	ufs.ShutdownFS(fs)
}
