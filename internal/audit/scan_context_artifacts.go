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

type (
	//goplint:ignore -- scanner state carries raw OS-native paths from filepath.WalkDir.
	luaModuleFileAppender struct {
		ctx        context.Context
		module     *ScannedModule
		modulePath string
		refs       []ScriptRef
		seen       map[string]bool
	}
)

func appendLuaFilesFromModule(ctx context.Context, refs []ScriptRef, module *ScannedModule) ([]ScriptRef, error) {
	if module == nil || module.Path == "" {
		return refs, nil
	}
	modulePath := string(module.Path)
	if _, err := os.Stat(modulePath); err != nil {
		return refs, nil //nolint:nilerr // synthetic test modules and partially loaded modules may not have a filesystem tree.
	}
	appender := luaModuleFileAppender{
		ctx:        ctx,
		module:     module,
		modulePath: modulePath,
		refs:       refs,
		seen:       moduleScriptPathSet(refs, module.Path),
	}
	err := filepath.WalkDir(modulePath, appender.walk)
	if err != nil {
		return refs, fmt.Errorf("walking module Lua files in %s: %w", module.Path, err)
	}
	return appender.refs, nil
}

//goplint:ignore -- filepath.WalkDir requires raw OS path callback parameters.
func (a *luaModuleFileAppender) walk(path string, entry fs.DirEntry, walkErr error) error {
	if err := scanWalkEntryErr(a.ctx, walkErr); err != nil {
		return err
	}
	if entry.IsDir() && entry.Name() == invowkmod.VendoredModulesDir {
		return filepath.SkipDir
	}
	if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
		return nil
	}
	return a.append(path, entry.Name())
}

//goplint:ignore -- Lua module discovery consumes raw filesystem walk paths before storing typed facts.
func (a *luaModuleFileAppender) append(path, fallbackName string) error {
	normalized := filesystemPathFromWalk(path)
	if a.seen[string(normalized)] {
		return nil
	}
	facts, err := readScriptFileFacts(a.ctx, path, a.modulePath)
	if err != nil {
		return err
	}
	scriptFile := invowkfilePathFromLuaModuleRelPath(a.relativePath(path, fallbackName))
	a.refs = append(a.refs, a.scriptRef(facts, scriptFile))
	a.seen[string(normalized)] = true
	return nil
}

//goplint:ignore -- filepath.Rel operates on raw OS paths and returns display-only CUE file paths.
func (a *luaModuleFileAppender) relativePath(path, fallbackName string) string {
	rel, err := filepath.Rel(a.modulePath, path)
	if err != nil {
		return fallbackName
	}
	return rel
}

func (a *luaModuleFileAppender) scriptRef(facts scriptFileFacts, scriptFile invowkfile.FilesystemPath) ScriptRef {
	return ScriptRef{
		SurfaceID:       a.module.SurfaceID,
		SurfaceKey:      a.module.SurfaceKey,
		SurfaceKind:     a.module.SurfaceKind,
		FilePath:        facts.Path,
		ModulePath:      a.module.Path,
		CommandName:     invowkfile.CommandName("lua-file"),
		ImplIndex:       -1,
		Script:          invowkfile.ImplementationScript{File: &scriptFile},
		IsFile:          true,
		Runtimes:        []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtualLua}},
		ScriptPath:      facts.Path,
		FileSize:        facts.Size,
		FileStatErr:     facts.StatErr,
		resolvedContent: facts.Content,
	}
}

func scanWalkEntryErr(ctx context.Context, err error) error {
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
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
	err := filepath.WalkDir(modPath, func(path string, entry fs.DirEntry, walkErr error) error {
		return appendSymlinkRef(ctx, modPath, &refs, path, entry, walkErr)
	})
	if err != nil {
		return refs, fmt.Errorf("walking module symlinks in %s: %w", modulePath, err)
	}
	return refs, nil
}

//goplint:ignore -- filepath.WalkDir requires raw OS path callback parameters.
func appendSymlinkRef(ctx context.Context, modulePath string, refs *[]SymlinkRef, path string, entry fs.DirEntry, walkErr error) error {
	if err := scanWalkEntryErr(ctx, walkErr); err != nil {
		return err
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return nil
	}
	ref, err := moduleSymlinkRef(modulePath, path)
	if err != nil {
		return err
	}
	*refs = append(*refs, ref)
	return nil
}

//goplint:ignore -- symlink scanning operates on raw OS paths reported by filepath.WalkDir.
func moduleSymlinkRef(modulePath, path string) (SymlinkRef, error) {
	relPath, err := filepath.Rel(modulePath, path)
	if err != nil {
		return SymlinkRef{}, fmt.Errorf("computing relative path for %s: %w", path, err)
	}
	ref := SymlinkRef{
		Path:    filesystemPathFromWalk(path),
		RelPath: relPath,
	}
	target, ok := readSymlinkTarget(path, &ref)
	if !ok {
		return ref, nil
	}
	ref.Target = cleanSymlinkTarget(path, target)
	ref.EscapesRoot = !isWithinBoundary(modulePath, ref.Target)
	if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
		ref.Dangling = true
	}
	ref.ChainTooDeep = symlinkChainTooDeep(path)
	return ref, nil
}

//goplint:ignore -- filepath.WalkDir yields already-normalized OS-native paths for scan facts.
func filesystemPathFromWalk(path string) types.FilesystemPath {
	return types.FilesystemPath(path) //goplint:ignore -- path comes from filesystem walk.
}

//goplint:ignore -- filepath.Rel result is stored as an invowkfile script file reference.
func invowkfilePathFromLuaModuleRelPath(path string) invowkfile.FilesystemPath {
	return invowkfile.FilesystemPath(path) //goplint:ignore -- relative path comes from scanned module filesystem.
}

//goplint:ignore -- os.Readlink consumes and returns raw OS-native symlink target strings.
func readSymlinkTarget(path string, ref *SymlinkRef) (string, bool) {
	target, err := os.Readlink(path)
	if err != nil {
		ref.ReadErr = err
		return "", false
	}
	return target, true
}

//goplint:ignore -- symlink normalization joins raw OS-native link and target paths.
func cleanSymlinkTarget(path, target string) string {
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target)
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
