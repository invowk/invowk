// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// agentRecordingLocker delegates to a real modulelock.FileLocker and
	// records each lock path; beforeGrant runs before the lock is taken and
	// atUnlock just before it is released.
	agentRecordingLocker struct {
		inner       *modulelock.FileLocker
		beforeGrant func()
		atUnlock    func()

		mu    sync.Mutex
		paths []types.FilesystemPath
	}

	// countingCompleter answers with response and counts its calls.
	countingCompleter struct {
		response string
		calls    int
	}
)

func (l *agentRecordingLocker) Lock(ctx context.Context, path types.FilesystemPath, onBusy func()) (func() error, error) {
	l.mu.Lock()
	l.paths = append(l.paths, path)
	l.mu.Unlock()
	if l.beforeGrant != nil {
		l.beforeGrant()
	}
	unlock, err := l.inner.Lock(ctx, path, onBusy)
	if err != nil {
		return nil, err
	}
	return func() error {
		if l.atUnlock != nil {
			l.atUnlock()
		}
		return unlock()
	}, nil
}

func (l *agentRecordingLocker) PhysicalDir(dir types.FilesystemPath) (types.FilesystemPath, error) {
	return l.inner.PhysicalDir(dir)
}

func (l *agentRecordingLocker) lockPaths() []types.FilesystemPath {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.paths)
}

func (c *countingCompleter) Complete(context.Context, string, string) (string, error) {
	c.calls++
	return c.response, nil
}

func TestCreateModuleDryRunDoesNotWrite(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	result, err := CreateModule(t.Context(), ModuleCreateOptions{
		ModuleID:    "io.example.tools",
		Description: "create a tools module",
		DryRun:      true,
		Completer:   fakeCompleter{response: moduleResponse("io.example.tools", "module generated")},
	})
	if err != nil {
		t.Fatalf("CreateModule() error = %v", err)
	}
	if result.ModuleID != "io.example.tools" {
		t.Fatalf("ModuleID = %q", result.ModuleID)
	}
	if !strings.Contains(result.Diff, filepath.Join("io.example.tools.invowkmod", "invowkmod.cue")) {
		t.Fatalf("dry-run diff missing module path:\n%s", result.Diff)
	}
	if _, err := os.Stat("io.example.tools.invowkmod"); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote module directory, stat err = %v", err)
	}
}

func TestCreateModuleRepairsModuleIDMismatch(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	completer := &sequenceCompleter{
		responses: []string{
			moduleResponse("io.example.other", "wrong"),
			moduleResponse("io.example.tools", "fixed"),
		},
	}

	result, err := CreateModule(t.Context(), ModuleCreateOptions{
		ModuleID:    "io.example.tools",
		Description: "create a tools module",
		DryRun:      true,
		Completer:   completer,
	})
	if err != nil {
		t.Fatalf("CreateModule() error = %v", err)
	}
	if result.ModuleID != "io.example.tools" {
		t.Fatalf("ModuleID = %q", result.ModuleID)
	}
	if len(completer.prompts) != 2 {
		t.Fatalf("completion calls = %d, want 2", len(completer.prompts))
	}
	for _, want := range []string{"generated module ID", "io.example.other", "io.example.tools"} {
		if !strings.Contains(completer.prompts[1], want) {
			t.Fatalf("repair prompt missing %q:\n%s", want, completer.prompts[1])
		}
	}
}

func TestChangeModuleUpdatesOnlyCueFiles(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	modulePath := writeModuleFixture(t, tmpDir, "io.example.tools")
	extraPath := filepath.Join(modulePath, "notes.txt")
	if err := os.WriteFile(extraPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write extra fixture: %v", err)
	}

	result, err := ChangeModule(t.Context(), ModuleChangeOptions{
		Target:      "io.example.tools",
		Description: "change the module",
		Completer:   fakeCompleter{response: moduleResponse("io.example.tools", "module changed")},
		EditLocks:   modulesync.EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(t.TempDir())},
	})
	if err != nil {
		t.Fatalf("ChangeModule() error = %v", err)
	}
	if result.ModuleID != "io.example.tools" {
		t.Fatalf("ModuleID = %q", result.ModuleID)
	}
	extra, err := os.ReadFile(extraPath)
	if err != nil {
		t.Fatalf("read extra file: %v", err)
	}
	if string(extra) != "keep me" {
		t.Fatalf("extra file changed: %q", extra)
	}
}

