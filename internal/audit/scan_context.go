// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
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
	}

	// ScannedInvowkfile wraps a standalone invowkfile (not inside a module) with
	// its parsed content and a surface identifier for finding attribution.
	ScannedInvowkfile struct {
		Path       types.FilesystemPath
		Invowkfile *invowkfile.Invowkfile
		SurfaceID  string
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
	}
)

// RootPath returns the scan root directory.
func (sc *ScanContext) RootPath() types.FilesystemPath { return sc.rootPath }

// Invowkfiles returns standalone invowkfiles (not inside modules).
func (sc *ScanContext) Invowkfiles() []*ScannedInvowkfile { return sc.invowkfiles }

// Modules returns all discovered modules.
func (sc *ScanContext) Modules() []*ScannedModule { return sc.modules }

// AllScripts returns the pre-computed list of all script references across all
// invowkfiles (standalone + module). Safe for concurrent use by multiple checkers.
func (sc *ScanContext) AllScripts() []ScriptRef { return sc.scripts }

// ScriptCount returns the total number of scripts across all surfaces.
func (sc *ScanContext) ScriptCount() int { return len(sc.scripts) }

// BuildScanContext discovers and loads all invowkfiles and modules at the given
// path, producing an immutable snapshot for checkers to analyze.
//
// Detection logic:
//   - If path ends with ".cue": standalone invowkfile
//   - If path ends with ".invowkmod": single module
//   - Otherwise: directory tree scan using discovery
func BuildScanContext(scanPath types.FilesystemPath, cfg *config.Config, includeGlobal bool) (*ScanContext, error) {
	absPath, err := filepath.Abs(string(scanPath))
	if err != nil {
		return nil, &ScanContextBuildError{Path: scanPath, Err: fmt.Errorf("resolving path: %w", err)}
	}

	sc := &ScanContext{
		rootPath: types.FilesystemPath(absPath),
	}

	pathStr := string(scanPath)

	switch {
	case strings.HasSuffix(pathStr, ".cue"):
		if err := sc.loadStandaloneInvowkfile(absPath); err != nil {
			return nil, &ScanContextBuildError{Path: scanPath, Err: err}
		}
	case strings.HasSuffix(pathStr, invowkmod.ModuleSuffix):
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

func (sc *ScanContext) loadStandaloneInvowkfile(absPath string) error {
	inv, err := invowkfile.Parse(invowkfile.FilesystemPath(absPath))
	if err != nil {
		return fmt.Errorf("parsing invowkfile %s: %w", absPath, err)
	}
	sc.invowkfiles = append(sc.invowkfiles, &ScannedInvowkfile{
		Path:       types.FilesystemPath(absPath),
		Invowkfile: inv,
		SurfaceID:  absPath,
	})
	return nil
}

func (sc *ScanContext) loadSingleModule(absPath string) error {
	mod, err := invowkmod.Load(types.FilesystemPath(absPath))
	if err != nil {
		return fmt.Errorf("loading module %s: %w", absPath, err)
	}

	sm := &ScannedModule{
		Path:      types.FilesystemPath(absPath),
		Module:    mod,
		SurfaceID: string(mod.Metadata.Module),
	}

	// Load invowkfile if present. Parse errors are captured rather than
	// discarded so that checkers can flag modules with corrupted invowkfiles.
	invPath := filepath.Join(absPath, "invowkfile.cue")
	if _, statErr := os.Stat(invPath); statErr == nil {
		inv, parseErr := invowkfile.Parse(invowkfile.FilesystemPath(invPath))
		if parseErr == nil {
			sm.Invowkfile = inv
		} else {
			sm.InvowkfileParseErr = parseErr
		}
	}

	// Load lock file if present.
	lockPath := filepath.Join(absPath, invowkmod.LockFileName)
	if _, statErr := os.Stat(lockPath); statErr == nil {
		lock, loadErr := invowkmod.LoadLockFile(lockPath)
		if loadErr == nil {
			sm.LockFile = lock
			sm.LockPath = types.FilesystemPath(lockPath) //goplint:ignore -- filepath.Join from validated module path
		}
	}

	// Scan vendored modules.
	sm.VendoredModules = loadVendoredModules(absPath)

	sc.modules = append(sc.modules, sm)
	return nil
}

func (sc *ScanContext) loadDirectoryTree(absPath string, cfg *config.Config, includeGlobal bool) error {
	// Try direct invowkfile.cue detection first (simplest case).
	invPath := filepath.Join(absPath, "invowkfile.cue")
	_, invCueErr := os.Stat(invPath)
	if invCueErr == nil {
		inv, parseErr := invowkfile.Parse(invowkfile.FilesystemPath(invPath))
		if parseErr == nil {
			sc.invowkfiles = append(sc.invowkfiles, &ScannedInvowkfile{
				Path:       types.FilesystemPath(invPath),
				Invowkfile: inv,
				SurfaceID:  invPath,
			})
		}
	}

	// Fall back to extensionless "invowkfile" variant when .cue is absent.
	if os.IsNotExist(invCueErr) {
		invPathNoExt := filepath.Join(absPath, "invowkfile")
		if _, statErr := os.Stat(invPathNoExt); statErr == nil {
			inv, parseErr := invowkfile.Parse(invowkfile.FilesystemPath(invPathNoExt))
			if parseErr == nil {
				sc.invowkfiles = append(sc.invowkfiles, &ScannedInvowkfile{
					Path:       types.FilesystemPath(invPathNoExt),
					Invowkfile: inv,
					SurfaceID:  invPathNoExt,
				})
			}
		}
	}

	// Scan for module directories.
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
			continue
		}
		modPath := filepath.Join(absPath, entry.Name())
		if loadErr := sc.loadSingleModule(modPath); loadErr != nil {
			continue // Skip invalid modules.
		}
	}

	// Use full discovery for additional sources (includes, global modules).
	if cfg != nil {
		opts := []discovery.Option{
			discovery.WithBaseDir(types.FilesystemPath(absPath)),
		}
		if !includeGlobal {
			opts = append(opts, discovery.WithCommandsDir(""))
		}

		disc := discovery.New(cfg, opts...)
		files, discErr := disc.DiscoverAll()
		if discErr == nil {
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

			sm := &ScannedModule{
				Path:      f.Module.Path,
				Module:    f.Module,
				SurfaceID: string(f.Module.Metadata.Module),
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
func loadVendoredModules(modulePath string) []*invowkmod.Module {
	vendorDir := filepath.Join(modulePath, "invowk_modules")
	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		return nil
	}

	var modules []*invowkmod.Module
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
			continue
		}
		modPath := filepath.Join(vendorDir, entry.Name())
		mod, loadErr := invowkmod.Load(types.FilesystemPath(modPath))
		if loadErr != nil {
			continue
		}
		modules = append(modules, mod)
	}
	return modules
}

// buildScriptRefs pre-computes all script references from invowkfiles and modules.
func buildScriptRefs(invowkfiles []*ScannedInvowkfile, modules []*ScannedModule) []ScriptRef {
	var refs []ScriptRef
	for _, sf := range invowkfiles {
		refs = appendScriptsFromInvowkfile(refs, sf.SurfaceID, sf.Path, "", sf.Invowkfile)
	}
	for _, sm := range modules {
		if sm.Invowkfile != nil {
			invPath := types.FilesystemPath(filepath.Join(string(sm.Path), "invowkfile.cue")) //goplint:ignore -- filepath.Join from validated module path
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
			refs = append(refs, ScriptRef{
				SurfaceID:   surfaceID,
				FilePath:    filePath,
				ModulePath:  modulePath,
				CommandName: cmd.Name,
				ImplIndex:   i,
				Script:      impl.Script,
				IsFile:      impl.IsScriptFile(),
				Runtimes:    impl.Runtimes,
			})
		}
	}
	return refs
}
