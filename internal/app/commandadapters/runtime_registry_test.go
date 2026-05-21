// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"errors"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type recordingInteractiveExecutor struct {
	args []string
}

func (e *recordingInteractiveExecutor) Execute(execCtx *runtime.ExecutionContext, _ invowkfile.CommandName, interactiveRT commandsvc.RuntimeInteractiveCommand) *runtime.Result {
	if err := interactiveRT.Validate(execCtx); err != nil {
		return &runtime.Result{ExitCode: 1, Error: err}
	}
	prepared, err := interactiveRT.PrepareInteractive(execCtx)
	if err != nil {
		return &runtime.Result{ExitCode: 1, Error: err}
	}
	if prepared.Cleanup != nil {
		defer prepared.Cleanup()
	}
	e.args = prepared.Cmd.Args
	return &runtime.Result{ExitCode: 0}
}

func TestRuntimeRegistryFactoryInjectsShInteractiveLauncher(t *testing.T) {
	t.Parallel()

	factory, err := NewRuntimeRegistryFactory()
	if err != nil {
		t.Fatalf("NewRuntimeRegistryFactory() error = %v", err)
	}
	session := factory.Create(config.DefaultConfig(), nil, invowkfile.RuntimeVirtualSh)
	t.Cleanup(session.Close)

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "hello",
			Implementations: []invowkfile.Implementation{{
				Script:    invowkfile.ImplementationScript{Content: "echo hello"},
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtualSh}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}},
	}
	ctx := runtime.NewExecutionContext(t.Context(), &inv.Commands[0], inv)
	ctx.SelectedRuntime = invowkfile.RuntimeVirtualSh
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]

	rt, err := session.RuntimeForContext(ctx)
	if err != nil {
		t.Fatalf("RuntimeForContext() error = %v", err)
	}
	interactiveRT := runtime.GetInteractiveRuntime(rt)
	if interactiveRT == nil {
		t.Fatalf("RuntimeForContext() returned %T, want interactive runtime", rt)
	}

	executor := &recordingInteractiveExecutor{}
	result := executor.Execute(ctx, inv.Commands[0].Name, interactiveRT)
	if !result.Success() {
		t.Fatalf("interactive Execute() result = %#v, want success", result)
	}

	if !slices.Contains(executor.args, "internal") || !slices.Contains(executor.args, "exec-virtual-sh") {
		t.Fatalf("prepared args = %v, want hidden virtual-sh exec command", executor.args)
	}
}

func TestRuntimeRegistryFactorySkipsContainerRuntimeForNonContainerExecution(t *testing.T) {
	t.Parallel()

	called := false
	factory := RuntimeRegistryFactory{
		containerRuntimeFactory: func(*config.Config) (*runtime.ContainerRuntime, error) {
			called = true
			return nil, errors.New("container runtime factory should not be called")
		},
	}
	session := factory.Create(config.DefaultConfig(), nil, invowkfile.RuntimeNative)
	t.Cleanup(session.Close)

	if called {
		t.Fatal("container runtime factory was called for native execution")
	}
	if session.ContainerInitErr() != nil {
		t.Fatalf("ContainerInitErr = %v, want nil", session.ContainerInitErr())
	}
	if len(session.Diagnostics()) != 0 {
		t.Fatalf("Diagnostics = %v, want none", session.Diagnostics())
	}

	nativeResult := session.Execute(runtimeContext(t, invowkfile.RuntimeNative))
	if !nativeResult.Success() {
		t.Fatalf("native Execute() result = %#v, want success", nativeResult)
	}

	virtualResult := session.Execute(runtimeContext(t, invowkfile.RuntimeVirtualSh))
	if !virtualResult.Success() {
		t.Fatalf("virtual Execute() result = %#v, want success", virtualResult)
	}

	containerResult := session.Execute(runtimeContext(t, invowkfile.RuntimeContainer))
	if !errors.Is(containerResult.Error, runtime.ErrRuntimeNotAvailable) {
		t.Fatalf("container Execute() error = %v, want ErrRuntimeNotAvailable", containerResult.Error)
	}
}

func runtimeContext(t testing.TB, mode invowkfile.RuntimeMode) *runtime.ExecutionContext {
	t.Helper()

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "hello",
			Implementations: []invowkfile.Implementation{{
				Script:    invowkfile.ImplementationScript{Content: "true"},
				Runtimes:  []invowkfile.RuntimeConfig{{Name: mode}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}},
	}
	ctx := runtime.NewExecutionContext(t.Context(), &inv.Commands[0], inv)
	ctx.SelectedRuntime = mode
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]
	return ctx
}
