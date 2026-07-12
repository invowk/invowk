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
		{
			name:    "valid with warning",
			result:  Result{Warnings: []Warning{{Message: "module skipped"}}},
			wantErr: false,
		},
		{
			name:    "invalid warning",
			result:  Result{Warnings: []Warning{{Message: "   "}}},
			wantErr: true,
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

func TestWarningValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		warning Warning
		wantErr bool
	}{
		{name: "valid", warning: Warning{Message: "module could not be copied"}},
		{name: "empty", warning: Warning{}, wantErr: true},
		{name: "whitespace", warning: Warning{Message: " \t\n"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.warning.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidWarningMessage) {
				t.Errorf("Validate() error = %v, want ErrInvalidWarningMessage", err)
			}
			if got := tt.warning.Message.String(); got != string(tt.warning.Message) {
				t.Errorf("Message.String() = %q, want %q", got, tt.warning.Message)
			}
		})
	}
}

func TestResultValidateCollectsImageAndWarningErrors(t *testing.T) {
	t.Parallel()

	result := Result{
		ImageTag: container.ImageTag("   "),
		Warnings: []Warning{{Message: ""}, {Message: "valid"}, {Message: "   "}},
	}
	err := result.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want image and warning errors")
	}
	if !errors.Is(err, ErrInvalidResult) {
		t.Errorf("Validate() error = %v, want ErrInvalidResult", err)
	}
	var invalidErr *InvalidResultError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("Validate() error type = %T, want *InvalidResultError", err)
	}
	if got, want := len(invalidErr.FieldErrors), 3; got != want {
		t.Errorf("FieldErrors length = %d, want %d: %v", got, want, invalidErr.FieldErrors)
	}
	if !errors.Is(invalidErr.FieldErrors[0], container.ErrInvalidImageTag) ||
		!errors.Is(invalidErr.FieldErrors[1], ErrInvalidWarningMessage) ||
		!errors.Is(invalidErr.FieldErrors[2], ErrInvalidWarningMessage) {
		t.Errorf("FieldErrors = %v, want image then warning sentinel identities", invalidErr.FieldErrors)
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
