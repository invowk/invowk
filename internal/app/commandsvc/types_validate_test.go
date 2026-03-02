// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			req:     Request{},
			wantErr: false,
		},
		{
			name: "valid with runtime",
			req: Request{
				Name:    "build",
				Runtime: invowkfile.RuntimeNative,
			},
			wantErr: false,
		},
		{
			name: "invalid runtime",
			req: Request{
				Runtime: invowkfile.RuntimeMode("bogus"),
			},
			wantErr: true,
		},
		{
			name: "invalid env inherit mode",
			req: Request{
				EnvInheritMode: invowkfile.EnvInheritMode("bogus"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Request.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequest_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	req := Request{Runtime: invowkfile.RuntimeMode("bogus")}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for invalid runtime")
	}

	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("errors.Is(err, ErrInvalidRequest) = false, want true")
	}

	var invalidErr *InvalidRequestError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidRequestError) = false, want true")
	}
}

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
			name: "valid exit code",
			result: Result{
				ExitCode: types.ExitCode(0),
			},
			wantErr: false,
		},
		{
			name: "invalid exit code",
			result: Result{
				ExitCode: types.ExitCode(-1),
			},
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

	result := Result{ExitCode: types.ExitCode(-1)}
	err := result.Validate()
	if err == nil {
		t.Fatal("expected error for invalid exit code")
	}

	if !errors.Is(err, ErrInvalidCommandsvcResult) {
		t.Errorf("errors.Is(err, ErrInvalidCommandsvcResult) = false, want true")
	}

	var invalidErr *InvalidResultError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidResultError) = false, want true")
	}
}

func TestDryRunData_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    DryRunData
		wantErr bool
	}{
		{
			name:    "zero value fails (selection requires valid mode)",
			data:    DryRunData{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.data.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRunData.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDryRunData_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	data := DryRunData{}
	err := data.Validate()
	if err == nil {
		t.Fatal("expected error for zero-value DryRunData")
	}

	if !errors.Is(err, ErrInvalidDryRunData) {
		t.Errorf("errors.Is(err, ErrInvalidDryRunData) = false, want true")
	}

	var invalidErr *InvalidDryRunDataError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidDryRunDataError) = false, want true")
	}
}
