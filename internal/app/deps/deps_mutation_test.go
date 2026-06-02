// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	depsMutationCommand         invowkfile.CommandName     = "build"
	depsMutationSource          discovery.SourceID         = "tools"
	depsMutationSimple          invowkfile.CommandName     = "lint"
	depsMutationModuleID        invowkmod.ModuleID         = "io.example.tools"
	depsMutationCallerID        invowkmod.ModuleID         = "io.example.caller"
	depsMutationGitURL          invowkmod.GitURL           = "https://github.com/example/tools.git"
	depsMutationVersion         invowkmod.SemVerConstraint = "^1.0.0"
	depsMutationResolvedVersion                            = "1.2.3"
	depsMutationGitCommit                                  = "0123456789abcdef0123456789abcdef01234567"
	depsMutationContentHash                                = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type (
	depsMutationContextKey struct{}

	panicCommandSetProvider struct {
		t *testing.T
	}

	contextRecordingCommandSetProvider struct {
		want   context.Context
		called bool
	}

	recordingRuntimeProbe struct {
		tools        []invowkfile.BinaryName
		filepaths    []invowkfile.FilepathDependency
		envVars      []invowkfile.EnvVarCheck
		capabilities []invowkfile.CapabilityName
		commands     []invowkfile.CommandName
		checks       []invowkfile.CustomCheck
		envErr       error
		toolErr      error
	}
)

func (p panicCommandSetProvider) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	p.t.Helper()
	p.t.Fatal("DiscoverCommandSet should not be called")
	return discovery.CommandSetResult{}, nil
}

func (p *contextRecordingCommandSetProvider) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	p.called = true
	if ctx != p.want {
		return discovery.CommandSetResult{}, errors.New("unexpected context")
	}
	return discovery.CommandSetResult{
		Set: &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{Name: depsMutationCommand}},
		},
	}, nil
}

func (p *recordingRuntimeProbe) CheckTool(tool invowkfile.BinaryName) error {
	p.tools = append(p.tools, tool)
	return p.toolErr
}

func (p *recordingRuntimeProbe) CheckFilepath(fp invowkfile.FilepathDependency) error {
	p.filepaths = append(p.filepaths, fp)
	return nil
}

func (p *recordingRuntimeProbe) CheckEnvVar(envVar invowkfile.EnvVarCheck) error {
	p.envVars = append(p.envVars, envVar)
	return p.envErr
}

func (p *recordingRuntimeProbe) CheckCapability(capability invowkfile.CapabilityName) error {
	p.capabilities = append(p.capabilities, capability)
	return nil
}

func (p *recordingRuntimeProbe) CheckCommand(command invowkfile.CommandName) error {
	p.commands = append(p.commands, command)
	return nil
}

func (p *recordingRuntimeProbe) RunCustomCheck(check invowkfile.CustomCheck) (CustomCheckResult, error) {
	p.checks = append(p.checks, check)
	return CustomCheckResult{}, nil
}

func TestValidateDependenciesMutationWrappers(t *testing.T) {
	t.Parallel()

	cmd := runtimeDependencyCommand(invowkfile.CommandDependencyRef(depsMutationCommand))
	cmdInfo := runtimeDependencyCommandInfo(cmd)
	ctx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer)
	disc := &stubCommandSetProvider{
		result: discovery.CommandSetResult{
			Set: &discovery.DiscoveredCommandSet{
				Commands: []*discovery.CommandInfo{{Name: depsMutationCommand}},
			},
		},
	}

	for _, tt := range []struct {
		name string
		call func() error
	}{
		{
			name: "public wrapper returns runtime dependency failure",
			call: func() error {
				return ValidateDependencies(disc, cmdInfo, ctx, nil)
			},
		},
		{
			name: "ports wrapper returns runtime dependency failure",
			call: func() error {
				return ValidateDependenciesWithPorts(disc, cmdInfo, nil, ctx, nil, nil, nil, nil)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call()
			if !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
				t.Fatalf("%s error = %v, want ErrRuntimeDependencyProbeRequired", tt.name, err)
			}
		})
	}
}

