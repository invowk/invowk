// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestRunResult_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  RunResult
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			result:  RunResult{},
			wantErr: false,
		},
		{
			name: "valid with container ID and exit code",
			result: RunResult{
				ContainerID: ContainerID("abc123"),
				ExitCode:    types.ExitCode(0),
			},
			wantErr: false,
		},
		{
			name: "invalid exit code",
			result: RunResult{
				ExitCode: types.ExitCode(-1),
			},
			wantErr: true,
		},
		{
			name: "valid non-zero exit code",
			result: RunResult{
				ExitCode: types.ExitCode(1),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.result.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RunResult.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunResult_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	result := RunResult{ExitCode: types.ExitCode(-1)}
	err := result.Validate()
	if err == nil {
		t.Fatal("expected error for invalid exit code")
	}

	if !errors.Is(err, ErrInvalidRunResult) {
		t.Errorf("errors.Is(err, ErrInvalidRunResult) = false, want true")
	}

	var invalidErr *InvalidRunResultError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidRunResultError) = false, want true")
	}
	if len(invalidErr.FieldErrors) == 0 {
		t.Error("expected non-empty FieldErrors")
	}
}
