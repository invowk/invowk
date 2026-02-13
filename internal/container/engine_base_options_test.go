// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"invowk-cli/internal/issue"
)

// T031: BaseCLIEngine WithExecCommand option tests
func TestBaseCLIEngine_WithExecCommand(t *testing.T) {
	t.Parallel()

	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		// Return a no-op command (using CommandContext to satisfy noctx linter)
		return exec.CommandContext(ctx, "true")
	}

	engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(mockExec))

	// Run a command that will capture args
	_, _ = engine.RunCommand(context.Background(), "version")

	if len(capturedArgs) < 2 {
		t.Fatalf("expected at least 2 args, got %d: %v", len(capturedArgs), capturedArgs)
	}

	if capturedArgs[0] != "/usr/bin/docker" {
		t.Errorf("binary path = %q, want %q", capturedArgs[0], "/usr/bin/docker")
	}

	if capturedArgs[1] != "version" {
		t.Errorf("command = %q, want %q", capturedArgs[1], "version")
	}
}

func TestBaseCLIEngine_WithVolumeFormatter(t *testing.T) {
	t.Parallel()

	formatter := func(v string) string {
		return v + ":z" // Simulate SELinux label addition
	}

	engine := NewBaseCLIEngine("/usr/bin/podman", WithVolumeFormatter(formatter))

	args := engine.RunArgs(RunOptions{
		Image:   "debian:stable-slim",
		Volumes: []string{"/host:/container"},
	})

	// Check that volume has the formatted value
	if !slices.Contains(args, "/host:/container:z") {
		t.Errorf("volume formatter not applied\nargs: %v", args)
	}
}

func TestBaseCLIEngine_BinaryPath(t *testing.T) {
	t.Parallel()

	engine := NewBaseCLIEngine("/usr/bin/docker")
	if got := engine.BinaryPath(); got != "/usr/bin/docker" {
		t.Errorf("BinaryPath() = %q, want %q", got, "/usr/bin/docker")
	}
}

// TestBaseCLIEngine_DefaultOptions verifies default values
func TestBaseCLIEngine_DefaultOptions(t *testing.T) {
	t.Parallel()

	engine := NewBaseCLIEngine("/usr/bin/docker")

	if engine.binaryPath != "/usr/bin/docker" {
		t.Errorf("binaryPath = %q, want %q", engine.binaryPath, "/usr/bin/docker")
	}

	if engine.execCommand == nil {
		t.Error("execCommand should not be nil")
	}

	if engine.volumeFormatter == nil {
		t.Error("volumeFormatter should not be nil")
	}

	// Test default volume formatter is identity
	input := "/host:/container"
	if got := engine.volumeFormatter(input); got != input {
		t.Errorf("default volumeFormatter(%q) = %q, want %q", input, got, input)
	}
}

// T106: Test for container build error format
func TestBuildContainerError(t *testing.T) {
	t.Parallel()

	cause := errors.New("build context not found")

	tests := []struct {
		name           string
		engine         string
		opts           BuildOptions
		errorContains  []string // Must be in .Error()
		formatContains []string // Must be in .Format()
	}{
		{
			name:   "with dockerfile",
			engine: "docker",
			opts: BuildOptions{
				Dockerfile: "Dockerfile.custom",
				ContextDir: "/app",
			},
			errorContains: []string{
				"build container image",
				"Dockerfile.custom",
			},
			formatContains: []string{
				"Check Dockerfile syntax",
				"build context path",
				"docker pull",
			},
		},
		{
			name:   "with context only",
			engine: "podman",
			opts: BuildOptions{
				ContextDir: "/app",
			},
			errorContains: []string{
				"build container image",
				"/app/Dockerfile",
			},
			formatContains: []string{
				"podman pull",
			},
		},
		{
			name:   "with tag only",
			engine: "docker",
			opts: BuildOptions{
				Tag: "myimage:latest",
			},
			errorContains: []string{
				"build container image",
				"myimage:latest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := buildContainerError(tt.engine, tt.opts, cause)

			if err == nil {
				t.Fatal("buildContainerError() should return error")
			}

			errStr := err.Error()
			for _, exp := range tt.errorContains {
				if !strings.Contains(errStr, exp) {
					t.Errorf("error should contain %q, got: %s", exp, errStr)
				}
			}

			// Check formatted output for suggestions (requires type assertion)
			if ae, ok := errors.AsType[*issue.ActionableError](err); ok {
				formatted := ae.Format(false)
				for _, exp := range tt.formatContains {
					if !strings.Contains(formatted, exp) {
						t.Errorf("formatted error should contain %q, got: %s", exp, formatted)
					}
				}
			} else if len(tt.formatContains) > 0 {
				t.Error("expected ActionableError for format checking")
			}
		})
	}
}

func TestRunContainerError(t *testing.T) {
	t.Parallel()

	cause := errors.New("image not found")

	err := runContainerError("docker", RunOptions{
		Image: "myimage:latest",
	}, cause)

	if err == nil {
		t.Fatal("runContainerError() should return error")
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "run container") {
		t.Errorf("error should contain operation, got: %s", errStr)
	}

	if !strings.Contains(errStr, "myimage:latest") {
		t.Errorf("error should contain image, got: %s", errStr)
	}

	// Check formatted output for suggestions
	if ae, ok := errors.AsType[*issue.ActionableError](err); ok {
		formatted := ae.Format(false)
		if !strings.Contains(formatted, "docker images") {
			t.Errorf("formatted error should contain suggestion, got: %s", formatted)
		}
	} else {
		t.Error("expected ActionableError")
	}
}

