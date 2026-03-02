// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestExecuteRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     ExecuteRequest
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			req:     ExecuteRequest{},
			wantErr: false,
		},
		{
			name: "valid with runtime",
			req: ExecuteRequest{
				Name:    "build",
				Runtime: invowkfile.RuntimeNative,
			},
			wantErr: false,
		},
		{
			name: "invalid runtime",
			req: ExecuteRequest{
				Runtime: invowkfile.RuntimeMode("bogus"),
			},
			wantErr: true,
		},
		{
			name: "invalid env inherit mode",
			req: ExecuteRequest{
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
				t.Errorf("ExecuteRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteRequest_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	req := ExecuteRequest{Runtime: invowkfile.RuntimeMode("bogus")}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for invalid runtime")
	}

	if !errors.Is(err, ErrInvalidExecuteRequest) {
		t.Errorf("errors.Is(err, ErrInvalidExecuteRequest) = false, want true")
	}

	var invalidErr *InvalidExecuteRequestError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidExecuteRequestError) = false, want true")
	}
}

func TestExecuteResult_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  ExecuteResult
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			result:  ExecuteResult{},
			wantErr: false,
		},
		{
			name: "valid exit code",
			result: ExecuteResult{
				ExitCode: types.ExitCode(0),
			},
			wantErr: false,
		},
		{
			name: "invalid exit code",
			result: ExecuteResult{
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
				t.Errorf("ExecuteResult.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteResult_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	result := ExecuteResult{ExitCode: types.ExitCode(-1)}
	err := result.Validate()
	if err == nil {
		t.Fatal("expected error for invalid exit code")
	}

	if !errors.Is(err, ErrInvalidExecuteResult) {
		t.Errorf("errors.Is(err, ErrInvalidExecuteResult) = false, want true")
	}

	var invalidErr *InvalidExecuteResultError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidExecuteResultError) = false, want true")
	}
}

func TestSourceFilter_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filter  SourceFilter
		wantErr bool
	}{
		{
			name:    "valid source filter",
			filter:  SourceFilter{SourceID: discovery.SourceIDInvowkfile},
			wantErr: false,
		},
		{
			name:    "valid module source",
			filter:  SourceFilter{SourceID: discovery.SourceID("foo")},
			wantErr: false,
		},
		{
			name:    "empty source ID is invalid",
			filter:  SourceFilter{SourceID: discovery.SourceID("")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.filter.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SourceFilter.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSourceFilter_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	filter := SourceFilter{SourceID: discovery.SourceID("")}
	err := filter.Validate()
	if err == nil {
		t.Fatal("expected error for empty source ID")
	}

	if !errors.Is(err, ErrInvalidSourceFilter) {
		t.Errorf("errors.Is(err, ErrInvalidSourceFilter) = false, want true")
	}

	var invalidErr *InvalidSourceFilterError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidSourceFilterError) = false, want true")
	}
}
