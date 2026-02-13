// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invowkfile"
)

// TestContainerRuntime_ProvisioningLayer_InvowkfileAccess tests that the invowkfile directory
// is correctly provisioned at /workspace in the container.
func TestContainerRuntime_ProvisioningLayer_InvowkfileAccess(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sem := testutil.ContainerSemaphore()
	sem <- struct{}{}
	defer func() { <-sem }()

	tmpDir, inv := setupTestInvowkfile(t)

	// Create a test file in the invowkfile directory
	testFileName := "test-provision-file.txt"
	testContent := "provisioned content verification"
	testFilePath := filepath.Join(tmpDir, testFileName)
	if err := os.WriteFile(testFilePath, []byte(testContent), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "test-provision",
		Implementations: []invowkfile.Implementation{
			{
				// The script reads a file from /workspace to verify provisioning
				Script: "cat /workspace/" + testFileName,

				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s",
			result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != testContent {
		t.Errorf("Execute() output = %q, want %q", output, testContent)
	}
}

// TestContainerRuntime_ProvisioningLayer_ScriptFileExecution tests that script files
// in the invowkfile directory are accessible and executable in the container.
func TestContainerRuntime_ProvisioningLayer_ScriptFileExecution(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sem := testutil.ContainerSemaphore()
	sem <- struct{}{}
	defer func() { <-sem }()

	tmpDir, inv := setupTestInvowkfile(t)

	// Create an executable script file in the invowkfile directory
	scriptFileName := "test-script.sh"
	scriptContent := `#!/bin/sh
echo "Script executed from /workspace"
`
	scriptFilePath := filepath.Join(tmpDir, scriptFileName)
	if err := os.WriteFile(scriptFilePath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "test-script-provision",
		Implementations: []invowkfile.Implementation{
			{
				// Execute the script file that was provisioned to /workspace
				Script: "./" + scriptFileName,

				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s",
			result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, "Script executed from /workspace") {
		t.Errorf("Execute() output = %q, want to contain 'Script executed from /workspace'", output)
	}
}

// TestContainerRuntime_ProvisioningLayer_NestedDirectories tests that nested directories
// in the invowkfile directory are correctly provisioned to /workspace.
func TestContainerRuntime_ProvisioningLayer_NestedDirectories(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sem := testutil.ContainerSemaphore()
	sem <- struct{}{}
	defer func() { <-sem }()

	tmpDir, inv := setupTestInvowkfile(t)

	// Create a nested directory structure
	nestedDir := filepath.Join(tmpDir, "scripts", "helpers")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	// Create a file in the nested directory
	nestedFileName := "helper.txt"
	nestedContent := "content from nested directory"
	nestedFilePath := filepath.Join(nestedDir, nestedFileName)
	if err := os.WriteFile(nestedFilePath, []byte(nestedContent), 0o644); err != nil {
		t.Fatalf("Failed to write nested file: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "test-nested-provision",
		Implementations: []invowkfile.Implementation{
			{
				Script: "cat /workspace/scripts/helpers/" + nestedFileName,

				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s",
			result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != nestedContent {
		t.Errorf("Execute() output = %q, want %q", output, nestedContent)
	}
}

// TestContainerRuntime_ProvisioningLayer_WorkspaceIsCwd tests that /workspace is the
// default current working directory in the container.
func TestContainerRuntime_ProvisioningLayer_WorkspaceIsCwd(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sem := testutil.ContainerSemaphore()
	sem <- struct{}{}
	defer func() { <-sem }()

	_, inv := setupTestInvowkfile(t)

	cmd := &invowkfile.Command{
		Name: "test-workspace-cwd",
		Implementations: []invowkfile.Implementation{
			{
				Script: "pwd",

				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()

	var stdout, stderr bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &stderr

	result := rt.Execute(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v, stderr: %s",
			result.ExitCode, result.Error, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != "/workspace" {
		t.Errorf("Execute() pwd = %q, want '/workspace'", output)
	}
}
