// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"invowk-cli/internal/issue"
	"invowk-cli/internal/testutil"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ContainerEngine != ContainerEnginePodman {
		t.Errorf("expected default container engine to be podman, got %s", cfg.ContainerEngine)
	}

	if cfg.DefaultRuntime != "native" {
		t.Errorf("expected default runtime to be native, got %s", cfg.DefaultRuntime)
	}

	if len(cfg.SearchPaths) != 0 {
		t.Errorf("expected default search paths to be empty, got %v", cfg.SearchPaths)
	}

	if !cfg.VirtualShell.EnableUrootUtils {
		t.Error("expected EnableUrootUtils to be true by default")
	}

	if cfg.UI.ColorScheme != "auto" {
		t.Errorf("expected default color scheme to be auto, got %s", cfg.UI.ColorScheme)
	}

	if cfg.UI.Verbose {
		t.Error("expected default verbose to be false")
	}

	if cfg.UI.Interactive {
		t.Error("expected default interactive to be false")
	}

	if !cfg.Container.AutoProvision.Enabled {
		t.Error("expected auto provisioning to be enabled by default")
	}

	if cfg.Container.AutoProvision.BinaryPath != "" {
		t.Errorf("expected auto provisioning binary path to be empty, got %q", cfg.Container.AutoProvision.BinaryPath)
	}

	if len(cfg.Container.AutoProvision.ModulesPaths) != 0 {
		t.Errorf("expected auto provisioning modules paths to be empty, got %v", cfg.Container.AutoProvision.ModulesPaths)
	}

	if cfg.Container.AutoProvision.CacheDir != "" {
		t.Errorf("expected auto provisioning cache dir to be empty, got %q", cfg.Container.AutoProvision.CacheDir)
	}
}

func TestConfigDir(t *testing.T) {
	// Reset environment for consistent testing
	originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDGConfigHome != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome) // Test cleanup; error non-critical
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME") // Test cleanup; error non-critical
		}
	}()

	// Test with XDG_CONFIG_HOME set (on Linux)
	if runtime.GOOS == "linux" {
		testXDGPath := "/tmp/test-xdg-config"
		restoreXDG := testutil.MustSetenv(t, "XDG_CONFIG_HOME", testXDGPath)

		dir, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() returned error: %v", err)
		}

		expected := filepath.Join(testXDGPath, AppName)
		if dir != expected {
			t.Errorf("ConfigDir() = %s, want %s", dir, expected)
		}

		// Test with XDG_CONFIG_HOME unset
		restoreXDG()
		testutil.MustUnsetenv(t, "XDG_CONFIG_HOME")
		dir, err = ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() returned error: %v", err)
		}

		// Should use ~/.config/invowk
		home, _ := os.UserHomeDir()
		expected = filepath.Join(home, ".config", AppName)
		if dir != expected {
			t.Errorf("ConfigDir() = %s, want %s", dir, expected)
		}
	}
}

