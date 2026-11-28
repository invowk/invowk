// SPDX-License-Identifier: MPL-2.0

package discovery

import "github.com/invowk/invowk/pkg/invowkfile"

type (
	// ArgsSubcommandConflictError is kept as a discovery-facing alias for callers
	// that still validate discovered command sets.
	ArgsSubcommandConflictError = invowkfile.ArgsSubcommandConflictError

	// PassthroughSubcommandConflictError is kept as a discovery-facing alias for callers
	// that validate discovered command sets.
	PassthroughSubcommandConflictError = invowkfile.PassthroughSubcommandConflictError
)

// ValidateCommandTree checks for args/subcommand and passthrough/subcommand conflicts
// across all commands. Returns ArgsSubcommandConflictError if a command has both args
// and subcommands, or PassthroughSubcommandConflictError if a command enables passthrough
// and has subcommands.
//
// The validation enforces a fundamental constraint: positional arguments can only
// be defined on leaf commands (commands without subcommands). This is because
// when a command has subcommands, any positional arguments would be interpreted
// as subcommand names by the CLI parser, making them unreachable.
func ValidateCommandTree(commands []*CommandInfo) error {
	entries := make([]invowkfile.CommandTreeEntry, 0, len(commands))
	for _, cmdInfo := range commands {
		if cmdInfo == nil {
			continue
		}
		entries = append(entries, invowkfile.CommandTreeEntry{
			Name:     cmdInfo.Name,
			Command:  cmdInfo.Command,
			FilePath: cmdInfo.FilePath,
		})
	}
	return invowkfile.ValidateCommandTree(entries)
}
