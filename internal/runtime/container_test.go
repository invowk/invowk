// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// MockEngine implements container.Engine for testing.
// It records calls and returns configured results without requiring Docker/Podman.
type MockEngine struct {
	mu sync.Mutex

	// Configuration
	name        string
	available   bool
	runResult   *container.RunResult
	runErr      error
	imageExists bool
	buildErr    error
	version     string

	// Call recording
	RunCalls        []container.RunOptions
	BuildCalls      []container.BuildOptions
	PrepareRunCalls []container.RunOptions
}

// NewMockEngine creates a MockEngine with sensible defaults.
func NewMockEngine() *MockEngine {
	return &MockEngine{
		name:        "mock",
		available:   true,
		imageExists: true,
		runResult:   &container.RunResult{ExitCode: 0},
		version:     "1.0.0",
	}
}

// WithName sets the engine name.
func (m *MockEngine) WithName(name string) *MockEngine {
	m.name = name
	return m
}

// WithAvailable sets whether the engine is available.
func (m *MockEngine) WithAvailable(available bool) *MockEngine {
	m.available = available
	return m
}

// WithRunResult configures the result of Run() calls.
func (m *MockEngine) WithRunResult(exitCode ExitCode, err error) *MockEngine {
	m.runResult = &container.RunResult{ExitCode: exitCode}
	m.runErr = err
	return m
}

// WithRunError configures Run() to return an error.
func (m *MockEngine) WithRunError(err error) *MockEngine {
	m.runErr = err
	return m
}

// WithImageExists configures whether images exist.
func (m *MockEngine) WithImageExists(exists bool) *MockEngine {
	m.imageExists = exists
	return m
}

// WithBuildError configures Build() to return an error.
func (m *MockEngine) WithBuildError(err error) *MockEngine {
	m.buildErr = err
	return m
}

// --- container.Engine interface implementation ---

func (m *MockEngine) Name() string {
	return m.name
}

func (m *MockEngine) Available() bool {
	return m.available
}

func (m *MockEngine) Version(_ context.Context) (string, error) {
	return m.version, nil
}

func (m *MockEngine) Build(_ context.Context, opts container.BuildOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BuildCalls = append(m.BuildCalls, opts)
	return m.buildErr
}

func (m *MockEngine) Run(_ context.Context, opts container.RunOptions) (*container.RunResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RunCalls = append(m.RunCalls, opts)
	if m.runErr != nil {
		return nil, m.runErr
	}
	return m.runResult, nil
}

func (m *MockEngine) Remove(_ context.Context, _ container.ContainerID, _ bool) error {
	return nil
}

func (m *MockEngine) ImageExists(_ context.Context, _ container.ImageTag) (bool, error) {
	return m.imageExists, nil
}

func (m *MockEngine) RemoveImage(_ context.Context, _ container.ImageTag, _ bool) error {
	return nil
}

func (m *MockEngine) BinaryPath() string {
	return "/usr/bin/" + m.name
}

func (m *MockEngine) BuildRunArgs(opts container.RunOptions) []string {
	args := []string{"run"}
	if opts.Remove {
		args = append(args, "--rm")
	}
	args = append(args, string(opts.Image))
	args = append(args, opts.Command...)
	return args
}

func (m *MockEngine) PrepareRunCommand(ctx context.Context, opts container.RunOptions) *exec.Cmd {
	m.mu.Lock()
	m.PrepareRunCalls = append(m.PrepareRunCalls, opts)
	m.mu.Unlock()
	return exec.CommandContext(ctx, m.BinaryPath(), m.BuildRunArgs(opts)...)
}

// --- Tests ---

// TestNewContainerRuntimeWithEngine tests the constructor with a mock engine.
func TestNewContainerRuntimeWithEngine(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
	}

	if rt == nil {
		t.Fatal("NewContainerRuntimeWithEngine() returned nil")
	}
	if rt.engine != engine {
		t.Error("NewContainerRuntimeWithEngine() engine not set")
	}
	if rt.provisioner == nil {
		t.Error("NewContainerRuntimeWithEngine() provisioner should be initialized")
	}
}

// TestContainerRuntime_Name tests that Name() returns "container".
func TestContainerRuntime_Name(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
	}

	if name := rt.Name(); name != "container" {
		t.Errorf("Name() = %q, want %q", name, "container")
	}
}

