// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestCommandDependencyStructuredFailuresStoredMutation(t *testing.T) {
	t.Parallel()

	blockedID := invowkmod.ModuleID("io.example.blocked")
	cmd := depsMutationHostCommand(&invowkfile.DependsOn{
		Commands: []invowkfile.CommandDependency{
			{Alternatives: []invowkfile.CommandDependencyRef{"missing"}},
			{Alternatives: []invowkfile.CommandDependencyRef{"@blocked lint"}},
		},
	})
	callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:  depsMutationCallerID,
		Version: "1.0.0",
	})
	callerInfo := depsMutationCommandInfo(cmd, &invowkfile.Invowkfile{Metadata: callerMeta})
	callerInfo.SourceID = "caller"
	available := &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{{
			Name:       "blocked lint",
			SimpleName: depsMutationSimple,
			SourceID:   "blocked",
			ModuleID:   &blockedID,
		}},
	}

	err := CheckCommandDependenciesExistWithLockProvider(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: available}},
		cmd.DependsOn,
		callerInfo,
		testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative),
		nil,
	)
	depErr := requireDependencyError(t, err)
	requireDependencyFailureKinds(t,
		depErr.StructuredFailures,
		DependencyFailureCommand,
		DependencyFailureForbiddenCommand,
	)
	if depErr.StructuredFailures[0].Detail() != depErr.MissingCommands[0] {
		t.Fatalf("StructuredFailures[0].Detail() = %q, want missing command detail %q",
			depErr.StructuredFailures[0].Detail(), depErr.MissingCommands[0])
	}
	if depErr.StructuredFailures[1].Detail() != depErr.ForbiddenCommands[0] {
		t.Fatalf("StructuredFailures[1].Detail() = %q, want forbidden command detail %q",
			depErr.StructuredFailures[1].Detail(), depErr.ForbiddenCommands[0])
	}
}

func TestMissingCommandMessageRootSingleAlternativeMutation(t *testing.T) {
	t.Parallel()

	got := formatMissingDiscoveredCommandDependency(
		map[invowkfile.CommandName]*discovery.CommandInfo{},
		"",
		commandDependencyAlternativesForTest(t, "lint"),
		false,
	)
	if got.String() != "lint - command not found" {
		t.Fatalf("formatMissingDiscoveredCommandDependency() = %q, want root missing command message", got)
	}
}

func TestToolDependencyBoundaryMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("container nil and empty deps skip probe", func(t *testing.T) {
		t.Parallel()

		if err := CheckToolDependenciesInContainer(nil, nil, ExecutionContext{}); err != nil {
			t.Fatalf("nil deps error = %v, want nil", err)
		}
		if err := CheckToolDependenciesInContainer(&invowkfile.DependsOn{}, nil, ExecutionContext{}); err != nil {
			t.Fatalf("empty deps error = %v, want nil", err)
		}
	})

	t.Run("tool failures store structured payloads", func(t *testing.T) {
		t.Parallel()

		ctx := testDependencyExecutionContext(t, &invowkfile.Command{Name: "build"}, invowkfile.RuntimeContainer)
		depsWithTool := &invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{{
				Alternatives: []invowkfile.BinaryName{"go"},
			}},
		}

		containerErr := CheckToolDependenciesInContainer(
			depsWithTool,
			&recordingRuntimeProbe{toolErr: errors.New("missing go")},
			ctx,
		)
		requireChecksMutationDependencyPayload(
			t,
			requireDependencyError(t, containerErr),
			ctx.CommandName,
			DependencyFailureTool,
			[]string{"missing go"},
		)

		hostErr := CheckHostToolDependenciesWithProbe(
			depsWithTool,
			ctx,
			&recordingHostProbe{
				toolErrors: map[invowkfile.BinaryName]error{"go": errors.New("host missing go")},
			},
		)
		requireChecksMutationDependencyPayload(
			t,
			requireDependencyError(t, hostErr),
			ctx.CommandName,
			DependencyFailureTool,
			[]string{"host missing go"},
		)
	})
}
