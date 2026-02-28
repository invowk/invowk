// SPDX-License-Identifier: MPL-2.0

package container

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// T028: BaseCLIEngine BuildArgs tests
func TestBaseCLIEngine_BuildArgs(t *testing.T) {
	t.Parallel()
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name          string
		opts          BuildOptions
		expected      []string
		skipOnWindows bool
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
			expected:      []string{"build", "-f", "/custom/Dockerfile", "/app"},
			skipOnWindows: true, // Unix-style absolute paths are not meaningful on Windows
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
			t.Parallel()
			if tt.skipOnWindows && runtime.GOOS == "windows" {
				t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
			}
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
	t.Parallel()
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
				Image: "debian:stable-slim",
			},
			contains: []string{"run", "debian:stable-slim"},
		},
		{
			name: "run with rm",
			opts: RunOptions{
				Image:  "debian:stable-slim",
				Remove: true,
			},
			contains: []string{"run", "--rm", "debian:stable-slim"},
		},
		{
			name: "run with name",
			opts: RunOptions{
				Image: "debian:stable-slim",
				Name:  "mycontainer",
			},
			contains: []string{"--name", "mycontainer"},
		},
		{
			name: "run with workdir",
			opts: RunOptions{
				Image:   "debian:stable-slim",
				WorkDir: "/app",
			},
			contains: []string{"-w", "/app"},
		},
		{
			name: "run interactive with tty",
			opts: RunOptions{
				Image:       "debian:stable-slim",
				Interactive: true,
				TTY:         true,
			},
			contains: []string{"-i", "-t"},
		},
		{
			name: "run with volumes",
			opts: RunOptions{
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{"/host:/container"},
			},
			contains: []string{"-v", "/host:/container"},
		},
		{
			name: "run with ports",
			opts: RunOptions{
				Image: "nginx",
				Ports: []invowkfile.PortMappingSpec{"8080:80"},
			},
			contains: []string{"-p", "8080:80"},
		},
		{
			name: "run with extra hosts",
			opts: RunOptions{
				Image:      "debian:stable-slim",
				ExtraHosts: []HostMapping{"host.docker.internal:host-gateway"},
			},
			contains: []string{"--add-host", "host.docker.internal:host-gateway"},
		},
		{
			name: "run with command",
			opts: RunOptions{
				Image:   "debian:stable-slim",
				Command: []string{"echo", "hello"},
			},
			contains: []string{"debian:stable-slim", "echo", "hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			args := engine.ExecArgs(ContainerID(tt.containerID), tt.command, tt.opts)

			for _, exp := range tt.contains {
				if !slices.Contains(args, exp) {
					t.Errorf("args missing %q\nfull args: %v", exp, args)
				}
			}
		})
	}
}

func TestBaseCLIEngine_RemoveArgs(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			args := engine.RemoveArgs(ContainerID(tt.containerID), tt.force)
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
	t.Parallel()
	engine := NewBaseCLIEngine("/usr/bin/docker")

	tests := []struct {
		name     string
		image    ImageTag
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
			t.Parallel()
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

// Test that CreateCommand returns a proper exec.Cmd
func TestBaseCLIEngine_CreateCommand(t *testing.T) {
	t.Parallel()
	engine := NewBaseCLIEngine("/usr/bin/docker")

	cmd := engine.CreateCommand(t.Context(), "version", "--format", "{{.Server.Version}}")

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
	t.Parallel()
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
	t.Parallel()
	engine := NewBaseCLIEngine("/usr/bin/docker")

	args := engine.RunArgs(RunOptions{
		Image: "debian:stable-slim",
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
