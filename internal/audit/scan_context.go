// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"fmt"
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
	invowkfileParseDiagFormat = "invowkfile parse error: %s: %v"
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
		diagnostics []string
	}

	// ScannedInvowkfile wraps a standalone invowkfile (not inside a module) with
	// its parsed content and a surface identifier for finding attribution.
	ScannedInvowkfile struct {
		Path       types.FilesystemPath
		Invowkfile *invowkfile.Invowkfile
		SurfaceID  string
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
		VendoredModules []*invowkmod.Module
		SurfaceID       string
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
		FilePath    types.FilesystemPath
		ModulePath  types.FilesystemPath
		CommandName invowkfile.CommandName
		ImplIndex   int
		Script      invowkfile.ScriptContent
		IsFile      bool
		Runtimes    []invowkfile.RuntimeConfig
		// resolvedContent holds the actual script body for content analysis.
		// For inline scripts this equals string(Script). For file-based scripts
		// this holds the file contents (read during context building, capped at
		// maxScriptFileSize bytes). Empty when the file could not be read.
		// Access via Content() method.
		resolvedContent string
	}
)

// Content returns the resolved script body for content analysis. For inline
// scripts this is the script text; for file-based scripts this is the file
// contents read during context building. Empty when the file could not be read.
func (r ScriptRef) Content() string { return r.resolvedContent }

// RootPath returns the scan root directory.
func (sc *ScanContext) RootPath() types.FilesystemPath { return sc.rootPath }

// Invowkfiles returns a copy of the standalone invowkfiles slice.
// The returned slice is safe to iterate without risk of concurrent mutation.
// The pointed-to ScannedInvowkfile structs are shared and must not be modified.
func (sc *ScanContext) Invowkfiles() []*ScannedInvowkfile {
	return append([]*ScannedInvowkfile(nil), sc.invowkfiles...)
}

// Modules returns a copy of the discovered modules slice.
// The returned slice is safe to iterate without risk of concurrent mutation.
// The pointed-to ScannedModule structs are shared and must not be modified.
func (sc *ScanContext) Modules() []*ScannedModule {
	return append([]*ScannedModule(nil), sc.modules...)
}

// AllScripts returns a copy of the pre-computed script references.
// Safe for concurrent use by multiple checkers.
func (sc *ScanContext) AllScripts() []ScriptRef {
	return append([]ScriptRef(nil), sc.scripts...)
}

// ScriptCount returns the total number of scripts across all surfaces.
func (sc *ScanContext) ScriptCount() int { return len(sc.scripts) }

// Diagnostics returns non-fatal warnings collected during context building
// (e.g., modules that failed to load, discovery errors, parse failures).
func (sc *ScanContext) Diagnostics() []string {
	return append([]string(nil), sc.diagnostics...)
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
		Path:      absPath,
		SurfaceID: string(absPath),
	}
	if parseErr == nil {
		si.Invowkfile = inv
	} else {
		// Capture parse errors rather than hard-failing — consistent with the
		// directory-tree path so checkers can flag corrupted standalone invowkfiles.
		si.ParseErr = parseErr
		sc.diagnostics = append(sc.diagnostics, fmt.Sprintf(invowkfileParseDiagFormat, absPath, parseErr))
	}
	sc.invowkfiles = append(sc.invowkfiles, si)
	return nil
}

