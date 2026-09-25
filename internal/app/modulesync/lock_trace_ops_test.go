// SPDX-License-Identifier: MPL-2.0

package modulesync_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/invowk/invowk/internal/app/moduleops"
	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/tlatrace"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	lockTraceGitURL   = "https://github.com/user/tools.git"
	lockTraceModuleID = invowkmod.ModuleID("io.example.tools")

	lockLabelGood     = "Good"
	lockLabelEvil     = "Evil"
	lockLabelOther    = "Other"
	lockLabelAbsent   = "absent"
	lockLabelNone     = "none"
	lockLabelRejected = "rejected"
)

var lockTraceRequirements = []modulesync.ModuleRef{{GitURL: lockTraceGitURL, Version: "^1.0.0"}}

type (
	// lockTraceInit is one of LockIntegrity.tla's initial states under
	// LegacyLock = TRUE.
	lockTraceInit struct {
		legacy   bool   // lockVer "v1" (no content hash) instead of "v2"
		cache    bool   // cache holds Good; otherwise absent
		vendored bool   // C's vendored copy holds Good; otherwise absent
		sibling  string // P's vendored copy: Good or Other
	}

	// lockTraceFixture holds the fixture trees and their content labels,
	// shared by every trace.
	lockTraceFixture struct {
		trees  map[string]string // label -> module tree
		labels map[invowkmod.ContentHash]string
		dir    string // canonical vendored directory name
	}

	// lockTraceRun is one trace: caller C's module directory, the module
	// cache, sibling P's vendored copy, and the resolver on a trace fetcher.
	lockTraceRun struct {
		t         *testing.T
		fx        *lockTraceFixture
		fetcher   *modulesync.TraceFetcher
		resolver  *modulesync.Resolver
		workDir   string
		cachePath string
		siblingP  string
		loaded    string
		called    string
		syncs     int
		rec       tlatrace.Recorder
	}
)

// newLockTraceFixture writes the three module trees. Good is the tree served
// at c1, Evil the tree at c2 (and the attacker's tamper content), and Other
// the sibling's pinned version; all are valid modules with the same identity.
func newLockTraceFixture(t *testing.T) *lockTraceFixture {
	t.Helper()
	root := t.TempDir()
	fx := &lockTraceFixture{trees: map[string]string{}, labels: map[invowkmod.ContentHash]string{}}
	for label, body := range map[string]string{
		lockLabelGood:  `cmds: {}`,
		lockLabelEvil:  `cmds: {} // served by a re-pointed tag or planted by an attacker`,
		lockLabelOther: `cmds: {} // the sibling's pinned version`,
	} {
		tree := filepath.Join(root, label)
		writeLockTraceFile(t, filepath.Join(tree, "invowkmod.cue"), fmt.Sprintf("module: %q\nversion: \"1.2.3\"\n", lockTraceModuleID))
		writeLockTraceFile(t, filepath.Join(tree, "invowkfile.cue"), body)
		hash, err := invowkmod.ComputeModuleHash(tree)
		if err != nil {
			t.Fatalf("ComputeModuleHash(%s) error = %v", label, err)
		}
		fx.trees[label], fx.labels[hash] = tree, label
	}
	dir, err := invowkmod.CanonicalModuleDirectoryName(lockTraceModuleID)
	if err != nil {
		t.Fatalf("CanonicalModuleDirectoryName() error = %v", err)
	}
	fx.dir = dir.String()
	return fx
}

