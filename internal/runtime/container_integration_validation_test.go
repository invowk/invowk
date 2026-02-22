// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestContainerRuntime_Validate tests the Validate method
func TestContainerRuntime_Validate(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, inv := setupTestInvowkfile(t)

	tests := []struct {
		name        string
		cmd         *invowkfile.Command
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid container config with image",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{
						Script: "echo test",

						Runtimes: []invowkfile.RuntimeConfig{
							{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
						},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing implementation",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
			},
			expectError: true,
			errorMatch:  "no implementation selected",
		},
		{
			name: "empty script",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{
						Script: "",

						Runtimes: []invowkfile.RuntimeConfig{
							{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
						},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
					},
				},
			},
			expectError: true,
			errorMatch:  "no script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := createContainerRuntime(t)
			execCtx := NewExecutionContext(context.Background(), tt.cmd, inv)

			err := rt.Validate(execCtx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
				} else if tt.errorMatch != "" && !strings.Contains(err.Error(), tt.errorMatch) {
					t.Errorf("Validate() error = %q, want to contain %q", err.Error(), tt.errorMatch)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestContainerRuntime_EnableHostSSH_NoServer tests that enable_host_ssh fails gracefully when no SSH server is configured
func TestContainerRuntime_EnableHostSSH_NoServer(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, inv := setupTestInvowkfile(t)

	cmd := &invowkfile.Command{
		Name: "test-ssh-no-server",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo test",

				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim", EnableHostSSH: true},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	// Create runtime WITHOUT SSH server
	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout, stderr bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &stderr

	result := rt.Execute(execCtx)

	// Should fail because SSH server is not configured
	if result.ExitCode == 0 {
		t.Error("Execute() expected non-zero exit code when SSH server is not configured")
	}
	if result.Error == nil {
		t.Error("Execute() expected error when SSH server is not configured")
	}
	if result.Error != nil && !strings.Contains(result.Error.Error(), "enable_host_ssh") {
		t.Errorf("Execute() error should mention enable_host_ssh, got: %v", result.Error)
	}
}

// TestContainerRuntime_BuildFromContainerfile tests building an image from a Containerfile
func TestContainerRuntime_BuildFromContainerfile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sem := testutil.ContainerSemaphore()
	sem <- struct{}{}
	defer func() { <-sem }()

	tmpDir, inv := setupTestInvowkfile(t)

	// Create a simple Containerfile
	containerfileContent := `FROM debian:stable-slim
RUN echo "Built from Containerfile" > /built.txt
`
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte(containerfileContent), 0o644); err != nil {
		t.Fatalf("Failed to write Containerfile: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "test-build",
		Implementations: []invowkfile.Implementation{
			{
				Script: "cat /built.txt",

				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Containerfile: "Containerfile"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(context.Background(), cmd, inv)

	execCtx.Verbose = true

	var stdout, stderr bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, "Built from Containerfile") {
		t.Errorf("Execute() output = %q, want to contain 'Built from Containerfile'", output)
	}

	// Cleanup: remove the built image
	if err := rt.CleanupImage(execCtx); err != nil {
		t.Logf("Warning: failed to cleanup image: %v", err)
	}
}
