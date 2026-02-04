// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"invowk-cli/internal/container"
	"invowk-cli/internal/provision"
	"invowk-cli/pkg/invkfile"
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
	RunCalls   []container.RunOptions
	BuildCalls []container.BuildOptions
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
func (m *MockEngine) WithRunResult(exitCode int, err error) *MockEngine {
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

func (m *MockEngine) Remove(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *MockEngine) ImageExists(_ context.Context, _ string) (bool, error) {
	return m.imageExists, nil
}

func (m *MockEngine) RemoveImage(_ context.Context, _ string, _ bool) error {
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
	args = append(args, opts.Image)
	args = append(args, opts.Command...)
	return args
}

// --- Tests ---

// TestNewContainerRuntimeWithEngine tests the constructor with a mock engine.
func TestNewContainerRuntimeWithEngine(t *testing.T) {
	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)

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
	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)

	if name := rt.Name(); name != "container" {
		t.Errorf("Name() = %q, want %q", name, "container")
	}
}

// TestContainerRuntime_Available tests that Available() delegates to the engine.
func TestContainerRuntime_Available(t *testing.T) {
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
			engine := NewMockEngine().WithAvailable(tt.engineAvailable)
			rt := NewContainerRuntimeWithEngine(engine)

			if got := rt.Available(); got != tt.want {
				t.Errorf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContainerRuntime_Available_NilEngine tests Available() with nil engine.
func TestContainerRuntime_Available_NilEngine(t *testing.T) {
	rt := &ContainerRuntime{engine: nil}

	if got := rt.Available(); got != false {
		t.Errorf("Available() with nil engine = %v, want false", got)
	}
}

// TestContainerRuntime_Validate_Unit tests the validation logic (unit tests without containers).
func TestContainerRuntime_Validate_Unit(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	tests := []struct {
		name    string
		cmd     *invkfile.Command
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with image",
			cmd: &invkfile.Command{
				Name: "valid",
				Implementations: []invkfile.Implementation{
					{
						Script:   "echo hello",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil implementation",
			cmd: &invkfile.Command{
				Name: "nil-impl",
				Implementations: []invkfile.Implementation{
					{
						Script:   "echo hello",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, // Wrong runtime
					},
				},
			},
			wantErr: true,
			errMsg:  "no implementation selected",
		},
		{
			name: "empty script",
			cmd: &invkfile.Command{
				Name: "empty-script",
				Implementations: []invkfile.Implementation{
					{
						Script:   "",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"}},
					},
				},
			},
			wantErr: true,
			errMsg:  "no script",
		},
		{
			name: "missing image and containerfile",
			cmd: &invkfile.Command{
				Name: "no-image",
				Implementations: []invkfile.Implementation{
					{
						Script:   "echo hello",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}, // No image
					},
				},
			},
			wantErr: true,
			errMsg:  "Containerfile or Dockerfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewMockEngine()
			rt := NewContainerRuntimeWithEngine(engine)

			ctx := NewExecutionContext(tt.cmd, inv)
			// For the "nil implementation" test, we need to manually set it to nil
			if tt.name == "nil implementation" {
				ctx.SelectedImpl = nil
			}

			err := rt.Validate(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestContainerRuntime_Validate_WithContainerfile tests validation with Containerfile present.
func TestContainerRuntime_Validate_WithContainerfile(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	// Create a Containerfile in the temp directory
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte("FROM debian:stable-slim\n"), 0o644); err != nil {
		t.Fatalf("failed to create Containerfile: %v", err)
	}

	cmd := &invkfile.Command{
		Name: "with-containerfile",
		Implementations: []invkfile.Implementation{
			{
				Script:   "echo hello",
				Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}, // No image, but Containerfile exists
			},
		},
	}

	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)
	ctx := NewExecutionContext(cmd, inv)

	err := rt.Validate(ctx)
	if err != nil {
		t.Errorf("Validate() with Containerfile unexpected error: %v", err)
	}
}

// TestContainerRuntime_Validate_WithDockerfile tests validation with Dockerfile present.
func TestContainerRuntime_Validate_WithDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	// Create a Dockerfile in the temp directory
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM debian:stable-slim\n"), 0o644); err != nil {
		t.Fatalf("failed to create Dockerfile: %v", err)
	}

	cmd := &invkfile.Command{
		Name: "with-dockerfile",
		Implementations: []invkfile.Implementation{
			{
				Script:   "echo hello",
				Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}, // No image, but Dockerfile exists
			},
		},
	}

	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)
	ctx := NewExecutionContext(cmd, inv)

	err := rt.Validate(ctx)
	if err != nil {
		t.Errorf("Validate() with Dockerfile unexpected error: %v", err)
	}
}

