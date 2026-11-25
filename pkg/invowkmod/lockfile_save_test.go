// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// saveTestBackdated is a fixed past generated: instant. Every test that
// expects a rewrite starts from a file stamped with it, because generated:
// has second resolution and two saves in a row can share a second.
var saveTestBackdated = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

func saveTestLock(modules map[ModuleRefKey]LockedModule) *LockFile {
	return &LockFile{Version: LockFileVersionV2, Generated: saveTestBackdated, Modules: modules}
}

func saveTestModules() map[ModuleRefKey]LockedModule {
	return map[ModuleRefKey]LockedModule{ModuleRefKey(lockMutationGitURL): validLockMutationLockedModule()}
}

// writeSaveTestFile writes data as the lock file and returns its path.
func writeSaveTestFile(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), LockFileName)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(lock) error = %v", err)
	}
	return path
}

func readSaveTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	return string(data)
}

// requireSaveIfChanged saves lock to path with SaveIfChanged and asserts that
// it succeeded and reported wantWritten.
func requireSaveIfChanged(t *testing.T, lock *LockFile, path string, wantWritten bool) {
	t.Helper()
	written, err := lock.SaveIfChanged(path)
	if err != nil {
		t.Fatalf("SaveIfChanged() error = %v", err)
	}
	if written != wantWritten {
		t.Fatalf("SaveIfChanged() written = %t, want %t", written, wantWritten)
	}
}

// requireRewrittenAfterBackdate asserts that the file at path was written by
// Save: it parses, carries saved's content, and both its generated: and the
// saved lock's Generated are later than saveTestBackdated.
func requireRewrittenAfterBackdate(t *testing.T, path string, saved *LockFile) {
	t.Helper()
	if !saved.Generated.After(saveTestBackdated) {
		t.Fatalf("saved lock Generated = %s, want the write time", saved.Generated)
	}
	loaded, err := LoadLockFile(path)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	if !loaded.Generated.After(saveTestBackdated) {
		t.Fatalf("written generated: = %s, want later than %s", loaded.Generated, saveTestBackdated)
	}
	if !loaded.sameContent(saved) {
		t.Fatalf("written lock = %+v, want the saved content %+v", loaded, saved)
	}
}

func TestLockFileSaveSkipsEquivalentNonCanonicalFile(t *testing.T) {
	t.Parallel()

	// Valid, equal in meaning to saveTestModules(), but not the writer's
	// layout: a +00:00 offset, field order, spacing and no header comment.
	const onDisk = `version: "2.0"
generated:   "2001-02-03T04:05:06+00:00"
modules: {
	"https://github.com/example/tools.git": {
		version: "^1.0.0"
		git_url: "https://github.com/example/tools.git"
		resolved_version: "1.2.3"
		git_commit: "0123456789abcdef0123456789abcdef01234567"
		alias: "tools"
		path: "modules/tools"
		namespace: "tools"
		command_source_id: "tools"
		module_id: "io.example.tools"
		content_hash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
}
`
	path := writeSaveTestFile(t, onDisk)
	lock := saveTestLock(saveTestModules())
	lock.Generated = time.Now()

	requireSaveIfChanged(t, lock, path, false)
	if got := readSaveTestFile(t, path); got != onDisk {
		t.Fatalf("Save() rewrote an equivalent lock:\n got: %s\nwant: %s", got, onDisk)
	}
	if !lock.Generated.Equal(saveTestBackdated) {
		t.Fatalf("lock.Generated = %s after a skipped write, want the file's %s", lock.Generated, saveTestBackdated)
	}
}

func TestLockFileSaveSkipsEquivalentEmptyLock(t *testing.T) {
	t.Parallel()

	const onDisk = "version: \"2.0\"\ngenerated: \"2001-02-03T04:05:06Z\"\nmodules: {}\n"
	path := writeSaveTestFile(t, onDisk)
	// A nil map and an empty one lock the same (no) modules.
	lock := &LockFile{Version: LockFileVersionV2, Generated: time.Now()}

	requireSaveIfChanged(t, lock, path, false)
	if got := readSaveTestFile(t, path); got != onDisk {
		t.Fatalf("Save() rewrote an equivalent empty lock:\n got: %s\nwant: %s", got, onDisk)
	}
}

func TestLockFileSaveWritesChangedContent(t *testing.T) {
	t.Parallel()

	changeField := func(change func(*LockedModule)) func() *LockFile {
		return func() *LockFile {
			mod := validLockMutationLockedModule()
			change(&mod)
			return saveTestLock(map[ModuleRefKey]LockedModule{ModuleRefKey(lockMutationGitURL): mod})
		}
	}
	tests := []struct {
		name string
		lock func() *LockFile
	}{
		{name: "git_url", lock: changeField(func(m *LockedModule) { m.GitURL = "https://github.com/example/fork.git" })},
		{name: "version", lock: changeField(func(m *LockedModule) { m.Version = "^1.2.0" })},
		{name: "resolved_version", lock: changeField(func(m *LockedModule) { m.ResolvedVersion = "1.2.4" })},
		{name: "git_commit", lock: changeField(func(m *LockedModule) { m.GitCommit = "fedcba9876543210fedcba9876543210fedcba98" })},
		{name: "alias", lock: changeField(func(m *LockedModule) { m.Alias = "tools2" })},
		{name: "path", lock: changeField(func(m *LockedModule) { m.Path = "modules/other" })},
		{name: "namespace", lock: changeField(func(m *LockedModule) { m.Namespace = "tools2" })},
		{name: "command_source_id", lock: changeField(func(m *LockedModule) { m.CommandSourceID = "tools2" })},
		{name: "module_id", lock: changeField(func(m *LockedModule) { m.ModuleID = "io.example.other" })},
		{name: "content_hash", lock: changeField(func(m *LockedModule) {
			m.ContentHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		})},
		{name: "added key", lock: func() *LockFile {
			modules := saveTestModules()
			other := validLockMutationLockedModule()
			other.GitURL = "https://github.com/example/other.git"
			modules[ModuleRefKey(other.GitURL)] = other
			return saveTestLock(modules)
		}},
		{name: "removed key", lock: func() *LockFile {
			return saveTestLock(map[ModuleRefKey]LockedModule{})
		}},
		{name: "renamed key", lock: func() *LockFile {
			return saveTestLock(map[ModuleRefKey]LockedModule{"https://github.com/example/tools.git#v2": validLockMutationLockedModule()})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeSaveTestFile(t, saveTestLock(saveTestModules()).toCUE())
			lock := tt.lock()
			requireSaveIfChanged(t, lock, path, true)
			requireRewrittenAfterBackdate(t, path, lock)
		})
	}
}