func TestCommandsDir(t *testing.T) {
	dir, err := CommandsDir()
	if err != nil {
		t.Fatalf("CommandsDir() returned error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".invowk", "cmds")
	if dir != expected {
		t.Errorf("CommandsDir() = %s, want %s", dir, expected)
	}
}

func TestReset(t *testing.T) {
	// Set the override
	SetConfigDirOverride("/some/override")

	// Reset should clear it
	Reset()

	if configDirOverride != "" {
		t.Error("expected configDirOverride to be empty after Reset()")
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	// Use direct override instead of env vars (more reliable across platforms)
	SetConfigDirOverride(configDir)
	defer Reset()

	err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() returned error: %v", err)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("EnsureConfigDir() did not create directory %s", configDir)
	}
}

func TestEnsureCommandsDir(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	cleanup := testutil.SetHomeDir(t, tmpDir)
	defer cleanup()

	err := EnsureCommandsDir()
	if err != nil {
		t.Fatalf("EnsureCommandsDir() returned error: %v", err)
	}

	expectedDir := filepath.Join(tmpDir, ".invowk", "cmds")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("EnsureCommandsDir() did not create directory %s", expectedDir)
	}
}

func TestLoadAndSave(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	SetConfigDirOverride(configDir)
	defer Reset()

	// Ensure config directory exists
	err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() returned error: %v", err)
	}

	// Create a custom config
	cfg := &Config{
		ContainerEngine: ContainerEngineDocker,
		SearchPaths:     []string{"/path/one", "/path/two"},
		DefaultRuntime:  "container",
		VirtualShell: VirtualShellConfig{
			EnableUrootUtils: false,
		},
		UI: UIConfig{
			ColorScheme: "dark",
			Verbose:     true,
			Interactive: true,
		},
		Container: ContainerConfig{
			AutoProvision: AutoProvisionConfig{
				Enabled:      false,
				BinaryPath:   "/custom/bin/invowk",
				ModulesPaths: []string{"/modules/one"},
				CacheDir:     "/tmp/invowk-cache",
			},
		},
	}

	// Save the config
	err = Save(cfg)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Reload from disk via loadWithOptions
	loaded, _, err := loadWithOptions(context.Background(), LoadOptions{
		ConfigDirPath: configDir,
	})
	if err != nil {
		t.Fatalf("loadWithOptions() returned error: %v", err)
	}

	// Verify loaded config matches what we saved
	if loaded.ContainerEngine != ContainerEngineDocker {
		t.Errorf("ContainerEngine = %s, want docker", loaded.ContainerEngine)
	}

	if loaded.DefaultRuntime != "container" {
		t.Errorf("DefaultRuntime = %s, want container", loaded.DefaultRuntime)
	}

	if len(loaded.SearchPaths) != 2 {
		t.Errorf("SearchPaths length = %d, want 2", len(loaded.SearchPaths))
	}

	if loaded.VirtualShell.EnableUrootUtils != false {
		t.Error("EnableUrootUtils = true, want false")
	}

	if loaded.UI.ColorScheme != "dark" {
		t.Errorf("ColorScheme = %s, want dark", loaded.UI.ColorScheme)
	}

	if loaded.UI.Verbose != true {
		t.Error("Verbose = false, want true")
	}

	if loaded.UI.Interactive != true {
		t.Error("Interactive = false, want true")
	}

	if loaded.Container.AutoProvision.Enabled != false {
		t.Error("AutoProvision.Enabled = true, want false")
	}

	if loaded.Container.AutoProvision.BinaryPath != "/custom/bin/invowk" {
		t.Errorf("AutoProvision.BinaryPath = %q, want /custom/bin/invowk", loaded.Container.AutoProvision.BinaryPath)
	}

	if len(loaded.Container.AutoProvision.ModulesPaths) != 1 || loaded.Container.AutoProvision.ModulesPaths[0] != "/modules/one" {
		t.Errorf("AutoProvision.ModulesPaths = %v, want [/modules/one]", loaded.Container.AutoProvision.ModulesPaths)
	}

	if loaded.Container.AutoProvision.CacheDir != "/tmp/invowk-cache" {
		t.Errorf("AutoProvision.CacheDir = %q, want /tmp/invowk-cache", loaded.Container.AutoProvision.CacheDir)
	}
}

func TestLoad_ReturnsDefaultsWhenNoConfigFile(t *testing.T) {
	// Use a temp directory with no config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	SetConfigDirOverride(configDir)
	defer Reset()

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cfg, _, err := loadWithOptions(context.Background(), LoadOptions{
		ConfigDirPath: configDir,
	})
	if err != nil {
		t.Fatalf("loadWithOptions() returned error: %v", err)
	}

	// Should return default values
	defaults := DefaultConfig()
	if cfg.ContainerEngine != defaults.ContainerEngine {
		t.Errorf("ContainerEngine = %s, want %s", cfg.ContainerEngine, defaults.ContainerEngine)
	}

	if cfg.DefaultRuntime != defaults.DefaultRuntime {
		t.Errorf("DefaultRuntime = %s, want %s", cfg.DefaultRuntime, defaults.DefaultRuntime)
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	// Use direct override instead of env vars (more reliable across platforms)
	SetConfigDirOverride(configDir)
	defer Reset()

	err := CreateDefaultConfig()
	if err != nil {
		t.Fatalf("CreateDefaultConfig() returned error: %v", err)
	}

	// Check that file was created
	expectedPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Errorf("CreateDefaultConfig() did not create file at %s", expectedPath)
	}

	// Read the file and verify it has content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	if len(content) == 0 {
		t.Error("config file is empty")
	}

	// Calling again should not error (file already exists)
	err = CreateDefaultConfig()
	if err != nil {
		t.Fatalf("CreateDefaultConfig() returned error on second call: %v", err)
	}
}

