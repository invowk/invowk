// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestInvalidValueErrorsPreserveInput(t *testing.T) {
	t.Parallel()

	t.Run("description text", testInvalidDescriptionTextPreservesInput)
	t.Run("exit code", testInvalidExitCodePreservesInput)
	t.Run("filesystem path", testInvalidFilesystemPathPreservesInput)
	t.Run("listen port", testInvalidListenPortPreservesInput)
	t.Run("runtime mode", testInvalidRuntimeModePreservesInput)
	t.Run("shell path", testInvalidShellPathPreservesInput)
}

func testInvalidDescriptionTextPreservesInput(t *testing.T) {
	t.Parallel()

	value := DescriptionText(" \t ")
	err := value.Validate()
	if !errors.Is(err, ErrInvalidDescriptionText) {
		t.Fatalf("Validate() error = %v, want ErrInvalidDescriptionText", err)
	}
	var valueErr *InvalidDescriptionTextError
	if !errors.As(err, &valueErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidDescriptionTextError", err)
	}
	if valueErr.Value != value {
		t.Fatalf("InvalidDescriptionTextError.Value = %q, want %q", valueErr.Value, value)
	}
}

func testInvalidExitCodePreservesInput(t *testing.T) {
	t.Parallel()

	value := ExitCode(-1)
	err := value.Validate()
	if !errors.Is(err, ErrInvalidExitCode) {
		t.Fatalf("Validate() error = %v, want ErrInvalidExitCode", err)
	}
	var valueErr *InvalidExitCodeError
	if !errors.As(err, &valueErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidExitCodeError", err)
	}
	if valueErr.Value != value {
		t.Fatalf("InvalidExitCodeError.Value = %d, want %d", valueErr.Value, value)
	}
}

func testInvalidFilesystemPathPreservesInput(t *testing.T) {
	t.Parallel()

	value := FilesystemPath(" \t ")
	err := value.Validate()
	if !errors.Is(err, ErrInvalidFilesystemPath) {
		t.Fatalf("Validate() error = %v, want ErrInvalidFilesystemPath", err)
	}
	var valueErr *InvalidFilesystemPathError
	if !errors.As(err, &valueErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidFilesystemPathError", err)
	}
	if valueErr.Value != value {
		t.Fatalf("InvalidFilesystemPathError.Value = %q, want %q", valueErr.Value, value)
	}
}

func testInvalidListenPortPreservesInput(t *testing.T) {
	t.Parallel()

	value := ListenPort(-1)
	err := value.Validate()
	if !errors.Is(err, ErrInvalidListenPort) {
		t.Fatalf("Validate() error = %v, want ErrInvalidListenPort", err)
	}
	var valueErr *InvalidListenPortError
	if !errors.As(err, &valueErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidListenPortError", err)
	}
	if valueErr.Value != value {
		t.Fatalf("InvalidListenPortError.Value = %d, want %d", valueErr.Value, value)
	}
}

func testInvalidRuntimeModePreservesInput(t *testing.T) {
	t.Parallel()

	value := RuntimeMode("bogus")
	err := value.Validate()
	if !errors.Is(err, ErrInvalidRuntimeMode) {
		t.Fatalf("Validate() error = %v, want ErrInvalidRuntimeMode", err)
	}
	var valueErr *InvalidRuntimeModeError
	if !errors.As(err, &valueErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidRuntimeModeError", err)
	}
	if valueErr.Value != value {
		t.Fatalf("InvalidRuntimeModeError.Value = %q, want %q", valueErr.Value, value)
	}
}

func testInvalidShellPathPreservesInput(t *testing.T) {
	t.Parallel()

	value := ShellPath(" \t ")
	err := value.Validate()
	if !errors.Is(err, ErrInvalidShellPath) {
		t.Fatalf("Validate() error = %v, want ErrInvalidShellPath", err)
	}
	var valueErr *InvalidShellPathError
	if !errors.As(err, &valueErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidShellPathError", err)
	}
	if valueErr.Value != value {
		t.Fatalf("InvalidShellPathError.Value = %q, want %q", valueErr.Value, value)
	}
}
