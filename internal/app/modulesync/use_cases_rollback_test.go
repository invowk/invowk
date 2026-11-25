// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

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
	return resolver
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
