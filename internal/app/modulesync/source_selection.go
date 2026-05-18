// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"errors"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	selectedSourceModule struct {
		path     *types.FilesystemPath
		metadata *invowkmod.Invowkmod
	}

	ambiguousModuleSourceError struct {
		repoPath   *types.FilesystemPath
		candidates []SubdirectoryPath
	}

	moduleSourceNotFoundError struct {
		path *types.FilesystemPath
	}

	invalidModuleSubpathError struct {
		path *SubdirectoryPath
		err  error
	}

	moduleSubpathIdentityMismatchError struct {
		path       SubdirectoryPath
		dirModule  ModuleID
		metaModule ModuleID
	}

	// CanonicalModuleCollisionError reports two source identities resolving to
	// the same canonical local module directory.
	CanonicalModuleCollisionError struct {
		ModuleID      ModuleID
		DirectoryName invowkmod.ModuleScaffoldDirectoryName
		ExistingKey   ModuleRefKey
		IncomingKey   ModuleRefKey
	}
)

func (e *ambiguousModuleSourceError) Error() string {
	candidates := make([]string, 0, len(e.candidates))
	for _, candidate := range e.candidates {
		candidates = append(candidates, candidate.String())
	}
	return fmt.Sprintf(
		"ambiguous module source in %s: repository root is not a module; set path to one of [%s]",
		formatFilesystemPath(e.repoPath),
		strings.Join(candidates, ", "),
	)
}

func (e *ambiguousModuleSourceError) Unwrap() error { return ErrAmbiguousModuleSource }

func (e *moduleSourceNotFoundError) Error() string {
	return fmt.Sprintf("module source not found in %s: expected invowkmod.cue and invowkfile.cue", formatFilesystemPath(e.path))
}

func (e *moduleSourceNotFoundError) Unwrap() error { return ErrModuleSourceNotFound }

func (e *invalidModuleSubpathError) Error() string {
	return fmt.Sprintf("invalid module subpath %q: %s", formatSubdirectoryPath(e.path), e.err)
}

func (e *invalidModuleSubpathError) Unwrap() error { return ErrInvalidModuleSubpath }

func (e *moduleSubpathIdentityMismatchError) Error() string {
	return fmt.Sprintf(
		"module subpath %q declares module %q but directory name implies %q",
		e.path,
		e.metaModule,
		e.dirModule,
	)
}

func (e *moduleSubpathIdentityMismatchError) Unwrap() error {
	return ErrModuleSubpathIdentityMismatch
}

// Error implements the error interface for CanonicalModuleCollisionError.
func (e *CanonicalModuleCollisionError) Error() string {
	return fmt.Sprintf(
		"canonical module collision for %s: %s and %s both materialize as %s",
		e.ModuleID,
		e.ExistingKey,
		e.IncomingKey,
		e.DirectoryName,
	)
}

// Unwrap returns ErrCanonicalModuleCollision for errors.Is compatibility.
func (e *CanonicalModuleCollisionError) Unwrap() error {
	return ErrCanonicalModuleCollision
}

func (m selectedSourceModule) Validate() error {
	var errs []error
	if m.path == nil {
		errs = append(errs, errors.New("source module path is required"))
	} else if err := m.path.Validate(); err != nil {
		errs = append(errs, err)
	}
	if m.metadata == nil {
		errs = append(errs, errors.New("source module metadata is required"))
	} else if err := m.metadata.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.New(types.FormatFieldErrors("selected source module", errs))
	}
	return nil
}

func (m selectedSourceModule) Path() types.FilesystemPath {
	if m.path == nil {
		return ""
	}
	return *m.path
}

func selectSourceModule(repoPath types.FilesystemPath, req ModuleRef) (selectedSourceModule, error) {
	if req.Path == "" {
		return selectRootSourceModule(repoPath)
	}
	return selectSubpathSourceModule(repoPath, req.Path)
}

func selectRootSourceModule(repoPath types.FilesystemPath) (selectedSourceModule, error) {
	if hasModuleSourceFiles(repoPath) {
		return parseSelectedSourceModule(repoPath)
	}

	candidates, err := childModuleCandidates(repoPath)
	if err != nil {
		return selectedSourceModule{}, err
	}
	if len(candidates) > 0 {
		return selectedSourceModule{}, &ambiguousModuleSourceError{
			repoPath:   new(repoPath),
			candidates: candidates,
		}
	}
	return selectedSourceModule{}, &moduleSourceNotFoundError{path: new(repoPath)}
}

