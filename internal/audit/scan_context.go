// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	invowkfileCUEFileName     = "invowkfile.cue"
	invowkfileNoExtFileName   = "invowkfile"
	invowkfileParseDiagFormat = "invowkfile parse error: %s: %v"

	diagnosticInvowkfileParseError  DiagnosticCode = "invowkfile_parse_error"
	diagnosticModuleSkipped         DiagnosticCode = "module_skipped"
	diagnosticDiscoveryPartial      DiagnosticCode = "discovery_partial"
	diagnosticVendoredModuleSkipped DiagnosticCode = "vendored_module_skipped"
	diagnosticNestedVendoredIgnored DiagnosticCode = "vendored_nested_ignored"
)

type (
	// ScanContext provides an immutable, read-only view of all discovered artifacts
	// for checkers to analyze. Built once by the scanner before checkers run and
	// shared concurrently across all checkers. Derived views (scripts) are
	// pre-computed at build time to avoid redundant allocations across checkers.
	ScanContext struct {
		rootPath    types.FilesystemPath
		invowkfiles []*ScannedInvowkfile
		modules     []*ScannedModule
		scripts     []ScriptRef
		// diagnostics collects non-fatal warnings from loading (e.g., modules
		// that failed to load, discovery errors). Surfaced in the audit report
		// so incomplete scans are visible to operators.
		diagnostics []Diagnostic
	}

	// ScannedInvowkfile wraps a standalone invowkfile (not inside a module) with
	// its parsed content and a surface identifier for finding attribution.
	ScannedInvowkfile struct {
		Path        types.FilesystemPath
		Invowkfile  *invowkfile.Invowkfile
		SurfaceID   string
		SurfaceKey  ScanSurfaceKey
		SurfaceKind SurfaceKind
		// ParseErr is non-nil when the invowkfile exists on disk but failed to
		// parse. Checkers can inspect this to flag corrupted standalone invowkfiles.
		ParseErr error
	}

	// ScannedModule wraps a discovered module with all its artifacts:
	// parsed invowkfile, lock file, and vendored dependencies.
	ScannedModule struct {
		Path            types.FilesystemPath
		Module          *invowkmod.Module
		Invowkfile      *invowkfile.Invowkfile
		LockFile        *invowkmod.LockFile
		LockPath        types.FilesystemPath
		LockFileSize    int64 //goplint:ignore -- immutable filesystem stat captured for checkers.
		LockFileStatErr error
		VendoredModules []*invowkmod.Module
		VendoredHashes  []invowkmod.VendoredHashEvaluation
		Symlinks        []SymlinkRef
		SymlinkScanErr  error
		SurfaceID       string
		SurfaceKey      ScanSurfaceKey
		SurfaceKind     SurfaceKind
		IsGlobal        bool
		// InvowkfileParseErr is non-nil when the invowkfile exists on disk but
		// failed to parse. Checkers can inspect this to flag modules with
		// corrupted or malformed invowkfiles that would otherwise go undetected.
		InvowkfileParseErr error
		// LockFileParseErr is non-nil when the lock file exists on disk but
		// failed to load/parse. Checkers can inspect this to flag modules with
		// corrupted lock files that would appear as "absent" without this field.
		LockFileParseErr error
	}

	// ScriptRef is a reference to a script within the scan context, annotated with
	// the surface it belongs to. Used by content-analysis checkers.
	ScriptRef struct {
		SurfaceID   string
		SurfaceKey  ScanSurfaceKey
		SurfaceKind SurfaceKind
		FilePath    types.FilesystemPath
		ModulePath  types.FilesystemPath
		CommandName invowkfile.CommandName
		ImplIndex   int
		Script      invowkfile.ScriptContent
		IsFile      bool
		Runtimes    []invowkfile.RuntimeConfig
		ScriptPath  types.FilesystemPath
		FileSize    int64 //goplint:ignore -- immutable filesystem stat captured for checkers.
		FileStatErr error
		// resolvedContent holds the actual script body for content analysis.
		// For inline scripts this equals string(Script). For file-based scripts
		// this holds the file contents (read during context building, capped at
		// maxScriptFileSize bytes). Empty when the file could not be read.
		// Access via Content() method.
		resolvedContent string
	}

	vendoredModuleArtifact struct {
		Path   types.FilesystemPath
		Module *invowkmod.Module
	}

	vendoredModuleArtifacts []vendoredModuleArtifact

	//goplint:ignore -- internal snapshot DTO for filesystem facts captured during scan construction.
	scriptFileFacts struct {
		Path    types.FilesystemPath
		Size    int64  //goplint:ignore -- immutable filesystem stat captured for checkers.
		Content string //goplint:ignore -- transient script body captured for content checkers.
		StatErr error
	}

	//goplint:ignore -- snapshot DTO built internally by scanner and inspected by checkers.
	// SymlinkRef is a point-in-time filesystem fact captured while building the scan context.
	SymlinkRef struct {
		Path         types.FilesystemPath
		RelPath      string //goplint:ignore -- display path relative to scanned module root.
		Target       string //goplint:ignore -- raw symlink target captured from filesystem.
		ReadErr      error
		Dangling     bool
		ChainTooDeep bool
		EscapesRoot  bool
	}

	scanSurfaceIdentity struct {
		id   scanSurfaceID
		key  ScanSurfaceKey
		kind SurfaceKind
		path *types.FilesystemPath
	}

	scanSurfaceID string
)

