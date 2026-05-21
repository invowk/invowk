// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func appendLuaFilesFromModule(ctx context.Context, refs []ScriptRef, module *ScannedModule) ([]ScriptRef, error) {
	if module == nil || module.Path == "" {
		return refs, nil
	}
	seen := moduleScriptPathSet(refs, module.Path)
	modulePath := string(module.Path)
	if _, err := os.Stat(modulePath); err != nil {
		return refs, nil //nolint:nilerr // synthetic test modules and partially loaded modules may not have a filesystem tree.
	}
	err := filepath.WalkDir(modulePath, func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == invowkmod.VendoredModulesDir {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
			return nil
		}
		normalized := types.FilesystemPath(path) //goplint:ignore -- path comes from filesystem walk.
		if seen[string(normalized)] {
			return nil
		}
		facts, factsErr := readScriptFileFacts(ctx, path, modulePath)
		if factsErr != nil {
			return factsErr
		}
		rel, relErr := filepath.Rel(modulePath, path)
		if relErr != nil {
			rel = entry.Name()
		}
		scriptFile := invowkfile.FilesystemPath(rel)
		refs = append(refs, ScriptRef{
			SurfaceID:       module.SurfaceID,
			SurfaceKey:      module.SurfaceKey,
			SurfaceKind:     module.SurfaceKind,
			FilePath:        facts.Path,
			ModulePath:      module.Path,
			CommandName:     invowkfile.CommandName("lua-file"),
			ImplIndex:       -1,
			Script:          invowkfile.ImplementationScript{File: &scriptFile},
			IsFile:          true,
			Runtimes:        []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtualLua}},
			ScriptPath:      facts.Path,
			FileSize:        facts.Size,
			FileStatErr:     facts.StatErr,
			resolvedContent: facts.Content,
		})
		seen[string(normalized)] = true
		return nil
	})
	if err != nil {
		return refs, fmt.Errorf("walking module Lua files in %s: %w", module.Path, err)
	}
	return refs, nil
}

func moduleScriptPathSet(refs []ScriptRef, modulePath types.FilesystemPath) map[string]bool {
	seen := make(map[string]bool)
	for i := range refs {
		ref := refs[i]
		if ref.ModulePath != modulePath || ref.ScriptPath == "" {
			continue
		}
		seen[string(ref.ScriptPath)] = true
	}
	return seen
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

func scanModuleSymlinks(ctx context.Context, modulePath types.FilesystemPath) ([]SymlinkRef, error) {
	if err := scanContextErr(ctx); err != nil {
		return nil, err
	}
	modPath := string(modulePath)
	var refs []SymlinkRef
	err := filepath.WalkDir(modPath, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			return ctxErr
		}
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
