// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"errors"
	"strings"
	"testing"
)

// T020: Error formatting tests
func TestFormatError(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()

		err := FormatError(nil, "test.cue")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("non-CUE error is wrapped with filepath", func(t *testing.T) {
		t.Parallel()

		originalErr := errors.New("some error")
		err := FormatError(originalErr, "test.cue")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "test.cue") {
			t.Errorf("error should contain filepath, got: %v", err)
		}
		if !strings.Contains(err.Error(), "some error") {
			t.Errorf("error should contain original message, got: %v", err)
		}
	})
}

func TestFormatPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     []string
		expected string
	}{
		{
			name:     "empty path",
			path:     []string{},
			expected: "",
		},
		{
			name:     "single element",
			path:     []string{"name"},
			expected: "name",
		},
		{
			name:     "nested path",
			path:     []string{"cmds", "script"},
			expected: "cmds.script",
		},
		{
			name:     "array index",
			path:     []string{"cmds", "0", "script"},
			expected: "cmds[0].script",
		},
		{
			name:     "multiple array indices",
			path:     []string{"cmds", "0", "implementations", "2", "script"},
			expected: "cmds[0].implementations[2].script",
		},
		{
			name:     "nested arrays",
			path:     []string{"items", "0", "values", "1"},
			expected: "items[0].values[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatPath(tt.path)
			if result != tt.expected {
				t.Errorf("formatPath(%v) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCheckFileSize(t *testing.T) {
	t.Parallel()

	t.Run("data within limit returns nil", func(t *testing.T) {
		t.Parallel()

		data := []byte("hello world")
		err := CheckFileSize(data, 100, "test.cue")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("data at exact limit returns nil", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 100)
		err := CheckFileSize(data, 100, "test.cue")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("data exceeding limit returns error", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 101)
		err := CheckFileSize(data, 100, "test.cue")
		if err == nil {
			t.Error("expected error")
		}
		if !strings.Contains(err.Error(), "test.cue") {
			t.Errorf("error should contain filename, got: %v", err)
		}
		if !strings.Contains(err.Error(), "101") {
			t.Errorf("error should contain actual size, got: %v", err)
		}
		if !strings.Contains(err.Error(), "100") {
			t.Errorf("error should contain max size, got: %v", err)
		}
	})

	t.Run("empty data returns nil", func(t *testing.T) {
		t.Parallel()

		err := CheckFileSize([]byte{}, 100, "test.cue")
		if err != nil {
			t.Errorf("expected nil for empty data, got %v", err)
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Parallel()

	t.Run("Error with path", func(t *testing.T) {
		t.Parallel()

		err := &ValidationError{
			FilePath: "config.cue",
			CUEPath:  "cmds[0].name",
			Message:  "expected string, got int",
		}
		expected := "config.cue: cmds[0].name: expected string, got int"
		if err.Error() != expected {
			t.Errorf("got %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Error without path", func(t *testing.T) {
		t.Parallel()

		err := &ValidationError{
			FilePath: "config.cue",
			CUEPath:  "",
			Message:  "syntax error",
		}
		expected := "config.cue: syntax error"
		if err.Error() != expected {
			t.Errorf("got %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Unwrap returns nil", func(t *testing.T) {
		t.Parallel()

		err := &ValidationError{
			FilePath: "config.cue",
			Message:  "some error",
		}
		if err.Unwrap() != nil {
			t.Error("Unwrap should return nil")
		}
	})

	t.Run("Suggestion field", func(t *testing.T) {
		t.Parallel()

		err := &ValidationError{
			FilePath:   "invowkfile.cue",
			CUEPath:    "cmds[0].runtime",
			Message:    "invalid runtime mode",
			Suggestion: "use 'native', 'virtual', or 'container'",
		}
		// Suggestion is stored but not included in Error() output
		if err.Suggestion == "" {
			t.Error("Suggestion should not be empty")
		}
	})
}
