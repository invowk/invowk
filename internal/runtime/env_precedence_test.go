// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"path/filepath"
	"testing"

	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
)

// TestBuildRuntimeEnv_PairwisePrecedence verifies that each adjacent precedence level
// correctly overrides the one below it. The 10-level hierarchy is:
//
//  1. Host environment (filtered by inherit mode)
//  2. Root-level env.files
//  3. Command-level env.files
//  4. Implementation-level env.files
//  5. Root-level env.vars
//  6. Command-level env.vars
//  7. Implementation-level env.vars
//  8. ExtraEnv (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn, ARGC)
//  9. RuntimeEnvFiles (--env-file flag)
//  10. RuntimeEnvVars (--env-var flag) - HIGHEST priority
func TestBuildRuntimeEnv_PairwisePrecedence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create env files for levels 2, 3, 4, and 9
	createEnvFile(t, tmpDir, "root.env", "KEY=level2_root_file")
	createEnvFile(t, tmpDir, "cmd.env", "KEY=level3_cmd_file")
	createEnvFile(t, tmpDir, "impl.env", "KEY=level4_impl_file")
	createEnvFile(t, tmpDir, "runtime.env", "KEY=level9_runtime_file")

	currentPlatform := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		setupCtx func(tmpDir string) (*ExecutionContext, func())
		wantVal  string
	}{
		{
			name: "level 2 root files override level 1 host env",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				restore := testutil.MustSetenv(t, "KEY", "level1_host")
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
					Env:      &invkfile.EnvConfig{Files: []string{"root.env"}},
				}
				cmd := testCommandWithScript("test", "echo test", invkfile.RuntimeNative)
				ctx := NewExecutionContext(cmd, inv)
				return ctx, restore
			},
			wantVal: "level2_root_file",
		},
		{
			name: "level 3 cmd files override level 2 root files",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
					Env:      &invkfile.EnvConfig{Files: []string{"root.env"}},
				}
				cmd := &invkfile.Command{
					Name: "test",
					Implementations: []invkfile.Implementation{{
						Script:    "echo test",
						Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
					}},
					Env: &invkfile.EnvConfig{Files: []string{"cmd.env"}},
				}
				ctx := NewExecutionContext(cmd, inv)
				return ctx, func() {}
			},
			wantVal: "level3_cmd_file",
		},
		{
			name: "level 4 impl files override level 3 cmd files",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				}
				cmd := &invkfile.Command{
					Name: "test",
					Implementations: []invkfile.Implementation{{
						Script:    "echo test",
						Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
						Env:       &invkfile.EnvConfig{Files: []string{"impl.env"}},
					}},
					Env: &invkfile.EnvConfig{Files: []string{"cmd.env"}},
				}
				ctx := NewExecutionContext(cmd, inv)
				return ctx, func() {}
			},
			wantVal: "level4_impl_file",
		},
		{
			name: "level 5 root vars override level 2 root files",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
					Env: &invkfile.EnvConfig{
						Files: []string{"root.env"},
						Vars:  map[string]string{"KEY": "level5_root_var"},
					},
				}
				cmd := testCommandWithScript("test", "echo test", invkfile.RuntimeNative)
				ctx := NewExecutionContext(cmd, inv)
				return ctx, func() {}
			},
			wantVal: "level5_root_var",
		},
		{
			name: "level 6 cmd vars override level 5 root vars",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
					Env: &invkfile.EnvConfig{
						Vars: map[string]string{"KEY": "level5_root_var"},
					},
				}
				cmd := &invkfile.Command{
					Name: "test",
					Implementations: []invkfile.Implementation{{
						Script:    "echo test",
						Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
					}},
					Env: &invkfile.EnvConfig{Vars: map[string]string{"KEY": "level6_cmd_var"}},
				}
				ctx := NewExecutionContext(cmd, inv)
				return ctx, func() {}
			},
			wantVal: "level6_cmd_var",
		},
		{
			name: "level 7 impl vars override level 6 cmd vars",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				}
				cmd := &invkfile.Command{
					Name: "test",
					Implementations: []invkfile.Implementation{{
						Script:    "echo test",
						Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
						Env:       &invkfile.EnvConfig{Vars: map[string]string{"KEY": "level7_impl_var"}},
					}},
					Env: &invkfile.EnvConfig{Vars: map[string]string{"KEY": "level6_cmd_var"}},
				}
				ctx := NewExecutionContext(cmd, inv)
				return ctx, func() {}
			},
			wantVal: "level7_impl_var",
		},
		{
			name: "level 8 extra env overrides level 7 impl vars",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				}
				cmd := &invkfile.Command{
					Name: "test",
					Implementations: []invkfile.Implementation{{
						Script:    "echo test",
						Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
						Env:       &invkfile.EnvConfig{Vars: map[string]string{"KEY": "level7_impl_var"}},
					}},
				}
				ctx := NewExecutionContext(cmd, inv)
				ctx.Env.ExtraEnv["KEY"] = "level8_extra"
				return ctx, func() {}
			},
			wantVal: "level8_extra",
		},
		{
			name: "level 9 runtime files override level 8 extra env",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				}
				cmd := testCommandWithScript("test", "echo test", invkfile.RuntimeNative)
				ctx := NewExecutionContext(cmd, inv)
				ctx.Env.ExtraEnv["KEY"] = "level8_extra"
				ctx.Env.RuntimeEnvFiles = []string{filepath.Join(tmpDir, "runtime.env")}
				return ctx, func() {}
			},
			wantVal: "level9_runtime_file",
		},
		{
			name: "level 10 runtime vars override level 9 runtime files",
			setupCtx: func(tmpDir string) (*ExecutionContext, func()) {
				inv := &invkfile.Invkfile{
					FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				}
				cmd := testCommandWithScript("test", "echo test", invkfile.RuntimeNative)
				ctx := NewExecutionContext(cmd, inv)
				ctx.Env.RuntimeEnvFiles = []string{filepath.Join(tmpDir, "runtime.env")}
				ctx.Env.RuntimeEnvVars = map[string]string{"KEY": "level10_runtime_var"}
				return ctx, func() {}
			},
			wantVal: "level10_runtime_var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cleanup := tt.setupCtx(tmpDir)
			defer cleanup()

			env, err := NewDefaultEnvBuilder().Build(ctx, invkfile.EnvInheritAll)
			if err != nil {
				t.Fatalf("Build() error: %v", err)
			}

			if got := env["KEY"]; got != tt.wantVal {
				t.Errorf("KEY = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

// TestBuildRuntimeEnv_NilEnvConfigs verifies that nil or empty Env configs at each
// level do not cause panics or incorrect behavior. The env builder must handle
// nil EnvConfig pointers gracefully via the GetFiles()/GetVars() nil-safe accessors.
func TestBuildRuntimeEnv_NilEnvConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	currentPlatform := invkfile.GetCurrentHostOS()

	tests := []struct {
		name    string
		inv     *invkfile.Invkfile
		cmd     *invkfile.Command
		wantErr bool
	}{
		{
			name: "nil root env config",
			inv: &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				Env:      nil,
			},
			cmd: testCommandWithScript("test", "echo test", invkfile.RuntimeNative),
		},
		{
			name: "nil command env config",
			inv: &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
			},
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{{
					Script:    "echo test",
					Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
					Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
				}},
				Env: nil,
			},
		},
		{
			name: "nil implementation env config",
			inv: &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
			},
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{{
					Script:    "echo test",
					Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
					Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
					Env:       nil,
				}},
			},
		},
		{
			name: "all env configs nil",
			inv: &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				Env:      nil,
			},
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{{
					Script:    "echo test",
					Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
					Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
					Env:       nil,
				}},
				Env: nil,
			},
		},
		{
			name: "empty vars and files",
			inv: &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				Env:      &invkfile.EnvConfig{Vars: map[string]string{}, Files: []string{}},
			},
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{{
					Script:    "echo test",
					Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
					Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
					Env:       &invkfile.EnvConfig{Vars: map[string]string{}, Files: []string{}},
				}},
				Env: &invkfile.EnvConfig{Vars: map[string]string{}, Files: []string{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewExecutionContext(tt.cmd, tt.inv)

			env, err := NewDefaultEnvBuilder().Build(ctx, invkfile.EnvInheritNone)
			if tt.wantErr {
				if err == nil {
					t.Error("Build() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Build() unexpected error: %v", err)
			}
			if env == nil {
				t.Error("Build() returned nil env map")
			}
		})
	}
}

// TestBuildHostEnv_FiltersInvowkVars verifies that buildHostEnv excludes INVOWK_*
// variables even when inherit mode is "all".
func TestBuildHostEnv_FiltersInvowkVars(t *testing.T) {
	restoreArg := testutil.MustSetenv(t, "INVOWK_ARG_TEST", "should_be_filtered")
	restoreFlag := testutil.MustSetenv(t, "INVOWK_FLAG_TEST", "should_be_filtered")
	restoreKeep := testutil.MustSetenv(t, "KEEP_THIS_VAR", "visible")
	defer restoreArg()
	defer restoreFlag()
	defer restoreKeep()

	cfg := envInheritConfig{mode: invkfile.EnvInheritAll}
	env := buildHostEnv(cfg)

	if _, ok := env["INVOWK_ARG_TEST"]; ok {
		t.Error("buildHostEnv() should filter INVOWK_ARG_* variables")
	}
	if _, ok := env["INVOWK_FLAG_TEST"]; ok {
		t.Error("buildHostEnv() should filter INVOWK_FLAG_* variables")
	}
	if got := env["KEEP_THIS_VAR"]; got != "visible" {
		t.Errorf("buildHostEnv() KEEP_THIS_VAR = %q, want %q", got, "visible")
	}
}
