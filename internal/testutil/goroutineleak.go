// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
)

// AssertNoGoroutineLeaks asserts that no goroutines attributable to attributions
// remain blocked on unreachable synchronization primitives, using the Go 1.27
// "goroutineleak" pprof profile.
//
// Call after driving a server (or other resource) to a terminal stopped state.
// Attributions are substrings matched against the debug=1 profile output (which
// contains fully-qualified function names and file paths); at least one entry
// in attributions must be present in a leaked goroutine's stack trace for that
// leak to be flagged. Typical attributions are the package's import path (for
// example, "github.com/invowk/invowk/internal/sshserver") and any receiver
// packages the server delegates to.
//
// Determinism: Profile.WriteTo internally invokes a goroutine-leak GC cycle
// that determines reachability of synchronization primitives; no time.Sleep is
// required. An explicit runtime.GC() precedes the write to eagerly reclaim any
// recently-finished goroutines whose sync primitives would otherwise still
// appear reachable through stale stack references.
//
// Parallelism: The pprof profile is process-wide. Do NOT call t.Parallel() on
// tests that use this helper unless the caller can guarantee attribution
// uniqueness across parallel tests in the same package.
func AssertNoGoroutineLeaks(t testing.TB, attributions ...string) {
	t.Helper()

	if len(attributions) == 0 {
		t.Fatalf("AssertNoGoroutineLeaks: at least one attribution string required")
	}

	profile := pprof.Lookup("goroutineleak")
	if profile == nil {
		t.Fatalf("goroutineleak profile not available (Go 1.27+ required)")
	}

	// Eagerly reclaim goroutines that have already returned so their sync
	// primitives become unreachable in this cycle rather than the next.
	runtime.GC()

	var buf bytes.Buffer
	if err := profile.WriteTo(&buf, 1); err != nil {
		t.Fatalf("goroutineleak profile: %v", err)
	}

	leaks := extractAttributedGoroutineLeaks(buf.String(), attributions)
	if len(leaks) == 0 {
		return
	}
	t.Errorf(
		"goroutineleak profile reports %d leak(s) attributable to %v:\n%s",
		len(leaks),
		attributions,
		strings.Join(leaks, "\n---\n"),
	)
}

// extractAttributedGoroutineLeaks parses the debug=1 goroutineleak profile
// output and returns the stack sections whose text contains at least one of
// the attribution substrings.
//
// The debug=1 format groups each leak as:
//
//	N @ 0xAAA 0xBBB ...
//	#	0xAAA	pkg.Func+0x33	/path/to/file.go:LINE
//	#	...
//
// followed by a blank line. Header lines like
// "goroutineleak profile: total N" and any trailing debug metadata are
// ignored.
func extractAttributedGoroutineLeaks(profileText string, attributions []string) []string {
	var (
		leaks   []string
		current strings.Builder
		inEntry bool
	)

	flush := func() {
		if !inEntry {
			return
		}
		block := strings.TrimRight(current.String(), "\n")
		current.Reset()
		inEntry = false
		if block == "" {
			return
		}
		if !containsAny(block, attributions) {
			return
		}
		leaks = append(leaks, block)
	}

	for line := range strings.SplitSeq(profileText, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case isLeakEntryHeader(trimmed):
			flush()
			inEntry = true
			current.WriteString(line)
			current.WriteByte('\n')
		case inEntry && (trimmed == "" || (!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "@"))):
			// A blank line, banner, or metadata line ends the current entry.
			flush()
		case inEntry:
			current.WriteString(line)
			current.WriteByte('\n')
		}
	}
	flush()

	return leaks
}

// isLeakEntryHeader reports whether the trimmed line looks like the leader of
// a leak entry ("N @ 0xAAA 0xBBB ...").
func isLeakEntryHeader(line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return false
	}
	if fields[1] != "@" {
		return false
	}
	var count int
	if _, err := fmt.Sscanf(fields[0], "%d", &count); err != nil {
		return false
	}
	return count > 0
}

// containsAny reports whether any of needles is a substring of haystack.
func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
