// SPDX-License-Identifier: MPL-2.0

//go:build windows && race

package tui

import (
	"os"
	"runtime"
	"testing"
)

// TestMain limits parallelism when the race detector is active on Windows.
//
// The race detector's 10x memory overhead combined with 390+ parallel TUI tests
// exceeds memory limits on windows-latest CI runners. GOMAXPROCS(1) serializes
// test execution to stay within bounds.
//
// Without -race, no TestMain exists for Windows — tests run in full parallel
// (like Linux/macOS), completing in ~3 minutes instead of ~15 minutes.
//
// Thread safety is NOT a concern here: lipgloss Style.Render() is a pure value
// type with zero global state, bubbletea Model.Init()/View() have no shared
// state outside Program, and terminal detection runs once at package init.
// See .agents/skills/windows-testing/SKILL.md § "Charm Library Thread Safety".
func TestMain(m *testing.M) {
	runtime.GOMAXPROCS(1)
	os.Exit(m.Run())
}
