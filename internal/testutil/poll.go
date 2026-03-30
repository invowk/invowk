// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"testing"
	"time"
)

// PollUntil calls check repeatedly at the given interval until it returns true
// or the timeout expires. Returns true if the condition was met, false on timeout.
//
// Use this for event-driven assertions where the exact timing is non-deterministic
// (e.g., waiting for a file watcher callback, tmux pane rendering, or goroutine
// completion). Prefer this over bare time.Sleep + assert.
func PollUntil(t testing.TB, timeout, interval time.Duration, check func() bool) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}

		time.Sleep(interval)
	}

	return false
}

// RequirePollUntil is like PollUntil but calls t.Fatalf on timeout with the
// given message. Use this when the condition must be met for the test to proceed.
func RequirePollUntil(t testing.TB, timeout, interval time.Duration, msg string, check func() bool) {
	t.Helper()

	if !PollUntil(t, timeout, interval, check) {
		t.Fatalf("timed out after %v: %s", timeout, msg)
	}
}

// AssertNeverTrue verifies that check never returns true within the wait period.
// This is the inverse of PollUntil — it proves that something does NOT happen.
//
// Use this for negative assertions such as "no spurious callback fired" or
// "lock was not acquired while held by another goroutine". The wait duration
// should be long enough to be confident the event would have occurred if the
// code were buggy, but short enough to keep tests fast.
func AssertNeverTrue(t testing.TB, wait, interval time.Duration, msg string, check func() bool) {
	t.Helper()

	if PollUntil(t, wait, interval, check) {
		t.Fatalf("condition unexpectedly became true: %s", msg)
	}
}
