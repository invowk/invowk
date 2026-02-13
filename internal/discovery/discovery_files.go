// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invowkfile"
	"invowk-cli/pkg/invowkmod"
)

const (
	// SourceCurrentDir indicates the file was found in the current directory
	SourceCurrentDir Source = iota
	// SourceModule indicates the file was found in an invowk module
	SourceModule
)

type (
	// Source represents where an invowkfile was found
	Source int

	// DiscoveredFile represents a found invowkfile with its source
	DiscoveredFile struct {
		// Path is the absolute path to the invowkfile
		Path string
		// Source indicates where the file was found
		Source Source
		// Invowkfile is the parsed content (may be nil if not yet parsed)
		Invowkfile *invowkfile.Invowkfile
		// Error contains any error that occurred during parsing
		Error error
		// Module is set if this file was discovered from a module
		Module *invowkmod.Module
	}
)

// String returns a human-readable source name
func (s Source) String() string {
	switch s {
	case SourceCurrentDir:
		return "current directory"
	case SourceModule:
		return "module"
	default:
		return "unknown"
	}
}

// DiscoverAll finds all invowkfiles from all sources in 4-level precedence order:
//  1. Current directory (highest precedence — the local invowkfile.cue)
//  2. Modules in the current directory (*.invowkmod directories)
//  3. Configured includes from config (module paths)
//  4. User commands directory (~/.invowk/cmds — modules only, non-recursive)
//
// Earlier sources take precedence for disambiguation when the same SimpleName
// appears in multiple sources.
func (d *Discovery) DiscoverAll() ([]*DiscoveredFile, error) {
	var files []*DiscoveredFile

	// 1. Current directory (highest precedence)
	if cwdFile := d.discoverInDir(".", SourceCurrentDir); cwdFile != nil {
		files = append(files, cwdFile)
	}

	// 2. Modules in current directory
	moduleFiles := d.discoverModulesInDir(".")
	files = append(files, moduleFiles...)

	// 3. Configured includes (explicit module paths from config)
	includeFiles := d.loadIncludes()
	files = append(files, includeFiles...)

	// 4. User commands directory (~/.invowk/cmds — modules only, non-recursive)
	userDir, err := config.CommandsDir()
	if err == nil {
		userModuleFiles := d.discoverModulesInDir(userDir)
		files = append(files, userModuleFiles...)
	}

	return files, nil
}

// discoverInDir looks for an invowkfile in a specific directory
func (d *Discovery) discoverInDir(dir string, source Source) *DiscoveredFile {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	// Check for invowkfile.cue first (preferred)
	path := filepath.Join(absDir, invowkfile.InvowkfileName+".cue")
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: path, Source: source}
	}

	// Check for invowkfile (no extension)
	path = filepath.Join(absDir, invowkfile.InvowkfileName)
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: path, Source: source}
	}

	return nil
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
		if !invowkmod.IsModule(entryPath) {
			continue
		}

		// Skip reserved module name "invowkfile" (FR-015)
		// The name "invowkfile" is reserved for the canonical namespace system
		// where @invowkfile refers to the root invowkfile.cue source
		moduleName := strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix)
		if moduleName == SourceIDInvowkfile {
			// Note: Warning will be displayed in verbose mode (FR-013)
			continue
		}

		// Load the module
		m, err := invowkmod.Load(entryPath)
		if err != nil {
			// Invalid module, skip it
			continue
		}

		files = append(files, &DiscoveredFile{
			Path:   m.InvowkfilePath(),
			Source: SourceModule,
			Module: m,
		})
	}

	return files
}

// loadIncludes processes configured module include entries from config.
// All entries are module directory paths (*.invowkmod) and are loaded as SourceModule.
//
// Entries that do not exist on disk or fail validation are silently skipped
// (they may reference optional or environment-specific paths).
func (d *Discovery) loadIncludes() []*DiscoveredFile {
	var files []*DiscoveredFile

	for _, entry := range d.cfg.Includes {
		if !invowkmod.IsModule(entry.Path) {
			continue
		}
		moduleName := strings.TrimSuffix(filepath.Base(entry.Path), invowkmod.ModuleSuffix)
		if moduleName == SourceIDInvowkfile {
			continue // Skip reserved module name (FR-015)
		}
		m, err := invowkmod.Load(entry.Path)
		if err != nil {
			continue // Skip invalid modules
		}
		files = append(files, &DiscoveredFile{
			Path:   m.InvowkfilePath(),
			Source: SourceModule,
			Module: m,
		})
	}

	return files
}

// getModuleShortName extracts the short name from a module path.
// e.g., "/path/to/foo.invowkmod" -> "foo"
func getModuleShortName(modulePath string) string {
	base := filepath.Base(modulePath)
	return strings.TrimSuffix(base, invowkmod.ModuleSuffix)
}
