// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestResolveRuntime(t *testing.T) {
	t.Parallel()

	// Helper to create a command with specific runtime support
	mkCmd := func() *invowkfile.Command {
		return &invowkfile.Command{
			Name: "test-cmd",
			Implementations: []invowkfile.Implementation{
				{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeNative},
						{Name: invowkfile.RuntimeVirtual},
					},
					Platforms: []invowkfile.PlatformConfig{
						{Name: invowkfile.PlatformLinux},
						{Name: invowkfile.PlatformMac},
						{Name: invowkfile.PlatformWindows},
					},
				},
			},
		}
	}

	// Helper to create a command with ONLY virtual runtime.
	// Uses AllPlatformConfigs() so tests work on all CI platforms (Linux, macOS, Windows).
	mkVirtualOnlyCmd := func() *invowkfile.Command {
		return &invowkfile.Command{
			Name: "virtual-cmd",
			Implementations: []invowkfile.Implementation{
				{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeVirtual},
					},
					Platforms: invowkfile.AllPlatformConfigs(),
				},
			},
		}
	}

	tests := []struct {
		name                  string
		cmd                   *invowkfile.Command
		override              invowkfile.RuntimeMode
		cfg                   *config.Config
		wantMode              invowkfile.RuntimeMode
		wantErr               bool
		wantRuntimeNotAllowed bool
	}{
		{
			name:     "CLI override success",
			cmd:      mkCmd(),
			override: invowkfile.RuntimeVirtual,
			wantMode: invowkfile.RuntimeVirtual,
		},
		{
			name:                  "CLI override not allowed",
			cmd:                   mkVirtualOnlyCmd(),
			override:              invowkfile.RuntimeNative,
			wantErr:               true,
			wantRuntimeNotAllowed: true,
		},
		{
			name:     "Config default success",
			cmd:      mkCmd(),
			cfg:      &config.Config{DefaultRuntime: config.RuntimeVirtual},
			wantMode: invowkfile.RuntimeVirtual,
		},
		{
			name:     "Config default ignored if not allowed",
			cmd:      mkVirtualOnlyCmd(),
			cfg:      &config.Config{DefaultRuntime: config.RuntimeNative},
			wantMode: invowkfile.RuntimeVirtual, // Falls back to command default (virtual)
		},
		{
			name:     "CLI override invalid mode (defense-in-depth)",
			cmd:      mkCmd(),
			override: invowkfile.RuntimeMode("bogus"),
			wantErr:  true,
		},
		{
			name:    "Config default invalid mode (defense-in-depth)",
			cmd:     mkCmd(),
			cfg:     &config.Config{DefaultRuntime: config.RuntimeMode("magical")},
			wantErr: true,
		},
		{
			name:     "Command default (first listed)",
			cmd:      mkCmd(),
			wantMode: invowkfile.RuntimeNative, // First in list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveRuntime(tt.cmd, "test", tt.override, tt.cfg, invowkfile.PlatformLinux)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantRuntimeNotAllowed {
					var runtimeErr *RuntimeNotAllowedError
					if !errors.As(err, &runtimeErr) {
						t.Errorf("expected error type *RuntimeNotAllowedError, got %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Mode != tt.wantMode {
				t.Errorf("got mode %q, want %q", got.Mode, tt.wantMode)
			}
			if got.Impl == nil {
				t.Error("got nil implementation")
			}
		})
	}
}

