// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/app/deps"
	appexec "github.com/invowk/invowk/internal/app/execute"
	"github.com/invowk/invowk/internal/config"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type recordingDependencyHostProbe struct {
	tools []invowkfile.BinaryName
}

func TestValidateInputs(t *testing.T) {
	t.Parallel()

	service := &Service{}
	cmdInfo := commandsvcTestCommandInfo(t, "build")

	t.Run("invalid flag value", func(t *testing.T) {
		t.Parallel()

		err := service.validateInputs(
			Request{Name: "build"},
			cmdInfo,
			resolvedDefinitions{
				flagDefs: []invowkfile.Flag{{Name: "count", Type: invowkfile.FlagTypeInt}},
				flagValues: map[invowkfile.FlagName]string{
					"count": "oops",
				},
			},
		)
		if err == nil {
			t.Fatal("expected flag validation error")
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		t.Parallel()

		err := service.validateInputs(
			Request{Name: "build"},
			cmdInfo,
			resolvedDefinitions{
				argDefs: []invowkfile.Argument{{Name: "target", Required: true}},
			},
		)
		var argErr *deps.ArgumentValidationError
		if !errors.As(err, &argErr) {
			t.Fatalf("errors.As(*ArgumentValidationError) = false for %T", err)
		}
	})

	t.Run("unsupported platform", func(t *testing.T) {
		t.Parallel()

		cmdInfo := commandsvcTestCommandInfo(t, "build")
		cmdInfo.Command.Implementations[0].Platforms = []invowkfile.PlatformConfig{{Name: unsupportedPlatform()}}
		err := service.validateInputs(Request{Name: "build"}, cmdInfo, resolvedDefinitions{})
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("errors.Is(ErrUnsupportedPlatform) = false for %v", err)
		}
	})
}

func TestResolveRuntime(t *testing.T) {
	t.Parallel()

	service := &Service{}
	cmdInfo := commandsvcTestCommandInfo(t, "build")

	selection, err := service.resolveRuntime(Request{Name: "build"}, cmdInfo, config.DefaultConfig())
	if err != nil {
		t.Fatalf("resolveRuntime(default) = %v", err)
	}
	if selection.Mode() != invowkfile.RuntimeVirtual {
		t.Fatalf("selection.Mode() = %s, want virtual", selection.Mode())
	}

	_, err = service.resolveRuntime(
		Request{Name: "build", Runtime: invowkfile.RuntimeContainer},
		cmdInfo,
		config.DefaultConfig(),
	)
	var notAllowed *appexec.RuntimeNotAllowedError
	if !errors.As(err, &notAllowed) {
		t.Fatalf("errors.As(*RuntimeNotAllowedError) = false for %T", err)
	}

	badCfg := config.DefaultConfig()
	badCfg.DefaultRuntime = config.RuntimeMode("bogus")
	_, err = service.resolveRuntime(Request{Name: "build"}, cmdInfo, badCfg)
	if !errors.Is(err, ErrRuntimeResolution) {
		t.Fatalf("errors.Is(ErrRuntimeResolution) = false for %v", err)
	}
}

func TestEnsureSSHIfNeeded(t *testing.T) {
	t.Parallel()

	service := &Service{hostAccess: noopHostAccess{}}
	if err := service.ensureSSHIfNeeded(t.Context(), appexec.RuntimeSelection{}); err != nil {
		t.Fatalf("ensureSSHIfNeeded(no host ssh) = %v", err)
	}
}

