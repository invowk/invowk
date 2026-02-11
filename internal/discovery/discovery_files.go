// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"
)

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

type (
	// Source represents where an invkfile was found
	Source int

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

// DiscoverAll finds all invkfiles from all sources in 4-level precedence order:
//  1. Current directory (highest precedence — the local invkfile.cue)
//  2. Modules in the current directory (*.invkmod directories)
//  3. User commands directory (~/.invowk/cmds)
//  4. Configured search paths from config (lowest precedence)
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

	// 3. User commands directory (~/.invowk/cmds)
	userDir, err := config.CommandsDir()
	if err == nil {
		userFiles := d.discoverInDirRecursive(userDir, SourceUserDir)
		files = append(files, userFiles...)

		// Also discover modules in user commands directory
		userModuleFiles := d.discoverModulesInDir(userDir)
		files = append(files, userModuleFiles...)
	}

	// 4. Configured includes (explicit invkfiles and modules from config)
	includeFiles := d.loadIncludes()
	files = append(files, includeFiles...)

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

// loadIncludes processes configured include entries from config. Each entry is either:
//   - A direct invkfile path (invkfile.cue or invkfile) -> SourceConfigPath
//   - A module directory path (*.invkmod) -> SourceModule
//
// Entries that do not exist on disk are silently skipped (they may reference
// optional or environment-specific paths).
func (d *Discovery) loadIncludes() []*DiscoveredFile {
	var files []*DiscoveredFile

	for _, entry := range d.cfg.Includes {
		if entry.IsModule() {
			// Module entry - load as module
			if !invkmod.IsModule(entry.Path) {
				continue
			}
			moduleName := strings.TrimSuffix(filepath.Base(entry.Path), invkmod.ModuleSuffix)
			if moduleName == SourceIDInvkfile {
				continue // Skip reserved module name (FR-015)
			}
			m, err := invkmod.Load(entry.Path)
			if err != nil {
				continue // Skip invalid modules
			}
			files = append(files, &DiscoveredFile{
				Path:   m.InvkfilePath(),
				Source: SourceModule,
				Module: m,
			})
		} else {
			// Invkfile entry - load directly
			if _, err := os.Stat(entry.Path); err == nil {
				files = append(files, &DiscoveredFile{Path: entry.Path, Source: SourceConfigPath})
			}
		}
	}

	return files
}

// getModuleShortName extracts the short name from a module path.
// e.g., "/path/to/foo.invkmod" -> "foo"
func getModuleShortName(modulePath string) string {
	base := filepath.Base(modulePath)
	return strings.TrimSuffix(base, invkmod.ModuleSuffix)
}
