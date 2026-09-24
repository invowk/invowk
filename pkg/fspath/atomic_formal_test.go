// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errInjected = errors.New("injected failure")

// failingTempFile wraps a real temp file and fails Write or Close on demand.
type failingTempFile struct {
	*os.File
	failWrite bool
	failClose bool
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

// TestAtomicWriteFile_FailureAtEveryStep binds formal/tla/AtomicWrite.tla to
// the real filesystem. For a failure injected at every step (and a failing
// cleanup), a returned error leaves the target's old content intact and no temp
// file behind unless the cleanup error is joined into the result
// (TempRemovedOnError, ReadersNeverSeePartial); success leaves the new content.
func TestAtomicWriteFile_FailureAtEveryStep(t *testing.T) {
	t.Parallel()

	steps := []string{"none", "create", "chmod", "write", "close", "rename"}
	for _, failAt := range steps {
		for _, removeFails := range []bool{false, true} {
			t.Run(failAt+map[bool]string{true: "/remove-fails", false: ""}[removeFails], func(t *testing.T) {
				t.Parallel()

				dir := t.TempDir()
				target := filepath.Join(dir, "invowkmod.lock.cue")
				if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				fail := func(step string) error {
					if failAt == step {
						return errInjected
					}
					return nil
				}
				ops := atomicWriteOps{
					createTemp: func(d, pattern string) (atomicTempFile, error) {
						if err := fail("create"); err != nil {
							return nil, err
						}
						file, err := os.CreateTemp(d, pattern)
						if err != nil {
							return nil, err
						}
						return &failingTempFile{File: file, failWrite: failAt == "write", failClose: failAt == "close"}, nil
					},
					chmod: func(name string, mode os.FileMode) error {
						if err := fail("chmod"); err != nil {
							return err
						}
						return os.Chmod(name, mode)
					},
					rename: func(from, to string) error {
						if err := fail("rename"); err != nil {
							return err
						}
						return os.Rename(from, to)
					},
					remove: func(name string) error {
						if removeFails {
							return errInjected
						}
						return os.Remove(name)
					},
				}

				err := atomicWriteFile(target, []byte("new"), DefaultFilePerm, ops)

				content, readErr := os.ReadFile(target)
				if readErr != nil {
					t.Fatalf("ReadFile(target) error = %v", readErr)
				}
				entries, dirErr := os.ReadDir(dir)
				if dirErr != nil {
					t.Fatalf("ReadDir() error = %v", dirErr)
				}
				var temps []string
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".tmp") {
						temps = append(temps, entry.Name())
					}
				}

				if failAt == "none" {
					if err != nil || string(content) != "new" || len(temps) != 0 {
						t.Fatalf("success: err=%v content=%q temps=%v", err, content, temps)
					}
					return
				}
				if !errors.Is(err, errInjected) {
					t.Fatalf("error = %v, want the injected failure", err)
				}
				if string(content) != "old" {
					t.Fatalf("ReadersNeverSeePartial: target content %q after a failed write, want %q", content, "old")
				}
				leftTemp := len(temps) > 0
				if leftTemp && !removeFails {
					t.Fatalf("TempRemovedOnError: temp files %v left behind", temps)
				}
			})
		}
	}
}
