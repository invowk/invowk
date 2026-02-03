// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"invowk-cli/pkg/invkfile"
)

// SourceIDInvkfile is the reserved source ID for the root invkfile.
// Used for multi-source discovery to identify commands from invkfile.cue.
const SourceIDInvkfile string = "invkfile"

type (
	// CommandInfo contains information about a discovered command
	CommandInfo struct {
		// Name is the command name (may include spaces, e.g., "test unit")
		Name string
		// Description is the command description
		Description string
		// Source is where the command was found
		Source Source
		// FilePath is the path to the invkfile containing this command
		FilePath string
		// Command is a reference to the actual command
		Command *invkfile.Command
		// Invkfile is a reference to the parent invkfile
		Invkfile *invkfile.Invkfile

		// SimpleName is the command name without module prefix (e.g., "deploy")
		// Used for the transparent namespace feature
		SimpleName string
		// SourceID identifies the source: "invkfile" for root invkfile,
		// or the module short name (e.g., "foo") for modules
		SourceID string
		// ModuleID is the full module identifier if from a module
		// (e.g., "io.invowk.sample"), empty for root invkfile
		ModuleID string
		// IsAmbiguous is true if SimpleName conflicts with another command
		// from a different source
		IsAmbiguous bool
	}

	// DiscoveredCommandSet holds aggregated discovery results with conflict analysis.
	// It provides indexed access to commands by simple name and source for
	// efficient conflict detection and grouped listing.
	DiscoveredCommandSet struct {
		// Commands contains all discovered commands
		Commands []*CommandInfo

		// BySimpleName indexes commands by their simple name for conflict detection.
		// Key: simple command name (e.g., "deploy")
		// Value: all commands with that name from different sources
		BySimpleName map[string][]*CommandInfo

		// AmbiguousNames contains simple names that have conflicts (>1 source)
		AmbiguousNames map[string]bool

		// BySource indexes commands by source for grouped listing.
		// Key: SourceID (e.g., "invkfile", "foo")
		BySource map[string][]*CommandInfo

		// SourceOrder is an ordered list of sources for consistent display.
		// "invkfile" always comes first if present, then modules alphabetically.
		SourceOrder []string
	}
)

// NewDiscoveredCommandSet creates a new DiscoveredCommandSet with initialized maps.
func NewDiscoveredCommandSet() *DiscoveredCommandSet {
	return &DiscoveredCommandSet{
		Commands:       make([]*CommandInfo, 0),
		BySimpleName:   make(map[string][]*CommandInfo),
		AmbiguousNames: make(map[string]bool),
		BySource:       make(map[string][]*CommandInfo),
		SourceOrder:    make([]string, 0),
	}
}

// Add adds a command to the set and updates indexes.
// The command's SimpleName and SourceID must be set before calling Add.
func (s *DiscoveredCommandSet) Add(cmd *CommandInfo) {
	s.Commands = append(s.Commands, cmd)

	// Index by simple name
	s.BySimpleName[cmd.SimpleName] = append(s.BySimpleName[cmd.SimpleName], cmd)

	// Index by source
	if _, exists := s.BySource[cmd.SourceID]; !exists {
		// First command from this source, add to source order
		s.SourceOrder = append(s.SourceOrder, cmd.SourceID)
	}
	s.BySource[cmd.SourceID] = append(s.BySource[cmd.SourceID], cmd)
}

// Analyze detects conflicts and marks ambiguous commands.
// Must be called after all commands have been added.
// It also sorts SourceOrder to ensure "invkfile" comes first, then modules alphabetically.
func (s *DiscoveredCommandSet) Analyze() {
	// Detect ambiguous names (commands with same SimpleName from different sources)
	for simpleName, cmds := range s.BySimpleName {
		if len(cmds) <= 1 {
			continue
		}

		// Check if commands come from different sources
		sources := make(map[string]bool)
		for _, cmd := range cmds {
			sources[cmd.SourceID] = true
		}

		if len(sources) > 1 {
			// Ambiguous: same name, different sources
			s.AmbiguousNames[simpleName] = true
			for _, cmd := range cmds {
				cmd.IsAmbiguous = true
			}
		}
	}

	// Sort SourceOrder: "invkfile" first, then modules alphabetically
	sort.Slice(s.SourceOrder, func(i, j int) bool {
		if s.SourceOrder[i] == SourceIDInvkfile {
			return true
		}
		if s.SourceOrder[j] == SourceIDInvkfile {
			return false
		}
		return s.SourceOrder[i] < s.SourceOrder[j]
	})
}