// TestBaseCLIEngine_RunCommandCombined verifies RunCommandCombined captures both stdout and stderr.
func TestBaseCLIEngine_RunCommandCombined(t *testing.T) {
	t.Parallel()

	t.Run("success with combined output", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stdout = "combined output"
		engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

		out, err := engine.RunCommandCombined(context.Background(), "version")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Use Contains because coverage mode may add extra output
		if !strings.Contains(string(out), "combined output") {
			t.Errorf("expected output to contain 'combined output', got %q", string(out))
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "version")
	})

	t.Run("error with output preserved", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stdout = "error details here"
		recorder.ExitCode = 1
		engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

		out, err := engine.RunCommandCombined(context.Background(), "invalid-command")
		if err == nil {
			t.Fatal("expected error for failed command")
		}

		// Even on error, output should be available (use Contains for coverage mode compatibility)
		if !strings.Contains(string(out), "error details here") {
			t.Errorf("expected output to contain 'error details here', got %q", string(out))
		}

		// Error should contain context
		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
	})
}

// TestBaseCLIEngine_RunCommand_ErrorHandling verifies error wrapping in RunCommand.
func TestBaseCLIEngine_RunCommand_ErrorHandling(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	recorder.Stderr = "command not found"
	recorder.ExitCode = 127
	engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

	_, err := engine.RunCommand(context.Background(), "nonexistent-subcommand")
	if err == nil {
		t.Fatal("expected error for failed command")
	}

	// Error should contain the command and arguments
	errStr := err.Error()
	if !strings.Contains(errStr, "docker") {
		t.Errorf("error should contain binary name, got: %s", errStr)
	}
	if !strings.Contains(errStr, "failed") {
		t.Errorf("error should indicate failure, got: %s", errStr)
	}
}

// --- CmdCustomizer / WithCmdEnvOverride / WithCmdExtraFile tests ---

func TestWithCmdEnvOverride_Single(t *testing.T) {
	t.Parallel()

	engine := NewBaseCLIEngine("/usr/bin/podman",
		WithCmdEnvOverride("CONTAINERS_CONF_OVERRIDE", "/dev/fd/3"),
	)

	cmd := exec.CommandContext(context.Background(), "true")
	engine.CustomizeCmd(cmd)

	if !slices.Contains(cmd.Env, "CONTAINERS_CONF_OVERRIDE=/dev/fd/3") {
		t.Error("expected CONTAINERS_CONF_OVERRIDE=/dev/fd/3 in cmd.Env")
	}
}

func TestWithCmdEnvOverride_Multiple(t *testing.T) {
	t.Parallel()

	engine := NewBaseCLIEngine("/usr/bin/podman",
		WithCmdEnvOverride("FOO", "bar"),
		WithCmdEnvOverride("BAZ", "qux"),
	)

	cmd := exec.CommandContext(context.Background(), "true")
	engine.CustomizeCmd(cmd)

	if !slices.Contains(cmd.Env, "FOO=bar") {
		t.Error("expected FOO=bar in cmd.Env")
	}
	if !slices.Contains(cmd.Env, "BAZ=qux") {
		t.Error("expected BAZ=qux in cmd.Env")
	}
}

func TestWithCmdExtraFile(t *testing.T) {
	t.Parallel()

	// Create a temp file to use as an extra file
	f, err := os.CreateTemp(t.TempDir(), "test-extra-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	engine := NewBaseCLIEngine("/usr/bin/podman",
		WithCmdExtraFile(f),
	)

	cmd := exec.CommandContext(context.Background(), "true")
	engine.CustomizeCmd(cmd)

	if len(cmd.ExtraFiles) != 1 {
		t.Fatalf("expected 1 extra file, got %d", len(cmd.ExtraFiles))
	}
	if cmd.ExtraFiles[0] != f {
		t.Error("extra file does not match the provided file")
	}
}

func TestCustomizeCmd_Empty(t *testing.T) {
	t.Parallel()

	// Engine with no overrides or extra files
	engine := NewBaseCLIEngine("/usr/bin/docker")

	cmd := exec.CommandContext(context.Background(), "true")
	engine.CustomizeCmd(cmd)

	// cmd.Env should remain nil (inherit parent env)
	if cmd.Env != nil {
		t.Errorf("expected nil Env for engine without overrides, got %d entries", len(cmd.Env))
	}
	// cmd.ExtraFiles should remain nil
	if cmd.ExtraFiles != nil {
		t.Errorf("expected nil ExtraFiles for engine without extra files, got %d entries", len(cmd.ExtraFiles))
	}
}

func TestWithSysctlOverrideActive(t *testing.T) {
	t.Parallel()

	t.Run("default is false", func(t *testing.T) {
		t.Parallel()
		engine := NewBaseCLIEngine("/usr/bin/podman")
		if engine.sysctlOverrideActive {
			t.Error("expected sysctlOverrideActive to default to false")
		}
	})

	t.Run("set to true", func(t *testing.T) {
		t.Parallel()
		engine := NewBaseCLIEngine("/usr/bin/podman",
			WithSysctlOverrideActive(true),
		)
		if !engine.sysctlOverrideActive {
			t.Error("expected sysctlOverrideActive to be true")
		}
	})
}

func TestCustomizeCmd_PreservesParentEnv(t *testing.T) {
	t.Parallel()

	engine := NewBaseCLIEngine("/usr/bin/podman",
		WithCmdEnvOverride("INVOWK_TEST_ONLY", "1"),
	)

	cmd := exec.CommandContext(context.Background(), "true")
	engine.CustomizeCmd(cmd)

	// os.Environ() should be the base — cmd.Env must contain more than just our override.
	// PATH is virtually always present in the parent environment.
	foundPath := false
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "PATH=") {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Error("expected parent PATH to be preserved in cmd.Env")
	}
}
