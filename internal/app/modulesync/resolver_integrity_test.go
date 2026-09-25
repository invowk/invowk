// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	integrityTestGitURL          = "https://github.com/user/tools.git"
	integrityTestRepoedCommit    = GitCommit("0000000000000000000000000000000000000001")
	integrityTestOriginalContent = `cmds: {}`
	integrityTestChangedContent  = `cmds: {} // served by a compromised or re-tagged remote`
)

type integrityFixture struct {
	workDir  string
	cacheDir string
	repoDir  string
	fetcher  *fakeModuleFetcher
	resolver *Resolver
}

func newIntegrityFixture(t *testing.T, cacheDir string, versions ...SemVer) *integrityFixture {
	t.Helper()

	f := &integrityFixture{
		workDir:  t.TempDir(),
		cacheDir: cacheDir,
		repoDir:  t.TempDir(),
	}
	writeIntegrityModule(t, f.repoDir, integrityTestOriginalContent)
	f.fetcher = &fakeModuleFetcher{
		repoPath:     types.FilesystemPath(f.repoDir),
		listVersions: versions,
	}
	resolver, err := newResolverWithFetcher(types.FilesystemPath(f.workDir), types.FilesystemPath(f.cacheDir), f.fetcher)
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	f.resolver = resolver
	return f
}

func writeIntegrityModule(t *testing.T, dir, invowkfile string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte(`module: "io.example.tools"
version: "1.2.3"`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(invowkfile), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}
}

func (f *integrityFixture) sync(ctx context.Context) ([]*ResolvedModule, error) {
	return f.resolver.Sync(ctx, []ModuleRef{{GitURL: integrityTestGitURL, Version: "^1.0.0"}})
}

func (f *integrityFixture) mustSync(t *testing.T) *ResolvedModule {
	t.Helper()
	resolved, err := f.sync(t.Context())
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("Sync() resolved %d modules, want 1", len(resolved))
	}
	return resolved[0]
}

func (f *integrityFixture) readLock(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.workDir, LockFileName))
	if err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	return data
}

func (f *integrityFixture) wipeCache(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(f.cacheDir); err != nil {
		t.Fatalf("RemoveAll(cache) error = %v", err)
	}
	if err := os.MkdirAll(f.cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be absent, stat error = %v", path, err)
	}
}

func TestSyncFreshCacheReproducesLockedHash(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	first := f.mustSync(t)
	lockBefore := f.readLock(t)

	f.wipeCache(t)
	second := f.mustSync(t)

	if second.ContentHash != first.ContentHash {
		t.Fatalf("ContentHash = %s, want unchanged %s", second.ContentHash, first.ContentHash)
	}
	if !bytes.Equal(f.readLock(t), lockBefore) {
		t.Fatal("lock file changed although the locked version reproduced its content")
	}
}

func TestSyncFreshCacheRejectsChangedContent(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	first := f.mustSync(t)
	lockBefore := f.readLock(t)

	f.wipeCache(t)
	writeIntegrityModule(t, f.repoDir, integrityTestChangedContent)

	_, err := f.sync(t.Context())
	if !errors.Is(err, invowkmod.ErrContentHashMismatch) {
		t.Fatalf("Sync() error = %v, want ErrContentHashMismatch", err)
	}
	mismatch, ok := errors.AsType[*invowkmod.ContentHashMismatchError](err)
	if !ok {
		t.Fatalf("Sync() error %v is not a ContentHashMismatchError", err)
	}
	if mismatch.ModuleKey != ModuleRefKey(integrityTestGitURL) || mismatch.Version != "1.2.3" {
		t.Fatalf("mismatch context = %q@%s, want %q@1.2.3", mismatch.ModuleKey, mismatch.Version, integrityTestGitURL)
	}
	if mismatch.Expected != first.ContentHash {
		t.Fatalf("mismatch.Expected = %s, want locked %s", mismatch.Expected, first.ContentHash)
	}
	assertPathAbsent(t, string(first.CachePath))
	if !bytes.Equal(f.readLock(t), lockBefore) {
		t.Fatal("lock file was rewritten after a failed verification")
	}
}

func TestSyncExistingCacheRejectsTamperedContent(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	first := f.mustSync(t)
	lockBefore := f.readLock(t)

	if err := os.WriteFile(filepath.Join(string(first.CachePath), "invowkfile.cue"), []byte(integrityTestChangedContent), 0o644); err != nil {
		t.Fatalf("WriteFile(tamper cache) error = %v", err)
	}

	_, err := f.sync(t.Context())
	if !errors.Is(err, invowkmod.ErrContentHashMismatch) {
		t.Fatalf("Sync() error = %v, want ErrContentHashMismatch", err)
	}
	if !bytes.Equal(f.readLock(t), lockBefore) {
		t.Fatal("lock file was rewritten after a failed verification")
	}
}

