// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
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
		Path types.FilesystemPath
		// Source indicates where the file was found
		Source Source
		// Invowkfile is the parsed content (may be nil if not yet parsed)
		Invowkfile *invowkfile.Invowkfile
		// Error contains any error that occurred during parsing
		Error error
		// Module is set if this file was discovered from a module
		Module *invowkmod.Module
		// ParentModule is set when this file was discovered from a vendored
		// module inside another module's invowk_modules/ directory.
		// nil for non-vendored files.
		ParentModule *invowkmod.Module
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

// IsValid returns whether the Source is one of the defined source types,
// and a list of validation errors if it is not.
func (s Source) IsValid() (bool, []error) {
	switch s {
	case SourceCurrentDir, SourceModule:
		return true, nil
	default:
		return false, []error{&InvalidSourceError{Value: s}}
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
	files, _, err := d.discoverAllWithDiagnostics()
	return files, err
}

// discoverAllWithDiagnostics discovers files plus non-fatal warnings about
// skipped modules/includes so callers can surface observability without failing.
func (d *Discovery) discoverAllWithDiagnostics() ([]*DiscoveredFile, []Diagnostic, error) {
	var files []*DiscoveredFile
	// Seed with any init-time diagnostics (e.g., os.Getwd or CommandsDir failures)
	// so they surface through the standard diagnostic rendering pipeline.
	diagnostics := make([]Diagnostic, 0, len(d.initDiagnostics))
	diagnostics = append(diagnostics, d.initDiagnostics...)

	// 1. Current directory (highest precedence)
	// Skip current-dir discovery when baseDir is empty (e.g., os.Getwd() failed
	// because the working directory was deleted). This prevents filepath.Abs("")
	// from silently resolving to the process working directory, which may not exist.
	if d.baseDir != "" {
		if cwdFile := d.discoverInDir(d.baseDir, SourceCurrentDir); cwdFile != nil {
			files = append(files, cwdFile)
		}

		// 2. Modules in current directory (+ their vendored dependencies)
		moduleFiles, moduleDiags := d.discoverModulesInDirWithDiagnostics(d.baseDir)
		files, diagnostics = d.appendModulesWithVendored(files, diagnostics, moduleFiles, moduleDiags)
	}

	// 3. Configured includes (explicit module paths from config) (+ their vendored dependencies)
	includeFiles, includeDiags := d.loadIncludesWithDiagnostics()
	files, diagnostics = d.appendModulesWithVendored(files, diagnostics, includeFiles, includeDiags)

	// 4. User commands directory (~/.invowk/cmds — modules only, non-recursive) (+ their vendored dependencies)
	if d.commandsDir != "" {
		userModuleFiles, userModuleDiags := d.discoverModulesInDirWithDiagnostics(d.commandsDir)
		files, diagnostics = d.appendModulesWithVendored(files, diagnostics, userModuleFiles, userModuleDiags)
	}

	return files, diagnostics, nil
}

// discoverInDir looks for an invowkfile in a specific directory.
func (d *Discovery) discoverInDir(dir types.FilesystemPath, source Source) *DiscoveredFile {
	absDir, err := filepath.Abs(string(dir))
	if err != nil {
		slog.Warn("failed to resolve absolute path for discovery directory", "dir", dir, "error", err)
		return nil
	}

	// Check for invowkfile.cue first (preferred)
	path := filepath.Join(absDir, invowkfile.InvowkfileName+".cue")
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: types.FilesystemPath(path), Source: source}
	}

	// Check for invowkfile (no extension)
	path = filepath.Join(absDir, invowkfile.InvowkfileName)
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: types.FilesystemPath(path), Source: source}
	}

	return nil
}

