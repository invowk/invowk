// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const rollbackToolsURL GitURL = "https://github.com/user/tools.git"

// newRollbackResolver returns a resolver on workDir whose fetcher serves one
// io.example.tools module, with a temporary cache root.
func newRollbackResolver(t *testing.T, workDir string) *Resolver {
	t.Helper()
	repoDir := t.TempDir()
	writeSourceModule(t, repoDir, "io.example.tools")
	resolver, err := newResolverWithFetcher(
		types.FilesystemPath(workDir),
		types.FilesystemPath(t.TempDir()),
		&fakeModuleFetcher{repoPath: types.FilesystemPath(repoDir), listVersions: []SemVer{"1.2.3"}},
	)
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	return withEditLocks(resolver)
}

// writeDuplicateToolsInvowkmod writes an invowkmod.cue that already requires
// the tools module, so an add of it fails its edit as a duplicate and rolls
// back.
func writeDuplicateToolsInvowkmod(t *testing.T, path string) {
	t.Helper()
	content := "module: \"io.example.root\"\nversion: \"1.0.0\"\nrequires: [\n\t{\n\t\tgit_url: \"" + string(rollbackToolsURL) + "\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func assertSameLink(t *testing.T, path, wantTarget string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%s) error = %v, want the link to survive the rollback", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s mode = %v, want a symbolic link", path, info.Mode())
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("Readlink(%s) error = %v", path, err)
	}
	if target != wantTarget {
		t.Fatalf("%s target = %q, want %q", path, target, wantTarget)
	}
}

// TestAddModuleDependencyRollbackKeepsDanglingInvowkmodLink checks that the
// rollback snapshot does not follow links: a dangling invowkmod.cue link reads
// as a link, not as a missing file, so the rollback leaves it in place instead
// of deleting it (spec "Dangling invowkmod.cue link").
func TestAddModuleDependencyRollbackKeepsDanglingInvowkmodLink(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	workDir := t.TempDir()
	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	const target = "missing-invowkmod.cue"
	fstree.MustSymlink(t, target, invowkmodPath)

	result, err := newRollbackResolver(t, workDir).AddModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), ModuleRef{GitURL: rollbackToolsURL, Version: "^1.0.0"})
	if err != nil {
		t.Fatalf("AddModuleDependency() error = %v", err)
	}
	if !errors.Is(result.Declaration().Err(), fs.ErrNotExist) {
		t.Fatalf("Declaration.Err = %v, want an error wrapping fs.ErrNotExist", result.Declaration().Err())
	}

	assertSameLink(t, invowkmodPath, target)
	if _, statErr := os.Lstat(filepath.Join(workDir, LockFileName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("lock file Lstat error = %v, want not exist after the rollback", statErr)
	}
}

// TestAddModuleDependencyRollbackRestoresLinkedLock checks that a lock path
// that is a link to a regular file is restored as that link: the add's save
// renames a regular file over the link, and the rollback puts the link back
// with its target untouched (spec "Linked lock file").
func TestAddModuleDependencyRollbackRestoresLinkedLock(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	workDir := t.TempDir()
	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	writeDuplicateToolsInvowkmod(t, invowkmodPath)
	realLock := filepath.Join(workDir, "real-lock.cue")
	if err := invowkmod.NewLockFile().Save(realLock); err != nil {
		t.Fatalf("Save(real lock) error = %v", err)
	}
	before, err := os.ReadFile(realLock)
	if err != nil {
		t.Fatalf("ReadFile(real lock) error = %v", err)
	}
	lockPath := filepath.Join(workDir, LockFileName)
	fstree.MustSymlink(t, "real-lock.cue", lockPath)

	result, err := newRollbackResolver(t, workDir).AddModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), ModuleRef{GitURL: rollbackToolsURL, Version: "^1.0.0"})
	if err != nil {
		t.Fatalf("AddModuleDependency() error = %v", err)
	}
	if !result.LockWritten() {
		t.Fatal("LockWritten = false, want the save to have replaced the link before the rollback")
	}
	if !errors.Is(result.Declaration().Err(), invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("Declaration.Err = %v, want ErrModuleAlreadyExists", result.Declaration().Err())
	}

	assertSameLink(t, lockPath, "real-lock.cue")
	after, err := os.ReadFile(realLock)
	if err != nil {
		t.Fatalf("ReadFile(real lock) after rollback error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("link target changed:\n got: %s\nwant: %s", after, before)
	}
}

