// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultFilePerm is the default permission mode for files created by AtomicWriteFile.
// Standard user-readable/writable file with group/other read access.
const DefaultFilePerm os.FileMode = 0o644

// AtomicWriteFile writes data to a file atomically using a temporary file and
// rename. The temp file is created in the same directory as the target using
// os.CreateTemp with an unpredictable name suffix, preventing symlink-race
// attacks where an attacker pre-creates a predictable temp path as a symlink
// to redirect writes to an arbitrary location.
//
// The rename operation is atomic on POSIX systems, ensuring readers see either
// the old content or the new content, never a partial write.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up the temp file on any error path.
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath) // Best-effort cleanup
		}
	}()

	if err = os.Chmod(tmpPath, perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting temporary file permissions: %w", err)
	}

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temporary file: %w", err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}

	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temporary file: %w", err)
	}

	return nil
}
