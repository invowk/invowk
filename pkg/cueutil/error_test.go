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
		if !errors.Is(err, originalErr) {
			t.Errorf("error should wrap original error, got: %v", err)
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

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{name: "data within limit returns nil", data: []byte("hello world")},
		{name: "data at exact limit returns nil", data: make([]byte, 100)},
		{name: "data exceeding limit returns error", data: make([]byte, 101), wantErr: true},
		{name: "empty data returns nil", data: []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requireFileSizeCheck(t, CheckFileSize(tt.data, 100, "test.cue"), tt.wantErr)
		})
	}
}

func requireFileSizeCheck(t *testing.T, err error, wantErr bool) {
	t.Helper()

	if !wantErr {
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("expected error")
	}
	for _, part := range []string{"test.cue", "101", "100"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error should contain %q, got: %v", part, err)
		}
	}
}

func TestValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            ValidationError
		wantText       string
		wantSuggestion bool
	}{
		{
			name: "Error with path",
			err: ValidationError{
				FilePath: "config.cue",
				CUEPath:  "cmds[0].name",
				Message:  "expected string, got int",
			},
			wantText: "config.cue: cmds[0].name: expected string, got int",
		},
		{
			name: "Error without path",
			err: ValidationError{
				FilePath: "config.cue",
				Message:  "syntax error",
			},
			wantText: "config.cue: syntax error",
		},
		{
			name: "Unwrap returns nil",
			err: ValidationError{
				FilePath: "config.cue",
				Message:  "some error",
			},
			wantText: "config.cue: some error",
		},
		{
			name: "Suggestion field",
			err: ValidationError{
				FilePath:   "invowkfile.cue",
				CUEPath:    "cmds[0].runtime",
				Message:    "invalid runtime mode",
				Suggestion: "use 'native', 'virtual-sh', 'virtual-lua', or 'container'",
			},
			wantText:       "invowkfile.cue: cmds[0].runtime: invalid runtime mode",
			wantSuggestion: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Error(); got != tt.wantText {
				t.Errorf("Error() = %q, want %q", got, tt.wantText)
			}
			if got := tt.err.Unwrap(); got != nil {
				t.Errorf("Unwrap() = %v, want nil", got)
			}
			if tt.wantSuggestion && tt.err.Suggestion == "" {
				t.Error("Suggestion should not be empty")
			}
		})
	}
}