func (a vendoredModuleArtifact) Validate() error {
	if err := a.Path.Validate(); err != nil {
		return err
	}
	if a.Module == nil {
		return fmt.Errorf("vendored module artifact %q has nil module", a.Path)
	}
	return a.Module.Validate()
}

func (f scriptFileFacts) Validate() error {
	if f.Path == "" {
		return nil
	}
	return f.Path.Validate()
}

// Validate returns nil when the captured symlink path is either empty or a valid filesystem path.
func (r SymlinkRef) Validate() error {
	if r.Path == "" {
		return nil
	}
	return r.Path.Validate()
}

func (id scanSurfaceID) String() string { return string(id) }

func (id scanSurfaceID) Validate() error { return nil }

//goplint:ignore -- constructor validates scanner-owned identity text before returning a typed value.
func newScanSurfaceID(raw string) scanSurfaceID {
	id := scanSurfaceID(raw)
	if err := id.Validate(); err != nil {
		return ""
	}
	return id
}

func (i scanSurfaceIdentity) Validate() error {
	var errs []error
	if err := i.id.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := i.key.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := i.kind.Validate(); err != nil {
		errs = append(errs, err)
	}
	if i.path != nil {
		if err := i.path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Content returns the resolved script body for content analysis. For inline
// scripts this is the script text; for file-based scripts this is the file
// contents read during context building. Empty when the file could not be read.
func (r ScriptRef) Content() string {
	if r.resolvedContent != "" {
		return r.resolvedContent
	}
	if !r.IsFile {
		return string(r.Script)
	}
	return ""
}

// RootPath returns the scan root directory.
func (sc *ScanContext) RootPath() types.FilesystemPath { return sc.rootPath }

// Invowkfiles returns cloned standalone invowkfile snapshots.
// Checkers may inspect the returned values concurrently without mutating the
// scanner-owned snapshot shared with other checkers.
func (sc *ScanContext) Invowkfiles() []*ScannedInvowkfile {
	files := make([]*ScannedInvowkfile, 0, len(sc.invowkfiles))
	for _, file := range sc.invowkfiles {
		files = append(files, cloneScannedInvowkfile(file))
	}
	return files
}

// Modules returns cloned discovered module snapshots.
// Checkers may inspect the returned values concurrently without mutating the
// scanner-owned snapshot shared with other checkers.
func (sc *ScanContext) Modules() []*ScannedModule {
	modules := make([]*ScannedModule, 0, len(sc.modules))
	for _, module := range sc.modules {
		modules = append(modules, cloneScannedModule(module))
	}
	return modules
}

// AllScripts returns a copy of the pre-computed script references.
// Safe for concurrent use by multiple checkers.
func (sc *ScanContext) AllScripts() []ScriptRef {
	scripts := make([]ScriptRef, 0, len(sc.scripts))
	for i := range sc.scripts {
		scripts = append(scripts, cloneScriptRef(sc.scripts[i]))
	}
	return scripts
}

// ScriptCount returns the total number of scripts across all surfaces.
func (sc *ScanContext) ScriptCount() int { return len(sc.scripts) }

// Diagnostics returns non-fatal warnings collected during context building
// (e.g., modules that failed to load, discovery errors, parse failures).
func (sc *ScanContext) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), sc.diagnostics...)
}

