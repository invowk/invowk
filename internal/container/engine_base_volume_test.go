// SPDX-License-Identifier: MPL-2.0

package container

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveDockerfilePath(t *testing.T) {
	tests := []struct {
		name           string
		contextPath    string
		dockerfilePath string
		expected       string
		wantErr        bool
		skipOnWindows  bool
	}{
		{
			name:           "empty path",
			contextPath:    "/app",
			dockerfilePath: "",
			expected:       "",
			wantErr:        false,
		},
		{
			name:           "absolute path",
			contextPath:    "/app",
			dockerfilePath: "/other/Dockerfile",
			expected:       "/other/Dockerfile",
			wantErr:        false,
			skipOnWindows:  true, // Unix-style absolute paths are not meaningful on Windows
		},
		{
			name:           "relative path",
			contextPath:    "/app",
			dockerfilePath: "Dockerfile.custom",
			//nolint:gocritic // filepathJoin: testing that production code joins paths correctly
			expected: filepath.Join("/app", "Dockerfile.custom"),
			wantErr:  false,
		},
		{
			name:           "nested relative path",
			contextPath:    "/app",
			dockerfilePath: "docker/Dockerfile.prod",
			//nolint:gocritic // filepathJoin: testing that production code joins paths correctly
			expected: filepath.Join("/app", "docker/Dockerfile.prod"),
			wantErr:  false,
		},
		{
			name:           "path traversal attempt",
			contextPath:    "/app",
			dockerfilePath: "../../../etc/passwd",
			expected:       "",
			wantErr:        true,
		},
		{
			name:           "complex path traversal",
			contextPath:    "/app/subdir",
			dockerfilePath: "../../outside",
			expected:       "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnWindows && runtime.GOOS == "windows" {
				t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
			}
			got, err := ResolveDockerfilePath(tt.contextPath, tt.dockerfilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveDockerfilePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ResolveDockerfilePath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatVolumeMount(t *testing.T) {
	tests := []struct {
		name     string
		mount    VolumeMount
		expected string
	}{
		{
			name: "simple mount",
			mount: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
			},
			expected: "/host:/container",
		},
		{
			name: "read-only mount",
			mount: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				ReadOnly:      true,
			},
			expected: "/host:/container:ro",
		},
		{
			name: "mount with SELinux",
			mount: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				SELinux:       "z",
			},
			expected: "/host:/container:z",
		},
		{
			name: "mount with SELinux private",
			mount: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				SELinux:       "Z",
			},
			expected: "/host:/container:Z",
		},
		{
			name: "read-only with SELinux",
			mount: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				ReadOnly:      true,
				SELinux:       "z",
			},
			expected: "/host:/container:ro,z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatVolumeMount(tt.mount)
			if got != tt.expected {
				t.Errorf("FormatVolumeMount() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseVolumeMount(t *testing.T) {
	tests := []struct {
		name     string
		volume   string
		expected VolumeMount
	}{
		{
			name:   "simple mount",
			volume: "/host:/container",
			expected: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
			},
		},
		{
			name:   "read-only mount",
			volume: "/host:/container:ro",
			expected: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				ReadOnly:      true,
			},
		},
		{
			name:   "SELinux mount",
			volume: "/host:/container:z",
			expected: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				SELinux:       "z",
			},
		},
		{
			name:   "read-only with SELinux",
			volume: "/host:/container:ro,z",
			expected: VolumeMount{
				HostPath:      "/host",
				ContainerPath: "/container",
				ReadOnly:      true,
				SELinux:       "z",
			},
		},
		{
			name:   "host only",
			volume: "/host",
			expected: VolumeMount{
				HostPath: "/host",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseVolumeMount(tt.volume)
			if got.HostPath != tt.expected.HostPath {
				t.Errorf("HostPath = %q, want %q", got.HostPath, tt.expected.HostPath)
			}
			if got.ContainerPath != tt.expected.ContainerPath {
				t.Errorf("ContainerPath = %q, want %q", got.ContainerPath, tt.expected.ContainerPath)
			}
			if got.ReadOnly != tt.expected.ReadOnly {
				t.Errorf("ReadOnly = %v, want %v", got.ReadOnly, tt.expected.ReadOnly)
			}
			if got.SELinux != tt.expected.SELinux {
				t.Errorf("SELinux = %q, want %q", got.SELinux, tt.expected.SELinux)
			}
		})
	}
}

func TestFormatPortMapping(t *testing.T) {
	tests := []struct {
		name     string
		mapping  PortMapping
		expected string
	}{
		{
			name: "simple mapping",
			mapping: PortMapping{
				HostPort:      8080,
				ContainerPort: 80,
			},
			expected: "8080:80",
		},
		{
			name: "same port",
			mapping: PortMapping{
				HostPort:      80,
				ContainerPort: 80,
			},
			expected: "80:80",
		},
		{
			name: "with tcp protocol (default)",
			mapping: PortMapping{
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      "tcp",
			},
			expected: "8080:80",
		},
		{
			name: "with udp protocol",
			mapping: PortMapping{
				HostPort:      53,
				ContainerPort: 53,
				Protocol:      "udp",
			},
			expected: "53:53/udp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPortMapping(tt.mapping)
			if got != tt.expected {
				t.Errorf("FormatPortMapping() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Integration test with real path (skipped if not on Unix)
func TestResolveDockerfilePath_RealPaths(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("skipping Unix-specific path test on non-Unix platform")
	}

	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Test that relative path within context is allowed
	resolved, err := ResolveDockerfilePath(tmpDir, "subdir/Dockerfile")
	if err != nil {
		t.Errorf("ResolveDockerfilePath() error = %v", err)
	}

	//nolint:gocritic // filepathJoin: testing that production code joins paths correctly
	expected := filepath.Join(tmpDir, "subdir/Dockerfile")
	if resolved != expected {
		t.Errorf("resolved = %q, want %q", resolved, expected)
	}

	// Test that path traversal outside context is rejected
	_, err = ResolveDockerfilePath(subDir, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}
