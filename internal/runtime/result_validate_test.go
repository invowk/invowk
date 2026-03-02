// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
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
			name:    "valid exit code",
			result:  Result{ExitCode: types.ExitCode(0)},
			wantErr: false,
		},
		{
			name:    "valid non-zero exit code",
			result:  Result{ExitCode: types.ExitCode(1)},
			wantErr: false,
		},
		{
			name:    "invalid exit code",
			result:  Result{ExitCode: types.ExitCode(-1)},
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

func TestResult_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	r := Result{ExitCode: types.ExitCode(-1)}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for invalid exit code")
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

func TestInitDiagnostic_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		diag    InitDiagnostic
		wantErr bool
	}{
		{
			name:    "valid diagnostic code",
			diag:    InitDiagnostic{Code: CodeContainerRuntimeInitFailed, Message: "test"},
			wantErr: false,
		},
		{
			name:    "invalid diagnostic code",
			diag:    InitDiagnostic{Code: InitDiagnosticCode("invalid_code")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.diag.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("InitDiagnostic.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitDiagnostic_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	diag := InitDiagnostic{Code: InitDiagnosticCode("bogus")}
	err := diag.Validate()
	if err == nil {
		t.Fatal("expected error for invalid code")
	}

	if !errors.Is(err, ErrInvalidInitDiagnostic) {
		t.Errorf("errors.Is(err, ErrInvalidInitDiagnostic) = false, want true")
	}

	var invalidErr *InvalidInitDiagnosticError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidInitDiagnosticError) = false, want true")
	}
}

func TestExecutionContext_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctx     ExecutionContext
		wantErr bool
	}{
		{
			name: "valid context with native runtime",
			ctx: ExecutionContext{
				SelectedRuntime: "native",
			},
			wantErr: false,
		},
		{
			name: "valid context with execution ID",
			ctx: ExecutionContext{
				SelectedRuntime: "virtual",
				ExecutionID:     ExecutionID("123456-1"),
			},
			wantErr: false,
		},
		{
			name: "invalid selected runtime",
			ctx: ExecutionContext{
				SelectedRuntime: "bogus",
			},
			wantErr: true,
		},
		{
			name: "invalid execution ID when non-empty",
			ctx: ExecutionContext{
				SelectedRuntime: "native",
				ExecutionID:     ExecutionID("bad-format"),
			},
			wantErr: true,
		},
		{
			name: "empty execution ID is valid",
			ctx: ExecutionContext{
				SelectedRuntime: "native",
				ExecutionID:     "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.ctx.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecutionContext.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutionContext_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	ctx := ExecutionContext{
		SelectedRuntime: "invalid-runtime",
	}
	err := ctx.Validate()
	if err == nil {
		t.Fatal("expected error for invalid runtime")
	}

	if !errors.Is(err, ErrInvalidExecutionContext) {
		t.Errorf("errors.Is(err, ErrInvalidExecutionContext) = false, want true")
	}

	var invalidErr *InvalidExecutionContextError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidExecutionContextError) = false, want true")
	}
	if len(invalidErr.FieldErrors) == 0 {
		t.Error("expected non-empty FieldErrors")
	}
}
