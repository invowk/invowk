// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	stubCommandSetProvider struct {
		result discovery.CommandSetResult
		err    error
	}

	recordingHostProbe struct {
		tools          []invowkfile.BinaryName
		toolErrors     map[invowkfile.BinaryName]error
		filepaths      []types.FilesystemPath
		filepathErrors map[types.FilesystemPath]error
		checks         []invowkfile.CheckName
		checkErrors    map[invowkfile.CheckName]error
	}

	staticCommandScopeLockProvider struct {
		lock *invowkmod.LockFile
		err  error
	}
)

func (s *stubCommandSetProvider) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	if s.err != nil {
		return discovery.CommandSetResult{}, s.err
	}
	return s.result, nil
}

func (p *recordingHostProbe) CheckTool(tool invowkfile.BinaryName) error {
	p.tools = append(p.tools, tool)
	if p.toolErrors != nil {
		return p.toolErrors[tool]
	}
	return nil
}

func (p *recordingHostProbe) CheckFilepath(_, resolvedPath types.FilesystemPath, _ invowkfile.FilepathDependency) error {
	p.filepaths = append(p.filepaths, resolvedPath)
	if p.filepathErrors != nil {
		return p.filepathErrors[resolvedPath]
	}
	return nil
}

func (p *recordingHostProbe) RunCustomCheck(_ context.Context, check invowkfile.CustomCheck) error {
	p.checks = append(p.checks, check.Name)
	if p.checkErrors != nil {
		return p.checkErrors[check.Name]
	}
	return nil
}

func (p staticCommandScopeLockProvider) LoadCommandScopeLock(*invowkfile.Invowkfile) (*invowkmod.LockFile, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.lock == nil {
		return &invowkmod.LockFile{}, nil
	}
	return p.lock, nil
}

