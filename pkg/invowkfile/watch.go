// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"time"
)

// WatchConfig defines file-watching behavior for automatic command re-execution.
type WatchConfig struct {
	// Patterns lists glob patterns for files to watch.
	// Supports ** for recursive matching (e.g., "src/**/*.go").
	// Paths are relative to the effective working directory.
	// CUE schema enforces at least one pattern; the CLI falls back to ["**/*"]
	// when no watch config is defined at all.
	Patterns []string `json:"patterns"`
	// Debounce specifies the delay before re-executing after a change.
	// Must be a valid Go duration string. Default: "500ms".
	Debounce string `json:"debounce,omitempty"`
	// ClearScreen clears the terminal before each re-execution.
	ClearScreen bool `json:"clear_screen,omitempty"`
	// Ignore lists glob patterns for files/directories to exclude from watching.
	// Common ignores (.git, node_modules) are applied by default.
	Ignore []string `json:"ignore,omitempty"`
}

// ParseDebounce parses the Debounce field into a time.Duration.
// Returns (0, nil) when Debounce is empty (caller should apply default).
// Returns an error for zero or negative durations, which are invalid
// for debounce timing. Callers treat this error as fatal rather than
// falling back to a default.
func (w *WatchConfig) ParseDebounce() (time.Duration, error) {
	if w.Debounce == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(w.Debounce)
	if err != nil {
		return 0, fmt.Errorf("invalid debounce %q: %w", w.Debounce, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid debounce %q: must be a positive duration", w.Debounce)
	}
	return d, nil
}
