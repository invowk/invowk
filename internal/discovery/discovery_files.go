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
func (d *Discovery) discoverInDir(dir string, source Source) *DiscoveredFile {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		slog.Warn("failed to resolve absolute path for discovery directory", "dir", dir, "error", err)
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

// discoverModulesInDirWithDiagnostics finds all valid modules in a directory and
// reports non-fatal warnings for skipped entries.
func (d *Discovery) discoverModulesInDirWithDiagnostics(dir string) ([]*DiscoveredFile, []Diagnostic) {
	var files []*DiscoveredFile
	diagnostics := make([]Diagnostic, 0)

	absDir, err := filepath.Abs(dir)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Code:     "module_scan_path_invalid",
			Message:  fmt.Sprintf("failed to resolve module scan path %q: %v", dir, err),
			Path:     dir,
			Cause:    err,
		})
		return files, diagnostics
	}

	// Check if directory exists
	if _, statErr := os.Stat(absDir); os.IsNotExist(statErr) {
		return files, diagnostics
	}

	// Read directory entries
	entries, err := os.ReadDir(absDir)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Code:     "module_scan_failed",
			Message:  fmt.Sprintf("failed to list directory %s while scanning modules: %v", absDir, err),
			Path:     absDir,
			Cause:    err,
		})
		return files, diagnostics
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
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "reserved_module_name_skipped",
				Message:  fmt.Sprintf("skipping reserved module name '%s'", moduleName),
				Path:     entryPath,
			})
			continue
		}

		// Load the module
		m, err := invowkmod.Load(entryPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "module_load_skipped",
				Message:  fmt.Sprintf("skipping invalid module at %s: %v", entryPath, err),
				Path:     entryPath,
				Cause:    err,
			})
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
		if !invowkmod.IsModule(entry.Path) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "include_not_module",
				Message:  fmt.Sprintf("configured include is not a valid module directory, skipping: %s", entry.Path),
				Path:     entry.Path,
			})
			continue
		}
		moduleName := strings.TrimSuffix(filepath.Base(entry.Path), invowkmod.ModuleSuffix)
		if moduleName == SourceIDInvowkfile {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "include_reserved_module_skipped",
				Message:  fmt.Sprintf("configured include uses reserved module name '%s', skipping", moduleName),
				Path:     entry.Path,
			})
			continue // Skip reserved module name (FR-015)
		}
		m, err := invowkmod.Load(entry.Path)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "include_module_load_failed",
				Message:  fmt.Sprintf("failed to load included module at %s: %v", entry.Path, err),
				Path:     entry.Path,
				Cause:    err,
			})
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
	if _, err := os.Stat(vendorDir); err != nil {
		// No vendor directory is common and not a warning
		return files, diagnostics
	}

	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Code:     "vendored_scan_failed",
			Message:  fmt.Sprintf("failed to read vendored modules directory %s: %v", vendorDir, err),
			Path:     vendorDir,
			Cause:    err,
		})
		return files, diagnostics
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(vendorDir, entry.Name())
		if !invowkmod.IsModule(entryPath) {
			continue
		}

		moduleName := strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix)
		if moduleName == SourceIDInvowkfile {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "vendored_reserved_module_skipped",
				Message:  fmt.Sprintf("skipping reserved module name '%s' in vendored modules of %s", moduleName, parentModule.Name()),
				Path:     entryPath,
			})
			continue
		}

		m, err := invowkmod.Load(entryPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "vendored_module_load_skipped",
				Message:  fmt.Sprintf("skipping invalid vendored module at %s: %v", entryPath, err),
				Path:     entryPath,
				Cause:    err,
			})
			continue
		}

		// Warn if the vendored module has its own invowk_modules/ (not recursed)
		nestedVendorDir := invowkmod.GetVendoredModulesDir(entryPath)
		if info, statErr := os.Stat(nestedVendorDir); statErr == nil && info.IsDir() {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     "vendored_nested_ignored",
				Message:  fmt.Sprintf("vendored module %s has its own invowk_modules/ which is not recursed into", m.Name()),
				Path:     nestedVendorDir,
			})
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
func getModuleShortName(modulePath string) string {
	base := filepath.Base(modulePath)
	return strings.TrimSuffix(base, invowkmod.ModuleSuffix)
}
