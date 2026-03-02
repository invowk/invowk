// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestValidationIssue_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		issue     ValidationIssue
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid issue with all fields",
			ValidationIssue{
				Type:    IssueTypeStructure,
				Message: "missing file",
				Path:    "some/path",
			},
			true, false, 0,
		},
		{
			"valid issue with empty optional fields",
			ValidationIssue{
				Type:    IssueTypeNaming,
				Message: "",
				Path:    "",
			},
			true, false, 0,
		},
		{
			"valid issue all types",
			ValidationIssue{Type: IssueTypeInvowkmod},
			true, false, 0,
		},
		{
			"invalid type (empty)",
			ValidationIssue{Type: ""},
			false, true, 1,
		},
		{
			"invalid type (unknown)",
			ValidationIssue{Type: "unknown"},
			false, true, 1,
		},
		{
			"zero value",
			ValidationIssue{},
			false, true, 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.issue.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ValidationIssue.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidationIssue.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidValidationIssue) {
					t.Errorf("error should wrap ErrInvalidValidationIssue, got: %v", err)
				}
				var issueErr *InvalidValidationIssueError
				if !errors.As(err, &issueErr) {
					t.Fatalf("error should be *InvalidValidationIssueError, got: %T", err)
				}
				if len(issueErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(issueErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("ValidationIssue.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestValidationResult_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		result    ValidationResult
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete result",
			ValidationResult{
				Valid:          true,
				ModulePath:     types.FilesystemPath("/home/user/module.invowkmod"),
				ModuleName:     ModuleShortName("mymodule"),
				InvowkmodPath:  types.FilesystemPath("/home/user/module.invowkmod/invowkmod.cue"),
				InvowkfilePath: types.FilesystemPath("/home/user/module.invowkmod/invowkfile.cue"),
				Issues: []ValidationIssue{
					{Type: IssueTypeStructure, Message: "test"},
				},
			},
			true, false, 0,
		},
		{
			"valid minimal result (zero values)",
			ValidationResult{},
			true, false, 0,
		},
		{
			"valid with empty optional paths",
			ValidationResult{
				Valid:      true,
				ModulePath: types.FilesystemPath("/some/path"),
			},
			true, false, 0,
		},
		{
			"invalid module name",
			ValidationResult{
				ModuleName: ModuleShortName("1invalid"),
			},
			false, true, 1,
		},
		{
			"invalid issue in slice",
			ValidationResult{
				Issues: []ValidationIssue{
					{Type: "invalid-type"},
				},
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			ValidationResult{
				ModuleName: ModuleShortName("1invalid"),
				Issues: []ValidationIssue{
					{Type: "bad-type"},
				},
			},
			false, true, 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.result.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ValidationResult.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidationResult.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidValidationResult) {
					t.Errorf("error should wrap ErrInvalidValidationResult, got: %v", err)
				}
				var resultErr *InvalidValidationResultError
				if !errors.As(err, &resultErr) {
					t.Fatalf("error should be *InvalidValidationResultError, got: %T", err)
				}
				if len(resultErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(resultErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("ValidationResult.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
