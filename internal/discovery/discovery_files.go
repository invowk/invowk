// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
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
		// FromCurrentDir is true when this file was discovered from the
		// current working directory (./invowkfile.cue or ./*.invowkmod).
		FromCurrentDir bool
		// TrustTier records the 3-tier containment level assigned by discovery.
		TrustTier SourceTrustTier
		// MountCeiling records the Layer 2 (<cwd>) or Layer 3 (IncludeEntry.workspace_root)
		// host containment ceiling when applicable.
		MountCeiling types.FilesystemPath //goplint:ignore -- optional nonzero path validated when non-empty.
	}

	// ModuleListResult contains discovered modules and any non-fatal diagnostics
	// produced while scanning discovery sources.
	ModuleListResult struct {
		Modules     []*DiscoveredFile
		Diagnostics []Diagnostic
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

// DiscoverAll finds all invowkfiles from all sources in 6-level precedence order:
//  1. Current directory (highest precedence — its invowkfile.cue, or the module
//     it is when the current directory is a module directory)
//  2. Modules in the current directory (*.invowkmod directories); this level
//     does not run when the current directory is itself a module directory
//  3. Configured includes from config (module paths)
//  4. Provisioned module entries supplied by the command adapter
//  5. Provisioned global module entries supplied by the command adapter
//  6. User commands directory (~/.invowk/cmds — modules only, non-recursive)
//
// Earlier sources take precedence for disambiguation when the same SimpleName
// appears in multiple sources. One module directory reached through several
// of these sources is loaded once, at the highest-precedence source.
func (d *Discovery) DiscoverAll() ([]*DiscoveredFile, error) {
	files, _, err := d.discoverAllWithDiagnostics()
	return files, err
}

// DiscoverModules finds all discovered modules and returns non-fatal diagnostics
// about skipped or invalid discovery inputs.
func (d *Discovery) DiscoverModules() (ModuleListResult, error) {
	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		return ModuleListResult{Diagnostics: diagnostics}, err
	}

	modules := make([]*DiscoveredFile, 0, len(files))
	for _, f := range files {
		if f.Module != nil {
			modules = append(modules, f)
		}
	}

	return ModuleListResult{Modules: modules, Diagnostics: diagnostics}, nil
}

// Validate returns nil if the module list result contains valid discovered
// files and diagnostics.
func (r ModuleListResult) Validate() error {
	var errs []error
	for _, module := range r.Modules {
		if module == nil {
			continue
		}
		if err := module.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for i := range r.Diagnostics {
		if err := r.Diagnostics[i].Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
	// One module directory reached through several sources is one module.
	appended := &appendedModuleDirs{}

	// 1. Current directory (highest precedence)
	// Skip current-dir discovery when baseDir is empty (e.g., os.Getwd() failed
	// because the working directory was deleted). This prevents filepath.Abs("")
	// from silently resolving to the process working directory, which may not exist.
	if d.baseDir != "" {
		baseCeiling := resolveMountBaseCeiling(d.baseDir)
		// The invowkfile and subdirectories of a module directory belong strictly
		// to that module: a current directory that is a module directory (or
		// strictly inside one) loads nothing as a plain invowkfile or child scan.
		isModuleDir, moduleFiles, moduleDiags := discoverCurrentDirModule(d.baseDir)
		if !isModuleDir {
			if cwdFile := d.discoverInDir(d.baseDir, SourceCurrentDir); cwdFile != nil {
				cwdFile.FromCurrentDir = true
				cwdFile.TrustTier = SourceTrustTierLocalCwd
				cwdFile.MountCeiling = baseCeiling
				files = append(files, cwdFile)
			}

			// 2. Modules in current directory (+ their vendored dependencies)
			moduleFiles, moduleDiags = d.discoverModulesInDirWithDiagnostics(d.baseDir)
		}
		for _, mf := range moduleFiles {
			mf.FromCurrentDir = true
			mf.TrustTier = SourceTrustTierLocalCwd
			mf.MountCeiling = baseCeiling
		}
		for i := range moduleDiags {
			moduleDiags[i] = moduleDiags[i].WithCurrentDir()
		}
		files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, moduleFiles, moduleDiags, appended)
		if verifyErr != nil {
			return nil, diagnostics, verifyErr
		}
	}

	// 3. Configured includes (explicit module paths from config) (+ their vendored dependencies)
	includeFiles, includeDiags := d.loadIncludesWithDiagnostics()
	files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, includeFiles, includeDiags, appended)
	if verifyErr != nil {
		return nil, diagnostics, verifyErr
	}

	// Provisioned container images pass module entries through the command
	// adapter so copied modules keep their original command namespace metadata.
	provisionedFiles, provisionedDiags := d.loadProvisionedModulesWithDiagnostics(d.provisionedModules, false)
	files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, provisionedFiles, provisionedDiags, appended)
	if verifyErr != nil {
		return nil, diagnostics, verifyErr
	}
	provisionedGlobalFiles, provisionedGlobalDiags := d.loadProvisionedModulesWithDiagnostics(d.provisionedGlobalModules, true)
	files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, provisionedGlobalFiles, provisionedGlobalDiags, appended)
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
			userModuleFiles[i].TrustTier = SourceTrustTierContained
		}
		files, diagnostics, verifyErr = d.appendModulesWithVendored(files, diagnostics, userModuleFiles, userModuleDiags, appended)
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

