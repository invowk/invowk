// SPDX-License-Identifier: EPL-2.0

// Package runtime provides integration tests for the container runtime functionality.
// These tests verify container-based command execution using Docker or Podman.
package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
)

// TestContainerRuntime_Integration tests the container runtime with real containers.
// These tests require Docker or Podman to be available.
func TestContainerRuntime_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if we can run containers using our own engine detection
	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("skipping container integration tests: no container engine available: %v", err)
	}
	if !engine.Available() {
		t.Skip("skipping container integration tests: container engine not available")
	}

	t.Run("BasicExecution", testContainerBasicExecution)
	t.Run("EnvironmentVariables", testContainerEnvironmentVariables)
	t.Run("MultiLineScript", testContainerMultiLineScript)
	t.Run("WorkingDirectory", testContainerWorkingDirectory)
	t.Run("VolumeMounts", testContainerVolumeMounts)
	t.Run("ExitCode", testContainerExitCode)
	t.Run("PositionalArgs", testContainerPositionalArgs)
	t.Run("EnableHostSSH_EnvVarsProvided", testContainerEnableHostSSHEnvVars)
}

// testContainerBasicExecution tests basic command execution in a container
func testContainerBasicExecution(t *testing.T) {
	_, inv := setupTestInvkfile(t)

	cmd := &invkfile.Command{
		Name: "test-basic",
		Implementations: []invkfile.Implementation{
			{
				Script: "echo 'Hello from container'",
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from container" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from container")
	}
}

// testContainerEnvironmentVariables tests environment variable handling in containers
func testContainerEnvironmentVariables(t *testing.T) {
	_, inv := setupTestInvkfile(t)

	currentPlatform := invkfile.GetCurrentHostOS()
	cmd := &invkfile.Command{
		Name: "test-env",
		Implementations: []invkfile.Implementation{
			{
				Script: `echo "VAR1=$MY_VAR1 VAR2=$MY_VAR2"`,
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
					},
					Platforms: []invkfile.PlatformConfig{
						{Name: currentPlatform},
					},
				Env: &invkfile.EnvConfig{Vars: map[string]string{"MY_VAR1": "impl_value"}},
			},
		},
		Env: &invkfile.EnvConfig{
			Vars: map[string]string{
				"MY_VAR2": "command_value",
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "VAR1=impl_value") {
		t.Errorf("Execute() output missing implementation env var, got: %q", output)
	}
	if !strings.Contains(output, "VAR2=command_value") {
		t.Errorf("Execute() output missing command env var, got: %q", output)
	}
}

// testContainerMultiLineScript tests multi-line script execution in containers
func testContainerMultiLineScript(t *testing.T) {
	_, inv := setupTestInvkfile(t)

	script := `echo "Line 1"
echo "Line 2"
VAR="hello"
echo "Variable: $VAR"`

	cmd := &invkfile.Command{
		Name: "test-multiline",
		Implementations: []invkfile.Implementation{
			{
				Script: script,
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Line 1") {
		t.Errorf("Execute() output missing 'Line 1', got: %q", output)
	}
	if !strings.Contains(output, "Line 2") {
		t.Errorf("Execute() output missing 'Line 2', got: %q", output)
	}
	if !strings.Contains(output, "Variable: hello") {
		t.Errorf("Execute() output missing variable expansion, got: %q", output)
	}
}

// testContainerWorkingDirectory tests working directory handling in containers
func testContainerWorkingDirectory(t *testing.T) {
	tmpDir, inv := setupTestInvkfile(t)

	// Create a subdirectory in the temp directory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	cmd := &invkfile.Command{
		Name:    "test-workdir",
		WorkDir: "subdir",
		Implementations: []invkfile.Implementation{
			{
				Script: "pwd",
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	// The working directory inside the container should be /workspace/subdir
	if !strings.HasSuffix(output, "/workspace/subdir") {
		t.Errorf("Execute() output = %q, want to end with '/workspace/subdir'", output)
	}
}

// testContainerVolumeMounts tests volume mounting in containers
func testContainerVolumeMounts(t *testing.T) {
	tmpDir, inv := setupTestInvkfile(t)

	// Create a file to mount
	testFile := filepath.Join(tmpDir, "test-data.txt")
	testContent := "test content from host"
	if err := os.WriteFile(testFile, []byte(testContent), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a directory for additional volume mount
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}
	dataFile := filepath.Join(dataDir, "data.txt")
	if err := os.WriteFile(dataFile, []byte("data from data dir"), 0o644); err != nil {
		t.Fatalf("Failed to write data file: %v", err)
	}

	cmd := &invkfile.Command{
		Name: "test-volumes",
		Implementations: []invkfile.Implementation{
			{
				Script: `cat /workspace/test-data.txt && echo "" && cat /data/data.txt`,
				
					Runtimes: []invkfile.RuntimeConfig{
						{
							Name:    invkfile.RuntimeContainer,
							Image:   "debian:stable-slim",
							Volumes: []string{dataDir + ":/data:ro"},
						},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, testContent) {
		t.Errorf("Execute() output missing content from workspace mount, got: %q", output)
	}
	if !strings.Contains(output, "data from data dir") {
		t.Errorf("Execute() output missing content from data mount, got: %q", output)
	}
}

// testContainerExitCode tests that non-zero exit codes are properly propagated
func testContainerExitCode(t *testing.T) {
	_, inv := setupTestInvkfile(t)

	cmd := &invkfile.Command{
		Name: "test-exit-code",
		Implementations: []invkfile.Implementation{
			{
				Script: "exit 42",
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 42 {
		t.Errorf("Execute() exit code = %d, want 42", result.ExitCode)
	}
}

// testContainerPositionalArgs tests that positional arguments are accessible via $1, $2, $@ in containers
func testContainerPositionalArgs(t *testing.T) {
	_, inv := setupTestInvkfile(t)

	cmd := &invkfile.Command{
		Name: "test-positional-args",
		Implementations: []invkfile.Implementation{
			{
				Script: `echo "ARG1=$1 ARG2=$2 ALL=$@ COUNT=$#"`,
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()
	execCtx.PositionalArgs = []string{"hello", "world"}

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ARG1=hello") {
		t.Errorf("Execute() output missing ARG1=hello, got: %q", output)
	}
	if !strings.Contains(output, "ARG2=world") {
		t.Errorf("Execute() output missing ARG2=world, got: %q", output)
	}
	if !strings.Contains(output, "ALL=hello world") {
		t.Errorf("Execute() output missing ALL=hello world, got: %q", output)
	}
	if !strings.Contains(output, "COUNT=2") {
		t.Errorf("Execute() output missing COUNT=2, got: %q", output)
	}
}

// testContainerEnableHostSSHEnvVars tests that SSH environment variables are provided when enable_host_ssh is true
func testContainerEnableHostSSHEnvVars(t *testing.T) {
	_, inv := setupTestInvkfile(t)

	cmd := &invkfile.Command{
		Name: "test-ssh-env",
		Implementations: []invkfile.Implementation{
			{
				Script: `echo "SSH_HOST=$INVOWK_SSH_HOST SSH_PORT=$INVOWK_SSH_PORT SSH_USER=$INVOWK_SSH_USER SSH_ENABLED=$INVOWK_SSH_ENABLED"`,
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim", EnableHostSSH: true},
					},
			},
		},
	}

	rt := createContainerRuntimeWithSSHServer(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

	result := rt.Execute(execCtx)

	// The execution should work and SSH env vars should be present
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s", result.ExitCode, result.Error, stderr.String())
	}

	output := stdout.String()
	// Check that SSH environment variables are set
	if !strings.Contains(output, "SSH_ENABLED=true") {
		t.Errorf("Execute() output missing SSH_ENABLED=true, got: %q", output)
	}
	if !strings.Contains(output, "SSH_HOST=") {
		t.Errorf("Execute() output missing SSH_HOST, got: %q", output)
	}
	if !strings.Contains(output, "SSH_PORT=") {
		t.Errorf("Execute() output missing SSH_PORT, got: %q", output)
	}
	if !strings.Contains(output, "SSH_USER=") {
		t.Errorf("Execute() output missing SSH_USER, got: %q", output)
	}
}

// TestContainerRuntime_Validate tests the Validate method
func TestContainerRuntime_Validate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, inv := setupTestInvkfile(t)

	tests := []struct {
		name        string
		cmd         *invkfile.Command
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid container config with image",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{
						Script: "echo test",
						
							Runtimes: []invkfile.RuntimeConfig{
								{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
							},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing implementation",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
			},
			expectError: true,
			errorMatch:  "no implementation selected",
		},
		{
			name: "empty script",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{
						Script: "",
						
							Runtimes: []invkfile.RuntimeConfig{
								{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"},
							},
					},
				},
			},
			expectError: true,
			errorMatch:  "no script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := createContainerRuntime(t)
			execCtx := NewExecutionContext(tt.cmd, inv)

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, inv := setupTestInvkfile(t)

	cmd := &invkfile.Command{
		Name: "test-ssh-no-server",
		Implementations: []invkfile.Implementation{
			{
				Script: "echo test",
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim", EnableHostSSH: true},
					},
			},
		},
	}

	// Create runtime WITHOUT SSH server
	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, inv := setupTestInvkfile(t)

	// Create a simple Containerfile
	containerfileContent := `FROM debian:stable-slim
RUN echo "Built from Containerfile" > /built.txt
`
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte(containerfileContent), 0o644); err != nil {
		t.Fatalf("Failed to write Containerfile: %v", err)
	}

	cmd := &invkfile.Command{
		Name: "test-build",
		Implementations: []invkfile.Implementation{
			{
				Script: "cat /built.txt",
				
					Runtimes: []invkfile.RuntimeConfig{
						{Name: invkfile.RuntimeContainer, Containerfile: "Containerfile"},
					},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()
	execCtx.Verbose = true

	var stdout, stderr bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &stderr

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

// Helper functions

// setupTestInvkfile creates a temporary directory and invkfile for testing.
// It uses a non-hidden directory under $HOME/invowk-test/ because Docker installed via snap
// cannot access hidden directories (those starting with '.') due to snap's home interface limitations.
func setupTestInvkfile(t *testing.T) (string, *invkfile.Invkfile) {
	t.Helper()

	// Create base directory for tests in user's home (not hidden - no leading dot)
	// Docker snap's home interface only allows access to non-hidden files/directories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	baseTmpDir := filepath.Join(homeDir, "invowk-test")
	if mkdirErr := os.MkdirAll(baseTmpDir, 0o755); mkdirErr != nil {
		t.Fatalf("Failed to create base temp dir: %v", mkdirErr)
	}

	tmpDir, err := os.MkdirTemp(baseTmpDir, "container-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Register cleanup to remove the temp dir after test
	t.Cleanup(func() {
		testutil.MustRemoveAll(t, tmpDir)
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	return tmpDir, inv
}

// createContainerRuntime creates a container runtime for testing
func createContainerRuntime(t *testing.T) *ContainerRuntime {
	t.Helper()

	// Try to auto-detect an available container engine
	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("skipping test: no container engine available: %v", err)
	}

	return NewContainerRuntimeWithEngine(engine)
}

// createContainerRuntimeWithSSHServer creates a container runtime with an SSH server for testing
func createContainerRuntimeWithSSHServer(t *testing.T) *ContainerRuntime {
	t.Helper()

	rt := createContainerRuntime(t)

	// Create and start an SSH server for testing
	srv, err := createTestSSHServer(t)
	if err != nil {
		t.Skipf("skipping test: failed to create SSH server: %v", err)
	}

	rt.SetSSHServer(srv)
	return rt
}

// createTestSSHServer creates a test SSH server
func createTestSSHServer(t *testing.T) (*sshserver.Server, error) {
	t.Helper()

	// Create a minimal SSH server configuration
	cfg := sshserver.Config{
		Host:     "127.0.0.1",
		Port:     0, // Random available port
		TokenTTL: 5 * time.Minute,
	}

	srv := sshserver.New(cfg)

	// Start the server with context. Server.Start() blocks until the server
	// is ready to accept connections or fails, eliminating the previous race
	// condition where we'd access srv.Address() before initialization completed.
	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start SSH server: %w", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		testutil.MustStop(t, srv)
	})

	return srv, nil
}
