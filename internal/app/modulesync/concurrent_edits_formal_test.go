// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/invowk/invowk/internal/testutil/procgate"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// Characterisation tests for formal/tla/ConcurrentModuleEdits.tla. Each test
// replays one counterexample against the real Resolver methods, with one
// Resolver, one fetcher, and one gate per simulated process; the processes
// synchronise only through channels. Each asserts today's behaviour (an open
// finding) and must be inverted by the change that fixes it.

const (
	concurrentURLA GitURL = "https://github.com/user/a.git"
	concurrentURLB GitURL = "https://github.com/user/b.git"
	concurrentURLR GitURL = "https://github.com/user/tools.git"
)

type (
	// gatedListFetcher parks its process when resolveOne lists versions, which
	// is the start of the fetch window (resolver_deps.go:114).
	gatedListFetcher struct {
		moduleFetcher
		gate *procgate.Gate
	}

	// worktreeGateFetcher serves a fixed version list (the real ListVersions
	// needs the network) and delegates Fetch to the real GitFetcher, then
	// parks its process when a gate is set: the shared worktree is checked out
	// but not yet copied into the cache.
	worktreeGateFetcher struct {
		inner    *GitFetcher
		versions []SemVer
		gate     *procgate.Gate
	}

	// concurrentProject is one invowkmod.cue and lock file with dependency a
	// added, plus module sources for a and b.
	concurrentProject struct {
		workDir       string
		cacheDir      string
		invowkmodPath types.FilesystemPath
		sources       map[GitURL]string
	}
)

func (f gatedListFetcher) ListVersions(ctx context.Context, gitURL GitURL) ([]SemVer, error) {
	f.gate.Park()
	return f.moduleFetcher.ListVersions(ctx, gitURL)
}

func (f *worktreeGateFetcher) ListVersions(context.Context, GitURL) ([]SemVer, error) {
	return f.versions, nil
}

func (f *worktreeGateFetcher) Fetch(ctx context.Context, gitURL GitURL, version SemVer) (types.FilesystemPath, GitCommit, error) {
	repoPath, commit, err := f.inner.Fetch(ctx, gitURL, version)
	if err == nil && f.gate != nil {
		f.gate.Park()
	}
	return repoPath, commit, err
}

func newConcurrentProject(t *testing.T) *concurrentProject {
	t.Helper()
	p := &concurrentProject{workDir: t.TempDir(), cacheDir: t.TempDir(), sources: map[GitURL]string{}}
	p.invowkmodPath = types.FilesystemPath(filepath.Join(p.workDir, "invowkmod.cue"))
	if err := os.WriteFile(string(p.invowkmodPath), []byte("module: \"io.example.root\"\nversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(root invowkmod.cue) error = %v", err)
	}
	for url, id := range map[GitURL]ModuleID{concurrentURLA: "io.example.a", concurrentURLB: "io.example.b"} {
		dir := t.TempDir()
		writeSourceModule(t, dir, id)
		p.sources[url] = dir
	}
	// Sequential setup through the real code: requires = {a}, lock = {a}.
	p.mustAdd(t, p.resolver(t, p.fetcher(concurrentURLA)), concurrentURLA)
	return p
}

// fetcher returns a fresh fake for one process; fakes are never shared.
func (p *concurrentProject) fetcher(url GitURL) *fakeModuleFetcher {
	return &fakeModuleFetcher{repoPath: types.FilesystemPath(p.sources[url]), listVersions: []SemVer{"1.0.0"}}
}

func (p *concurrentProject) resolver(t *testing.T, fetcher moduleFetcher) *Resolver {
	t.Helper()
	r, err := newResolverWithFetcher(types.FilesystemPath(p.workDir), types.FilesystemPath(p.cacheDir), fetcher)
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	return r
}

func (p *concurrentProject) mustAdd(t *testing.T, r *Resolver, url GitURL) {
	t.Helper()
	result, err := r.AddModuleDependency(t.Context(), p.invowkmodPath, ModuleRef{GitURL: url, Version: "^1.0.0"})
	if err != nil || result.Declaration().Err() != nil {
		t.Fatalf("AddModuleDependency(%s) error = %v, declaration error = %v", url, err, result.Declaration().Err())
	}
}

func (p *concurrentProject) requiresKeys(t *testing.T) []ModuleRefKey {
	t.Helper()
	reqs, err := LoadRequirements(p.invowkmodPath)
	if err != nil {
		t.Fatalf("LoadRequirements() error = %v", err)
	}
	keys := make([]ModuleRefKey, 0, len(reqs))
	for _, req := range reqs {
		keys = append(keys, req.Key())
	}
	slices.Sort(keys)
	return keys
}

