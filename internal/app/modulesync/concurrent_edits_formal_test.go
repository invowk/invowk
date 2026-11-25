// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/internal/testutil/procgate"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// Replays of formal/tla/ConcurrentModuleEdits.tla against the real Resolver
// methods, with one Resolver, one fetcher, one modulelock.FileLocker and one
// gate per simulated process on one shared cache root; the processes
// synchronise only through channels, the lock wait callback and the OS lock
// on a real file. The F8, F9 and F10 replays assert the fixed behaviour and
// kill mutants of the lock and restore code.

const (
	concurrentURLA GitURL = "https://github.com/user/a.git"
	concurrentURLB GitURL = "https://github.com/user/b.git"
	concurrentURLR GitURL = "https://github.com/user/tools.git"

	// twoTagModulePath is the module's subdirectory in newTwoTagModuleRepo.
	twoTagModulePath SubdirectoryPath = "io.example.tools.invowkmod"
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

// resolver returns one process's Resolver: its own fetcher and its own
// FileLocker on the project's shared cache root. onWait, when set, is called
// when the process finds a lock busy.
func (p *concurrentProject) resolver(t *testing.T, fetcher moduleFetcher, onWait ...func(LockWait)) *Resolver {
	t.Helper()
	locks := EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(p.cacheDir)}
	if len(onWait) > 0 {
		locks.OnWait = onWait[0]
	}
	r, err := newResolverWithFetcher(types.FilesystemPath(p.workDir), types.FilesystemPath(p.cacheDir), fetcher, WithEditLocks(locks))
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	return r
}

func (p *concurrentProject) mustAdd(t *testing.T, r *Resolver, url GitURL) {
	t.Helper()
	if err := addOK(t.Context(), r, p.invowkmodPath, url); err != nil {
		t.Fatal(err)
	}
}

// addOK adds url with r and fails unless both the lock and the invowkmod.cue
// edit succeeded; it returns an error so a goroutine can report it.
func addOK(ctx context.Context, r *Resolver, invowkmodPath types.FilesystemPath, url GitURL) error {
	result, err := r.AddModuleDependency(ctx, invowkmodPath, ModuleRef{GitURL: url, Version: "^1.0.0"})
	if err != nil || result.Declaration().Err() != nil {
		return fmt.Errorf("AddModuleDependency(%s): %w", url, errors.Join(err, result.Declaration().Err()))
	}
	return nil
}

// files returns the bytes of invowkmod.cue and the lock file.
func (p *concurrentProject) files(t *testing.T) [2][]byte {
	t.Helper()
	var out [2][]byte
	for i, path := range []string{string(p.invowkmodPath), filepath.Join(p.workDir, LockFileName)} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		out[i] = data
	}
	return out
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

// raceFetchWindow runs the inverted replays (design D13): p1 runs on its own
// Resolver and parks when it lists versions, inside its fetch window and
// holding the project lock; p2 runs in a goroutine on another Resolver and
// FileLocker with p2Fetcher. The test waits until p2 reports the busy project
// lock through OnWait (failing fast when p2 finishes without waiting or never
// waits), checks that p2 has written neither file, opens p1's gate, and joins
// both. It returns p1's and p2's errors.
func (p *concurrentProject) raceFetchWindow(t *testing.T, url GitURL, p1 func(*Resolver) error, p2Fetcher moduleFetcher, p2 func(*Resolver) error) (p1Err, p2Err error) {
	t.Helper()
	gate := procgate.New()
	openGate := sync.OnceFunc(gate.Open)
	t.Cleanup(openGate)
	r1 := p.resolver(t, gatedListFetcher{moduleFetcher: p.fetcher(url), gate: gate})
	done1 := procgate.Run(func() (struct{}, error) { return struct{}{}, p1(r1) })
	procgate.AwaitParked(t, gate, done1)

	before := p.files(t)
	waited := make(chan struct{})
	signal := sync.OnceFunc(func() { close(waited) })
	r2 := p.resolver(t, p2Fetcher, func(wait LockWait) {
		if wait.Kind == LockWaitProject {
			signal()
		}
	})
	done2 := procgate.Run(func() (struct{}, error) { return struct{}{}, p2(r2) })
	awaitLockWait(t, waited, done2)
	if after := p.files(t); !bytes.Equal(after[0], before[0]) || !bytes.Equal(after[1], before[1]) {
		t.Fatal("P2 wrote invowkmod.cue or the lock file while P1 held the project lock")
	}
	openGate()
	return (<-done1).Err, (<-done2).Err
}

