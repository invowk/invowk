// SPDX-License-Identifier: MPL-2.0

// Package modulelock is the operating-system adapter behind the module
// commands' edit locks: it takes and releases an exclusive advisory lock on a
// lock file (flock on Unix, LockFileEx on Windows) and resolves a directory to
// its physical path. It decides nothing about which files are locked or for how
// long; internal/app/modulesync owns that policy through its EditLocker port.
//
// The OS releases a lock when its descriptor closes or its process dies, so no
// lock can go stale. Lock files are opened, never truncated, and never deleted.
package modulelock