func lockKeys(t *testing.T, workDir string) []ModuleRefKey {
	t.Helper()
	lock := loadLockFileForTest(t, filepath.Join(workDir, LockFileName))
	keys := make([]ModuleRefKey, 0, len(lock.Modules))
	for key := range lock.Modules {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func assertKeys(t *testing.T, what string, got []ModuleRefKey, want ...GitURL) {
	t.Helper()
	wantKeys := make([]ModuleRefKey, 0, len(want))
	for _, url := range want {
		wantKeys = append(wantKeys, ModuleRefKey(url))
	}
	if !slices.Equal(got, wantKeys) {
		t.Fatalf("%s keys = %v, want %v", what, got, wantKeys)
	}
}

// raceFetchWindow runs p1 on its own Resolver, parks it when it lists
// versions (the start of its fetch window), runs p2 to completion, then lets
// p1 finish and returns p1's error.
func (p *concurrentProject) raceFetchWindow(t *testing.T, url GitURL, p1 func(*Resolver) error, p2 func()) error {
	t.Helper()
	gate := procgate.New()
	r := p.resolver(t, gatedListFetcher{moduleFetcher: p.fetcher(url), gate: gate})
	done := procgate.Run(func() (struct{}, error) { return struct{}{}, p1(r) })
	procgate.AwaitParked(t, gate, done)
	p2()
	gate.Open()
	return (<-done).Err
}

// TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd replays
// ConcurrentModuleEdits.findingF8SyncOverwritesConcurrentAdd (open finding
// F8): sync rebuilds the lock from the requires it read before its fetch, so a
// module added meanwhile is in invowkmod.cue but not in the lock, and both
// commands succeed. P1 transcribes SyncModule's body (LoadRequirements, then
// Resolver.Sync). Asserts current behaviour; invert it with the fix.
func TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	reqs, err := LoadRequirements(p.invowkmodPath)
	if err != nil {
		t.Fatalf("LoadRequirements() error = %v", err)
	}
	err = p.raceFetchWindow(t, concurrentURLA,
		func(r *Resolver) error { _, syncErr := r.Sync(t.Context(), reqs); return syncErr },
		func() { p.mustAdd(t, p.resolver(t, p.fetcher(concurrentURLB)), concurrentURLB) })
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertKeys(t, "requires", p.requiresKeys(t), concurrentURLA, concurrentURLB)
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA)
}

func updateAll(t *testing.T) func(*Resolver) error {
	t.Helper()
	return func(r *Resolver) error { _, err := r.Update(t.Context(), ""); return err }
}

// TestConcurrentEdits_FindingF8_UpdateOverwritesConcurrentAdd replays
// ConcurrentModuleEdits.findingF8UpdateOverwritesConcurrentAdd (open finding
// F8): update saves the lock it read before fetching, dropping a concurrent
// add from the lock. Asserts current behaviour; invert it with the fix.
func TestConcurrentEdits_FindingF8_UpdateOverwritesConcurrentAdd(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	err := p.raceFetchWindow(t, concurrentURLA, updateAll(t),
		func() { p.mustAdd(t, p.resolver(t, p.fetcher(concurrentURLB)), concurrentURLB) })
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	assertKeys(t, "requires", p.requiresKeys(t), concurrentURLA, concurrentURLB)
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA)
}

// TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove replays
// ConcurrentModuleEdits.findingF8UpdateResurrectsConcurrentRemove (open
// finding F8): a module removed while update fetches comes back into the lock.
// Asserts current behaviour; invert it with the fix.
func TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	err := p.raceFetchWindow(t, concurrentURLA, updateAll(t), func() {
		removed, removeErr := p.resolver(t, p.fetcher(concurrentURLA)).RemoveModuleDependency(t.Context(), p.invowkmodPath, string(concurrentURLA))
		if removeErr != nil || len(removed.Removed()) != 1 || removed.Declarations()[0].Err() != nil {
			t.Fatalf("RemoveModuleDependency() = %+v, error = %v", removed, removeErr)
		}
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	assertKeys(t, "requires", p.requiresKeys(t))
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA)
}

// TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd replays
// ConcurrentModuleEdits.findingF8DuplicateAddRollsBackOtherAdd (open finding
// F8): two adds of one module both snapshot the files before fetching; the
// later one's edit fails as a duplicate and restores its stale snapshots,
// erasing the add that already reported success. Asserts current behaviour;
// invert it with the fix.
func TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	var declErr error
	err := p.raceFetchWindow(t, concurrentURLB,
		func(r *Resolver) error {
			result, addErr := r.AddModuleDependency(t.Context(), p.invowkmodPath, ModuleRef{GitURL: concurrentURLB, Version: "^1.0.0"})
			declErr = result.Declaration().Err()
			return addErr
		},
		func() { p.mustAdd(t, p.resolver(t, p.fetcher(concurrentURLB)), concurrentURLB) })
	if err != nil {
		t.Fatalf("second AddModuleDependency() error = %v", err)
	}
	if !errors.Is(declErr, invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("second AddModuleDependency() declaration error = %v, want ErrModuleAlreadyExists", declErr)
	}

	assertKeys(t, "requires", p.requiresKeys(t), concurrentURLA)
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA)
}

