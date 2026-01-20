// SPDX-License-Identifier: EPL-2.0

// Package testutil provides helper functions for tests that handle errors
// appropriately, reducing boilerplate and ensuring consistent error handling.
package testutil

import (
	"io"
	"os"
	"testing"
)

// Stopper is an interface for types that have a Stop method returning an error.
// This is commonly used for server types.
type Stopper interface {
	Stop() error
}

// MustChdir changes the current working directory to dir.
// It returns a cleanup function that restores the original directory.
// The test fails immediately if the directory change fails.
func MustChdir(t testing.TB, dir string) func() {
	t.Helper()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change directory to %s: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore directory to %s: %v", originalWd, err)
		}
	}
}

// MustSetenv sets the environment variable key to value.
// It returns a cleanup function that restores the original value (or unsets it).
// The test fails immediately if the operation fails.
func MustSetenv(t testing.TB, key, value string) func() {
	t.Helper()
	originalValue, hadValue := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	return func() {
		if hadValue {
			if err := os.Setenv(key, originalValue); err != nil {
				t.Errorf("failed to restore env %s: %v", key, err)
			}
		} else {
			if err := os.Unsetenv(key); err != nil {
				t.Errorf("failed to unset env %s: %v", key, err)
			}
		}
	}
}

// MustUnsetenv unsets the environment variable key.
// It returns a cleanup function that restores the original value (if any).
// The test fails immediately if the operation fails.
func MustUnsetenv(t testing.TB, key string) func() {
	t.Helper()
	originalValue, hadValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
	return func() {
		if hadValue {
			if err := os.Setenv(key, originalValue); err != nil {
				t.Errorf("failed to restore env %s: %v", key, err)
			}
		}
	}
}

// MustMkdirAll creates a directory along with any necessary parents.
// The test fails immediately if the operation fails.
func MustMkdirAll(t testing.TB, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, perm); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

// MustRemoveAll removes path and any children it contains.
// Unlike other Must* functions, this logs errors but doesn't fail the test,
// as cleanup failures are typically non-fatal.
func MustRemoveAll(t testing.TB, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Logf("warning: failed to remove %s: %v", path, err)
	}
}

// MustClose closes the given io.Closer.
// The test fails immediately if the close fails.
func MustClose(t testing.TB, c io.Closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}
}

// MustStop stops the given Stopper (typically a server).
// Unlike MustClose, this logs errors but doesn't fail the test,
// as shutdown errors during cleanup are typically non-fatal.
func MustStop(t testing.TB, s Stopper) {
	t.Helper()
	if err := s.Stop(); err != nil {
		t.Logf("warning: stop returned error: %v", err)
	}
}

// DeferClose returns a cleanup function that closes the given io.Closer,
// logging any errors. Useful for defer statements in tests.
func DeferClose(t testing.TB, c io.Closer) func() {
	t.Helper()
	return func() {
		t.Helper()
		if err := c.Close(); err != nil {
			t.Logf("warning: close returned error: %v", err)
		}
	}
}

// DeferStop returns a cleanup function that stops the given Stopper,
// logging any errors. Useful for defer statements in tests.
func DeferStop(t testing.TB, s Stopper) func() {
	t.Helper()
	return func() {
		t.Helper()
		if err := s.Stop(); err != nil {
			t.Logf("warning: stop returned error: %v", err)
		}
	}
}
