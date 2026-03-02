// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/container"
)

func TestResult_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  Result
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			result:  Result{},
			wantErr: false,
		},
		{
			name: "valid with image tag",
			result: Result{
				ImageTag: container.ImageTag("invowk-provisioned:abc123"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.result.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Result.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResult_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	// ImageTag with whitespace-only value triggers validation error.
	result := Result{ImageTag: container.ImageTag("   ")}
	err := result.Validate()
	if err == nil {
		t.Fatal("expected error for whitespace-only image tag")
	}

	if !errors.Is(err, ErrInvalidResult) {
		t.Errorf("errors.Is(err, ErrInvalidResult) = false, want true")
	}

	var invalidErr *InvalidResultError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidResultError) = false, want true")
	}
	if len(invalidErr.FieldErrors) == 0 {
		t.Error("expected non-empty FieldErrors")
	}
}
