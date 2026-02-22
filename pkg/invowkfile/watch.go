// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidGlobPattern is the sentinel error wrapped by InvalidGlobPatternError.
var ErrInvalidGlobPattern = errors.New("invalid glob pattern")

type (
	// GlobPattern represents a file-matching glob pattern (e.g., "**/*.go", "src/**/*.ts").
	// Must be non-empty.
	GlobPattern string

	// InvalidGlobPatternError is returned when a GlobPattern value is empty.
	// It wraps ErrInvalidGlobPattern for errors.Is() compatibility.
	InvalidGlobPatternError struct {
		Value GlobPattern
	}

	// WatchConfig defines file-watching behavior for automatic command re-execution.
	WatchConfig struct {
		// Patterns lists glob patterns for files to watch.
		// Supports ** for recursive matching (e.g., "src/**/*.go").
		// Paths are relative to the effective working directory.
		// CUE schema enforces at least one pattern; the CLI falls back to ["**/*"]
		// when no watch config is defined at all.
		Patterns []GlobPattern `json:"patterns"`
		// Debounce specifies the delay before re-executing after a change.
		// Must be a valid Go duration string. Default: "500ms".
		Debounce DurationString `json:"debounce,omitempty"`
		// ClearScreen clears the terminal before each re-execution.
		ClearScreen bool `json:"clear_screen,omitempty"`
		// Ignore lists glob patterns for files/directories to exclude from watching.
		// Common ignores (.git, node_modules) are applied by default.
		Ignore []GlobPattern `json:"ignore,omitempty"`
	}
)

// Error implements the error interface for InvalidGlobPatternError.
func (e *InvalidGlobPatternError) Error() string {
	return fmt.Sprintf("invalid glob pattern %q (must not be empty)", e.Value)
}

// Unwrap returns ErrInvalidGlobPattern for errors.Is() compatibility.
func (e *InvalidGlobPatternError) Unwrap() error { return ErrInvalidGlobPattern }

// IsValid returns whether the GlobPattern is valid.
// A valid GlobPattern must not be empty.
func (g GlobPattern) IsValid() (bool, []error) {
	if g == "" {
		return false, []error{&InvalidGlobPatternError{Value: g}}
	}
	return true, nil
}

// String returns the string representation of the GlobPattern.
func (g GlobPattern) String() string { return string(g) }

// ParseDebounce parses the Debounce field into a time.Duration.
// Returns (0, nil) when Debounce is empty (caller should apply default).
// Returns an error for zero or negative durations, which are invalid
// for debounce timing. Callers treat this error as fatal rather than
// falling back to a default.
func (w *WatchConfig) ParseDebounce() (time.Duration, error) {
	return parseDuration("debounce", w.Debounce)
}