func TestParseModuleGenerationResponseRejectsExtraFiles(t *testing.T) {
	t.Parallel()

	raw := `{"invowkmod_cue":"module: \"io.example.tools\"\nversion: \"1.0.0\"","invowkfile_cue":"cmds: []","summary":"bad","files":{"scripts/run.sh":"echo bad"}}`
	_, err := ParseModuleGenerationResponse(raw)
	if err == nil {
		t.Fatal("expected error for arbitrary extra file field")
	}
}

func TestRemoveModuleRequiresForceAndRejectsSymlink(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	modulePath := writeModuleFixture(t, tmpDir, "io.example.tools")

	dryRun, err := RemoveModule(t.Context(), ModuleRemoveOptions{
		Target: "io.example.tools",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("RemoveModule() dry-run error = %v", err)
	}
	if !strings.Contains(dryRun.Diff, "delete io.example.tools.invowkmod") {
		t.Fatalf("dry-run delete plan = %q", dryRun.Diff)
	}
	if _, statErr := os.Stat(modulePath); statErr != nil {
		t.Fatalf("dry-run removed module: %v", statErr)
	}

	_, err = RemoveModule(t.Context(), ModuleRemoveOptions{Target: "io.example.tools"})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("RemoveModule() force error = %v, want --force", err)
	}

	if runtime.GOOS != "windows" {
		linkPath := filepath.Join(tmpDir, "io.example.link.invowkmod")
		if symlinkErr := os.Symlink(modulePath, linkPath); symlinkErr != nil {
			t.Fatalf("create symlink fixture: %v", symlinkErr)
		}
		_, err = RemoveModule(t.Context(), ModuleRemoveOptions{Target: linkPath, DryRun: true})
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("RemoveModule() symlink error = %v, want symlink rejection", err)
		}
	}

	_, err = RemoveModule(t.Context(), ModuleRemoveOptions{
		Target: "io.example.tools",
		Force:  true,
	})
	if err != nil {
		t.Fatalf("RemoveModule() force error = %v", err)
	}
	if _, err := os.Stat(modulePath); !os.IsNotExist(err) {
		t.Fatalf("force remove kept module, stat err = %v", err)
	}
}

func TestRemoveCommandDeletesFinalInvowkfile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(target, []byte(wrapCommandObject(testCommandObject)), 0o644); err != nil {
		t.Fatalf("write target fixture: %v", err)
	}

	result, err := RemoveCommand(t.Context(), RemoveOptions{
		Name:       "hello generated",
		TargetPath: target,
	})
	if err != nil {
		t.Fatalf("RemoveCommand() error = %v", err)
	}
	if result.Content != "" {
		t.Fatalf("Content = %q, want empty deletion marker", result.Content)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("RemoveCommand() kept target file, stat err = %v", err)
	}
}

func writeModuleFixture(t *testing.T, root, moduleID string) string {
	t.Helper()

	modulePath := filepath.Join(root, moduleID+invowkmod.ModuleSuffix)
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(moduleCUE(moduleID)), 0o644); err != nil {
		t.Fatalf("write invowkmod fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte(moduleInvowkfileCUE()), 0o644); err != nil {
		t.Fatalf("write invowkfile fixture: %v", err)
	}
	return modulePath
}

func moduleResponse(moduleID, summary string) string {
	return `{"invowkmod_cue":` + quoteForJSON(moduleCUE(moduleID)) +
		`,"invowkfile_cue":` + quoteForJSON(moduleInvowkfileCUE()) +
		`,"summary":` + quoteForJSON(summary) + `}`
}

func moduleCUE(moduleID string) string {
	return `module: "` + moduleID + `"
version: "1.0.0"
description: "Generated tools"
`
}

func moduleInvowkfileCUE() string {
	return `cmds: [{
	name: "modhello"
	description: "Print module greeting"
	implementations: [{
		script: {content: "echo module"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`
}

func TestValidateModuleTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target  string
		wantErr bool
	}{
		{target: "io.example.tools"},
		{target: " io.example.tools "},
		{target: "./tools"},
		{target: "dir/tools"},
		{target: `dir\tools`},
		{target: "io.example.tools.invowkmod"},
		{target: ".hidden"},
		{target: "bad id", wantErr: true},
		{target: "1bad", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()
			err := ValidateModuleTarget(tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateModuleTarget(%q) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

// changedModuleResponse is a generation response that differs from the
// fixture in both files.
func changedModuleResponse(moduleID string) string {
	invowkmodCUE := "module: \"" + moduleID + "\"\nversion: \"1.1.0\"\ndescription: \"Changed tools\"\n"
	invowkfileCUE := strings.Replace(moduleInvowkfileCUE(), "echo module", "echo changed", 1)
	return `{"invowkmod_cue":` + quoteForJSON(invowkmodCUE) +
		`,"invowkfile_cue":` + quoteForJSON(invowkfileCUE) +
		`,"summary":"module changed"}`
}

func readFixtureFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return data
}

// TestChangeModuleWritesAtomicallyUnderProjectLock checks design D15: a
// writing `agent mod change` takes exactly one project lock, on the lock file
// Resolver.LockProject of a resolver on the module directory uses, writes
// both files before it releases the lock, and replaces each file atomically
// (a new file, so os.SameFile with the file before is false; the base's
// os.WriteFile kept the same file).
func TestChangeModuleWritesAtomicallyUnderProjectLock(t *testing.T) {
	t.Parallel()

	modulePath := writeModuleFixture(t, t.TempDir(), "io.example.tools")
	cacheDir := t.TempDir()
	files := []string{filepath.Join(modulePath, invowkmodFileName), filepath.Join(modulePath, invowkfileFileName)}
	before := make([]os.FileInfo, len(files))
	for i, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", path, err)
		}
		before[i] = info
	}
	var atUnlock [][]byte
	locker := &agentRecordingLocker{inner: modulelock.NewFileLocker()}
	locker.atUnlock = func() {
		for _, path := range files {
			data, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			atUnlock = append(atUnlock, data)
		}
	}

	result, err := ChangeModule(t.Context(), ModuleChangeOptions{
		Target:      modulePath,
		Description: "change the module",
		Completer:   fakeCompleter{response: changedModuleResponse("io.example.tools")},
		EditLocks:   modulesync.EditLocks{Locker: locker, CacheDir: types.FilesystemPath(cacheDir)},
	})
	if err != nil {
		t.Fatalf("ChangeModule() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want the generated change written")
	}

	want := []string{result.InvowkmodCUE, result.InvowkfileCUE}
	for i, path := range files {
		if got := string(readFixtureFile(t, path)); got != want[i] {
			t.Fatalf("%s = %q, want the generated %q", path, got, want[i])
		}
		after, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("Stat(%s) after error = %v", path, statErr)
		}
		if os.SameFile(before[i], after) {
			t.Fatalf("%s is the same file as before the change, want an atomic replace", path)
		}
		if i >= len(atUnlock) || string(atUnlock[i]) != want[i] {
			t.Fatalf("%s at the lock release did not hold the generated content", path)
		}
	}

	physical, err := modulelock.NewFileLocker().PhysicalDir(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("PhysicalDir() error = %v", err)
	}
	sum := sha256.Sum256([]byte(physical))
	wantLock := types.FilesystemPath(filepath.Join(cacheDir, "locks", "project-"+hex.EncodeToString(sum[:])+".lock"))
	if got := locker.lockPaths(); !slices.Equal(got, []types.FilesystemPath{wantLock}) {
		t.Fatalf("lock paths = %v, want exactly the project lock %s", got, wantLock)
	}
}

// TestChangeModuleKeepsFileModes checks that the atomic replace keeps each
// existing file's permission bits, as the in-place write it replaced did:
// a 0600 invowkfile.cue does not become 0644.
func TestChangeModuleKeepsFileModes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits to keep")
	}

	modulePath := writeModuleFixture(t, t.TempDir(), "io.example.tools")
	modes := map[string]os.FileMode{
		filepath.Join(modulePath, invowkmodFileName):  0o640,
		filepath.Join(modulePath, invowkfileFileName): 0o600,
	}
	for path, mode := range modes {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("Chmod(%s) error = %v", path, err)
		}
	}

	result, err := ChangeModule(t.Context(), ModuleChangeOptions{
		Target:      modulePath,
		Description: "change the module",
		Completer:   fakeCompleter{response: changedModuleResponse("io.example.tools")},
		EditLocks:   modulesync.EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(t.TempDir())},
	})
	if err != nil || !result.Changed {
		t.Fatalf("ChangeModule() = %+v, %v, want the generated change written", result, err)
	}
	for path, mode := range modes {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("Stat(%s) error = %v", path, statErr)
		}
		if got := info.Mode().Perm(); got != mode {
			t.Fatalf("%s mode = %v, want %v kept", path, got, mode)
		}
	}
}

