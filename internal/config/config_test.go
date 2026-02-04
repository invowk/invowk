// SPDX-License-Identifier: MPL-2.0

// Package config handles application configuration using Viper.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	// Load config first
	cfg := DefaultConfig()
	cfg.DefaultRuntime = "virtual"
	globalConfig = cfg
	configPath = "/some/path"

	// Reset
	Reset()

	if globalConfig != nil {
		t.Error("expected globalConfig to be nil after Reset()")
	}

	if configPath != "" {
		t.Error("expected configPath to be empty after Reset()")
	}
}

func TestGet_ReturnsDefaultOnNoConfig(t *testing.T) {
	// Reset to ensure no config is loaded
	Reset()

	// Create a temp directory to avoid loading any real config
	tmpDir := t.TempDir()
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cfg := Get()

	if cfg == nil {
		t.Fatal("Get() returned nil")
	}

	// Should return default config values
	if cfg.ContainerEngine != ContainerEnginePodman {
		t.Errorf("expected default container engine, got %s", cfg.ContainerEngine)
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
	// Reset global state
	Reset()

	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	// Use direct override instead of env vars (more reliable across platforms)
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

	// Clear cached config to force reload from disk (but preserve the override)
	ResetCache()

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
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
	// Reset global state
	Reset()

	// Use a temp directory with no config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	// Use direct override instead of env vars (more reliable across platforms)
	SetConfigDirOverride(configDir)
	defer Reset()

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
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

func TestLoad_ReturnsCachedConfig(t *testing.T) {
	// Reset global state
	Reset()

	// Set up a cached config
	cachedCfg := &Config{
		DefaultRuntime: "cached-runtime",
	}
	globalConfig = cachedCfg

	// Load should return the cached config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DefaultRuntime != "cached-runtime" {
		t.Errorf("expected cached config, got DefaultRuntime = %s", cfg.DefaultRuntime)
	}

	// Reset for other tests
	Reset()
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

func TestConfigFilePath(t *testing.T) {
	// Reset
	Reset()

	// Initially should be empty
	if path := ConfigFilePath(); path != "" {
		t.Errorf("ConfigFilePath() = %s, want empty string", path)
	}

	// Set configPath directly
	configPath = "/some/test/path"

	if path := ConfigFilePath(); path != "/some/test/path" {
		t.Errorf("ConfigFilePath() = %s, want /some/test/path", path)
	}

	// Reset for cleanup
	Reset()
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

// T097: Test config error visibility
func TestGet_StoresLoadErrorForLaterRetrieval(t *testing.T) {
	// Reset global state
	Reset()

	// Create a temp directory with an invalid config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Write invalid CUE content
	invalidConfig := `this is not valid CUE syntax`
	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// Use direct override
	SetConfigDirOverride(configDir)
	defer Reset()

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Get() should return defaults but store the error
	cfg := Get()

	// Should return default config
	if cfg.ContainerEngine != ContainerEnginePodman {
		t.Errorf("expected default container engine, got %s", cfg.ContainerEngine)
	}

	// Error should be stored and retrievable
	err := LastLoadError()
	if err == nil {
		t.Fatal("expected LastLoadError() to return error for invalid config")
	}

	// Error should contain actionable context
	errStr := err.Error()
	if !strings.Contains(errStr, "load configuration") {
		t.Errorf("error should contain 'load configuration', got: %s", errStr)
	}
}

func TestLastLoadError_NilWhenSuccessful(t *testing.T) {
	// Reset global state
	Reset()

	// Create a temp directory with a valid config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Write valid CUE content
	validConfig := `container_engine: "docker"`
	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("failed to write valid config: %v", err)
	}

	// Use direct override
	SetConfigDirOverride(configDir)
	defer Reset()

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Load should succeed
	cfg := Get()

	// Should load the config correctly
	if cfg.ContainerEngine != ContainerEngineDocker {
		t.Errorf("expected docker, got %s", cfg.ContainerEngine)
	}

	// No error should be stored
	if err := LastLoadError(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestLoad_ActionableErrorFormat(t *testing.T) {
	// Reset global state
	Reset()

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

	// Use direct override
	SetConfigDirOverride(configDir)
	defer Reset()

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Load should fail with actionable error
	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to return error for invalid config")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if !strings.Contains(errStr, "load configuration") {
		t.Errorf("error should contain operation, got: %s", errStr)
	}
	if !strings.Contains(errStr, cfgPath) {
		t.Errorf("error should contain resource path, got: %s", errStr)
	}
}

func TestSetConfigFilePathOverride_SetsVariable(t *testing.T) {
	// Reset first
	Reset()
	defer Reset()

	// Set override
	SetConfigFilePathOverride("/some/custom/path.cue")

	// Verify it's set (we can verify by checking that Load() uses it)
	// Since there's no direct getter, we verify the behavior
	if configFilePathOverride != "/some/custom/path.cue" {
		t.Errorf("configFilePathOverride = %q, want /some/custom/path.cue", configFilePathOverride)
	}
}

func TestSetConfigFilePathOverride_ClearsCache(t *testing.T) {
	// Reset first
	Reset()
	defer Reset()

	// Set up a cached config
	globalConfig = &Config{DefaultRuntime: "cached"}
	configPath = "/old/path"

	// Set new override - should clear cache
	SetConfigFilePathOverride("/new/path.cue")

	// Verify cache was cleared
	if globalConfig != nil {
		t.Error("expected globalConfig to be nil after SetConfigFilePathOverride")
	}
	if configPath != "" {
		t.Error("expected configPath to be empty after SetConfigFilePathOverride")
	}
}

func TestLoad_CustomPath_Valid(t *testing.T) {
	// Reset global state
	Reset()
	defer Reset()

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

	// Set the custom path override
	SetConfigFilePathOverride(customConfigPath)

	// Change to temp dir to avoid loading config from current directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Load should use the custom path
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Verify the custom config was loaded
	if cfg.ContainerEngine != ContainerEngineDocker {
		t.Errorf("ContainerEngine = %s, want docker", cfg.ContainerEngine)
	}
	if cfg.DefaultRuntime != "virtual" {
		t.Errorf("DefaultRuntime = %s, want virtual", cfg.DefaultRuntime)
	}

	// Verify configPath was set to the custom path
	if ConfigFilePath() != customConfigPath {
		t.Errorf("ConfigFilePath() = %s, want %s", ConfigFilePath(), customConfigPath)
	}
}

func TestLoad_CustomPath_NotFound_ReturnsError(t *testing.T) {
	// Reset global state
	Reset()
	defer Reset()

	// Set a non-existent path
	nonExistentPath := "/this/path/does/not/exist/config.cue"
	SetConfigFilePathOverride(nonExistentPath)

	// Load should fail with an actionable error
	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to return error for non-existent config file")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if !strings.Contains(errStr, "load configuration") {
		t.Errorf("error should contain 'load configuration', got: %s", errStr)
	}
	if !strings.Contains(errStr, nonExistentPath) {
		t.Errorf("error should contain the path, got: %s", errStr)
	}
	if !strings.Contains(errStr, "config file not found") {
		t.Errorf("error should contain 'config file not found', got: %s", errStr)
	}

	// Verify suggestions are present via ActionableError type
	var ae *issue.ActionableError
	if !errors.As(err, &ae) {
		t.Fatal("expected error to be *issue.ActionableError")
	}
	if len(ae.Suggestions) == 0 {
		t.Error("expected ActionableError to have suggestions")
	}
	foundSuggestion := false
	for _, s := range ae.Suggestions {
		if strings.Contains(s, "Verify the file path is correct") {
			foundSuggestion = true
			break
		}
	}
	if !foundSuggestion {
		t.Errorf("expected suggestion 'Verify the file path is correct', got: %v", ae.Suggestions)
	}
}

func TestLoad_CustomPath_InvalidCUE_ReturnsError(t *testing.T) {
	// Reset global state
	Reset()
	defer Reset()

	// Create a temp directory with an invalid config file
	tmpDir := t.TempDir()
	customConfigPath := filepath.Join(tmpDir, "invalid-config.cue")

	// Write invalid CUE content
	invalidConfig := `this is not valid CUE syntax {{{{`
	if err := os.WriteFile(customConfigPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// Set the custom path override
	SetConfigFilePathOverride(customConfigPath)

	// Load should fail with an actionable error
	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to return error for invalid CUE config file")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if !strings.Contains(errStr, "load configuration") {
		t.Errorf("error should contain 'load configuration', got: %s", errStr)
	}
	if !strings.Contains(errStr, customConfigPath) {
		t.Errorf("error should contain the path, got: %s", errStr)
	}
}

func TestReset_ClearsCustomPath(t *testing.T) {
	// Set up some state
	configFilePathOverride = "/custom/path.cue"
	globalConfig = &Config{DefaultRuntime: "test"}
	configPath = "/some/path"
	configDirOverride = "/dir/override"
	errLastLoad = fmt.Errorf("test error")

	// Reset should clear everything
	Reset()

	if configFilePathOverride != "" {
		t.Errorf("configFilePathOverride = %q, want empty string", configFilePathOverride)
	}
	if globalConfig != nil {
		t.Error("globalConfig should be nil after Reset")
	}
	if configPath != "" {
		t.Error("configPath should be empty after Reset")
	}
	if configDirOverride != "" {
		t.Error("configDirOverride should be empty after Reset")
	}
	if errLastLoad != nil {
		t.Error("errLastLoad should be nil after Reset")
	}
}
