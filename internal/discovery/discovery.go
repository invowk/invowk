// SPDX-License-Identifier: MPL-2.0

// Package discovery handles finding and loading invkfiles from various locations.
package discovery

import (
	"fmt"
	"invowk-cli/internal/config"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//nolint:iotamixing // SourceIDInvkfile is a string constant related to Source types; keeping together for discoverability
const (
	// SourceCurrentDir indicates the file was found in the current directory
	SourceCurrentDir Source = iota
	// SourceUserDir indicates the file was found in ~/.invowk/cmds
	SourceUserDir
	// SourceConfigPath indicates the file was found in a configured search path
	SourceConfigPath
	// SourceModule indicates the file was found in an invowk module
	SourceModule

	// SourceIDInvkfile is the reserved source ID for the root invkfile.
	// Used for multi-source discovery to identify commands from invkfile.cue.
	SourceIDInvkfile string = "invkfile"
)

type (
	// Source represents where an invkfile was found
	Source int

	// ModuleCollisionError is returned when two modules have the same module identifier.
	ModuleCollisionError struct {
		ModuleID     string
		FirstSource  string
		SecondSource string
	}

	// DiscoveredFile represents a found invkfile with its source
	DiscoveredFile struct {
		// Path is the absolute path to the invkfile
		Path string
		// Source indicates where the file was found
		Source Source
		// Invkfile is the parsed content (may be nil if not yet parsed)
		Invkfile *invkfile.Invkfile
		// Error contains any error that occurred during parsing
		Error error
		// Module is set if this file was discovered from a module
		Module *invkmod.Module
	}

	// Discovery handles finding invkfiles
	Discovery struct {
		cfg *config.Config
	}

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

// Error implements the error interface.
func (e *ModuleCollisionError) Error() string {
	return fmt.Sprintf(
		"module name collision: '%s' defined in both:\n"+
			"  - %s\n"+
			"  - %s\n\n"+
			"Use an alias to disambiguate:\n"+
			"  invowk module alias %q <new-alias>\n"+
			"  invowk module alias %q <new-alias>",
		e.ModuleID, e.FirstSource, e.SecondSource,
		e.FirstSource, e.SecondSource)
}

// String returns a human-readable source name
func (s Source) String() string {
	switch s {
	case SourceCurrentDir:
		return "current directory"
	case SourceUserDir:
		return "user commands (~/.invowk/cmds)"
	case SourceConfigPath:
		return "configured search path"
	case SourceModule:
		return "module"
	default:
		return "unknown"
	}
}

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

// New creates a new Discovery instance
func New(cfg *config.Config) *Discovery {
	return &Discovery{cfg: cfg}
}

// DiscoverAll finds all invkfiles from all sources in order of precedence
func (d *Discovery) DiscoverAll() ([]*DiscoveredFile, error) {
	var files []*DiscoveredFile

	// 1. Current directory (highest precedence)
	if cwdFile := d.discoverInDir(".", SourceCurrentDir); cwdFile != nil {
		files = append(files, cwdFile)
	}

	// 2. Modules in current directory
	moduleFiles := d.discoverModulesInDir(".")
	files = append(files, moduleFiles...)

	// 3. User commands directory (~/.invowk/cmds)
	userDir, err := config.CommandsDir()
	if err == nil {
		userFiles := d.discoverInDirRecursive(userDir, SourceUserDir)
		files = append(files, userFiles...)

		// Also discover modules in user commands directory
		userModuleFiles := d.discoverModulesInDir(userDir)
		files = append(files, userModuleFiles...)
	}

	// 4. Configured search paths
	for _, searchPath := range d.cfg.SearchPaths {
		pathFiles := d.discoverInDirRecursive(searchPath, SourceConfigPath)
		files = append(files, pathFiles...)

		// Also discover modules in search paths
		searchPathModuleFiles := d.discoverModulesInDir(searchPath)
		files = append(files, searchPathModuleFiles...)
	}

	return files, nil
}

// LoadAll parses all discovered files
func (d *Discovery) LoadAll() ([]*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		var inv *invkfile.Invkfile
		var parseErr error

		if file.Module != nil {
			// Use module-aware parsing
			parsed, err := invkfile.ParseModule(file.Module.Path)
			if err != nil {
				parseErr = err
			} else {
				inv = invkfile.GetModuleCommands(parsed)
			}
		} else {
			inv, parseErr = invkfile.Parse(file.Path)
		}

		if parseErr != nil {
			file.Error = parseErr
		} else {
			file.Invkfile = inv
		}
	}

	return files, nil
}

// LoadFirst loads the first valid invkfile found (respecting precedence)
func (d *Discovery) LoadFirst() (*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no invkfile found")
	}

	file := files[0]
	var inv *invkfile.Invkfile
	var parseErr error

	if file.Module != nil {
		// Use module-aware parsing
		parsed, err := invkfile.ParseModule(file.Module.Path)
		if err != nil {
			parseErr = err
		} else {
			inv = invkfile.GetModuleCommands(parsed)
		}
	} else {
		inv, parseErr = invkfile.Parse(file.Path)
	}

	if parseErr != nil {
		file.Error = parseErr
		return file, parseErr
	}

	file.Invkfile = inv
	return file, nil
}

