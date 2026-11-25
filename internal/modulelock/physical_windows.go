// SPDX-License-Identifier: MPL-2.0

//go:build windows

package modulelock

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// GetFinalPathNameByHandle flags (fileapi.h), which golang.org/x/sys/windows
// does not define: a normalized name with a drive letter.
const (
	fileNameNormalized = 0x0
	volumeNameDOS      = 0x0
)

// physicalDir opens the directory and asks Windows for its final path, which
// follows junctions and symbolic links (filepath.EvalSymlinks does not follow
// junctions on Go 1.23+, formal/README.md F18) and gives the on-disk case and
// long names. It opens through os.Open, which opens a directory with
// FILE_FLAG_BACKUP_SEMANTICS and adds the \\?\ prefix to a path longer than
// MAX_PATH, so a deep project directory locks like any other.
//
//goplint:ignore -- private helper over raw OS path text.
func physicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only directory handle; nothing to flush.
	h := windows.Handle(f.Fd())

	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), fileNameNormalized|volumeNameDOS)
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			return trimFinalPathPrefix(windows.UTF16ToString(buf[:n])), nil
		}
		buf = make([]uint16, n+1)
	}
}
