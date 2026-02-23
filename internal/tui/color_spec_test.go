// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestColorSpec_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		c    ColorSpec
		want string
	}{
		{ColorSpec(""), ""},
		{ColorSpec("#ff0000"), "#ff0000"},
		{ColorSpec("212"), "212"},
		{ColorSpec("red"), "red"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.c.String()
			if got != tt.want {
				t.Errorf("ColorSpec(%q).String() = %q, want %q", tt.c, got, tt.want)
			}
		})
	}
}

func TestColorSpec_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		c       ColorSpec
		want    bool
		wantErr bool
	}{
		{"empty", ColorSpec(""), true, false},
		{"hex_color", ColorSpec("#ff0000"), true, false},
		{"ansi_code", ColorSpec("212"), true, false},
		{"named_color", ColorSpec("red"), true, false},
		{"whitespace_only_space", ColorSpec("   "), false, true},
		{"whitespace_only_tab", ColorSpec("\t"), false, true},
		{"whitespace_only_mixed", ColorSpec(" \t\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.c.IsValid()
			if isValid != tt.want {
				t.Errorf("ColorSpec(%q).IsValid() = %v, want %v", tt.c, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ColorSpec(%q).IsValid() returned no errors, want error", tt.c)
				}
				if !errors.Is(errs[0], ErrInvalidColorSpec) {
					t.Errorf("error should wrap ErrInvalidColorSpec, got: %v", errs[0])
				}
				var csErr *InvalidColorSpecError
				if !errors.As(errs[0], &csErr) {
					t.Errorf("error should be *InvalidColorSpecError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ColorSpec(%q).IsValid() returned unexpected errors: %v", tt.c, errs)
			}
		})
	}
}

func TestInvalidColorSpecError(t *testing.T) {
	t.Parallel()

	err := &InvalidColorSpecError{Value: "  "}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidColorSpec) {
		t.Error("expected error to wrap ErrInvalidColorSpec")
	}
}
