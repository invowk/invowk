// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"errors"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/config"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestRuntimeSelectionMutationPlatformValidationAndAccessors(t *testing.T) {
	t.Parallel()

	impl := &invowkfile.Implementation{}
	selection, err := NewRuntimeSelection(invowkfile.RuntimeNative, invowkfile.PlatformLinux, impl)
	if err != nil {
		t.Fatalf("NewRuntimeSelection() error = %v", err)
	}
	if selection.Platform() != invowkfile.PlatformLinux {
		t.Fatalf("Platform() = %q, want linux", selection.Platform())
	}
	if selection.Impl() != impl {
		t.Fatalf("Impl() = %p, want %p", selection.Impl(), impl)
	}

	_, err = NewRuntimeSelection(invowkfile.RuntimeNative, invowkfile.Platform("plan9"), impl)
	if err == nil {
		t.Fatal("NewRuntimeSelection(invalid platform) error = nil, want ErrInvalidPlatform")
	}
	if !errors.Is(err, invowkfile.ErrInvalidPlatform) {
		t.Fatalf("NewRuntimeSelection(invalid platform) error = %v, want ErrInvalidPlatform", err)
	}

	invalidSelection := RuntimeSelection{
		mode:     invowkfile.RuntimeNative,
		platform: invowkfile.Platform("plan9"),
		impl:     impl,
	}
	err = invalidSelection.Validate()
	if err == nil {
		t.Fatal("RuntimeSelection.Validate(invalid platform) error = nil")
	}
	if !errors.Is(err, ErrInvalidRuntimeSelection) {
		t.Fatalf("RuntimeSelection.Validate(invalid platform) error = %v, want ErrInvalidRuntimeSelection", err)
	}
	var selectionErr *InvalidRuntimeSelectionError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("RuntimeSelection.Validate(invalid platform) error = %T, want *InvalidRuntimeSelectionError", err)
	}
	if len(selectionErr.FieldErrors) != 1 || !errors.Is(selectionErr.FieldErrors[0], invowkfile.ErrInvalidPlatform) {
		t.Fatalf("RuntimeSelection.Validate(invalid platform) field errors = %v, want ErrInvalidPlatform", selectionErr.FieldErrors)
	}
}

func TestResolveRuntimeMutationErrorContracts(t *testing.T) {
	t.Parallel()

	cmd := executeMutationCommand()

	_, err := ResolveRuntime(cmd, "deploy", invowkfile.RuntimeMode("bogus"), nil, invowkfile.PlatformLinux)
	if err == nil {
		t.Fatal("ResolveRuntime(invalid override) error = nil, want ErrInvalidRuntimeMode")
	}
	if !errors.Is(err, invowkfile.ErrInvalidRuntimeMode) {
		t.Fatalf("ResolveRuntime(invalid override) error = %v, want ErrInvalidRuntimeMode", err)
	}

	_, err = ResolveRuntime(cmd, "deploy", "", &config.Config{DefaultRuntime: config.RuntimeMode("magical")}, invowkfile.PlatformLinux)
	if err == nil {
		t.Fatal("ResolveRuntime(invalid config default) error = nil, want wrapped ErrInvalidRuntimeMode")
	}
	if !errors.Is(err, invowkfile.ErrInvalidRuntimeMode) {
		t.Fatalf("ResolveRuntime(invalid config default) error = %v, want wrapped ErrInvalidRuntimeMode", err)
	}

	got, err := ResolveRuntime(cmd, "deploy", "", &config.Config{}, invowkfile.PlatformLinux)
	if err != nil {
		t.Fatalf("ResolveRuntime(empty config default) error = %v", err)
	}
	if got.Mode() != invowkfile.RuntimeNative {
		t.Fatalf("ResolveRuntime(empty config default) mode = %q, want native", got.Mode())
	}

	virtualOnly := executeMutationCommandWithRuntimes(invowkfile.RuntimeVirtualSh)
	_, err = ResolveRuntime(virtualOnly, "deploy", invowkfile.RuntimeNative, nil, invowkfile.PlatformLinux)
	if err == nil {
		t.Fatal("ResolveRuntime(disallowed override) error = nil")
	}
	var runtimeErr *RuntimeNotAllowedError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("ResolveRuntime(disallowed override) error = %T, want *RuntimeNotAllowedError", err)
	}
	if runtimeErr.CommandName != "deploy" {
		t.Fatalf("RuntimeNotAllowedError.CommandName = %q, want deploy", runtimeErr.CommandName)
	}
	if runtimeErr.Runtime != invowkfile.RuntimeNative {
		t.Fatalf("RuntimeNotAllowedError.Runtime = %q, want native", runtimeErr.Runtime)
	}
	if runtimeErr.Platform != invowkfile.PlatformLinux {
		t.Fatalf("RuntimeNotAllowedError.Platform = %q, want linux", runtimeErr.Platform)
	}
	if !slices.Equal(runtimeErr.Allowed, []invowkfile.RuntimeMode{invowkfile.RuntimeVirtualSh}) {
		t.Fatalf("RuntimeNotAllowedError.Allowed = %v, want [virtual-sh]", runtimeErr.Allowed)
	}
}

