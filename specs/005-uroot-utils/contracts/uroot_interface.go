// SPDX-License-Identifier: MPL-2.0

// Package contracts defines the interfaces for the u-root utils integration.
// This file is a design document - it will be moved to internal/uroot/ during implementation.
package contracts

import (
	"context"
	"io"
)

// UrootCommand defines the interface for u-root utility implementations.
// Each utility (cat, cp, ls, etc.) implements this interface.
type UrootCommand interface {
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

// FlagInfo describes a supported flag.
type FlagInfo struct {
	// Name is the flag name without dashes (e.g., "r" for -r, "recursive" for --recursive).
	Name string
	// ShortName is the single-character alias (e.g., "r" for --recursive).
	ShortName string
	// Description explains what the flag does.
	Description string
	// TakesValue indicates if the flag requires a value (e.g., -n 10).
	TakesValue bool
}

// HandlerContext provides execution context for u-root commands.
// This is extracted from mvdan/sh's interp.HandlerCtx.
type HandlerContext struct {
	// Stdin is the input stream for the command.
	Stdin io.Reader
	// Stdout is the output stream for the command.
	Stdout io.Writer
	// Stderr is the error output stream for the command.
	Stderr io.Writer
	// Dir is the current working directory.
	Dir string
	// LookupEnv retrieves environment variables.
	LookupEnv func(string) (string, bool)
}

// Registry manages the mapping of command names to their implementations.
type Registry interface {
	// Register adds a command to the registry.
	// Panics if a command with the same name is already registered.
	Register(cmd UrootCommand)

	// Lookup retrieves a command by name.
	// Returns nil, false if the command is not registered.
	Lookup(name string) (UrootCommand, bool)

	// Names returns the names of all registered commands.
	Names() []string

	// Run executes a command by name with the given context and arguments.
	// Returns an error if the command is not found or execution fails.
	Run(ctx context.Context, name string, args []string) error
}