// TestAddModuleDependencyRollbackLinkRestoreFails checks that a link that
// cannot be recreated is a restore error, never a silent skip (spec "Restore of
// a link fails"): the lock path is a link, the add's save replaces it, and the
// injected link creator fails.
func TestAddModuleDependencyRollbackLinkRestoreFails(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	workDir := t.TempDir()
	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	writeDuplicateToolsInvowkmod(t, invowkmodPath)
	if err := invowkmod.NewLockFile().Save(filepath.Join(workDir, "real-lock.cue")); err != nil {
		t.Fatalf("Save(real lock) error = %v", err)
	}
	fstree.MustSymlink(t, "real-lock.cue", filepath.Join(workDir, LockFileName))

	injected := errors.New("injected: symbolic links unavailable")
	resolver := newRollbackResolver(t, workDir)
	resolver.restoreLinker = func(string, string) error { return injected }

	result, err := resolver.AddModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), ModuleRef{GitURL: rollbackToolsURL, Version: "^1.0.0"})
	if err != nil {
		t.Fatalf("AddModuleDependency() error = %v", err)
	}
	if !result.LockWritten() {
		t.Fatal("LockWritten = false, want the save to have replaced the link")
	}
	declErr := result.Declaration().Err()
	if !errors.Is(declErr, invowkmod.ErrModuleAlreadyExists) || !errors.Is(declErr, injected) {
		t.Fatalf("Declaration.Err = %v, want the duplicate edit error and the injected restore error", declErr)
	}
	if got := result.Declaration().Rollback(); got != DeclarationRollbackIncomplete {
		t.Fatalf("Declaration.Rollback = %s, want incomplete", got)
	}
}

// TestAddModuleDependencyRollbackLockRestoreFails forces a real lock-restore
// failure through the restore-writer seam: the outcome is incomplete and Err
// carries the restore error (spec "Rollback outcome is reported by the
// operation").
func TestAddModuleDependencyRollbackLockRestoreFails(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	writeDuplicateToolsInvowkmod(t, invowkmodPath)
	lockPath := filepath.Join(workDir, LockFileName)
	if err := invowkmod.NewLockFile().Save(lockPath); err != nil {
		t.Fatalf("Save(lock) error = %v", err)
	}

	injected := errors.New("injected: disk full")
	resolver := newRollbackResolver(t, workDir)
	resolver.restoreWriter = func(path string, data []byte, perm os.FileMode) error {
		if path == lockPath {
			return injected
		}
		return os.WriteFile(path, data, perm)
	}

	result, err := resolver.AddModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), ModuleRef{GitURL: rollbackToolsURL, Version: "^1.0.0"})
	if err != nil {
		t.Fatalf("AddModuleDependency() error = %v", err)
	}
	declaration := result.Declaration()
	if got := declaration.Rollback(); got != DeclarationRollbackIncomplete {
		t.Fatalf("Declaration.Rollback = %s, want incomplete", got)
	}
	if !errors.Is(declaration.Err(), injected) || !errors.Is(declaration.Err(), invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("Declaration.Err = %v, want the edit error and the injected restore error", declaration.Err())
	}
}

func TestDeclarationEditResultValidate(t *testing.T) {
	t.Parallel()

	editErr := errors.New("edit failed")
	tests := []struct {
		name     string
		err      error
		rollback DeclarationRollback
		wantErr  bool
	}{
		{name: "success", rollback: DeclarationRollbackNone},
		{name: "complete rollback", err: editErr, rollback: DeclarationRollbackComplete},
		{name: "incomplete rollback", err: editErr, rollback: DeclarationRollbackIncomplete},
		{name: "error without rollback", err: editErr, rollback: DeclarationRollbackNone, wantErr: true},
		{name: "rollback without error", rollback: DeclarationRollbackComplete, wantErr: true},
		{name: "unknown outcome", err: editErr, rollback: "partial", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewDeclarationEditResult(false, tt.err, tt.rollback)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewDeclarationEditResult() error = %v, want error %t", err, tt.wantErr)
			}
		})
	}
}