func TestBuildExecutionContextMutationProjectsExecutionOptions(t *testing.T) {
	t.Parallel()

	cmd := &invowkfile.Command{Name: "deploy"}
	inv := &invowkfile.Invowkfile{}
	impl := &invowkfile.Implementation{}
	selection := RuntimeSelection{
		mode:     invowkfile.RuntimeVirtualSh,
		platform: invowkfile.PlatformLinux,
		impl:     impl,
	}
	envVars := map[string]string{"TOKEN": "secret"}

	got, err := BuildExecutionContext(t.Context(), BuildExecutionContextOptions{
		Command:         cmd,
		CommandFullName: "module deploy",
		Invowkfile:      inv,
		Selection:       selection,
		Args:            []string{"one", "two"},
		Verbose:         true,
		Workdir:         "workspace",
		ForceRebuild:    true,
		ContainerName:   "dev-container",
		EnvFiles:        []invowkfile.DotenvFilePath{"service.env"},
		EnvVars:         envVars,
		EnvInheritMode:  invowkfile.EnvInheritAllow,
		EnvInheritAllow: []invowkfile.EnvVarName{"HOME"},
		EnvInheritDeny:  []invowkfile.EnvVarName{"SECRET"},
		ArgDefs: []invowkfile.Argument{
			{Name: "first"},
			{Name: "second", DefaultValue: "fallback"},
			{Name: "rest", Variadic: true},
		},
		Platform: invowkfile.PlatformLinux,
	})
	if err != nil {
		t.Fatalf("BuildExecutionContext() error = %v", err)
	}

	requireBuildExecutionContextProjectedOptions(t, got, impl)
}

func requireBuildExecutionContextProjectedOptions(
	t *testing.T,
	got *runtimepkg.ExecutionContext,
	impl *invowkfile.Implementation,
) {
	t.Helper()

	requireExecutionContextSelectionOptions(t, got, impl)
	requireExecutionContextEnvOptions(t, got)
	requireExecutionContextArgEnv(t, got)
}

func requireExecutionContextSelectionOptions(
	t *testing.T,
	got *runtimepkg.ExecutionContext,
	impl *invowkfile.Implementation,
) {
	t.Helper()

	if !got.Verbose {
		t.Fatal("Verbose = false, want true")
	}
	if got.SelectedRuntime != invowkfile.RuntimeVirtualSh {
		t.Fatalf("SelectedRuntime = %q, want virtual-sh", got.SelectedRuntime)
	}
	if got.SelectedPlatform != invowkfile.PlatformLinux {
		t.Fatalf("SelectedPlatform = %q, want linux", got.SelectedPlatform)
	}
	if got.SelectedImpl != impl {
		t.Fatalf("SelectedImpl = %p, want %p", got.SelectedImpl, impl)
	}
	if !slices.Equal(got.PositionalArgs, []string{"one", "two"}) {
		t.Fatalf("PositionalArgs = %v, want [one two]", got.PositionalArgs)
	}
	if got.WorkDir != "workspace" {
		t.Fatalf("WorkDir = %q, want workspace", got.WorkDir)
	}
	if !got.ForceRebuild {
		t.Fatal("ForceRebuild = false, want true")
	}
	if got.ContainerNameOverride != "dev-container" {
		t.Fatalf("ContainerNameOverride = %q, want dev-container", got.ContainerNameOverride)
	}
	if got.CommandFullName != "module deploy" {
		t.Fatalf("CommandFullName = %q, want module deploy", got.CommandFullName)
	}
}

