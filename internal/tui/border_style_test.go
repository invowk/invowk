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

func TestBorderStyle_Validate(t *testing.T) {
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
			err := tt.bs.Validate()
			if (err == nil) != tt.want {
				t.Errorf("BorderStyle(%q).Validate() err = %v, wantValid %v", tt.bs, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("BorderStyle(%q).Validate() returned nil, want error", tt.bs)
				}
				if !errors.Is(err, ErrInvalidBorderStyle) {
					t.Errorf("error should wrap ErrInvalidBorderStyle, got: %v", err)
				}
				var bsErr *InvalidBorderStyleError
				if !errors.As(err, &bsErr) {
					t.Errorf("error should be *InvalidBorderStyleError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("BorderStyle(%q).Validate() returned unexpected error: %v", tt.bs, err)
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
