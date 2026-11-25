// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/modulelock"
)

// moduleEditLocks returns the edit locks of one command run: a fresh
// modulelock.FileLocker, and a wait callback printing moduleLockWaitLine to
// stderr. The cache root is the default (INVOWK_MODULES_PATH or
// ~/.invowk/modules).
func moduleEditLocks() modulesync.EditLocks {
	return modulesync.EditLocks{
		Locker: modulelock.NewFileLocker(),
		OnWait: func(wait modulesync.LockWait) {
			fmt.Fprintln(os.Stderr, moduleLockWaitLine(wait))
		},
	}
}

// moduleLockWaitLine returns the line a module command prints once when it
// finds a lock busy and starts waiting for it.
//
//plint:render
func moduleLockWaitLine(wait modulesync.LockWait) string {
	if wait.Kind == modulesync.LockWaitSource {
		return fmt.Sprintf("%s Waiting for another invowk module command on module source %s (lock: %s)", moduleInfoIcon, wait.GitURL, wait.LockPath)
	}
	return fmt.Sprintf("%s Waiting for another invowk module command on this project (lock: %s)", moduleInfoIcon, wait.LockPath)
}
