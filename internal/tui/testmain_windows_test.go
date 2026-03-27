// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"os"
	"runtime"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestMain addresses two Windows-specific TUI test issues:
//
//  1. Race detector crash: lipgloss/termenv use Windows Console API calls for
//     terminal detection that are not safe under the race detector when invoked
//     concurrently from t.Parallel() goroutines. Pre-initializing here caches
//     the result before parallel tests start.
//
//  2. Race detector timeout: 281 parallel tests with race instrumentation add
//     ~10x overhead, exceeding Go's per-package timeout on slow CI runners.
//     Setting GOMAXPROCS(1) limits t.Parallel() concurrency to 1, which
//     eliminates both the race and the resource pressure.
func TestMain(m *testing.M) {
	runtime.GOMAXPROCS(1)
	_ = lipgloss.NewStyle().Render("")
	os.Exit(m.Run())
}
