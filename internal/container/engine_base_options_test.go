// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"invowk-cli/internal/issue"
)

// T031: BaseCLIEngine WithExecCommand option tests
func TestBaseCLIEngine_WithExecCommand(t *testing.T) {
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
	formatter := func(v string) string {
		return v + ":z" // Simulate SELinux label addition
	}

	engine := NewBaseCLIEngine("/usr/bin/podman", WithVolumeFormatter(formatter))

	args := engine.RunArgs(RunOptions{
		Image:   "alpine",
		Volumes: []string{"/host:/container"},
	})

	// Check that volume has the formatted value
	if !slices.Contains(args, "/host:/container:z") {
		t.Errorf("volume formatter not applied\nargs: %v", args)
	}
}

func TestBaseCLIEngine_BinaryPath(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")
	if got := engine.BinaryPath(); got != "/usr/bin/docker" {
		t.Errorf("BinaryPath() = %q, want %q", got, "/usr/bin/docker")
	}
}

// TestBaseCLIEngine_DefaultOptions verifies default values
func TestBaseCLIEngine_DefaultOptions(t *testing.T) {
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
			var ae *issue.ActionableError
			if errors.As(err, &ae) {
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
	var ae *issue.ActionableError
	if errors.As(err, &ae) {
		formatted := ae.Format(false)
		if !strings.Contains(formatted, "docker images") {
			t.Errorf("formatted error should contain suggestion, got: %s", formatted)
		}
	} else {
		t.Error("expected ActionableError")
	}
}
