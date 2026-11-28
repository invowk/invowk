// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

type (
	// CommandTreeEntry is the minimal command-model data needed to validate
	// command tree invariants independently from discovery DTOs.
	CommandTreeEntry struct {
		Name     CommandName
		Command  *Command
		FilePath types.FilesystemPath
	}

	// ArgsSubcommandConflictError is returned when a command defines both
	// positional arguments and has subcommands.
	ArgsSubcommandConflictError struct {
		CommandName CommandName
		Args        []Argument
		Subcommands []CommandName
		FilePath    types.FilesystemPath
	}

	// PassthroughSubcommandConflictError is returned when a command enables
	// passthrough and also has subcommands.
	PassthroughSubcommandConflictError struct {
		CommandName CommandName          //goplint:ignore -- error diagnostic context
		Subcommands []CommandName        //goplint:ignore -- error diagnostic context
		FilePath    types.FilesystemPath //goplint:ignore -- optional source file path in error diagnostic
	}
)

// Error implements the error interface.
func (e *ArgsSubcommandConflictError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "command '%s' has both args and subcommands", e.CommandName)
	if e.FilePath != "" {
		fmt.Fprintf(&sb, " in %s", string(e.FilePath))
	}
	sb.WriteString("\n  defined args: ")
	for i, arg := range e.Args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(arg.Name))
	}
	sb.WriteString("\n  subcommands: ")
	for i, sub := range e.Subcommands {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(sub))
	}
	return sb.String()
}

// Error implements the error interface.
func (e *PassthroughSubcommandConflictError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "command '%s' has passthrough enabled and also has subcommands", e.CommandName)
	if e.FilePath != "" {
		fmt.Fprintf(&sb, " in %s", string(e.FilePath))
	}
	sb.WriteString("\n  subcommands: ")
	for i, sub := range e.Subcommands {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(sub))
	}
	return sb.String()
}

// Validate returns nil when the entry contains valid typed fields.
func (e CommandTreeEntry) Validate() error {
	var errs []error
	if err := e.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if e.Command != nil {
		if err := e.Command.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if e.FilePath != "" {
		if err := e.FilePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ValidateCommandTree checks the command-model invariants that positional
// arguments and passthrough mode can only be used on leaf commands without subcommands.
func ValidateCommandTree(commands []CommandTreeEntry) error {
	if len(commands) == 0 {
		return nil
	}

	childCommands := make(map[string][]CommandName)

	for _, entry := range commands {
		if entry.Command == nil {
			continue
		}
		parts := strings.Fields(string(entry.Name))
		for i := 1; i < len(parts); i++ {
			parentName := strings.Join(parts[:i], " ")
			childCommands[parentName] = append(childCommands[parentName], entry.Name)
		}
	}

	for _, entry := range commands {
		if entry.Command == nil {
			continue
		}
		children, hasChildren := childCommands[string(entry.Name)]
		if !hasChildren {
			continue
		}
		if len(entry.Command.Args) > 0 {
			return &ArgsSubcommandConflictError{
				CommandName: entry.Name,
				Args:        entry.Command.Args,
				Subcommands: children,
				FilePath:    entry.FilePath,
			}
		}
		if entry.Command.Passthrough {
			return &PassthroughSubcommandConflictError{
				CommandName: entry.Name,
				Subcommands: children,
				FilePath:    entry.FilePath,
			}
		}
	}

	return nil
}