func (sc *ScanContext) addDiagnostic(code DiagnosticCode, message string, path types.FilesystemPath) {
	diagnostic, err := NewDiagnostic("warning", code, DiagnosticMessage(message), withDiagnosticPath(path)) //goplint:ignore -- diagnostic message is assembled at scanner boundary.
	if err != nil {
		return
	}
	sc.diagnostics = append(sc.diagnostics, diagnostic)
}

// BuildScanContext discovers and loads all invowkfiles and modules at the given
// path, producing an immutable snapshot for checkers to analyze.
//
// Detection logic:
//   - If path ends with ".cue": standalone invowkfile
//   - If path ends with ".invowkmod": single module
//   - Otherwise: directory tree scan using discovery
func BuildScanContext(scanPath types.FilesystemPath, cfg *config.Config, includeGlobal bool) (*ScanContext, error) {
	absPath, err := fspath.Abs(scanPath)
	if err != nil {
		return nil, &ScanContextBuildError{Path: scanPath, Err: fmt.Errorf("resolving path: %w", err)}
	}

	sc := &ScanContext{
		rootPath: absPath,
	}

	// filepath.Abs removes trailing separators and resolves "." components;
	// suffix checks on the raw scanPath would fail for paths like "./foo.invowkmod/".
	switch {
	case strings.HasSuffix(string(absPath), ".cue"):
		if err := sc.loadStandaloneInvowkfile(absPath); err != nil {
			return nil, &ScanContextBuildError{Path: scanPath, Err: err}
		}
	case strings.HasSuffix(string(absPath), invowkmod.ModuleSuffix):
		if err := sc.loadSingleModule(absPath); err != nil {
			return nil, &ScanContextBuildError{Path: scanPath, Err: err}
		}
	default:
		if err := sc.loadDirectoryTree(absPath, cfg, includeGlobal); err != nil {
			return nil, &ScanContextBuildError{Path: scanPath, Err: err}
		}
	}

	if len(sc.invowkfiles) == 0 && len(sc.modules) == 0 {
		return nil, &ScanContextBuildError{Path: scanPath, Err: ErrNoScanTargets}
	}

	// Pre-compute derived views so checkers share a single allocation.
	sc.scripts = buildScriptRefs(sc.invowkfiles, sc.modules)

	return sc, nil
}

func (sc *ScanContext) loadStandaloneInvowkfile(absPath types.FilesystemPath) error {
	inv, parseErr := invowkfile.Parse(absPath)
	si := &ScannedInvowkfile{
		Path:        absPath,
		SurfaceID:   string(absPath),
		SurfaceKey:  scanSurfaceKey(SurfaceKindRootInvowkfile, absPath),
		SurfaceKind: SurfaceKindRootInvowkfile,
	}
	if parseErr == nil {
		si.Invowkfile = inv
	} else {
		// Capture parse errors rather than hard-failing — consistent with the
		// directory-tree path so checkers can flag corrupted standalone invowkfiles.
		si.ParseErr = parseErr
		sc.addDiagnostic(diagnosticInvowkfileParseError, fmt.Sprintf(invowkfileParseDiagFormat, absPath, parseErr), absPath)
	}
	sc.invowkfiles = append(sc.invowkfiles, si)
	return nil
}

