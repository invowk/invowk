// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultFilePerm is the default permission mode for files created by AtomicWriteFile.
// Standard user-readable/writable file with group/other read access.
const DefaultFilePerm os.FileMode = 0o644

type (
	atomicTempFile interface {
		Name() string
		Write([]byte) (int, error)
		Sync() error
		Close() error
	}

	atomicWriteOps struct {
		createTemp func(string, string) (atomicTempFile, error)
		chmod      func(string, os.FileMode) error
		rename     func(string, string) error
		remove     func(string) error
		syncDir    func(string) error
	}
)

// AtomicWriteFile writes data to a file atomically using a temporary file and
// rename. The temp file is created in the same directory as the target using
// os.CreateTemp with an unpredictable name suffix, preventing symlink-race
// attacks where an attacker pre-creates a predictable temp path as a symlink
// to redirect writes to an arbitrary location.
//
// The rename operation is atomic on POSIX systems, ensuring readers see either
// the old content or the new content, never a partial write. The temp file is
// fsynced before the rename and the directory after it, so after power loss
// the file holds the complete old or new content, and a returned nil means the
// new content is durable (formal/tla/AtomicWrite.tla, finding F3). If the
// directory fsync fails, the new content is already in place and the error
// reports that its durability is unknown.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return atomicWriteFile(path, data, perm, defaultAtomicWriteOps())
}

//goplint:ignore -- private helper mirrors os.WriteFile-style primitives for injectable filesystem ops.
func atomicWriteFile(path string, data []byte, perm os.FileMode, ops atomicWriteOps) (err error) {
	dir := filepath.Dir(path)

	tmp, err := ops.createTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	renamed := false

	// Clean up the temp file on any error path before the rename; after it,
	// the temp name no longer exists.
	defer func() {
		if err != nil && !renamed {
			err = errors.Join(err, ops.remove(tmpPath))
		}
	}()

	if err = ops.chmod(tmpPath, perm); err != nil {
		return errors.Join(fmt.Errorf("setting temporary file permissions: %w", err), tmp.Close())
	}

	if _, err = tmp.Write(data); err != nil {
		return errors.Join(fmt.Errorf("writing temporary file: %w", err), tmp.Close())
	}

	if err = tmp.Sync(); err != nil {
		return errors.Join(fmt.Errorf("syncing temporary file: %w", err), tmp.Close())
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}

	if err = ops.rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temporary file: %w", err)
	}
	renamed = true

	if err = ops.syncDir(dir); err != nil {
		return fmt.Errorf("syncing directory after rename (new content in place, durability unknown): %w", err)
	}

	return nil
}

// syncDirectory fsyncs a directory so a rename inside it survives power loss.
// Windows cannot open a directory for fsync; NTFS journals the rename itself.
//
//goplint:ignore -- private helper mirrors os.Open-style primitives for directory fsync.
func syncDirectory(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("opening directory: %w", err)
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return fmt.Errorf("fsync directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing directory: %w", closeErr)
	}
	return nil
}

func defaultAtomicWriteOps() atomicWriteOps {
	return atomicWriteOps{
		createTemp: func(dir, pattern string) (atomicTempFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		chmod:   os.Chmod,
		rename:  os.Rename,
		remove:  os.Remove,
		syncDir: syncDirectory,
	}
}
