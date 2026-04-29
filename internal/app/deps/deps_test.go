// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

type stubCommandSetProvider struct {
	result discovery.CommandSetResult
	err    error
}

func (s *stubCommandSetProvider) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	if s.err != nil {
		return discovery.CommandSetResult{}, s.err
	}
	return s.result, nil
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

	t.Run("allows non-aliased direct dependency module id", func(t *testing.T) {
		t.Parallel()

		depID := invowkmod.ModuleID("io.example.dep")
		callerMeta := invowkfile.NewModuleMetadataFromInvowkmod(&invowkfile.Invowkmod{
			Module:  "io.example.caller",
			Version: "1.0.0",
			Requires: []invowkmod.ModuleRequirement{{
				GitURL:  "https://github.com/example/dep.git",
				Version: "^1.0.0",
			}},
		})
		callerInfo := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("build"),
			Command:    &invowkfile.Command{Name: "build"},
			Invowkfile: &invowkfile.Invowkfile{Metadata: callerMeta},
		}
		commandSet := &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{
				Name:     invowkfile.CommandName("io.example.dep test"),
				SourceID: discovery.SourceID("dep"),
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

		if err := CheckCommandDependenciesExist(disc, deps, callerInfo, ctx); err != nil {
			t.Fatalf("CheckCommandDependenciesExist() = %v", err)
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

		if err := ValidateDependencies(disc, cmdInfo, runtimepkg.NewRegistry(), execCtx, nil); err != nil {
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

		err := ValidateDependencies(disc, cmdInfo, runtimepkg.NewRegistry(), execCtx, nil)
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

		if err := ValidateDependencies(disc, cmdInfo, runtimepkg.NewRegistry(), execCtx, nil); err != nil {
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

		err := ValidateDependenciesWithCapabilityChecker(disc, cmdInfo, runtimepkg.NewRegistry(), execCtx, nil,
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

		err := ValidateRuntimeDependencies(cmdInfo, runtimepkg.NewRegistry(), &runtimepkg.ExecutionContext{
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

		registry := runtimepkg.NewRegistry()
		registry.Register(runtimepkg.RuntimeTypeContainer, &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				if strings.Contains(string(ctx.SelectedImpl.Script), "check-cmd 'build'") {
					return &runtimepkg.Result{ExitCode: 0}
				}
				return &runtimepkg.Result{ExitCode: 1}
			},
		})

		err := ValidateRuntimeDependencies(cmdInfo, registry, &runtimepkg.ExecutionContext{
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