func (sc *ScanContext) loadSingleModule(absPath types.FilesystemPath) error {
	sm, vendored, err := sc.loadScannedModule(absPath, nil, nil, false, false)
	if err != nil {
		return err
	}
	sc.modules = append(sc.modules, sm)
	sc.appendVendoredScannedModules(vendored, false)
	return nil
}

func (sc *ScanContext) loadScannedModule(
	absPath types.FilesystemPath,
	mod *invowkmod.Module,
	inv *invowkfile.Invowkfile,
	isGlobal bool,
	isVendored bool,
) (*ScannedModule, []vendoredModuleArtifact, error) {
	var err error
	if mod == nil {
		mod, err = invowkmod.Load(absPath)
		if err != nil {
			return nil, nil, fmt.Errorf("loading module %s: %w", absPath, err)
		}
	}

	surfaceID := string(absPath)
	if mod.Metadata != nil {
		surfaceID = string(mod.Metadata.Module)
	}

	sm := &ScannedModule{
		Path:        absPath,
		Module:      mod,
		Invowkfile:  inv,
		SurfaceID:   surfaceID,
		SurfaceKey:  scanSurfaceKey(moduleSurfaceKind(isGlobal, isVendored), absPath),
		SurfaceKind: moduleSurfaceKind(isGlobal, isVendored),
		IsGlobal:    isGlobal,
	}

	invPath := fspath.JoinStr(absPath, invowkfileCUEFileName)
	if sm.Invowkfile == nil {
		if _, statErr := os.Stat(string(invPath)); statErr == nil {
			parsed, parseErr := invowkfile.Parse(invPath)
			if parseErr == nil {
				sm.Invowkfile = parsed
			} else {
				sm.InvowkfileParseErr = parseErr
			}
		}
	}

	lockPath := fspath.JoinStr(absPath, invowkmod.LockFileName)
	lockSnapshot := invowkmod.InspectLockFile(lockPath)
	if lockSnapshot.Present || lockSnapshot.StatErr != nil {
		sm.LockPath = lockPath
		sm.LockFile = lockSnapshot.LockFile
		sm.LockFileSize = lockSnapshot.Size
		sm.LockFileStatErr = lockSnapshot.StatErr
		sm.LockFileParseErr = lockSnapshot.ParseErr
	}

	vendored, vendorDiags := loadVendoredModules(absPath)
	sm.VendoredModules = vendored.moduleList()
	sm.VendoredHashes = buildVendoredHashEvaluations(sm.LockFile, sm.VendoredModules)
	sm.Symlinks, sm.SymlinkScanErr = scanModuleSymlinks(absPath)
	sc.diagnostics = append(sc.diagnostics, vendorDiags...)

	return sm, vendored, nil
}

func (sc *ScanContext) loadDirectoryTree(absPath types.FilesystemPath, cfg *config.Config, includeGlobal bool) error {
	sc.loadDirectoryInvowkfile(absPath)
	if err := sc.loadDirectoryModules(absPath); err != nil {
		return err
	}
	sc.loadDiscoveryResults(absPath, cfg, includeGlobal)
	return nil
}

func (sc *ScanContext) loadDirectoryInvowkfile(absPath types.FilesystemPath) {
	invPath := fspath.JoinStr(absPath, invowkfileCUEFileName)
	_, invCueErr := os.Stat(string(invPath))
	if invCueErr == nil {
		sc.appendParsedInvowkfile(invPath)
		return
	}

	// Fall back to extensionless "invowkfile" variant when .cue is absent.
	if !os.IsNotExist(invCueErr) {
		return
	}

	invPathNoExt := fspath.JoinStr(absPath, invowkfileNoExtFileName)
	if _, statErr := os.Stat(string(invPathNoExt)); statErr == nil {
		sc.appendParsedInvowkfile(invPathNoExt)
	}
}