func TestCheckCommandDependenciesExist(t *testing.T) {
	t.Parallel()

	ctx := &runtimepkg.ExecutionContext{
		Command: &invowkfile.Command{Name: "build"},
		Context: t.Context(),
	}

	// Root invowkfile cmdInfo — no module metadata, no scope enforcement.
	rootCmdInfo := &discovery.CommandInfo{
		Name:       invowkfile.CommandName("build"),
		Command:    &invowkfile.Command{Name: "build"},
		Invowkfile: &invowkfile.Invowkfile{},
	}

	// Module cmdInfo with module "mod" — for qualified name lookup.
	modMeta := invowkfile.NewModuleMetadataFromInvowkmod(&invowkfile.Invowkmod{
		Module:  "mod",
		Version: "1.0.0",
	})
	modCmdInfo := &discovery.CommandInfo{
		Name:       invowkfile.CommandName("build"),
		Command:    &invowkfile.Command{Name: "build"},
		Invowkfile: &invowkfile.Invowkfile{Metadata: modMeta},
	}

	t.Run("accepts exact and module-qualified alternatives", func(t *testing.T) {
		t.Parallel()

		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{
				{Name: invowkfile.CommandName("deploy")},
				{Name: invowkfile.CommandName("mod build")},
			},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandName{"build"}},
				{Alternatives: []invowkfile.CommandName{"deploy"}},
			},
		}

		// Module cmdInfo: "build" matches via qualified form "mod build".
		if err := CheckCommandDependenciesExist(disc, deps, modCmdInfo, ctx); err != nil {
			t.Fatalf("CheckCommandDependenciesExist() = %v", err)
		}
	})

	t.Run("reports missing alternatives", func(t *testing.T) {
		t.Parallel()

		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandName{"missing", "other"}},
			},
		}

		err := CheckCommandDependenciesExist(disc, deps, rootCmdInfo, ctx)
		if err == nil {
			t.Fatal("expected dependency error")
		}

		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.MissingCommands) != 1 {
			t.Fatalf("len(depErr.MissingCommands) = %d, want 1", len(depErr.MissingCommands))
		}
		if !strings.Contains(depErr.MissingCommands[0].String(), "none of [missing, other] found") {
			t.Fatalf("depErr.MissingCommands[0] = %q", depErr.MissingCommands[0])
		}
	})

	t.Run("allows locked non-aliased direct dependency module id", func(t *testing.T) {
		t.Parallel()

		depID := invowkmod.ModuleID("io.example.dep")
		req := invowkmod.ModuleRequirement{
			GitURL:  "https://github.com/example/mono.git",
			Version: "^1.0.0",
			Path:    "modules/dep-tools",
		}
		moduleDir := t.TempDir()
		lock := invowkmod.NewLockFile()
		lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
			GitURL:          req.GitURL,
			Version:         req.Version,
			ResolvedVersion: "1.2.3",
			GitCommit:       "0123456789abcdef0123456789abcdef01234567",
			Path:            req.Path,
			Namespace:       "dep-tools@1.2.3",
			ModuleID:        depID,
			ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}
		callerMeta := invowkfile.NewModuleMetadataFromInvowkmod(&invowkfile.Invowkmod{
			Module:   "io.example.caller",
			Version:  "1.0.0",
			Requires: []invowkmod.ModuleRequirement{req},
		})
		callerInfo := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("build"),
			Command:    &invowkfile.Command{Name: "build"},
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(moduleDir), Metadata: callerMeta},
		}
		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{
				Name:     invowkfile.CommandName("io.example.dep test"),
				SourceID: discovery.SourceID("dep-tools"),
				ModuleID: &depID,
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandName{"io.example.dep test"}},
			},
		}

		if err := CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{lock: lock}); err != nil {
			t.Fatalf("CheckCommandDependenciesExist() = %v", err)
		}
	})

	t.Run("rejects same module identity under unmatched source namespace", func(t *testing.T) {
		t.Parallel()

		req := invowkmod.ModuleRequirement{
			GitURL:  "https://github.com/example/tools.git",
			Version: "^1.0.0",
			Alias:   "allowed-tools",
		}
		moduleDir := t.TempDir()
		depID := invowkmod.ModuleID("io.example.tools")
		lock := invowkmod.NewLockFile()
		lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
			GitURL:          req.GitURL,
			Version:         req.Version,
			ResolvedVersion: "1.2.3",
			GitCommit:       "0123456789abcdef0123456789abcdef01234567",
			Alias:           req.Alias,
			Namespace:       "allowed-tools",
			ModuleID:        depID,
			ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}
		callerMeta := invowkfile.NewModuleMetadataFromInvowkmod(&invowkfile.Invowkmod{
			Module:   "io.example.caller",
			Version:  "1.0.0",
			Requires: []invowkmod.ModuleRequirement{req},
		})
		callerInfo := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("build"),
			Command:    &invowkfile.Command{Name: "build"},
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(moduleDir), Metadata: callerMeta},
		}
		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{
				Name:     invowkfile.CommandName("other-tools test"),
				SourceID: discovery.SourceID("other-tools"),
				ModuleID: &depID,
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandName{"other-tools test"}},
			},
		}

		err := CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{lock: lock})
		if err == nil {
			t.Fatal("CheckCommandDependenciesExist() error = nil, want forbidden dependency")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.ForbiddenCommands) != 1 {
			t.Fatalf("len(depErr.ForbiddenCommands) = %d, want 1", len(depErr.ForbiddenCommands))
		}
	})

	t.Run("rejects alias collision without locked module identity", func(t *testing.T) {
		t.Parallel()

		req := invowkmod.ModuleRequirement{
			GitURL:  "https://github.com/example/mono.git",
			Version: "^1.0.0",
			Path:    "modules/dep-tools",
			Alias:   "tools",
		}
		moduleDir := t.TempDir()
		lock := invowkmod.NewLockFile()
		lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
			GitURL:          req.GitURL,
			Version:         req.Version,
			ResolvedVersion: "1.2.3",
			GitCommit:       "0123456789abcdef0123456789abcdef01234567",
			Path:            req.Path,
			Alias:           req.Alias,
			Namespace:       "tools",
			ModuleID:        "io.example.expected",
			ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}
		callerMeta := invowkfile.NewModuleMetadataFromInvowkmod(&invowkfile.Invowkmod{
			Module:   "io.example.caller",
			Version:  "1.0.0",
			Requires: []invowkmod.ModuleRequirement{req},
		})
		callerInfo := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("build"),
			Command:    &invowkfile.Command{Name: "build"},
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(moduleDir), Metadata: callerMeta},
		}
		unrelatedID := invowkmod.ModuleID("io.example.unrelated")
		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{
				Name:     invowkfile.CommandName("tools build"),
				SourceID: discovery.SourceID("tools"),
				ModuleID: &unrelatedID,
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandName{"tools build"}},
			},
		}

		err := CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{lock: lock})
		if err == nil {
			t.Fatal("CheckCommandDependenciesExist() error = nil, want forbidden dependency")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.ForbiddenCommands) != 1 {
			t.Fatalf("len(depErr.ForbiddenCommands) = %d, want 1", len(depErr.ForbiddenCommands))
		}
		if !strings.Contains(depErr.ForbiddenCommands[0].String(), "module 'tools' is not accessible") {
			t.Fatalf("ForbiddenCommands[0] = %q", depErr.ForbiddenCommands[0])
		}
		if !strings.Contains(depErr.ForbiddenCommands[0].String(), "~/.invowk/cmds/") {
			t.Fatalf("ForbiddenCommands[0] missing user commands directory: %q", depErr.ForbiddenCommands[0])
		}
		if strings.Contains(depErr.ForbiddenCommands[0].String(), "~/.invowk/modules/") {
			t.Fatalf("ForbiddenCommands[0] points at module cache path: %q", depErr.ForbiddenCommands[0])
		}
	})

	t.Run("reports corrupt command scope lock", func(t *testing.T) {
		t.Parallel()

		moduleDir := t.TempDir()
		callerMeta := invowkfile.NewModuleMetadataFromInvowkmod(&invowkfile.Invowkmod{
			Module:  "io.example.caller",
			Version: "1.0.0",
		})
		callerInfo := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("build"),
			Command:    &invowkfile.Command{Name: "build"},
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(moduleDir), Metadata: callerMeta},
		}
		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{
				Name: invowkfile.CommandName("build"),
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandName{"build"}},
			},
		}

		providerErr := &CommandScopeLockError{
			Path: types.FilesystemPath(filepath.Join(moduleDir, invowkmod.LockFileName)),
			Err:  errors.New("corrupt lock"),
		}
		err := CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{err: providerErr})
		if err == nil {
			t.Fatal("CheckCommandDependenciesExist() error = nil, want lock error")
		}
		if !errors.Is(err, ErrCommandScopeLockLoadFailed) {
			t.Fatalf("errors.Is(err, ErrCommandScopeLockLoadFailed) = false for %v", err)
		}
		var lockErr *CommandScopeLockError
		if !errors.As(err, &lockErr) {
			t.Fatalf("errors.As(err, *CommandScopeLockError) = false for %v", err)
		}
	})
}

