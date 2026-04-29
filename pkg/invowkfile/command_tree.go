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

// ValidateCommandTree checks the command-model invariant that positional
// arguments can only be defined on leaf commands.
func ValidateCommandTree(commands []CommandTreeEntry) error {
	if len(commands) == 0 {
		return nil
	}

	commandsWithArgs := make(map[CommandName]CommandTreeEntry)
	childCommands := make(map[CommandName][]CommandName)

	for _, entry := range commands {
		if entry.Command == nil {
			continue
		}
		if len(entry.Command.Args) > 0 {
			commandsWithArgs[entry.Name] = entry
		}
		parts := strings.Fields(string(entry.Name))
		for i := 1; i < len(parts); i++ {
			parentName := CommandName(strings.Join(parts[:i], " ")) //goplint:ignore -- derived from valid command name parts
			childCommands[parentName] = append(childCommands[parentName], entry.Name)
		}
	}

	for cmdName, entry := range commandsWithArgs {
		if children, hasChildren := childCommands[cmdName]; hasChildren {
			return &ArgsSubcommandConflictError{
				CommandName: cmdName,
				Args:        entry.Command.Args,
				Subcommands: children,
				FilePath:    entry.FilePath,
			}
		}
	}

	return nil
}
