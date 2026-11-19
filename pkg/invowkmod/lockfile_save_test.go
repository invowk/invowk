// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

var (
	// saveTestBackdated is a fixed past generated: instant. Every test that
	// expects a rewrite starts from a file stamped with it, because generated:
	// has second resolution and two saves in a row can share a second.
	saveTestBackdated = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

	saveTestGeneratedLine = regexp.MustCompile(`(?m)^generated: "(.*)"$`)
)

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

// requireRewrittenAfterBackdate asserts that the file at path was written by
// Save: it parses, carries saved's content, and its generated: is later than
// saveTestBackdated and matches the saved lock's Generated.
func requireRewrittenAfterBackdate(t *testing.T, path string, saved *LockFile) {
	t.Helper()
	data := readSaveTestFile(t, path)
	match := saveTestGeneratedLine.FindStringSubmatch(data)
	if match == nil {
		t.Fatalf("written lock has no generated: line:\n%s", data)
	}
	generated, err := time.Parse(time.RFC3339, match[1])
	if err != nil {
		t.Fatalf("written generated: %q does not parse: %v", match[1], err)
	}
	if !generated.After(saveTestBackdated) {
		t.Fatalf("written generated: = %s, want later than %s", generated, saveTestBackdated)
	}
	if !saved.Generated.After(saveTestBackdated) {
		t.Fatalf("saved lock Generated = %s, want the write time", saved.Generated)
	}
	loaded, err := LoadLockFile(path)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
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

	if err := lock.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
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

	if err := lock.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
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
			if err := lock.Save(path); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			requireRewrittenAfterBackdate(t, path, lock)
		})
	}
}

func TestLockFileSaveWritesVersionChange(t *testing.T) {
	t.Parallel()

	path := writeSaveTestFile(t, "version: \"1.0\"\ngenerated: \"2001-02-03T04:05:06Z\"\nmodules: {}\n")
	lock := saveTestLock(map[ModuleRefKey]LockedModule{})

	if err := lock.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveCreatesMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", LockFileName)
	lock := saveTestLock(saveTestModules())

	if err := lock.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	requireRewrittenAfterBackdate(t, path, lock)
}

func TestLockFileSaveReplacesUnparseableFile(t *testing.T) {
	t.Parallel()

	path := writeSaveTestFile(t, "this is { not a lock file\n")
	lock := saveTestLock(saveTestModules())

	if err := lock.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	requireRewrittenAfterBackdate(t, path, lock)
}
