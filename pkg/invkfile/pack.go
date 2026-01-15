// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"invowk-cli/pkg/invkpack"
)

// PackRequirement represents a dependency on another pack from a Git repository.
// This is the type alias for invkpack.PackRequirement.
type PackRequirement = invkpack.PackRequirement

// Invkpack represents a loaded pack with metadata and optional commands.
// This is the type alias for invkpack.Invkpack.
// Use ParsePack() to load a pack with both metadata and commands.
type Invkpack = invkpack.Invkpack

// CommandScope defines what commands a pack can access.
// This is a type alias for invkpack.CommandScope.
type CommandScope = invkpack.CommandScope

// NewCommandScope creates a CommandScope for a parsed pack.
// This is a wrapper for invkpack.NewCommandScope.
func NewCommandScope(packID string, globalPackIDs []string, directRequirements []PackRequirement) *CommandScope {
	return invkpack.NewCommandScope(packID, globalPackIDs, directRequirements)
}

// ExtractPackFromCommand extracts the pack prefix from a fully qualified command name.
// This is a wrapper for invkpack.ExtractPackFromCommand.
func ExtractPackFromCommand(cmd string) string {
	return invkpack.ExtractPackFromCommand(cmd)
}