func (d *Discovery) loadProvisionedModulesWithDiagnostics(entries ProvisionedModuleEntries, isGlobal bool) ([]*DiscoveredFile, []Diagnostic) {
	var files []*DiscoveredFile
	var diagnostics []Diagnostic
	for _, entry := range entries {
		entryFiles, entryDiags := d.loadProvisionedModuleEntryWithDiagnostics(entry, isGlobal)
		files = append(files, entryFiles...)
		diagnostics = append(diagnostics, entryDiags...)
	}
	return files, diagnostics
}

func (d *Discovery) loadProvisionedModuleEntryWithDiagnostics(entry ProvisionedModuleEntry, isGlobal bool) ([]*DiscoveredFile, []Diagnostic) {
	if entry.Path == "" {
		return nil, nil
	}
	if err := entry.Validate(); err != nil {
		return nil, []Diagnostic{mustDiagnosticWithCause(
			SeverityWarning,
			CodeModuleScanPathInvalid,
			fmt.Sprintf("invalid provisioned module entry %s: %v", entry.Path, err),
			entry.Path,
			err,
		)}
	}
	if invowkmod.IsModule(entry.Path) {
		moduleFile, diagnostics := d.loadProvisionedModuleWithDiagnostics(entry, isGlobal)
		if moduleFile == nil {
			return nil, diagnostics
		}
		return []*DiscoveredFile{moduleFile}, diagnostics
	}

	moduleFiles, diagnostics := d.discoverModulesInDirWithDiagnostics(entry.Path)
	applyProvisionedModuleAttributes(moduleFiles, isGlobal, entry.CommandNamespace)
	return moduleFiles, diagnostics
}

func applyProvisionedModuleAttributes(moduleFiles []*DiscoveredFile, isGlobal bool, namespace invowkmod.ModuleNamespace) {
	for i := range moduleFiles {
		if isGlobal {
			moduleFiles[i].IsGlobalModule = true
		}
		moduleFiles[i].TrustTier = SourceTrustTierContained
		if namespace != "" {
			moduleFiles[i].CommandNamespace = namespace
		}
	}
}

func (d *Discovery) loadProvisionedModuleWithDiagnostics(entry ProvisionedModuleEntry, isGlobal bool) (*DiscoveredFile, []Diagnostic) {
	moduleName := strings.TrimSuffix(filepath.Base(string(entry.Path)), invowkmod.ModuleSuffix)
	if SourceID(moduleName) == SourceIDInvowkfile {
		return nil, []Diagnostic{mustDiagnosticWithPath(
			SeverityWarning,
			CodeReservedModuleNameSkipped,
			fmt.Sprintf("skipping reserved module name '%s'", moduleName),
			entry.Path,
		)}
	}
	m, err := invowkmod.Load(entry.Path)
	if err != nil {
		return nil, []Diagnostic{mustDiagnosticWithCause(
			SeverityWarning,
			CodeModuleLoadSkipped,
			fmt.Sprintf("skipping invalid provisioned module at %s: %v", entry.Path, err),
			entry.Path,
			err,
		)}
	}
	return &DiscoveredFile{
		Path:             m.InvowkfilePath(),
		Source:           SourceModule,
		Module:           m,
		CommandNamespace: entry.CommandNamespace,
		IsGlobalModule:   isGlobal,
		TrustTier:        SourceTrustTierContained,
	}, nil
}