func TestFindMatchingCommand(t *testing.T) {
	t.Parallel()

	moduleID := invowkmod.ModuleID("io.example.mod")
	depID := invowkmod.ModuleID("io.example.dep")
	rootBuild := &discovery.CommandInfo{Name: invowkfile.CommandName("build")}
	moduleBuild := &discovery.CommandInfo{
		Name:     invowkfile.CommandName("mod build"),
		SourceID: discovery.SourceID("mod"),
		ModuleID: &moduleID,
	}
	depBuild := &discovery.CommandInfo{
		Name:     invowkfile.CommandName("dep build"),
		SourceID: discovery.SourceID("dep"),
		ModuleID: &depID,
	}

	t.Run("prefers module-local qualified command over root exact match", func(t *testing.T) {
		t.Parallel()

		available := map[invowkfile.CommandName]*discovery.CommandInfo{
			rootBuild.Name:   rootBuild,
			moduleBuild.Name: moduleBuild,
		}

		got := findMatchingCommand(available, "mod", []invowkfile.CommandName{"build"})
		if got != moduleBuild {
			t.Fatalf("findMatchingCommand() = %v, want module-local command", got)
		}
	})

	t.Run("root caller keeps exact root match", func(t *testing.T) {
		t.Parallel()

		available := map[invowkfile.CommandName]*discovery.CommandInfo{
			rootBuild.Name:   rootBuild,
			moduleBuild.Name: moduleBuild,
		}

		got := findMatchingCommand(available, "", []invowkfile.CommandName{"build"})
		if got != rootBuild {
			t.Fatalf("findMatchingCommand() = %v, want root command", got)
		}
	})

	t.Run("explicit qualified module command keeps exact match", func(t *testing.T) {
		t.Parallel()

		available := map[invowkfile.CommandName]*discovery.CommandInfo{
			moduleBuild.Name: moduleBuild,
			depBuild.Name:    depBuild,
		}

		got := findMatchingCommand(available, "mod", []invowkfile.CommandName{"dep build"})
		if got != depBuild {
			t.Fatalf("findMatchingCommand() = %v, want explicit dependency command", got)
		}
	})
}