func seedMultiSubpathMonorepoLock(t *testing.T, workDir string, gitURL GitURL, subPaths ...SubdirectoryPath) []byte {
	t.Helper()

	lock := invowkmod.NewLockFile()
	for _, subPath := range subPaths {
		lock.AddModule(&ResolvedModule{
			ModuleRef: ModuleRef{
				GitURL:  gitURL,
				Version: "^1.0.0",
				Path:    subPath,
			},
			ResolvedVersion: "1.0.0",
			GitCommit:       defaultFakeCommit,
			ModuleID:        ModuleID("io.example." + filepath.Base(string(subPath))),
			Namespace:       ModuleNamespace("io.example." + filepath.Base(string(subPath)) + "@1.0.0"),
			CommandSourceID: invowkmod.ModuleSourceID("io.example." + filepath.Base(string(subPath))),
			ContentHash:     "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		})
	}
	lockPath := filepath.Join(workDir, LockFileName)
	if err := lock.Save(lockPath); err != nil {
		t.Fatalf("Save(lock) error = %v", err)
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	return data
}

func TestRemoveModuleDependencyRollbackRestoresSymlinkedInvowkmodTarget(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	const monorepoURL GitURL = "https://github.com/user/monorepo.git"
	// Step 1 removes one entry while requires[0] remains valid; step 2 removes
	// the entire requires field, which breaks _firstDep and triggers rollback.
	const initialManifest = `module: "io.example.root"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path:    "packages/a"
	},
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path:    "packages/b"
	},
]

_firstDep: requires[0].git_url
`

	workDir := t.TempDir()
	sharedDir := t.TempDir()
	realManifest := filepath.Join(sharedDir, "shared-invowkmod.cue")
	if err := os.WriteFile(realManifest, []byte(initialManifest), 0o600); err != nil {
		t.Fatalf("WriteFile(realManifest) error = %v", err)
	}
	resolvedRealManifest, err := filepath.EvalSymlinks(realManifest)
	if err != nil {
		t.Fatalf("EvalSymlinks(realManifest) error = %v", err)
	}
	beforeInfo := statFileIdentity(t, resolvedRealManifest)

	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	fstree.MustSymlink(t, realManifest, invowkmodPath)
	lockBefore := seedMultiSubpathMonorepoLock(t, workDir, monorepoURL, "packages/a", "packages/b")

	resolver, err := newLockOnlyResolver(types.FilesystemPath(workDir), WithEditLocks(EditLocks{
		Locker:   modulelock.NewFileLocker(),
		CacheDir: types.FilesystemPath(t.TempDir()),
	}))
	if err != nil {
		t.Fatalf("newLockOnlyResolver() error = %v", err)
	}

	result, err := resolver.RemoveModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), string(monorepoURL))
	if err != nil {
		t.Fatalf("RemoveModuleDependency() error = %v", err)
	}
	if !result.LockWritten() {
		t.Fatal("LockWritten() = false, want true")
	}
	decls := result.Declarations()
	if len(decls) != 2 {
		t.Fatalf("Declarations() = %d entries, want 2 (step 1 edit + step 2 rollback)", len(decls))
	}
	if !decls[0].Updated() || decls[0].Err() != nil {
		t.Fatalf("Declarations()[0] = %+v, want step 1 edit succeeded before step 2 failure", decls[0])
	}
	if decls[1].Updated() {
		t.Fatal("Declarations()[1].Updated() = true, want false after rollback")
	}
	if decls[1].Rollback() != DeclarationRollbackComplete {
		t.Fatalf("Declarations()[1].Rollback() = %s, want complete", decls[1].Rollback())
	}
	if decls[1].Err() == nil {
		t.Fatal("Declarations()[1].Err() = nil, want CUE edit error")
	}

	assertSameLink(t, invowkmodPath, realManifest)
	afterData, err := os.ReadFile(resolvedRealManifest)
	if err != nil {
		t.Fatalf("ReadFile(resolvedRealManifest) error = %v", err)
	}
	if string(afterData) != initialManifest {
		t.Fatalf("resolved target content mismatch after rollback:\ngot:\n%s\nwant:\n%s", afterData, initialManifest)
	}
	afterInfo := statFileIdentity(t, resolvedRealManifest)
	if os.SameFile(beforeInfo, afterInfo) {
		t.Fatal("resolved target kept same file identity, want atomic replace during rollback")
	}
	if runtime.GOOS != "windows" && afterInfo.Mode().Perm() != 0o600 {
		t.Fatalf("resolved target perm = %v, want 0600", afterInfo.Mode().Perm())
	}
	lockAfter, err := os.ReadFile(filepath.Join(workDir, LockFileName))
	if err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	if !bytes.Equal(lockAfter, lockBefore) {
		t.Fatalf("lock file mismatch after rollback:\ngot:\n%s\nwant:\n%s", lockAfter, lockBefore)
	}

	t.Run("target restore failure yields incomplete rollback", func(t *testing.T) {
		t.Parallel()

		subWorkDir := t.TempDir()
		subSharedDir := t.TempDir()
		subReal := filepath.Join(subSharedDir, "shared-invowkmod.cue")
		if writeErr := os.WriteFile(subReal, []byte(initialManifest), 0o600); writeErr != nil {
			t.Fatalf("WriteFile(subReal) error = %v", writeErr)
		}
		resolvedSubReal, evalErr := filepath.EvalSymlinks(subReal)
		if evalErr != nil {
			t.Fatalf("EvalSymlinks(subReal) error = %v", evalErr)
		}
		subLink := filepath.Join(subWorkDir, "invowkmod.cue")
		fstree.MustSymlink(t, subReal, subLink)
		seedMultiSubpathMonorepoLock(t, subWorkDir, monorepoURL, "packages/a", "packages/b")

		injected := errors.New("injected: target restore failed")
		subResolver, resErr := newLockOnlyResolver(types.FilesystemPath(subWorkDir), WithEditLocks(EditLocks{
			Locker:   modulelock.NewFileLocker(),
			CacheDir: types.FilesystemPath(t.TempDir()),
		}))
		if resErr != nil {
			t.Fatalf("newLockOnlyResolver() error = %v", resErr)
		}
		subResolver.restoreWriter = func(path string, data []byte, perm os.FileMode) error {
			if path == resolvedSubReal {
				return injected
			}
			return os.WriteFile(path, data, perm)
		}

		subResult, rmErr := subResolver.RemoveModuleDependency(t.Context(), types.FilesystemPath(subLink), string(monorepoURL))
		if rmErr != nil {
			t.Fatalf("RemoveModuleDependency() error = %v", rmErr)
		}
		subDecls := subResult.Declarations()
		if len(subDecls) != 2 || subDecls[1].Rollback() != DeclarationRollbackIncomplete || !errors.Is(subDecls[1].Err(), injected) {
			t.Fatalf("Declarations() = %+v, want step 2 incomplete rollback carrying injected error", subDecls)
		}
		assertSameLink(t, subLink, subReal)
	})
}

func TestFileSnapshotSymlinkTargetValidationAndDirectoryLink(t *testing.T) {
	t.Parallel()

	p := types.FilesystemPath(filepath.Join(t.TempDir(), "invowkmod.cue"))
	target := types.FilesystemPath(filepath.Join(t.TempDir(), "target.cue"))
	badSnapshot := fileSnapshot{
		path:       &p,
		kind:       snapshotRegular,
		targetPath: &target,
	}
	if err := badSnapshot.Validate(); err == nil {
		t.Fatal("Validate() = nil for non-symlink snapshot with targetPath, want error")
	}

	fstree.Require(t, fstree.CapSymlink)
	root := t.TempDir()
	dirTarget := filepath.Join(root, "dir")
	if err := os.Mkdir(dirTarget, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	dirLink := filepath.Join(root, "invowkmod.cue")
	fstree.MustSymlink(t, dirTarget, dirLink)

	snap, err := readFileSnapshot(types.FilesystemPath(dirLink))
	if err != nil {
		t.Fatalf("readFileSnapshot(dirLink) error = %v", err)
	}
	if snap.kind != snapshotSymlink || snap.targetPath != nil {
		t.Fatalf("snapshot = %+v, want kind=symlink with nil targetPath", snap)
	}
	writerCalled := false
	snap.writeFile = func(string, []byte, os.FileMode) error {
		writerCalled = true
		return nil
	}
	if err := snap.restore(); err != nil {
		t.Fatalf("restore() error = %v", err)
	}
	if writerCalled {
		t.Fatal("restore() called fileWriter for directory symlink")
	}
	assertSameLink(t, dirLink, dirTarget)
}
