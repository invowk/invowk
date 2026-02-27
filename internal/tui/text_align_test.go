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

func TestTextAlign_Validate(t *testing.T) {
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
			err := tt.a.Validate()
			if (err == nil) != tt.want {
				t.Errorf("TextAlign(%q).Validate() err = %v, wantValid %v", tt.a, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TextAlign(%q).Validate() returned nil, want error", tt.a)
				}
				if !errors.Is(err, ErrInvalidTextAlign) {
					t.Errorf("error should wrap ErrInvalidTextAlign, got: %v", err)
				}
				var taErr *InvalidTextAlignError
				if !errors.As(err, &taErr) {
					t.Errorf("error should be *InvalidTextAlignError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("TextAlign(%q).Validate() returned unexpected error: %v", tt.a, err)
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
