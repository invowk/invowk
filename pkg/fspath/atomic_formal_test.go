// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	errInjected = errors.New("injected failure")

	// atomicWriteSteps are the steps a failure can be injected at ("none": no failure).
	atomicWriteSteps = []string{"none", "create", "chmod", "write", "sync", "close", "rename", "syncdir"}
)

// failingTempFile wraps a real temp file and fails Write or Close on demand.
type failingTempFile struct {
	*os.File
	failWrite bool
	failSync  bool
	failClose bool
}

func (f *failingTempFile) Sync() error {
	if f.failSync {
		return errInjected
	}
	if err := f.File.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	return nil
}

func (f *failingTempFile) Write(data []byte) (int, error) {
	if f.failWrite {
		return 0, errInjected
	}
	return f.File.Write(data)
}

func (f *failingTempFile) Close() error {
	closeErr := f.File.Close()
	if f.failClose {
		return errInjected
	}
	return closeErr
}

// injectingAtomicWriteOps runs the real filesystem operations, fails the one
// named by failAt, fails cleanup when removeFails, and calls observe after the
// temp file is created and after the rename.
func injectingAtomicWriteOps(failAt string, removeFails bool, observe func()) atomicWriteOps {
	if observe == nil {
		observe = func() {}
	}
	return atomicWriteOps{
		createTemp: func(d, pattern string) (atomicTempFile, error) {
			if failAt == "create" {
				return nil, errInjected
			}
			file, err := os.CreateTemp(d, pattern)
			if err != nil {
				return nil, fmt.Errorf("create temp: %w", err)
			}
			observe()
			return &failingTempFile{File: file, failWrite: failAt == "write", failSync: failAt == "sync", failClose: failAt == "close"}, nil
		},
		chmod: func(name string, mode os.FileMode) error {
			if failAt == "chmod" {
				return errInjected
			}
			return os.Chmod(name, mode)
		},
		rename: func(from, to string) error {
			if failAt == "rename" {
				return errInjected
			}
			if err := os.Rename(from, to); err != nil {
				return fmt.Errorf("rename: %w", err)
			}
			observe()
			return nil
		},
		remove: func(name string) error {
			if removeFails {
				return errInjected
			}
			return os.Remove(name)
		},
		syncDir: func(dir string) error {
			if failAt == "syncdir" {
				return errInjected
			}
			return syncDirectory(dir)
		},
	}
}

// tempFilesIn lists the atomic-write temp files left in dir.
func tempFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	var temps []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			temps = append(temps, entry.Name())
		}
	}
	return temps
}

// TestAtomicWriteFile_FailureAtEveryStep binds formal/tla/AtomicWrite.tla to
// the real filesystem. For a failure injected at every step (and a failing
// cleanup), a returned error leaves the target's old content intact and no temp
// file behind unless the cleanup error is joined into the result
// (TempRemovedOnError, ReadersNeverSeePartial); success leaves the new content.
func TestAtomicWriteFile_FailureAtEveryStep(t *testing.T) {
	t.Parallel()

	for _, failAt := range atomicWriteSteps {
		for _, removeFails := range []bool{false, true} {
			name := failAt
			if removeFails {
				name += "/remove-fails"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				dir := t.TempDir()
				target := filepath.Join(dir, "invowkmod.lock.cue")
				if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				ops := injectingAtomicWriteOps(failAt, removeFails, nil)

				err := atomicWriteFile(target, []byte("new"), DefaultFilePerm, ops)

				content, readErr := os.ReadFile(target)
				if readErr != nil {
					t.Fatalf("ReadFile(target) error = %v", readErr)
				}
				temps := tempFilesIn(t, dir)

				if failAt == "none" {
					if err != nil || string(content) != "new" || len(temps) != 0 {
						t.Fatalf("success: err=%v content=%q temps=%v", err, content, temps)
					}
					return
				}
				if !errors.Is(err, errInjected) {
					t.Fatalf("error = %v, want the injected failure", err)
				}
				// A failed directory fsync happens after the rename: the new
				// content is in place and only its durability is unknown.
				want := "old"
				if failAt == "syncdir" {
					want = "new"
				}
				if string(content) != want {
					t.Fatalf("ReadersNeverSeePartial: target content %q after a failed write at %s, want %q", content, failAt, want)
				}
				leftTemp := len(temps) > 0
				if leftTemp && !removeFails {
					t.Fatalf("TempRemovedOnError: temp files %v left behind", temps)
				}
			})
		}
	}
}
