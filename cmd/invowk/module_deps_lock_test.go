// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"slices"
	"testing"
)

// TestModuleLockLines covers every row of the lock-outcome wording that module
// sync, update, add and remove print (spec "Sync and update say whether the
// lock changed" and "Add and remove report each file truthfully").
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
		{name: "remove written", render: moduleRemoveSummaryLine, lockWritten: true, want: moduleSuccessIcon + " Lock file and invowkmod.cue updated"},
		{name: "remove skipped", render: moduleRemoveSummaryLine, lockWritten: false, want: moduleSuccessIcon + " Lock file unchanged; invowkmod.cue updated"},
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
