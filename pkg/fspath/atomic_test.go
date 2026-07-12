// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingAtomicTempFile struct {
	name       string
	writeErr   error
	closeErr   error
	writeCalls int
	closeCalls int
}

func TestAtomicWriteFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "creates_new_file", run: func(t *testing.T) {
			t.Helper()

			dir := t.TempDir()
			path := filepath.Join(dir, "test.txt")

			if err := AtomicWriteFile(path, []byte("hello"), 0o644); err != nil {
				t.Fatalf("AtomicWriteFile() error: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error: %v", err)
			}
			if string(data) != "hello" {
				t.Errorf("content = %q, want %q", string(data), "hello")
			}
		}},
		{name: "overwrites_existing_file", run: func(t *testing.T) {
			t.Helper()

			dir := t.TempDir()
			path := filepath.Join(dir, "test.txt")

			if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}
			if err := AtomicWriteFile(path, []byte("new"), 0o644); err != nil {
				t.Fatalf("AtomicWriteFile() error: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error: %v", err)
			}
			if string(data) != "new" {
				t.Errorf("content = %q, want %q", string(data), "new")
			}
		}},
		{name: "no_stale_temp_on_success", run: func(t *testing.T) {
			t.Helper()

			dir := t.TempDir()
			path := filepath.Join(dir, "test.txt")

			if err := AtomicWriteFile(path, []byte("content"), 0o644); err != nil {
				t.Fatalf("AtomicWriteFile() error: %v", err)
			}

			// After success, no .tmp files should remain.
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir() error: %v", err)
			}
			for _, e := range entries {
				if e.Name() != "test.txt" {
					t.Errorf("unexpected file %q remaining after atomic write", e.Name())
				}
			}
		}},
		{name: "error_on_nonexistent_directory", run: func(t *testing.T) {
			t.Helper()

			path := filepath.Join(t.TempDir(), "nonexistent", "test.txt")

			err := AtomicWriteFile(path, []byte("data"), 0o644)
			if err == nil {
				t.Fatal("expected error for nonexistent directory, got nil")
			}
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("AtomicWriteFile() error = %v, want os.ErrNotExist wrapping", err)
			}
		}},
		{name: "error_when_target_is_existing_directory", run: func(t *testing.T) {
			t.Helper()

			dir := t.TempDir()
			path := filepath.Join(dir, "target")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("Mkdir() error: %v", err)
			}

			err := AtomicWriteFile(path, []byte("data"), 0o644)
			if err == nil {
				t.Fatal("expected error for existing directory target, got nil")
			}
			if !strings.Contains(err.Error(), "renaming temporary file") {
				t.Fatalf("AtomicWriteFile() error = %v, want rename context", err)
			}

			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("Stat() error: %v", statErr)
			}
			if !info.IsDir() {
				t.Fatal("target IsDir() = false, want directory preserved")
			}

			entries, readErr := os.ReadDir(dir)
			if readErr != nil {
				t.Fatalf("ReadDir() error: %v", readErr)
			}
			if len(entries) != 1 || entries[0].Name() != "target" {
				t.Fatalf("directory entries after failed write = %#v, want only target directory", entries)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestAtomicWriteFileOperationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		operation      string
		wantContext    string
		wantWriteCalls int
	}{
		{name: "chmod_error", operation: "chmod", wantContext: "setting temporary file permissions"},
		{name: "write_error", operation: "write", wantContext: "writing temporary file", wantWriteCalls: 1},
		{name: "close_error", operation: "close", wantContext: "closing temporary file", wantWriteCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cause := errors.New(tt.operation + " failed")
			file := &recordingAtomicTempFile{name: filepath.Join(t.TempDir(), "target.tmp")}
			var chmodErr error
			switch tt.operation {
			case "chmod":
				chmodErr = cause
			case "write":
				file.writeErr = cause
			case "close":
				file.closeErr = cause
			}
			removed, renamed := newAtomicWriteRecording()
			err := atomicWriteFile(filepath.Join(t.TempDir(), "target.txt"), []byte("payload"), 0o600, recordingAtomicWriteOps(file, chmodErr, removed, renamed))
			requireAtomicWriteOperationError(t, err, cause, tt.wantContext)
			if file.writeCalls != tt.wantWriteCalls {
				t.Fatalf("Write calls = %d, want %d", file.writeCalls, tt.wantWriteCalls)
			}
			if file.closeCalls != 1 {
				t.Fatalf("Close calls = %d, want 1", file.closeCalls)
			}
			requireAtomicWriteCleanup(t, file.name, *removed, *renamed)
		})
	}
}

func (f *recordingAtomicTempFile) Name() string {
	return f.name
}

func (f *recordingAtomicTempFile) Write(data []byte) (int, error) {
	f.writeCalls++
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(data), nil
}

func (f *recordingAtomicTempFile) Close() error {
	f.closeCalls++
	return f.closeErr
}

func newAtomicWriteRecording() (removed *[]string, renamed *bool) {
	removedPaths := []string{}
	renamedValue := false
	return &removedPaths, &renamedValue
}

func recordingAtomicWriteOps(file *recordingAtomicTempFile, chmodErr error, removed *[]string, renamed *bool) atomicWriteOps {
	return atomicWriteOps{
		createTemp: func(string, string) (atomicTempFile, error) {
			return file, nil
		},
		chmod: func(string, os.FileMode) error {
			return chmodErr
		},
		rename: func(string, string) error {
			*renamed = true
			return nil
		},
		remove: func(path string) error {
			*removed = append(*removed, path)
			return nil
		},
	}
}

func requireAtomicWriteOperationError(t *testing.T, err, cause error, wantContext string) {
	t.Helper()

	if err == nil {
		t.Fatalf("atomicWriteFile() error = nil, want %q", wantContext)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("atomicWriteFile() error = %v, want wrapping %v", err, cause)
	}
	if !strings.Contains(err.Error(), wantContext) {
		t.Fatalf("atomicWriteFile() error = %v, want context %q", err, wantContext)
	}
}

func requireAtomicWriteCleanup(t *testing.T, tmpPath string, removed []string, renamed bool) {
	t.Helper()

	if renamed {
		t.Fatal("rename called on failed atomic write")
	}
	if len(removed) != 1 || removed[0] != tmpPath {
		t.Fatalf("removed paths = %v, want [%s]", removed, tmpPath)
	}
}