func TestDiscoverAvailableCommands(t *testing.T) {
	t.Parallel()

	t.Run("uses execution context when present", func(t *testing.T) {
		t.Parallel()

		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{
				Set: &discovery.DiscoveredCommandSet{
					Commands: []*discovery.CommandInfo{
						{Name: invowkfile.CommandName("deploy")},
					},
				},
			},
		}
		ctx := &runtimepkg.ExecutionContext{
			Command: &invowkfile.Command{Name: "build"},
			Context: t.Context(),
		}

		available, err := discoverAvailableCommands(disc, ctx)
		if err != nil {
			t.Fatalf("discoverAvailableCommands() = %v", err)
		}
		if available[invowkfile.CommandName("deploy")] == nil {
			t.Fatalf("available missing deploy: %v", available)
		}
	})

	t.Run("wraps discovery failure", func(t *testing.T) {
		t.Parallel()

		disc := &stubCommandSetProvider{err: errors.New("boom")}
		_, err := discoverAvailableCommands(disc, nil)
		if err == nil {
			t.Fatal("discoverAvailableCommands() = nil, want error")
		}
		if !errors.Is(err, ErrDependencyDiscoveryFailed) {
			t.Fatalf("errors.Is(err, ErrDependencyDiscoveryFailed) = false for %v", err)
		}
	})
}