func (sc *ScanContext) appendParsedInvowkfile(path types.FilesystemPath) {
	inv, parseErr := invowkfile.Parse(path)
	si := &ScannedInvowkfile{
		Path:        path,
		SurfaceID:   string(path),
		SurfaceKey:  scanSurfaceKey(SurfaceKindRootInvowkfile, path),
		SurfaceKind: SurfaceKindRootInvowkfile,
	}
	if parseErr == nil {
		si.Invowkfile = inv
	} else {
		si.ParseErr = parseErr
		sc.addDiagnostic(diagnosticInvowkfileParseError, fmt.Sprintf(invowkfileParseDiagFormat, path, parseErr), path)
	}
	sc.invowkfiles = append(sc.invowkfiles, si)
}

func (sc *ScanContext) loadDirectoryModules(absPath types.FilesystemPath) error {
	entries, err := os.ReadDir(string(absPath))
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
			continue
		}
		modPath := fspath.JoinStr(absPath, entry.Name())
		if loadErr := sc.loadSingleModule(modPath); loadErr != nil {
			sc.addDiagnostic(diagnosticModuleSkipped, fmt.Sprintf("skipped invalid module %s: %v", entry.Name(), loadErr), modPath)
			continue
		}
	}

	return nil
}

func (sc *ScanContext) loadDiscoveryResults(absPath types.FilesystemPath, cfg *config.Config, includeGlobal bool) {
	if cfg == nil {
		return
	}

	opts := []discovery.Option{
		discovery.WithBaseDir(absPath),
		discovery.WithVendoredIntegrityVerification(false),
	}
	if !includeGlobal {
		opts = append(opts, discovery.WithCommandsDir(""))
	}

	disc := discovery.New(cfg, opts...)
	files, discErr := disc.DiscoverAll()
	if discErr != nil {
		sc.addDiagnostic(diagnosticDiscoveryPartial, fmt.Sprintf("discovery error (partial results): %v", discErr), "")
	}
	if files != nil {
		sc.mergeDiscoveryResults(files)
	}
}

// mergeDiscoveryResults adds newly-discovered files that weren't already found
// by the direct scan above.
func (sc *ScanContext) mergeDiscoveryResults(files []*discovery.DiscoveredFile) {
	seenInvowkfiles := make(map[string]bool)
	for _, sf := range sc.invowkfiles {
		seenInvowkfiles[string(sf.Path)] = true
	}
	seenModules := make(map[string]bool)
	for _, sm := range sc.modules {
		seenModules[string(sm.Path)] = true
	}

	for _, f := range files {
		if f.Module != nil {
			modPath := string(f.Module.Path)
			if seenModules[modPath] {
				continue
			}
			seenModules[modPath] = true

			// Use module ID when metadata is available, fall back to path
			// to avoid nil-dereference when discovery returns partial results.
			discSurfaceID := string(f.Module.Path)
			if f.Module.Metadata != nil {
				discSurfaceID = string(f.Module.Metadata.Module)
			}

			sm, vendored, err := sc.loadScannedModule(f.Module.Path, f.Module, f.Invowkfile, f.IsGlobalModule, f.ParentModule != nil)
			if err != nil {
				sc.addDiagnostic(diagnosticModuleSkipped, fmt.Sprintf("skipped invalid module %s: %v", f.Module.Name(), err), f.Module.Path)
				continue
			}
			if sm.SurfaceID == string(f.Module.Path) {
				sm.SurfaceID = discSurfaceID
			}
			sm.SurfaceKind = moduleSurfaceKind(f.IsGlobalModule, f.ParentModule != nil)
			sc.modules = append(sc.modules, sm)
			sc.appendVendoredScannedModules(vendored, f.IsGlobalModule)
		} else if f.Invowkfile != nil && !seenInvowkfiles[string(f.Path)] {
			sc.invowkfiles = append(sc.invowkfiles, &ScannedInvowkfile{
				Path:        f.Path,
				Invowkfile:  f.Invowkfile,
				SurfaceID:   string(f.Path),
				SurfaceKey:  scanSurfaceKey(SurfaceKindRootInvowkfile, f.Path),
				SurfaceKind: SurfaceKindRootInvowkfile,
			})
		}
	}
}

