// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	t.Parallel()

	t.Run("creates_new_file", func(t *testing.T) {
		t.Parallel()

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
	})

	t.Run("overwrites_existing_file", func(t *testing.T) {
		t.Parallel()

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
	})

	t.Run("no_stale_temp_on_success", func(t *testing.T) {
		t.Parallel()

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
	})

	t.Run("error_on_nonexistent_directory", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "nonexistent", "test.txt")

		err := AtomicWriteFile(path, []byte("data"), 0o644)
		if err == nil {
			t.Fatal("expected error for nonexistent directory, got nil")
		}
	})
}
