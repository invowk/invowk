// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
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