func resolveMountBaseCeiling(baseDir types.FilesystemPath) types.FilesystemPath {
	absBaseDir, absErr := filepath.Abs(string(baseDir))
	if absErr != nil {
		return baseDir
	}
	candidate := types.FilesystemPath(absBaseDir)
	if err := candidate.Validate(); err != nil {
		return baseDir
	}
	return candidate
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
	absDirPath := types.FilesystemPath(absDir)
	if validationErr := absDirPath.Validate(); validationErr != nil {
		diagnostics = append(diagnostics, mustDiagnostic(
			SeverityWarning,
			CodeModuleScanPathInvalid,
			fmt.Sprintf("resolved module scan path %q is invalid: %v", absDir, validationErr),
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
		file, entryDiags := loadModuleEntry(absDirPath, entry)
		diagnostics = append(diagnostics, entryDiags...)
		if file != nil {
			files = append(files, file)
		}
	}

	return files, diagnostics
}

// loadModuleEntry classifies one directory entry of parentDir as a module and
// loads it. It is the single entry classifier shared by the scan of a
// directory's children and by the current-directory module check, so a module
// directory is loaded or skipped the same way whichever directory invowk runs
// from. (The current-directory check also reports a link entry that this
// helper skips without a diagnostic, such as a Windows junction.)
// It returns the module file, or the diagnostics explaining a skip; an entry
// that is not a module candidate returns neither.
func loadModuleEntry(parentDir types.FilesystemPath, entry os.DirEntry) (*DiscoveredFile, []Diagnostic) {
	// Skip symlinks to prevent boundary escape attacks. Defense-in-depth:
	// IsModule also rejects symlinks via os.Lstat, but filtering here
	// avoids unnecessary Lstat calls and provides diagnostic visibility.
	if entry.Type()&os.ModeSymlink != 0 {
		if !strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
			return nil, nil
		}
		message := fmt.Sprintf(symlinkModuleSkippedFormat, entry.Name())
		linkPath, err := validatedDiscoveredEntryPath(parentDir, entry)
		if err != nil {
			return nil, []Diagnostic{mustDiagnostic(SeverityWarning, CodeModuleSymlinkSkipped, message)}
		}
		return nil, []Diagnostic{mustDiagnosticWithPath(SeverityWarning, CodeModuleSymlinkSkipped, message, linkPath)}
	}
	if !entry.IsDir() {
		return nil, nil
	}

	// Check if it's a module
	modPath, err := validatedDiscoveredEntryPath(parentDir, entry)
	if err != nil {
		return nil, []Diagnostic{mustDiagnostic(
			SeverityWarning,
			CodeModuleLoadSkipped,
			fmt.Sprintf("skipping module with invalid path: %v", err),
		)}
	}
	if !invowkmod.IsModule(modPath) {
		return nil, nil
	}

	// Skip reserved module name "invowkfile" (FR-015)
	// The name "invowkfile" is reserved for the canonical namespace system
	// where @invowkfile refers to the root invowkfile.cue source
	moduleName := strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix)
	if SourceID(moduleName) == SourceIDInvowkfile {
		return nil, []Diagnostic{mustDiagnosticWithPath(
			SeverityWarning,
			CodeReservedModuleNameSkipped,
			fmt.Sprintf("skipping reserved module name '%s'", moduleName),
			modPath,
		)}
	}

	// Load the module
	m, err := invowkmod.Load(modPath)
	if err != nil {
		return nil, []Diagnostic{mustDiagnosticWithCause(
			SeverityWarning,
			CodeModuleLoadSkipped,
			fmt.Sprintf("skipping invalid module at %s: %v", modPath, err),
			modPath,
			err,
		)}
	}

	return &DiscoveredFile{
		Path:   m.InvowkfilePath(),
		Source: SourceModule,
		Module: m,
	}, nil
}

