// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestBorderStyle_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bs   BorderStyle
		want string
	}{
		{BorderNone, ""},
		{BorderNormal, "normal"},
		{BorderRounded, "rounded"},
		{BorderThick, "thick"},
		{BorderDouble, "double"},
		{BorderHidden, "hidden"},
		{BorderStyle("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.bs.String()
			if got != tt.want {
				t.Errorf("BorderStyle(%q).String() = %q, want %q", tt.bs, got, tt.want)
			}
		})
	}
}

func TestBorderStyle_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bs      BorderStyle
		want    bool
		wantErr bool
	}{
		{BorderNone, true, false},
		{BorderNormal, true, false},
		{BorderRounded, true, false},
		{BorderThick, true, false},
		{BorderDouble, true, false},
		{BorderHidden, true, false},
		{"invalid", false, true},
		{"NORMAL", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.bs), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.bs.IsValid()
			if isValid != tt.want {
				t.Errorf("BorderStyle(%q).IsValid() = %v, want %v", tt.bs, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("BorderStyle(%q).IsValid() returned no errors, want error", tt.bs)
				}
				if !errors.Is(errs[0], ErrInvalidBorderStyle) {
					t.Errorf("error should wrap ErrInvalidBorderStyle, got: %v", errs[0])
				}
				var bsErr *InvalidBorderStyleError
				if !errors.As(errs[0], &bsErr) {
					t.Errorf("error should be *InvalidBorderStyleError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("BorderStyle(%q).IsValid() returned unexpected errors: %v", tt.bs, errs)
			}
		})
	}
}

func TestBorderStyle_Constants(t *testing.T) {
	t.Parallel()

	if BorderNone != "" {
		t.Errorf("expected BorderNone to be empty string, got %q", BorderNone)
	}
	if BorderNormal != "normal" {
		t.Errorf("expected BorderNormal to be 'normal', got %q", BorderNormal)
	}
	if BorderRounded != "rounded" {
		t.Errorf("expected BorderRounded to be 'rounded', got %q", BorderRounded)
	}
	if BorderThick != "thick" {
		t.Errorf("expected BorderThick to be 'thick', got %q", BorderThick)
	}
	if BorderDouble != "double" {
		t.Errorf("expected BorderDouble to be 'double', got %q", BorderDouble)
	}
	if BorderHidden != "hidden" {
		t.Errorf("expected BorderHidden to be 'hidden', got %q", BorderHidden)
	}
}

func TestInvalidBorderStyleError(t *testing.T) {
	t.Parallel()

	err := &InvalidBorderStyleError{Value: "bad"}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidBorderStyle) {
		t.Error("expected error to wrap ErrInvalidBorderStyle")
	}
}