// newTwoTagModuleRepo builds a local repository whose tags 1.0.0 and 2.0.0
// hold the same module ID with different content, and returns the commit of
// each tag.
func newTwoTagModuleRepo(t *testing.T) (dir string, commits map[SemVer]GitCommit) {
	t.Helper()
	dir = t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	commits = map[SemVer]GitCommit{}
	when := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, tag := range []SemVer{"1.0.0", "2.0.0"} {
		writeIntegrityModule(t, dir, "cmds: {} // content of "+string(tag))
		if _, err := w.Add("."); err != nil {
			t.Fatalf("git add: %v", err)
		}
		hash, err := w.Commit("release "+string(tag), &git.CommitOptions{
			Author: &object.Signature{Name: "Test", Email: "test@example.com", When: when.Add(time.Duration(i) * time.Hour)},
		})
		if err != nil {
			t.Fatalf("git commit: %v", err)
		}
		if _, err := repo.CreateTag(string(tag), hash, nil); err != nil {
			t.Fatalf("git tag %s: %v", tag, err)
		}
		commits[tag] = GitCommit(hash.String())
	}
	return dir, commits
}

// seedSourceClone clones repoDir into the GitFetcher source path of gitURL
// under cacheDir, so Fetch opens it and never contacts gitURL.
func seedSourceClone(t *testing.T, repoDir, cacheDir string, gitURL GitURL) {
	t.Helper()
	dest := NewGitFetcher(types.FilesystemPath(cacheDir)).getRepoCachePath(gitURL)
	if _, err := git.PlainClone(string(dest), false, &git.CloneOptions{URL: string(fileGitURL(repoDir))}); err != nil {
		t.Fatalf("seed clone: %v", err)
	}
}

func newWorktreeResolver(t *testing.T, cacheDir string, gate *procgate.Gate) *Resolver {
	t.Helper()
	fetcher := &worktreeGateFetcher{inner: NewGitFetcher(types.FilesystemPath(cacheDir)), versions: []SemVer{"2.0.0", "1.0.0"}, gate: gate}
	r, err := newResolverWithFetcher(types.FilesystemPath(t.TempDir()), types.FilesystemPath(cacheDir), fetcher)
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	return r
}

// TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent replays
// ConcurrentModuleEdits.findingF9SharedWorktreeRecordsWrongContent (open
// finding F9): two projects sharing one module cache add versions 1.0.0 and
// 2.0.0 of one repository. The Git source has one worktree per URL, so P1
// copies the worktree after P2 checked out 2.0.0, and P1's lock records the
// 1.0.0 commit with 2.0.0's content hash. Asserts current behaviour; invert
// it with the fix.
func TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent(t *testing.T) {
	t.Parallel()
	repoDir, commits := newTwoTagModuleRepo(t)

	// Reference: 1.0.0 alone, in its own cache.
	refCache := t.TempDir()
	seedSourceClone(t, repoDir, refCache, concurrentURLR)
	reference, err := newWorktreeResolver(t, refCache, nil).Add(t.Context(), ModuleRef{GitURL: concurrentURLR, Version: "^1.0.0"})
	if err != nil {
		t.Fatalf("reference Add() error = %v", err)
	}

	cacheDir := t.TempDir()
	seedSourceClone(t, repoDir, cacheDir, concurrentURLR)
	gate := procgate.New()
	p1 := newWorktreeResolver(t, cacheDir, gate)
	done := procgate.Run(func() (*ResolvedModule, error) {
		return p1.Add(t.Context(), ModuleRef{GitURL: concurrentURLR, Version: "^1.0.0"})
	})
	procgate.AwaitParked(t, gate, done)

	v2, err := newWorktreeResolver(t, cacheDir, nil).Add(t.Context(), ModuleRef{GitURL: concurrentURLR, Version: "^2.0.0"})
	if err != nil {
		t.Fatalf("P2 Add() error = %v", err)
	}
	gate.Open()
	o := <-done
	if o.Err != nil {
		t.Fatalf("P1 Add() error = %v", o.Err)
	}

	entry := loadLockFileForTest(t, filepath.Join(string(p1.WorkingDir()), LockFileName)).Modules[ModuleRefKey(concurrentURLR)]
	if entry.ResolvedVersion != "1.0.0" || entry.GitCommit != commits["1.0.0"] {
		t.Fatalf("P1 lock entry = %s@%s, want 1.0.0@%s", entry.ResolvedVersion, entry.GitCommit, commits["1.0.0"])
	}
	if entry.ContentHash == reference.ContentHash || entry.ContentHash != v2.ContentHash {
		t.Fatalf("P1 locked hash %s: want 2.0.0's %s instead of 1.0.0's %s", entry.ContentHash, v2.ContentHash, reference.ContentHash)
	}
}
