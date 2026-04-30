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

	//goplint:validate-all
	//
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
		// CommandNamespace overrides the command source namespace for vendored
		// modules when the parent lock file records an alias.
		CommandNamespace invowkmod.ModuleNamespace
		// IsGlobalModule is true when this file was discovered from the user
		// commands directory (~/.invowk/cmds). Global module commands are always
		// accessible by any module's CommandScope.
		IsGlobalModule bool
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

// Validate returns nil if the Source is one of the defined source types,
// or an error wrapping ErrInvalidSource if it is not.
func (s Source) Validate() error {
	switch s {
	case SourceCurrentDir, SourceModule:
		return nil
	default:
		return &InvalidSourceError{Value: s}
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

	var verifyErr error

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
		files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, moduleFiles, moduleDiags)
		if verifyErr != nil {
			return nil, diagnostics, verifyErr
		}
	}

	// 3. Configured includes (explicit module paths from config) (+ their vendored dependencies)
	includeFiles, includeDiags := d.loadIncludesWithDiagnostics()
	files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, includeFiles, includeDiags)
	if verifyErr != nil {
		return nil, diagnostics, verifyErr
	}

	// 4. User commands directory (~/.invowk/cmds — modules only, non-recursive) (+ their vendored dependencies)
	// Mark all user-dir modules as global — their commands are accessible by any module's CommandScope.
	// Vendored children inherit IsGlobalModule inside appendModulesWithVendored.
	if d.commandsDir != "" {
		userModuleFiles, userModuleDiags := d.discoverModulesInDirWithDiagnostics(d.commandsDir)
		for i := range userModuleFiles {
			userModuleFiles[i].IsGlobalModule = true
		}
		files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, userModuleFiles, userModuleDiags)
		if verifyErr != nil {
			return nil, diagnostics, verifyErr
		}
	}

	// Check for local modules that shadow global modules (SC-10 defense-in-depth).
	// A local module with the same ModuleID as a globally installed module wins
	// via discovery precedence, which is the safe direction (local doesn't gain
	// global trust). Warn to detect potential namespace collisions or typosquatting.
	diagnostics = append(diagnostics, d.detectModuleShadowing(files)...)

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
		return &DiscoveredFile{Path: types.FilesystemPath(path), Source: source} //goplint:ignore -- os.Stat confirmed path exists
	}

	// Check for invowkfile (no extension)
	path = filepath.Join(absDir, invowkfile.InvowkfileName)
	if _, err := os.Stat(path); err == nil {
		return &DiscoveredFile{Path: types.FilesystemPath(path), Source: source} //goplint:ignore -- os.Stat confirmed path exists
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
			types.FilesystemPath(absDir), //goplint:ignore -- OS-resolved path for diagnostic
			err,
		))
		return files, diagnostics
	}

	for _, entry := range entries {
		// Skip symlinks to prevent boundary escape attacks. Defense-in-depth:
		// IsModule also rejects symlinks via os.Lstat, but filtering here
		// avoids unnecessary Lstat calls and provides diagnostic visibility.
		if entry.Type()&os.ModeSymlink != 0 {
			if strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
				diagnostics = append(diagnostics, mustDiagnosticWithPath(
					SeverityWarning,
					CodeModuleSymlinkSkipped,
					fmt.Sprintf("skipping symlinked module directory %s — symlink modules are not supported", entry.Name()),
					types.FilesystemPath(filepath.Join(absDir, entry.Name())),
				))
			}
			continue
		}
		if !entry.IsDir() {
			continue
		}

		// Check if it's a module
		entryPath := filepath.Join(absDir, entry.Name())
		modPath := types.FilesystemPath(entryPath) //goplint:ignore -- filepath.Join from OS-listed entry
		if !invowkmod.IsModule(modPath) {
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
				modPath,
			))
			continue
		}

		// Load the module
		m, err := invowkmod.Load(modPath)
		if err != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeModuleLoadSkipped,
				fmt.Sprintf("skipping invalid module at %s: %v", entryPath, err),
				modPath,
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
		includePath := types.FilesystemPath(pathStr) //goplint:ignore -- cross-type conversion from config.ModuleIncludePath
		if !invowkmod.IsModule(includePath) {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeIncludeNotModule,
				fmt.Sprintf("configured include is not a valid module directory, skipping: %s", entry.Path),
				includePath,
			))
			continue
		}
		moduleName := strings.TrimSuffix(filepath.Base(pathStr), invowkmod.ModuleSuffix)
		if SourceID(moduleName) == SourceIDInvowkfile {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeIncludeReservedSkipped,
				fmt.Sprintf("configured include uses reserved module name '%s', skipping", moduleName),
				includePath,
			))
			continue // Skip reserved module name (FR-015)
		}
		m, err := invowkmod.Load(includePath)
		if err != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeIncludeModuleLoadFailed,
				fmt.Sprintf("failed to load included module at %s: %v", entry.Path, err),
				includePath,
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

	if parentModule == nil || parentModule.Metadata == nil {
		return files, diagnostics
	}

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

	lockPath := types.FilesystemPath(filepath.Join(string(parentModule.Path), invowkmod.LockFileName)) //goplint:ignore -- derived from validated module path and constant filename
	lock, err := invowkmod.LoadLockFile(string(lockPath))
	if err != nil {
		diagnostics = append(diagnostics, mustDiagnosticWithCause(
			SeverityWarning,
			CodeVendoredScanFailed,
			fmt.Sprintf("failed to load lock file for vendored modules in %s: %v", parentModule.Name(), err),
			lockPath,
			err,
		))
		return files, diagnostics
	}

	for _, entry := range entries {
		// Skip symlinks in vendored modules (same defense as discoverModulesInDirWithDiagnostics).
		if entry.Type()&os.ModeSymlink != 0 {
			if strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
				diagnostics = append(diagnostics, mustDiagnosticWithPath(
					SeverityWarning,
					CodeVendoredSymlinkSkipped,
					fmt.Sprintf("skipping symlinked vendored module %s in %s", entry.Name(), parentModule.Name()),
					types.FilesystemPath(filepath.Join(vendorDirStr, entry.Name())),
				))
			}
			continue
		}
		if !entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(vendorDirStr, entry.Name())
		vendoredModPath := types.FilesystemPath(entryPath) //goplint:ignore -- filepath.Join from OS-listed entry
		if !invowkmod.IsModule(vendoredModPath) {
			continue
		}

		moduleName := strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix)
		if SourceID(moduleName) == SourceIDInvowkfile {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredReservedSkipped,
				fmt.Sprintf("skipping reserved module name '%s' in vendored modules of %s", moduleName, parentModule.Name()),
				vendoredModPath,
			))
			continue
		}

		m, err := invowkmod.Load(vendoredModPath)
		if err != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeVendoredModuleLoadSkipped,
				fmt.Sprintf("skipping invalid vendored module at %s: %v", entryPath, err),
				vendoredModPath,
				err,
			))
			continue
		}
		if !invowkmod.IsDeclaredLockedModule(parentModule.Metadata.Requires, lock, m.Metadata.Module) {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredUndeclaredSkipped,
				fmt.Sprintf("skipping vendored module %s in %s: not declared and locked by parent invowkmod.cue", m.Name(), parentModule.Name()),
				vendoredModPath,
			))
			continue
		}

		// Warn if the vendored module has its own invowk_modules/ (not recursed)
		nestedVendorDir := invowkmod.GetVendoredModulesDir(vendoredModPath)
		if info, statErr := os.Stat(string(nestedVendorDir)); statErr == nil && info.IsDir() {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredNestedIgnored,
				fmt.Sprintf("vendored module %s has its own invowk_modules/ which is not recursed into", m.Name()),
				nestedVendorDir,
			))
		}

		files = append(files, &DiscoveredFile{
			Path:             m.InvowkfilePath(),
			Source:           SourceModule,
			Module:           m,
			ParentModule:     parentModule,
			CommandNamespace: vendoredCommandNamespace(parentModule.Metadata.Requires, lock, m),
		})
	}

	return files, diagnostics
}

