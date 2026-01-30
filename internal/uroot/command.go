// SPDX-License-Identifier: MPL-2.0

package uroot

import "context"

type (
	// Command defines the interface for u-root utility implementations.
	// Each utility (cat, cp, ls, etc.) implements this interface.
	Command interface {
		// Name returns the command name (e.g., "cp", "ls", "cat").
		Name() string

		// Run executes the command with the given context and arguments.
		// The context contains the HandlerContext with stdin/stdout/stderr.
		// args[0] is the command name (for error messages), args[1:] are the arguments.
		// Returns nil on success, or an error prefixed with "[uroot] <cmd>:".
		Run(ctx context.Context, args []string) error

		// SupportedFlags returns the flags this implementation supports.
		// Unsupported flags are silently ignored during execution.
		// This is used for documentation and introspection.
		SupportedFlags() []FlagInfo
	}

	// FlagInfo describes a supported flag for a u-root command.
	FlagInfo struct {
		// Name is the flag name without dashes (e.g., "r" for -r, "recursive" for --recursive).
		Name string
		// ShortName is the single-character alias (e.g., "r" for --recursive).
		// Empty if no short form exists.
		ShortName string
		// Description explains what the flag does.
		Description string
		// TakesValue indicates if the flag requires a value (e.g., -n 10).
		TakesValue bool
	}
)