// discoverModulesInDirWithDiagnostics finds all valid modules in a directory and
// reports non-fatal warnings for skipped entries.
func (d *Discovery) discoverModulesInDirWithDiagnostics(dir types.FilesystemPath) ([]*DiscoveredFile, []Diagnostic) {
	var files []*DiscoveredFile
	diagnostics := make([]Diagnostic, 0)

	absDir, err := filepath.Abs(string(dir))
	if err != nil {
		diagnostics = append(diagnostics, mustDiagnosticWithCause(
			SeverityWarning,
			CodeModuleScanPathInvalid,
			fmt.Sprintf("failed to resolve module scan path %q: %v", dir, err),
			dir,
			err,
		))
		return files, diagnostics
	}

	// Check if directory exists
	if _, statErr := os.Stat(absDir); os.IsNotExist(statErr) {
		return files, diagnostics
	}

	// Read directory entries
	entries, err := os.ReadDir(absDir)
	if err != nil {
		diagnostics = append(diagnostics, mustDiagnosticWithCause(
			SeverityWarning,
			CodeModuleScanFailed,
			fmt.Sprintf("failed to list directory %s while scanning modules: %v", absDir, err),
			types.FilesystemPath(absDir),
			err,
		))
		return files, diagnostics
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a module
		entryPath := filepath.Join(absDir, entry.Name())
		if !invowkmod.IsModule(types.FilesystemPath(entryPath)) {
			continue
		}

		// Skip reserved module name "invowkfile" (FR-015)
		// The name "invowkfile" is reserved for the canonical namespace system
		// where @invowkfile refers to the root invowkfile.cue source
		moduleName := strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix)
		if SourceID(moduleName) == SourceIDInvowkfile {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeReservedModuleNameSkipped,
				fmt.Sprintf("skipping reserved module name '%s'", moduleName),
				types.FilesystemPath(entryPath),
			))
			continue
		}

		// Load the module
		m, err := invowkmod.Load(types.FilesystemPath(entryPath))
		if err != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeModuleLoadSkipped,
				fmt.Sprintf("skipping invalid module at %s: %v", entryPath, err),
				types.FilesystemPath(entryPath),
				err,
			))
			continue
		}

		files = append(files, &DiscoveredFile{
			Path:   m.InvowkfilePath(),
			Source: SourceModule,
			Module: m,
		})
	}

	return files, diagnostics
}

// loadIncludesWithDiagnostics processes configured module include entries and
// emits warnings for skipped entries while keeping permissive discovery behavior.
func (d *Discovery) loadIncludesWithDiagnostics() ([]*DiscoveredFile, []Diagnostic) {
	var files []*DiscoveredFile
	diagnostics := make([]Diagnostic, 0)

	if d.cfg == nil {
		return files, diagnostics
	}

	for _, entry := range d.cfg.Includes {
		pathStr := string(entry.Path)
		if !invowkmod.IsModule(types.FilesystemPath(pathStr)) {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeIncludeNotModule,
				fmt.Sprintf("configured include is not a valid module directory, skipping: %s", entry.Path),
				types.FilesystemPath(pathStr),
			))
			continue
		}
		moduleName := strings.TrimSuffix(filepath.Base(pathStr), invowkmod.ModuleSuffix)
		if SourceID(moduleName) == SourceIDInvowkfile {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeIncludeReservedSkipped,
				fmt.Sprintf("configured include uses reserved module name '%s', skipping", moduleName),
				types.FilesystemPath(pathStr),
			))
			continue // Skip reserved module name (FR-015)
		}
		m, err := invowkmod.Load(types.FilesystemPath(pathStr))
		if err != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeIncludeModuleLoadFailed,
				fmt.Sprintf("failed to load included module at %s: %v", entry.Path, err),
				types.FilesystemPath(pathStr),
				err,
			))
			continue // Skip invalid modules
		}
		files = append(files, &DiscoveredFile{
			Path:   m.InvowkfilePath(),
			Source: SourceModule,
			Module: m,
		})
	}

	return files, diagnostics
}