// awaitLockWait waits until waited is closed (the process reported a busy
// lock), failing fast when the process finishes first or a deadline passes,
// so a no-op lock fails the replay instead of hanging it.
func awaitLockWait[T any](t *testing.T, waited <-chan struct{}, done <-chan procgate.Outcome[T]) {
	t.Helper()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	select {
	case <-waited:
	case o := <-done:
		t.Fatalf("P2 finished without waiting for the lock (error %v)", o.Err)
	case <-deadline.C:
		t.Fatal("P2 did not wait for the lock before the deadline")
	}
}

func addURL(t *testing.T, invowkmodPath types.FilesystemPath, url GitURL) func(*Resolver) error {
	t.Helper()
	return func(r *Resolver) error { return addOK(t.Context(), r, invowkmodPath, url) }
}

func updateAll(t *testing.T) func(*Resolver) error {
	t.Helper()
	return func(r *Resolver) error { _, err := r.UpdateModule(t.Context(), ""); return err }
}

// TestConcurrentEdits_F8SyncKeepsConcurrentAdd replays the sync_add case of
// finding F8, fixed by the project lock: ConcurrentModuleEdits.syncAddProjectLock
// passes, and the pre-fix code is kept as
// ConcurrentModuleEdits.mutantPreFixF8SyncOverwritesConcurrentAdd. P1 runs
// Resolver.SyncModule and parks in its fetch; P2 adds b and must wait for the
// project lock, so its add lands after the sync: requires and lock both hold
// a and b.
func TestConcurrentEdits_F8SyncKeepsConcurrentAdd(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	p1Err, p2Err := p.raceFetchWindow(t, concurrentURLA,
		func(r *Resolver) error { _, err := r.SyncModule(t.Context(), p.invowkmodPath); return err },
		p.fetcher(concurrentURLB), addURL(t, p.invowkmodPath, concurrentURLB))
	if p1Err != nil || p2Err != nil {
		t.Fatalf("SyncModule() error = %v, add error = %v", p1Err, p2Err)
	}

	assertKeys(t, "requires", p.requiresKeys(t), concurrentURLA, concurrentURLB)
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA, concurrentURLB)
}

// TestConcurrentEdits_F8UpdateKeepsConcurrentAdd replays the update_add case
// of finding F8, fixed by the project lock:
// ConcurrentModuleEdits.updateAddProjectLock passes, and the pre-fix code is
// kept as ConcurrentModuleEdits.mutantPreFixF8UpdateOverwritesConcurrentAdd.
// P1 runs Resolver.UpdateModule and parks in its fetch; P2's add of b waits
// for the project lock, so the lock keeps b.
func TestConcurrentEdits_F8UpdateKeepsConcurrentAdd(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	p1Err, p2Err := p.raceFetchWindow(t, concurrentURLA, updateAll(t),
		p.fetcher(concurrentURLB), addURL(t, p.invowkmodPath, concurrentURLB))
	if p1Err != nil || p2Err != nil {
		t.Fatalf("UpdateModule() error = %v, add error = %v", p1Err, p2Err)
	}

	assertKeys(t, "requires", p.requiresKeys(t), concurrentURLA, concurrentURLB)
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA, concurrentURLB)
}

// TestConcurrentEdits_F8UpdateKeepsConcurrentRemove replays the update_remove
// case of finding F8, fixed by the project lock:
// ConcurrentModuleEdits.updateRemoveProjectLock passes, and the pre-fix code
// is kept as ConcurrentModuleEdits.mutantPreFixF8UpdateResurrectsConcurrentRemove.
// P1 runs Resolver.UpdateModule and parks in its fetch; P2's remove of a
// waits for the project lock, so a does not come back into the lock.
func TestConcurrentEdits_F8UpdateKeepsConcurrentRemove(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	p1Err, p2Err := p.raceFetchWindow(t, concurrentURLA, updateAll(t), p.fetcher(concurrentURLA), func(r *Resolver) error {
		removed, removeErr := r.RemoveModuleDependency(t.Context(), p.invowkmodPath, string(concurrentURLA))
		if removeErr != nil || len(removed.Removed()) != 1 || removed.Declarations()[0].Err() != nil {
			return fmt.Errorf("RemoveModuleDependency() = %+v: %w", removed, removeErr)
		}
		return nil
	})
	if p1Err != nil || p2Err != nil {
		t.Fatalf("UpdateModule() error = %v, remove error = %v", p1Err, p2Err)
	}

	assertKeys(t, "requires", p.requiresKeys(t))
	assertKeys(t, "lock", lockKeys(t, p.workDir))
}

