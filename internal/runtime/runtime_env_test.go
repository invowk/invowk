// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// testCommandWithScript creates a Command with a single script for testing.
// This helper is shared across test files in the runtime package.
func testCommandWithScript(name, script string, runtime invowkfile.RuntimeMode) *invowkfile.Command {
	return &invowkfile.Command{
		Name: name,
		Implementations: []invowkfile.Implementation{
			{Script: script, Runtimes: []invowkfile.RuntimeConfig{{Name: runtime}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
		},
	}
}

// testCommandWithInterpreter creates a Command with a script and explicit interpreter.
// This helper is shared across test files in the runtime package.
func testCommandWithInterpreter(name, script, interpreter string, runtime invowkfile.RuntimeMode) *invowkfile.Command {
	return &invowkfile.Command{
		Name: name,
		Implementations: []invowkfile.Implementation{
			{
				Script:    script,
				Runtimes:  []invowkfile.RuntimeConfig{{Name: runtime, Interpreter: interpreter}},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}},
			},
		},
	}
}

func TestRuntime_ScriptNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("missing", "./nonexistent.sh", invowkfile.RuntimeNative)

	t.Run("native runtime", func(t *testing.T) {
		rt := NewNativeRuntime()
		ctx := NewExecutionContext(context.Background(), cmd, inv)
		ctx.IO.Stdout = &bytes.Buffer{}
		ctx.IO.Stderr = &bytes.Buffer{}

		result := rt.Execute(ctx)
		if result.Error == nil {
			t.Error("Execute() expected error for missing script file, got nil")
		}
	})

	t.Run("virtual runtime", func(t *testing.T) {
		cmdVirtual := testCommandWithScript("missing", "./nonexistent.sh", invowkfile.RuntimeVirtual)
		rt := NewVirtualRuntime(false)
		ctx := NewExecutionContext(context.Background(), cmdVirtual, inv)

		ctx.IO.Stdout = &bytes.Buffer{}
		ctx.IO.Stderr = &bytes.Buffer{}

		result := rt.Execute(ctx)
		if result.Error == nil {
			t.Error("Execute() expected error for missing script file, got nil")
		}
	})
}

func TestRuntime_EnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	currentPlatform := invowkfile.CurrentPlatform()
	cmd := &invowkfile.Command{
		Name: "env-test",
		Implementations: []invowkfile.Implementation{
			{
				Script: `echo "Impl: $IMPL_VAR, Command: $CMD_VAR"`,

				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
				Platforms: []invowkfile.PlatformConfig{{Name: currentPlatform}},
				Env:       &invowkfile.EnvConfig{Vars: map[string]string{"IMPL_VAR": "impl_value"}},
			},
		},
		Env: &invowkfile.EnvConfig{
			Vars: map[string]string{
				"CMD_VAR": "command_value",
			},
		},
	}

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "impl_value") {
		t.Errorf("Execute() output missing impl env var, got: %q", output)
	}
	if !strings.Contains(output, "command_value") {
		t.Errorf("Execute() output missing command env var, got: %q", output)
	}
}

func TestFilterInvowkEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    []string
	}{
		{
			name:    "empty environment",
			environ: []string{},
			want:    []string{},
		},
		{
			name:    "no invowk vars",
			environ: []string{"PATH=/usr/bin", "HOME=/home/user", "SHELL=/bin/bash"},
			want:    []string{"PATH=/usr/bin", "HOME=/home/user", "SHELL=/bin/bash"},
		},
		{
			name:    "filter INVOWK_ARG_ vars",
			environ: []string{"PATH=/usr/bin", "INVOWK_ARG_NAME=value", "INVOWK_ARG_FILE=test.txt"},
			want:    []string{"PATH=/usr/bin"},
		},
		{
			name:    "filter INVOWK_FLAG_ vars",
			environ: []string{"HOME=/home/user", "INVOWK_FLAG_VERBOSE=true", "INVOWK_FLAG_DRY_RUN=false"},
			want:    []string{"HOME=/home/user"},
		},
		{
			name:    "filter ARGC var",
			environ: []string{"PATH=/usr/bin", "ARGC=3", "SHELL=/bin/bash"},
			want:    []string{"PATH=/usr/bin", "SHELL=/bin/bash"},
		},
		{
			name:    "filter ARGn vars",
			environ: []string{"PATH=/usr/bin", "ARG1=first", "ARG2=second", "ARG10=tenth"},
			want:    []string{"PATH=/usr/bin"},
		},
		{
			name:    "keep ARG prefix with non-digits",
			environ: []string{"PATH=/usr/bin", "ARGS=all", "ARGNAME=test", "ARG_COUNT=5"},
			want:    []string{"PATH=/usr/bin", "ARGS=all", "ARGNAME=test", "ARG_COUNT=5"},
		},
		{
			name:    "mixed filtering",
			environ: []string{"PATH=/usr/bin", "INVOWK_ARG_X=1", "HOME=/home", "ARGC=2", "ARG1=a", "INVOWK_FLAG_Y=2", "USER=test"},
			want:    []string{"PATH=/usr/bin", "HOME=/home", "USER=test"},
		},
		{
			name:    "malformed env var kept",
			environ: []string{"PATH=/usr/bin", "MALFORMED", "HOME=/home/user"},
			want:    []string{"PATH=/usr/bin", "MALFORMED", "HOME=/home/user"},
		},
		{
			name:    "empty value preserved",
			environ: []string{"EMPTY=", "INVOWK_ARG_EMPTY="},
			want:    []string{"EMPTY="},
		},
		{
			name:    "value with equals sign",
			environ: []string{"CONFIG=key=value", "INVOWK_ARG_TEST=foo=bar"},
			want:    []string{"CONFIG=key=value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterInvowkEnvVars(tt.environ)
			if !slices.Equal(got, tt.want) {
				t.Errorf("FilterInvowkEnvVars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldFilterEnvVar(t *testing.T) {
	tests := []struct {
		envVar string
		want   bool
	}{
		// INVOWK_ARG_* cases
		{"INVOWK_ARG_NAME", true},
		{"INVOWK_ARG_X", true},
		{"INVOWK_ARG_LONG_NAME", true},
		{"INVOWK_ARG_", true},

		// INVOWK_FLAG_* cases
		{"INVOWK_FLAG_VERBOSE", true},
		{"INVOWK_FLAG_V", true},
		{"INVOWK_FLAG_", true},

		// Metadata env vars (injected by projectEnvVars, constants from pkg/platform)
		{"INVOWK_CMD_NAME", true},
		{"INVOWK_RUNTIME", true},
		{"INVOWK_SOURCE", true},
		{"INVOWK_PLATFORM", true},

		// ARGC case
		{"ARGC", true},

		// ARGn cases
		{"ARG1", true},
		{"ARG2", true},
		{"ARG10", true},
		{"ARG999", true},
		{"ARG0", true},

		// Should NOT be filtered
		{"PATH", false},
		{"HOME", false},
		{"INVOWK", false},
		{"INVOWK_", false},
		{"INVOWK_OTHER", false},
		{"ARG", false},
		{"ARGS", false},
		{"ARGNAME", false},
		{"ARG_1", false},
		{"ARG1NAME", false},
		{"MY_ARGC", false},
		{"MY_ARG1", false},
		{"INVOWK_ARGS", false},
	}

	for _, tt := range tests {
		t.Run(tt.envVar, func(t *testing.T) {
			got := shouldFilterEnvVar(tt.envVar)
			if got != tt.want {
				t.Errorf("shouldFilterEnvVar(%q) = %v, want %v", tt.envVar, got, tt.want)
			}
		})
	}
}