func TestValidateDependencies(t *testing.T) {
	t.Parallel()

	disc := &stubCommandSetProvider{
		result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}},
	}

	t.Run("no deps passes both phases", func(t *testing.T) {
		t.Parallel()

		cmd := invowkfiletest.NewTestCommand("build", invowkfiletest.WithScript("echo hello"))
		cmdInfo := &discovery.CommandInfo{
			Name:       cmd.Name,
			Command:    cmd,
			Invowkfile: &invowkfile.Invowkfile{},
		}
		execCtx := &runtimepkg.ExecutionContext{
			Command:         cmd,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeVirtual,
			SelectedImpl:    &cmd.Implementations[0],
		}

		if err := ValidateDependencies(disc, cmdInfo, execCtx, nil); err != nil {
			t.Fatalf("ValidateDependencies() = %v", err)
		}
	})

	t.Run("host failure short-circuits before runtime phase", func(t *testing.T) {
		t.Parallel()

		cmd := &invowkfile.Command{
			Name: "deploy",
			DependsOn: &invowkfile.DependsOn{
				Tools: []invowkfile.ToolDependency{
					{Alternatives: []invowkfile.BinaryName{"___nonexistent_tool_for_test___"}},
				},
			},
			Implementations: []invowkfile.Implementation{{
				Script: "echo deploy",
				Runtimes: []invowkfile.RuntimeConfig{
					{
						Name: invowkfile.RuntimeContainer,
						DependsOn: &invowkfile.DependsOn{
							Tools: []invowkfile.ToolDependency{
								{Alternatives: []invowkfile.BinaryName{"also-missing"}},
							},
						},
					},
				},
			}},
		}
		cmdInfo := &discovery.CommandInfo{
			Name:       cmd.Name,
			Command:    cmd,
			Invowkfile: &invowkfile.Invowkfile{},
		}
		execCtx := &runtimepkg.ExecutionContext{
			Command:         cmd,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeContainer,
			SelectedImpl:    &cmd.Implementations[0],
		}

		err := ValidateDependenciesWithHostProbe(
			disc,
			cmdInfo,
			nil,
			execCtx,
			nil,
			nil,
			&recordingHostProbe{
				toolErrors: map[invowkfile.BinaryName]error{
					"___nonexistent_tool_for_test___": errors.New("missing host tool"),
				},
			},
		)
		if err == nil {
			t.Fatal("expected host dependency error")
		}

		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("expected *DependencyError, got %T", err)
		}
		if len(depErr.MissingTools) != 1 {
			t.Fatalf("expected exactly 1 MissingTools (host only, phase 2 skipped), got %d", len(depErr.MissingTools))
		}
	})

	t.Run("non-container runtime skips phase 2", func(t *testing.T) {
		t.Parallel()

		cmd := &invowkfile.Command{
			Name: "lint",
			Implementations: []invowkfile.Implementation{{
				Script: "echo lint",
				Runtimes: []invowkfile.RuntimeConfig{
					{
						Name: invowkfile.RuntimeContainer,
						DependsOn: &invowkfile.DependsOn{
							Tools: []invowkfile.ToolDependency{
								{Alternatives: []invowkfile.BinaryName{"container-only-tool"}},
							},
						},
					},
				},
			}},
		}
		cmdInfo := &discovery.CommandInfo{
			Name:       cmd.Name,
			Command:    cmd,
			Invowkfile: &invowkfile.Invowkfile{},
		}
		execCtx := &runtimepkg.ExecutionContext{
			Command:         cmd,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeVirtual,
			SelectedImpl:    &cmd.Implementations[0],
		}

		if err := ValidateDependencies(disc, cmdInfo, execCtx, nil); err != nil {
			t.Fatalf("ValidateDependencies() = %v, expected nil (phase 2 skipped for non-container)", err)
		}
	})

	t.Run("host capability checker is injectable", func(t *testing.T) {
		t.Parallel()

		cmd := &invowkfile.Command{
			Name: "net",
			DependsOn: &invowkfile.DependsOn{
				Capabilities: []invowkfile.CapabilityDependency{
					{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityInternet}},
				},
			},
			Implementations: []invowkfile.Implementation{{
				Script:   "echo net",
				Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
			}},
		}
		cmdInfo := &discovery.CommandInfo{
			Name:       cmd.Name,
			Command:    cmd,
			Invowkfile: &invowkfile.Invowkfile{},
		}
		execCtx := &runtimepkg.ExecutionContext{
			Command:         cmd,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeVirtual,
			SelectedImpl:    &cmd.Implementations[0],
		}

		err := ValidateDependenciesWithCapabilityChecker(disc, cmdInfo, nil, execCtx, nil,
			fakeCapabilityChecker{
				invowkfile.CapabilityInternet: errors.New("offline"),
			},
		)
		if err == nil {
			t.Fatal("expected injected capability checker error")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.MissingCapabilities) != 1 {
			t.Fatalf("len(MissingCapabilities) = %d, want 1", len(depErr.MissingCapabilities))
		}
	})
}

