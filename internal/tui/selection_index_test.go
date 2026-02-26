// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestSelectionIndex_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		index SelectionIndex
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-1, "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.index.String()
			if got != tt.want {
				t.Errorf("SelectionIndex(%d).String() = %q, want %q", tt.index, got, tt.want)
			}
		})
	}
}

func TestSelectionIndex_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		index   SelectionIndex
		want    bool
		wantErr bool
	}{
		{0, true, false},
		{1, true, false},
		{999, true, false},
		{-1, false, true},
		{-15, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.index.String(), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.index.IsValid()
			if isValid != tt.want {
				t.Errorf("SelectionIndex(%d).IsValid() = %v, want %v", tt.index, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("SelectionIndex(%d).IsValid() returned no errors, want error", tt.index)
				}
				if !errors.Is(errs[0], ErrInvalidSelectionIndex) {
					t.Errorf("error should wrap ErrInvalidSelectionIndex, got: %v", errs[0])
				}
				var idxErr *InvalidSelectionIndexError
				if !errors.As(errs[0], &idxErr) {
					t.Errorf("error should be *InvalidSelectionIndexError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("SelectionIndex(%d).IsValid() returned unexpected errors: %v", tt.index, errs)
			}
		})
	}
}

func TestInvalidSelectionIndexError(t *testing.T) {
	t.Parallel()

	err := &InvalidSelectionIndexError{Value: -1}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidSelectionIndex) {
		t.Error("expected error to wrap ErrInvalidSelectionIndex")
	}
}
