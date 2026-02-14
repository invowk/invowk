// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// createEnvFile creates an env file with the given content and returns its path.
func createEnvFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create env file %s: %v", path, err)
	}
	return path
}

func TestDefaultEnvBuilder_InheritAllFiltersInvowkVars(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("env", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	builder := &DefaultEnvBuilder{
		Environ: func() []string {
			return []string{"INVOWK_ARG_PARENT=parent_value", "CUSTOM_HOST_ENV=host_value"}
		},
	}
	env, err := builder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("DefaultEnvBuilder.Build() error: %v", err)
	}

	if _, ok := env["INVOWK_ARG_PARENT"]; ok {
		t.Errorf("DefaultEnvBuilder.Build() should filter INVOWK_ARG_PARENT")
	}
	if got := env["CUSTOM_HOST_ENV"]; got != "host_value" {
		t.Errorf("DefaultEnvBuilder.Build() CUSTOM_HOST_ENV = %q, want %q", got, "host_value")
	}
}

func TestDefaultEnvBuilder_InheritAllowAndDeny(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("env", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Env.InheritModeOverride = invowkfile.EnvInheritAllow
	ctx.Env.InheritAllowOverride = []string{"ALLOW_ME", "DENY_ME"}
	ctx.Env.InheritDenyOverride = []string{"DENY_ME"}

	builder := &DefaultEnvBuilder{
		Environ: func() []string {
			return []string{"ALLOW_ME=allowed", "DENY_ME=denied"}
		},
	}
	env, err := builder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("DefaultEnvBuilder.Build() error: %v", err)
	}

	if got := env["ALLOW_ME"]; got != "allowed" {
		t.Errorf("DefaultEnvBuilder.Build() ALLOW_ME = %q, want %q", got, "allowed")
	}
	if _, ok := env["DENY_ME"]; ok {
		t.Errorf("DefaultEnvBuilder.Build() should deny DENY_ME")
	}
}

func TestValidateWorkDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a test file (not a directory)
	testFile := filepath.Join(tmpDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		dir       string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "empty string is valid (uses current dir)",
			dir:     "",
			wantErr: false,
		},
		{
			name:    "existing directory",
			dir:     tmpDir,
			wantErr: false,
		},
		{
			name:      "non-existent directory",
			dir:       filepath.Join(tmpDir, "nonexistent_subdir"),
			wantErr:   true,
			errSubstr: "does not exist",
		},
		{
			name:      "file instead of directory",
			dir:       testFile,
			wantErr:   true,
			errSubstr: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateWorkDir(tt.dir)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateWorkDir(%q) expected error, got nil", tt.dir)
				} else if tt.errSubstr != "" && !containsString(err.Error(), tt.errSubstr) {
					t.Errorf("validateWorkDir(%q) error = %q, want error containing %q", tt.dir, err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("validateWorkDir(%q) unexpected error: %v", tt.dir, err)
				}
			}
		})
	}
}

