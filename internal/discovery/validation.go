// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// ArgsSubcommandConflictError is returned when a command defines both
// positional arguments and has subcommands. This is a structural error because
// positional arguments can only be accepted by leaf commands (commands without
// subcommands). When a command has subcommands, args passed to it are interpreted
// as subcommand names, making positional arguments unreachable.
type ArgsSubcommandConflictError struct {
	// CommandName is the name of the conflicting command
	CommandName invowkfile.CommandName
	// Args are the positional arguments defined on the command
	Args []invowkfile.Argument
	// Subcommands are the child command names
	Subcommands []string
	// FilePath is the path to the invowkfile containing this command
	FilePath string
}

// Error implements the error interface.
func (e *ArgsSubcommandConflictError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "command '%s' has both args and subcommands", e.CommandName)
	if e.FilePath != "" {
		fmt.Fprintf(&sb, " in %s", e.FilePath)
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
		sb.WriteString(sub)
	}
	return sb.String()
}

// ValidateCommandTree checks for args/subcommand conflicts across all commands.
// Returns ArgsSubcommandConflictError if a command has both args and subcommands.
//
// The validation enforces a fundamental constraint: positional arguments can only
// be defined on leaf commands (commands without subcommands). This is because
// when a command has subcommands, any positional arguments would be interpreted
// as subcommand names by the CLI parser, making them unreachable.
func ValidateCommandTree(commands []*CommandInfo) error {
	if len(commands) == 0 {
		return nil
	}

	// Track commands with args
	commandsWithArgs := make(map[invowkfile.CommandName]*CommandInfo)

	// Track parent-child relationships
	childCommands := make(map[invowkfile.CommandName][]string)

	for _, cmdInfo := range commands {
		if cmdInfo == nil || cmdInfo.Command == nil {
			continue
		}

		if len(cmdInfo.Command.Args) > 0 {
			commandsWithArgs[cmdInfo.Name] = cmdInfo
		}

		// Record parent-child relationships by splitting the command name
		parts := strings.Fields(string(cmdInfo.Name))
		for i := 1; i < len(parts); i++ {
			parentName := invowkfile.CommandName(strings.Join(parts[:i], " "))
			childCommands[parentName] = append(childCommands[parentName], string(cmdInfo.Name))
		}
	}

	// Check for conflicts: commands that have both args and children
	for cmdName, cmdInfo := range commandsWithArgs {
		if children, hasChildren := childCommands[cmdName]; hasChildren {
			return &ArgsSubcommandConflictError{
				CommandName: cmdName,
				Args:        cmdInfo.Command.Args,
				Subcommands: children,
				FilePath:    cmdInfo.FilePath,
			}
		}
	}

	return nil
}