func TestValidateHostDependenciesMutationShortCircuit(t *testing.T) {
	t.Parallel()

	t.Run("missing env vars stop before host probes", func(t *testing.T) {
		t.Parallel()

		cmd := depsMutationHostCommand(&invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{{
				Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING_ENV"}},
			}},
			Tools: []invowkfile.ToolDependency{{
				Alternatives: []invowkfile.BinaryName{"tool-after-env"},
			}},
		})
		probe := &recordingHostProbe{}
		err := ValidateHostDependenciesWithHostProbe(
			&stubCommandSetProvider{},
			depsMutationCommandInfo(cmd, nil),
			testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative),
			map[string]string{},
			nil,
			probe,
		)
		depErr := requireDependencyError(t, err)
		if len(depErr.MissingEnvVars) != 1 {
			t.Fatalf("MissingEnvVars = %v, want one missing env var", depErr.MissingEnvVars)
		}
		if len(probe.tools) != 0 || len(probe.filepaths) != 0 || len(probe.checks) != 0 {
			t.Fatalf("host probe was called after env failure: %+v", probe)
		}
	})

	t.Run("filepath failure stops before custom checks and command discovery", func(t *testing.T) {
		t.Parallel()

		invowkfilePath := types.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue"))
		resolvedPath := types.FilesystemPath(filepath.Join(filepath.Dir(string(invowkfilePath)), "missing.txt"))
		cmd := depsMutationHostCommand(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{
				Alternatives: []invowkfile.FilesystemPath{"missing.txt"},
			}},
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Name:   "after-filepath",
				Script: invowkfile.CustomCheckScript{Content: "exit 0"},
			}},
			Commands: []invowkfile.CommandDependency{{
				Alternatives: []invowkfile.CommandDependencyRef{invowkfile.CommandDependencyRef(depsMutationCommand)},
			}},
		})
		probe := &recordingHostProbe{
			filepathErrors: map[types.FilesystemPath]error{
				resolvedPath: errors.New("missing file"),
			},
		}
		err := ValidateHostDependenciesWithHostProbe(
			panicCommandSetProvider{t: t},
			depsMutationCommandInfo(cmd, &invowkfile.Invowkfile{FilePath: invowkfilePath}),
			testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative),
			map[string]string{},
			nil,
			probe,
		)
		depErr := requireDependencyError(t, err)
		if len(depErr.MissingFilepaths) != 1 {
			t.Fatalf("MissingFilepaths = %v, want one missing filepath", depErr.MissingFilepaths)
		}
		if len(probe.checks) != 0 {
			t.Fatalf("custom checks ran after filepath failure: %v", probe.checks)
		}
	})
}

func TestValidateRuntimeDependenciesMutationBoundaries(t *testing.T) {
	t.Parallel()

	cmd := runtimeDependencyCommand(invowkfile.CommandDependencyRef(depsMutationCommand))
	cmdInfo := runtimeDependencyCommandInfo(cmd)

	t.Run("container nil and empty runtime deps do not require a probe", func(t *testing.T) {
		t.Parallel()

		ctx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer)
		ctx.RuntimeDependsOn = nil
		if err := ValidateRuntimeDependencies(panicCommandSetProvider{t: t}, cmdInfo, nil, ctx, nil); err != nil {
			t.Fatalf("nil RuntimeDependsOn error = %v, want nil", err)
		}

		ctx.RuntimeDependsOn = &invowkfile.DependsOn{}
		if err := ValidateRuntimeDependencies(panicCommandSetProvider{t: t}, cmdInfo, nil, ctx, nil); err != nil {
			t.Fatalf("empty RuntimeDependsOn error = %v, want nil", err)
		}
	})

	t.Run("runtime env failure stops before later runtime probes", func(t *testing.T) {
		t.Parallel()

		envErr := errors.New("env unavailable")
		probe := &recordingRuntimeProbe{envErr: envErr}
		ctx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer)
		ctx.RuntimeDependsOn = &invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{{
				Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING_ENV"}},
			}},
			Tools: []invowkfile.ToolDependency{{
				Alternatives: []invowkfile.BinaryName{"tool-after-env"},
			}},
		}

		err := ValidateRuntimeDependencies(
			&stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}}},
			cmdInfo,
			probe,
			ctx,
			nil,
		)
		depErr := requireDependencyError(t, err)
		if len(depErr.MissingEnvVars) != 1 {
			t.Fatalf("MissingEnvVars = %v, want one runtime env failure", depErr.MissingEnvVars)
		}
		if len(probe.tools) != 0 || len(probe.filepaths) != 0 || len(probe.commands) != 0 {
			t.Fatalf("runtime probe continued after env failure: %+v", probe)
		}
	})
}

func TestCommandResolutionMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("nil and empty command deps do not discover", func(t *testing.T) {
		t.Parallel()

		resolved, err := resolveCommandDependenciesWithLockProvider(panicCommandSetProvider{t: t}, nil, nil, ExecutionContext{}, nil)
		if err != nil {
			t.Fatalf("nil deps error = %v, want nil", err)
		}
		if resolved != nil {
			t.Fatalf("nil deps resolved = %v, want nil", resolved)
		}

		resolved, err = resolveCommandDependenciesWithLockProvider(
			panicCommandSetProvider{t: t},
			&invowkfile.DependsOn{},
			nil,
			ExecutionContext{},
			nil,
		)
		if err != nil {
			t.Fatalf("empty deps error = %v, want nil", err)
		}
		if resolved != nil {
			t.Fatalf("empty deps resolved = %v, want nil", resolved)
		}
	})

	t.Run("resolved command records matched discovery name and original alternatives", func(t *testing.T) {
		t.Parallel()

		available := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{Name: depsMutationCommand}},
		}
		resolved, err := resolveCommandDependenciesWithLockProvider(
			&stubCommandSetProvider{result: discovery.CommandSetResult{Set: available}},
			&invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{
				Alternatives: []invowkfile.CommandDependencyRef{invowkfile.CommandDependencyRef(depsMutationCommand)},
			}}},
			&discovery.CommandInfo{Invowkfile: &invowkfile.Invowkfile{}},
			ExecutionContext{CommandName: depsMutationCommand},
			nil,
		)
		if err != nil {
			t.Fatalf("resolveCommandDependenciesWithLockProvider() error = %v", err)
		}
		if len(resolved) != 1 {
			t.Fatalf("resolved length = %d, want 1", len(resolved))
		}
		if resolved[0].Command == nil || *resolved[0].Command != depsMutationCommand {
			t.Fatalf("resolved command = %v, want %q", resolved[0].Command, depsMutationCommand)
		}
		if len(resolved[0].Alternatives) != 1 || resolved[0].Alternatives[0] != invowkfile.CommandDependencyRef(depsMutationCommand) {
			t.Fatalf("resolved alternatives = %v, want original ref", resolved[0].Alternatives)
		}
	})

	t.Run("discover uses execution context value", func(t *testing.T) {
		t.Parallel()

		wantCtx := context.WithValue(t.Context(), depsMutationContextKey{}, "sentinel")
		provider := &contextRecordingCommandSetProvider{want: wantCtx}
		available, err := discoverAvailableCommands(provider, ExecutionContext{Context: wantCtx})
		if err != nil {
			t.Fatalf("discoverAvailableCommands() error = %v", err)
		}
		if !provider.called {
			t.Fatal("provider was not called")
		}
		if available[depsMutationCommand] == nil {
			t.Fatalf("available commands = %v, want %q", available, depsMutationCommand)
		}
	})
}

func TestCommandScopeMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("lock provider fallback only loads for module paths", func(t *testing.T) {
		t.Parallel()

		providerErr := &CommandScopeLockError{Path: "invowkmod.lock.cue", Err: errors.New("corrupt")}
		provider := staticCommandScopeLockProvider{err: providerErr}

		lock, err := commandScopeLock(provider, nil)
		if err != nil {
			t.Fatalf("nil inv lock error = %v, want nil", err)
		}
		if lock == nil {
			t.Fatal("nil inv lock = nil, want empty lock")
		}

		lock, err = commandScopeLock(provider, &invowkfile.Invowkfile{})
		if err != nil {
			t.Fatalf("empty module path lock error = %v, want nil", err)
		}
		if lock == nil {
			t.Fatal("empty module path lock = nil, want empty lock")
		}

		_, err = commandScopeLock(provider, &invowkfile.Invowkfile{ModulePath: "module.invowkmod"})
		if !errors.Is(err, ErrCommandScopeLockLoadFailed) {
			t.Fatalf("module path lock error = %v, want ErrCommandScopeLockLoadFailed", err)
		}
	})

	t.Run("direct requirement matching requires command identity source and lock", func(t *testing.T) {
		t.Parallel()

		req, lock := depsMutationRequirementAndLock()
		matchingID := depsMutationModuleID
		matching := &discovery.CommandInfo{
			SourceID: depsMutationSource,
			ModuleID: &matchingID,
		}
		if !commandMatchesDirectRequirement([]invowkmod.ModuleRequirement{req}, lock, matching) {
			t.Fatal("commandMatchesDirectRequirement() = false, want true for matching lock identity")
		}
		if commandMatchesDirectRequirement([]invowkmod.ModuleRequirement{req}, nil, matching) {
			t.Fatal("commandMatchesDirectRequirement() = true with nil lock")
		}
		if commandMatchesDirectRequirement([]invowkmod.ModuleRequirement{req}, lock, nil) {
			t.Fatal("commandMatchesDirectRequirement() = true with nil command")
		}
		if commandMatchesDirectRequirement([]invowkmod.ModuleRequirement{req}, lock, &discovery.CommandInfo{SourceID: depsMutationSource}) {
			t.Fatal("commandMatchesDirectRequirement() = true with nil module ID")
		}
		otherID := invowkmod.ModuleID("io.example.other")
		if commandMatchesDirectRequirement([]invowkmod.ModuleRequirement{req}, lock, &discovery.CommandInfo{SourceID: depsMutationSource, ModuleID: &otherID}) {
			t.Fatal("commandMatchesDirectRequirement() = true for mismatched module ID")
		}
	})

	t.Run("accessible command reports allowed forbidden and root decisions", func(t *testing.T) {
		t.Parallel()

		allowedID := depsMutationModuleID
		allowed := &discovery.CommandInfo{
			Name:       "tools lint",
			SimpleName: depsMutationSimple,
			SourceID:   depsMutationSource,
			ModuleID:   &allowedID,
		}
		blockedID := invowkmod.ModuleID("io.example.blocked")
		blocked := &discovery.CommandInfo{
			Name:       "blocked lint",
			SimpleName: depsMutationSimple,
			SourceID:   "blocked",
			ModuleID:   &blockedID,
		}
		available := map[invowkfile.CommandName]*discovery.CommandInfo{
			allowed.Name: allowed,
			blocked.Name: blocked,
		}
		scope := invowkmod.NewCommandScope(depsMutationCallerID)
		scope.AddDirectDependency(depsMutationModuleID, invowkmod.ModuleSourceID(depsMutationSource))

		matched, forbidden, found := findAccessibleCommand(
			available,
			"",
			commandDependencyAlternativesForTest(t, "@tools lint"),
			scope,
		)
		if !found || matched != allowed || len(forbidden) != 0 {
			t.Fatalf("allowed lookup matched=%v forbidden=%v found=%v, want allowed command", matched, forbidden, found)
		}

		matched, forbidden, found = findAccessibleCommand(
			available,
			"",
			commandDependencyAlternativesForTest(t, "@blocked lint"),
			scope,
		)
		if !found || matched != nil || len(forbidden) != 1 {
			t.Fatalf("blocked lookup matched=%v forbidden=%v found=%v, want one forbidden", matched, forbidden, found)
		}
		if !strings.Contains(forbidden[0].String(), "module 'blocked' is not accessible") {
			t.Fatalf("forbidden detail = %q, want inaccessible blocked module", forbidden[0])
		}

		decision := commandScopeDecision(nil, &discovery.CommandInfo{Name: depsMutationCommand})
		if !decision.Allowed || decision.TargetCommand != invowkmod.CommandReference(depsMutationCommand) {
			t.Fatalf("root decision = %+v, want allowed target command", decision)
		}
	})
}

func TestCommandCandidateMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("source candidates de-duplicate prioritized lookups and fallback scan", func(t *testing.T) {
		t.Parallel()

		shared := &discovery.CommandInfo{
			Name:       "tools lint",
			SimpleName: depsMutationSimple,
			SourceID:   depsMutationSource,
		}
		fallback := &discovery.CommandInfo{
			Name:       "not-indexed",
			SimpleName: depsMutationSimple,
			SourceID:   depsMutationSource,
		}
		available := map[invowkfile.CommandName]*discovery.CommandInfo{
			"tools lint": shared,
			"lint":       shared,
			"fallback":   fallback,
		}

		got := sourceCommandCandidates(available, invowkmod.ModuleSourceID(depsMutationSource), depsMutationSimple)
		if len(got) != 2 || got[0] != shared || got[1] != fallback {
			t.Fatalf("sourceCommandCandidates() = %v, want shared then fallback", got)
		}
	})

	t.Run("source and simple-name helpers classify command identity", func(t *testing.T) {
		t.Parallel()

		if commandInfoSourceID(nil) != "" {
			t.Fatal("commandInfoSourceID(nil) should be empty")
		}
		if commandInfoSourceID(&discovery.CommandInfo{}) != "" {
			t.Fatal("commandInfoSourceID(empty) should be empty")
		}
		cmd := &discovery.CommandInfo{Name: "tools lint", SourceID: depsMutationSource}
		if commandInfoSourceID(cmd) != invowkmod.ModuleSourceID(depsMutationSource) {
			t.Fatalf("commandInfoSourceID() = %q, want %q", commandInfoSourceID(cmd), depsMutationSource)
		}
		if commandInfoSimpleName(nil, invowkmod.ModuleSourceID(depsMutationSource)) != "" {
			t.Fatal("commandInfoSimpleName(nil) should be empty")
		}
		if got := commandInfoSimpleName(&discovery.CommandInfo{Name: "tools lint", SimpleName: "fmt"}, invowkmod.ModuleSourceID(depsMutationSource)); got != "fmt" {
			t.Fatalf("explicit SimpleName = %q, want fmt", got)
		}
		if got := commandInfoSimpleName(cmd, invowkmod.ModuleSourceID(depsMutationSource)); got != depsMutationSimple {
			t.Fatalf("derived SimpleName = %q, want %q", got, depsMutationSimple)
		}
		if got := commandInfoSimpleName(cmd, "other"); got != cmd.Name {
			t.Fatalf("nonmatching prefix SimpleName = %q, want full command name", got)
		}
	})

	t.Run("current command source falls back from source id to metadata", func(t *testing.T) {
		t.Parallel()

		if currentCommandSourceID(nil) != "" {
			t.Fatal("currentCommandSourceID(nil) should be empty")
		}
		if got := currentCommandSourceID(&discovery.CommandInfo{SourceID: depsMutationSource}); got != invowkmod.ModuleSourceID(depsMutationSource) {
			t.Fatalf("current explicit source = %q, want %q", got, depsMutationSource)
		}
		meta := mustModuleMetadata(t, &invowkfile.Invowkmod{Module: depsMutationModuleID, Version: "1.0.0"})
		if got := currentCommandSourceID(&discovery.CommandInfo{Invowkfile: &invowkfile.Invowkfile{Metadata: meta}}); got != invowkmod.ModuleSourceID(depsMutationModuleID) {
			t.Fatalf("current metadata source = %q, want %q", got, depsMutationModuleID)
		}
	})
}

