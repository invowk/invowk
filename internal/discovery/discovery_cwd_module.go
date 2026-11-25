// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// symlinkModuleSkippedFormat is the module_symlink_skipped message for a
	// symlinked module directory, found by a parent scan or as the current
	// directory.
	symlinkModuleSkippedFormat = "skipping symlinked module directory %s — symlink modules are not supported"

	// maxLinkHops bounds the chain of links linkChainTargets follows from the
	// current directory's own entry, as filepath.EvalSymlinks bounds its walk.
	maxLinkHops = 255
)

// discoverCurrentDirModule decides whether the current directory is a module
// directory and, when it is, loads it as that module.
//
// The invowkfile of a module directory is always loaded as part of that module,
// whichever directory invowk runs from. The decision is made on the current
// directory's own entry, as the directory its parent stores it in:
//   - a module-directory candidate is a directory any of whose names ends in
//     ".invowkmod", compared ignoring case: the logical base name, the name the
//     parent directory stores, the physical (EvalSymlinks) base name and, for a
//     link entry, the base name of every target along its chain of links;
//   - a link entry (a symbolic link, or a junction or other name-surrogate
//     reparse point, which Windows reports as ModeIrregular) that is a candidate
//     loads nothing and is reported as module_symlink_skipped, as the scan of
//     its parent directory treats it;
//   - a candidate directory is classified by loadModuleEntry with its stored
//     entry, exactly as the scan of its parent directory would classify it;
//   - every case that cannot be judged, and every candidate that does not load,
//     contributes nothing and is reported.
//
// It returns isModuleDir=false only when the current directory is an ordinary
// directory, which the caller then reads through discoverInDir and the scan of
// its children. For isModuleDir=true the caller reads nothing else from the
// current directory: no plain invowkfile parse and no child-module scan.
// Only the current directory's own entry counts; a link in an ancestor directory
// does not change the result, because os.Lstat does not follow the last path
// component.
func discoverCurrentDirModule(baseDir types.FilesystemPath) (isModuleDir bool, files []*DiscoveredFile, diagnostics []Diagnostic) {
	absPath, err := absolutePath(baseDir)
	if err != nil {
		if hasModuleSuffixFold(baseDir) {
			return true, nil, []Diagnostic{currentDirSkipped(baseDir, fmt.Errorf("cannot resolve the directory: %w", err))}
		}
		// discoverInDir and the child scan report or skip an unresolvable path.
		return false, nil, nil
	}
	names := []types.FilesystemPath{absPath}
	physical, evalErr := resolvedPath(absPath)
	if evalErr == nil {
		names = append(names, physical)
	}

	info, err := os.Lstat(string(absPath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !anyModuleSuffixFold(names) {
			// Nothing exists to read; the ordinary path finds nothing either.
			return false, nil, nil
		}
		return true, nil, []Diagnostic{currentDirSkipped(absPath, fmt.Errorf("cannot read the directory entry: %w", err))}
	}

	parentDir, parentErr := parentPath(absPath)
	volumeRoot := parentErr == nil && filepath.Clean(string(parentDir)) == filepath.Clean(string(absPath))
	var stored os.DirEntry
	hasStored := false
	if parentErr == nil && !volumeRoot {
		stored, hasStored = storedDirEntry(parentDir, absPath, info)
	}
	if hasStored {
		storedPath, pathErr := validatedDiscoveredEntryPath(parentDir, stored)
		if pathErr == nil {
			names = append(names, storedPath)
		}
	}

	if isLinkEntry(info) {
		targets, resolveErr := linkTargets(absPath)
		if resolveErr != nil {
			return true, nil, []Diagnostic{currentDirSkipped(absPath, fmt.Errorf("cannot resolve the link target: %w", resolveErr))}
		}
		if !anyModuleSuffixFold(append(names, targets...)) {
			// A link to an ordinary directory stays an ordinary directory.
			return false, nil, nil
		}
		return true, nil, []Diagnostic{mustDiagnosticWithPath(
			SeverityWarning,
			CodeModuleSymlinkSkipped,
			fmt.Sprintf(symlinkModuleSkippedFormat, absPath),
			absPath,
		)}
	}

	candidate := anyModuleSuffixFold(names)
	if !hasStored {
		return unstoredEntryVerdict(absPath, candidate, volumeRoot, evalErr)
	}
	if !candidate {
		return false, nil, nil
	}

	file, entryDiags := loadModuleEntry(parentDir, stored)
	if file == nil && len(entryDiags) == 0 {
		// A stored suffix in another letter case, an invalid module name, or an
		// entry that is not a directory: the parent scan loads nothing from it,
		// and from inside it is reported instead of being read as plain files.
		entryDiags = []Diagnostic{currentDirSkipped(absPath, errors.New("its name is not a loadable module directory name"))}
	}
	if file != nil {
		files = []*DiscoveredFile{file}
	}
	return true, files, entryDiags
}

// unstoredEntryVerdict judges a current directory whose stored name is unknown:
// a volume root, or a directory whose parent cannot be listed or does not list
// it.
//   - A candidate cannot be classified as the parent scan would classify it, so
//     it loads nothing.
//   - A volume root has no stored name by nature, and none of its names can
//     carry the suffix, so a volume root that is not a candidate is an ordinary
//     directory.
//   - Any other directory that is not a candidate is an ordinary directory only
//     when its physical name is known (evalErr is nil); with neither the stored
//     nor the physical name, a name that ends in ".invowkmod" may be hidden (a
//     Windows short name), so it loads nothing.
func unstoredEntryVerdict(absPath types.FilesystemPath, candidate, volumeRoot bool, evalErr error) (isModuleDir bool, files []*DiscoveredFile, diagnostics []Diagnostic) {
	if candidate {
		return true, nil, []Diagnostic{currentDirSkipped(absPath, errors.New("cannot find its entry in the parent directory"))}
	}
	if evalErr != nil && !volumeRoot {
		return true, nil, []Diagnostic{currentDirSkipped(absPath, fmt.Errorf("cannot find its entry in the parent directory or resolve it: %w", evalErr))}
	}
	return false, nil, nil
}

// storedDirEntry returns the entry parentDir stores for absPath: the one that
// is the same file as info. Entries whose names equal the base name ignoring
// case are compared first; every other entry is compared only when none of
// those match (Windows short names, Unicode normalisation on macOS). The caller
// never passes a volume root, which has no stored entry.
func storedDirEntry(parentDir, absPath types.FilesystemPath, info os.FileInfo) (os.DirEntry, bool) {
	entries, err := os.ReadDir(string(parentDir))
	if err != nil {
		return nil, false
	}
	base := filepath.Base(string(absPath))
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), base) && sameEntry(parentDir, entry, info) {
			return entry, true
		}
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), base) && sameEntry(parentDir, entry, info) {
			return entry, true
		}
	}
	return nil, false
}