// discoverVendoredModulesWithDiagnostics performs a flat, one-level scan of a
// module's invowk_modules/ directory to find vendored dependencies. Nested
// vendoring (invowk_modules inside vendored modules) is not recursed into;
// a diagnostic warning is emitted if detected.
func (d *Discovery) discoverVendoredModulesWithDiagnostics(parentModule *invowkmod.Module) ([]*DiscoveredFile, []Diagnostic) {
	var files []*DiscoveredFile
	diagnostics := make([]Diagnostic, 0)

	vendorDir := invowkmod.GetVendoredModulesDir(parentModule.Path)
	vendorDirStr := string(vendorDir)
	if _, err := os.Stat(vendorDirStr); err != nil {
		// No vendor directory is common and not a warning
		return files, diagnostics
	}

	entries, err := os.ReadDir(vendorDirStr)
	if err != nil {
		diagnostics = append(diagnostics, mustDiagnosticWithCause(
			SeverityWarning,
			CodeVendoredScanFailed,
			fmt.Sprintf("failed to read vendored modules directory %s: %v", vendorDir, err),
			vendorDir,
			err,
		))
		return files, diagnostics
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(vendorDirStr, entry.Name())
		if !invowkmod.IsModule(types.FilesystemPath(entryPath)) {
			continue
		}

		moduleName := strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix)
		if SourceID(moduleName) == SourceIDInvowkfile {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredReservedSkipped,
				fmt.Sprintf("skipping reserved module name '%s' in vendored modules of %s", moduleName, parentModule.Name()),
				types.FilesystemPath(entryPath),
			))
			continue
		}

		m, err := invowkmod.Load(types.FilesystemPath(entryPath))
		if err != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeVendoredModuleLoadSkipped,
				fmt.Sprintf("skipping invalid vendored module at %s: %v", entryPath, err),
				types.FilesystemPath(entryPath),
				err,
			))
			continue
		}

		// Warn if the vendored module has its own invowk_modules/ (not recursed)
		nestedVendorDir := invowkmod.GetVendoredModulesDir(types.FilesystemPath(entryPath))
		if info, statErr := os.Stat(string(nestedVendorDir)); statErr == nil && info.IsDir() {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredNestedIgnored,
				fmt.Sprintf("vendored module %s has its own invowk_modules/ which is not recursed into", m.Name()),
				nestedVendorDir,
			))
		}

		files = append(files, &DiscoveredFile{
			Path:         m.InvowkfilePath(),
			Source:       SourceModule,
			Module:       m,
			ParentModule: parentModule,
		})
	}

	return files, diagnostics
}

// appendModulesWithVendored appends the module files and diagnostics, then for
// each discovered module, scans its invowk_modules/ directory for vendored
// dependencies. This DRYs the pattern used at all 3 module discovery sites
// (local modules, includes, user-dir).
func (d *Discovery) appendModulesWithVendored(
	files []*DiscoveredFile,
	diagnostics []Diagnostic,
	moduleFiles []*DiscoveredFile,
	moduleDiags []Diagnostic,
) ([]*DiscoveredFile, []Diagnostic) {
	diagnostics = append(diagnostics, moduleDiags...)

	for _, mf := range moduleFiles {
		files = append(files, mf)

		// Scan vendored modules owned by this module
		if mf.Module != nil {
			vendoredFiles, vendoredDiags := d.discoverVendoredModulesWithDiagnostics(mf.Module)
			files = append(files, vendoredFiles...)
			diagnostics = append(diagnostics, vendoredDiags...)
		}
	}

	return files, diagnostics
}

// getModuleShortName extracts the short name from a module path.
// e.g., "/path/to/foo.invowkmod" -> "foo"
func getModuleShortName(modulePath string) invowkmod.ModuleShortName {
	base := filepath.Base(modulePath)
	return invowkmod.ModuleShortName(strings.TrimSuffix(base, invowkmod.ModuleSuffix))
}
