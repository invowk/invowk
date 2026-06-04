// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
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

func TestOperationsValidateMutationTreeEntryContracts(t *testing.T) {
	t.Parallel()

	t.Run("symlink named vendored directory is still reported", func(t *testing.T) {
		t.Parallel()

		modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
		target := filepath.Join(modulePath, "target")
		if err := os.WriteFile(target, []byte("target\n"), 0o644); err != nil {
			t.Fatalf("write target: %v", err)
		}
		linkPath := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Symlink(target, linkPath); err != nil {
			t.Skipf("symlink test skipped: %v", err)
		}

		result, err := Validate(types.FilesystemPath(modulePath))
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if !validationResultContainsIssue(result, IssueTypeSecurity, VendoredModulesDir, "symlinks are not allowed") {
			t.Fatalf("Validate() issues = %#v, want vendored-name symlink security issue", result.Issues)
		}
	})

	t.Run("file ending in module suffix is not a nested module", func(t *testing.T) {
		t.Parallel()

		modulePath := createValidModule(t, t.TempDir(), "tools.invowkmod", "tools")
		if err := os.WriteFile(filepath.Join(modulePath, "notes.invowkmod"), []byte("not a directory\n"), 0o644); err != nil {
			t.Fatalf("write suffix file: %v", err)
		}

		result, err := Validate(types.FilesystemPath(modulePath))
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if !result.Valid {
			t.Fatalf("Validate().Valid = false, want true; issues = %#v", result.Issues)
		}
	})
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