// TestResolveEnvInheritConfig tests the inheritance mode resolution logic.
func TestResolveEnvInheritConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	tests := []struct {
		name        string
		setupCtx    func(*ExecutionContext)
		defaultMode invowkfile.EnvInheritMode
		wantMode    invowkfile.EnvInheritMode
		wantAllow   []string
		wantDeny    []string
	}{
		{
			name:        "uses default mode when no overrides",
			setupCtx:    func(_ *ExecutionContext) {},
			defaultMode: invowkfile.EnvInheritAll,
			wantMode:    invowkfile.EnvInheritAll,
		},
		{
			name:        "default mode none",
			setupCtx:    func(_ *ExecutionContext) {},
			defaultMode: invowkfile.EnvInheritNone,
			wantMode:    invowkfile.EnvInheritNone,
		},
		{
			name: "context override takes precedence",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.Env.InheritModeOverride = invowkfile.EnvInheritNone
			},
			defaultMode: invowkfile.EnvInheritAll,
			wantMode:    invowkfile.EnvInheritNone,
		},
		{
			name: "context override allow mode with allowlist",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.Env.InheritModeOverride = invowkfile.EnvInheritAllow
				ctx.Env.InheritAllowOverride = []string{"PATH", "HOME"}
			},
			defaultMode: invowkfile.EnvInheritAll,
			wantMode:    invowkfile.EnvInheritAllow,
			wantAllow:   []string{"PATH", "HOME"},
		},
		{
			name: "context denylist override",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.Env.InheritDenyOverride = []string{"SECRET", "API_KEY"}
			},
			defaultMode: invowkfile.EnvInheritAll,
			wantMode:    invowkfile.EnvInheritAll,
			wantDeny:    []string{"SECRET", "API_KEY"},
		},
		{
			name: "runtime config mode used when no context override",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SelectedImpl.Runtimes = []invowkfile.RuntimeConfig{{
					Name:           invowkfile.RuntimeNative,
					EnvInheritMode: invowkfile.EnvInheritNone,
				}}
			},
			defaultMode: invowkfile.EnvInheritAll,
			wantMode:    invowkfile.EnvInheritNone,
		},
		{
			name: "context override beats runtime config",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SelectedImpl.Runtimes = []invowkfile.RuntimeConfig{{
					Name:           invowkfile.RuntimeNative,
					EnvInheritMode: invowkfile.EnvInheritNone,
				}}
				ctx.Env.InheritModeOverride = invowkfile.EnvInheritAll
			},
			defaultMode: invowkfile.EnvInheritNone,
			wantMode:    invowkfile.EnvInheritAll,
		},
		{
			name: "runtime config allowlist used when no context override",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SelectedImpl.Runtimes = []invowkfile.RuntimeConfig{{
					Name:            invowkfile.RuntimeNative,
					EnvInheritMode:  invowkfile.EnvInheritAllow,
					EnvInheritAllow: []string{"SHELL"},
				}}
			},
			defaultMode: invowkfile.EnvInheritAll,
			wantMode:    invowkfile.EnvInheritAllow,
			wantAllow:   []string{"SHELL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := testCommandWithScript("test", "echo test", invowkfile.RuntimeNative)
			ctx := NewExecutionContext(cmd, inv)
			tt.setupCtx(ctx)

			cfg := resolveEnvInheritConfig(ctx, tt.defaultMode)

			if cfg.mode != tt.wantMode {
				t.Errorf("resolveEnvInheritConfig() mode = %q, want %q", cfg.mode, tt.wantMode)
			}
			if len(cfg.allow) != len(tt.wantAllow) {
				t.Errorf("resolveEnvInheritConfig() allow = %v, want %v", cfg.allow, tt.wantAllow)
			}
			if len(cfg.deny) != len(tt.wantDeny) {
				t.Errorf("resolveEnvInheritConfig() deny = %v, want %v", cfg.deny, tt.wantDeny)
			}
		})
	}
}

// TestBuildHostEnv tests the host environment filtering logic.
func TestBuildHostEnv(t *testing.T) {
	t.Parallel()

	fakeEnviron := func() []string {
		return []string{
			"TEST_HOST_PATH=/usr/bin",
			"TEST_HOST_HOME=/home/user",
			"TEST_SECRET=confidential",
		}
	}

	tests := []struct {
		name     string
		cfg      envInheritConfig
		wantVars map[string]string
		dontWant []string
	}{
		{
			name: "inherit none returns empty",
			cfg: envInheritConfig{
				mode:    invowkfile.EnvInheritNone,
				environ: fakeEnviron,
			},
			wantVars: map[string]string{},
			dontWant: []string{"TEST_HOST_PATH", "TEST_HOST_HOME"},
		},
		{
			name: "inherit all includes host vars",
			cfg: envInheritConfig{
				mode:    invowkfile.EnvInheritAll,
				environ: fakeEnviron,
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
				"TEST_HOST_HOME": "/home/user",
			},
		},
		{
			name: "inherit allow only includes whitelisted",
			cfg: envInheritConfig{
				mode:    invowkfile.EnvInheritAllow,
				allow:   []string{"TEST_HOST_PATH"},
				environ: fakeEnviron,
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
			},
			dontWant: []string{"TEST_HOST_HOME"},
		},
		{
			name: "inherit all with denylist excludes denied",
			cfg: envInheritConfig{
				mode:    invowkfile.EnvInheritAll,
				deny:    []string{"TEST_SECRET"},
				environ: fakeEnviron,
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
				"TEST_HOST_HOME": "/home/user",
			},
			dontWant: []string{"TEST_SECRET"},
		},
		{
			name: "denylist takes precedence over allowlist",
			cfg: envInheritConfig{
				mode:    invowkfile.EnvInheritAllow,
				allow:   []string{"TEST_HOST_PATH", "TEST_SECRET"},
				deny:    []string{"TEST_SECRET"},
				environ: fakeEnviron,
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
			},
			dontWant: []string{"TEST_SECRET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := buildHostEnv(tt.cfg)

			for k, v := range tt.wantVars {
				if got := env[k]; got != v {
					t.Errorf("buildHostEnv() %s = %q, want %q", k, got, v)
				}
			}

			for _, k := range tt.dontWant {
				if _, ok := env[k]; ok {
					t.Errorf("buildHostEnv() should not include %s", k)
				}
			}
		})
	}
}

