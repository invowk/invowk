// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestOperationsValidateMutationResultContracts(t *testing.T) {
	t.Parallel()

	modulePath := types.FilesystemPath(filepath.Join(t.TempDir(), "tools.invowkmod"))
	result := newValidationResult(modulePath)
	if !result.Valid {
		t.Fatal("newValidationResult().Valid = false, want true")
	}
	if result.ModulePath != modulePath {
		t.Fatalf("newValidationResult().ModulePath = %q, want %q", result.ModulePath, modulePath)
	}
	if result.Issues == nil {
		t.Fatal("newValidationResult().Issues = nil, want initialized empty slice")
	}
	if len(result.Issues) != 0 {
		t.Fatalf("newValidationResult().Issues length = %d, want 0", len(result.Issues))
	}
}

func TestOperationsValidateMutationRecordsInvowkmodPath(t *testing.T) {
	t.Parallel()

	modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
	result, err := Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	want := types.FilesystemPath(filepath.Join(modulePath, invowkmodCueFileName))
	if result.InvowkmodPath != want {
		t.Fatalf("Validate().InvowkmodPath = %q, want %q", result.InvowkmodPath, want)
	}
}

func TestOperationsValidateMutationReportsInaccessibleInvowkmodCue(t *testing.T) {
	t.Parallel()
	skipUnixPermissionMutationTest(t)

	modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
	if err := os.Chmod(modulePath, 0); err != nil {
		t.Fatalf("chmod module dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(modulePath, 0o755); err != nil && !os.IsNotExist(err) {
			t.Errorf("restore module dir permissions: %v", err)
		}
	})

	result, err := Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if !validationResultContainsIssue(result, IssueTypeStructure, "", "cannot access invowkmod.cue") {
		t.Fatalf("Validate() issues = %#v, want inaccessible invowkmod.cue issue", result.Issues)
	}
	if validationResultContainsIssue(result, IssueTypeStructure, "", "missing required invowkmod.cue") {
		t.Fatalf("Validate() issues = %#v, want permission error instead of missing-file error", result.Issues)
	}
}

func TestOperationsValidateMutationTreeEntryContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "symlink named vendored directory is still reported", run: testOperationsValidateMutationVendoredSymlinkReported},
		{name: "file ending in module suffix is not a nested module", run: testOperationsValidateMutationSuffixFileIsNotNestedModule},
		{name: "unreadable nested directory is skipped", run: testOperationsValidateMutationUnreadableNestedDirSkipped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testOperationsValidateMutationVendoredSymlinkReported(t *testing.T) {
	t.Helper()

	modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
	target := filepath.Join(modulePath, "target")
	if err := os.WriteFile(target, []byte("target\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	linkPath := filepath.Join(modulePath, VendoredModulesDir)
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink test skipped: %v", err)
	}

	result := requireValidateModuleSuccess(t, modulePath)
	if !validationResultContainsIssue(result, IssueTypeSecurity, VendoredModulesDir, "symlinks are not allowed") {
		t.Fatalf("Validate() issues = %#v, want vendored-name symlink security issue", result.Issues)
	}
}

func testOperationsValidateMutationSuffixFileIsNotNestedModule(t *testing.T) {
	t.Helper()

	modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
	if err := os.WriteFile(filepath.Join(modulePath, "notes.invowkmod"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("write suffix file: %v", err)
	}
	requireValidModuleValidationResult(t, requireValidateModuleSuccess(t, modulePath))
}

func testOperationsValidateMutationUnreadableNestedDirSkipped(t *testing.T) {
	t.Helper()
	skipUnixPermissionMutationTest(t)

	modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
	unreadablePath := filepath.Join(modulePath, "private")
	if err := os.Mkdir(unreadablePath, 0o755); err != nil {
		t.Fatalf("mkdir unreadable dir: %v", err)
	}
	if err := os.Chmod(unreadablePath, 0); err != nil {
		t.Fatalf("chmod unreadable dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(unreadablePath, 0o755); err != nil && !os.IsNotExist(err) {
			t.Errorf("restore unreadable dir permissions: %v", err)
		}
	})

	requireValidModuleValidationResult(t, requireValidateModuleSuccess(t, modulePath))
}

func requireValidateModuleSuccess(t *testing.T, modulePath string) *ValidationResult {
	t.Helper()

	result, err := Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	return result
}

func requireValidModuleValidationResult(t *testing.T, result *ValidationResult) {
	t.Helper()

	if !result.Valid {
		t.Fatalf("Validate().Valid = false, want true; issues = %#v", result.Issues)
	}
}

func TestOperationsLoadMutationAggregatesIssueMessages(t *testing.T) {
	t.Parallel()

	modulePath := filepath.Join(t.TempDir(), "tools.invowkmod")
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	if err := os.Mkdir(filepath.Join(modulePath, "nested.invowkmod"), 0o755); err != nil {
		t.Fatalf("mkdir nested module: %v", err)
	}

	_, err := Load(types.FilesystemPath(modulePath))
	if err == nil {
		t.Fatal("Load() error = nil, want invalid module error")
	}
	errText := err.Error()
	for _, want := range []string{
		"missing required invowkmod.cue",
		"nested modules are not allowed",
	} {
		if !strings.Contains(errText, want) {
			t.Fatalf("Load() error = %q, want message %q", errText, want)
		}
	}
}

func validationResultContainsIssue(result *ValidationResult, issueType ValidationIssueType, path, message string) bool {
	for _, issue := range result.Issues {
		if issue.Type == issueType && issue.Path == path && strings.Contains(issue.Message, message) {
			return true
		}
	}
	return false
}

func skipUnixPermissionMutationTest(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission checks are Unix-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can traverse directories regardless of permission bits")
	}
}