// DiscoverCommands finds all available commands from all invkfiles.
// Commands are aggregated from the root invkfile and all sibling modules,
// with conflict detection for commands that share the same simple name.
//
// For non-module sources (current dir, user dir, config paths), the original
// precedence behavior is maintained - higher precedence wins.
// For modules in the current directory, all commands are included with
// conflict detection when names collide across sources.
//
// This method delegates to DiscoverCommandSet() and extracts the sorted command list.
func (d *Discovery) DiscoverCommands() ([]*CommandInfo, error) {
	commandSet, err := d.DiscoverCommandSet()
	if err != nil {
		return nil, err
	}

	// Sort commands by name for consistent ordering
	sort.Slice(commandSet.Commands, func(i, j int) bool {
		return commandSet.Commands[i].Name < commandSet.Commands[j].Name
	})

	return commandSet.Commands, nil
}

// DiscoverCommandSet finds all available commands and returns them as a
// DiscoveredCommandSet with conflict analysis. This is useful for CLI listing
// where commands need to be grouped by source with ambiguity annotations.
func (d *Discovery) DiscoverCommandSet() (*DiscoveredCommandSet, error) {
	files, err := d.LoadAll()
	if err != nil {
		return nil, err
	}

	commandSet := NewDiscoveredCommandSet()
	// Track seen commands for precedence within non-module sources
	seenNonModule := make(map[string]bool)

	for _, file := range files {
		if file.Error != nil {
			// Log parsing errors to help diagnose discovery issues
			fmt.Fprintf(os.Stderr, "Warning: skipping invkfile at %s: %v\n", file.Path, file.Error)
			continue
		}
		if file.Invkfile == nil {
			continue
		}

		// Determine source ID and module ID for this file
		var sourceID, moduleID string
		isModule := file.Module != nil
		switch {
		case isModule:
			// From a module - use short name from folder
			sourceID = getModuleShortName(file.Module.Path)
			moduleID = file.Module.Name()
		case file.Source == SourceCurrentDir:
			// From root invkfile in current directory
			sourceID = SourceIDInvkfile
		default:
			// From user directory or config path - use source type as ID
			sourceID = file.Source.String()
		}

		flatCmds := file.Invkfile.FlattenCommands()
		for fullName, cmd := range flatCmds {
			// Extract simple name for conflict detection and display.
			// For modules, FlattenCommands() returns prefixed names like "foo build",
			// so we use cmd.Name which is the original unprefixed name.
			simpleName := cmd.Name

			// For non-module sources, maintain precedence (skip if already seen)
			if !isModule {
				if seenNonModule[simpleName] {
					continue
				}
				seenNonModule[simpleName] = true
			}

			commandSet.Add(&CommandInfo{
				Name:        fullName, // Full name for Cobra registration (may be prefixed)
				Description: cmd.Description,
				Source:      file.Source,
				FilePath:    file.Path,
				Command:     cmd,
				Invkfile:    file.Invkfile,
				SimpleName:  simpleName, // Simple name for conflict detection
				SourceID:    sourceID,
				ModuleID:    moduleID,
			})
		}
	}

	// Analyze for conflicts
	commandSet.Analyze()

	return commandSet, nil
}

// DiscoverAndValidateCommands finds all commands and validates the command tree.
// Returns an error if any command has both args and subcommands (leaf-only args constraint).
// This method should be used instead of DiscoverCommands() when you want to ensure
// the command tree is structurally valid before proceeding with registration.
func (d *Discovery) DiscoverAndValidateCommands() ([]*CommandInfo, error) {
	commands, err := d.DiscoverCommands()
	if err != nil {
		return nil, err
	}

	if err := ValidateCommandTree(commands); err != nil {
		return nil, err
	}

	return commands, nil
}

// DiscoverAndValidateCommandSet finds all commands as a DiscoveredCommandSet
// and validates the command tree. This method provides access to ambiguity
// detection through AmbiguousNames, enabling transparent namespace for
// unambiguous commands and disambiguation for ambiguous ones.
func (d *Discovery) DiscoverAndValidateCommandSet() (*DiscoveredCommandSet, error) {
	commandSet, err := d.DiscoverCommandSet()
	if err != nil {
		return nil, err
	}

	if err := ValidateCommandTree(commandSet.Commands); err != nil {
		return nil, err
	}

	return commandSet, nil
}

// GetCommand finds a specific command by name
func (d *Discovery) GetCommand(name string) (*CommandInfo, error) {
	commands, err := d.DiscoverCommands()
	if err != nil {
		return nil, err
	}

	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd, nil
		}
	}

	return nil, fmt.Errorf("command '%s' not found", name)
}

// GetCommandsWithPrefix returns commands that start with the given prefix
func (d *Discovery) GetCommandsWithPrefix(prefix string) ([]*CommandInfo, error) {
	commands, err := d.DiscoverCommands()
	if err != nil {
		return nil, err
	}

	var matching []*CommandInfo
	for _, cmd := range commands {
		if prefix == "" || strings.HasPrefix(cmd.Name, prefix) {
			matching = append(matching, cmd)
		}
	}

	return matching, nil
}