func TestSyncRejectsRepointedTag(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	first := f.mustSync(t)
	lockBefore := f.readLock(t)

	f.wipeCache(t)
	f.fetcher.commit = integrityTestRepoedCommit

	_, err := f.sync(t.Context())
	if !errors.Is(err, ErrLockedCommitMismatch) {
		t.Fatalf("Sync() error = %v, want ErrLockedCommitMismatch", err)
	}
	commitErr, ok := errors.AsType[*LockedCommitMismatchError](err)
	if !ok {
		t.Fatalf("Sync() error %v is not a LockedCommitMismatchError", err)
	}
	if commitErr.Locked != defaultFakeCommit || commitErr.Fetched != integrityTestRepoedCommit {
		t.Fatalf("commit mismatch = locked %s fetched %s", commitErr.Locked, commitErr.Fetched)
	}
	assertPathAbsent(t, string(first.CachePath))
	if !bytes.Equal(f.readLock(t), lockBefore) {
		t.Fatal("lock file was rewritten after a commit mismatch")
	}
}

func TestSyncVersionChangeRecordsNewBaseline(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	first := f.mustSync(t)

	f.fetcher.listVersions = []SemVer{"1.2.3", "1.3.0"}
	f.fetcher.commit = integrityTestRepoedCommit
	writeIntegrityModule(t, f.repoDir, integrityTestChangedContent)

	second := f.mustSync(t)
	if second.ResolvedVersion != "1.3.0" {
		t.Fatalf("ResolvedVersion = %s, want 1.3.0", second.ResolvedVersion)
	}
	if second.ContentHash == first.ContentHash {
		t.Fatal("new version recorded the old version's content hash")
	}
	lock, err := invowkmod.LoadLockFile(filepath.Join(f.workDir, LockFileName))
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	entry := lock.Modules[ModuleRefKey(integrityTestGitURL)]
	if entry.ResolvedVersion != "1.3.0" || entry.GitCommit != integrityTestRepoedCommit || entry.ContentHash != second.ContentHash {
		t.Fatalf("lock entry = %s %s %s, want the new version's baseline", entry.ResolvedVersion, entry.GitCommit, entry.ContentHash)
	}
}

// TestSyncVersionChangeReusesCacheWithoutStaleExpectation covers the latent
// false mismatch: the expected hash used to be keyed by source only, so a new
// version already cached by another project was compared with the old
// version's hash.
func TestSyncVersionChangeReusesCacheWithoutStaleExpectation(t *testing.T) {
	t.Parallel()

	sharedCache := t.TempDir()

	project := newIntegrityFixture(t, sharedCache, "1.2.3")
	project.mustSync(t)

	other := newIntegrityFixture(t, sharedCache, "1.3.0")
	writeIntegrityModule(t, other.repoDir, integrityTestChangedContent)
	cached := other.mustSync(t)

	project.fetcher.listVersions = []SemVer{"1.2.3", "1.3.0"}
	writeIntegrityModule(t, project.repoDir, integrityTestChangedContent)
	upgraded := project.mustSync(t)

	if upgraded.ResolvedVersion != "1.3.0" || upgraded.ContentHash != cached.ContentHash {
		t.Fatalf("upgrade = %s %s, want 1.3.0 %s", upgraded.ResolvedVersion, upgraded.ContentHash, cached.ContentHash)
	}
}

func TestUpdateWithoutNewerVersionVerifiesBaseline(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	f.mustSync(t)

	f.fetcher.commit = integrityTestRepoedCommit
	_, err := f.resolver.Update(t.Context(), "")
	if !errors.Is(err, ErrLockedCommitMismatch) {
		t.Fatalf("Update() error = %v, want ErrLockedCommitMismatch", err)
	}
}

func TestTidyPassesLockedModulesToEveryRound(t *testing.T) {
	t.Parallel()

	refA := ModuleRef{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"}
	refB := ModuleRef{GitURL: "https://github.com/org/B.git", Version: "^1.0.0"}
	locked := map[ModuleRefKey]LockedModule{refA.Key(): {ResolvedVersion: "1.0.0"}}

	rounds := 0
	resolveAll := func(_ context.Context, requirements []ModuleRef, got map[ModuleRefKey]LockedModule) ([]*ResolvedModule, error) {
		rounds++
		if _, ok := got[refA.Key()]; !ok || len(got) != len(locked) {
			t.Errorf("round %d received lock entries %v, want %v", rounds, got, locked)
		}
		resolved := make([]*ResolvedModule, 0, len(requirements))
		for _, req := range requirements {
			mod := &ResolvedModule{ModuleRef: req, ModuleID: ModuleID(req.Key())}
			if req.Key() == refA.Key() {
				mod.TransitiveDeps = []ModuleRef{refB}
			}
			resolved = append(resolved, mod)
		}
		return resolved, nil
	}

	if _, err := tidyToFixedPoint(t.Context(), []ModuleRef{refA}, locked, resolveAll); err != nil {
		t.Fatalf("tidyToFixedPoint() error = %v", err)
	}
	if rounds != 2 {
		t.Fatalf("rounds = %d, want 2", rounds)
	}
}