func writeLockTraceFile(t *testing.T, path, content string) {
	t.Helper()
	testutil.MustMkdirAll(t, filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

// newLockTraceRun sets up st before the first record: a setup sync (not
// counted), then vendoring, wiping the cache, the legacy downgrade (as
// TestSyncRefetchesWhenLockedEntryHasNoHash does), and the
// sibling's copy.
func newLockTraceRun(t *testing.T, fx *lockTraceFixture, st lockTraceInit) *lockTraceRun {
	t.Helper()
	base := t.TempDir()
	r := &lockTraceRun{
		t: t, fx: fx,
		fetcher: &modulesync.TraceFetcher{
			Trees: map[modulesync.GitCommit]types.FilesystemPath{
				modulesync.TraceCommitC1: types.FilesystemPath(fx.trees[lockLabelGood]),
				modulesync.TraceCommitC2: types.FilesystemPath(fx.trees[lockLabelEvil]),
			},
			Commit:   modulesync.TraceCommitC1,
			Versions: []modulesync.SemVer{"1.2.3"},
		},
		workDir:  filepath.Join(base, "caller"),
		siblingP: filepath.Join(base, "sibling", moduleops.VendoredModulesDir, fx.dir),
		loaded:   lockLabelNone,
		called:   lockLabelNone,
	}
	testutil.MustMkdirAll(t, r.workDir, 0o755)
	resolver, err := modulesync.NewResolverWithFetcher(types.FilesystemPath(r.workDir), types.FilesystemPath(filepath.Join(base, "cache")), r.fetcher)
	if err != nil {
		t.Fatalf("NewResolverWithFetcher() error = %v", err)
	}
	r.resolver = resolver
	resolved, err := resolver.Sync(t.Context(), lockTraceRequirements)
	if err != nil {
		t.Fatalf("setup Sync() error = %v", err)
	}
	r.cachePath = string(resolved[0].CachePath)
	if st.vendored {
		if err = r.vendor(); err != nil {
			t.Fatalf("setup vendor error = %v", err)
		}
	}
	if !st.cache {
		testutil.MustRemoveAll(r.t, r.cachePath)
	}
	if st.legacy {
		modulesync.DowngradeLockToV1(t, r.workDir, lockTraceGitURL)
	}
	r.replaceTree(r.siblingP, st.sibling)
	return r
}

func (r *lockTraceRun) lockPath() string { return filepath.Join(r.workDir, modulesync.LockFileName) }

func (r *lockTraceRun) vendoredPath() string {
	return filepath.Join(r.workDir, moduleops.VendoredModulesDir, r.fx.dir)
}

// replaceTree replaces the directory at path with a copy of the label's tree.
func (r *lockTraceRun) replaceTree(path, label string) {
	testutil.MustRemoveAll(r.t, path)
	testutil.MustMkdirAll(r.t, filepath.Dir(path), 0o755)
	if err := os.CopyFS(path, os.DirFS(r.fx.trees[label])); err != nil {
		r.t.Fatalf("CopyFS(%s) error = %v", path, err)
	}
}

// label maps the content at path to Good, Evil, or Other, or absent when the
// directory is missing. An unmapped hash fails the harness.
func (r *lockTraceRun) label(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return lockLabelAbsent
	}
	hash, err := invowkmod.ComputeModuleHash(path)
	if err != nil {
		r.t.Fatalf("ComputeModuleHash(%s) error = %v", path, err)
	}
	return r.hashLabel(hash)
}

func (r *lockTraceRun) hashLabel(hash invowkmod.ContentHash) string {
	label, ok := r.fx.labels[hash]
	if !ok {
		r.t.Fatalf("content hash %s maps to no model content", hash)
	}
	return label
}

func (r *lockTraceRun) commitLabel(commit modulesync.GitCommit) string {
	switch commit {
	case modulesync.TraceCommitC1:
		return "c1"
	case modulesync.TraceCommitC2:
		return "c2"
	}
	r.t.Fatalf("commit %s maps to no model commit", commit)
	return ""
}

// lockEntry reads C's lock from disk as command scope does (LoadLockFile,
// which parses v1.0) and returns it with the module's entry.
func (r *lockTraceRun) lockEntry() (*invowkmod.LockFile, invowkmod.LockedModule) {
	lock, err := invowkmod.LoadLockFile(r.lockPath())
	if err != nil {
		r.t.Fatalf("LoadLockFile() error = %v", err)
	}
	entry, ok := lock.Modules[lockTraceRequirements[0].Key()]
	if !ok {
		r.t.Fatalf("lock has no entry for %s", lockTraceRequirements[0].Key())
	}
	return lock, entry
}

// projection renders LockIntegrityTrace.tla's Proj from the lock file and
// directories on disk and from the call results.
func (r *lockTraceRun) projection() tlatrace.Record {
	lock, entry := r.lockEntry()
	lockVer := map[invowkmod.LockFileVersion]string{invowkmod.LockFileVersionV1: "v1", invowkmod.LockFileVersionV2: "v2"}[lock.Version]
	if lockVer == "" {
		r.t.Fatalf("lock version %q has no model counterpart", lock.Version)
	}
	lockHash := lockLabelNone
	if entry.ContentHash != "" {
		lockHash = r.hashLabel(entry.ContentHash)
	}
	str := tlatrace.Str
	return tlatrace.Record{
		"remoteCommit": str(r.commitLabel(r.fetcher.Commit)), "lockVer": str(lockVer),
		"lockCommit": str(r.commitLabel(entry.GitCommit)), "lockHash": str(lockHash),
		"cache": str(r.label(r.cachePath)), "vendored": str(r.label(r.vendoredPath())), "pVendored": str(r.label(r.siblingP)),
		"loaded": str(r.loaded), "called": str(r.called), "syncs": strconv.Itoa(r.syncs),
	}
}

func (r *lockTraceRun) observe() { r.rec.Observe(r.projection()) }

// apply performs one operation. Attacker actions follow the model's guards;
// code operations run whenever the code allows the call, within the model's
// bounds (two syncs, one discovery, one sibling call without a vendored copy).
func (r *lockTraceRun) apply(op string) {
	r.t.Helper()
	switch op {
	case "MoveTag":
		r.fetcher.Commit = modulesync.TraceCommitC2
	case "TamperCache":
		if l := r.label(r.cachePath); l != lockLabelAbsent && l != lockLabelEvil {
			r.replaceTree(r.cachePath, lockLabelEvil)
		}
	case "WipeCache":
		testutil.MustRemoveAll(r.t, r.cachePath)
	case "TamperVendor":
		if l := r.label(r.vendoredPath()); l != lockLabelAbsent && l != lockLabelEvil {
			r.replaceTree(r.vendoredPath(), lockLabelEvil)
		}
	case "Sync":
		if r.syncs < 2 {
			r.sync()
		}
	case "Vendor":
		cacheAbsent := r.label(r.cachePath) == lockLabelAbsent
		if err := r.vendor(); err != nil && !cacheAbsent &&
			!errors.Is(err, invowkmod.ErrLockFileV1UpgradeRequired) && !errors.Is(err, invowkmod.ErrContentHashMismatch) {
			r.t.Fatalf("vendor error = %v", err)
		}
	case "Discover":
		if r.loaded == lockLabelNone && r.label(r.vendoredPath()) != lockLabelAbsent {
			r.discover()
		}
	case "CallViaSibling":
		if r.called == lockLabelNone && r.label(r.vendoredPath()) == lockLabelAbsent {
			r.callViaSibling()
		}
	default:
		r.t.Fatalf("unknown lock trace operation %q", op)
	}
}

func (r *lockTraceRun) sync() {
	r.syncs++
	_, err := r.resolver.Sync(r.t.Context(), lockTraceRequirements)
	if err != nil && !errors.Is(err, modulesync.ErrLockedCommitMismatch) && !errors.Is(err, invowkmod.ErrContentHashMismatch) {
		r.t.Fatalf("Sync() error = %v", err)
	}
}

// vendor is the update-false path of resolveVendorDependencies when a lock
// exists: LoadDeclaredFromLock, then VendorModules on its result.
func (r *lockTraceRun) vendor() error {
	resolved, err := r.resolver.LoadDeclaredFromLock(r.t.Context(), lockTraceRequirements)
	if err != nil {
		return fmt.Errorf("LoadDeclaredFromLock: %w", err)
	}
	if _, err = moduleops.VendorModules(moduleops.VendorOptions{ModulePath: types.FilesystemPath(r.workDir), Modules: resolved}); err != nil {
		return fmt.Errorf("VendorModules: %w", err)
	}
	return nil
}

// discover checks C's vendored copy against C's lock entry, as
// discoverVendoredModulesWithDiagnostics does.
func (r *lockTraceRun) discover() {
	module, err := invowkmod.Load(types.FilesystemPath(r.vendoredPath()))
	if err != nil {
		r.t.Fatalf("Load(vendored) error = %v", err)
	}
	_, entry := r.lockEntry()
	err = invowkmod.VerifyLockedVendoredModuleHash(lockTraceRequirements[0].Key(), entry, module)
	switch {
	case err == nil:
		r.loaded = r.label(r.vendoredPath())
	case errors.Is(err, invowkmod.ErrContentHashMismatch), errors.Is(err, invowkmod.ErrLockEntryWithoutContentHash):
		r.loaded = lockLabelRejected
	default:
		r.t.Fatalf("VerifyLockedVendoredModuleHash() error = %v", err)
	}
}

// callViaSibling asks command-scope admission whether C may run M through
// P's vendored copy, with C's lock loaded as LoadCommandScopeLock loads it.
func (r *lockTraceRun) callViaSibling() {
	lock, _ := r.lockEntry()
	requirements := []invowkmod.ModuleRequirement{invowkmod.ModuleRequirement(lockTraceRequirements[0])}
	sourceID := invowkmod.ModuleSourceID(lockTraceModuleID)
	if invowkmod.IsDeclaredLockedCommandSource(requirements, lock, lockTraceModuleID, sourceID, types.FilesystemPath(r.siblingP)) {
		r.called = r.label(r.siblingP)
	} else {
		r.called = lockLabelRejected
	}
}
