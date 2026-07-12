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
	mutationModuleID           ModuleID         = "io.example.tools"
	mutationSemVer             SemVer           = "1.0.0"
	mutationSemVerConstraint   SemVerConstraint = "^1.2.3"
	mutationGitURL             GitURL           = "https://github.com/example/tools.git"
	mutationRelativeScript                      = "scripts/build.sh"
	mutationAbsolutePathReason                  = "absolute paths not allowed"
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

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "joins every invalid optional and required field", run: testModuleRequirementAllInvalidFields},
		{name: "keeps primary constraint parse failures observable", run: testModuleRequirementConstraintParseFailure},
		{name: "accepts fully valid requirement", run: testModuleRequirementValid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testModuleRequirementAllInvalidFields(t *testing.T) {
	t.Helper()

	req := ModuleRequirement{GitURL: "", Version: "^v1.2.3", Alias: "1bad", Path: "nested/../escape"}
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
}

func testModuleRequirementConstraintParseFailure(t *testing.T) {
	t.Helper()

	req := ModuleRequirement{GitURL: mutationGitURL, Version: ">>1.2.3"}
	err := req.Validate()
	if !errors.Is(err, ErrInvalidSemVerConstraint) {
		t.Fatalf("ModuleRequirement.Validate() error = %v, want ErrInvalidSemVerConstraint", err)
	}
	if got, want := joinedErrorLen(t, err), 1; got != want {
		t.Fatalf("joined error count = %d, want %d", got, want)
	}
}

func testModuleRequirementValid(t *testing.T) {
	t.Helper()

	req := ModuleRequirement{
		GitURL: mutationGitURL, Version: mutationSemVerConstraint, Alias: "tools", Path: "modules/tools",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("ModuleRequirement.Validate() error = %v, want nil", err)
	}
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
			name: "single letter relative path is valid",
			path: "a",
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
			wantReason: mutationAbsolutePathReason,
		},
		{
			name:       "uppercase windows drive rejected",
			path:       "C:/modules/tools",
			wantReason: mutationAbsolutePathReason,
		},
		{
			name:       "uppercase boundary windows drive rejected",
			path:       "A:/modules/tools",
			wantReason: mutationAbsolutePathReason,
		},
		{
			name:       "lowercase windows drive rejected",
			path:       "c:/modules/tools",
			wantReason: mutationAbsolutePathReason,
		},
		{
			name:       "lowercase boundary windows drive rejected",
			path:       "z:/modules/tools",
			wantReason: mutationAbsolutePathReason,
		},
		{
			name:       "bare uppercase windows drive rejected",
			path:       "Z:",
			wantReason: mutationAbsolutePathReason,
		},
		{
			name:       "bare lowercase windows drive rejected",
			path:       "a:",
			wantReason: mutationAbsolutePathReason,
		},
		{
			name: "colon after non-letter is relative",
			path: "1:/modules/tools",
		},
		{
			name: "colon after character before uppercase letter is relative",
			path: "@:/modules/tools",
		},
		{
			name: "colon after character after uppercase letter is relative",
			path: "[:/modules/tools",
		},
		{
			name: "colon after character after lowercase letter is relative",
			path: "{:/modules/tools",
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

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "type validation covers every public issue type", run: testValidationIssueTypeContracts},
		{name: "error formatting includes path only when present", run: testValidationIssueErrorFormatting},
		{name: "add issue appends exact fields and marks invalid", run: testValidationResultAddIssue},
		{name: "add issue panics for invalid issue type", run: testValidationResultAddIssueInvalidType},
		{name: "invalid issue type error includes value", run: testInvalidValidationIssueTypeErrorIncludesValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestModuleMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "path helpers expose metadata and library-only state", run: testModulePathHelpers},
		{name: "resolve script path preserves absolute unix input and joins relative input", run: testModuleResolveScriptPath},
		{name: "validate script path rejects symlink escapes", run: testModuleValidateScriptPathSymlinkEscape},
		{name: "validate script path rejects direct traversal", run: testModuleValidateScriptPathTraversal},
		{name: "validate collects invalid metadata and path", run: testModuleValidateInvalidMetadataAndPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestModuleValueErrorMutationFormatting(t *testing.T) {
	t.Parallel()

	moduleIDErr := requireModuleIDError(t, ModuleID("1bad").Validate())
	if got, want := moduleIDErr.Error(), "invalid module ID \"1bad\": must match format 'segment.segment...' where each segment starts with a letter followed by alphanumeric characters, max 256 characters"; got != want {
		t.Fatalf("InvalidModuleIDError.Error() = %q, want %q", got, want)
	}

	aliasErr := requireModuleAliasError(t, ModuleAlias("1bad").Validate())
	if got, want := aliasErr.Error(), "invalid module alias \"1bad\" (must start with a letter and contain only letters, digits, dots, underscores, or hyphens)"; got != want {
		t.Fatalf("InvalidModuleAliasError.Error() = %q, want %q", got, want)
	}

	pathErr := requireSubdirectoryPathError(t, SubdirectoryPath("../escape").Validate())
	if got, want := pathErr.Error(), "invalid subdirectory path \"../escape\": path traversal not allowed"; got != want {
		t.Fatalf("InvalidSubdirectoryPathError.Error() = %q, want %q", got, want)
	}
}

func testValidationIssueTypeContracts(t *testing.T) {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("ValidationResult.AddIssue() did not panic for invalid issue type")
		}
	}()
	result := &ValidationResult{Valid: true}
	result.AddIssue("unknown", "bad issue type", mutationRelativeScript)
}

