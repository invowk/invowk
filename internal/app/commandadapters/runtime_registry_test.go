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

func TestRuntimeRegistryFactoryInjectsVirtualInteractiveLauncher(t *testing.T) {
	t.Parallel()

	factory, err := NewRuntimeRegistryFactory()
	if err != nil {
		t.Fatalf("NewRuntimeRegistryFactory() error = %v", err)
	}
	session := factory.Create(config.DefaultConfig(), nil, invowkfile.RuntimeVirtual)
	t.Cleanup(session.Close)

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "hello",
			Implementations: []invowkfile.Implementation{{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}},
	}
	ctx := runtime.NewExecutionContext(t.Context(), &inv.Commands[0], inv)
	ctx.SelectedRuntime = invowkfile.RuntimeVirtual
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]

	executor := &recordingInteractiveExecutor{}
	result, _, err := session.Execute(ctx, inv.Commands[0].Name, true, executor)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}

	if !slices.Contains(executor.args, "internal") || !slices.Contains(executor.args, "exec-virtual") {
		t.Fatalf("prepared args = %v, want hidden virtual exec command", executor.args)
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

	nativeResult, _, err := session.Execute(runtimeContext(t, invowkfile.RuntimeNative), "", false, nil)
	if err != nil {
		t.Fatalf("native Execute() error = %v", err)
	}
	if !nativeResult.Success() {
		t.Fatalf("native Execute() result = %#v, want success", nativeResult)
	}

	virtualResult, _, err := session.Execute(runtimeContext(t, invowkfile.RuntimeVirtual), "", false, nil)
	if err != nil {
		t.Fatalf("virtual Execute() error = %v", err)
	}
	if !virtualResult.Success() {
		t.Fatalf("virtual Execute() result = %#v, want success", virtualResult)
	}

	containerResult, _, err := session.Execute(runtimeContext(t, invowkfile.RuntimeContainer), "", false, nil)
	if err != nil {
		t.Fatalf("container Execute() error = %v", err)
	}
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
				Script:    "true",
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
