// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"invowk-cli/internal/issue"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// T028: BaseCLIEngine BuildArgs tests
func TestBaseCLIEngine_BuildArgs(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name     string
		opts     BuildOptions
		expected []string
	}{
		{
			name: "minimal build",
			opts: BuildOptions{
				ContextDir: ".",
			},
			expected: []string{"build", "."},
		},
		{
			name: "build with tag",
			opts: BuildOptions{
				ContextDir: "/app",
				Tag:        "myimage:latest",
			},
			expected: []string{"build", "-t", "myimage:latest", "/app"},
		},
		{
			name: "build with dockerfile",
			opts: BuildOptions{
				ContextDir: "/app",
				Dockerfile: "Dockerfile.custom",
			},
			//nolint:gocritic // filepathJoin: testing that production code joins paths correctly
			expected: []string{"build", "-f", filepath.Join("/app", "Dockerfile.custom"), "/app"},
		},
		{
			name: "build with absolute dockerfile",
			opts: BuildOptions{
				ContextDir: "/app",
				Dockerfile: "/custom/Dockerfile",
			},
			expected: []string{"build", "-f", "/custom/Dockerfile", "/app"},
		},
		{
			name: "build with no-cache",
			opts: BuildOptions{
				ContextDir: ".",
				NoCache:    true,
			},
			expected: []string{"build", "--no-cache", "."},
		},
		{
			name: "build with all options",
			opts: BuildOptions{
				ContextDir: "/app",
				Dockerfile: "Dockerfile.prod",
				Tag:        "myimage:v1",
				NoCache:    true,
			},
			//nolint:gocritic // filepathJoin: testing that production code joins paths correctly
			expected: []string{"build", "-f", filepath.Join("/app", "Dockerfile.prod"), "-t", "myimage:v1", "--no-cache", "/app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := engine.BuildArgs(tt.opts)

			// Check all expected args are present in order
			if len(args) != len(tt.expected) {
				t.Errorf("got %d args, want %d args\ngot:  %v\nwant: %v", len(args), len(tt.expected), args, tt.expected)
				return
			}

			for i, exp := range tt.expected {
				if args[i] != exp {
					t.Errorf("arg[%d] = %q, want %q\nfull args: %v", i, args[i], exp, args)
				}
			}
		})
	}
}

// T029: BaseCLIEngine RunArgs tests
func TestBaseCLIEngine_RunArgs(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name     string
		opts     RunOptions
		contains []string // args that must be present
		excludes []string // args that must not be present
	}{
		{
			name: "minimal run",
			opts: RunOptions{
				Image: "alpine",
			},
			contains: []string{"run", "alpine"},
		},
		{
			name: "run with rm",
			opts: RunOptions{
				Image:  "alpine",
				Remove: true,
			},
			contains: []string{"run", "--rm", "alpine"},
		},
		{
			name: "run with name",
			opts: RunOptions{
				Image: "alpine",
				Name:  "mycontainer",
			},
			contains: []string{"--name", "mycontainer"},
		},
		{
			name: "run with workdir",
			opts: RunOptions{
				Image:   "alpine",
				WorkDir: "/app",
			},
			contains: []string{"-w", "/app"},
		},
		{
			name: "run interactive with tty",
			opts: RunOptions{
				Image:       "alpine",
				Interactive: true,
				TTY:         true,
			},
			contains: []string{"-i", "-t"},
		},
		{
			name: "run with volumes",
			opts: RunOptions{
				Image:   "alpine",
				Volumes: []string{"/host:/container"},
			},
			contains: []string{"-v", "/host:/container"},
		},
		{
			name: "run with ports",
			opts: RunOptions{
				Image: "nginx",
				Ports: []string{"8080:80"},
			},
			contains: []string{"-p", "8080:80"},
		},
		{
			name: "run with extra hosts",
			opts: RunOptions{
				Image:      "alpine",
				ExtraHosts: []string{"host.docker.internal:host-gateway"},
			},
			contains: []string{"--add-host", "host.docker.internal:host-gateway"},
		},
		{
			name: "run with command",
			opts: RunOptions{
				Image:   "alpine",
				Command: []string{"echo", "hello"},
			},
			contains: []string{"alpine", "echo", "hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := engine.RunArgs(tt.opts)

			for _, exp := range tt.contains {
				if !slices.Contains(args, exp) {
					t.Errorf("args missing %q\nfull args: %v", exp, args)
				}
			}

			for _, exc := range tt.excludes {
				if slices.Contains(args, exc) {
					t.Errorf("args should not contain %q\nfull args: %v", exc, args)
				}
			}
		})
	}
}

