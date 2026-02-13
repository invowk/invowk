// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"context"
	"fmt"
	"sort"

	"invowk-cli/pkg/invowkfile"
)

// SourceIDInvowkfile is the reserved source ID for the root invowkfile.
// Used for multi-source discovery to identify commands from invowkfile.cue.
const SourceIDInvowkfile string = "invowkfile"

type (
	// CommandInfo contains information about a discovered command
	CommandInfo struct {
		// Name is the command name (may include spaces, e.g., "test unit")
		Name string
		// Description is the command description
		Description string
		// Source is where the command was found
		Source Source
		// FilePath is the path to the invowkfile containing this command
		FilePath string
		// Command is a reference to the actual command
		Command *invowkfile.Command
		// Invowkfile is a reference to the parent invowkfile
		Invowkfile *invowkfile.Invowkfile

		// SimpleName is the command name without module prefix (e.g., "deploy")
		// Used for the transparent namespace feature
		SimpleName string
		// SourceID identifies the source: "invowkfile" for root invowkfile,
		// or the module short name (e.g., "foo") for modules
		SourceID string
		// ModuleID is the full module identifier if from a module
		// (e.g., "io.invowk.sample"), empty for root invowkfile
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
		// Key: SourceID (e.g., "invowkfile", "foo")
		BySource map[string][]*CommandInfo

		// SourceOrder is an ordered list of sources for consistent display.
		// "invowkfile" always comes first if present, then modules alphabetically.
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
// Must be called after all commands have been added and before presenting results
// to users. It marks SimpleName entries with commands from >1 source as IsAmbiguous,
// and sorts SourceOrder ("invowkfile" first, then modules alphabetically) for
// consistent display ordering.
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

	// Sort SourceOrder: "invowkfile" first, then modules alphabetically
	sort.Slice(s.SourceOrder, func(i, j int) bool {
		if s.SourceOrder[i] == SourceIDInvowkfile {
			return true
		}
		if s.SourceOrder[j] == SourceIDInvowkfile {
			return false
		}
		return s.SourceOrder[i] < s.SourceOrder[j]
	})
}

// DiscoverCommandSet finds all available commands and returns them as a
// DiscoveredCommandSet with conflict analysis. This is useful for CLI listing
// where commands need to be grouped by source with ambiguity annotations.
func (d *Discovery) DiscoverCommandSet(ctx context.Context) (CommandSetResult, error) {
	select {
	case <-ctx.Done():
		return CommandSetResult{}, fmt.Errorf("discover command set canceled: %w", ctx.Err())
	default:
	}

	files, err := d.LoadAll()
	if err != nil {
		return CommandSetResult{}, err
	}

	commandSet := NewDiscoveredCommandSet()
	diagnostics := make([]Diagnostic, 0)
	// Track seen commands for precedence within non-module sources
	seenNonModule := make(map[string]bool)

	for _, file := range files {
		select {
		case <-ctx.Done():
			return CommandSetResult{}, fmt.Errorf("discover command set canceled while processing files: %w", ctx.Err())
		default:
		}

		if file.Error != nil {
			// Parse failures are recoverable for discovery: keep traversing and
			// return structured diagnostics to the caller instead of writing output.
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "invowkfile_parse_skipped",
				Message:  fmt.Sprintf("skipping invowkfile at %s: %v", file.Path, file.Error),
				Path:     file.Path,
				Cause:    file.Error,
			})
			continue
		}
		if file.Invowkfile == nil {
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
		default:
			// Non-module source: root invowkfile in current directory
			sourceID = SourceIDInvowkfile
		}

		flatCmds := file.Invowkfile.FlattenCommands()
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
				Invowkfile:  file.Invowkfile,
				SimpleName:  simpleName, // Simple name for conflict detection
				SourceID:    sourceID,
				ModuleID:    moduleID,
			})
		}
	}

	// Analyze for conflicts
	commandSet.Analyze()
	sort.Slice(commandSet.Commands, func(i, j int) bool {
		return commandSet.Commands[i].Name < commandSet.Commands[j].Name
	})

	return CommandSetResult{Set: commandSet, Diagnostics: diagnostics}, nil
}

// DiscoverAndValidateCommandSet finds all commands as a DiscoveredCommandSet
// and validates the command tree. This method provides access to ambiguity
// detection through AmbiguousNames, enabling transparent namespace for
// unambiguous commands and disambiguation for ambiguous ones.
func (d *Discovery) DiscoverAndValidateCommandSet(ctx context.Context) (CommandSetResult, error) {
	result, err := d.DiscoverCommandSet(ctx)
	if err != nil {
		return CommandSetResult{}, err
	}

	if err := ValidateCommandTree(result.Set.Commands); err != nil {
		return result, err
	}

	return result, nil
}

// GetCommand finds a specific command by name
func (d *Discovery) GetCommand(ctx context.Context, name string) (LookupResult, error) {
	result, err := d.DiscoverCommandSet(ctx)
	if err != nil {
		return LookupResult{}, err
	}

	for _, cmd := range result.Set.Commands {
		if cmd.Name == name {
			return LookupResult{Command: cmd, Diagnostics: result.Diagnostics}, nil
		}
	}

	// Command-not-found is represented as a diagnostic so CLI callers can choose
	// the rendering policy (execute/list/completion) consistently.
	result.Diagnostics = append(result.Diagnostics, Diagnostic{
		Severity: SeverityError,
		Code:     "command_not_found",
		Message:  fmt.Sprintf("command '%s' not found", name),
	})

	return LookupResult{Diagnostics: result.Diagnostics}, nil
}