func testInvalidValidationIssueTypeErrorIncludesValue(t *testing.T) {
	t.Helper()

	err := ValidationIssueType("unknown").Validate()
	issueErr := requireValidationIssueTypeError(t, err)
	if got, want := issueErr.Error(), "invalid validation issue type \"unknown\" (valid: structure, naming, invowkmod, security, compatibility, invowkfile, command_tree)"; got != want {
		t.Fatalf("InvalidValidationIssueTypeError.Error() = %q, want %q", got, want)
	}
}

func testModulePathHelpers(t *testing.T) {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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

	parentLinkPath := filepath.Join(moduleRoot, "parent")
	if symlinkErr := os.Symlink(tempDir, parentLinkPath); symlinkErr != nil {
		t.Skipf("parent symlink test skipped: %v", symlinkErr)
	}
	err = module.ValidateScriptPath("parent")
	if err == nil {
		t.Fatal("Module.ValidateScriptPath() returned nil for parent symlink, want symlink escape error")
	}
	if !strings.Contains(err.Error(), "symlink escape") {
		t.Fatalf("Module.ValidateScriptPath() parent symlink error = %v, want symlink escape", err)
	}
}

func testModuleValidateScriptPathTraversal(t *testing.T) {
	t.Helper()

	module := &Module{Path: types.FilesystemPath(t.TempDir())}

	for _, scriptPath := range []types.FilesystemPath{"..", "../escape.sh"} {
		err := module.ValidateScriptPath(scriptPath)
		if err == nil {
			t.Fatalf("Module.ValidateScriptPath(%q) error = nil, want traversal error", scriptPath)
		}
		if !strings.Contains(err.Error(), "escapes the module directory") {
			t.Fatalf("Module.ValidateScriptPath(%q) error = %v, want traversal error", scriptPath, err)
		}
	}
}

func TestModuleContainsPathReturnsFalseWhenAbsFails(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tempDir := t.TempDir()
	deletedDir := filepath.Join(tempDir, "deleted-cwd")
	if err := os.Mkdir(deletedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deletedDir)
	if err := os.Remove(deletedDir); err != nil {
		t.Skipf("removing current directory is unsupported on this platform: %v", err)
	}

	module := &Module{Path: types.FilesystemPath(".")}
	if module.ContainsPath("script.sh") {
		t.Fatal("Module.ContainsPath() = true, want false when filepath.Abs fails")
	}
}

func testModuleValidateInvalidMetadataAndPath(t *testing.T) {
	t.Helper()

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
		{path: "A:", want: true},
		{path: "a:", want: true},
		{path: "c:/module", want: true},
		{path: "z:/module", want: true},
		{path: "Z:", want: true},
		{path: "1:/module", want: false},
		{path: "@:/module", want: false},
		{path: "[:/module", want: false},
		{path: "{:/module", want: false},
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

func requireModuleIDError(t *testing.T, err error) *InvalidModuleIDError {
	t.Helper()

	if !errors.Is(err, ErrInvalidModuleID) {
		t.Fatalf("error = %v, want ErrInvalidModuleID", err)
	}
	var moduleIDErr *InvalidModuleIDError
	if !errors.As(err, &moduleIDErr) {
		t.Fatalf("error type = %T, want *InvalidModuleIDError", err)
	}
	return moduleIDErr
}

func requireModuleAliasError(t *testing.T, err error) *InvalidModuleAliasError {
	t.Helper()

	if !errors.Is(err, ErrInvalidModuleAlias) {
		t.Fatalf("error = %v, want ErrInvalidModuleAlias", err)
	}
	var aliasErr *InvalidModuleAliasError
	if !errors.As(err, &aliasErr) {
		t.Fatalf("error type = %T, want *InvalidModuleAliasError", err)
	}
	return aliasErr
}

func requireValidationIssueTypeError(t *testing.T, err error) *InvalidValidationIssueTypeError {
	t.Helper()

	if !errors.Is(err, ErrInvalidValidationIssueType) {
		t.Fatalf("error = %v, want ErrInvalidValidationIssueType", err)
	}
	var issueTypeErr *InvalidValidationIssueTypeError
	if !errors.As(err, &issueTypeErr) {
		t.Fatalf("error type = %T, want *InvalidValidationIssueTypeError", err)
	}
	return issueTypeErr
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
