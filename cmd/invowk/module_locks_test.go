// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/types"
)

// TestModuleLockWaitLine covers the two waiting lines a module command prints
// on stderr when it finds a lock busy (design D6).
func TestModuleLockWaitLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		wait modulesync.LockWait
		want string
	}{
		{
			name: "project",
			wait: modulesync.LockWait{Kind: modulesync.LockWaitProject, LockPath: "/cache/locks/project-ab.lock"},
			want: moduleInfoIcon + " Waiting for another invowk module command on this project (lock: /cache/locks/project-ab.lock)",
		},
		{
			name: "source",
			wait: modulesync.LockWait{Kind: modulesync.LockWaitSource, LockPath: "/cache/locks/source-cd.lock", GitURL: "https://github.com/user/tools.git"},
			want: moduleInfoIcon + " Waiting for another invowk module command on module source https://github.com/user/tools.git (lock: /cache/locks/source-cd.lock)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := moduleLockWaitLine(tt.wait); got != tt.want {
				t.Fatalf("moduleLockWaitLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestModuleLockWaitTimeoutExitCode passes each locking command's wrapped
// lock timeout through exitCodeForError (design D4): a timed-out wait exits 1.
func TestModuleLockWaitTimeoutExitCode(t *testing.T) {
	t.Parallel()

	timeout := &modulelock.TimeoutError{Path: types.FilesystemPath("/cache/locks/project-ab.lock"), Timeout: 5 * time.Minute}
	// Each command returns the use case error as is, or wrapped as its error
	// path wraps it (vendor: "failed to resolve dependencies: %w").
	for _, command := range []string{"add", "remove", "sync", "update", "tidy", "vendor", "agent mod change"} {
		err := fmt.Errorf("module %s: %w", command, timeout)
		if got := exitCodeForError(err); got != 1 {
			t.Fatalf("%s: exitCodeForError(%v) = %d, want 1", command, err, got)
		}
	}
}
