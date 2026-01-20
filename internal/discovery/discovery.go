// SPDX-License-Identifier: EPL-2.0

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

// ModuleCollisionError is returned when two modules have the same module identifier.
type ModuleCollisionError struct {
	ModuleID     string
	FirstSource  string
	SecondSource string
}

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

// Source represents where an invkfile was found
type Source int

const (
	// SourceCurrentDir indicates the file was found in the current directory
	SourceCurrentDir Source = iota
	// SourceUserDir indicates the file was found in ~/.invowk/cmds
	SourceUserDir
	// SourceConfigPath indicates the file was found in a configured search path
	SourceConfigPath
	// SourceModule indicates the file was found in an invowk module
	SourceModule
)

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

// DiscoveredFile represents a found invkfile with its source
type DiscoveredFile struct {
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
type Discovery struct {
	cfg *config.Config
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

// CommandInfo contains information about a discovered command
type CommandInfo struct {
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
}

// DiscoverCommands finds all available commands from all invkfiles
func (d *Discovery) DiscoverCommands() ([]*CommandInfo, error) {
	files, err := d.LoadAll()
	if err != nil {
		return nil, err
	}

	var commands []*CommandInfo
	seen := make(map[string]bool)

	for _, file := range files {
		if file.Error != nil || file.Invkfile == nil {
			continue
		}

		flatCmds := file.Invkfile.FlattenCommands()
		for name, cmd := range flatCmds {
			// Skip if we've already seen this command (higher precedence wins)
			if seen[name] {
				continue
			}
			seen[name] = true

			commands = append(commands, &CommandInfo{
				Name:        name, // Use the fully qualified name with group prefix
				Description: cmd.Description,
				Source:      file.Source,
				FilePath:    file.Path,
				Command:     cmd,
				Invkfile:    file.Invkfile,
			})
		}
	}

	// Sort commands by name for consistent ordering
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands, nil
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