func requireExecutionContextEnvOptions(t *testing.T, got *runtimepkg.ExecutionContext) {
	t.Helper()

	if !slices.Equal(got.Env.RuntimeEnvFiles, []invowkfile.DotenvFilePath{"service.env"}) {
		t.Fatalf("RuntimeEnvFiles = %v, want [service.env]", got.Env.RuntimeEnvFiles)
	}
	if got.Env.RuntimeEnvVars["TOKEN"] != "secret" {
		t.Fatalf("RuntimeEnvVars[TOKEN] = %q, want secret", got.Env.RuntimeEnvVars["TOKEN"])
	}
	if got.Env.InheritModeOverride != invowkfile.EnvInheritAllow {
		t.Fatalf("InheritModeOverride = %q, want allow", got.Env.InheritModeOverride)
	}
	if !slices.Equal(got.Env.InheritAllowOverride, []invowkfile.EnvVarName{"HOME"}) {
		t.Fatalf("InheritAllowOverride = %v, want [HOME]", got.Env.InheritAllowOverride)
	}
	if !slices.Equal(got.Env.InheritDenyOverride, []invowkfile.EnvVarName{"SECRET"}) {
		t.Fatalf("InheritDenyOverride = %v, want [SECRET]", got.Env.InheritDenyOverride)
	}
}

func requireExecutionContextArgEnv(t *testing.T, got *runtimepkg.ExecutionContext) {
	t.Helper()

	if got.Env.ExtraEnv["INVOWK_ARG_FIRST"] != "one" {
		t.Fatalf("INVOWK_ARG_FIRST = %q, want one", got.Env.ExtraEnv["INVOWK_ARG_FIRST"])
	}
	if got.Env.ExtraEnv["INVOWK_ARG_SECOND"] != "two" {
		t.Fatalf("INVOWK_ARG_SECOND = %q, want two", got.Env.ExtraEnv["INVOWK_ARG_SECOND"])
	}
	if got.Env.ExtraEnv["INVOWK_ARG_REST_COUNT"] != "0" {
		t.Fatalf("INVOWK_ARG_REST_COUNT = %q, want 0", got.Env.ExtraEnv["INVOWK_ARG_REST_COUNT"])
	}
	if _, ok := got.Env.ExtraEnv["INVOWK_ARG_REST_1"]; ok {
		t.Fatalf("INVOWK_ARG_REST_1 = %q, want absent", got.Env.ExtraEnv["INVOWK_ARG_REST_1"])
	}
}

func TestBuildExecutionContextMutationPlatformAndArgumentBoundaries(t *testing.T) {
	t.Parallel()

	cmd := &invowkfile.Command{Name: "deploy"}
	inv := &invowkfile.Invowkfile{}
	impl := &invowkfile.Implementation{}
	platform := executeMutationNonHostPlatform()

	got, err := BuildExecutionContext(t.Context(), BuildExecutionContextOptions{
		Command:    cmd,
		Invowkfile: inv,
		Selection: RuntimeSelection{
			mode:     invowkfile.RuntimeNative,
			platform: platform,
			impl:     impl,
		},
		Args: []string{"provided"},
		ArgDefs: []invowkfile.Argument{
			{Name: "first"},
			{Name: "second", DefaultValue: "fallback"},
		},
	})
	if err != nil {
		t.Fatalf("BuildExecutionContext(boundaries) error = %v", err)
	}
	if got.SelectedPlatform != platform {
		t.Fatalf("SelectedPlatform = %q, want %q", got.SelectedPlatform, platform)
	}
	if got.Env.ExtraEnv["INVOWK_PLATFORM"] != string(platform) {
		t.Fatalf("INVOWK_PLATFORM = %q, want %q", got.Env.ExtraEnv["INVOWK_PLATFORM"], platform)
	}
	if got.Env.ExtraEnv["INVOWK_ARG_FIRST"] != "provided" {
		t.Fatalf("INVOWK_ARG_FIRST = %q, want provided", got.Env.ExtraEnv["INVOWK_ARG_FIRST"])
	}
	if got.Env.ExtraEnv["INVOWK_ARG_SECOND"] != "fallback" {
		t.Fatalf("INVOWK_ARG_SECOND = %q, want fallback", got.Env.ExtraEnv["INVOWK_ARG_SECOND"])
	}
}

