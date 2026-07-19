// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
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
		checkScripts   []invowkfile.ScriptContent
		checkInterps   []invowkfile.InterpreterSpec
		checkErrors    map[invowkfile.CheckName]error
		checkResults   map[invowkfile.CheckName]CustomCheckResult
	}

	staticCommandScopeLockProvider struct {
		lock *invowkmod.LockFile
		err  error
	}
)

func mustModuleMetadata(t *testing.T, meta *invowkfile.Invowkmod) *invowkfile.ModuleMetadata {
	t.Helper()
	metadata, err := invowkfile.NewModuleMetadataFromInvowkmod(meta)
	if err != nil {
		t.Fatalf("NewModuleMetadataFromInvowkmod() error = %v", err)
	}
	return metadata
}

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

func (p *recordingHostProbe) RunCustomCheck(_ context.Context, check invowkfile.CustomCheck) (CustomCheckResult, error) {
	p.checks = append(p.checks, check.Name)
	p.checkScripts = append(p.checkScripts, check.Script.Content)
	p.checkInterps = append(p.checkInterps, check.Script.Interpreter)
	if p.checkErrors != nil {
		return CustomCheckResult{}, p.checkErrors[check.Name]
	}
	if p.checkResults != nil {
		return p.checkResults[check.Name], nil
	}
	return CustomCheckResult{}, nil
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

func testDependencyExecutionContext(t testing.TB, cmd *invowkfile.Command, selectedRuntime invowkfile.RuntimeMode) ExecutionContext {
	t.Helper()

	ctx := ExecutionContext{
		Context:         t.Context(),
		SelectedRuntime: selectedRuntime,
	}
	if cmd == nil {
		return ctx
	}
	ctx.CommandName = cmd.Name
	if len(cmd.Implementations) == 0 {
		return ctx
	}
	impl := &cmd.Implementations[0]
	ctx.ImplementationDependsOn = impl.DependsOn
	if rt := invowkfile.FindRuntimeConfig(impl.Runtimes, selectedRuntime); rt != nil {
		ctx.RuntimeDependsOn = rt.DependsOn
	}
	return ctx
}

func TestCheckCommandDependenciesExist(t *testing.T) {
	t.Parallel()

	ctx := testDependencyExecutionContext(t, &invowkfile.Command{Name: "build"}, "")

	// Root invowkfile cmdInfo — no module metadata, no scope enforcement.
	rootCmdInfo := &discovery.CommandInfo{
		Name:       invowkfile.CommandName("build"),
		Command:    &invowkfile.Command{Name: "build"},
		Invowkfile: &invowkfile.Invowkfile{},
	}

	// Module cmdInfo with module "mod" — for qualified name lookup.
	modMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:  "mod",
		Version: "1.0.0",
	})
	modID := modMeta.Module()
	modCmdInfo := &discovery.CommandInfo{
		Name:       invowkfile.CommandName("build"),
		SourceID:   discovery.SourceID("mod"),
		ModuleID:   &modID,
		Command:    &invowkfile.Command{Name: "build"},
		Invowkfile: &invowkfile.Invowkfile{Metadata: modMeta},
	}

	t.Run("accepts module-local bare alternatives", func(t *testing.T) {
		t.Parallel()

		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{
				{Name: invowkfile.CommandName("deploy")},
				{Name: invowkfile.CommandName("mod build"), SimpleName: "build", SourceID: discovery.SourceID("mod"), ModuleID: &modID},
			},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"build"}},
			},
		}

		// Module cmdInfo: bare "build" resolves only within the caller's source.
		if err := CheckCommandDependenciesExist(disc, deps, modCmdInfo, ctx); err != nil {
			t.Fatalf("CheckCommandDependenciesExist() = %v", err)
		}
	})

	t.Run("rejects module fallback to root invowkfile command", func(t *testing.T) {
		t.Parallel()

		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{
				{Name: invowkfile.CommandName("deploy")},
			},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"deploy"}},
			},
		}

		err := CheckCommandDependenciesExist(disc, deps, modCmdInfo, ctx)
		if err == nil {
			t.Fatal("CheckCommandDependenciesExist() error = nil, want root command to be inaccessible")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(err, *DependencyError) = false for %v", err)
		}
		if len(depErr.MissingCommands) != 1 {
			t.Fatalf("MissingCommands = %v, want one inaccessible root command", depErr.MissingCommands)
		}
	})

	t.Run("rejects module qualified access to root invowkfile command", func(t *testing.T) {
		t.Parallel()

		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{
				{
					Name:       invowkfile.CommandName("deploy"),
					SimpleName: "deploy",
					SourceID:   discovery.SourceIDInvowkfile,
				},
			},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"@invowkfile deploy"}},
			},
		}

		err := CheckCommandDependenciesExist(disc, deps, modCmdInfo, ctx)
		if err == nil {
			t.Fatal("CheckCommandDependenciesExist() error = nil, want root source to be inaccessible")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(err, *DependencyError) = false for %v", err)
		}
		if len(depErr.ForbiddenCommands) != 1 {
			t.Fatalf("ForbiddenCommands = %v, want one forbidden root command", depErr.ForbiddenCommands)
		}
	})

	t.Run("reports missing alternatives", func(t *testing.T) {
		t.Parallel()

		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"missing", "other"}},
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

	t.Run("distinguishes unknown qualified source from missing command", func(t *testing.T) {
		t.Parallel()

		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{
				Name:       invowkfile.CommandName("tools fmt"),
				SimpleName: "fmt",
				SourceID:   discovery.SourceID("tools"),
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}

		tests := []struct {
			name string
			ref  invowkfile.CommandDependencyRef
			want string
		}{
			{
				name: "unknown source",
				ref:  "@missing-tools lint",
				want: `@missing-tools lint - source "missing-tools" not found`,
			},
			{
				name: "missing command in known source",
				ref:  "@tools lint",
				want: `@tools lint - command "lint" not found in source "tools"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				deps := &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{
						{Alternatives: []invowkfile.CommandDependencyRef{tt.ref}},
					},
				}

				err := CheckCommandDependenciesExist(disc, deps, rootCmdInfo, ctx)
				if err == nil {
					t.Fatal("CheckCommandDependenciesExist() error = nil, want missing command dependency")
				}
				var depErr *DependencyError
				if !errors.As(err, &depErr) {
					t.Fatalf("errors.As(*DependencyError) = false for %T", err)
				}
				if len(depErr.MissingCommands) != 1 {
					t.Fatalf("len(depErr.MissingCommands) = %d, want 1", len(depErr.MissingCommands))
				}
				if !strings.Contains(depErr.MissingCommands[0].String(), tt.want) {
					t.Fatalf("depErr.MissingCommands[0] = %q, want %q", depErr.MissingCommands[0], tt.want)
				}
			})
		}
	})

	t.Run("allows locked non-aliased direct dependency module id", func(t *testing.T) {
		t.Parallel()

		depID := invowkmod.ModuleID("io.example.dep")
		req := invowkmod.ModuleRequirement{
			GitURL:  "https://github.com/example/mono.git",
			Version: "^1.0.0",
			Path:    "modules/io.example.dep.invowkmod",
		}
		moduleDir := t.TempDir()
		lock := invowkmod.NewLockFile()
		lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
			GitURL:          req.GitURL,
			Version:         req.Version,
			ResolvedVersion: "1.2.3",
			GitCommit:       "0123456789abcdef0123456789abcdef01234567",
			Path:            req.Path,
			Namespace:       "io.example.dep@1.2.3",
			ModuleID:        depID,
			CommandSourceID: invowkmod.ModuleSourceID(depID),
			ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}
		callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
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
				Name:       invowkfile.CommandName("io.example.dep test"),
				SimpleName: "test",
				SourceID:   discovery.SourceID(depID),
				ModuleID:   &depID,
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"@io.example.dep test"}},
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
		callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
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
				Name:       invowkfile.CommandName("other-tools test"),
				SimpleName: "test",
				SourceID:   discovery.SourceID("other-tools"),
				ModuleID:   &depID,
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"@other-tools test"}},
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
		callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
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
				Name:       invowkfile.CommandName("tools build"),
				SimpleName: "build",
				SourceID:   discovery.SourceID("tools"),
				ModuleID:   &unrelatedID,
			}},
		}
		disc := &stubCommandSetProvider{
			result: discovery.CommandSetResult{Set: commandSet},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"@tools build"}},
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
		callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
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
				{Alternatives: []invowkfile.CommandDependencyRef{"build"}},
			},
		}

		providerErr := &CommandScopeLockError{
			Path: filepath.Join(moduleDir, invowkmod.LockFileName),
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

	tests := []struct {
		name      string
		available map[invowkfile.CommandName]*discovery.CommandInfo
		sourceID  invowkmod.ModuleSourceID
		ref       invowkfile.CommandDependencyRef
		want      *discovery.CommandInfo
	}{
		{name: "prefers module-local qualified command over root exact match", sourceID: "mod", ref: "build", want: moduleBuild, available: map[invowkfile.CommandName]*discovery.CommandInfo{
			rootBuild.Name:   rootBuild,
			moduleBuild.Name: moduleBuild,
		}},
		{name: "root caller keeps exact root match", ref: "build", want: rootBuild, available: map[invowkfile.CommandName]*discovery.CommandInfo{
			rootBuild.Name:   rootBuild,
			moduleBuild.Name: moduleBuild,
		}},
		{name: "module caller cannot match root exact command", sourceID: "mod", ref: "build", available: map[invowkfile.CommandName]*discovery.CommandInfo{
			rootBuild.Name: rootBuild,
		}},
		{name: "explicit qualified module command keeps exact match", sourceID: "mod", ref: "@dep build", want: depBuild, available: map[invowkfile.CommandName]*discovery.CommandInfo{
			moduleBuild.Name: moduleBuild,
			depBuild.Name:    depBuild,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findMatchingCommand(tt.available, tt.sourceID, commandDependencyAlternativesForTest(t, tt.ref))
			if got != tt.want {
				t.Fatalf("findMatchingCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func commandDependencyAlternativesForTest(t testing.TB, refs ...invowkfile.CommandDependencyRef) []commandDependencyAlternative {
	t.Helper()
	alternatives := normalizedCommandAlternatives(invowkfile.CommandDependency{Alternatives: refs})
	if len(alternatives) != len(refs) {
		t.Fatalf("normalizedCommandAlternatives(%v) returned %d refs, want %d", refs, len(alternatives), len(refs))
	}
	return alternatives
}

func TestDiscoverAvailableCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		provider    *stubCommandSetProvider
		ctx         ExecutionContext
		wantCommand invowkfile.CommandName
		wantIs      error
	}{
		{name: "uses execution context when present", wantCommand: "deploy", provider: &stubCommandSetProvider{
			result: discovery.CommandSetResult{
				Set: &discovery.DiscoveredCommandSet{
					Commands: []*discovery.CommandInfo{
						{Name: invowkfile.CommandName("deploy")},
					},
				},
			},
		}, ctx: testDependencyExecutionContext(t, &invowkfile.Command{Name: "build"}, "")},
		{name: "wraps discovery failure", provider: &stubCommandSetProvider{err: errors.New("boom")}, wantIs: ErrDependencyDiscoveryFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			available, err := discoverAvailableCommands(tt.provider, tt.ctx)
			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("error = %v, want wrapping %v", err, tt.wantIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("discoverAvailableCommands() = %v", err)
			}
			if available[tt.wantCommand] == nil {
				t.Fatalf("available missing %q: %v", tt.wantCommand, available)
			}
		})
	}
}

func TestValidateDependencies(t *testing.T) {
	t.Parallel()

	disc := &stubCommandSetProvider{
		result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}},
	}

	tests := []struct {
		name             string
		cmd              *invowkfile.Command
		runtime          invowkfile.RuntimeMode
		validationMode   string
		wantMissingTools int
		wantMissingCaps  int
	}{
		{name: "no deps passes both phases", cmd: invowkfiletest.NewTestCommand("build", invowkfiletest.WithScript("echo hello")), runtime: invowkfile.RuntimeVirtualSh},
		{name: "host failure short-circuits before runtime phase", validationMode: "host-probe", runtime: invowkfile.RuntimeContainer, wantMissingTools: 1, cmd: &invowkfile.Command{
			Name: "deploy",
			DependsOn: &invowkfile.DependsOn{
				Tools: []invowkfile.ToolDependency{
					{Alternatives: []invowkfile.BinaryName{"___nonexistent_tool_for_test___"}},
				},
			},
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{Content: "echo deploy"},
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
		}},
		{name: "non-container runtime skips phase 2", runtime: invowkfile.RuntimeVirtualSh, cmd: &invowkfile.Command{
			Name: "lint",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{Content: "echo lint"},
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
		}},
		{name: "host capability checker is injectable", validationMode: "capability", runtime: invowkfile.RuntimeVirtualSh, wantMissingCaps: 1, cmd: &invowkfile.Command{
			Name: "net",
			DependsOn: &invowkfile.DependsOn{
				Capabilities: []invowkfile.CapabilityDependency{
					{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityInternet}},
				},
			},
			Implementations: []invowkfile.Implementation{{
				Script:   invowkfile.ImplementationScript{Content: "echo net"},
				Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtualSh}},
			}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmdInfo := &discovery.CommandInfo{Name: tt.cmd.Name, Command: tt.cmd, Invowkfile: &invowkfile.Invowkfile{}}
			execCtx := testDependencyExecutionContext(t, tt.cmd, tt.runtime)
			var err error
			switch tt.validationMode {
			case "":
				err = ValidateDependencies(disc, cmdInfo, execCtx, nil)
			case "host-probe":
				err = ValidateDependenciesWithHostProbe(disc, cmdInfo, nil, execCtx, nil, nil, &recordingHostProbe{
					toolErrors: map[invowkfile.BinaryName]error{"___nonexistent_tool_for_test___": errors.New("missing host tool")},
				})
			case "capability":
				err = ValidateDependenciesWithCapabilityChecker(disc, cmdInfo, nil, execCtx, nil, fakeCapabilityChecker{
					invowkfile.CapabilityInternet: errors.New("offline"),
				})
			default:
				t.Fatalf("unknown validation mode %q", tt.validationMode)
			}
			wantErr := tt.wantMissingTools > 0 || tt.wantMissingCaps > 0
			if (err != nil) != wantErr {
				t.Fatalf("ValidateDependencies() error = %v, wantErr %v", err, wantErr)
			}
			if wantErr {
				var depErr *DependencyError
				if !errors.As(err, &depErr) {
					t.Fatalf("expected *DependencyError, got %T", err)
				}
				if len(depErr.MissingTools) != tt.wantMissingTools {
					t.Fatalf("len(MissingTools) = %d, want %d", len(depErr.MissingTools), tt.wantMissingTools)
				}
				if len(depErr.MissingCapabilities) != tt.wantMissingCaps {
					t.Fatalf("len(MissingCapabilities) = %d, want %d", len(depErr.MissingCapabilities), tt.wantMissingCaps)
				}
			}
		})
	}
}