// TestBuildRuntimeEnv_Precedence tests the 10-level environment precedence hierarchy.
func TestBuildRuntimeEnv_Precedence(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create env files for different levels
	createEnvFile(t, tmpDir, "root.env", "SHARED=root_file\nROOT_FILE_ONLY=root_file")
	createEnvFile(t, tmpDir, "cmd.env", "SHARED=cmd_file\nCMD_FILE_ONLY=cmd_file")
	createEnvFile(t, tmpDir, "impl.env", "SHARED=impl_file\nIMPL_FILE_ONLY=impl_file")
	createEnvFile(t, tmpDir, "runtime.env", "SHARED=runtime_file\nRUNTIME_FILE_ONLY=runtime_file")

	currentPlatform := invowkfile.GetCurrentHostOS()
	cmd := &invowkfile.Command{
		Name: "precedence-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo test",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				Platforms: []invowkfile.PlatformConfig{{Name: currentPlatform}},
				Env: &invowkfile.EnvConfig{
					Files: []string{"impl.env"},
					Vars:  map[string]string{"SHARED": "impl_var", "IMPL_VAR_ONLY": "impl_var"},
				},
			},
		},
		Env: &invowkfile.EnvConfig{
			Files: []string{"cmd.env"},
			Vars:  map[string]string{"SHARED": "cmd_var", "CMD_VAR_ONLY": "cmd_var"},
		},
	}

	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
		Env: &invowkfile.EnvConfig{
			Files: []string{"root.env"},
			Vars:  map[string]string{"SHARED": "root_var", "ROOT_VAR_ONLY": "root_var"},
		},
	}

	ctx := NewExecutionContext(cmd, inv)
	ctx.Env.ExtraEnv = map[string]string{"SHARED": "extra_env", "EXTRA_ONLY": "extra_value"}
	ctx.Env.RuntimeEnvFiles = []string{filepath.Join(tmpDir, "runtime.env")}
	ctx.Env.RuntimeEnvVars = map[string]string{"SHARED": "runtime_var", "RUNTIME_VAR_ONLY": "runtime_var"}

	builder := &DefaultEnvBuilder{
		Environ: func() []string {
			return []string{"SHARED=host_env", "HOST_ONLY=host_value"}
		},
	}
	env, err := builder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("DefaultEnvBuilder.Build() error: %v", err)
	}

	// Test: RuntimeEnvVars (level 10) should win for SHARED
	if got := env["SHARED"]; got != "runtime_var" {
		t.Errorf("SHARED = %q, want %q (runtime_var should have highest precedence)", got, "runtime_var")
	}

	// Test: Each level's unique variables are present
	checkVars := map[string]string{
		"HOST_ONLY":         "host_value",
		"ROOT_FILE_ONLY":    "root_file",
		"CMD_FILE_ONLY":     "cmd_file",
		"IMPL_FILE_ONLY":    "impl_file",
		"ROOT_VAR_ONLY":     "root_var",
		"CMD_VAR_ONLY":      "cmd_var",
		"IMPL_VAR_ONLY":     "impl_var",
		"EXTRA_ONLY":        "extra_value",
		"RUNTIME_FILE_ONLY": "runtime_file",
		"RUNTIME_VAR_ONLY":  "runtime_var",
	}

	for k, want := range checkVars {
		if got := env[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

// TestBuildRuntimeEnv_EnvFiles tests env file loading at all levels.
func TestBuildRuntimeEnv_EnvFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create env files
	createEnvFile(t, tmpDir, "root.env", "ROOT_VAR=from_root")
	createEnvFile(t, tmpDir, "cmd.env", "CMD_VAR=from_cmd")
	createEnvFile(t, tmpDir, "impl.env", "IMPL_VAR=from_impl")

	currentPlatform := invowkfile.GetCurrentHostOS()
	cmd := &invowkfile.Command{
		Name: "env-files-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo test",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				Platforms: []invowkfile.PlatformConfig{{Name: currentPlatform}},
				Env: &invowkfile.EnvConfig{
					Files: []string{"impl.env"},
				},
			},
		},
		Env: &invowkfile.EnvConfig{
			Files: []string{"cmd.env"},
		},
	}

	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
		Env: &invowkfile.EnvConfig{
			Files: []string{"root.env"},
		},
	}

	ctx := NewExecutionContext(cmd, inv)
	env, err := NewDefaultEnvBuilder().Build(ctx, invowkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("NewDefaultEnvBuilder().Build() error: %v", err)
	}

	expectedVars := map[string]string{
		"ROOT_VAR": "from_root",
		"CMD_VAR":  "from_cmd",
		"IMPL_VAR": "from_impl",
	}

	for k, want := range expectedVars {
		if got := env[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

// TestBuildRuntimeEnv_EnvVars tests env.vars merging at all levels.
func TestBuildRuntimeEnv_EnvVars(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	currentPlatform := invowkfile.GetCurrentHostOS()
	cmd := &invowkfile.Command{
		Name: "env-vars-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo test",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				Platforms: []invowkfile.PlatformConfig{{Name: currentPlatform}},
				Env: &invowkfile.EnvConfig{
					Vars: map[string]string{
						"IMPL_VAR":  "impl_value",
						"OVERWRITE": "impl_wins",
					},
				},
			},
		},
		Env: &invowkfile.EnvConfig{
			Vars: map[string]string{
				"CMD_VAR":   "cmd_value",
				"OVERWRITE": "cmd_value",
			},
		},
	}

	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
		Env: &invowkfile.EnvConfig{
			Vars: map[string]string{
				"ROOT_VAR":  "root_value",
				"OVERWRITE": "root_value",
			},
		},
	}

	ctx := NewExecutionContext(cmd, inv)
	env, err := NewDefaultEnvBuilder().Build(ctx, invowkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("NewDefaultEnvBuilder().Build() error: %v", err)
	}

	// Each level's variable should be present
	if got := env["ROOT_VAR"]; got != "root_value" {
		t.Errorf("ROOT_VAR = %q, want %q", got, "root_value")
	}
	if got := env["CMD_VAR"]; got != "cmd_value" {
		t.Errorf("CMD_VAR = %q, want %q", got, "cmd_value")
	}
	if got := env["IMPL_VAR"]; got != "impl_value" {
		t.Errorf("IMPL_VAR = %q, want %q", got, "impl_value")
	}

	// Implementation-level vars should override command and root
	if got := env["OVERWRITE"]; got != "impl_wins" {
		t.Errorf("OVERWRITE = %q, want %q (impl should override)", got, "impl_wins")
	}
}