// loadVendoredModules scans the invowk_modules/ directory for vendored deps.
// Returns the loaded modules and any diagnostics for modules that failed to load.
func loadVendoredModules(modulePath types.FilesystemPath) (modules vendoredModuleArtifacts, diagnostics []Diagnostic) {
	vendorDir := fspath.JoinStr(modulePath, "invowk_modules")
	entries, err := os.ReadDir(string(vendorDir))
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
			continue
		}
		modPath := fspath.JoinStr(vendorDir, entry.Name())
		mod, loadErr := invowkmod.Load(modPath)
		if loadErr != nil {
			diagnostic, diagErr := NewDiagnostic(
				"warning",
				diagnosticVendoredModuleSkipped,
				DiagnosticMessage(fmt.Sprintf("skipped vendored module %s: %v", entry.Name(), loadErr)),
				withDiagnosticPath(modulePath),
			)
			if diagErr == nil {
				diagnostics = append(diagnostics, diagnostic)
			}
			continue
		}
		artifact := vendoredModuleArtifact{Path: modPath, Module: mod}
		if err := artifact.Validate(); err != nil {
			diagnostic, diagErr := NewDiagnostic(
				"warning",
				diagnosticVendoredModuleSkipped,
				DiagnosticMessage(fmt.Sprintf("skipped invalid vendored module %s: %v", entry.Name(), err)),
				withDiagnosticPath(modulePath),
			)
			if diagErr == nil {
				diagnostics = append(diagnostics, diagnostic)
			}
			continue
		}
		modules = append(modules, artifact)
	}
	return modules, diagnostics
}

func (modules vendoredModuleArtifacts) moduleList() []*invowkmod.Module {
	result := make([]*invowkmod.Module, 0, len(modules))
	for _, module := range modules {
		result = append(result, module.Module)
	}
	return result
}

func (sc *ScanContext) appendVendoredScannedModules(vendored vendoredModuleArtifacts, isGlobal bool) {
	for _, artifact := range vendored {
		sm, nested, err := sc.loadScannedModule(artifact.Path, artifact.Module, nil, isGlobal, true)
		if err != nil {
			sc.addDiagnostic(diagnosticModuleSkipped, fmt.Sprintf("skipped invalid vendored module %s: %v", artifact.Path, err), artifact.Path)
			continue
		}
		if len(nested) > 0 {
			sc.addDiagnostic(
				diagnosticNestedVendoredIgnored,
				fmt.Sprintf("vendored module %s contains %d nested vendored module(s); nested vendored modules are ignored by the flat explicit-only audit policy", artifact.Path, len(nested)),
				artifact.Path,
			)
		}
		sm.SurfaceKind = moduleSurfaceKind(isGlobal, true)
		sm.SurfaceKey = scanSurfaceKey(sm.SurfaceKind, sm.Path)
		sc.modules = append(sc.modules, sm)
	}
}

// buildScriptRefs pre-computes all script references from invowkfiles and modules.
func buildScriptRefs(invowkfiles []*ScannedInvowkfile, modules []*ScannedModule) []ScriptRef {
	var refs []ScriptRef
	for _, sf := range invowkfiles {
		if sf.Invowkfile == nil {
			continue // Parse-failed invowkfiles have no scripts to analyze.
		}
		refs = appendScriptsFromInvowkfile(refs, sf.SurfaceID, sf.SurfaceKey, sf.SurfaceKind, sf.Path, "", sf.Invowkfile)
	}
	for _, sm := range modules {
		if sm.Invowkfile != nil {
			invPath := fspath.JoinStr(sm.Path, invowkfileCUEFileName)
			refs = appendScriptsFromInvowkfile(refs, sm.SurfaceID, sm.SurfaceKey, sm.SurfaceKind, invPath, sm.Path, sm.Invowkfile)
		}
	}
	return refs
}