func vendoredCommandNamespace(requirements []invowkmod.ModuleRequirement, lock *invowkmod.LockFile, childModule *invowkmod.Module) invowkmod.ModuleNamespace {
	if childModule == nil || childModule.Metadata == nil {
		return ""
	}
	locked, ok := invowkmod.DeclaredLockedModule(requirements, lock, childModule.Metadata.Module)
	if !ok || locked.Alias == "" {
		return ""
	}
	return invowkmod.ModuleNamespace(locked.Alias)
}

// appendModulesWithVendored appends the module files and diagnostics, then for
// each discovered module, verifies vendored dependency integrity and scans its
// invowk_modules/ directory. This DRYs the pattern used at all 3 module
// discovery sites (local modules, includes, user-dir).
//
// Hash verification (SC-10 defense) runs before vendored module discovery so
// that tampered modules are never loaded into the command tree. Returns a hard
// error on hash mismatch — callers must abort discovery.
//
// Vendored children inherit IsGlobalModule from their parent so that scope
// enforcement treats globally-installed vendored commands identically to their
// parent module's commands.
func (d *Discovery) appendModulesWithVendored(
	files []*DiscoveredFile,
	diagnostics []Diagnostic,
	moduleFiles []*DiscoveredFile,
	moduleDiags []Diagnostic,
) ([]*DiscoveredFile, []Diagnostic, error) {
	diagnostics = append(diagnostics, moduleDiags...)

	for _, mf := range moduleFiles {
		files = append(files, mf)

		// Scan vendored modules owned by this module
		if mf.Module != nil && d.verifyVendoredIntegrity {
			// Verify vendored module content hashes against the lock file before
			// loading any vendored code. A mismatch indicates tampering (e.g.,
			// malicious commits, CI cache poisoning) and must abort discovery.
			if err := invowkmod.VerifyVendoredModuleHashes(mf.Module.Path); err != nil {
				return files, diagnostics, fmt.Errorf("vendored module integrity check failed for %s: %w", mf.Module.Name(), err)
			}
		}

		if mf.Module != nil {
			vendoredFiles, vendoredDiags := d.discoverVendoredModulesWithDiagnostics(mf.Module)
			// Inherit IsGlobalModule from the parent module.
			if mf.IsGlobalModule {
				for _, vf := range vendoredFiles {
					vf.IsGlobalModule = true
				}
			}
			files = append(files, vendoredFiles...)
			diagnostics = append(diagnostics, vendoredDiags...)
		}
	}

	return files, diagnostics, nil
}