func validatedDiscoveredEntryPath(baseDir types.FilesystemPath, entry os.DirEntry) (types.FilesystemPath, error) {
	if entry == nil {
		return "", errors.New("directory entry is required")
	}
	entryPath := types.FilesystemPath(filepath.Join(string(baseDir), entry.Name()))
	if err := entryPath.Validate(); err != nil {
		return "", fmt.Errorf("validate discovered entry path: %w", err)
	}
	return entryPath, nil
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
		includePath := types.FilesystemPath(pathStr)
		if err := includePath.Validate(); err != nil {
			diagnostics = append(diagnostics, mustDiagnostic(
				SeverityWarning,
				CodeIncludeNotModule,
				fmt.Sprintf("configured include has invalid path %q: %v", entry.Path, err),
			))
			continue
		}
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
		var mountCeiling types.FilesystemPath
		if entry.WorkspaceRoot != "" {
			candidate := types.FilesystemPath(entry.WorkspaceRoot) //goplint:ignore -- validated immediately below.
			if err := candidate.Validate(); err != nil {
				diagnostics = append(diagnostics, mustDiagnosticWithCause(
					SeverityWarning,
					CodeIncludeModuleLoadFailed,
					fmt.Sprintf("invalid workspace_root for included module %s: %v", entry.Path, err),
					includePath,
					err,
				))
				continue
			}
			mountCeiling = candidate
		}
		files = append(files, &DiscoveredFile{
			Path:         m.InvowkfilePath(),
			Source:       SourceModule,
			Module:       m,
			TrustTier:    SourceTrustTierInclude,
			MountCeiling: mountCeiling,
		})
	}

	return files, diagnostics
}

// discoverVendoredModulesWithDiagnostics performs a flat, one-level scan of a
// module's invowk_modules/ directory to find vendored dependencies. Nested
// vendoring (invowk_modules inside vendored modules) is not recursed into;
// a diagnostic warning is emitted if detected.
func (d *Discovery) discoverVendoredModulesWithDiagnostics(parentModule *invowkmod.Module) ([]*DiscoveredFile, []Diagnostic, error) {
	var files []*DiscoveredFile
	diagnostics := make([]Diagnostic, 0)

	if parentModule == nil || parentModule.Metadata == nil {
		return files, diagnostics, nil
	}

	vendorDir := invowkmod.GetVendoredModulesDir(parentModule.Path)
	vendorDirStr := string(vendorDir)
	if _, err := os.Stat(vendorDirStr); err != nil {
		if !os.IsNotExist(err) {
			diagnostics = append(diagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeVendoredScanFailed,
				fmt.Sprintf("failed to stat vendored modules directory %s: %v", vendorDir, err),
				vendorDir,
				err,
			))
		}
		return files, diagnostics, nil
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
		return files, diagnostics, nil
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
		return files, diagnostics, nil
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
		vendoredModPath := types.FilesystemPath(entryPath)
		if err := vendoredModPath.Validate(); err != nil {
			return nil, diagnostics, fmt.Errorf("validate vendored module path: %w", err)
		}
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
		ambiguousKeys := invowkmod.AmbiguousDeclaredLockedModuleEntries(parentModule.Metadata.Requires, lock, m.Metadata.Module)
		if ambiguousKeys != nil {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredAmbiguousLockSkipped,
				fmt.Sprintf("skipping vendored module %s in %s: ambiguous declared lock entries [%s] match module ID %s",
					m.Name(), parentModule.Name(), joinModuleRefKeys(ambiguousKeys), m.Metadata.Module),
				vendoredModPath,
			))
			continue
		}

		moduleKey, locked, ok := invowkmod.DeclaredLockedModuleEntry(parentModule.Metadata.Requires, lock, m.Metadata.Module)
		if !ok {
			diagnostics = append(diagnostics, mustDiagnosticWithPath(
				SeverityWarning,
				CodeVendoredUndeclaredSkipped,
				fmt.Sprintf("skipping vendored module %s in %s: not declared and locked by parent invowkmod.cue", m.Name(), parentModule.Name()),
				vendoredModPath,
			))
			continue
		}
		if d.verifyVendoredIntegrity {
			if err := invowkmod.VerifyLockedVendoredModuleHash(moduleKey, locked, m); err != nil {
				return files, diagnostics, fmt.Errorf("vendored module integrity check failed for %s: %w", m.Name(), err)
			}
		}
		if diag, ok := vendoredTransitiveDiagnostic(parentModule, m, vendoredModPath); ok {
			diagnostics = append(diagnostics, diag)
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
			TrustTier:        SourceTrustTierContained,
		})
	}

	return files, diagnostics, nil
}

