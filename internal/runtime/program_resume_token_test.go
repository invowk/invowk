// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestProgramResumeTokenHelpers(t *testing.T) {
	t.Parallel()

	// Nil safety
	StripProgramResumeToken(nil)
	if got := FilterProgramResumeToken(nil); got != nil {
		t.Fatalf("FilterProgramResumeToken(nil) = %v, want nil", got)
	}

	envMap := map[string]string{
		ansi.ProgramResumeTokenEnv: "secret-token",
		"OSC7577_TOKEN_EXTRA":      "lookalike-kept",
		"KEEP":                     "yes",
	}
	StripProgramResumeToken(envMap)
	if _, ok := envMap[ansi.ProgramResumeTokenEnv]; ok {
		t.Fatalf("StripProgramResumeToken did not remove %s", ansi.ProgramResumeTokenEnv)
	}
	if envMap["OSC7577_TOKEN_EXTRA"] != "lookalike-kept" || envMap["KEEP"] != "yes" {
		t.Fatalf("StripProgramResumeToken altered unrelated keys: %v", envMap)
	}

	filtered := FilterProgramResumeToken([]string{
		"KEEP=1",
		ansi.ProgramResumeTokenEnv + "=secret-token",
		ansi.ProgramResumeTokenEnv + "=",
		ansi.ProgramResumeTokenEnv + "=duplicate",
		"OSC7577_TOKEN_EXTRA=lookalike",
		"MY_OSC7577_TOKEN=prefix-lookalike",
		ansi.ProgramResumeTokenEnv, // malformed without '='
		"OTHER=2",
		"MALFORMED",
	})
	want := []string{
		"KEEP=1",
		"OSC7577_TOKEN_EXTRA=lookalike",
		"MY_OSC7577_TOKEN=prefix-lookalike",
		ansi.ProgramResumeTokenEnv,
		"OTHER=2",
		"MALFORMED",
	}
	if !slices.Equal(filtered, want) {
		t.Fatalf("FilterProgramResumeToken() = %v, want %v", filtered, want)
	}
}

func TestContainerRuntimeStripsProgramResumeTokenUnconditionally(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inheritMode    invowkfile.EnvInheritMode
		inheritAllow   []invowkfile.EnvVarName
		configEnvVars  map[invowkfile.EnvVarName]string
		runtimeEnvVars map[string]string
		wantCustom     string
	}{
		{
			name:        "env_inherit_mode all",
			inheritMode: invowkfile.EnvInheritAll,
			wantCustom:  "visible",
		},
		{
			name:         "explicit env_inherit_allow naming OSC7577_TOKEN",
			inheritMode:  invowkfile.EnvInheritAllow,
			inheritAllow: []invowkfile.EnvVarName{invowkfile.EnvVarName(ansi.ProgramResumeTokenEnv), "CUSTOM_ALLOWED"},
			wantCustom:   "visible",
		},
		{
			name:           "env.vars and --ivk-env-var overrides",
			inheritMode:    invowkfile.EnvInheritNone,
			configEnvVars:  map[invowkfile.EnvVarName]string{invowkfile.EnvVarName(ansi.ProgramResumeTokenEnv): "from-cue"},
			runtimeEnvVars: map[string]string{ansi.ProgramResumeTokenEnv: "from-cli", "CUSTOM_ALLOWED": "visible"},
			wantCustom:     "visible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			inv := &invowkfile.Invowkfile{
				FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
			}
			cmd := &invowkfile.Command{
				Name: "container-token-test",
				Implementations: []invowkfile.Implementation{{
					Script: invowkfile.ImplementationScript{Content: "env"},
					Env:    &invowkfile.EnvConfig{Vars: tt.configEnvVars},
					Runtimes: []invowkfile.RuntimeConfig{{
						Name:            invowkfile.RuntimeContainer,
						Image:           "debian:stable-slim",
						EnvInheritMode:  tt.inheritMode,
						EnvInheritAllow: tt.inheritAllow,
					}},
					Platforms: invowkfile.AllPlatformConfigs(),
				}},
			}

			engine := NewMockEngine()
			envBuilder := &DefaultEnvBuilder{
				Environ: func() []string {
					return []string{
						ansi.ProgramResumeTokenEnv + "=secret-container-token",
						"CUSTOM_ALLOWED=visible",
					}
				},
			}
			rt, err := NewContainerRuntimeWithEngine(engine, WithContainerEnvBuilder(envBuilder))
			if err != nil {
				t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
			}

			ctx := NewExecutionContext(t.Context(), cmd, inv)
			ctx.SelectedRuntime = invowkfile.RuntimeContainer
			ctx.SelectedImpl = &cmd.Implementations[0]
			ctx.Env.RuntimeEnvVars = tt.runtimeEnvVars

			result := rt.ExecuteCapture(ctx)
			if result.ExitCode != 0 || result.Error != nil {
				t.Fatalf("ExecuteCapture() = exit %d, err %v", result.ExitCode, result.Error)
			}
			if len(engine.RunCalls) != 1 {
				t.Fatalf("RunCalls = %d, want 1", len(engine.RunCalls))
			}
			gotEnv := engine.RunCalls[0].Env
			if _, ok := gotEnv[ansi.ProgramResumeTokenEnv]; ok {
				t.Fatalf("container env leaked %s", ansi.ProgramResumeTokenEnv)
			}
			if gotEnv["CUSTOM_ALLOWED"] != tt.wantCustom {
				t.Fatalf("container env CUSTOM_ALLOWED = %q, want %q", gotEnv["CUSTOM_ALLOWED"], tt.wantCustom)
			}
		})
	}
}

