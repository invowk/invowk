// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestTextAlign_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a    TextAlign
		want string
	}{
		{AlignLeft, "left"},
		{AlignCenter, "center"},
		{AlignRight, "right"},
		{TextAlign(""), ""},
		{TextAlign("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.a.String()
			if got != tt.want {
				t.Errorf("TextAlign(%q).String() = %q, want %q", tt.a, got, tt.want)
			}
		})
	}
}

func TestTextAlign_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a       TextAlign
		want    bool
		wantErr bool
	}{
		{"", true, false},
		{AlignLeft, true, false},
		{AlignCenter, true, false},
		{AlignRight, true, false},
		{"justify", false, true},
		{"LEFT", false, true},
		{"  ", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.a), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.a.IsValid()
			if isValid != tt.want {
				t.Errorf("TextAlign(%q).IsValid() = %v, want %v", tt.a, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("TextAlign(%q).IsValid() returned no errors, want error", tt.a)
				}
				if !errors.Is(errs[0], ErrInvalidTextAlign) {
					t.Errorf("error should wrap ErrInvalidTextAlign, got: %v", errs[0])
				}
				var taErr *InvalidTextAlignError
				if !errors.As(errs[0], &taErr) {
					t.Errorf("error should be *InvalidTextAlignError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("TextAlign(%q).IsValid() returned unexpected errors: %v", tt.a, errs)
			}
		})
	}
}

func TestTextAlign_Constants(t *testing.T) {
	t.Parallel()

	if AlignLeft != "left" {
		t.Errorf("expected AlignLeft to be 'left', got %q", AlignLeft)
	}
	if AlignCenter != "center" {
		t.Errorf("expected AlignCenter to be 'center', got %q", AlignCenter)
	}
	if AlignRight != "right" {
		t.Errorf("expected AlignRight to be 'right', got %q", AlignRight)
	}
}

func TestInvalidTextAlignError(t *testing.T) {
	t.Parallel()

	err := &InvalidTextAlignError{Value: "bad"}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidTextAlign) {
		t.Error("expected error to wrap ErrInvalidTextAlign")
	}
}