func TestApplyEnvInheritOverridesMutationContracts(t *testing.T) {
	t.Parallel()

	execCtx := newExecuteMutationExecutionContext(t)
	applyEnvInheritOverrides(BuildExecutionContextOptions{
		EnvInheritMode:  invowkfile.EnvInheritAllow,
		EnvInheritAllow: []invowkfile.EnvVarName{"HOME"},
		EnvInheritDeny:  []invowkfile.EnvVarName{"SECRET"},
	}, execCtx)
	if execCtx.Env.InheritModeOverride != invowkfile.EnvInheritAllow {
		t.Fatalf("InheritModeOverride = %q, want allow", execCtx.Env.InheritModeOverride)
	}
	if !slices.Equal(execCtx.Env.InheritAllowOverride, []invowkfile.EnvVarName{"HOME"}) {
		t.Fatalf("InheritAllowOverride = %v, want [HOME]", execCtx.Env.InheritAllowOverride)
	}
	if !slices.Equal(execCtx.Env.InheritDenyOverride, []invowkfile.EnvVarName{"SECRET"}) {
		t.Fatalf("InheritDenyOverride = %v, want [SECRET]", execCtx.Env.InheritDenyOverride)
	}
}

func TestBuildExecutionContextMutationRejectsInvalidEnvInheritMode(t *testing.T) {
	t.Parallel()

	err := buildExecuteMutationContextError(t, BuildExecutionContextOptions{
		EnvInheritMode: invowkfile.EnvInheritMode("bogus"),
	})
	if err == nil {
		t.Fatal("BuildExecutionContext(invalid mode) error = nil")
	}
	invalidErr := requireInvalidBuildExecutionContextOptionsError(t, err)
	if !executeFieldErrorsContain(invalidErr.FieldErrors, invowkfile.ErrInvalidEnvInheritMode) {
		t.Fatalf("BuildExecutionContext(invalid mode) field errors = %v, want ErrInvalidEnvInheritMode", invalidErr.FieldErrors)
	}
}

func TestBuildExecutionContextMutationRejectsInvalidEnvInheritNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts BuildExecutionContextOptions
	}{
		{
			name: "allow",
			opts: BuildExecutionContextOptions{
				EnvInheritAllow: []invowkfile.EnvVarName{"1BAD"},
			},
		},
		{
			name: "deny",
			opts: BuildExecutionContextOptions{
				EnvInheritDeny: []invowkfile.EnvVarName{"BAD-NAME"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := buildExecuteMutationContextError(t, tt.opts)
			if err == nil {
				t.Fatalf("BuildExecutionContext(invalid %s) error = nil", tt.name)
			}
			invalidErr := requireInvalidBuildExecutionContextOptionsError(t, err)
			if !executeFieldErrorsContain(invalidErr.FieldErrors, invowkfile.ErrInvalidEnvVarName) {
				t.Fatalf(
					"BuildExecutionContext(invalid %s) field errors = %v, want ErrInvalidEnvVarName",
					tt.name,
					invalidErr.FieldErrors,
				)
			}
		})
	}
}

func buildExecuteMutationContextError(t *testing.T, opts BuildExecutionContextOptions) error {
	t.Helper()

	cmd := &invowkfile.Command{Name: "deploy"}
	inv := &invowkfile.Invowkfile{}
	impl := &invowkfile.Implementation{}
	opts.Command = cmd
	opts.Invowkfile = inv
	opts.Selection = RuntimeSelection{
		mode: invowkfile.RuntimeNative,
		impl: impl,
	}
	_, err := BuildExecutionContext(t.Context(), opts)
	return err
}

func newExecuteMutationExecutionContext(t *testing.T) *runtimepkg.ExecutionContext {
	t.Helper()

	cmd := &invowkfile.Command{Name: "deploy"}
	inv := &invowkfile.Invowkfile{}
	return runtimepkg.NewExecutionContext(t.Context(), cmd, inv)
}

func executeMutationCommand() *invowkfile.Command {
	return executeMutationCommandWithRuntimes(invowkfile.RuntimeNative, invowkfile.RuntimeVirtualSh)
}

func executeMutationCommandWithRuntimes(runtimes ...invowkfile.RuntimeMode) *invowkfile.Command {
	runtimeConfigs := make([]invowkfile.RuntimeConfig, 0, len(runtimes))
	for _, mode := range runtimes {
		runtimeConfigs = append(runtimeConfigs, invowkfile.RuntimeConfig{Name: mode})
	}
	return &invowkfile.Command{
		Name: "deploy",
		Implementations: []invowkfile.Implementation{{
			Runtimes:  runtimeConfigs,
			Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
		}},
	}
}

func executeMutationNonHostPlatform() invowkfile.Platform {
	switch invowkfile.CurrentPlatform() {
	case invowkfile.PlatformLinux:
		return invowkfile.PlatformMac
	default:
		return invowkfile.PlatformLinux
	}
}
