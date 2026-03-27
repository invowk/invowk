// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"os"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestMain pre-initializes lipgloss terminal detection before any parallel
// tests start. The Charm library stack (bubbletea/lipgloss/termenv) caches
// the terminal color profile after the first query. On Windows, this query
// uses Windows Console API calls that are not safe under the race detector
// when invoked concurrently from multiple t.Parallel() goroutines. By
// triggering the detection once here in the main goroutine, all subsequent
// parallel test calls use the cached result without racing.
func TestMain(m *testing.M) {
	_ = lipgloss.NewStyle().Render("")
	os.Exit(m.Run())
}