func appendScriptsFromInvowkfile(refs []ScriptRef, surfaceID string, surfaceKey ScanSurfaceKey, surfaceKind SurfaceKind, filePath, modulePath types.FilesystemPath, inv *invowkfile.Invowkfile) []ScriptRef {
	for ci := range inv.Commands {
		cmd := &inv.Commands[ci]
		for i := range cmd.Implementations {
			impl := &cmd.Implementations[i]
			isFile := impl.IsScriptFile()
			ref := ScriptRef{
				SurfaceID:   surfaceID,
				SurfaceKey:  surfaceKey,
				SurfaceKind: surfaceKind,
				FilePath:    filePath,
				ModulePath:  modulePath,
				CommandName: cmd.Name,
				ImplIndex:   i,
				Script:      impl.Script,
				IsFile:      isFile,
				Runtimes:    impl.Runtimes,
			}

			// Resolve actual script content for content-analysis checkers.
			// For inline scripts, Script already holds the content. For file-based
			// scripts, read the file (capped at maxScriptFileSize) so that
			// content-analysis checkers can inspect the real script body.
			if isFile {
				scriptPath := impl.GetScriptFilePathWithModule(filePath, modulePath)
				facts := readScriptFileFacts(string(scriptPath), string(modulePath))
				ref.ScriptPath = facts.Path
				ref.FileSize = facts.Size
				ref.FileStatErr = facts.StatErr
				ref.resolvedContent = facts.Content
			} else {
				ref.resolvedContent = string(impl.Script)
			}

			refs = append(refs, ref)
		}
	}
	return refs
}

func moduleSurfaceKind(isGlobal, isVendored bool) SurfaceKind {
	switch {
	case isGlobal:
		return SurfaceKindGlobalModule
	case isVendored:
		return SurfaceKindVendoredModule
	default:
		return SurfaceKindLocalModule
	}
}

func (sc *ScanContext) enrichFindingSurfaceIdentity(findings []Finding) {
	surfaces := sc.surfaceIdentities()
	for i := range findings {
		identity, ok := matchSurfaceIdentity(findings[i], surfaces)
		if !ok {
			continue
		}
		if findings[i].SurfaceKind == "" {
			findings[i].SurfaceKind = identity.kind
		}
		if findings[i].SurfaceKey == "" {
			findings[i].SurfaceKey = identity.key
		}
	}
}

func (sc *ScanContext) surfaceIdentities() []scanSurfaceIdentity {
	surfaces := make([]scanSurfaceIdentity, 0, len(sc.invowkfiles)+len(sc.modules))
	for _, sf := range sc.invowkfiles {
		surfaces = append(surfaces, scanSurfaceIdentity{id: newScanSurfaceID(sf.SurfaceID), key: sf.SurfaceKey, kind: sf.SurfaceKind, path: &sf.Path})
	}
	for _, sm := range sc.modules {
		surfaces = append(surfaces, scanSurfaceIdentity{id: newScanSurfaceID(sm.SurfaceID), key: sm.SurfaceKey, kind: sm.SurfaceKind, path: &sm.Path})
	}
	return surfaces
}

func matchSurfaceIdentity(finding Finding, surfaces []scanSurfaceIdentity) (scanSurfaceIdentity, bool) {
	var candidates []scanSurfaceIdentity
	for _, surface := range surfaces {
		if finding.SurfaceID != "" && surface.id.String() != finding.SurfaceID {
			continue
		}
		if finding.SurfaceKind != "" && surface.kind != finding.SurfaceKind {
			continue
		}
		candidates = append(candidates, surface)
	}
	if len(candidates) == 0 {
		return scanSurfaceIdentity{}, false
	}
	if len(candidates) == 1 || finding.FilePath == "" {
		return candidates[0], true
	}
	for _, candidate := range candidates {
		if candidate.path != nil && sameAuditSurfacePath(*candidate.path, finding.FilePath) {
			return candidate, true
		}
	}
	return candidates[0], true
}

func sameAuditSurfacePath(surfacePath, findingPath types.FilesystemPath) bool {
	if surfacePath == "" || findingPath == "" {
		return false
	}
	return string(surfacePath) == string(findingPath) || isWithinBoundary(string(surfacePath), string(findingPath))
}

