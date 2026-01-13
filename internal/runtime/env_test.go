// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/pkg/invkfile"
)

func TestBuildRuntimeEnv_InheritAllFiltersInvowkVars(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}
	cmd := testCommandWithScript("env", "echo test", invkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)

	os.Setenv("INVOWK_ARG_PARENT", "parent_value")
	os.Setenv("CUSTOM_HOST_ENV", "host_value")
	defer func() {
		os.Unsetenv("INVOWK_ARG_PARENT")
		os.Unsetenv("CUSTOM_HOST_ENV")
	}()

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

	os.Setenv("ALLOW_ME", "allowed")
	os.Setenv("DENY_ME", "denied")
	defer func() {
		os.Unsetenv("ALLOW_ME")
		os.Unsetenv("DENY_ME")
	}()

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
