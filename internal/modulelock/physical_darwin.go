// SPDX-License-Identifier: MPL-2.0

//go:build darwin

package modulelock

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

// physicalDir opens the directory and asks Darwin's kernel for its canonical
// path via F_GETPATH, which resolves symbolic links, APFS firmlinks, and
// on-disk letter case on case-insensitive volumes without falling back to
// case-preserving filepath.EvalSymlinks.
//
//goplint:ignore -- private helper over raw OS path text.
func physicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}

	fd, err := openDarwinPathReadOnly(abs)
	if err != nil {
		return "", fmt.Errorf("open directory %s: %w", abs, err)
	}
	defer func() {
		_ = unix.Close(fd) //nolint:errcheck // read-only directory descriptor; nothing to flush.
	}()

	var buf [unix.PathMax]byte
	for {
		//nolint:gosec // G103: unsafe.Pointer is required by Darwin F_GETPATH fcntl buffer semantics.
		_, _, errno := unix.Syscall(unix.SYS_FCNTL, uintptr(fd), unix.F_GETPATH, uintptr(unsafe.Pointer(&buf[0])))
		if errno == unix.EINTR {
			continue
		}
		if errno != 0 {
			return "", fmt.Errorf("resolve canonical path via F_GETPATH for %s: %w", abs, errno)
		}
		break
	}

	nul := bytes.IndexByte(buf[:], 0)
	if nul < 0 {
		return "", fmt.Errorf("F_GETPATH returned unterminated path for %s", abs)
	}
	if nul == 0 {
		return "", fmt.Errorf("F_GETPATH returned empty path for %s", abs)
	}
	return string(buf[:nul]), nil
}

func openDarwinPathReadOnly(abs string) (int, error) {
	for {
		fd, err := unix.Open(abs, unix.O_RDONLY|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return fd, err
	}
}
