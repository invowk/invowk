// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidGlobPattern is the sentinel error wrapped by InvalidGlobPatternError.
	ErrInvalidGlobPattern = errors.New("invalid glob pattern")
	// ErrInvalidWatchConfig is the sentinel error wrapped by InvalidWatchConfigError.
	ErrInvalidWatchConfig = errors.New("invalid watch config")
)

type (
	// GlobPattern represents a file-matching glob pattern (e.g., "**/*.go", "src/**/*.ts").
	// Must be non-empty.
	GlobPattern string

	// InvalidGlobPatternError is returned when a GlobPattern value is empty or
	// syntactically invalid. It wraps ErrInvalidGlobPattern for errors.Is() compatibility.
	InvalidGlobPatternError struct {
		Value  GlobPattern
		Reason string
	}

	// InvalidWatchConfigError is returned when a WatchConfig has invalid fields.
	// It wraps ErrInvalidWatchConfig for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidWatchConfigError struct {
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// WatchConfig defines file-watching behavior for automatic command re-execution.
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
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
	return fmt.Sprintf("invalid glob pattern %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidGlobPattern for errors.Is() compatibility.
func (e *InvalidGlobPatternError) Unwrap() error { return ErrInvalidGlobPattern }

// Validate returns nil if the GlobPattern is valid, or a validation error if not.
// A valid GlobPattern must not be empty and must have valid glob syntax.
//
//goplint:nonzero
func (g GlobPattern) Validate() error {
	if g == "" {
		return &InvalidGlobPatternError{Value: g, Reason: "must not be empty"}
	}
	if _, err := doublestar.Match(string(g), ""); err != nil {
		return &InvalidGlobPatternError{Value: g, Reason: fmt.Sprintf("invalid syntax: %v", err)}
	}
	return nil
}

// String returns the string representation of the GlobPattern.
func (g GlobPattern) String() string { return string(g) }

// Validate returns nil if the WatchConfig has valid fields,
// or an error collecting all field-level validation failures.
// Delegates to GlobPattern.Validate() for Patterns and Ignore,
// and DurationString.Validate() for Debounce.
func (w WatchConfig) Validate() error {
	var errs []error
	for _, p := range w.Patterns {
		if err := p.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := w.Debounce.Validate(); err != nil {
		errs = append(errs, err)
	}
	for _, ig := range w.Ignore {
		if err := ig.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidWatchConfigError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidWatchConfigError.
func (e *InvalidWatchConfigError) Error() string {
	return types.FormatFieldErrors("watch config", e.FieldErrors)
}

// Unwrap returns ErrInvalidWatchConfig for errors.Is() compatibility.
func (e *InvalidWatchConfigError) Unwrap() error { return ErrInvalidWatchConfig }

// ParseDebounce parses the Debounce field into a time.Duration.
// Returns (0, nil) when Debounce is empty (caller should apply default).
// Returns an error for zero or negative durations, which are invalid
// for debounce timing. Callers treat this error as fatal rather than
// falling back to a default.
func (w *WatchConfig) ParseDebounce() (time.Duration, error) {
	return parseDuration("debounce", w.Debounce)
}