func scanSurfaceKey(kind SurfaceKind, path types.FilesystemPath) ScanSurfaceKey {
	if path == "" {
		return ""
	}
	return newScanSurfaceKey(string(kind) + "\x00" + string(path))
}

//goplint:ignore -- constructor validates scanner-owned identity text before returning a typed value.
func newScanSurfaceKey(raw string) ScanSurfaceKey {
	key := ScanSurfaceKey(raw)
	if err := key.Validate(); err != nil {
		return ""
	}
	return key
}

func buildVendoredHashEvaluations(lock *invowkmod.LockFile, modules []*invowkmod.Module) []invowkmod.VendoredHashEvaluation {
	if lock == nil || len(modules) == 0 {
		return nil
	}
	evaluations := make([]invowkmod.VendoredHashEvaluation, 0, len(modules))
	for _, module := range modules {
		evaluations = append(evaluations, invowkmod.EvaluateVendoredModuleHash(lock, module))
	}
	return evaluations
}

func scanModuleSymlinks(modulePath types.FilesystemPath) ([]SymlinkRef, error) {
	modPath := string(modulePath)
	var refs []SymlinkRef
	err := filepath.WalkDir(modPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}

		relPath, relErr := filepath.Rel(modPath, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		ref := SymlinkRef{
			Path:    types.FilesystemPath(path), //goplint:ignore -- path comes from filesystem walk.
			RelPath: relPath,
		}
		target, readErr := os.Readlink(path)
		if readErr != nil {
			ref.ReadErr = readErr
			refs = append(refs, ref)
			return continueSymlinkWalk()
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		ref.Target = filepath.Clean(target)
		ref.EscapesRoot = !isWithinBoundary(modPath, ref.Target)
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			ref.Dangling = true
		}
		ref.ChainTooDeep = symlinkChainTooDeep(path)
		refs = append(refs, ref)
		return nil
	})
	if err != nil {
		return refs, fmt.Errorf("walking module symlinks in %s: %w", modulePath, err)
	}
	return refs, nil
}

func continueSymlinkWalk() error {
	return nil
}

//goplint:ignore -- helper walks raw OS-native symlink paths captured from filepath.WalkDir.
func symlinkChainTooDeep(path string) bool {
	current := path
	for range maxSymlinkChainDepth - 1 {
		target, err := os.Readlink(current)
		if err != nil {
			return false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		info, lstatErr := os.Lstat(target)
		if lstatErr != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return false
		}
		current = target
	}
	return true
}

// readScriptFileFacts reads file-based script metadata and contents for content analysis.
// Empty content means the file could not be read or exceeded the scan size cap.
//
//goplint:ignore -- helper resolves raw script path text from invowkfile declarations.
func readScriptFileFacts(scriptPath, modulePath string) scriptFileFacts {
	resolved := strings.TrimSpace(scriptPath)
	if modulePath != "" && !filepath.IsAbs(resolved) {
		resolved = filepath.Join(modulePath, resolved)
	}
	resolvedPath := types.FilesystemPath(resolved) //goplint:ignore -- resolved from validated module/script path inputs.
	facts := scriptFileFacts{Path: resolvedPath}

	// Defense-in-depth: verify the resolved path stays within the module
	// boundary. The invowkfile parser's script path containment check (SC-01)
	// blocks traversal paths at parse time, but the audit scanner should not
	// rely on upstream validation alone.
	if modulePath != "" && !isWithinBoundary(modulePath, resolved) {
		return facts
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		facts.StatErr = statErr
		return facts
	}
	facts.Size = info.Size()

	data, err := os.ReadFile(resolved)
	if err != nil || len(data) > maxScriptFileSize {
		facts.StatErr = err
		return facts
	}
	facts.Content = string(data)
	return facts
}

// isWithinBoundary checks whether target resolves to a path within the base
// directory. Used by multiple checkers for module boundary enforcement.
func isWithinBoundary(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