// TestBuildRuntimeEnv_RuntimeFlags tests --ivk-env-file and --ivk-env-var flag handling.
func TestBuildRuntimeEnv_RuntimeFlags(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create runtime env file (loaded via --ivk-env-file flag)
	createEnvFile(t, tmpDir, "flag.env", "FLAG_FILE_VAR=from_flag_file\nSHARED=flag_file")

	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("flags-test", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	// Use absolute path instead of MustChdir for CWD-relative resolution
	ctx.Env.RuntimeEnvFiles = []string{filepath.Join(tmpDir, "flag.env")}
	ctx.Env.RuntimeEnvVars = map[string]string{"FLAG_VAR": "from_flag_var", "SHARED": "flag_var_wins"}

	env, err := NewDefaultEnvBuilder().Build(ctx, invowkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("NewDefaultEnvBuilder().Build() error: %v", err)
	}

	if got := env["FLAG_FILE_VAR"]; got != "from_flag_file" {
		t.Errorf("FLAG_FILE_VAR = %q, want %q", got, "from_flag_file")
	}
	if got := env["FLAG_VAR"]; got != "from_flag_var" {
		t.Errorf("FLAG_VAR = %q, want %q", got, "from_flag_var")
	}

	// --ivk-env-var takes precedence over --ivk-env-file
	if got := env["SHARED"]; got != "flag_var_wins" {
		t.Errorf("SHARED = %q, want %q (--ivk-env-var should override --ivk-env-file)", got, "flag_var_wins")
	}
}

// TestBuildRuntimeEnv_InheritModes tests EnvInheritAll/Allow/None modes.
func TestBuildRuntimeEnv_InheritModes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	fakeEnviron := func() []string {
		return []string{
			"HOST_INHERITED=host_value",
			"ALLOWED_VAR=allowed_value",
			"DENIED_VAR=denied_value",
		}
	}

	tests := []struct {
		name     string
		mode     invowkfile.EnvInheritMode
		allow    []string
		deny     []string
		wantVars map[string]string
		dontWant []string
	}{
		{
			name: "EnvInheritAll includes all host vars",
			mode: invowkfile.EnvInheritAll,
			wantVars: map[string]string{
				"HOST_INHERITED": "host_value",
				"ALLOWED_VAR":    "allowed_value",
				"DENIED_VAR":     "denied_value",
			},
		},
		{
			name:     "EnvInheritNone excludes all host vars",
			mode:     invowkfile.EnvInheritNone,
			dontWant: []string{"HOST_INHERITED", "ALLOWED_VAR", "DENIED_VAR"},
		},
		{
			name:  "EnvInheritAllow includes only allowlisted",
			mode:  invowkfile.EnvInheritAllow,
			allow: []string{"ALLOWED_VAR"},
			wantVars: map[string]string{
				"ALLOWED_VAR": "allowed_value",
			},
			dontWant: []string{"HOST_INHERITED", "DENIED_VAR"},
		},
		{
			name: "EnvInheritAll with denylist excludes denied",
			mode: invowkfile.EnvInheritAll,
			deny: []string{"DENIED_VAR"},
			wantVars: map[string]string{
				"HOST_INHERITED": "host_value",
				"ALLOWED_VAR":    "allowed_value",
			},
			dontWant: []string{"DENIED_VAR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := &invowkfile.Invowkfile{
				FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
			}
			cmd := testCommandWithScript("inherit-test", "echo test", invowkfile.RuntimeNative)
			ctx := NewExecutionContext(cmd, inv)
			ctx.Env.InheritModeOverride = tt.mode
			ctx.Env.InheritAllowOverride = tt.allow
			ctx.Env.InheritDenyOverride = tt.deny

			builder := &DefaultEnvBuilder{Environ: fakeEnviron}
			env, err := builder.Build(ctx, invowkfile.EnvInheritAll)
			if err != nil {
				t.Fatalf("DefaultEnvBuilder.Build() error: %v", err)
			}

			for k, v := range tt.wantVars {
				if got := env[k]; got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}

			for _, k := range tt.dontWant {
				if _, ok := env[k]; ok {
					t.Errorf("should not include %s", k)
				}
			}
		})
	}
}