func sameEntry(parentDir types.FilesystemPath, entry os.DirEntry, info os.FileInfo) bool {
	entryInfo, err := os.Lstat(filepath.Join(string(parentDir), entry.Name()))
	if err != nil {
		return false
	}
	return os.SameFile(entryInfo, info)
}

// isLinkEntry reports whether an os.Lstat result is a link rather than a
// directory: a symbolic link, or (on Windows) a junction or other
// name-surrogate reparse point, which has ModeIrregular and no ModeDir.
func isLinkEntry(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	return info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// linkTargets returns a link's targets: every target reached by following
// os.Readlink (it reads junction targets too) from the link, each also
// resolved, and the path filepath.EvalSymlinks resolves for the link. It fails
// when neither os.Readlink nor filepath.EvalSymlinks resolves the link, and
// when a link further along the chain cannot be followed.
func linkTargets(link types.FilesystemPath) ([]types.FilesystemPath, error) {
	var targets []types.FilesystemPath
	target, readErr := readlinkPath(link)
	if readErr == nil {
		chain, err := linkChainTargets(target)
		if err != nil {
			return nil, err
		}
		targets = append(targets, chain...)
	}
	resolved, evalErr := resolvedPath(link)
	if evalErr == nil {
		targets = append(targets, resolved)
	}
	if readErr != nil && evalErr != nil {
		return nil, errors.Join(readErr, evalErr)
	}
	return targets, nil
}

// linkChainTargets returns target and every later target of the chain of links
// that starts at it, and the filepath.EvalSymlinks result of the chain's end
// when that resolves. filepath.EvalSymlinks does not follow Windows junctions,
// so a chain that runs through junctions is followed here, one os.Readlink at a
// time. The chain ends at the first target that is not a link or does not
// exist. A target whose entry cannot be read, a link that cannot be read, and a
// chain longer than maxLinkHops are errors, since the end of the chain is then
// unknown.
func linkChainTargets(target types.FilesystemPath) ([]types.FilesystemPath, error) {
	var targets []types.FilesystemPath
	for range maxLinkHops {
		targets = append(targets, target)
		info, err := os.Lstat(string(target))
		if errors.Is(err, fs.ErrNotExist) {
			return targets, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read link target entry: %w", err)
		}
		if !isLinkEntry(info) {
			if resolved, evalErr := resolvedPath(target); evalErr == nil {
				targets = append(targets, resolved)
			}
			return targets, nil
		}
		next, err := readlinkPath(target)
		if err != nil {
			return nil, err
		}
		target = next
	}
	return nil, fmt.Errorf("more than %d links in a chain", maxLinkHops)
}

func absolutePath(path types.FilesystemPath) (types.FilesystemPath, error) {
	abs, err := filepath.Abs(string(path))
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	absPath := types.FilesystemPath(abs)
	if err := absPath.Validate(); err != nil {
		return "", err
	}
	return absPath, nil
}

func resolvedPath(path types.FilesystemPath) (types.FilesystemPath, error) {
	resolved, err := filepath.EvalSymlinks(string(path))
	if err != nil {
		return "", fmt.Errorf("resolve links: %w", err)
	}
	resolvedPath := types.FilesystemPath(resolved)
	if err := resolvedPath.Validate(); err != nil {
		return "", err
	}
	return resolvedPath, nil
}

func readlinkPath(link types.FilesystemPath) (types.FilesystemPath, error) {
	target, err := os.Readlink(string(link))
	if err != nil {
		return "", fmt.Errorf("read link: %w", err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(string(link)), target)
	}
	targetPath := types.FilesystemPath(target)
	if err := targetPath.Validate(); err != nil {
		return "", err
	}
	return targetPath, nil
}

func parentPath(path types.FilesystemPath) (types.FilesystemPath, error) {
	parent := filepath.Dir(string(path))
	parentDir := types.FilesystemPath(parent)
	if err := parentDir.Validate(); err != nil {
		return "", err
	}
	return parentDir, nil
}

// hasModuleSuffixFold reports whether the base name of path ends in
// ".invowkmod", compared ignoring case.
func hasModuleSuffixFold(path types.FilesystemPath) bool {
	return strings.HasSuffix(strings.ToLower(filepath.Base(string(path))), invowkmod.ModuleSuffix)
}

func anyModuleSuffixFold(paths []types.FilesystemPath) bool {
	return slices.ContainsFunc(paths, hasModuleSuffixFold)
}

// currentDirSkipped reports a current directory that loads nothing because it
// cannot be judged or is not a loadable module directory.
func currentDirSkipped(path types.FilesystemPath, reason error) Diagnostic {
	return mustDiagnosticWithCause(
		SeverityWarning,
		CodeModuleLoadSkipped,
		fmt.Sprintf("skipping current directory %s as a module directory: %v", path, reason),
		path,
		reason,
	)
}
