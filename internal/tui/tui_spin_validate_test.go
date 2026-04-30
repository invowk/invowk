// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestSpinOptions_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    SpinOptions
		wantErr bool
	}{
		{
			name:    "zero value is valid (SpinnerLine = 0)",
			opts:    SpinOptions{},
			wantErr: false,
		},
		{
			name: "valid with spinner type",
			opts: SpinOptions{
				Title: "Loading...",
				Type:  SpinnerDot,
			},
			wantErr: false,
		},
		{
			name: "invalid spinner type",
			opts: SpinOptions{
				Type: SpinnerType(999),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SpinOptions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpinOptions_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	opts := SpinOptions{Type: SpinnerType(999)}
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected error for invalid spinner type")
	}

	if !errors.Is(err, ErrInvalidSpinOptions) {
		t.Errorf("errors.Is(err, ErrInvalidSpinOptions) = false, want true")
	}

	var invalidErr *InvalidSpinOptionsError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidSpinOptionsError) = false, want true")
	}
}

func TestSpinCommandOptions_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    SpinCommandOptions
		wantErr bool
	}{
		{
			name:    "zero value is valid (SpinnerLine = 0)",
			opts:    SpinCommandOptions{},
			wantErr: false,
		},
		{
			name: "valid with run command and type",
			opts: SpinCommandOptions{
				Title: "Running...",
				Run:   testSpinRun(),
				Type:  SpinnerGlobe,
			},
			wantErr: false,
		},
		{
			name: "invalid spinner type",
			opts: SpinCommandOptions{
				Type: SpinnerType(999),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SpinCommandOptions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpinCommandOptions_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{Type: SpinnerType(999)}
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected error for invalid spinner type")
	}

	if !errors.Is(err, ErrInvalidSpinCommandOptions) {
		t.Errorf("errors.Is(err, ErrInvalidSpinCommandOptions) = false, want true")
	}

	var invalidErr *InvalidSpinCommandOptionsError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidSpinCommandOptionsError) = false, want true")
	}
}
