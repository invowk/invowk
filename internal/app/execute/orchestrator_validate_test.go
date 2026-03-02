// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestBuildExecutionContextOptions_Validate(t *testing.T) {
	t.Parallel()

	validSelection := RuntimeSelectionOf(invowkfile.RuntimeNative, &invowkfile.Implementation{})

	tests := []struct {
		name    string
		opts    BuildExecutionContextOptions
		wantErr bool
	}{
		{
			name: "valid with required fields",
			opts: BuildExecutionContextOptions{
				Selection: validSelection,
			},
			wantErr: false,
		},
		{
			name: "invalid selection (zero value)",
			opts: BuildExecutionContextOptions{
				Selection: RuntimeSelection{},
			},
			wantErr: true,
		},
		{
			name: "valid with source ID and platform",
			opts: BuildExecutionContextOptions{
				Selection: validSelection,
				SourceID:  discovery.SourceIDInvowkfile,
				Platform:  invowkfile.PlatformLinux,
			},
			wantErr: false,
		},
		{
			name: "invalid env inherit mode",
			opts: BuildExecutionContextOptions{
				Selection:      validSelection,
				EnvInheritMode: invowkfile.EnvInheritMode("bogus"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildExecutionContextOptions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildExecutionContextOptions_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	opts := BuildExecutionContextOptions{
		Selection: RuntimeSelection{},
	}
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected error for zero-value selection")
	}

	if !errors.Is(err, ErrInvalidBuildExecutionContextOptions) {
		t.Errorf("errors.Is(err, ErrInvalidBuildExecutionContextOptions) = false, want true")
	}

	var invalidErr *InvalidBuildExecutionContextOptionsError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidBuildExecutionContextOptionsError) = false, want true")
	}
	if len(invalidErr.FieldErrors) == 0 {
		t.Error("expected non-empty FieldErrors")
	}
}
