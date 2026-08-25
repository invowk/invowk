// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"runtime"
	"strings"
	"sync"
	"testing"
)

// TestAssertNoGoroutineLeaks_CleanReturnsQuietly verifies the helper reports
// no leaks when all goroutines a test spawned have completed.
//
//nolint:paralleltest // Leak profile is process-wide; parallel tests create attribution ambiguity.
func TestAssertNoGoroutineLeaks_CleanReturnsQuietly(t *testing.T) {
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			// no-op
		})
	}
	wg.Wait()
	runtime.GC()

	// The helper must not report leaks attributable to this package for a
	// clean lifecycle.
	AssertNoGoroutineLeaks(t, "github.com/invowk/invowk/internal/testutil/leakcheck_never_matches")
}

// TestExtractAttributedGoroutineLeaks_ParsesDebug1 verifies the parser
// isolates leak blocks by attribution string.
func TestExtractAttributedGoroutineLeaks_ParsesDebug1(t *testing.T) {
	t.Parallel()

	profile := `goroutineleak profile: total 2
1 @ 0x123 0x456
#	0x122	github.com/example/pkg1.leakingFunc+0x11	/path/pkg1/file.go:12
#	0x455	github.com/other/pkg.helper+0x22	/path/other/file.go:34

1 @ 0x789 0xabc
#	0x788	github.com/example/pkg2.otherLeak+0x11	/path/pkg2/file.go:56
`

	leaks := extractAttributedGoroutineLeaks(profile, []string{"github.com/example/pkg1"})
	if len(leaks) != 1 {
		t.Fatalf("got %d leaks, want 1", len(leaks))
	}
	if !strings.Contains(leaks[0], "pkg1.leakingFunc") {
		t.Fatalf("leak missing pkg1 frame: %q", leaks[0])
	}

	both := extractAttributedGoroutineLeaks(profile, []string{"github.com/example/pkg1", "github.com/example/pkg2"})
	if len(both) != 2 {
		t.Fatalf("got %d leaks, want 2", len(both))
	}

	none := extractAttributedGoroutineLeaks(profile, []string{"github.com/nothing"})
	if len(none) != 0 {
		t.Fatalf("got %d leaks, want 0", len(none))
	}

	empty := extractAttributedGoroutineLeaks("", []string{"anything"})
	if len(empty) != 0 {
		t.Fatalf("empty profile: got %d leaks, want 0", len(empty))
	}
}