// detectModuleShadowing checks for local/include modules that have the same
// ModuleID as a globally installed module. Returns warnings for each collision.
func (d *Discovery) detectModuleShadowing(files []*DiscoveredFile) []Diagnostic {
	// Collect global module IDs.
	globalIDs := make(map[invowkmod.ModuleID]types.FilesystemPath)
	for _, f := range files {
		if f.IsGlobalModule && f.Module != nil {
			globalIDs[f.Module.Metadata.Module] = f.Path
		}
	}

	if len(globalIDs) == 0 {
		return nil
	}

	var diagnostics []Diagnostic
	for _, f := range files {
		if f.IsGlobalModule || f.Module == nil {
			continue
		}
		if globalPath, shadows := globalIDs[f.Module.Metadata.Module]; shadows {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeModuleShadowsGlobal,
				fmt.Sprintf(
					"local module '%s' at %s shadows global module at %s — local takes precedence",
					f.Module.Metadata.Module, f.Path, globalPath,
				),
				f.Path,
			))
		}
	}

	return diagnostics
}

// getModuleShortName extracts the short name from a module path.
// e.g., "/path/to/foo.invowkmod" -> "foo"
func getModuleShortName(modulePath types.FilesystemPath) invowkmod.ModuleShortName {
	return invowkmod.ModuleShortName(invowkmod.CommandSourceIDFromModulePath(modulePath))
}
