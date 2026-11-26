// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"io/fs"
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

// TestModuleRemoveSummaryLine covers the summary rows of module remove (spec
// "Add and remove report each file truthfully" and "A failed invowkmod.cue
// edit reports the rollback"), including a restore that failed as well.
func TestModuleRemoveSummaryLine(t *testing.T) {
	t.Parallel()

	wrote := mustDeclarationEditResult(t, true, nil, modulesync.DeclarationRollbackNone)
	skipped := mustDeclarationEditResult(t, false, nil, modulesync.DeclarationRollbackNone)
	// The remove use case reports the rollback outcome with the edit error,
	// joined with the restore errors when a restore failed.
	readErr := errors.New("failed to read invowkmod.cue: is a directory")
	failed := mustDeclarationEditResult(t, false, readErr, modulesync.DeclarationRollbackComplete)
	restoreFailed := mustDeclarationEditResult(t, false, errors.Join(readErr, errors.New("rollback invowkmod.lock.cue: permission denied")), modulesync.DeclarationRollbackIncomplete)
	rollback := moduleInfoIcon + " Lock file changes were rolled back; invowkmod.cue needs manual review"
	restoreFailedLine := moduleInfoIcon + " Lock file rollback may be incomplete; invowkmod.cue needs manual review"

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
		{name: "edit failed, a restore failed", declarations: []modulesync.DeclarationEditResult{restoreFailed}, lockWritten: true, want: restoreFailedLine},
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

// TestModuleAddDeclarationFailureLine covers "Add without invowkmod.cue",
// "Add whose invowkmod.cue edit fails otherwise" and "A restore that fails as
// well" on what the add use case returns: the rollback outcome, and the
// AddRequirement error joined with the restore errors that failed.
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
	duplicateErr := invowkmod.AddRequirement(types.FilesystemPath(duplicatePath), ref)
	if !errors.Is(duplicateErr, invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("second AddRequirement() error = %v, want ErrModuleAlreadyExists", duplicateErr)
	}

	notFound := moduleInfoIcon + " invowkmod.cue not found — dependency was not persisted"
	rollbackPrefix := moduleInfoIcon + " Could not auto-edit invowkmod.cue; lock file changes were rolled back: "
	incompletePrefix := moduleInfoIcon + " Could not auto-edit invowkmod.cue; lock file rollback may be incomplete: "
	missingRestoreFailed := errors.Join(missingErr, errors.New("rollback invowkmod.lock.cue: permission denied"))
	// A restore that fails with its own not-exist error must not turn a
	// duplicate-requirement failure into the not-found line.
	duplicateRestoreNotExist := errors.Join(invowkmod.ErrModuleAlreadyExists, &fs.PathError{Op: "open", Path: "invowkmod.lock.cue", Err: fs.ErrNotExist})
	tests := []struct {
		name     string
		err      error
		rollback modulesync.DeclarationRollback
		want     string
	}{
		{name: "missing file, restores succeeded", err: missingErr, rollback: modulesync.DeclarationRollbackComplete, want: notFound},
		{name: "missing file, a restore failed", err: missingRestoreFailed, rollback: modulesync.DeclarationRollbackIncomplete, want: incompletePrefix + missingRestoreFailed.Error()},
		{name: "duplicate requirement", err: duplicateErr, rollback: modulesync.DeclarationRollbackComplete, want: rollbackPrefix + duplicateErr.Error()},
		{name: "duplicate requirement, restore not found", err: duplicateRestoreNotExist, rollback: modulesync.DeclarationRollbackIncomplete, want: incompletePrefix + duplicateRestoreNotExist.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			declaration := mustDeclarationEditResult(t, false, tt.err, tt.rollback)
			if got := moduleAddDeclarationFailureLine(declaration); got != tt.want {
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

// TestModuleDeclarationExitError covers the exit status of module add and
// module remove after a rollback (spec "A failed invowkmod.cue edit exits
// with status 1"): both outcomes of a rollback exit 1 and carry the
// declaration error; an edit that did not roll back returns nil. module add
// has no offline testscript case (its resolve needs a Git fetch), so this test
// covers the error runModuleAdd returns.
func TestModuleDeclarationExitError(t *testing.T) {
	t.Parallel()

	editErr := errors.New("failed to read invowkmod.cue: is a directory")
	tests := []struct {
		name     string
		err      error
		rollback modulesync.DeclarationRollback
		wantCode types.ExitCode
	}{
		{name: "no rollback", rollback: modulesync.DeclarationRollbackNone, wantCode: 0},
		{name: "complete rollback", err: editErr, rollback: modulesync.DeclarationRollbackComplete, wantCode: 1},
		{name: "incomplete rollback", err: errors.Join(editErr, errors.New("restore failed")), rollback: modulesync.DeclarationRollbackIncomplete, wantCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := moduleDeclarationExitError(mustDeclarationEditResult(t, false, tt.err, tt.rollback))
			if got := exitCodeForError(err); got != tt.wantCode {
				t.Fatalf("exitCodeForError(%v) = %d, want %d", err, got, tt.wantCode)
			}
			if tt.err == nil {
				if err != nil {
					t.Fatalf("moduleDeclarationExitError() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, editErr) {
				t.Fatalf("moduleDeclarationExitError() = %v, want it to carry the declaration error", err)
			}
		})
	}
}

func mustDeclarationEditResult(t *testing.T, updated bool, err error, rollback modulesync.DeclarationRollback) modulesync.DeclarationEditResult {
	t.Helper()

	result, resultErr := modulesync.NewDeclarationEditResult(updated, err, rollback)
	if resultErr != nil {
		t.Fatalf("NewDeclarationEditResult() error = %v", resultErr)
	}
	return result
}
