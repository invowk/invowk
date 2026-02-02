// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
)

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