// TestConcurrentEdits_F8DuplicateAddKeepsOtherAdd replays the add_add_same
// case of finding F8, fixed by the project lock:
// ConcurrentModuleEdits.addAddSameProjectLock passes, and the pre-fix code is
// kept as ConcurrentModuleEdits.mutantPreFixF8DuplicateAddRollsBackOtherAdd.
// P1 adds b and parks in its fetch; P2's add of b waits for the project lock,
// so it snapshots the files after P1's add: its edit fails as a duplicate, its
// rollback completes, and the files keep P1's add.
func TestConcurrentEdits_F8DuplicateAddKeepsOtherAdd(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)

	var second DeclarationEditResult
	p1Err, p2Err := p.raceFetchWindow(t, concurrentURLB, addURL(t, p.invowkmodPath, concurrentURLB),
		p.fetcher(concurrentURLB), func(r *Resolver) error {
			result, addErr := r.AddModuleDependency(t.Context(), p.invowkmodPath, ModuleRef{GitURL: concurrentURLB, Version: "^1.0.0"})
			second = result.Declaration()
			return addErr
		})
	if p1Err != nil || p2Err != nil {
		t.Fatalf("first add error = %v, second AddModuleDependency() error = %v", p1Err, p2Err)
	}
	if second.Rollback() != DeclarationRollbackComplete || !errors.Is(second.Err(), invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("second add declaration = %s, %v; want a complete rollback of ErrModuleAlreadyExists", second.Rollback(), second.Err())
	}

	assertKeys(t, "requires", p.requiresKeys(t), concurrentURLA, concurrentURLB)
	assertKeys(t, "lock", lockKeys(t, p.workDir), concurrentURLA, concurrentURLB)
}

// newTwoTagModuleRepo builds a local repository whose tags 1.0.0 and 2.0.0
// hold the same module ID with different content in the subdirectory
// twoTagModulePath, and returns the commit of each tag. The module is not
// the repository root, so its cache copy and content hash leave out the
// checkout's .git directory, whose index differs between clones: the hash
// of one version is the same in every cache.
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
	moduleDir := filepath.Join(dir, string(twoTagModulePath))
	if err := os.Mkdir(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	for i, tag := range []SemVer{"1.0.0", "2.0.0"} {
		writeIntegrityModule(t, moduleDir, "cmds: {} // content of "+string(tag))
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

// newWorktreeResolver returns a resolver on its own project that fetches
// through the real GitFetcher on cacheDir, with its own FileLocker on
// cacheDir; onWait, when set, is called when it finds a lock busy.
func newWorktreeResolver(t *testing.T, cacheDir string, gate *procgate.Gate, onWait ...func(LockWait)) *Resolver {
	t.Helper()
	fetcher := &worktreeGateFetcher{inner: NewGitFetcher(types.FilesystemPath(cacheDir)), versions: []SemVer{"2.0.0", "1.0.0"}, gate: gate}
	locks := EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheDir)}
	if len(onWait) > 0 {
		locks.OnWait = onWait[0]
	}
	r, err := newResolverWithFetcher(types.FilesystemPath(t.TempDir()), types.FilesystemPath(cacheDir), fetcher, WithEditLocks(locks))
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	return r
}

