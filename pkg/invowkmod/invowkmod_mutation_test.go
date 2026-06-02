// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

const (
	mutationModuleID         ModuleID         = "io.example.tools"
	mutationSemVer           SemVer           = "1.0.0"
	mutationSemVerConstraint SemVerConstraint = "^1.2.3"
	mutationGitURL           GitURL           = "https://github.com/example/tools.git"
	mutationRelativeScript                    = "scripts/build.sh"
)

func TestInvowkmodValidateMutationContracts(t *testing.T) {
	t.Parallel()

	mod := Invowkmod{
		Module:      "1.invalid",
		Version:     "v1.2.3",
		Description: " \t ",
		Requires: []ModuleRequirement{
			{
				GitURL:  "git://example.com/tools.git",
				Version: "^v1.2.3",
				Alias:   "-bad",
				Path:    "../escape",
			},
		},
		FilePath: " \t ",
	}

	err := mod.Validate()
	modErr := requireInvowkmodError(t, err)
	if got, want := modErr.Error(), "invalid invowkmod: 5 field error(s)"; got != want {
		t.Fatalf("InvalidInvowkmodError.Error() = %q, want %q", got, want)
	}
	if got, want := len(modErr.FieldErrors), 5; got != want {
		t.Fatalf("FieldErrors length = %d, want %d", got, want)
	}

	wantWrappedErrors := []error{
		ErrInvalidModuleID,
		ErrInvalidSemVer,
		types.ErrInvalidDescriptionText,
		ErrInvalidGitURL,
		ErrInvalidSemVerConstraint,
		ErrInvalidModuleAlias,
		ErrInvalidSubdirectoryPath,
		types.ErrInvalidFilesystemPath,
	}
	for _, want := range wantWrappedErrors {
		if !fieldErrorsContain(modErr.FieldErrors, want) {
			t.Fatalf("FieldErrors should contain %v, got %#v", want, modErr.FieldErrors)
		}
	}
}

func TestModuleRequirementValidateMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("joins every invalid optional and required field", func(t *testing.T) {
		t.Parallel()

		req := ModuleRequirement{
			GitURL:  "",
			Version: "^v1.2.3",
			Alias:   "1bad",
			Path:    "nested/../escape",
		}

		err := req.Validate()
		if err == nil {
			t.Fatal("ModuleRequirement.Validate() returned nil, want error")
		}
		if got, want := joinedErrorLen(t, err), 4; got != want {
			t.Fatalf("joined error count = %d, want %d", got, want)
		}
		for _, want := range []error{
			ErrInvalidGitURL,
			ErrInvalidSemVerConstraint,
			ErrInvalidModuleAlias,
			ErrInvalidSubdirectoryPath,
		} {
			if !errors.Is(err, want) {
				t.Fatalf("ModuleRequirement.Validate() error should wrap %v, got %v", want, err)
			}
		}
	})

	t.Run("keeps primary constraint parse failures observable", func(t *testing.T) {
		t.Parallel()

		req := ModuleRequirement{
			GitURL:  mutationGitURL,
			Version: ">>1.2.3",
		}

		err := req.Validate()
		if !errors.Is(err, ErrInvalidSemVerConstraint) {
			t.Fatalf("ModuleRequirement.Validate() error = %v, want ErrInvalidSemVerConstraint", err)
		}
		if got, want := joinedErrorLen(t, err), 1; got != want {
			t.Fatalf("joined error count = %d, want %d", got, want)
		}
	})

	t.Run("accepts fully valid requirement", func(t *testing.T) {
		t.Parallel()

		req := ModuleRequirement{
			GitURL:  mutationGitURL,
			Version: mutationSemVerConstraint,
			Alias:   "tools",
			Path:    "modules/tools",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("ModuleRequirement.Validate() error = %v, want nil", err)
		}
	})
}

func TestSubdirectoryPathValidateMutationEdges(t *testing.T) {
	t.Parallel()

	tooLongReason := fmt.Sprintf("too long (%d chars, max %d)", MaxPathLength+1, MaxPathLength)
	tests := []struct {
		name       string
		path       SubdirectoryPath
		wantReason string
	}{
		{
			name: "max length is valid",
			path: SubdirectoryPath(strings.Repeat("a", MaxPathLength)),
		},
		{
			name:       "over max length rejected before normalization",
			path:       SubdirectoryPath(strings.Repeat("a", MaxPathLength+1)),
			wantReason: tooLongReason,
		},
		{
			name:       "null byte rejected before normalization",
			path:       "path\x00evil",
			wantReason: "contains null byte",
		},
		{
			name:       "slash traversal segment rejected",
			path:       "modules/../tools",
			wantReason: "path traversal not allowed",
		},
		{
			name:       "backslash traversal segment rejected",
			path:       `modules\..\tools`,
			wantReason: "path traversal not allowed",
		},
		{
			name:       "unix absolute path rejected",
			path:       "/modules/tools",
			wantReason: "absolute paths not allowed",
		},
		{
			name:       "uppercase windows drive rejected",
			path:       "C:/modules/tools",
			wantReason: "absolute paths not allowed",
		},
		{
			name:       "lowercase windows drive rejected",
			path:       "c:/modules/tools",
			wantReason: "absolute paths not allowed",
		},
		{
			name: "colon after non-letter is relative",
			path: "1:/modules/tools",
		},
		{
			name: "embedded dot runes are not traversal",
			path: "modules/v1..2/tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.path.Validate()
			if tt.wantReason == "" {
				if err != nil {
					t.Fatalf("SubdirectoryPath(%q).Validate() error = %v, want nil", tt.path, err)
				}
				return
			}

			pathErr := requireSubdirectoryPathError(t, err)
			if pathErr.Value != tt.path {
				t.Fatalf("InvalidSubdirectoryPathError.Value = %q, want %q", pathErr.Value, tt.path)
			}
			if pathErr.Reason != tt.wantReason {
				t.Fatalf("InvalidSubdirectoryPathError.Reason = %q, want %q", pathErr.Reason, tt.wantReason)
			}
		})
	}
}

func TestValidationIssueMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("type validation covers every public issue type", testValidationIssueTypeContracts)
	t.Run("error formatting includes path only when present", testValidationIssueErrorFormatting)
	t.Run("add issue appends exact fields and marks invalid", testValidationResultAddIssue)
	t.Run("add issue panics for invalid issue type", testValidationResultAddIssueInvalidType)
}

func TestModuleMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("path helpers expose metadata and library-only state", testModulePathHelpers)
	t.Run("resolve script path preserves absolute unix input and joins relative input", testModuleResolveScriptPath)
	t.Run("validate script path rejects symlink escapes", testModuleValidateScriptPathSymlinkEscape)
	t.Run("validate collects invalid metadata and path", testModuleValidateInvalidMetadataAndPath)
}

func testValidationIssueTypeContracts(t *testing.T) {
	t.Parallel()

	for _, issueType := range []ValidationIssueType{
		IssueTypeStructure,
		IssueTypeNaming,
		IssueTypeInvowkmod,
		IssueTypeSecurity,
		IssueTypeCompatibility,
		IssueTypeInvowkfile,
		IssueTypeCommandTree,
	} {
		if err := issueType.Validate(); err != nil {
			t.Fatalf("ValidationIssueType(%q).Validate() error = %v, want nil", issueType, err)
		}
	}

	err := ValidationIssueType("unknown").Validate()
	if !errors.Is(err, ErrInvalidValidationIssueType) {
		t.Fatalf("ValidationIssueType(\"unknown\").Validate() error = %v, want ErrInvalidValidationIssueType", err)
	}
}

func testValidationIssueErrorFormatting(t *testing.T) {
	t.Parallel()

	withPath := ValidationIssue{
		Type:    IssueTypeSecurity,
		Message: "symlink escape",
		Path:    mutationRelativeScript,
	}
	if got, want := withPath.Error(), "[security] scripts/build.sh: symlink escape"; got != want {
		t.Fatalf("ValidationIssue.Error() = %q, want %q", got, want)
	}

	withoutPath := ValidationIssue{
		Type:    IssueTypeStructure,
		Message: "missing invowkmod.cue",
	}
	if got, want := withoutPath.Error(), "[structure] missing invowkmod.cue"; got != want {
		t.Fatalf("ValidationIssue.Error() = %q, want %q", got, want)
	}
}

func testValidationResultAddIssue(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddIssue(IssueTypeCompatibility, "uses host-specific path", mutationRelativeScript)

	if result.Valid {
		t.Fatal("ValidationResult.Valid = true, want false")
	}
	if got, want := len(result.Issues), 1; got != want {
		t.Fatalf("Issues length = %d, want %d", got, want)
	}
	issue := result.Issues[0]
	if issue.Type != IssueTypeCompatibility || issue.Message != "uses host-specific path" || issue.Path != mutationRelativeScript {
		t.Fatalf("ValidationResult.AddIssue() issue = %#v, want exact fields", issue)
	}
}

func testValidationResultAddIssueInvalidType(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("ValidationResult.AddIssue() did not panic for invalid issue type")
		}
	}()
	result := &ValidationResult{Valid: true}
	result.AddIssue("unknown", "bad issue type", mutationRelativeScript)
}

func testModulePathHelpers(t *testing.T) {
	t.Parallel()

	moduleRoot := filepath.Join(t.TempDir(), "io.example.tools.invowkmod")
	module := &Module{
		Metadata: &Invowkmod{
			Module:  mutationModuleID,
			Version: mutationSemVer,
		},
		Path: types.FilesystemPath(moduleRoot),
	}

	if got := (&Module{}).Name(); got != "" {
		t.Fatalf("Module.Name() without metadata = %q, want empty", got)
	}
	if got := module.Name(); got != mutationModuleID {
		t.Fatalf("Module.Name() = %q, want %q", got, mutationModuleID)
	}
	if got, want := module.InvowkmodPath(), types.FilesystemPath(filepath.Join(moduleRoot, "invowkmod.cue")); got != want {
		t.Fatalf("Module.InvowkmodPath() = %q, want %q", got, want)
	}
	if got, want := module.InvowkfilePath(), types.FilesystemPath(filepath.Join(moduleRoot, "invowkfile.cue")); got != want {
		t.Fatalf("Module.InvowkfilePath() = %q, want %q", got, want)
	}

	module.IsLibraryOnly = true
	if got := module.InvowkfilePath(); got != "" {
		t.Fatalf("library-only Module.InvowkfilePath() = %q, want empty", got)
	}
}