func (sc *ScanContext) loadSingleModule(absPath types.FilesystemPath) error {
	mod, err := invowkmod.Load(absPath)
	if err != nil {
		return fmt.Errorf("loading module %s: %w", absPath, err)
	}

	// Use module ID as surface identifier when metadata is available.
	// Fall back to path when Metadata is nil (defense against future Load() changes).
	surfaceID := string(absPath)
	if mod.Metadata != nil {
		surfaceID = string(mod.Metadata.Module)
	}

	sm := &ScannedModule{
		Path:      absPath,
		Module:    mod,
		SurfaceID: surfaceID,
	}

	// Load invowkfile if present. Parse errors are captured rather than
	// discarded so that checkers can flag modules with corrupted invowkfiles.
	invPath := fspath.JoinStr(absPath, invowkfileCUEFileName)
	if _, statErr := os.Stat(string(invPath)); statErr == nil {
		inv, parseErr := invowkfile.Parse(invPath)
		if parseErr == nil {
			sm.Invowkfile = inv
		} else {
			sm.InvowkfileParseErr = parseErr
		}
	}

	// Load lock file if present. Parse errors are captured rather than
	// discarded so checkers can flag corrupt lock files that would otherwise
	// appear as absent.
	lockPath := fspath.JoinStr(absPath, invowkmod.LockFileName)
	if _, statErr := os.Stat(string(lockPath)); statErr == nil {
		lock, loadErr := invowkmod.LoadLockFile(string(lockPath))
		if loadErr == nil {
			sm.LockFile = lock
		} else {
			sm.LockFileParseErr = loadErr
		}
		sm.LockPath = lockPath
	}

	// Scan vendored modules (diagnostics propagated for failed loads).
	var vendorDiags []string
	sm.VendoredModules, vendorDiags = loadVendoredModules(absPath)
	sc.diagnostics = append(sc.diagnostics, vendorDiags...)

	sc.modules = append(sc.modules, sm)
	return nil
}

func (sc *ScanContext) loadDirectoryTree(absPath types.FilesystemPath, cfg *config.Config, includeGlobal bool) error {
	// Try direct invowkfile.cue detection first (simplest case).
	// Parse errors are captured in ParseErr so checkers can flag corrupted
	// invowkfiles that would otherwise be silently excluded from the scan.
	invPath := fspath.JoinStr(absPath, invowkfileCUEFileName)
	_, invCueErr := os.Stat(string(invPath))
	if invCueErr == nil {
		inv, parseErr := invowkfile.Parse(invPath)
		si := &ScannedInvowkfile{
			Path:      invPath,
			SurfaceID: string(invPath),
		}
		if parseErr == nil {
			si.Invowkfile = inv
		} else {
			si.ParseErr = parseErr
			sc.diagnostics = append(sc.diagnostics, fmt.Sprintf(invowkfileParseDiagFormat, invPath, parseErr))
		}
		sc.invowkfiles = append(sc.invowkfiles, si)
	}

	// Fall back to extensionless "invowkfile" variant when .cue is absent.
	if os.IsNotExist(invCueErr) {
		invPathNoExt := fspath.JoinStr(absPath, "invowkfile")
		if _, statErr := os.Stat(string(invPathNoExt)); statErr == nil {
			inv, parseErr := invowkfile.Parse(invPathNoExt)
			si := &ScannedInvowkfile{
				Path:      invPathNoExt,
				SurfaceID: string(invPathNoExt),
			}
			if parseErr == nil {
				si.Invowkfile = inv
			} else {
				si.ParseErr = parseErr
				sc.diagnostics = append(sc.diagnostics, fmt.Sprintf(invowkfileParseDiagFormat, invPathNoExt, parseErr))
			}
			sc.invowkfiles = append(sc.invowkfiles, si)
		}
	}

	// Scan for module directories.
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
			sc.diagnostics = append(sc.diagnostics, fmt.Sprintf("skipped invalid module %s: %v", entry.Name(), loadErr))
			continue
		}
	}

	// Use full discovery for additional sources (includes, global modules).
	if cfg != nil {
		opts := []discovery.Option{
			discovery.WithBaseDir(absPath),
		}
		if !includeGlobal {
			opts = append(opts, discovery.WithCommandsDir(""))
		}

		disc := discovery.New(cfg, opts...)
		files, discErr := disc.DiscoverAll()
		if discErr != nil {
			sc.diagnostics = append(sc.diagnostics, fmt.Sprintf("discovery error (partial results): %v", discErr))
		}
		if files != nil {
			sc.mergeDiscoveryResults(files)
		}
	}

	return nil
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

			sm := &ScannedModule{
				Path:      f.Module.Path,
				Module:    f.Module,
				SurfaceID: discSurfaceID,
				IsGlobal:  f.IsGlobalModule,
			}
			if f.Invowkfile != nil {
				sm.Invowkfile = f.Invowkfile
			}
			sc.modules = append(sc.modules, sm)
		} else if f.Invowkfile != nil && !seenInvowkfiles[string(f.Path)] {
			sc.invowkfiles = append(sc.invowkfiles, &ScannedInvowkfile{
				Path:       f.Path,
				Invowkfile: f.Invowkfile,
				SurfaceID:  string(f.Path),
			})
		}
	}
}

