// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/dag"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ArgsSubcommandConflictError is returned when a command defines both
// positional arguments and has subcommands. This is a structural error because
// positional arguments can only be accepted by leaf commands (commands without
// subcommands). When a command has subcommands, args passed to it are interpreted
// as subcommand names, making positional arguments unreachable.
type ArgsSubcommandConflictError struct {
	// CommandName is the name of the conflicting command
	CommandName string
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
		sb.WriteString(arg.Name)
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

// ValidateExecutionDAG checks for cycles among commands connected by execute: true
// dependencies. Only deps with Execute=true form graph edges; discoverability-only
// deps are ignored. Also validates that commands referenced by execute deps do not
// have required args or flags (since execute deps run without arguments).
//
// Returns a dag.CycleError if cycles exist, or a descriptive error for
// required args/flags violations.
func ValidateExecutionDAG(commands []*CommandInfo) error {
	if len(commands) == 0 {
		return nil
	}

	// Index commands by name for cross-reference lookups.
	byName := make(map[string]*CommandInfo, len(commands))
	for _, cmd := range commands {
		if cmd != nil && cmd.Command != nil {
			byName[cmd.Name] = cmd
		}
	}

	g := dag.New()

	for _, cmd := range commands {
		if cmd == nil || cmd.Command == nil {
			continue
		}

		// Collect execute deps from all levels (root + command + implementations).
		// Root-level deps come from the parent Invowkfile and apply to all commands.
		var execDeps []invowkfile.CommandDependency
		if cmd.Invowkfile != nil && cmd.Invowkfile.DependsOn != nil {
			execDeps = append(execDeps, cmd.Invowkfile.DependsOn.GetExecutableCommandDeps()...)
		}
		if cmd.Command.DependsOn != nil {
			execDeps = append(execDeps, cmd.Command.DependsOn.GetExecutableCommandDeps()...)
		}
		for i := range cmd.Command.Implementations {
			impl := &cmd.Command.Implementations[i]
			if impl.DependsOn != nil {
				execDeps = append(execDeps, impl.DependsOn.GetExecutableCommandDeps()...)
			}
		}

		if len(execDeps) == 0 {
			continue
		}

		g.AddNode(cmd.Name)
		for _, dep := range execDeps {
			// Add edges for ALL alternatives — each could form a cycle, so all must
			// be checked. At execution time, only the first discoverable alternative is used.
			for _, alt := range dep.Alternatives {
				g.AddEdge(alt, cmd.Name)

				// Validate that the dependency command has no required args/flags.
				if target, ok := byName[alt]; ok {
					if err := validateNoRequiredInputs(target, cmd.Name); err != nil {
						return err
					}
				}
			}
		}
	}

	_, err := g.TopologicalSort()
	return err
}

// validateNoRequiredInputs checks that a command has no required args or flags.
// Commands used as execute dependencies cannot receive arguments, so required
// inputs would always fail.
func validateNoRequiredInputs(target *CommandInfo, parentName string) error {
	for _, arg := range target.Command.Args {
		if arg.Required {
			return fmt.Errorf(
				"command '%s' has execute dependency on '%s', but '%s' has required argument '%s'",
				parentName, target.Name, target.Name, arg.Name,
			)
		}
	}
	for _, flag := range target.Command.Flags {
		if flag.Required {
			return fmt.Errorf(
				"command '%s' has execute dependency on '%s', but '%s' has required flag '--%s'",
				parentName, target.Name, target.Name, flag.Name,
			)
		}
	}
	return nil
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
	commandsWithArgs := make(map[string]*CommandInfo)

	// Track parent-child relationships
	childCommands := make(map[string][]string)

	for _, cmdInfo := range commands {
		if cmdInfo == nil || cmdInfo.Command == nil {
			continue
		}

		if len(cmdInfo.Command.Args) > 0 {
			commandsWithArgs[cmdInfo.Name] = cmdInfo
		}

		// Record parent-child relationships by splitting the command name
		parts := strings.Fields(cmdInfo.Name)
		for i := 1; i < len(parts); i++ {
			parentName := strings.Join(parts[:i], " ")
			childCommands[parentName] = append(childCommands[parentName], cmdInfo.Name)
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