// TestIsWindowsContainerImage tests detection of Windows container images.
func TestIsWindowsContainerImage(t *testing.T) {
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
		{"ubuntu:22.04", false},
		{"python:3.11-slim", false},
		{"mcr.microsoft.com/dotnet/runtime:7.0", false}, // Linux .NET image
		{"mcr.microsoft.com/azure-cli:latest", false},   // Linux Azure CLI
		{"my-custom-image:latest", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := isWindowsContainerImage(tt.image)
			if got != tt.want {
				t.Errorf("isWindowsContainerImage(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

// TestGetContainerWorkDir tests the working directory resolution for containers.
func TestGetContainerWorkDir(t *testing.T) {
	tmpDir := t.TempDir()
	invowkDir := tmpDir
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	tests := []struct {
		name               string
		cmdWorkDir         string
		ctxWorkDirOverride string
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
			name:       "absolute path inside invkfile dir maps to /workspace",
			cmdWorkDir: subDir, // absolute path inside invkfile dir
			want:       "/workspace/subdir",
		},
		{
			name:               "CLI override takes precedence",
			cmdWorkDir:         "",
			ctxWorkDirOverride: "cli-override",
			want:               "/workspace/cli-override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &invkfile.Command{
				Name:    "workdir-test",
				WorkDir: tt.cmdWorkDir,
				Implementations: []invkfile.Implementation{
					{
						Script:   "pwd",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"}},
					},
				},
			}
			inv := &invkfile.Invkfile{
				FilePath: invkfilePath,
			}

			engine := NewMockEngine()
			rt := NewContainerRuntimeWithEngine(engine)
			ctx := NewExecutionContext(cmd, inv)
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
	t.Run("nil runtime config", func(t *testing.T) {
		cfg := containerConfigFromRuntime(nil)
		if cfg.Image != "" || cfg.Containerfile != "" {
			t.Error("containerConfigFromRuntime(nil) should return empty config")
		}
	})

	t.Run("with all fields", func(t *testing.T) {
		rtConfig := &invkfile.RuntimeConfig{
			Name:          invkfile.RuntimeContainer,
			Image:         "debian:stable-slim",
			Containerfile: "Containerfile.test",
			Volumes:       []string{"/data:/data:ro"},
			Ports:         []string{"8080:80"},
		}

		cfg := containerConfigFromRuntime(rtConfig)

		if cfg.Image != "debian:stable-slim" {
			t.Errorf("Image = %q, want %q", cfg.Image, "debian:stable-slim")
		}
		if cfg.Containerfile != "Containerfile.test" {
			t.Errorf("Containerfile = %q, want %q", cfg.Containerfile, "Containerfile.test")
		}
		if len(cfg.Volumes) != 1 || cfg.Volumes[0] != "/data:/data:ro" {
			t.Errorf("Volumes = %v, want %v", cfg.Volumes, []string{"/data:/data:ro"})
		}
		if len(cfg.Ports) != 1 || cfg.Ports[0] != "8080:80" {
			t.Errorf("Ports = %v, want %v", cfg.Ports, []string{"8080:80"})
		}
	})
}

// TestContainerRuntime_SetProvisionConfig tests updating provision config.
func TestContainerRuntime_SetProvisionConfig(t *testing.T) {
	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)

	// Get initial provisioner
	initialProvisioner := rt.provisioner

	// Set new config
	newCfg := &provision.Config{
		Enabled:          true,
		InvowkBinaryPath: "/custom/invowk",
		BinaryMountPath:  "/opt/invowk",
		ModulesMountPath: "/opt/modules",
	}
	rt.SetProvisionConfig(newCfg)

	// Provisioner should be replaced
	if rt.provisioner == initialProvisioner {
		t.Error("SetProvisionConfig() should create new provisioner")
	}
}

// TestContainerRuntime_SetProvisionConfig_Nil tests that nil config is handled.
func TestContainerRuntime_SetProvisionConfig_Nil(t *testing.T) {
	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)

	initialProvisioner := rt.provisioner

	// Setting nil config should not change provisioner
	rt.SetProvisionConfig(nil)

	if rt.provisioner != initialProvisioner {
		t.Error("SetProvisionConfig(nil) should not change provisioner")
	}
}

// TestContainerRuntime_SupportsInteractive tests that container runtime supports interactive mode.
func TestContainerRuntime_SupportsInteractive(t *testing.T) {
	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)

	if !rt.SupportsInteractive() {
		t.Error("SupportsInteractive() = false, want true")
	}
}

// TestDefaultProvisionConfig_Defaults tests the default provisioning configuration values.
func TestDefaultProvisionConfig_Defaults(t *testing.T) {
	cfg := provision.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Check defaults - values from provision package
	if cfg.BinaryMountPath != "/invowk/bin" {
		t.Errorf("BinaryMountPath = %q, want %q", cfg.BinaryMountPath, "/invowk/bin")
	}
	if cfg.ModulesMountPath != "/invowk/modules" {
		t.Errorf("ModulesMountPath = %q, want %q", cfg.ModulesMountPath, "/invowk/modules")
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
}

// TestContainerRuntime_generateImageTag tests the image tag generation.
func TestContainerRuntime_generateImageTag(t *testing.T) {
	engine := NewMockEngine()
	rt := NewContainerRuntimeWithEngine(engine)

	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	tag, err := rt.generateImageTag(invkfilePath)
	if err != nil {
		t.Fatalf("generateImageTag() error: %v", err)
	}

	// Tag should start with "invowk-" and end with ":latest"
	if len(tag) < 20 {
		t.Errorf("generateImageTag() tag too short: %q", tag)
	}
	if tag[:7] != "invowk-" {
		t.Errorf("generateImageTag() tag should start with 'invowk-': %q", tag)
	}
	if tag[len(tag)-7:] != ":latest" {
		t.Errorf("generateImageTag() tag should end with ':latest': %q", tag)
	}

	// Same path should generate same tag
	tag2, _ := rt.generateImageTag(invkfilePath)
	if tag != tag2 {
		t.Errorf("generateImageTag() should be deterministic: %q != %q", tag, tag2)
	}

	// Different path should generate different tag
	otherPath := filepath.Join(tmpDir, "other", "invkfile.cue")
	tag3, _ := rt.generateImageTag(otherPath)
	if tag == tag3 {
		t.Errorf("generateImageTag() different paths should generate different tags")
	}
}