func TestMissingCommandMessageMutationContracts(t *testing.T) {
	t.Parallel()

	available := map[invowkfile.CommandName]*discovery.CommandInfo{
		"tools fmt": {
			Name:       "tools fmt",
			SimpleName: "fmt",
			SourceID:   depsMutationSource,
		},
	}

	tests := []struct {
		name        string
		alts        []commandDependencyAlternative
		current     invowkmod.ModuleSourceID
		inContainer bool
		want        string
	}{
		{
			name: "unknown qualified source",
			alts: commandDependencyAlternativesForTest(t, "@missing lint"),
			want: `@missing lint - source "missing" not found`,
		},
		{
			name:        "missing command in known source in container",
			alts:        commandDependencyAlternativesForTest(t, "@tools lint"),
			inContainer: true,
			want:        `@tools lint - command "lint" not found in source "tools" in container`,
		},
		{
			name:    "bare command missing in current source",
			alts:    commandDependencyAlternativesForTest(t, "lint"),
			current: invowkmod.ModuleSourceID(depsMutationSource),
			want:    `lint - command not found in source "tools"`,
		},
		{
			name:        "multiple alternatives in container",
			alts:        commandDependencyAlternativesForTest(t, "lint", "@tools fmt"),
			inContainer: true,
			want:        "none of [lint, @tools fmt] found in container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatMissingDiscoveredCommandDependency(available, tt.current, tt.alts, tt.inContainer)
			if got.String() != tt.want {
				t.Fatalf("formatMissingDiscoveredCommandDependency() = %q, want %q", got, tt.want)
			}
		})
	}
}

func depsMutationHostCommand(dependsOn *invowkfile.DependsOn) *invowkfile.Command {
	return &invowkfile.Command{
		Name:      depsMutationCommand,
		DependsOn: dependsOn,
		Implementations: []invowkfile.Implementation{{
			Script:   invowkfile.ImplementationScript{Content: "echo ok"},
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
		}},
	}
}

func depsMutationCommandInfo(cmd *invowkfile.Command, inv *invowkfile.Invowkfile) *discovery.CommandInfo {
	if inv == nil {
		inv = &invowkfile.Invowkfile{}
	}
	return &discovery.CommandInfo{
		Name:       cmd.Name,
		Command:    cmd,
		Invowkfile: inv,
	}
}

func depsMutationRequirementAndLock() (invowkmod.ModuleRequirement, *invowkmod.LockFile) {
	req := invowkmod.ModuleRequirement{
		GitURL:  depsMutationGitURL,
		Version: depsMutationVersion,
		Alias:   invowkmod.ModuleAlias(depsMutationSource),
	}
	lock := invowkmod.NewLockFile()
	lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
		GitURL:          req.GitURL,
		Version:         req.Version,
		ResolvedVersion: depsMutationResolvedVersion,
		GitCommit:       depsMutationGitCommit,
		Alias:           req.Alias,
		Namespace:       invowkmod.ModuleNamespace(depsMutationSource),
		ModuleID:        depsMutationModuleID,
		CommandSourceID: invowkmod.ModuleSourceID(depsMutationSource),
		ContentHash:     depsMutationContentHash,
	}
	return req, lock
}

func requireDependencyError(t *testing.T, err error) *DependencyError {
	t.Helper()

	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("error = %v, want *DependencyError", err)
	}
	return depErr
}
