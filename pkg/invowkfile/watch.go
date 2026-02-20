// SPDX-License-Identifier: MPL-2.0

package invowkfile

// WatchConfig defines file-watching behavior for automatic command re-execution.
type WatchConfig struct {
	// Patterns lists glob patterns for files to watch (required, at least one).
	// Supports ** for recursive matching (e.g., "src/**/*.go").
	// Paths are relative to the effective working directory.
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