// TestBuildRuntimeEnv_ExtraEnv tests ExtraEnv injection (flags, args, etc.).
func TestBuildRuntimeEnv_ExtraEnv(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("extra-env-test", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Env.ExtraEnv = map[string]string{
		"INVOWK_FLAG_VERBOSE": "true",
		"INVOWK_ARG_FILE":     "test.txt",
		"ARG1":                "first",
		"ARGC":                "1",
		"CUSTOM_EXTRA":        "custom_value",
	}

	env, err := NewDefaultEnvBuilder().Build(ctx, invowkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("NewDefaultEnvBuilder().Build() error: %v", err)
	}

	// ExtraEnv variables should be present
	expectedVars := map[string]string{
		"INVOWK_FLAG_VERBOSE": "true",
		"INVOWK_ARG_FILE":     "test.txt",
		"ARG1":                "first",
		"ARGC":                "1",
		"CUSTOM_EXTRA":        "custom_value",
	}

	for k, want := range expectedVars {
		if got := env[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

// TestBuildRuntimeEnv_EnvFileNotFound tests error handling for missing env files.
func TestBuildRuntimeEnv_EnvFileNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
		Env: &invowkfile.EnvConfig{
			Files: []string{"nonexistent.env"},
		},
	}
	cmd := testCommandWithScript("missing-env-test", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	_, err := NewDefaultEnvBuilder().Build(ctx, invowkfile.EnvInheritNone)
	if err == nil {
		t.Error("NewDefaultEnvBuilder().Build() should error for missing required env file")
	}
	if !strings.Contains(err.Error(), "nonexistent.env") {
		t.Errorf("error should mention the missing file, got: %v", err)
	}
}

// TestBuildRuntimeEnv_OptionalEnvFile tests optional env file handling.
func TestBuildRuntimeEnv_OptionalEnvFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
		Env: &invowkfile.EnvConfig{
			Files: []string{"nonexistent.env?"},
		},
	}
	cmd := testCommandWithScript("optional-env-test", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	// Optional file should not cause an error
	_, err := NewDefaultEnvBuilder().Build(ctx, invowkfile.EnvInheritNone)
	if err != nil {
		t.Errorf("NewDefaultEnvBuilder().Build() should not error for optional missing env file: %v", err)
	}
}