func TestLockFileSaveWritesVersionChange(t *testing.T) {
	t.Parallel()

	path := writeSaveTestFile(t, "version: \"1.0\"\ngenerated: \"2001-02-03T04:05:06Z\"\nmodules: {}\n")
	lock := saveTestLock(map[ModuleRefKey]LockedModule{})

	requireSaveIfChanged(t, lock, path, true)
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveCreatesMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", LockFileName)
	lock := saveTestLock(saveTestModules())

	requireSaveIfChanged(t, lock, path, true)
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveReplacesUnparseableFile(t *testing.T) {
	t.Parallel()

	path := writeSaveTestFile(t, "this is { not a lock file\n")
	lock := saveTestLock(saveTestModules())

	requireSaveIfChanged(t, lock, path, true)
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveReplacesUnknownVersionFile(t *testing.T) {
	t.Parallel()

	// Valid CUE whose version the parser rejects (UnknownLockFileVersionError):
	// it is not a valid lock, so Save writes over it.
	path := writeSaveTestFile(t, "version: \"9.0\"\ngenerated: \"2001-02-03T04:05:06Z\"\nmodules: {}\n")
	lock := saveTestLock(saveTestModules())

	requireSaveIfChanged(t, lock, path, true)
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveReplacesOversizedFile(t *testing.T) {
	t.Parallel()

	// One byte over LockFileSizeLimit: InspectLockFile reports a size error
	// without parsing, so Save writes over it.
	path := writeSaveTestFile(t, strings.Repeat("x", LockFileSizeLimit+1))
	lock := saveTestLock(saveTestModules())

	requireSaveIfChanged(t, lock, path, true)
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveDelegatesSkipToSaveIfChanged(t *testing.T) {
	t.Parallel()

	// Save keeps its signature: on the equal-content skip it returns nil and
	// leaves the file byte-identical, as SaveIfChanged does.
	const onDisk = "version: \"2.0\"\ngenerated: \"2001-02-03T04:05:06Z\"\nmodules: {}\n"
	path := writeSaveTestFile(t, onDisk)
	lock := &LockFile{Version: LockFileVersionV2, Generated: time.Now()}

	if err := lock.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got := readSaveTestFile(t, path); got != onDisk {
		t.Fatalf("Save() rewrote an equivalent lock:\n got: %s\nwant: %s", got, onDisk)
	}
}

func TestLockFileSaveIfChangedFailedSaveIsNotAWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lock    *LockFile
		wantErr error
	}{
		{name: "nil lock", lock: nil},
		{
			name:    "invalid version",
			lock:    &LockFile{Version: "9.9", Modules: map[ModuleRefKey]LockedModule{}},
			wantErr: ErrInvalidLockFileVersion,
		},
		{
			name:    "invalid module key",
			lock:    saveTestLock(map[ModuleRefKey]LockedModule{" \t": validLockMutationLockedModule()}),
			wantErr: ErrInvalidModuleRefKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), LockFileName)
			written, err := tt.lock.SaveIfChanged(path)
			if err == nil {
				t.Fatal("SaveIfChanged() error = nil, want a validation error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("SaveIfChanged() error = %v, want %v", err, tt.wantErr)
			}
			if written {
				t.Fatal("SaveIfChanged() written = true on a failed save")
			}
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Stat(lock) error = %v, want the failed save to leave no file", statErr)
			}
		})
	}
}

func TestLockFileSaveIfChangedMkdirErrorIsNotAWrite(t *testing.T) {
	t.Parallel()

	// A regular file where the parent directory should be makes MkdirAll fail.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	written, err := saveTestLock(saveTestModules()).SaveIfChanged(filepath.Join(blocker, "child", LockFileName))
	if err == nil {
		t.Fatal("SaveIfChanged() error = nil, want parent directory failure")
	}
	if written {
		t.Fatal("SaveIfChanged() written = true on a failed save")
	}
}

func TestLockFileSaveIfChangedAtomicWriteErrorIsNotAWrite(t *testing.T) {
	t.Parallel()

	// A non-empty directory at the lock path: the parent exists, so MkdirAll
	// succeeds, and the atomic write fails when it renames its temporary file
	// over the directory.
	path := filepath.Join(t.TempDir(), LockFileName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(lock path) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(keep) error = %v", err)
	}
	written, err := saveTestLock(saveTestModules()).SaveIfChanged(path)
	if err == nil {
		t.Fatal("SaveIfChanged() error = nil, want the atomic write to fail")
	}
	if written {
		t.Fatal("SaveIfChanged() written = true on a failed write")
	}
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("Stat(lock path) = %v, %v; want the directory left in place", info, statErr)
	}
}
