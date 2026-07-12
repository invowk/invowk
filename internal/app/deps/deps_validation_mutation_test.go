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
	"github.com/invowk/invowk/pkg/types"
)

func TestExecutionContextGoContextMutationContracts(t *testing.T) {
	t.Parallel()

	if (ExecutionContext{}).GoContext() == nil {
		t.Fatal("ExecutionContext{}.GoContext() = nil, want fallback context")
	}

	want := context.WithValue(t.Context(), depsMutationContextKey{}, "marker")
	if got := (ExecutionContext{Context: want}).GoContext(); got != want {
		t.Fatalf("GoContext() = %p, want explicit context %p", got, want)
	}
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

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "missing env vars stop before host probes", run: func(t *testing.T) {
			t.Helper()

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
		}},

		{name: "filepath failure stops before custom checks and command discovery", run: func(t *testing.T) {
			t.Helper()

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
		}},

		{name: "custom check failure stops before command discovery", run: func(t *testing.T) {
			t.Helper()

			checkErr := errors.New("host check failed")
			cmd := depsMutationHostCommand(&invowkfile.DependsOn{
				CustomChecks: []invowkfile.CustomCheckDependency{{
					Name:   "host-check",
					Script: invowkfile.CustomCheckScript{Content: "exit 1"},
				}},
				Commands: []invowkfile.CommandDependency{{
					Alternatives: []invowkfile.CommandDependencyRef{invowkfile.CommandDependencyRef(depsMutationCommand)},
				}},
			})
			probe := &recordingHostProbe{
				checkErrors: map[invowkfile.CheckName]error{
					"host-check": checkErr,
				},
			}
			err := ValidateHostDependenciesWithHostProbe(
				panicCommandSetProvider{t: t},
				depsMutationCommandInfo(cmd, &invowkfile.Invowkfile{}),
				testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative),
				map[string]string{},
				nil,
				probe,
			)
			depErr := requireDependencyError(t, err)
			if len(depErr.FailedCustomChecks) != 1 {
				t.Fatalf("FailedCustomChecks = %v, want one custom check failure", depErr.FailedCustomChecks)
			}
			requireDependencyFailureKinds(t, depErr.Failures(), DependencyFailureCustomCheck)
		}},

		{name: "host command failure is not reported as a container failure", run: func(t *testing.T) {
			t.Helper()

			cmd := depsMutationHostCommand(&invowkfile.DependsOn{
				Commands: []invowkfile.CommandDependency{{
					Alternatives: []invowkfile.CommandDependencyRef{"lint"},
				}},
			})
			err := ValidateHostDependenciesWithHostProbe(
				&stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}}},
				depsMutationCommandInfo(cmd, &invowkfile.Invowkfile{}),
				testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative),
				map[string]string{},
				nil,
				nil,
			)
			depErr := requireDependencyError(t, err)
			if len(depErr.MissingCommands) != 1 {
				t.Fatalf("MissingCommands = %v, want one missing command", depErr.MissingCommands)
			}
			if strings.Contains(depErr.MissingCommands[0].String(), "container") {
				t.Fatalf("MissingCommands[0] = %q, should be host-scoped", depErr.MissingCommands[0])
			}
			requireDependencyFailureKinds(t, depErr.Failures(), DependencyFailureCommand)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestValidateRuntimeDependenciesMutationBoundaries(t *testing.T) {
	t.Parallel()

	cmd := runtimeDependencyCommand(invowkfile.CommandDependencyRef(depsMutationCommand))
	cmdInfo := runtimeDependencyCommandInfo(cmd)

	t.Run("container nil and empty runtime deps do not require a probe", func(t *testing.T) {
		t.Parallel()

		ctx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer)
		for _, deps := range []*invowkfile.DependsOn{nil, {}} {
			ctx.RuntimeDependsOn = deps
			if err := ValidateRuntimeDependencies(panicCommandSetProvider{t: t}, cmdInfo, nil, ctx, nil); err != nil {
				t.Fatalf("RuntimeDependsOn=%v error = %v, want nil", deps, err)
			}
		}
	})

	t.Run("runtime env failure stops before later runtime probes", func(t *testing.T) {
		t.Parallel()

		probe := &recordingRuntimeProbe{envErr: errors.New("env unavailable")}
		ctx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer)
		ctx.RuntimeDependsOn = &invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING_ENV"}}}},
			Tools:   []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"tool-after-env"}}},
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

	t.Run("runtime dependency failures preserve failure kinds", testRuntimeDependencyFailureKindsMutation)
}

func testRuntimeDependencyFailureKindsMutation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dependsOn    *invowkfile.DependsOn
		probe        *recordingRuntimeProbe
		wantKind     DependencyFailureKind
		wantCommands int
		wantTools    int
		wantFiles    int
		wantCaps     int
		wantChecks   int
	}{
		{
			name: "tool",
			dependsOn: &invowkfile.DependsOn{
				Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"missing-tool"}}},
			},
			probe:     &recordingRuntimeProbe{toolErr: errors.New("missing tool")},
			wantKind:  DependencyFailureTool,
			wantTools: 1,
		},
		{
			name: "filepath",
			dependsOn: &invowkfile.DependsOn{
				Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/missing"}}},
			},
			probe:     &recordingRuntimeProbe{filepathErr: errors.New("missing filepath")},
			wantKind:  DependencyFailureFilepath,
			wantFiles: 1,
		},
		{
			name: "capability",
			dependsOn: &invowkfile.DependsOn{
				Capabilities: []invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}}},
			},
			probe:    &recordingRuntimeProbe{capabilityErr: errors.New("missing tty")},
			wantKind: DependencyFailureCapability,
			wantCaps: 1,
		},
		{
			name: "custom check",
			dependsOn: &invowkfile.DependsOn{
				CustomChecks: []invowkfile.CustomCheckDependency{{
					Name:   "runtime-check",
					Script: invowkfile.CustomCheckScript{Content: "exit 1"},
				}},
			},
			probe:      &recordingRuntimeProbe{checkErr: errors.New("runtime check failed")},
			wantKind:   DependencyFailureCustomCheck,
			wantChecks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := depsMutationRuntimeCommand(tt.dependsOn)
			ctx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer)
			err := ValidateRuntimeDependencies(
				panicCommandSetProvider{t: t},
				depsMutationCommandInfo(cmd, &invowkfile.Invowkfile{}),
				tt.probe,
				ctx,
				nil,
			)
			depErr := requireDependencyError(t, err)
			requireDependencyFailureKinds(t, depErr.Failures(), tt.wantKind)
			if len(tt.probe.commands) != tt.wantCommands ||
				len(tt.probe.tools) != tt.wantTools ||
				len(tt.probe.filepaths) != tt.wantFiles ||
				len(tt.probe.capabilities) != tt.wantCaps ||
				len(tt.probe.checks) != tt.wantChecks {
				t.Fatalf("runtime probe calls = %+v, want commands=%d tools=%d files=%d caps=%d checks=%d",
					tt.probe, tt.wantCommands, tt.wantTools, tt.wantFiles, tt.wantCaps, tt.wantChecks)
			}
		})
	}
}
