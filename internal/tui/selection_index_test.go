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

func TestSelectionIndex_Validate(t *testing.T) {
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
			err := tt.index.Validate()
			if (err == nil) != tt.want {
				t.Errorf("SelectionIndex(%d).Validate() err = %v, wantValid %v", tt.index, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SelectionIndex(%d).Validate() returned nil, want error", tt.index)
				}
				if !errors.Is(err, ErrInvalidSelectionIndex) {
					t.Errorf("error should wrap ErrInvalidSelectionIndex, got: %v", err)
				}
				var idxErr *InvalidSelectionIndexError
				if !errors.As(err, &idxErr) {
					t.Errorf("error should be *InvalidSelectionIndexError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("SelectionIndex(%d).Validate() returned unexpected error: %v", tt.index, err)
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