func TestValidateHostDependenciesWithHostProbeUsesInjectedProbe(t *testing.T) {
	t.Parallel()

	invowkfilePath := filepath.Join(t.TempDir(), "work", "invowkfile.cue")
	expectedFilepath := filepath.Join(filepath.Dir(invowkfilePath), "data", "input.txt")

	cmd := &invowkfile.Command{
		Name: "build",
		DependsOn: &invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{{
				Alternatives: []invowkfile.BinaryName{"tool-a"},
			}},
			Filepaths: []invowkfile.FilepathDependency{{
				Alternatives: []invowkfile.FilesystemPath{"data/input.txt"},
				Readable:     true,
			}},
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Name:        "custom",
				CheckScript: "exit 0",
			}},
		},
		Implementations: []invowkfile.Implementation{{
			Script:   "echo ok",
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
		}},
	}
	cmdInfo := &discovery.CommandInfo{
		Name:       cmd.Name,
		Command:    cmd,
		Invowkfile: &invowkfile.Invowkfile{FilePath: types.FilesystemPath(invowkfilePath)},
	}
	execCtx := &runtimepkg.ExecutionContext{
		Command:      cmd,
		Context:      t.Context(),
		SelectedImpl: &cmd.Implementations[0],
	}
	probe := &recordingHostProbe{}

	err := ValidateHostDependenciesWithHostProbe(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}}},
		cmdInfo,
		execCtx,
		map[string]string{},
		nil,
		probe,
	)
	if err != nil {
		t.Fatalf("ValidateHostDependenciesWithHostProbe() = %v", err)
	}
	if len(probe.tools) != 1 || probe.tools[0] != "tool-a" {
		t.Fatalf("probe tools = %v, want [tool-a]", probe.tools)
	}
	if len(probe.filepaths) != 1 || probe.filepaths[0] != types.FilesystemPath(expectedFilepath) {
		t.Fatalf("probe filepaths = %v, want resolved path", probe.filepaths)
	}
	if len(probe.checks) != 1 || probe.checks[0] != "custom" {
		t.Fatalf("probe checks = %v, want [custom]", probe.checks)
	}
}

func TestValidateRuntimeDependencies(t *testing.T) {
	t.Parallel()

	cmd := &invowkfile.Command{
		Name: "build",
		Implementations: []invowkfile.Implementation{{
			Script: "echo hello",
			Runtimes: []invowkfile.RuntimeConfig{
				{
					Name: invowkfile.RuntimeContainer,
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []invowkfile.CommandName{"build"}},
						},
					},
				},
			},
		}},
	}
	cmdInfo := &discovery.CommandInfo{
		Name:    cmd.Name,
		Command: cmd,
	}

	t.Run("non-container runtime is a no-op", func(t *testing.T) {
		t.Parallel()

		err := ValidateRuntimeDependencies(cmdInfo, nil, &runtimepkg.ExecutionContext{
			Command:         cmd,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeVirtual,
			SelectedImpl:    &cmd.Implementations[0],
		})
		if err != nil {
			t.Fatalf("ValidateRuntimeDependencies() = %v", err)
		}
	})

	t.Run("container runtime delegates to container checks", func(t *testing.T) {
		t.Parallel()

		probe := &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				if strings.Contains(string(ctx.SelectedImpl.Script), "check-cmd 'build'") {
					return &runtimepkg.Result{ExitCode: 0}
				}
				return &runtimepkg.Result{ExitCode: 1}
			},
		}

		err := ValidateRuntimeDependencies(cmdInfo, probe, &runtimepkg.ExecutionContext{
			Command:         cmd,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeContainer,
			SelectedImpl:    &cmd.Implementations[0],
		})
		if err != nil {
			t.Fatalf("ValidateRuntimeDependencies() = %v", err)
		}
	})
}