// loadVendoredModules scans the invowk_modules/ directory for vendored deps.
// Returns the loaded modules and any diagnostics for modules that failed to load.
func loadVendoredModules(modulePath types.FilesystemPath) (modules []*invowkmod.Module, diagnostics []string) {
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
			diagnostics = append(diagnostics, fmt.Sprintf("skipped vendored module %s: %v", entry.Name(), loadErr))
			continue
		}
		modules = append(modules, mod)
	}
	return modules, diagnostics
}

// buildScriptRefs pre-computes all script references from invowkfiles and modules.
func buildScriptRefs(invowkfiles []*ScannedInvowkfile, modules []*ScannedModule) []ScriptRef {
	var refs []ScriptRef
	for _, sf := range invowkfiles {
		if sf.Invowkfile == nil {
			continue // Parse-failed invowkfiles have no scripts to analyze.
		}
		refs = appendScriptsFromInvowkfile(refs, sf.SurfaceID, sf.Path, "", sf.Invowkfile)
	}
	for _, sm := range modules {
		if sm.Invowkfile != nil {
			invPath := fspath.JoinStr(sm.Path, invowkfileCUEFileName)
			refs = appendScriptsFromInvowkfile(refs, sm.SurfaceID, invPath, sm.Path, sm.Invowkfile)
		}
	}
	return refs
}

func appendScriptsFromInvowkfile(refs []ScriptRef, surfaceID string, filePath, modulePath types.FilesystemPath, inv *invowkfile.Invowkfile) []ScriptRef {
	for ci := range inv.Commands {
		cmd := &inv.Commands[ci]
		for i := range cmd.Implementations {
			impl := &cmd.Implementations[i]
			isFile := impl.IsScriptFile()
			ref := ScriptRef{
				SurfaceID:   surfaceID,
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
				ref.resolvedContent = readScriptFileContent(string(impl.Script), string(modulePath))
			} else {
				ref.resolvedContent = string(impl.Script)
			}

			refs = append(refs, ref)
		}
	}
	return refs
}

// readScriptFileContent reads a file-based script's contents for content analysis.
// Returns empty string if the file cannot be read (checkers handle empty gracefully).
func readScriptFileContent(scriptPath, modulePath string) string {
	resolved := strings.TrimSpace(scriptPath)
	if modulePath != "" && !filepath.IsAbs(resolved) {
		resolved = filepath.Join(modulePath, resolved)
	}

	// Defense-in-depth: verify the resolved path stays within the module
	// boundary. The invowkfile parser's script path containment check (SC-01)
	// blocks traversal paths at parse time, but the audit scanner should not
	// rely on upstream validation alone.
	if modulePath != "" && !isWithinBoundary(modulePath, resolved) {
		return ""
	}

	data, err := os.ReadFile(resolved)
	if err != nil || len(data) > maxScriptFileSize {
		return ""
	}
	return string(data)
}

// isWithinBoundary checks whether target resolves to a path within the base
// directory. Used by multiple checkers for module boundary enforcement.
func isWithinBoundary(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	return err == nil && !strings.HasPrefix(rel, "..")
}
