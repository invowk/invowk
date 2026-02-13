// SPDX-License-Identifier: MPL-2.0

//go:build linux

package container

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSysctlOverrideMemfd_Content(t *testing.T) {
	t.Parallel()

	memfd, err := createSysctlOverrideMemfd()
	if err != nil {
		t.Fatalf("createSysctlOverrideMemfd() error: %v", err)
	}
	defer memfd.Close()

	content, err := io.ReadAll(memfd)
	if err != nil {
		t.Fatalf("reading memfd: %v", err)
	}

	expected := "[containers]\ndefault_sysctls = []\n"
	if string(content) != expected {
		t.Errorf("memfd content = %q, want %q", string(content), expected)
	}
}

// TestCreateSysctlOverrideMemfd_ReadableFromNewFD verifies that opening the
// memfd via /proc/self/fd/<N> gives an independent read offset. This simulates
// what Podman does when it opens CONTAINERS_CONF_OVERRIDE=/dev/fd/3 — os.Open()
// creates a new file description with offset 0, so each child reads from the start.
func TestCreateSysctlOverrideMemfd_ReadableFromNewFD(t *testing.T) {
	t.Parallel()

	memfd, err := createSysctlOverrideMemfd()
	if err != nil {
		t.Fatalf("createSysctlOverrideMemfd() error: %v", err)
	}
	defer memfd.Close()

	// Read all content to advance the offset
	_, _ = io.ReadAll(memfd)

	// Open via /proc/self/fd/N — this is how Podman reads it in the child process
	procPath := fmt.Sprintf("/proc/self/fd/%d", memfd.Fd())
	f2, err := os.Open(procPath)
	if err != nil {
		t.Fatalf("opening %s: %v", procPath, err)
	}
	defer f2.Close()

	content, err := io.ReadAll(f2)
	if err != nil {
		t.Fatalf("reading from new fd: %v", err)
	}

	expected := "[containers]\ndefault_sysctls = []\n"
	if string(content) != expected {
		t.Errorf("content from new fd = %q, want %q", string(content), expected)
	}
}

func TestSysctlOverrideOpts_LocalPodman(t *testing.T) {
	t.Parallel()

	// A local podman binary should get the full override
	opts := sysctlOverrideOpts("/usr/bin/podman")

	// On Linux with memfd support, should return exactly 3 options
	// (WithCmdExtraFile + WithCmdEnvOverride + WithSysctlOverrideActive)
	if len(opts) != 3 {
		t.Fatalf("sysctlOverrideOpts(\"/usr/bin/podman\") returned %d options, want 3", len(opts))
	}

	// Apply options to a test engine and verify
	engine := NewBaseCLIEngine("/usr/bin/podman", opts...)

	if len(engine.cmdExtraFiles) != 1 {
		t.Errorf("expected 1 extra file, got %d", len(engine.cmdExtraFiles))
	}
	if engine.cmdEnvOverrides["CONTAINERS_CONF_OVERRIDE"] != "/dev/fd/3" {
		t.Errorf("expected CONTAINERS_CONF_OVERRIDE=/dev/fd/3, got %q",
			engine.cmdEnvOverrides["CONTAINERS_CONF_OVERRIDE"])
	}
	if !engine.sysctlOverrideActive {
		t.Error("expected sysctlOverrideActive to be true for local podman")
	}

	// Clean up the memfd
	if len(engine.cmdExtraFiles) > 0 {
		engine.cmdExtraFiles[0].Close()
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
