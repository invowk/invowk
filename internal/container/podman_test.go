// SPDX-License-Identifier: MPL-2.0

package container

import (
	"os/exec"
	"slices"
	"testing"
)

// TestFindPodmanBinary_PreferenceOrder verifies that "podman" is preferred over "podman-remote".
func TestFindPodmanBinary_PreferenceOrder(t *testing.T) {
	t.Parallel()
	// Verify the binary names list has the expected order
	if len(podmanBinaryNames) != 2 {
		t.Fatalf("expected 2 binary names, got %d", len(podmanBinaryNames))
	}
	if podmanBinaryNames[0] != "podman" {
		t.Errorf("expected first binary name to be 'podman', got %q", podmanBinaryNames[0])
	}
	if podmanBinaryNames[1] != "podman-remote" {
		t.Errorf("expected second binary name to be 'podman-remote', got %q", podmanBinaryNames[1])
	}
}

// TestFindPodmanBinary_Integration tests actual binary discovery.
// This is an integration test that depends on system state.
func TestFindPodmanBinary_Integration(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	path := findPodmanBinary()

	// Check what's actually available on the system
	podmanPath, podmanErr := exec.LookPath("podman")
	podmanRemotePath, podmanRemoteErr := exec.LookPath("podman-remote")

	switch {
	case podmanErr == nil:
		// "podman" is available, it should be preferred
		if path != podmanPath {
			t.Errorf("expected findPodmanBinary() to return %q (podman), got %q", podmanPath, path)
		}
	case podmanRemoteErr == nil:
		// Only "podman-remote" is available
		if path != podmanRemotePath {
			t.Errorf("expected findPodmanBinary() to return %q (podman-remote), got %q", podmanRemotePath, path)
		}
	default:
		// Neither is available
		if path != "" {
			t.Errorf("expected findPodmanBinary() to return empty string when no podman binary found, got %q", path)
		}
	}
}

// TestFindPodmanBinary_ReturnsPath verifies the function returns a valid path when podman exists.
func TestFindPodmanBinary_ReturnsPath(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	path := findPodmanBinary()
	if path == "" {
		t.Skip("skipping: no podman binary found on system")
	}

	// The returned path should be an absolute path
	if path[0] != '/' {
		t.Errorf("expected absolute path, got %q", path)
	}
}

// TestMakeUsernsKeepIDAdder tests the --userns=keep-id transformer.
func TestMakeUsernsKeepIDAdder(t *testing.T) {
	t.Parallel()
	transformer := makeUsernsKeepIDAdder()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "simple run command",
			args: []string{"run", "debian:stable-slim"},
			want: []string{"run", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with --rm flag",
			args: []string{"run", "--rm", "debian:stable-slim"},
			want: []string{"run", "--rm", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with multiple flags",
			args: []string{"run", "--rm", "-i", "-t", "debian:stable-slim"},
			want: []string{"run", "--rm", "-i", "-t", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with flag taking value",
			args: []string{"run", "-w", "/workspace", "debian:stable-slim"},
			want: []string{"run", "-w", "/workspace", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with volume mount",
			args: []string{"run", "-v", "/host:/container", "debian:stable-slim"},
			want: []string{"run", "-v", "/host:/container", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with env var",
			args: []string{"run", "-e", "FOO=bar", "debian:stable-slim"},
			want: []string{"run", "-e", "FOO=bar", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with --name flag",
			args: []string{"run", "--name", "mycontainer", "debian:stable-slim"},
			want: []string{"run", "--name", "mycontainer", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with combined flags",
			args: []string{"run", "--rm", "-w", "/app", "-e", "FOO=bar", "-v", "/a:/b", "debian:stable-slim", "echo", "hello"},
			want: []string{"run", "--rm", "-w", "/app", "-e", "FOO=bar", "-v", "/a:/b", "--userns=keep-id", "debian:stable-slim", "echo", "hello"},
		},
		{
			name: "run with --add-host flag",
			args: []string{"run", "--add-host", "myhost:127.0.0.1", "debian:stable-slim"},
			want: []string{"run", "--add-host", "myhost:127.0.0.1", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "run with port mapping",
			args: []string{"run", "-p", "8080:80", "debian:stable-slim"},
			want: []string{"run", "-p", "8080:80", "--userns=keep-id", "debian:stable-slim"},
		},
		{
			name: "non-run command unchanged",
			args: []string{"build", "-t", "myimage", "."},
			want: []string{"build", "-t", "myimage", "."},
		},
		{
			name: "exec command unchanged",
			args: []string{"exec", "-i", "mycontainer", "bash"},
			want: []string{"exec", "-i", "mycontainer", "bash"},
		},
		{
			name: "empty args unchanged",
			args: []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := transformer(tt.args)
			if !slices.Equal(got, tt.want) {
				t.Errorf("makeUsernsKeepIDAdder() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsSELinuxPresent verifies the SELinux detection logic.
// The function checks for /sys/fs/selinux existence rather than enforce status
// because :z labels are needed even when SELinux is disabled but present.
func TestIsSELinuxPresent(t *testing.T) {
	t.Parallel()
	// This is an integration test that checks the actual system state
	result := isSELinuxPresent()

	// The result depends on whether SELinux is installed on the test system.
	// On Linux with SELinux (Fedora, RHEL, etc.) this returns true.
	// On Linux without SELinux (some Debian/Ubuntu) this returns false.
	// On macOS/Windows this returns false.
	t.Logf("isSELinuxPresent() = %v", result)

	// We can't assert a specific value since it depends on the test environment,
	// but we can verify the function doesn't panic and returns a boolean.
	_ = result
}

// TestNewPodmanEngine_AppliesUsernsTransformer verifies the constructor wires up the transformer.
func TestNewPodmanEngine_AppliesUsernsTransformer(t *testing.T) {
	t.Parallel()
	// Create engine with SELinux disabled to isolate the userns behavior
	engine := NewPodmanEngineWithSELinuxCheck(func() bool { return false })

	// Build run args and verify --userns=keep-id is present
	args := engine.RunArgs(RunOptions{
		Image:  "debian:stable-slim",
		Remove: true,
	})

	// Verify --userns=keep-id is in the args
	if !slices.Contains(args, "--userns=keep-id") {
		t.Errorf("expected --userns=keep-id in args, got %v", args)
	}

	// Verify it appears before the image name
	imageIdx := -1
	usernsIdx := -1
	for i, arg := range args {
		if arg == "--userns=keep-id" {
			usernsIdx = i
		}
		if arg == "debian:stable-slim" {
			imageIdx = i
		}
	}
	if usernsIdx == -1 || imageIdx == -1 {
		t.Fatalf("missing expected args: userns=%d, image=%d", usernsIdx, imageIdx)
	}
	if usernsIdx >= imageIdx {
		t.Errorf("--userns=keep-id should appear before image, userns at %d, image at %d", usernsIdx, imageIdx)
	}
}
