// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
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

		if err := CheckCommandDependenciesExist(disc, deps, "mod", ctx); err != nil {
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

		err := CheckCommandDependenciesExist(disc, deps, "mod", ctx)
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
		if _, ok := available[invowkfile.CommandName("deploy")]; !ok {
			t.Fatalf("available missing deploy: %v", available)
		}
	})

	t.Run("wraps discovery failure", func(t *testing.T) {
		t.Parallel()

		disc := &stubCommandSetProvider{err: errors.New("boom")}
		_, err := discoverAvailableCommands(disc, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to discover commands for dependency validation") {
			t.Fatalf("err = %v", err)
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