// DiscoverCommands finds all available commands from all invkfiles.
// Commands are aggregated from the root invkfile and all sibling modules,
// with conflict detection for commands that share the same simple name.
//
// For non-module sources (current dir, user dir, config paths), the original
// precedence behavior is maintained - higher precedence wins.
// For modules in the current directory, all commands are included with
// conflict detection when names collide across sources.
func (d *Discovery) DiscoverCommands() ([]*CommandInfo, error) {
	files, err := d.LoadAll()
	if err != nil {
		return nil, err
	}

	commandSet := NewDiscoveredCommandSet()
	// Track seen commands for precedence within non-module sources
	seenNonModule := make(map[string]bool)

	for _, file := range files {
		if file.Error != nil || file.Invkfile == nil {
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

	// Sort commands by name for consistent ordering
	sort.Slice(commandSet.Commands, func(i, j int) bool {
		return commandSet.Commands[i].Name < commandSet.Commands[j].Name
	})

	return commandSet.Commands, nil
}

// getModuleShortName extracts the short name from a module path.
// e.g., "/path/to/foo.invkmod" -> "foo"
func getModuleShortName(modulePath string) string {
	base := filepath.Base(modulePath)
	return strings.TrimSuffix(base, invkmod.ModuleSuffix)
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
		if file.Error != nil || file.Invkfile == nil {
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

// CheckModuleCollisions checks for module ID collisions among discovered files.
// It returns a ModuleCollisionError if two modules have the same module identifier
// and neither has an alias configured.
func (d *Discovery) CheckModuleCollisions(files []*DiscoveredFile) error {
	// Map module IDs to their sources (considering aliases)
	moduleSources := make(map[string]string)

	for _, file := range files {
		if file.Error != nil || file.Invkfile == nil {
			continue
		}

		moduleID := file.Invkfile.GetModule()
		if moduleID == "" {
			continue
		}

		// Check if there's an alias configured for this path
		if d.cfg != nil && d.cfg.ModuleAliases != nil {
			if alias, ok := d.cfg.ModuleAliases[file.Path]; ok {
				moduleID = alias
			}
		}

		// Check for collision
		if existingSource, exists := moduleSources[moduleID]; exists {
			return &ModuleCollisionError{
				ModuleID:     moduleID,
				FirstSource:  existingSource,
				SecondSource: file.Path,
			}
		}

		moduleSources[moduleID] = file.Path
	}

	return nil
}

// GetEffectiveModuleID returns the effective module ID for a file, considering aliases.
func (d *Discovery) GetEffectiveModuleID(file *DiscoveredFile) string {
	if file.Invkfile == nil {
		return ""
	}

	moduleID := file.Invkfile.GetModule()

	// Check if there's an alias configured for this path
	if d.cfg != nil && d.cfg.ModuleAliases != nil {
		if alias, ok := d.cfg.ModuleAliases[file.Path]; ok {
			return alias
		}
	}

	return moduleID
}

// discoverInDir looks for an invkfile in a specific directory
func (d *Discovery) discoverInDir(dir string, source Source) *DiscoveredFile {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	// Check for invkfile.cue first (preferred)
	path := filepath.Join(absDir, invkfile.InvkfileName+".cue")
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: path, Source: source}
	}

	// Check for invkfile (no extension)
	path = filepath.Join(absDir, invkfile.InvkfileName)
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: path, Source: source}
	}

	return nil
}

// discoverInDirRecursive finds all invkfiles in a directory tree
func (d *Discovery) discoverInDirRecursive(dir string, source Source) []*DiscoveredFile {
	var files []*DiscoveredFile

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return files
	}

	// Check if directory exists
	if _, statErr := os.Stat(absDir); os.IsNotExist(statErr) {
		return files
	}

	err = filepath.WalkDir(absDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // Intentionally skip errors to continue walking
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == invkfile.InvkfileName || name == invkfile.InvkfileName+".cue" {
			files = append(files, &DiscoveredFile{Path: path, Source: source})
		}

		return nil
	})
	if err != nil {
		return files
	}

	return files
}

// discoverModulesInDir finds all valid modules in a directory.
// It only looks at immediate subdirectories (modules are not nested).
func (d *Discovery) discoverModulesInDir(dir string) []*DiscoveredFile {
	var files []*DiscoveredFile

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return files
	}

	// Check if directory exists
	if _, statErr := os.Stat(absDir); os.IsNotExist(statErr) {
		return files
	}

	// Read directory entries
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a module
		entryPath := filepath.Join(absDir, entry.Name())
		if !invkmod.IsModule(entryPath) {
			continue
		}

		// Skip reserved module name "invkfile" (FR-015)
		// The name "invkfile" is reserved for the canonical namespace system
		// where @invkfile refers to the root invkfile.cue source
		moduleName := strings.TrimSuffix(entry.Name(), invkmod.ModuleSuffix)
		if moduleName == SourceIDInvkfile {
			// Note: Warning will be displayed in verbose mode (FR-013)
			continue
		}

		// Load the module
		m, err := invkmod.Load(entryPath)
		if err != nil {
			// Invalid module, skip it
			continue
		}

		files = append(files, &DiscoveredFile{
			Path:   m.InvkfilePath(),
			Source: SourceModule,
			Module: m,
		})
	}

	return files
}