func selectSubpathSourceModule(repoPath types.FilesystemPath, subPath SubdirectoryPath) (selectedSourceModule, error) {
	normalized, err := normalizeModuleSubpath(subPath)
	if err != nil {
		return selectedSourceModule{}, err
	}

	dirName, err := invowkmod.ParseModuleName(slashpath.Base(normalized.String()))
	if err != nil {
		return selectedSourceModule{}, &invalidModuleSubpathError{
			path: new(subPath),
			err:  errors.New("subpath basename must be a valid .invowkmod directory name"),
		}
	}

	sourcePath := fspath.JoinStr(repoPath, filepath.FromSlash(normalized.String()))
	if !hasModuleSourceFiles(sourcePath) {
		return selectedSourceModule{}, &moduleSourceNotFoundError{path: new(sourcePath)}
	}

	selected, err := parseSelectedSourceModule(sourcePath)
	if err != nil {
		return selectedSourceModule{}, err
	}
	dirModule := ModuleID(dirName)
	if selected.metadata.Module != dirModule {
		return selectedSourceModule{}, &moduleSubpathIdentityMismatchError{
			path:       normalized,
			dirModule:  dirModule,
			metaModule: selected.metadata.Module,
		}
	}
	return selected, nil
}

func normalizeModuleSubpath(subPath SubdirectoryPath) (SubdirectoryPath, error) {
	if err := subPath.Validate(); err != nil {
		return "", &invalidModuleSubpathError{path: new(subPath), err: err}
	}
	normalized := SubdirectoryPath(slashpath.Clean(strings.ReplaceAll(subPath.String(), "\\", "/"))) //goplint:ignore -- validated repository-relative path normalization.
	if err := normalized.Validate(); err != nil {
		return "", &invalidModuleSubpathError{path: new(subPath), err: err}
	}
	return normalized, nil
}

func parseSelectedSourceModule(sourcePath types.FilesystemPath) (selectedSourceModule, error) {
	meta, err := invowkmod.ParseModuleMetadataOnly(sourcePath)
	if err != nil {
		return selectedSourceModule{}, err
	}
	selected := selectedSourceModule{
		path:     new(sourcePath),
		metadata: meta,
	}
	if err := selected.Validate(); err != nil {
		return selectedSourceModule{}, err
	}
	return selected, nil
}

func hasModuleSourceFiles(dir types.FilesystemPath) bool {
	return regularFileExists(fspath.JoinStr(dir, "invowkmod.cue")) &&
		regularFileExists(fspath.JoinStr(dir, "invowkfile.cue"))
}

func regularFileExists(path types.FilesystemPath) bool {
	info, err := os.Stat(string(path))
	return err == nil && info.Mode().IsRegular()
}

func childModuleCandidates(repoPath types.FilesystemPath) ([]SubdirectoryPath, error) {
	entries, err := os.ReadDir(string(repoPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &moduleSourceNotFoundError{path: new(repoPath)}
		}
		return nil, fmt.Errorf("read source repository %s: %w", repoPath, err)
	}

	candidates := make([]SubdirectoryPath, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := invowkmod.ParseModuleName(entry.Name()); err == nil {
			candidates = append(candidates, SubdirectoryPath(entry.Name())) //goplint:ignore -- directory name accepted by ParseModuleName.
		}
	}
	slices.SortFunc(candidates, func(a, b SubdirectoryPath) int {
		return strings.Compare(a.String(), b.String())
	})
	return candidates, nil
}

func checkCanonicalModuleCollision(seen map[ModuleID]*ResolvedModule, incoming *ResolvedModule) error {
	if incoming == nil || incoming.ModuleID == "" {
		return nil
	}
	existing, ok := seen[incoming.ModuleID]
	if !ok {
		seen[incoming.ModuleID] = incoming
		return nil
	}
	if existing.ModuleRef.Key() == incoming.ModuleRef.Key() {
		return nil
	}
	dirName, err := invowkmod.CanonicalModuleDirectoryName(incoming.ModuleID)
	if err != nil {
		return err
	}
	return &CanonicalModuleCollisionError{
		ModuleID:      incoming.ModuleID,
		DirectoryName: dirName,
		ExistingKey:   existing.ModuleRef.Key(),
		IncomingKey:   incoming.ModuleRef.Key(),
	}
}

func formatFilesystemPath(path *types.FilesystemPath) types.FilesystemPath {
	if path == nil {
		return ""
	}
	return *path
}

func formatSubdirectoryPath(path *SubdirectoryPath) SubdirectoryPath {
	if path == nil {
		return ""
	}
	return *path
}
