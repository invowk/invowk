// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
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

func TestBuildRuntimeEnv_InheritAllFiltersInvowkVars(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}
	cmd := testCommandWithScript("env", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	restoreParent := testutil.MustSetenv(t, "INVOWK_ARG_PARENT", "parent_value")
	restoreHost := testutil.MustSetenv(t, "CUSTOM_HOST_ENV", "host_value")
	defer restoreParent()
	defer restoreHost()

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
	}

	if _, ok := env["INVOWK_ARG_PARENT"]; ok {
		t.Errorf("buildRuntimeEnv() should filter INVOWK_ARG_PARENT")
	}
	if got := env["CUSTOM_HOST_ENV"]; got != "host_value" {
		t.Errorf("buildRuntimeEnv() CUSTOM_HOST_ENV = %q, want %q", got, "host_value")
	}
}

func TestBuildRuntimeEnv_InheritAllowAndDeny(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}
	cmd := testCommandWithScript("env", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	ctx.EnvInheritModeOverride = invkfile.EnvInheritAllow
	ctx.EnvInheritAllowOverride = []string{"ALLOW_ME", "DENY_ME"}
	ctx.EnvInheritDenyOverride = []string{"DENY_ME"}

	restoreAllow := testutil.MustSetenv(t, "ALLOW_ME", "allowed")
	restoreDeny := testutil.MustSetenv(t, "DENY_ME", "denied")
	defer restoreAllow()
	defer restoreDeny()

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
	}

	if got := env["ALLOW_ME"]; got != "allowed" {
		t.Errorf("buildRuntimeEnv() ALLOW_ME = %q, want %q", got, "allowed")
	}
	if _, ok := env["DENY_ME"]; ok {
		t.Errorf("buildRuntimeEnv() should deny DENY_ME")
	}
}