func TestVirtualRuntimesStripProgramResumeToken(t *testing.T) {
	t.Parallel()

	envBuilder := &DefaultEnvBuilder{
		Environ: func() []string {
			return []string{
				ansi.ProgramResumeTokenEnv + "=secret-virtual-token",
				"OTHER_HOST_VAR=kept",
			}
		},
	}

	t.Run("virtual-sh non-interactive", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
		cmd := testCommandWithScript("sh-token", `printf '%s|%s' "${OSC7577_TOKEN:-unset}" "$OTHER_HOST_VAR"`, invowkfile.RuntimeVirtualSh)
		ctx := NewExecutionContext(t.Context(), cmd, inv)
		var stdout bytes.Buffer
		ctx.IO.Stdout = &stdout
		ctx.IO.Stderr = &bytes.Buffer{}

		rt := NewShRuntime(false, WithShEnvBuilder(envBuilder))
		result := rt.Execute(ctx)
		if result.ExitCode != 0 || result.Error != nil {
			t.Fatalf("ShRuntime.Execute() = exit %d, err %v", result.ExitCode, result.Error)
		}
		if got := strings.TrimSpace(stdout.String()); got != "unset|kept" {
			t.Fatalf("stdout = %q, want %q", got, "unset|kept")
		}
	})

	t.Run("virtual-lua non-interactive", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
		cmd := testCommandWithScript("lua-token", `print(tostring(os.getenv("OSC7577_TOKEN")) .. "|" .. tostring(os.getenv("OTHER_HOST_VAR")))`, invowkfile.RuntimeVirtualLua)
		ctx := NewExecutionContext(t.Context(), cmd, inv)
		var stdout bytes.Buffer
		ctx.IO.Stdout = &stdout
		ctx.IO.Stderr = &bytes.Buffer{}

		rt := NewLuaRuntime(false, WithLuaEnvBuilder(envBuilder))
		result := rt.Execute(ctx)
		if result.ExitCode != 0 || result.Error != nil {
			t.Fatalf("LuaRuntime.Execute() = exit %d, err %v", result.ExitCode, result.Error)
		}
		if got := strings.TrimSpace(stdout.String()); got != "nil|kept" {
			t.Fatalf("stdout = %q, want %q", got, "nil|kept")
		}
	})

	t.Run("virtual interactive subprocess envJSON", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
		cmd := testCommandWithScript("virt-interactive", "echo hi", invowkfile.RuntimeVirtualSh)
		ctx := NewExecutionContext(t.Context(), cmd, inv)

		prep, err := prepareVirtualInteractiveSubprocess(ctx, "echo hi", "invowk-test-*.sh", "sh", "sh", envBuilder)
		if err != nil {
			t.Fatalf("prepareVirtualInteractiveSubprocess() error = %v", err)
		}
		t.Cleanup(prep.cleanup)

		var decoded map[string]string
		if err := json.Unmarshal(prep.envJSON, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(envJSON) error = %v", err)
		}
		if _, ok := decoded[ansi.ProgramResumeTokenEnv]; ok {
			t.Fatalf("virtual interactive envJSON leaked %s", ansi.ProgramResumeTokenEnv)
		}
		if decoded["OTHER_HOST_VAR"] != "kept" {
			t.Fatalf("envJSON OTHER_HOST_VAR = %q, want %q", decoded["OTHER_HOST_VAR"], "kept")
		}
	})

	t.Run("RunShScript and RunLuaScript strip OSC7577_TOKEN at runner boundary", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		shOutFile, err := os.CreateTemp(tmpDir, "sh-stdout-*")
		if err != nil {
			t.Fatalf("os.CreateTemp() error = %v", err)
		}
		t.Cleanup(func() { _ = shOutFile.Close() })
		shErrFile, err := os.CreateTemp(tmpDir, "sh-stderr-*")
		if err != nil {
			t.Fatalf("os.CreateTemp() error = %v", err)
		}
		t.Cleanup(func() { _ = shErrFile.Close() })

		if runErr := RunShScript(t.Context(), ShScriptOptions{
			Script:           `printf '%s|%s' "${OSC7577_TOKEN:-unset}" "$KEEP_VAR"`,
			ScriptName:       "sh-boundary",
			WorkDir:          tmpDir,
			ScriptBasePath:   tmpDir,
			Env:              []string{ansi.ProgramResumeTokenEnv + "=direct-secret", "KEEP_VAR=ok"},
			FilesystemAccess: invowkfile.VirtualFilesystemAccessRestricted,
			Stdout:           shOutFile,
			Stderr:           shErrFile,
		}); runErr != nil {
			t.Fatalf("RunShScript() error = %v", runErr)
		}
		shOutBytes, readErr := os.ReadFile(shOutFile.Name())
		if readErr != nil {
			t.Fatalf("os.ReadFile() error = %v", readErr)
		}
		if got := strings.TrimSpace(string(shOutBytes)); got != "unset|ok" {
			t.Fatalf("RunShScript stdout = %q, want %q", got, "unset|ok")
		}

		var luaOut bytes.Buffer
		if luaErr := RunLuaScript(t.Context(), LuaScriptOptions{
			Script:           `print(tostring(os.getenv("OSC7577_TOKEN")) .. "|" .. tostring(os.getenv("KEEP_VAR")))`,
			ScriptName:       "lua-boundary",
			WorkDir:          tmpDir,
			ScriptBasePath:   tmpDir,
			Env:              []string{ansi.ProgramResumeTokenEnv + "=direct-secret", "KEEP_VAR=ok"},
			FilesystemAccess: invowkfile.VirtualFilesystemAccessRestricted,
			Stdout:           &luaOut,
			Stderr:           &bytes.Buffer{},
		}); luaErr != nil {
			t.Fatalf("RunLuaScript() error = %v", luaErr)
		}
		if got := strings.TrimSpace(luaOut.String()); got != "nil|ok" {
			t.Fatalf("RunLuaScript stdout = %q, want %q", got, "nil|ok")
		}
	})
}