// TestContainerRuntime_Available tests that Available() delegates to the engine.
func TestContainerRuntime_Available(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		engineAvailable bool
		want            bool
	}{
		{
			name:            "engine available",
			engineAvailable: true,
			want:            true,
		},
		{
			name:            "engine not available",
			engineAvailable: false,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := NewMockEngine().WithAvailable(tt.engineAvailable)
			rt, err := NewContainerRuntimeWithEngine(engine)
			if err != nil {
				t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
			}

			if got := rt.Available(); got != tt.want {
				t.Errorf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContainerRuntime_Available_NilEngine tests Available() with nil engine.
func TestContainerRuntime_Available_NilEngine(t *testing.T) {
	t.Parallel()

	rt := &ContainerRuntime{engine: nil}

	if got := rt.Available(); got != false {
		t.Errorf("Available() with nil engine = %v, want false", got)
	}
}

func TestContainerRuntime_PrepareCommandValidatesRunOptions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	cmd := &invowkfile.Command{
		Name: "invalid-port",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo hello",
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:  invowkfile.RuntimeContainer,
					Image: "debian:stable-slim",
					Ports: []invowkfile.PortMappingSpec{
						"not-a-port",
					},
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
		},
	}

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.SelectedRuntime = invowkfile.RuntimeContainer
	ctx.SelectedImpl = &cmd.Implementations[0]

	_, err = rt.PrepareCommand(ctx)
	if err == nil {
		t.Fatal("PrepareCommand() returned nil error, want invalid run options")
	}
	if len(engine.PrepareRunCalls) != 0 {
		t.Fatalf("PrepareRunCommand calls = %d, want 0", len(engine.PrepareRunCalls))
	}
}

// TestContainerRuntime_Validate_Unit tests the validation logic (unit tests without containers).
func TestContainerRuntime_Validate_Unit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	tests := []struct {
		name         string
		cmd          *invowkfile.Command
		wantErr      bool
		wantSentinel error  // sentinel for errors.Is check (preferred)
		errMsg       string // substring for format verification (when no sentinel exists)
	}{
		{
			name: "valid with image",
			cmd: &invowkfile.Command{
				Name: "valid",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo hello",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil implementation",
			cmd: &invowkfile.Command{
				Name: "nil-impl",
				Implementations: []invowkfile.Implementation{
					{
						Script:   "echo hello",
						Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, // Wrong runtime
					},
				},
			},
			wantErr:      true,
			wantSentinel: errContainerNoImpl,
		},
		{
			name: "empty script",
			cmd: &invowkfile.Command{
				Name: "empty-script",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
			wantErr:      true,
			wantSentinel: errContainerNoScript,
		},
		{
			name: "missing image and containerfile",
			cmd: &invowkfile.Command{
				Name: "no-image",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo hello",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}, // No image
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
			wantErr:      true,
			wantSentinel: ErrContainerBuildConfig,
			errMsg:       "containerfile or image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := NewMockEngine()
			rt, err := NewContainerRuntimeWithEngine(engine)
			if err != nil {
				t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
			}

			ctx := NewExecutionContext(t.Context(), tt.cmd, inv)
			// For the "nil implementation" test, we need to manually set it to nil
			if tt.name == "nil implementation" {
				ctx.SelectedImpl = nil
			}

			err = rt.Validate(ctx)

			switch {
			case tt.wantErr && err == nil:
				t.Error("Validate() expected error, got nil")
			case tt.wantErr && tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel):
				t.Errorf("Validate() error = %q, want sentinel %q", err.Error(), tt.wantSentinel)
			case tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg):
				t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
			case !tt.wantErr && err != nil:
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

// TestIsWindowsContainerImage tests detection of Windows container images.
func TestIsWindowsContainerImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		image string
		want  bool
	}{
		// Windows images
		{"mcr.microsoft.com/windows/servercore:ltsc2022", true},
		{"mcr.microsoft.com/windows/nanoserver:ltsc2022", true},
		{"mcr.microsoft.com/powershell:lts-nanoserver-ltsc2022", true},
		{"mcr.microsoft.com/powershell:latest", true},
		{"microsoft/windowsservercore", true},
		{"microsoft/nanoserver", true},
		{"MCR.MICROSOFT.COM/WINDOWS/SERVERCORE:latest", true}, // Case insensitive

		// Linux images (should NOT match)
		{"debian:stable-slim", false},
		{"alpine:latest", false},
		{"registry.example.com/team/app:22.04", false},
		{"python:3.11-slim", false},
		{"mcr.microsoft.com/dotnet/runtime:7.0", false}, // Linux .NET image
		{"mcr.microsoft.com/azure-cli:latest", false},   // Linux Azure CLI
		{"my-custom-image:latest", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()

			got := errors.Is(container.ValidateSupportedRuntimeImage(container.ImageTag(tt.image)), container.ErrWindowsContainerImage)
			if got != tt.want {
				t.Errorf("ValidateSupportedRuntimeImage(%q) windows error = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

func TestIsAlpineContainerImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		image string
		want  bool
	}{
		// Positive matches: bare name, tagged, and registry-qualified.
		{"alpine", true},
		{"alpine:3.20", true},
		{"docker.io/library/alpine:latest", true},
		{"ghcr.io/example/alpine:edge", true},
		{"alpine@sha256:abcdef1234567890", true},

		// Negative: images whose names contain "alpine" as a substring but are
		// NOT the official Alpine image (segment-aware matching).
		{"go-alpine-builder:v1", false},
		{"myorg/alpine-tools:latest", false},
		{"registry.example.com/team/go-alpine:v2", false},

		// Negative: unrelated images.
		{"debian:stable-slim", false},
		{"registry.example.com/team/app:22.04", false},
		{"mcr.microsoft.com/windows/servercore:ltsc2022", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()

			got := errors.Is(container.ValidateSupportedRuntimeImage(container.ImageTag(tt.image)), container.ErrAlpineContainerImage)
			if got != tt.want {
				t.Errorf("ValidateSupportedRuntimeImage(%q) alpine error = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

func TestValidateSupportedContainerImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		image      container.ImageTag
		wantErr    bool
		wantTarget error
	}{
		{
			name:       "windows image rejected",
			image:      container.ImageTag("mcr.microsoft.com/windows/servercore:ltsc2022"),
			wantErr:    true,
			wantTarget: container.ErrWindowsContainerImage,
		},
		{
			name:       "alpine image rejected",
			image:      container.ImageTag("alpine:latest"),
			wantErr:    true,
			wantTarget: container.ErrAlpineContainerImage,
		},
		{
			name:    "debian image allowed",
			image:   container.ImageTag("debian:stable-slim"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := container.ValidateSupportedRuntimeImage(tt.image)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateSupportedRuntimeImage(%q) expected error, got nil", tt.image)
				}
				if tt.wantTarget != nil && !errors.Is(err, tt.wantTarget) {
					t.Fatalf("ValidateSupportedRuntimeImage(%q) error = %q, want errors.Is(%v)", tt.image, err.Error(), tt.wantTarget)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateSupportedRuntimeImage(%q) unexpected error: %v", tt.image, err)
			}
		})
	}
}

// TestGetContainerWorkDir tests the working directory resolution for containers.
func TestGetContainerWorkDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	invowkDir := tmpDir
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	tests := []struct {
		name               string
		cmdWorkDir         string
		ctxWorkDirOverride invowkfile.WorkDir
		want               string
	}{
		{
			name:       "no workdir defaults to /workspace",
			cmdWorkDir: "",
			want:       "/workspace",
		},
		{
			name:       "relative workdir maps to /workspace/subdir",
			cmdWorkDir: "subdir",
			want:       "/workspace/subdir",
		},
		{
			name:       "absolute path inside invowkfile dir maps to /workspace",
			cmdWorkDir: subDir, // absolute path inside invowkfile dir
			want:       "/workspace/subdir",
		},
		{
			name:       "container absolute workdir stays absolute",
			cmdWorkDir: "/app",
			want:       "/app",
		},
		{
			name:               "CLI override takes precedence",
			cmdWorkDir:         "",
			ctxWorkDirOverride: "cli-override",
			want:               "/workspace/cli-override",
		},
		{
			name:               "container absolute CLI override stays absolute",
			cmdWorkDir:         "",
			ctxWorkDirOverride: "/app",
			want:               "/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &invowkfile.Command{
				Name:    "workdir-test",
				WorkDir: invowkfile.WorkDir(tt.cmdWorkDir),
				Implementations: []invowkfile.Implementation{
					{
						Script:    "pwd",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
					},
				},
			}
			inv := &invowkfile.Invowkfile{
				FilePath: invowkfile.FilesystemPath(invowkfilePath),
			}

			engine := NewMockEngine()
			rt, err := NewContainerRuntimeWithEngine(engine)
			if err != nil {
				t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
			}
			ctx := NewExecutionContext(t.Context(), cmd, inv)
			if tt.ctxWorkDirOverride != "" {
				ctx.WorkDir = tt.ctxWorkDirOverride
			}

			got := rt.getContainerWorkDir(ctx, invowkDir)
			if got != tt.want {
				t.Errorf("getContainerWorkDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestContainerConfigFromRuntime tests the helper that extracts container config.
func TestContainerConfigFromRuntime(t *testing.T) {
	t.Parallel()

	t.Run("nil runtime config", func(t *testing.T) {
		t.Parallel()

		cfg := containerConfigFromRuntime(nil)
		if cfg.Image != "" || cfg.Containerfile != "" {
			t.Error("containerConfigFromRuntime(nil) should return empty config")
		}
	})

	t.Run("with all fields", func(t *testing.T) {
		t.Parallel()

		rtConfig := &invowkfile.RuntimeConfig{
			Name:          invowkfile.RuntimeContainer,
			Image:         "debian:stable-slim",
			Containerfile: "Containerfile.test",
			Volumes:       []invowkfile.VolumeMountSpec{"/data:/data:ro"},
			Ports:         []invowkfile.PortMappingSpec{"8080:80"},
		}

		cfg := containerConfigFromRuntime(rtConfig)

		if cfg.Image != "debian:stable-slim" {
			t.Errorf("Image = %q, want %q", cfg.Image, "debian:stable-slim")
		}
		if cfg.Containerfile != "Containerfile.test" {
			t.Errorf("Containerfile = %q, want %q", cfg.Containerfile, "Containerfile.test")
		}
		if len(cfg.Volumes) != 1 || cfg.Volumes[0] != "/data:/data:ro" {
			t.Errorf("Volumes = %v, want %v", cfg.Volumes, []invowkfile.VolumeMountSpec{"/data:/data:ro"})
		}
		if len(cfg.Ports) != 1 || cfg.Ports[0] != "8080:80" {
			t.Errorf("Ports = %v, want %v", cfg.Ports, []invowkfile.PortMappingSpec{"8080:80"})
		}
	})
}