func TestValidateWorkDir(t *testing.T) {
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
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	tests := []struct {
		name        string
		setupCtx    func(*ExecutionContext)
		defaultMode invkfile.EnvInheritMode
		wantMode    invkfile.EnvInheritMode
		wantAllow   []string
		wantDeny    []string
	}{
		{
			name:        "uses default mode when no overrides",
			setupCtx:    func(_ *ExecutionContext) {},
			defaultMode: invkfile.EnvInheritAll,
			wantMode:    invkfile.EnvInheritAll,
		},
		{
			name:        "default mode none",
			setupCtx:    func(_ *ExecutionContext) {},
			defaultMode: invkfile.EnvInheritNone,
			wantMode:    invkfile.EnvInheritNone,
		},
		{
			name: "context override takes precedence",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.EnvInheritModeOverride = invkfile.EnvInheritNone
			},
			defaultMode: invkfile.EnvInheritAll,
			wantMode:    invkfile.EnvInheritNone,
		},
		{
			name: "context override allow mode with allowlist",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.EnvInheritModeOverride = invkfile.EnvInheritAllow
				ctx.EnvInheritAllowOverride = []string{"PATH", "HOME"}
			},
			defaultMode: invkfile.EnvInheritAll,
			wantMode:    invkfile.EnvInheritAllow,
			wantAllow:   []string{"PATH", "HOME"},
		},
		{
			name: "context denylist override",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.EnvInheritDenyOverride = []string{"SECRET", "API_KEY"}
			},
			defaultMode: invkfile.EnvInheritAll,
			wantMode:    invkfile.EnvInheritAll,
			wantDeny:    []string{"SECRET", "API_KEY"},
		},
		{
			name: "runtime config mode used when no context override",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SelectedImpl.Runtimes = []invkfile.RuntimeConfig{{
					Name:           invkfile.RuntimeNative,
					EnvInheritMode: invkfile.EnvInheritNone,
				}}
			},
			defaultMode: invkfile.EnvInheritAll,
			wantMode:    invkfile.EnvInheritNone,
		},
		{
			name: "context override beats runtime config",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SelectedImpl.Runtimes = []invkfile.RuntimeConfig{{
					Name:           invkfile.RuntimeNative,
					EnvInheritMode: invkfile.EnvInheritNone,
				}}
				ctx.EnvInheritModeOverride = invkfile.EnvInheritAll
			},
			defaultMode: invkfile.EnvInheritNone,
			wantMode:    invkfile.EnvInheritAll,
		},
		{
			name: "runtime config allowlist used when no context override",
			setupCtx: func(ctx *ExecutionContext) {
				ctx.SelectedImpl.Runtimes = []invkfile.RuntimeConfig{{
					Name:            invkfile.RuntimeNative,
					EnvInheritMode:  invkfile.EnvInheritAllow,
					EnvInheritAllow: []string{"SHELL"},
				}}
			},
			defaultMode: invkfile.EnvInheritAll,
			wantMode:    invkfile.EnvInheritAllow,
			wantAllow:   []string{"SHELL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := testCommandWithScript("test", "echo test", invkfile.RuntimeNative)
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
	// Set up known host environment variables for testing
	restorePath := testutil.MustSetenv(t, "TEST_HOST_PATH", "/usr/bin")
	restoreHome := testutil.MustSetenv(t, "TEST_HOST_HOME", "/home/user")
	restoreSecret := testutil.MustSetenv(t, "TEST_SECRET", "confidential")
	defer restorePath()
	defer restoreHome()
	defer restoreSecret()

	tests := []struct {
		name     string
		cfg      envInheritConfig
		wantVars map[string]string
		dontWant []string
	}{
		{
			name: "inherit none returns empty",
			cfg: envInheritConfig{
				mode: invkfile.EnvInheritNone,
			},
			wantVars: map[string]string{},
			dontWant: []string{"TEST_HOST_PATH", "TEST_HOST_HOME"},
		},
		{
			name: "inherit all includes host vars",
			cfg: envInheritConfig{
				mode: invkfile.EnvInheritAll,
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
				"TEST_HOST_HOME": "/home/user",
			},
		},
		{
			name: "inherit allow only includes whitelisted",
			cfg: envInheritConfig{
				mode:  invkfile.EnvInheritAllow,
				allow: []string{"TEST_HOST_PATH"},
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
			},
			dontWant: []string{"TEST_HOST_HOME"},
		},
		{
			name: "inherit all with denylist excludes denied",
			cfg: envInheritConfig{
				mode: invkfile.EnvInheritAll,
				deny: []string{"TEST_SECRET"},
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
				mode:  invkfile.EnvInheritAllow,
				allow: []string{"TEST_HOST_PATH", "TEST_SECRET"},
				deny:  []string{"TEST_SECRET"},
			},
			wantVars: map[string]string{
				"TEST_HOST_PATH": "/usr/bin",
			},
			dontWant: []string{"TEST_SECRET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	tmpDir := t.TempDir()

	// Create env files for different levels
	createEnvFile(t, tmpDir, "root.env", "SHARED=root_file\nROOT_FILE_ONLY=root_file")
	createEnvFile(t, tmpDir, "cmd.env", "SHARED=cmd_file\nCMD_FILE_ONLY=cmd_file")
	createEnvFile(t, tmpDir, "impl.env", "SHARED=impl_file\nIMPL_FILE_ONLY=impl_file")
	createEnvFile(t, tmpDir, "runtime.env", "SHARED=runtime_file\nRUNTIME_FILE_ONLY=runtime_file")

	// Set up host environment (level 1)
	restoreShared := testutil.MustSetenv(t, "SHARED", "host_env")
	restoreHost := testutil.MustSetenv(t, "HOST_ONLY", "host_value")
	defer restoreShared()
	defer restoreHost()

	currentPlatform := invkfile.GetCurrentHostOS()
	cmd := &invkfile.Command{
		Name: "precedence-test",
		Implementations: []invkfile.Implementation{
			{
				Script:    "echo test",
				Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
				Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
				Env: &invkfile.EnvConfig{
					Files: []string{"impl.env"},
					Vars:  map[string]string{"SHARED": "impl_var", "IMPL_VAR_ONLY": "impl_var"},
				},
			},
		},
		Env: &invkfile.EnvConfig{
			Files: []string{"cmd.env"},
			Vars:  map[string]string{"SHARED": "cmd_var", "CMD_VAR_ONLY": "cmd_var"},
		},
	}

	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
		Env: &invkfile.EnvConfig{
			Files: []string{"root.env"},
			Vars:  map[string]string{"SHARED": "root_var", "ROOT_VAR_ONLY": "root_var"},
		},
	}

	ctx := NewExecutionContext(cmd, inv)
	ctx.ExtraEnv = map[string]string{"SHARED": "extra_env", "EXTRA_ONLY": "extra_value"}
	ctx.RuntimeEnvFiles = []string{filepath.Join(tmpDir, "runtime.env")}
	ctx.RuntimeEnvVars = map[string]string{"SHARED": "runtime_var", "RUNTIME_VAR_ONLY": "runtime_var"}

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
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
	tmpDir := t.TempDir()

	// Create env files
	createEnvFile(t, tmpDir, "root.env", "ROOT_VAR=from_root")
	createEnvFile(t, tmpDir, "cmd.env", "CMD_VAR=from_cmd")
	createEnvFile(t, tmpDir, "impl.env", "IMPL_VAR=from_impl")

	currentPlatform := invkfile.GetCurrentHostOS()
	cmd := &invkfile.Command{
		Name: "env-files-test",
		Implementations: []invkfile.Implementation{
			{
				Script:    "echo test",
				Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
				Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
				Env: &invkfile.EnvConfig{
					Files: []string{"impl.env"},
				},
			},
		},
		Env: &invkfile.EnvConfig{
			Files: []string{"cmd.env"},
		},
	}

	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
		Env: &invkfile.EnvConfig{
			Files: []string{"root.env"},
		},
	}

	ctx := NewExecutionContext(cmd, inv)
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
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
	tmpDir := t.TempDir()

	currentPlatform := invkfile.GetCurrentHostOS()
	cmd := &invkfile.Command{
		Name: "env-vars-test",
		Implementations: []invkfile.Implementation{
			{
				Script:    "echo test",
				Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
				Platforms: []invkfile.PlatformConfig{{Name: currentPlatform}},
				Env: &invkfile.EnvConfig{
					Vars: map[string]string{
						"IMPL_VAR":  "impl_value",
						"OVERWRITE": "impl_wins",
					},
				},
			},
		},
		Env: &invkfile.EnvConfig{
			Vars: map[string]string{
				"CMD_VAR":   "cmd_value",
				"OVERWRITE": "cmd_value",
			},
		},
	}

	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
		Env: &invkfile.EnvConfig{
			Vars: map[string]string{
				"ROOT_VAR":  "root_value",
				"OVERWRITE": "root_value",
			},
		},
	}

	ctx := NewExecutionContext(cmd, inv)
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
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

// TestBuildRuntimeEnv_RuntimeFlags tests --env-file and --env-var flag handling.
func TestBuildRuntimeEnv_RuntimeFlags(t *testing.T) {
	tmpDir := t.TempDir()

	// Create runtime env file (loaded via --env-file flag)
	createEnvFile(t, tmpDir, "flag.env", "FLAG_FILE_VAR=from_flag_file\nSHARED=flag_file")

	// Change to tmpDir so relative path works
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}
	cmd := testCommandWithScript("flags-test", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	ctx.RuntimeEnvFiles = []string{"flag.env"}
	ctx.RuntimeEnvVars = map[string]string{"FLAG_VAR": "from_flag_var", "SHARED": "flag_var_wins"}

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
	}

	if got := env["FLAG_FILE_VAR"]; got != "from_flag_file" {
		t.Errorf("FLAG_FILE_VAR = %q, want %q", got, "from_flag_file")
	}
	if got := env["FLAG_VAR"]; got != "from_flag_var" {
		t.Errorf("FLAG_VAR = %q, want %q", got, "from_flag_var")
	}

	// --env-var takes precedence over --env-file
	if got := env["SHARED"]; got != "flag_var_wins" {
		t.Errorf("SHARED = %q, want %q (--env-var should override --env-file)", got, "flag_var_wins")
	}
}

// TestBuildRuntimeEnv_InheritModes tests EnvInheritAll/Allow/None modes.
func TestBuildRuntimeEnv_InheritModes(t *testing.T) {
	tmpDir := t.TempDir()

	restoreHost := testutil.MustSetenv(t, "HOST_INHERITED", "host_value")
	restoreAllowed := testutil.MustSetenv(t, "ALLOWED_VAR", "allowed_value")
	restoreDenied := testutil.MustSetenv(t, "DENIED_VAR", "denied_value")
	defer restoreHost()
	defer restoreAllowed()
	defer restoreDenied()

	tests := []struct {
		name     string
		mode     invkfile.EnvInheritMode
		allow    []string
		deny     []string
		wantVars map[string]string
		dontWant []string
	}{
		{
			name: "EnvInheritAll includes all host vars",
			mode: invkfile.EnvInheritAll,
			wantVars: map[string]string{
				"HOST_INHERITED": "host_value",
				"ALLOWED_VAR":    "allowed_value",
				"DENIED_VAR":     "denied_value",
			},
		},
		{
			name:     "EnvInheritNone excludes all host vars",
			mode:     invkfile.EnvInheritNone,
			dontWant: []string{"HOST_INHERITED", "ALLOWED_VAR", "DENIED_VAR"},
		},
		{
			name:  "EnvInheritAllow includes only allowlisted",
			mode:  invkfile.EnvInheritAllow,
			allow: []string{"ALLOWED_VAR"},
			wantVars: map[string]string{
				"ALLOWED_VAR": "allowed_value",
			},
			dontWant: []string{"HOST_INHERITED", "DENIED_VAR"},
		},
		{
			name: "EnvInheritAll with denylist excludes denied",
			mode: invkfile.EnvInheritAll,
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
			inv := &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
			}
			cmd := testCommandWithScript("inherit-test", "echo test", invkfile.RuntimeNative)
			ctx := NewExecutionContext(cmd, inv)
			ctx.EnvInheritModeOverride = tt.mode
			ctx.EnvInheritAllowOverride = tt.allow
			ctx.EnvInheritDenyOverride = tt.deny

			env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
			if err != nil {
				t.Fatalf("buildRuntimeEnv() error: %v", err)
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
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}
	cmd := testCommandWithScript("extra-env-test", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	ctx.ExtraEnv = map[string]string{
		"INVOWK_FLAG_VERBOSE": "true",
		"INVOWK_ARG_FILE":     "test.txt",
		"ARG1":                "first",
		"ARGC":                "1",
		"CUSTOM_EXTRA":        "custom_value",
	}

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		t.Fatalf("buildRuntimeEnv() error: %v", err)
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
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
		Env: &invkfile.EnvConfig{
			Files: []string{"nonexistent.env"},
		},
	}
	cmd := testCommandWithScript("missing-env-test", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	_, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err == nil {
		t.Error("buildRuntimeEnv() should error for missing required env file")
	}
	if !strings.Contains(err.Error(), "nonexistent.env") {
		t.Errorf("error should mention the missing file, got: %v", err)
	}
}

// TestBuildRuntimeEnv_OptionalEnvFile tests optional env file handling.
func TestBuildRuntimeEnv_OptionalEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
		Env: &invkfile.EnvConfig{
			Files: []string{"nonexistent.env?"},
		},
	}
	cmd := testCommandWithScript("optional-env-test", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	// Optional file should not cause an error
	_, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		t.Errorf("buildRuntimeEnv() should not error for optional missing env file: %v", err)
	}
}