func TestBuildExecutionContext(t *testing.T) {
	t.Parallel()

	cmd := &invowkfile.Command{Name: "test"}
	inv := &invowkfile.Invowkfile{}
	sel := RuntimeSelection{Mode: invowkfile.RuntimeNative, Impl: &invowkfile.Implementation{}}

	tests := []struct {
		name string
		opts BuildExecutionContextOptions
		want map[string]string // ExtraEnv checks
	}{
		{
			name: "Basic args projection",
			opts: BuildExecutionContextOptions{
				Command:    cmd,
				Invowkfile: inv,
				Selection:  sel,
				Args:       []string{"val1", "val2"},
			},
			want: map[string]string{
				"ARG1": "val1",
				"ARG2": "val2",
				"ARGC": "2",
			},
		},
		{
			name: "Named arguments",
			opts: BuildExecutionContextOptions{
				Command:    cmd,
				Invowkfile: inv,
				Selection:  sel,
				Args:       []string{"val1"},
				ArgDefs: []invowkfile.Argument{
					{Name: "first-arg"},
				},
			},
			want: map[string]string{
				"INVOWK_ARG_FIRST_ARG": "val1",
			},
		},
		{
			name: "Variadic arguments",
			opts: BuildExecutionContextOptions{
				Command:    cmd,
				Invowkfile: inv,
				Selection:  sel,
				Args:       []string{"v1", "v2", "v3"},
				ArgDefs: []invowkfile.Argument{
					{Name: "files", Variadic: true},
				},
			},
			want: map[string]string{
				"INVOWK_ARG_FILES":       "v1 v2 v3",
				"INVOWK_ARG_FILES_COUNT": "3",
				"INVOWK_ARG_FILES_1":     "v1",
				"INVOWK_ARG_FILES_2":     "v2",
				"INVOWK_ARG_FILES_3":     "v3",
			},
		},
		{
			name: "Flags projection",
			opts: BuildExecutionContextOptions{
				Command:    cmd,
				Invowkfile: inv,
				Selection:  sel,
				FlagValues: map[invowkfile.FlagName]string{
					"output-file": "/tmp/out",
					"verbose":     "true",
				},
			},
			want: map[string]string{
				"INVOWK_FLAG_OUTPUT_FILE": "/tmp/out",
				"INVOWK_FLAG_VERBOSE":     "true",
			},
		},
		{
			name: "Metadata env vars injected",
			opts: BuildExecutionContextOptions{
				Command:    cmd,
				Invowkfile: inv,
				Selection:  sel,
				SourceID:   "my-module",
				Platform:   invowkfile.PlatformLinux,
			},
			want: map[string]string{
				"INVOWK_CMD_NAME": "test",
				"INVOWK_RUNTIME":  "native",
				"INVOWK_SOURCE":   "my-module",
				"INVOWK_PLATFORM": "linux",
			},
		},
		{
			name: "Metadata env vars omitted when empty",
			opts: BuildExecutionContextOptions{
				Command:    cmd,
				Invowkfile: inv,
				Selection:  sel,
			},
			want: map[string]string{
				"INVOWK_CMD_NAME": "test",
				"INVOWK_RUNTIME":  "native",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotCtx, err := BuildExecutionContext(tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, v := range tt.want {
				if got, ok := gotCtx.Env.ExtraEnv[k]; !ok || got != v {
					t.Errorf("Env[%q] = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestBuildExecutionContext_MetadataOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	cmd := &invowkfile.Command{Name: "test"}
	inv := &invowkfile.Invowkfile{}
	sel := RuntimeSelection{Mode: invowkfile.RuntimeNative, Impl: &invowkfile.Implementation{}}

	gotCtx, err := BuildExecutionContext(BuildExecutionContextOptions{
		Command:    cmd,
		Invowkfile: inv,
		Selection:  sel,
		// SourceID and Platform deliberately left empty.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// INVOWK_CMD_NAME and INVOWK_RUNTIME are always set.
	if got := gotCtx.Env.ExtraEnv["INVOWK_CMD_NAME"]; got != "test" {
		t.Errorf("INVOWK_CMD_NAME = %q, want %q", got, "test")
	}
	if got := gotCtx.Env.ExtraEnv["INVOWK_RUNTIME"]; got != "native" {
		t.Errorf("INVOWK_RUNTIME = %q, want %q", got, "native")
	}

	// INVOWK_SOURCE and INVOWK_PLATFORM should NOT be present when SourceID/Platform are empty.
	if _, ok := gotCtx.Env.ExtraEnv["INVOWK_SOURCE"]; ok {
		t.Error("INVOWK_SOURCE should not be set when SourceID is empty")
	}
	if _, ok := gotCtx.Env.ExtraEnv["INVOWK_PLATFORM"]; ok {
		t.Error("INVOWK_PLATFORM should not be set when Platform is empty")
	}
}

func TestBuildExecutionContext_InheritanceValidation(t *testing.T) {
	t.Parallel()

	cmd := &invowkfile.Command{Name: "test"}
	inv := &invowkfile.Invowkfile{}
	sel := RuntimeSelection{Mode: invowkfile.RuntimeNative, Impl: &invowkfile.Implementation{}}

	tests := []struct {
		name    string
		opts    BuildExecutionContextOptions
		wantErr bool
	}{
		{
			name: "Valid inherit mode",
			opts: BuildExecutionContextOptions{
				Command:        cmd,
				Invowkfile:     inv,
				Selection:      sel,
				EnvInheritMode: invowkfile.EnvInheritNone,
			},
			wantErr: false,
		},
		{
			name: "Invalid inherit mode (defense-in-depth)",
			opts: BuildExecutionContextOptions{
				Command:        cmd,
				Invowkfile:     inv,
				Selection:      sel,
				EnvInheritMode: invowkfile.EnvInheritMode("invalid-mode"),
			},
			wantErr: true,
		},
		{
			name: "Invalid allow var name",
			opts: BuildExecutionContextOptions{
				Command:         cmd,
				Invowkfile:      inv,
				Selection:       sel,
				EnvInheritAllow: []invowkfile.EnvVarName{"INVALID-NAME!"},
			},
			wantErr: true,
		},
		{
			name: "Invalid deny var name",
			opts: BuildExecutionContextOptions{
				Command:        cmd,
				Invowkfile:     inv,
				Selection:      sel,
				EnvInheritDeny: []invowkfile.EnvVarName{"1START_WITH_NUMBER"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := BuildExecutionContext(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildExecutionContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewRuntimeSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    invowkfile.RuntimeMode
		impl    *invowkfile.Implementation
		wantErr string
	}{
		{
			name: "Valid mode and non-nil impl",
			mode: invowkfile.RuntimeNative,
			impl: &invowkfile.Implementation{},
		},
		{
			name:    "Nil impl returns error",
			mode:    invowkfile.RuntimeNative,
			impl:    nil,
			wantErr: "implementation must not be nil",
		},
		{
			name:    "Invalid mode returns error",
			mode:    invowkfile.RuntimeMode("bogus"),
			impl:    &invowkfile.Implementation{},
			wantErr: "invalid runtime mode",
		},
		{
			name:    "Empty mode returns error",
			mode:    invowkfile.RuntimeMode(""),
			impl:    &invowkfile.Implementation{},
			wantErr: "invalid runtime mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewRuntimeSelection(tt.mode, tt.impl)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr)) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Mode != tt.mode {
				t.Errorf("got mode %q, want %q", got.Mode, tt.mode)
			}
			if got.Impl != tt.impl {
				t.Error("got different impl pointer than expected")
			}
		})
	}
}

func TestRuntimeNotAllowedError_Format(t *testing.T) {
	t.Parallel()

	err := &RuntimeNotAllowedError{
		CommandName: "deploy",
		Runtime:     invowkfile.RuntimeContainer,
		Platform:    invowkfile.PlatformLinux,
		Allowed:     []invowkfile.RuntimeMode{invowkfile.RuntimeNative, invowkfile.RuntimeVirtual},
	}

	msg := err.Error()
	// Check for key components in the error message
	expectedSubstrings := []string{
		"runtime 'container' is not allowed",
		"command 'deploy'",
		"platform 'linux'",
		"native",
		"virtual",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(msg, sub) {
			t.Errorf("error message %q missing substring %q", msg, sub)
		}
	}
}