func vendoredTransitiveDiagnostic(parentModule, childModule *invowkmod.Module, childPath types.FilesystemPath) (Diagnostic, bool) {
	if parentModule == nil || parentModule.Metadata == nil {
		return Diagnostic{}, false
	}
	diags := invowkmod.CheckMissingVendoredTransitiveDeps(parentModule.Metadata.Requires, []*invowkmod.Module{childModule})
	if len(diags) == 0 {
		return Diagnostic{}, false
	}
	missing := diags[0]
	return mustDiagnosticWithPath(
		SeverityWarning,
		CodeVendoredTransitiveSkipped,
		fmt.Sprintf(
			"skipping vendored module %s in %s: transitive dependency %s must be declared by parent invowkmod.cue",
			childModule.Name(),
			parentModule.Name(),
			missing.MissingRef.Key(),
		),
		childPath,
	), true
}

func vendoredCommandNamespace(requirements []invowkmod.ModuleRequirement, lock *invowkmod.LockFile, childModule *invowkmod.Module) invowkmod.ModuleNamespace {
	if childModule == nil || childModule.Metadata == nil {
		return ""
	}
	locked, ok := invowkmod.DeclaredLockedModule(requirements, lock, childModule.Metadata.Module)
	if !ok {
		return ""
	}
	return invowkmod.ModuleNamespace(locked.EffectiveCommandSourceID())
}

//goplint:ignore -- diagnostic display text assembled from validated module ref keys.
func joinModuleRefKeys(keys []invowkmod.ModuleRefKey) string {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key.String())
	}
	return strings.Join(values, ", ")
}

// appendModulesWithVendored appends the module files and diagnostics, then for
// each discovered module, scans its
// invowk_modules/ directory. This DRYs the pattern used at all 3 module
// discovery sites (local modules, includes, user-dir).
//
// Hash verification (SC-10 defense) runs after an entry is confirmed as an
// explicit locked dependency. Returns a hard error on hash mismatch — callers
// must abort discovery.
//
// Vendored children inherit IsGlobalModule from their parent so that scope
// enforcement treats globally-installed vendored commands identically to their
// parent module's commands.
//
// A module directory that is the same physical directory as one appended earlier
// in this discovery run (appended) is merged into that entry instead of being
// appended again, before its vendored modules are scanned.
func (d *Discovery) appendModulesWithVendored(
	files []*DiscoveredFile,
	diagnostics []Diagnostic,
	moduleFiles []*DiscoveredFile,
	moduleDiags []Diagnostic,
	appended *appendedModuleDirs,
) ([]*DiscoveredFile, []Diagnostic, error) {
	diagnostics = append(diagnostics, moduleDiags...)

	for _, mf := range moduleFiles {
		dirInfo, hasDirInfo := topLevelModuleDirInfo(mf)
		if hasDirInfo {
			if kept := appended.find(dirInfo); kept != nil {
				d.mergeDuplicateModule(kept, mf)
				continue
			}
		}

		files = append(files, mf)

		if mf.Module != nil {
			vendoredFiles, vendoredDiags, err := d.discoverVendoredModulesWithDiagnostics(mf.Module)
			if err != nil {
				return files, diagnostics, err
			}
			// Inherit IsGlobalModule from the parent module.
			if mf.IsGlobalModule {
				for _, vf := range vendoredFiles {
					vf.IsGlobalModule = true
				}
			}
			files = append(files, vendoredFiles...)
			diagnostics = append(diagnostics, vendoredDiags...)
			if hasDirInfo {
				appended.add(dirInfo, mf, vendoredFiles)
			}
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