// TestConcurrentEdits_F9SourceLockRecordsMatchingContent replays finding F9,
// fixed by the source lock: ConcurrentModuleEdits.fetchProjectAndSourceLock
// passes, and the pre-fix code is kept as
// ConcurrentModuleEdits.mutantPreFixF9SharedWorktreeRecordsWrongContent. Two
// projects sharing one module cache add versions 1.0.0 and 2.0.0 of one
// repository, whose Git source has one worktree per URL. P1 parks after its
// checkout of 1.0.0, holding the source lock; P2 runs in a goroutine and must
// report the busy source lock before it checks out 2.0.0. Once P1 finishes,
// its lock entry records 1.0.0's commit with 1.0.0's content hash.
func TestConcurrentEdits_F9SourceLockRecordsMatchingContent(t *testing.T) {
	t.Parallel()
	repoDir, commits := newTwoTagModuleRepo(t)

	// Reference: 1.0.0 alone, in its own cache.
	refCache := t.TempDir()
	seedSourceClone(t, repoDir, refCache, concurrentURLR)
	reference, _, err := newWorktreeResolver(t, refCache, nil).Add(t.Context(), ModuleRef{GitURL: concurrentURLR, Version: "^1.0.0", Path: twoTagModulePath})
	if err != nil {
		t.Fatalf("reference Add() error = %v", err)
	}

	cacheDir := t.TempDir()
	seedSourceClone(t, repoDir, cacheDir, concurrentURLR)
	gate := procgate.New()
	openGate := sync.OnceFunc(gate.Open)
	t.Cleanup(openGate)
	p1 := newWorktreeResolver(t, cacheDir, gate)
	done := procgate.Run(func() (*ResolvedModule, error) {
		resolved, _, addErr := p1.Add(t.Context(), ModuleRef{GitURL: concurrentURLR, Version: "^1.0.0", Path: twoTagModulePath})
		return resolved, addErr
	})
	procgate.AwaitParked(t, gate, done)

	waited := make(chan struct{})
	signal := sync.OnceFunc(func() { close(waited) })
	p2 := newWorktreeResolver(t, cacheDir, nil, func(wait LockWait) {
		if wait.Kind == LockWaitSource && wait.GitURL == concurrentURLR {
			signal()
		}
	})
	p2Done := procgate.Run(func() (*ResolvedModule, error) {
		resolved, _, addErr := p2.Add(t.Context(), ModuleRef{GitURL: concurrentURLR, Version: "^2.0.0", Path: twoTagModulePath})
		return resolved, addErr
	})
	awaitLockWait(t, waited, p2Done)
	openGate()
	o := <-done
	if o.Err != nil {
		t.Fatalf("P1 Add() error = %v", o.Err)
	}
	o2 := <-p2Done
	if o2.Err != nil || o2.Val.ResolvedVersion != "2.0.0" {
		t.Fatalf("P2 Add() = %+v, %v; want 2.0.0", o2.Val, o2.Err)
	}
	v2 := o2.Val
	if v2.ContentHash == reference.ContentHash {
		t.Fatalf("2.0.0 hash %s equals 1.0.0's; the fixture must give the versions different content", v2.ContentHash)
	}

	entry := loadLockFileForTest(t, filepath.Join(string(p1.WorkingDir()), LockFileName)).Modules[ModuleRef{GitURL: concurrentURLR, Path: twoTagModulePath}.Key()]
	if entry.ResolvedVersion != "1.0.0" || entry.GitCommit != commits["1.0.0"] {
		t.Fatalf("P1 lock entry = %s@%s, want 1.0.0@%s", entry.ResolvedVersion, entry.GitCommit, commits["1.0.0"])
	}
	if entry.ContentHash != reference.ContentHash {
		t.Fatalf("P1 locked hash %s, want 1.0.0's %s (2.0.0's is %s)", entry.ContentHash, reference.ContentHash, v2.ContentHash)
	}
}

// TestConcurrentEdits_F10RestoreReplacesFile is the Go side of finding F10,
// fixed by an atomic restore (ConcurrentModuleEdits.rollbackReadAtomicRestore
// passes; the pre-fix in-place os.WriteFile is kept as
// ConcurrentModuleEdits.mutantPreFixF10NonAtomicRollback). The interleaving
// itself is model-only, since a rename has no intermediate state to replay;
// this test checks that the restore replaces the lock file instead of
// rewriting it in place. The lock already holds the exact entry the add
// resolves, so the add's save is skipped, its invowkmod.cue edit fails as a
// duplicate, and only the rollback can replace the file: a new file means a
// rename, and an in-place write keeps the same file.
func TestConcurrentEdits_F10RestoreReplacesFile(t *testing.T) {
	t.Parallel()
	p := newConcurrentProject(t)
	lockPath := filepath.Join(p.workDir, LockFileName)
	before, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	beforeInfo := statFileIdentity(t, lockPath)

	result, err := p.resolver(t, p.fetcher(concurrentURLA)).AddModuleDependency(t.Context(), p.invowkmodPath, ModuleRef{GitURL: concurrentURLA, Version: "^1.0.0"})
	if err != nil {
		t.Fatalf("AddModuleDependency() error = %v", err)
	}
	if result.LockWritten() {
		t.Fatal("LockWritten = true, want the save skipped so only the restore can replace the lock")
	}
	if !errors.Is(result.Declaration().Err(), invowkmod.ErrModuleAlreadyExists) {
		t.Fatalf("Declaration.Err = %v, want ErrModuleAlreadyExists", result.Declaration().Err())
	}

	after, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile(lock) after rollback error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("restored lock differs from the snapshot:\n got: %s\nwant: %s", after, before)
	}
	afterInfo := statFileIdentity(t, lockPath)
	if os.SameFile(beforeInfo, afterInfo) {
		t.Fatal("the restore rewrote the lock file in place; want an atomic replace (a new file renamed over the path)")
	}
}

func statFileIdentity(t *testing.T, path string) os.FileInfo {
	t.Helper()
	// Open and call File.Stat so Windows populates NTFS file IDs before a
	// rename; os.Stat defers loading them until os.SameFile, which then opens
	// the renamed path and compares the new file to itself.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(%s) error = %v", path, err)
	}
	info, statErr := f.Stat()
	closeErr := f.Close()
	if statErr != nil {
		t.Fatalf("Stat(%s) error = %v", path, statErr)
	}
	if closeErr != nil {
		t.Fatalf("Close(%s) error = %v", path, closeErr)
	}
	return info
}
