// Package discovery handles finding and loading invowkfiles from various locations.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invowkfile"
)

// Source represents where an invowkfile was found
type Source int

const (
	// SourceCurrentDir indicates the file was found in the current directory
	SourceCurrentDir Source = iota
	// SourceUserDir indicates the file was found in ~/.invowk/cmds
	SourceUserDir
	// SourceConfigPath indicates the file was found in a configured search path
	SourceConfigPath
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
	default:
		return "unknown"
	}
}

// DiscoveredFile represents a found invowkfile with its source
type DiscoveredFile struct {
	// Path is the absolute path to the invowkfile
	Path string
	// Source indicates where the file was found
	Source Source
	// Invowkfile is the parsed content (may be nil if not yet parsed)
	Invowkfile *invowkfile.Invowkfile
	// Error contains any error that occurred during parsing
	Error error
}

// Discovery handles finding invowkfiles
type Discovery struct {
	cfg *config.Config
}

// New creates a new Discovery instance
func New(cfg *config.Config) *Discovery {
	return &Discovery{cfg: cfg}
}

// DiscoverAll finds all invowkfiles from all sources in order of precedence
func (d *Discovery) DiscoverAll() ([]*DiscoveredFile, error) {
	var files []*DiscoveredFile

	// 1. Current directory (highest precedence)
	if cwdFile := d.discoverInDir(".", SourceCurrentDir); cwdFile != nil {
		files = append(files, cwdFile)
	}

	// 2. User commands directory (~/.invowk/cmds)
	userDir, err := config.CommandsDir()
	if err == nil {
		userFiles := d.discoverInDirRecursive(userDir, SourceUserDir)
		files = append(files, userFiles...)
	}

	// 3. Configured search paths
	for _, searchPath := range d.cfg.SearchPaths {
		pathFiles := d.discoverInDirRecursive(searchPath, SourceConfigPath)
		files = append(files, pathFiles...)
	}

	return files, nil
}

// discoverInDir looks for an invowkfile in a specific directory
func (d *Discovery) discoverInDir(dir string, source Source) *DiscoveredFile {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	// Check for invowkfile (no extension)
	path := filepath.Join(absDir, invowkfile.InvowkfileName)
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: path, Source: source}
	}

	// Check for invowkfile.toml
	path = filepath.Join(absDir, invowkfile.InvowkfileName+".toml")
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: path, Source: source}
	}

	return nil
}

// discoverInDirRecursive finds all invowkfiles in a directory tree
func (d *Discovery) discoverInDirRecursive(dir string, source Source) []*DiscoveredFile {
	var files []*DiscoveredFile

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return files
	}

	// Check if directory exists
	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		return files
	}

	err = filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == invowkfile.InvowkfileName || name == invowkfile.InvowkfileName+".toml" {
			files = append(files, &DiscoveredFile{Path: path, Source: source})
		}

		return nil
	})

	if err != nil {
		return files
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
		inv, parseErr := invowkfile.Parse(file.Path)
		if parseErr != nil {
			file.Error = parseErr
		} else {
			file.Invowkfile = inv
		}
	}

	return files, nil
}

// LoadFirst loads the first valid invowkfile found (respecting precedence)
func (d *Discovery) LoadFirst() (*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no invowkfile found")
	}

	file := files[0]
	inv, parseErr := invowkfile.Parse(file.Path)
	if parseErr != nil {
		file.Error = parseErr
		return file, parseErr
	}

	file.Invowkfile = inv
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
	// FilePath is the path to the invowkfile containing this command
	FilePath string
	// Command is a reference to the actual command
	Command *invowkfile.Command
	// Invowkfile is a reference to the parent invowkfile
	Invowkfile *invowkfile.Invowkfile
}

// DiscoverCommands finds all available commands from all invowkfiles
func (d *Discovery) DiscoverCommands() ([]*CommandInfo, error) {
	files, err := d.LoadAll()
	if err != nil {
		return nil, err
	}

	var commands []*CommandInfo
	seen := make(map[string]bool)

	for _, file := range files {
		if file.Error != nil || file.Invowkfile == nil {
			continue
		}

		flatCmds := file.Invowkfile.FlattenCommands()
		for name, cmd := range flatCmds {
			// Skip if we've already seen this command (higher precedence wins)
			if seen[name] {
				continue
			}
			seen[name] = true

			commands = append(commands, &CommandInfo{
				Name:        cmd.Name,
				Description: cmd.Description,
				Source:      file.Source,
				FilePath:    file.Path,
				Command:     cmd,
				Invowkfile:  file.Invowkfile,
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
		if len(prefix) == 0 || strings.HasPrefix(cmd.Name, prefix) {
			matching = append(matching, cmd)
		}
	}

	return matching, nil
}