func TestBuildExecContextAndValidateDeps(t *testing.T) {
	t.Parallel()

	service := &Service{discovery: &stubCommandDiscovery{}}
	cmdInfo := commandsvcTestCommandInfo(t, "build")
	flagName := invowkfile.FlagName("mode")

	execCtx, err := service.buildExecContext(
		t.Context(),
		Request{
			Name:            "build",
			Args:            []string{"target"},
			EnvVars:         map[string]string{"FOO": "bar"},
			FlagValues:      map[invowkfile.FlagName]string{flagName: "safe"},
			ArgDefs:         []invowkfile.Argument{{Name: "target"}},
			FlagDefs:        []invowkfile.Flag{{Name: flagName, Type: invowkfile.FlagTypeString}},
			EnvInheritMode:  invowkfile.EnvInheritNone,
			EnvInheritAllow: []invowkfile.EnvVarName{"PATH"},
			EnvInheritDeny:  []invowkfile.EnvVarName{"SECRET"},
		},
		cmdInfo,
		resolvedDefinitions{
			flagDefs:   []invowkfile.Flag{{Name: flagName, Type: invowkfile.FlagTypeString}},
			argDefs:    []invowkfile.Argument{{Name: "target"}},
			flagValues: map[invowkfile.FlagName]string{flagName: "safe"},
		},
		mustResolveRuntime(t, cmdInfo.Command),
	)
	if err != nil {
		t.Fatalf("buildExecContext() = %v", err)
	}
	if execCtx.Env.RuntimeEnvVars["FOO"] != "bar" {
		t.Fatalf("execCtx.Env.RuntimeEnvVars = %v", execCtx.Env.RuntimeEnvVars)
	}
	if execCtx.Env.ExtraEnv["INVOWK_FLAG_MODE"] != "safe" {
		t.Fatalf("execCtx.Env.ExtraEnv = %v", execCtx.Env.ExtraEnv)
	}

	registry := runtimepkg.NewRegistry()
	registry.Register(runtimepkg.RuntimeTypeVirtual, &stubRuntime{name: "virtual"})
	if err := service.validateDeps(cmdInfo, execCtx, registry, map[string]string{}); err != nil {
		t.Fatalf("validateDeps() = %v", err)
	}
}

func TestValidateDepsUsesInjectedHostProbe(t *testing.T) {
	t.Parallel()

	probe := &recordingDependencyHostProbe{}
	service := &Service{
		discovery: &stubCommandDiscovery{},
		hostProbe: probe,
	}
	cmdInfo := commandsvcTestCommandInfo(t, "build")
	cmdInfo.Command.DependsOn = &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"tool-from-probe"}}},
	}
	execCtx, err := service.buildExecContext(
		t.Context(),
		Request{Name: "build"},
		cmdInfo,
		resolvedDefinitions{},
		mustResolveRuntime(t, cmdInfo.Command),
	)
	if err != nil {
		t.Fatalf("buildExecContext() = %v", err)
	}

	registry := runtimepkg.NewRegistry()
	registry.Register(runtimepkg.RuntimeTypeVirtual, &stubRuntime{name: "virtual"})
	if err := service.validateDeps(cmdInfo, execCtx, registry, map[string]string{}); err != nil {
		t.Fatalf("validateDeps() = %v", err)
	}
	if len(probe.tools) != 1 || probe.tools[0] != "tool-from-probe" {
		t.Fatalf("probe.tools = %v, want [tool-from-probe]", probe.tools)
	}
}

func unsupportedPlatform() invowkfile.PlatformType {
	switch invowkfile.CurrentPlatform() {
	case invowkfile.PlatformLinux:
		return invowkfile.PlatformWindows
	case invowkfile.PlatformWindows:
		return invowkfile.PlatformMac
	default:
		return invowkfile.PlatformLinux
	}
}

func mustResolveRuntime(t *testing.T, cmd *invowkfile.Command) appexec.RuntimeSelection {
	t.Helper()

	selection, err := appexec.ResolveRuntime(cmd, cmd.Name, "", config.DefaultConfig(), invowkfile.CurrentPlatform())
	if err != nil {
		t.Fatalf("ResolveRuntime() = %v", err)
	}
	return selection
}

func (p *recordingDependencyHostProbe) CheckTool(tool invowkfile.BinaryName) error {
	p.tools = append(p.tools, tool)
	return nil
}

func (*recordingDependencyHostProbe) CheckFilepath(types.FilesystemPath, types.FilesystemPath, invowkfile.FilepathDependency) error {
	return nil
}

func (*recordingDependencyHostProbe) RunCustomCheck(context.Context, invowkfile.CustomCheck) error {
	return nil
}