// T030: BaseCLIEngine ExecArgs tests
func TestBaseCLIEngine_ExecArgs(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name        string
		containerID string
		command     []string
		opts        RunOptions
		contains    []string
	}{
		{
			name:        "simple exec",
			containerID: "abc123",
			command:     []string{"ls"},
			opts:        RunOptions{},
			contains:    []string{"exec", "abc123", "ls"},
		},
		{
			name:        "exec interactive",
			containerID: "abc123",
			command:     []string{"/bin/bash"},
			opts:        RunOptions{Interactive: true, TTY: true},
			contains:    []string{"exec", "-i", "-t", "abc123", "/bin/bash"},
		},
		{
			name:        "exec with workdir",
			containerID: "abc123",
			command:     []string{"pwd"},
			opts:        RunOptions{WorkDir: "/app"},
			contains:    []string{"-w", "/app"},
		},
		{
			name:        "exec with env",
			containerID: "abc123",
			command:     []string{"env"},
			opts:        RunOptions{Env: map[string]string{"FOO": "bar"}},
			contains:    []string{"-e", "FOO=bar"},
		},
		{
			name:        "exec with multi-word command",
			containerID: "abc123",
			command:     []string{"sh", "-c", "echo hello"},
			opts:        RunOptions{},
			contains:    []string{"abc123", "sh", "-c", "echo hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := engine.ExecArgs(tt.containerID, tt.command, tt.opts)

			for _, exp := range tt.contains {
				if !slices.Contains(args, exp) {
					t.Errorf("args missing %q\nfull args: %v", exp, args)
				}
			}
		})
	}
}

func TestBaseCLIEngine_RemoveArgs(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name        string
		containerID string
		force       bool
		expected    []string
	}{
		{
			name:        "remove without force",
			containerID: "abc123",
			force:       false,
			expected:    []string{"rm", "abc123"},
		},
		{
			name:        "remove with force",
			containerID: "abc123",
			force:       true,
			expected:    []string{"rm", "-f", "abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := engine.RemoveArgs(tt.containerID, tt.force)
			if len(args) != len(tt.expected) {
				t.Errorf("got %d args, want %d\ngot: %v\nwant: %v", len(args), len(tt.expected), args, tt.expected)
				return
			}
			for i, exp := range tt.expected {
				if args[i] != exp {
					t.Errorf("arg[%d] = %q, want %q", i, args[i], exp)
				}
			}
		})
	}
}

func TestBaseCLIEngine_RemoveImageArgs(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name     string
		image    string
		force    bool
		expected []string
	}{
		{
			name:     "remove image without force",
			image:    "myimage:latest",
			force:    false,
			expected: []string{"rmi", "myimage:latest"},
		},
		{
			name:     "remove image with force",
			image:    "myimage:latest",
			force:    true,
			expected: []string{"rmi", "-f", "myimage:latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := engine.RemoveImageArgs(tt.image, tt.force)
			if len(args) != len(tt.expected) {
				t.Errorf("got %d args, want %d\ngot: %v\nwant: %v", len(args), len(tt.expected), args, tt.expected)
				return
			}
			for i, exp := range tt.expected {
				if args[i] != exp {
					t.Errorf("arg[%d] = %q, want %q", i, args[i], exp)
				}
			}
		})
	}
}

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

func TestResolveDockerfilePath(t *testing.T) {
	tests := []struct {
		name           string
		contextPath    string
		dockerfilePath string
		expected       string
		wantErr        bool
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

// Test that CreateCommand returns a proper exec.Cmd
func TestBaseCLIEngine_CreateCommand(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	cmd := engine.CreateCommand(context.Background(), "version", "--format", "{{.Server.Version}}")

	if cmd.Path == "" {
		t.Error("CreateCommand returned cmd with empty Path")
	}

	// Check args contain what we expect (args[0] is typically the binary name)
	if !slices.Contains(cmd.Args, "version") {
		t.Errorf("CreateCommand args should contain 'version', got: %v", cmd.Args)
	}
}

// Test that build args include build args properly
func TestBaseCLIEngine_BuildArgsWithBuildArgs(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	args := engine.BuildArgs(BuildOptions{
		ContextDir: ".",
		BuildArgs: map[string]string{
			"VERSION": "1.0.0",
			"ENV":     "prod",
		},
	})

	// Since map iteration order is non-deterministic, just check both are present
	versionFound := false
	envFound := false

	for i, arg := range args {
		if arg == "--build-arg" && i+1 < len(args) {
			if args[i+1] == "VERSION=1.0.0" {
				versionFound = true
			}
			if args[i+1] == "ENV=prod" {
				envFound = true
			}
		}
	}

	if !versionFound {
		t.Errorf("missing VERSION=1.0.0 build arg\nargs: %v", args)
	}
	if !envFound {
		t.Errorf("missing ENV=prod build arg\nargs: %v", args)
	}
}

// Test run args with env vars
func TestBaseCLIEngine_RunArgsWithEnv(t *testing.T) {
	engine := NewBaseCLIEngine("/usr/bin/docker")

	args := engine.RunArgs(RunOptions{
		Image: "alpine",
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	})

	// Check both env vars are present
	fooFound := false
	bazFound := false

	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) {
			if args[i+1] == "FOO=bar" {
				fooFound = true
			}
			if args[i+1] == "BAZ=qux" {
				bazFound = true
			}
		}
	}

	if !fooFound {
		t.Errorf("missing FOO=bar env var\nargs: %v", args)
	}
	if !bazFound {
		t.Errorf("missing BAZ=qux env var\nargs: %v", args)
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