func TestContainerEngineConstants(t *testing.T) {
	if ContainerEnginePodman != "podman" {
		t.Errorf("ContainerEnginePodman = %s, want podman", ContainerEnginePodman)
	}

	if ContainerEngineDocker != "docker" {
		t.Errorf("ContainerEngineDocker = %s, want docker", ContainerEngineDocker)
	}
}

func TestConstants(t *testing.T) {
	if AppName != "invowk" {
		t.Errorf("AppName = %s, want invowk", AppName)
	}

	if ConfigFileName != "config" {
		t.Errorf("ConfigFileName = %s, want config", ConfigFileName)
	}

	if ConfigFileExt != "cue" {
		t.Errorf("ConfigFileExt = %s, want cue", ConfigFileExt)
	}
}

func TestLoad_ActionableErrorFormat(t *testing.T) {
	// Create a temp directory with an invalid config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Write invalid CUE content - wrong type for container_engine
	invalidConfig := `container_engine: 123`
	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// loadWithOptions should fail with actionable error
	_, _, err := loadWithOptions(context.Background(), LoadOptions{
		ConfigDirPath: configDir,
	})
	if err == nil {
		t.Fatal("expected loadWithOptions() to return error for invalid config")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
}

func TestLoad_CustomPath_Valid(t *testing.T) {
	// Create a temp directory with a valid config file
	tmpDir := t.TempDir()
	customConfigPath := filepath.Join(tmpDir, "custom-config.cue")

	// Write valid CUE content
	validConfig := `container_engine: "docker"
default_runtime: "virtual"
`
	if err := os.WriteFile(customConfigPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Load using custom path via LoadOptions
	cfg, resolvedPath, err := loadWithOptions(context.Background(), LoadOptions{
		ConfigFilePath: customConfigPath,
	})
	if err != nil {
		t.Fatalf("loadWithOptions() returned error: %v", err)
	}

	// Verify the custom config was loaded
	if cfg.ContainerEngine != ContainerEngineDocker {
		t.Errorf("ContainerEngine = %s, want docker", cfg.ContainerEngine)
	}
	if cfg.DefaultRuntime != "virtual" {
		t.Errorf("DefaultRuntime = %s, want virtual", cfg.DefaultRuntime)
	}

	// Verify resolvedPath matches
	if resolvedPath != customConfigPath {
		t.Errorf("resolvedPath = %s, want %s", resolvedPath, customConfigPath)
	}
}

func TestLoad_CustomPath_NotFound_ReturnsError(t *testing.T) {
	// Set a non-existent path
	nonExistentPath := "/this/path/does/not/exist/config.cue"

	// loadWithOptions should fail with an actionable error
	_, _, err := loadWithOptions(context.Background(), LoadOptions{
		ConfigFilePath: nonExistentPath,
	})
	if err == nil {
		t.Fatal("expected loadWithOptions() to return error for non-existent config file")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}

	// Verify suggestions are present via ActionableError type
	var ae *issue.ActionableError
	if !errors.As(err, &ae) {
		t.Fatal("expected error to be *issue.ActionableError")
	}
	if len(ae.Suggestions) == 0 {
		t.Error("expected ActionableError to have suggestions")
	}
}

func TestLoad_CustomPath_InvalidCUE_ReturnsError(t *testing.T) {
	// Create a temp directory with an invalid config file
	tmpDir := t.TempDir()
	customConfigPath := filepath.Join(tmpDir, "invalid-config.cue")

	// Write invalid CUE content
	invalidConfig := `this is not valid CUE syntax {{{{`
	if err := os.WriteFile(customConfigPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// loadWithOptions should fail with an actionable error
	_, _, err := loadWithOptions(context.Background(), LoadOptions{
		ConfigFilePath: customConfigPath,
	})
	if err == nil {
		t.Fatal("expected loadWithOptions() to return error for invalid CUE config file")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
}