func testModuleResolveScriptPath(t *testing.T) {
	t.Parallel()

	moduleRoot := filepath.Join(t.TempDir(), "io.example.tools.invowkmod")
	module := &Module{Path: types.FilesystemPath(moduleRoot)}
	absolute := types.FilesystemPath("/tmp/script.sh")
	if got := module.ResolveScriptPath(absolute); got != absolute {
		t.Fatalf("Module.ResolveScriptPath(%q) = %q, want unchanged", absolute, got)
	}

	relative := types.FilesystemPath(mutationRelativeScript)
	want := types.FilesystemPath(filepath.Join(moduleRoot, filepath.FromSlash(mutationRelativeScript)))
	if got := module.ResolveScriptPath(relative); got != want {
		t.Fatalf("Module.ResolveScriptPath(%q) = %q, want %q", relative, got, want)
	}
}

func testModuleValidateScriptPathSymlinkEscape(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	moduleRoot := filepath.Join(tempDir, "io.example.tools.invowkmod")
	if err := os.Mkdir(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(tempDir, "outside.sh")
	if err := os.WriteFile(outsidePath, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(moduleRoot, "escape.sh")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("symlink test skipped: %v", err)
	}

	module := &Module{Path: types.FilesystemPath(moduleRoot)}
	err := module.ValidateScriptPath("escape.sh")
	if err == nil {
		t.Fatal("Module.ValidateScriptPath() returned nil, want symlink escape error")
	}
	if !strings.Contains(err.Error(), "symlink escape") {
		t.Fatalf("Module.ValidateScriptPath() error = %v, want symlink escape", err)
	}
}

func testModuleValidateInvalidMetadataAndPath(t *testing.T) {
	t.Parallel()

	module := Module{
		Metadata: &Invowkmod{
			Module:  "bad-module",
			Version: "v1.0.0",
		},
		Path: " \t ",
	}

	err := module.Validate()
	moduleErr := requireModuleError(t, err)
	if got, want := moduleErr.Error(), "invalid module: 2 field error(s)"; got != want {
		t.Fatalf("InvalidModuleError.Error() = %q, want %q", got, want)
	}
	if !fieldErrorsContain(moduleErr.FieldErrors, ErrInvalidInvowkmod) {
		t.Fatalf("Module field errors should contain ErrInvalidInvowkmod, got %#v", moduleErr.FieldErrors)
	}
	if !fieldErrorsContain(moduleErr.FieldErrors, types.ErrInvalidFilesystemPath) {
		t.Fatalf("Module field errors should contain ErrInvalidFilesystemPath, got %#v", moduleErr.FieldErrors)
	}
}

func TestIsWindowsDrivePathMutationEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "C:/module", want: true},
		{path: "c:/module", want: true},
		{path: "Z:", want: true},
		{path: "1:/module", want: false},
		{path: ":/module", want: false},
		{path: "C", want: false},
		{path: "/C:/module", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			if got := isWindowsDrivePath(tt.path); got != tt.want {
				t.Fatalf("isWindowsDrivePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func requireInvowkmodError(t *testing.T, err error) *InvalidInvowkmodError {
	t.Helper()

	if !errors.Is(err, ErrInvalidInvowkmod) {
		t.Fatalf("error = %v, want ErrInvalidInvowkmod", err)
	}
	var modErr *InvalidInvowkmodError
	if !errors.As(err, &modErr) {
		t.Fatalf("error type = %T, want *InvalidInvowkmodError", err)
	}
	return modErr
}

func requireModuleError(t *testing.T, err error) *InvalidModuleError {
	t.Helper()

	if !errors.Is(err, ErrInvalidModule) {
		t.Fatalf("error = %v, want ErrInvalidModule", err)
	}
	var moduleErr *InvalidModuleError
	if !errors.As(err, &moduleErr) {
		t.Fatalf("error type = %T, want *InvalidModuleError", err)
	}
	return moduleErr
}

func requireSubdirectoryPathError(t *testing.T, err error) *InvalidSubdirectoryPathError {
	t.Helper()

	if !errors.Is(err, ErrInvalidSubdirectoryPath) {
		t.Fatalf("error = %v, want ErrInvalidSubdirectoryPath", err)
	}
	var pathErr *InvalidSubdirectoryPathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error type = %T, want *InvalidSubdirectoryPathError", err)
	}
	return pathErr
}

func fieldErrorsContain(fieldErrors []error, target error) bool {
	for _, err := range fieldErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func joinedErrorLen(t *testing.T, err error) int {
	t.Helper()

	if err == nil {
		t.Fatal("joined error is nil")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return 1
	}
	return len(joined.Unwrap())
}
