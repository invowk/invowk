// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
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
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	tests := []struct {
		name    string
		cmd     *invowkfile.Command
		wantErr bool
		errMsg  string
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
			wantErr: true,
			errMsg:  "no implementation selected",
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
			wantErr: true,
			errMsg:  "no script",
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
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
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
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	// Create a Containerfile in the temp directory
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	if err := os.WriteFile(containerfilePath, []byte("FROM debian:stable-slim\n"), 0o644); err != nil {
		t.Fatalf("failed to create Containerfile: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "with-containerfile",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}, // No image, but Containerfile exists
				Platforms: invowkfile.AllPlatformConfigs(),
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
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	// Create a Dockerfile in the temp directory
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM debian:stable-slim\n"), 0o644); err != nil {
		t.Fatalf("failed to create Dockerfile: %v", err)
	}

	cmd := &invowkfile.Command{
		Name: "with-dockerfile",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}, // No image, but Dockerfile exists
				Platforms: invowkfile.AllPlatformConfigs(),
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

func TestIsAlpineContainerImage(t *testing.T) {
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
		{"ubuntu:22.04", false},
		{"mcr.microsoft.com/windows/servercore:ltsc2022", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := isAlpineContainerImage(tt.image)
			if got != tt.want {
				t.Errorf("isAlpineContainerImage(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

func TestValidateSupportedContainerImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantErr  bool
		contains string
	}{
		{
			name:     "windows image rejected",
			image:    "mcr.microsoft.com/windows/servercore:ltsc2022",
			wantErr:  true,
			contains: "windows container images are not supported",
		},
		{
			name:     "alpine image rejected",
			image:    "alpine:latest",
			wantErr:  true,
			contains: "alpine-based container images are not supported",
		},
		{
			name:    "debian image allowed",
			image:   "debian:stable-slim",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSupportedContainerImage(tt.image)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateSupportedContainerImage(%q) expected error, got nil", tt.image)
				}
				if tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
					t.Fatalf("validateSupportedContainerImage(%q) error = %q, want to contain %q", tt.image, err.Error(), tt.contains)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateSupportedContainerImage(%q) unexpected error: %v", tt.image, err)
			}
		})
	}
}

// TestGetContainerWorkDir tests the working directory resolution for containers.
func TestGetContainerWorkDir(t *testing.T) {
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
			name:       "absolute path inside invowkfile dir maps to /workspace",
			cmdWorkDir: subDir, // absolute path inside invowkfile dir
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
			cmd := &invowkfile.Command{
				Name:    "workdir-test",
				WorkDir: tt.cmdWorkDir,
				Implementations: []invowkfile.Implementation{
					{
						Script:    "pwd",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
					},
				},
			}
			inv := &invowkfile.Invowkfile{
				FilePath: invowkfilePath,
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
		rtConfig := &invowkfile.RuntimeConfig{
			Name:          invowkfile.RuntimeContainer,
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
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	tag, err := rt.generateImageTag(invowkfilePath)
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
	tag2, _ := rt.generateImageTag(invowkfilePath)
	if tag != tag2 {
		t.Errorf("generateImageTag() should be deterministic: %q != %q", tag, tag2)
	}

	// Different path should generate different tag
	otherPath := filepath.Join(tmpDir, "other", "invowkfile.cue")
	tag3, _ := rt.generateImageTag(otherPath)
	if tag == tag3 {
		t.Errorf("generateImageTag() different paths should generate different tags")
	}
}

// TestBuildProvisionConfig_StrictPropagation tests that the Strict field
// from config.AutoProvisionConfig is propagated to provision.Config.
func TestBuildProvisionConfig_StrictPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		strict bool
	}{
		{"strict enabled", true},
		{"strict disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.DefaultConfig()
			cfg.Container.AutoProvision.Strict = tt.strict

			provCfg := buildProvisionConfig(cfg)

			if provCfg.Strict != tt.strict {
				t.Errorf("buildProvisionConfig().Strict = %v, want %v", provCfg.Strict, tt.strict)
			}
		})
	}
}

// TestEnsureProvisionedImage_StrictMode tests that strict provisioning mode
// returns a hard error when provisioning fails.
func TestEnsureProvisionedImage_StrictMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	cmd := &invowkfile.Command{
		Name: "strict-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
		},
	}

	engine := NewMockEngine().WithImageExists(false).WithBuildError(fmt.Errorf("disk full"))

	// Configure provisioner with strict=true and a non-existent binary path
	// to force Provision() to fail during resource hash computation.
	provCfg := &provision.Config{
		Enabled:          true,
		Strict:           true,
		InvowkBinaryPath: filepath.Join(tmpDir, "nonexistent-invowk"),
		BinaryMountPath:  "/invowk/bin",
		ModulesMountPath: "/invowk/modules",
	}
	rt := NewContainerRuntimeWithEngine(engine)
	rt.SetProvisionConfig(provCfg)

	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()
	var stderr bytes.Buffer
	execCtx.IO.Stderr = &stderr
	execCtx.IO.Stdout = &bytes.Buffer{}

	cfg := invowkfileContainerConfig{Image: "debian:stable-slim"}
	_, _, err := rt.ensureProvisionedImage(execCtx, cfg, tmpDir)

	if err == nil {
		t.Fatal("ensureProvisionedImage() with strict=true should return error on provisioning failure")
	}
	if !strings.Contains(err.Error(), "strict mode enabled") {
		t.Errorf("error should mention strict mode, got: %v", err)
	}
}

// TestEnsureProvisionedImage_NonStrictMode tests that non-strict provisioning mode
// falls back to the base image with a warning when provisioning fails.
func TestEnsureProvisionedImage_NonStrictMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	cmd := &invowkfile.Command{
		Name: "non-strict-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
		},
	}

	engine := NewMockEngine().WithImageExists(false).WithBuildError(fmt.Errorf("disk full"))

	// Configure provisioner with strict=false and a non-existent binary path
	provCfg := &provision.Config{
		Enabled:          true,
		Strict:           false,
		InvowkBinaryPath: filepath.Join(tmpDir, "nonexistent-invowk"),
		BinaryMountPath:  "/invowk/bin",
		ModulesMountPath: "/invowk/modules",
	}
	rt := NewContainerRuntimeWithEngine(engine)
	rt.SetProvisionConfig(provCfg)

	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = context.Background()
	var stderr bytes.Buffer
	execCtx.IO.Stderr = &stderr
	execCtx.IO.Stdout = &bytes.Buffer{}

	cfg := invowkfileContainerConfig{Image: "debian:stable-slim"}
	imageName, _, err := rt.ensureProvisionedImage(execCtx, cfg, tmpDir)
	if err != nil {
		t.Fatalf("ensureProvisionedImage() with strict=false should not return error, got: %v", err)
	}
	if imageName != "debian:stable-slim" {
		t.Errorf("imageName = %q, want %q (should fall back to base image)", imageName, "debian:stable-slim")
	}

	// Verify the warning message contains actionable information
	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "WARNING") {
		t.Error("stderr should contain WARNING")
	}
	if !strings.Contains(stderrOutput, "strict") {
		t.Error("stderr should mention strict mode as the remedy")
	}
	if !strings.Contains(stderrOutput, "Nested invowk commands") {
		t.Error("stderr should explain consequences (nested invowk commands won't work)")
	}
}
