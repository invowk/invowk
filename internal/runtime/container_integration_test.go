// Package runtime provides integration tests for the container runtime functionality.
// These tests use testcontainers-go to verify container-based command execution.
package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"

	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invowkfile"
)

// checkTestcontainersAvailable safely checks if testcontainers can be used.
// Returns true if containers are available, false otherwise.
func checkTestcontainersAvailable() (available bool) {
	defer func() {
		if r := recover(); r != nil {
			available = false
		}
	}()

	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		return false
	}
	defer provider.Close()
	return true
}

// TestContainerRuntime_Integration tests the container runtime with real containers.
// These tests require Docker or Podman to be available.
func TestContainerRuntime_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if we can run containers using our own engine detection
	// This is more robust than testcontainers-go's detection which can panic
	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("skipping container integration tests: no container engine available: %v", err)
	}
	if !engine.Available() {
		t.Skip("skipping container integration tests: container engine not available")
	}

	// Also check via testcontainers for additional verification
	if !checkTestcontainersAvailable() {
		t.Skip("skipping container integration tests: testcontainers provider not available")
	}

	t.Run("BasicExecution", testContainerBasicExecution)
	t.Run("EnvironmentVariables", testContainerEnvironmentVariables)
	t.Run("MultiLineScript", testContainerMultiLineScript)
	t.Run("WorkingDirectory", testContainerWorkingDirectory)
	t.Run("VolumeMounts", testContainerVolumeMounts)
	t.Run("ExitCode", testContainerExitCode)
	t.Run("EnableHostSSH_EnvVarsProvided", testContainerEnableHostSSHEnvVars)
}

// testContainerBasicExecution tests basic command execution in a container
func testContainerBasicExecution(t *testing.T) {
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	cmd := &invowkfile.Command{
		Name: "test-basic",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo 'Hello from container'",
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
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

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from container" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from container")
	}
}

// testContainerEnvironmentVariables tests environment variable handling in containers
func testContainerEnvironmentVariables(t *testing.T) {
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	currentPlatform := invowkfile.GetCurrentHostOS()
	cmd := &invowkfile.Command{
		Name: "test-env",
		Implementations: []invowkfile.Implementation{
			{
				Script: `echo "VAR1=$MY_VAR1 VAR2=$MY_VAR2"`,
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
					},
					Platforms: []invowkfile.PlatformConfig{
						{Name: currentPlatform, Env: map[string]string{"MY_VAR1": "platform_value"}},
					},
				},
			},
		},
		Env: map[string]string{
			"MY_VAR2": "command_value",
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
	if !strings.Contains(output, "VAR1=platform_value") {
		t.Errorf("Execute() output missing platform env var, got: %q", output)
	}
	if !strings.Contains(output, "VAR2=command_value") {
		t.Errorf("Execute() output missing command env var, got: %q", output)
	}
}

// testContainerMultiLineScript tests multi-line script execution in containers
func testContainerMultiLineScript(t *testing.T) {
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	script := `echo "Line 1"
echo "Line 2"
VAR="hello"
echo "Variable: $VAR"`

	cmd := &invowkfile.Command{
		Name: "test-multiline",
		Implementations: []invowkfile.Implementation{
			{
				Script: script,
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
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
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory in the temp directory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	cmd := &invowkfile.Command{
		Name:    "test-workdir",
		WorkDir: "subdir",
		Implementations: []invowkfile.Implementation{
			{
				Script: "pwd",
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
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

	output := strings.TrimSpace(stdout.String())
	// The working directory inside the container should be /workspace/subdir
	if !strings.HasSuffix(output, "/workspace/subdir") {
		t.Errorf("Execute() output = %q, want to end with '/workspace/subdir'", output)
	}
}

// testContainerVolumeMounts tests volume mounting in containers
func testContainerVolumeMounts(t *testing.T) {
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	// Create a file to mount
	testFile := filepath.Join(tmpDir, "test-data.txt")
	testContent := "test content from host"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a directory for additional volume mount
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}
	dataFile := filepath.Join(dataDir, "data.txt")
	if err := os.WriteFile(dataFile, []byte("data from data dir"), 0644); err != nil {
		t.Fatalf("Failed to write data file: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "test-volumes",
		Implementations: []invowkfile.Implementation{
			{
				Script: `cat /workspace/test-data.txt && echo "" && cat /data/data.txt`,
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{
							Name:    invowkfile.RuntimeContainer,
							Image:   "alpine:latest",
							Volumes: []string{dataDir + ":/data:ro"},
						},
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
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	cmd := &invowkfile.Command{
		Name: "test-exit-code",
		Implementations: []invowkfile.Implementation{
			{
				Script: "exit 42",
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
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
	if result.ExitCode != 42 {
		t.Errorf("Execute() exit code = %d, want 42", result.ExitCode)
	}
}

// testContainerEnableHostSSHEnvVars tests that SSH environment variables are provided when enable_host_ssh is true
func testContainerEnableHostSSHEnvVars(t *testing.T) {
	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	cmd := &invowkfile.Command{
		Name: "test-ssh-env",
		Implementations: []invowkfile.Implementation{
			{
				Script: `echo "SSH_HOST=$INVOWK_SSH_HOST SSH_PORT=$INVOWK_SSH_PORT SSH_USER=$INVOWK_SSH_USER SSH_ENABLED=$INVOWK_SSH_ENABLED"`,
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest", EnableHostSSH: true},
					},
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

	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

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
						Target: invowkfile.Target{
							Runtimes: []invowkfile.RuntimeConfig{
								{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
							},
						},
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
						Target: invowkfile.Target{
							Runtimes: []invowkfile.RuntimeConfig{
								{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"},
							},
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

	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	cmd := &invowkfile.Command{
		Name: "test-ssh-no-server",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo test",
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Image: "alpine:latest", EnableHostSSH: true},
					},
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

	tmpDir, inv := setupTestInvowkfile(t)
	defer os.RemoveAll(tmpDir)

	// Create a simple Containerfile
	containerfileContent := `FROM alpine:latest
RUN echo "Built from Containerfile" > /built.txt
`
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte(containerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Containerfile: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "test-build",
		Implementations: []invowkfile.Implementation{
			{
				Script: "cat /built.txt",
				Target: invowkfile.Target{
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeContainer, Containerfile: "Containerfile"},
					},
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

// setupTestInvowkfile creates a temporary directory and invowkfile for testing
func setupTestInvowkfile(t *testing.T) (string, *invowkfile.Invowkfile) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "invowk-container-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
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
	cfg := &sshserver.Config{
		Host:     "127.0.0.1",
		Port:     0, // Random available port
		TokenTTL: 5 * time.Minute,
	}

	srv, err := sshserver.New(cfg)
	if err != nil {
		return nil, err
	}

	// Start the server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("SSH server stopped: %v", err)
		}
	}()

	// Wait a bit for the server to start
	time.Sleep(100 * time.Millisecond)

	// Register cleanup
	t.Cleanup(func() {
		srv.Stop()
	})

	return srv, nil
}
