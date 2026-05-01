// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"errors"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestRuntimeRegistryFactoryInjectsVirtualInteractiveLauncher(t *testing.T) {
	t.Parallel()

	factory, err := NewRuntimeRegistryFactory()
	if err != nil {
		t.Fatalf("NewRuntimeRegistryFactory() error = %v", err)
	}
	result := factory.Create(config.DefaultConfig(), nil, invowkfile.RuntimeVirtual)
	rt, err := result.Registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		t.Fatalf("registry.Get(virtual) error = %v", err)
	}
	interactive, ok := rt.(runtime.InteractiveRuntime)
	if !ok {
		t.Fatalf("virtual runtime does not implement InteractiveRuntime")
	}

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

	prepared, err := interactive.PrepareInteractive(ctx)
	if err != nil {
		t.Fatalf("PrepareInteractive() error = %v", err)
	}
	t.Cleanup(prepared.Cleanup)

	if !slices.Contains(prepared.Cmd.Args, "internal") || !slices.Contains(prepared.Cmd.Args, "exec-virtual") {
		t.Fatalf("prepared args = %v, want hidden virtual exec command", prepared.Cmd.Args)
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
	result := factory.Create(config.DefaultConfig(), nil, invowkfile.RuntimeNative)
	defer result.Cleanup()

	if called {
		t.Fatal("container runtime factory was called for native execution")
	}
	if _, err := result.Registry.Get(runtime.RuntimeTypeNative); err != nil {
		t.Fatalf("native runtime not registered: %v", err)
	}
	if _, err := result.Registry.Get(runtime.RuntimeTypeVirtual); err != nil {
		t.Fatalf("virtual runtime not registered: %v", err)
	}
	if _, err := result.Registry.Get(runtime.RuntimeTypeContainer); !errors.Is(err, runtime.ErrRuntimeNotAvailable) {
		t.Fatalf("container runtime lookup error = %v, want ErrRuntimeNotAvailable", err)
	}
	if result.ContainerInitErr != nil {
		t.Fatalf("ContainerInitErr = %v, want nil", result.ContainerInitErr)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %v, want none", result.Diagnostics)
	}
}