// TestChangeModuleFailsWhenFilesChangedDuringGeneration checks design D15's
// compare under the lock: invowkmod.cue changes between the generation and
// the lock grant, so the change fails without writing.
func TestChangeModuleFailsWhenFilesChangedDuringGeneration(t *testing.T) {
	t.Parallel()

	modulePath := writeModuleFixture(t, t.TempDir(), "io.example.tools")
	invowkmodPath := filepath.Join(modulePath, invowkmodFileName)
	invowkfilePath := filepath.Join(modulePath, invowkfileFileName)
	invowkfileBefore := readFixtureFile(t, invowkfilePath)
	concurrent := moduleCUE("io.example.tools") + "// edited by another module command\n"
	locker := &agentRecordingLocker{
		inner: modulelock.NewFileLocker(),
		beforeGrant: func() {
			if err := os.WriteFile(invowkmodPath, []byte(concurrent), 0o644); err != nil {
				panic(err)
			}
		},
	}

	_, err := ChangeModule(t.Context(), ModuleChangeOptions{
		Target:      modulePath,
		Description: "change the module",
		Completer:   fakeCompleter{response: changedModuleResponse("io.example.tools")},
		EditLocks:   modulesync.EditLocks{Locker: locker, CacheDir: types.FilesystemPath(t.TempDir())},
	})
	if err == nil || !strings.Contains(err.Error(), "changed while the change was generated; nothing was written") {
		t.Fatalf("ChangeModule() error = %v, want the changed-during-generation error", err)
	}
	if got := string(readFixtureFile(t, invowkmodPath)); got != concurrent {
		t.Fatalf("invowkmod.cue = %q, want the concurrent edit kept", got)
	}
	if got := readFixtureFile(t, invowkfilePath); !bytes.Equal(got, invowkfileBefore) {
		t.Fatalf("invowkfile.cue changed: %q", got)
	}
}

// TestChangeModulePreviewTakesNoLock checks design D15: --dry-run and --print
// take no lock and create no locks directory, and a writing change without a
// locker fails with ErrEditLockerRequired before the completer is called.
func TestChangeModulePreviewTakesNoLock(t *testing.T) {
	t.Parallel()

	for _, preview := range []struct {
		name             string
		dryRun, printOut bool
	}{
		{name: "dry-run", dryRun: true},
		{name: "print", printOut: true},
	} {
		t.Run(preview.name, func(t *testing.T) {
			t.Parallel()
			modulePath := writeModuleFixture(t, t.TempDir(), "io.example.tools")
			cacheDir := t.TempDir()
			locker := &agentRecordingLocker{inner: modulelock.NewFileLocker()}
			_, err := ChangeModule(t.Context(), ModuleChangeOptions{
				Target:      modulePath,
				Description: "change the module",
				DryRun:      preview.dryRun,
				PrintOnly:   preview.printOut,
				Completer:   fakeCompleter{response: changedModuleResponse("io.example.tools")},
				EditLocks:   modulesync.EditLocks{Locker: locker, CacheDir: types.FilesystemPath(cacheDir)},
			})
			if err != nil {
				t.Fatalf("ChangeModule() error = %v", err)
			}
			if got := locker.lockPaths(); len(got) != 0 {
				t.Fatalf("lock paths = %v, want no lock for a preview", got)
			}
			if _, statErr := os.Stat(filepath.Join(cacheDir, "locks")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("locks directory stat error = %v, want not exist", statErr)
			}
		})
	}

	t.Run("write without a locker", func(t *testing.T) {
		t.Parallel()
		modulePath := writeModuleFixture(t, t.TempDir(), "io.example.tools")
		completer := &countingCompleter{response: changedModuleResponse("io.example.tools")}
		_, err := ChangeModule(t.Context(), ModuleChangeOptions{
			Target:      modulePath,
			Description: "change the module",
			Completer:   completer,
		})
		if !errors.Is(err, modulesync.ErrEditLockerRequired) {
			t.Fatalf("ChangeModule() error = %v, want ErrEditLockerRequired", err)
		}
		if completer.calls != 0 {
			t.Fatalf("completer calls = %d, want none before the locker check", completer.calls)
		}
	})
}
