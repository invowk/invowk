// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// TestModuleLockLines covers every row of the lock-outcome wording that module
// sync, update and add print (spec "Sync and update say whether the lock
// changed" and "Add and remove report each file truthfully"). The remove
// summary has its own table in TestModuleRemoveSummaryLine.
func TestModuleLockLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		render      func(bool) string
		lockWritten bool
		want        string
	}{
		{name: "sync/update written", render: moduleLockFileLine, lockWritten: true, want: moduleSuccessIcon + " Lock file updated: invowkmod.lock.cue"},
		{name: "sync/update skipped", render: moduleLockFileLine, lockWritten: false, want: moduleSuccessIcon + " Lock file unchanged: invowkmod.lock.cue"},
		{name: "add written", render: moduleAddLockLine, lockWritten: true, want: moduleSuccessIcon + " Module lock file updated"},
		{name: "add skipped", render: moduleAddLockLine, lockWritten: false, want: moduleSuccessIcon + " Module lock file unchanged"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.render(tt.lockWritten); got != tt.want {
				t.Fatalf("line = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestModuleRemoveSummaryLine covers the five summary rows of module remove
// (spec "Add and remove report each file truthfully" and "A failed
// invowkmod.cue edit reports the rollback").
func TestModuleRemoveSummaryLine(t *testing.T) {
	t.Parallel()

	wrote := mustDeclarationEditResult(t, true, nil)
	skipped := mustDeclarationEditResult(t, false, nil)
	failed := mustDeclarationEditResult(t, false, errors.New("failed to read invowkmod.cue: is a directory"))
	rollback := moduleInfoIcon + " Lock file changes were rolled back; invowkmod.cue needs manual review"

	tests := []struct {
		name         string
		declarations []modulesync.DeclarationEditResult
		lockWritten  bool
		want         string
	}{
		{name: "both written", declarations: []modulesync.DeclarationEditResult{wrote}, lockWritten: true, want: moduleSuccessIcon + " Lock file and invowkmod.cue updated"},
		{name: "lock skipped, invowkmod written", declarations: []modulesync.DeclarationEditResult{wrote}, lockWritten: false, want: moduleSuccessIcon + " Lock file unchanged; invowkmod.cue updated"},
		{name: "lock written, invowkmod unchanged", declarations: []modulesync.DeclarationEditResult{skipped}, lockWritten: true, want: moduleSuccessIcon + " Lock file updated; invowkmod.cue unchanged"},
		{name: "neither written", declarations: []modulesync.DeclarationEditResult{skipped}, lockWritten: false, want: moduleSuccessIcon + " Lock file and invowkmod.cue unchanged"},
		{name: "several edits, one wrote", declarations: []modulesync.DeclarationEditResult{skipped, wrote, skipped}, lockWritten: true, want: moduleSuccessIcon + " Lock file and invowkmod.cue updated"},
		{name: "several edits, none wrote", declarations: []modulesync.DeclarationEditResult{skipped, skipped}, lockWritten: true, want: moduleSuccessIcon + " Lock file updated; invowkmod.cue unchanged"},
		{name: "edit failed, lock written", declarations: []modulesync.DeclarationEditResult{failed}, lockWritten: true, want: rollback},
		{name: "edit failed, lock skipped", declarations: []modulesync.DeclarationEditResult{failed}, lockWritten: false, want: rollback},
		{name: "edit failed after one wrote", declarations: []modulesync.DeclarationEditResult{wrote, failed}, lockWritten: true, want: rollback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := moduleRemoveSummaryLine(tt.declarations, tt.lockWritten); got != tt.want {
				t.Fatalf("moduleRemoveSummaryLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestModuleAddDeclarationFailureLine covers "Add without invowkmod.cue" and
// "Add whose invowkmod.cue edit fails otherwise" on the error shape the add
// use case returns: the AddRequirement error joined with both restore errors.
func TestModuleAddDeclarationFailureLine(t *testing.T) {
	t.Parallel()

	ref := invowkmod.ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"}
	missingErr := invowkmod.AddRequirement(types.FilesystemPath(filepath.Join(t.TempDir(), "invowkmod.cue")), ref)
	if missingErr == nil {
		t.Fatal("AddRequirement() on a missing file = nil, want an error")
	}

	duplicatePath := filepath.Join(t.TempDir(), "invowkmod.cue")
	if err := os.WriteFile(duplicatePath, []byte(`module: "io.example.root"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	if err := invowkmod.AddRequirement(types.FilesystemPath(duplicatePath), ref); err != nil {
		t.Fatalf("AddRequirement() first add error = %v", err)
	}
	duplicateErr := errors.Join(invowkmod.AddRequirement(types.FilesystemPath(duplicatePath), ref), nil, nil)
	if !errors.Is(duplicateErr, invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("second AddRequirement() error = %v, want ErrModuleAlreadyExists", duplicateErr)
	}

	notFound := moduleInfoIcon + " invowkmod.cue not found — dependency was not persisted"
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing file, restores succeeded", err: errors.Join(missingErr, nil, nil), want: notFound},
		{name: "missing file, a restore failed", err: errors.Join(missingErr, nil, errors.New("restore lock: permission denied")), want: notFound},
		{name: "duplicate requirement", err: duplicateErr, want: moduleInfoIcon + " Could not auto-edit invowkmod.cue; lock file changes were rolled back: " + duplicateErr.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := moduleAddDeclarationFailureLine(tt.err); got != tt.want {
				t.Fatalf("moduleAddDeclarationFailureLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestModuleLockLinesNoModulesToUpdate covers "Update creates a missing lock"
// and its repeat: an update with no module still reports the lock save.
func TestModuleLockLinesNoModulesToUpdate(t *testing.T) {
	t.Parallel()

	for _, lockWritten := range []bool{true, false} {
		want := []string{moduleInfoIcon + " No modules to update", moduleLockFileLine(lockWritten)}
		if got := moduleNoUpdatesLines(lockWritten); !slices.Equal(got, want) {
			t.Fatalf("moduleNoUpdatesLines(%t) = %q, want %q", lockWritten, got, want)
		}
	}
}

func mustDeclarationEditResult(t *testing.T, updated bool, err error) modulesync.DeclarationEditResult {
	t.Helper()

	result, resultErr := modulesync.NewDeclarationEditResult(updated, err)
	if resultErr != nil {
		t.Fatalf("NewDeclarationEditResult() error = %v", resultErr)
	}
	return result
}