func TestNativeRuntimePreservesProgramResumeTokenAndRespectsDeny(t *testing.T) {
	t.Parallel()

	envBuilder := &DefaultEnvBuilder{
		Environ: func() []string {
			return []string{ansi.ProgramResumeTokenEnv + "=native-token"}
		},
	}

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
	cmd := testCommandWithScript("native-token", "echo hi", invowkfile.RuntimeNative)
	rt := NewNativeRuntime(WithEnvBuilder(envBuilder))

	t.Run("preserves by default", func(t *testing.T) {
		t.Parallel()

		ctx := NewExecutionContext(t.Context(), cmd, inv)
		prep, err := rt.PrepareInteractive(ctx)
		if err != nil {
			t.Fatalf("PrepareInteractive() error = %v", err)
		}
		if !slices.Contains(prep.Cmd.Env, ansi.ProgramResumeTokenEnv+"=native-token") {
			t.Fatalf("native command dropped %s", ansi.ProgramResumeTokenEnv)
		}
	})

	t.Run("respects env_inherit_deny", func(t *testing.T) {
		t.Parallel()

		ctx := NewExecutionContext(t.Context(), cmd, inv)
		ctx.Env.InheritDenyOverride = []invowkfile.EnvVarName{invowkfile.EnvVarName(ansi.ProgramResumeTokenEnv)}
		prep, err := rt.PrepareInteractive(ctx)
		if err != nil {
			t.Fatalf("PrepareInteractive() error = %v", err)
		}
		for _, kv := range prep.Cmd.Env {
			if strings.HasPrefix(kv, ansi.ProgramResumeTokenEnv+"=") {
				t.Fatalf("native command leaked denied %s", ansi.ProgramResumeTokenEnv)
			}
		}
	})
}
