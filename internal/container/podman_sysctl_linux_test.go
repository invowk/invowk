// SPDX-License-Identifier: MPL-2.0

//go:build linux

package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSysctlOverrideTempFile_Content(t *testing.T) {
	t.Parallel()

	tempPath, err := createSysctlOverrideTempFile()
	if err != nil {
		t.Fatalf("createSysctlOverrideTempFile() error: %v", err)
	}
	defer os.Remove(tempPath)

	content, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("reading temp file: %v", err)
	}

	expected := "[containers]\ndefault_sysctls = []\n"
	if string(content) != expected {
		t.Errorf("temp file content = %q, want %q", string(content), expected)
	}
}

func TestCreateSysctlOverrideTempFile_UniquePerCall(t *testing.T) {
	t.Parallel()

	path1, err := createSysctlOverrideTempFile()
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	defer os.Remove(path1)

	path2, err := createSysctlOverrideTempFile()
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	defer os.Remove(path2)

	if path1 == path2 {
		t.Errorf("expected unique paths, got %q both times", path1)
	}
}

func TestCreateSysctlOverrideTempFile_HasTomlExtension(t *testing.T) {
	t.Parallel()

	tempPath, err := createSysctlOverrideTempFile()
	if err != nil {
		t.Fatalf("createSysctlOverrideTempFile() error: %v", err)
	}
	defer os.Remove(tempPath)

	if !strings.HasSuffix(tempPath, ".toml") {
		t.Errorf("expected .toml suffix, got %q", tempPath)
	}
}

func TestSysctlOverrideOpts_LocalPodman(t *testing.T) {
	t.Parallel()

	// A local podman binary should get the full override
	opts := sysctlOverrideOpts("/usr/bin/podman")

	// On Linux, should return exactly 3 options
	// (WithCmdEnvOverride + WithSysctlOverridePath + WithSysctlOverrideActive)
	if len(opts) != 3 {
		t.Fatalf("sysctlOverrideOpts(\"/usr/bin/podman\") returned %d options, want 3", len(opts))
	}

	// Apply options to a test engine and verify
	engine := NewBaseCLIEngine("/usr/bin/podman", opts...)
	defer engine.Close()

	if engine.sysctlOverridePath == "" {
		t.Error("expected sysctlOverridePath to be set")
	}
	if engine.cmdEnvOverrides["CONTAINERS_CONF_OVERRIDE"] != engine.sysctlOverridePath {
		t.Errorf("expected CONTAINERS_CONF_OVERRIDE=%q, got %q",
			engine.sysctlOverridePath, engine.cmdEnvOverrides["CONTAINERS_CONF_OVERRIDE"])
	}
	if !engine.sysctlOverrideActive {
		t.Error("expected sysctlOverrideActive to be true for local podman")
	}

	// Verify the temp file is readable with correct content
	content, err := os.ReadFile(engine.sysctlOverridePath)
	if err != nil {
		t.Fatalf("reading override file: %v", err)
	}
	if string(content) != "[containers]\ndefault_sysctls = []\n" {
		t.Errorf("override file content = %q", string(content))
	}
}

func TestSysctlOverrideOpts_RemotePodman(t *testing.T) {
	t.Parallel()

	// podman-remote should get no override (env var doesn't reach the service)
	opts := sysctlOverrideOpts("/usr/bin/podman-remote")

	if len(opts) != 0 {
		t.Errorf("sysctlOverrideOpts(\"/usr/bin/podman-remote\") returned %d options, want 0", len(opts))
	}
}

func TestBaseCLIEngine_Close_RemovesTempFile(t *testing.T) {
	t.Parallel()

	opts := sysctlOverrideOpts("/usr/bin/podman")
	if len(opts) == 0 {
		t.Skip("sysctl override not available")
	}

	engine := NewBaseCLIEngine("/usr/bin/podman", opts...)
	tempPath := engine.sysctlOverridePath

	// Verify the file exists
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("temp file should exist before Close: %v", err)
	}

	// Close should remove it
	if err := engine.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify the file is gone
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temp file should be removed after Close, stat error: %v", err)
	}

	// Second Close should be a no-op (idempotent)
	if err := engine.Close(); err != nil {
		t.Errorf("second Close() should be no-op, got error: %v", err)
	}
}

func TestIsRemotePodman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		binaryPath string
		want       bool
	}{
		{"direct podman-remote", "/usr/bin/podman-remote", true},
		{"local podman", "/usr/bin/podman", false},
		{"nested path podman-remote", "/usr/local/bin/podman-remote", true},
		{"just filename", "podman-remote", true},
		{"just podman", "podman", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isRemotePodman(tt.binaryPath); got != tt.want {
				t.Errorf("isRemotePodman(%q) = %v, want %v", tt.binaryPath, got, tt.want)
			}
		})
	}
}

func TestIsRemotePodman_Symlink(t *testing.T) {
	t.Parallel()

	// Create a temp directory with a symlink: podman -> podman-remote
	dir := t.TempDir()
	remotePath := filepath.Join(dir, "podman-remote")
	symlinkPath := filepath.Join(dir, "podman")

	// Create a fake podman-remote file
	if err := os.WriteFile(remotePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("creating fake binary: %v", err)
	}
	// Create symlink: podman -> podman-remote
	if err := os.Symlink(remotePath, symlinkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// isRemotePodman should detect the symlink target
	if !isRemotePodman(symlinkPath) {
		t.Errorf("isRemotePodman(%q -> %q) = false, want true", symlinkPath, remotePath)
	}
}
