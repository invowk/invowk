// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.ContainerEngine != ContainerEnginePodman {
		t.Errorf("expected default container engine to be podman, got %s", cfg.ContainerEngine)
	}

	if cfg.DefaultRuntime != "native" {
		t.Errorf("expected default runtime to be native, got %s", cfg.DefaultRuntime)
	}

	if len(cfg.Includes) != 0 {
		t.Errorf("expected default includes to be empty, got %v", cfg.Includes)
	}

	if !cfg.Virtual.Utilities.Enabled {
		t.Error("expected virtual utilities to be enabled by default")
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

	if len(cfg.Container.AutoProvision.Includes) != 0 {
		t.Errorf("expected auto provisioning includes to be empty, got %v", cfg.Container.AutoProvision.Includes)
	}

	if !cfg.Container.AutoProvision.InheritIncludes {
		t.Error("expected auto provisioning inherit_includes to be true by default")
	}

	if cfg.Container.AutoProvision.CacheDir != "" {
		t.Errorf("expected auto provisioning cache dir to be empty, got %q", cfg.Container.AutoProvision.CacheDir)
	}
}

func TestLoadAndSave(t *testing.T) {
	t.Parallel()
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	includePathTwo := filepath.Join(tmpDir, "path", "two.invowkmod")
	includePathThree := filepath.Join(tmpDir, "path", "three.invowkmod")
	binaryPath := filepath.Join(tmpDir, "custom", "bin", "invowk")
	autoProvisionIncludePath := filepath.Join(tmpDir, "modules", "one.invowkmod")
	cacheDir := filepath.Join(tmpDir, "tmp", "invowk-cache")

	// Ensure config directory exists
	err := EnsureConfigDir(types.FilesystemPath(configDir))
	if err != nil {
		t.Fatalf("EnsureConfigDir() returned error: %v", err)
	}

	// Create a custom config
	cfg := &Config{
		ContainerEngine: ContainerEngineDocker,
		Includes: []IncludeEntry{
			{Path: ModuleIncludePath(includePathTwo), Alias: "two-alias"},
			{Path: ModuleIncludePath(includePathThree)},
		},
		DefaultRuntime: "container",
		Virtual:        VirtualConfig{Utilities: VirtualUtilitiesConfig{Enabled: false}},
		UI: UIConfig{
			ColorScheme: "dark",
			Verbose:     true,
			Interactive: true,
		},
		Container: ContainerConfig{
			AutoProvision: AutoProvisionConfig{
				Enabled:         false,
				BinaryPath:      BinaryFilePath(binaryPath),
				Includes:        []IncludeEntry{{Path: ModuleIncludePath(autoProvisionIncludePath)}},
				InheritIncludes: false,
				CacheDir:        CacheDirPath(cacheDir),
			},
		},
	}

	// Save the config
	err = Save(cfg, types.FilesystemPath(configDir))
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Reload from disk via loadWithOptions
	loaded, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigDirPath: types.FilesystemPath(configDir),
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

	if len(loaded.Includes) != 2 {
		t.Errorf("Includes length = %d, want 2", len(loaded.Includes))
	}

	if loaded.Virtual.Utilities.Enabled != false {
		t.Error("Virtual.Utilities.Enabled = true, want false")
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

	if loaded.Container.AutoProvision.BinaryPath != BinaryFilePath(binaryPath) {
		t.Errorf("AutoProvision.BinaryPath = %q, want %q", loaded.Container.AutoProvision.BinaryPath, binaryPath)
	}

	if len(loaded.Container.AutoProvision.Includes) != 1 || loaded.Container.AutoProvision.Includes[0].Path != ModuleIncludePath(autoProvisionIncludePath) {
		t.Errorf("AutoProvision.Includes = %v, want [{Path:%s}]", loaded.Container.AutoProvision.Includes, autoProvisionIncludePath)
	}

	if loaded.Container.AutoProvision.InheritIncludes != false {
		t.Error("AutoProvision.InheritIncludes = true, want false")
	}

	if loaded.Container.AutoProvision.CacheDir != CacheDirPath(cacheDir) {
		t.Errorf("AutoProvision.CacheDir = %q, want %q", loaded.Container.AutoProvision.CacheDir, cacheDir)
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "invalid default runtime",
			mutate: func(cfg *Config) {
				cfg.DefaultRuntime = RuntimeMode("invalid-runtime")
			},
		},
		{
			name: "invalid auto provision include",
			mutate: func(cfg *Config) {
				cfg.Container.AutoProvision.Includes = []IncludeEntry{{Path: ModuleIncludePath("relative.invowkmod")}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			configDir := filepath.Join(tmpDir, AppName)
			cfg := DefaultConfig()
			tt.mutate(cfg)

			err := Save(cfg, types.FilesystemPath(configDir))
			if err == nil {
				t.Fatal("Save() returned nil, want invalid config error")
			}
			if _, statErr := os.Stat(filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)); !os.IsNotExist(statErr) {
				t.Fatalf("Save() wrote config despite validation failure, stat err=%v", statErr)
			}
		})
	}
}

func TestLoad_ReturnsDefaultsWhenNoConfigFile(t *testing.T) {
	t.Parallel()
	// Use a temp directory with no config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	cfg, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigDirPath: types.FilesystemPath(configDir),
		BaseDir:       types.FilesystemPath(tmpDir),
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
	t.Parallel()
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	err := CreateDefaultConfig(types.FilesystemPath(configDir))
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
	err = CreateDefaultConfig(types.FilesystemPath(configDir))
	if err != nil {
		t.Fatalf("CreateDefaultConfig() returned error on second call: %v", err)
	}
}

func TestConfigOperationsRejectInvalidOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "ensure config dir",
			call: func() error { return EnsureConfigDir("   ") },
		},
		{
			name: "ensure commands dir",
			call: func() error { return EnsureCommandsDir("   ") },
		},
		{
			name: "create default config",
			call: func() error { return CreateDefaultConfig("   ") },
		},
		{
			name: "save",
			call: func() error { return Save(DefaultConfig(), "   ") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.call()
			if !errors.Is(err, types.ErrInvalidFilesystemPath) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, types.ErrInvalidFilesystemPath)
			}
		})
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	t.Parallel()
	// An empty config.cue should not error — it should produce defaults.
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to write empty config: %v", err)
	}

	cfg, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigDirPath: types.FilesystemPath(configDir),
		BaseDir:       types.FilesystemPath(tmpDir),
	})
	if err != nil {
		t.Fatalf("loadWithOptions() returned error for empty config: %v", err)
	}

	// Verify defaults are used when the config file is empty
	defaults := DefaultConfig()
	if cfg.ContainerEngine != defaults.ContainerEngine {
		t.Errorf("ContainerEngine = %s, want default %s", cfg.ContainerEngine, defaults.ContainerEngine)
	}
	if cfg.DefaultRuntime != defaults.DefaultRuntime {
		t.Errorf("DefaultRuntime = %s, want default %s", cfg.DefaultRuntime, defaults.DefaultRuntime)
	}
	if cfg.UI.ColorScheme != defaults.UI.ColorScheme {
		t.Errorf("UI.ColorScheme = %s, want default %s", cfg.UI.ColorScheme, defaults.UI.ColorScheme)
	}
}

func TestLoad_UnknownFieldsRejected(t *testing.T) {
	t.Parallel()
	// Config is a closed effective schema. Unknown fields fail validation
	// instead of being ignored as forward-compatible patch data.
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `container_engine: "docker"
some_future_field: "value"
`
	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigDirPath: types.FilesystemPath(configDir),
		BaseDir:       types.FilesystemPath(tmpDir),
	})
	if err == nil {
		t.Fatal("loadWithOptions() succeeded, want unknown field error")
	}
	if !errors.Is(err, ErrConfigLoadFailed) {
		t.Fatalf("loadWithOptions() error = %v, want ErrConfigLoadFailed", err)
	}
}

func TestLoad_MalformedCUE_PartiallyValid(t *testing.T) {
	t.Parallel()
	// Completely broken CUE syntax must return an error.
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte("{broken"), 0o644); err != nil {
		t.Fatalf("failed to write malformed config: %v", err)
	}

	_, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigDirPath: types.FilesystemPath(configDir),
		BaseDir:       types.FilesystemPath(tmpDir),
	})
	if err == nil {
		t.Fatal("expected loadWithOptions() to return error for malformed CUE syntax")
	}

	if err.Error() == "" {
		t.Error("expected non-empty error string for malformed CUE")
	}
}

func TestLoad_ActionableErrorFormat(t *testing.T) {
	t.Parallel()
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

	// loadWithOptions should fail with actionable error
	_, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigDirPath: types.FilesystemPath(configDir),
		BaseDir:       types.FilesystemPath(tmpDir),
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

func TestLoad_DuplicateIncludeAliasKeepsActionableSuggestions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, ConfigFileName+"."+ConfigFileExt)
	first := filepath.Join(tmpDir, "first.invowkmod")
	second := filepath.Join(tmpDir, "second.invowkmod")
	cfg := fmt.Sprintf(`includes: [
	{path: %q, alias: "same"},
	{path: %q, alias: "same"},
]`, first, second)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigFilePath: types.FilesystemPath(cfgPath),
		BaseDir:        types.FilesystemPath(tmpDir),
	})
	if err == nil {
		t.Fatal("expected duplicate include alias error")
	}
	if !errors.Is(err, ErrInvalidIncludeCollection) {
		t.Fatalf("errors.Is(err, ErrInvalidIncludeCollection) = false for %v", err)
	}
	var includeErr *InvalidIncludeCollectionError
	if !errors.As(err, &includeErr) {
		t.Fatalf("errors.As(err, *InvalidIncludeCollectionError) = false for %v", err)
	}
	if includeErr.Field != "includes" {
		t.Fatalf("include field = %q, want includes", includeErr.Field)
	}
}

func TestLoad_CustomPath_Valid(t *testing.T) {
	t.Parallel()
	// Create a temp directory with a valid config file
	tmpDir := t.TempDir()
	customConfigPath := filepath.Join(tmpDir, "custom-config.cue")

	// Write valid CUE content
	validConfig := `container_engine: "docker"
default_runtime: "virtual-sh"
`
	if err := os.WriteFile(customConfigPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	// Load using custom path via LoadOptions
	cfg, resolvedPath, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigFilePath: types.FilesystemPath(customConfigPath),
		BaseDir:        types.FilesystemPath(tmpDir),
	})
	if err != nil {
		t.Fatalf("loadWithOptions() returned error: %v", err)
	}

	// Verify the custom config was loaded
	if cfg.ContainerEngine != ContainerEngineDocker {
		t.Errorf("ContainerEngine = %s, want docker", cfg.ContainerEngine)
	}
	if cfg.DefaultRuntime != RuntimeVirtualSh {
		t.Errorf("DefaultRuntime = %s, want virtual-sh", cfg.DefaultRuntime)
	}

	// Verify resolvedPath matches
	if resolvedPath != types.FilesystemPath(customConfigPath) {
		t.Errorf("resolvedPath = %s, want %s", resolvedPath, customConfigPath)
	}
}

func TestLoad_CustomPath_NotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	// Set a non-existent path
	nonExistentPath := "/this/path/does/not/exist/config.cue"

	// loadWithOptions should fail with an actionable error
	_, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigFilePath: types.FilesystemPath(nonExistentPath),
	})
	if err == nil {
		t.Fatal("expected loadWithOptions() to return error for non-existent config file")
	}

	// Verify error contains actionable context
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}

	var notFound *FileNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected FileNotFoundError, got %T", err)
	}
	if !errors.Is(err, ErrConfigFileNotFound) {
		t.Fatalf("expected ErrConfigFileNotFound, got %v", err)
	}
}

func TestNewProvider_Load(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	validConfig := `container_engine: "docker"
default_runtime: "virtual-sh"
`
	cfgPath := filepath.Join(configDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	provider := NewProvider()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "loads config from directory", run: func(t *testing.T) {
			t.Helper()

			cfg, err := provider.Load(t.Context(), LoadOptions{
				ConfigDirPath: types.FilesystemPath(configDir),
				BaseDir:       types.FilesystemPath(tmpDir),
			})
			if err != nil {
				t.Fatalf("Provider.Load() returned error: %v", err)
			}

			if cfg.ContainerEngine != ContainerEngineDocker {
				t.Errorf("ContainerEngine = %s, want docker", cfg.ContainerEngine)
			}
			if cfg.DefaultRuntime != RuntimeVirtualSh {
				t.Errorf("DefaultRuntime = %s, want virtual-sh", cfg.DefaultRuntime)
			}
		}},

		{name: "loads config from explicit file path", run: func(t *testing.T) {
			t.Helper()

			cfg, err := provider.Load(t.Context(), LoadOptions{
				ConfigFilePath: types.FilesystemPath(cfgPath),
				BaseDir:        types.FilesystemPath(tmpDir),
			})
			if err != nil {
				t.Fatalf("Provider.Load() returned error: %v", err)
			}

			if cfg.ContainerEngine != ContainerEngineDocker {
				t.Errorf("ContainerEngine = %s, want docker", cfg.ContainerEngine)
			}
		}},

		{name: "reports config directory source", run: func(t *testing.T) {
			t.Helper()

			result, err := provider.LoadWithSource(t.Context(), LoadOptions{
				ConfigDirPath: types.FilesystemPath(configDir),
				BaseDir:       types.FilesystemPath(tmpDir),
			})
			if err != nil {
				t.Fatalf("Provider.LoadWithSource() returned error: %v", err)
			}
			if result.Config.ContainerEngine != ContainerEngineDocker {
				t.Errorf("ContainerEngine = %s, want docker", result.Config.ContainerEngine)
			}
			if result.SourcePath != types.FilesystemPath(cfgPath) {
				t.Errorf("SourcePath = %s, want %s", result.SourcePath, cfgPath)
			}
		}},

		{name: "reports local fallback source", run: func(t *testing.T) {
			t.Helper()

			baseDir := t.TempDir()
			localPath := filepath.Join(baseDir, ConfigFileName+"."+ConfigFileExt)
			if err := os.WriteFile(localPath, []byte(validConfig), 0o644); err != nil {
				t.Fatalf("failed to write local config: %v", err)
			}

			result, err := provider.LoadWithSource(t.Context(), LoadOptions{
				ConfigDirPath: types.FilesystemPath(t.TempDir()),
				BaseDir:       types.FilesystemPath(baseDir),
			})
			if err != nil {
				t.Fatalf("Provider.LoadWithSource() returned error: %v", err)
			}
			if result.SourcePath != types.FilesystemPath(localPath) {
				t.Errorf("SourcePath = %s, want %s", result.SourcePath, localPath)
			}
		}},

		{name: "returns defaults when no config exists", run: func(t *testing.T) {
			t.Helper()

			emptyDir := t.TempDir()
			cfg, err := provider.Load(t.Context(), LoadOptions{
				ConfigDirPath: types.FilesystemPath(emptyDir),
				BaseDir:       types.FilesystemPath(emptyDir),
			})
			if err != nil {
				t.Fatalf("Provider.Load() returned error: %v", err)
			}

			defaults := DefaultConfig()
			if cfg.ContainerEngine != defaults.ContainerEngine {
				t.Errorf("ContainerEngine = %s, want %s", cfg.ContainerEngine, defaults.ContainerEngine)
			}
		}},

		{name: "returns error for non-existent explicit path", run: func(t *testing.T) {
			t.Helper()

			_, err := provider.Load(t.Context(), LoadOptions{
				ConfigFilePath: "/this/path/does/not/exist.cue",
			})

			if err == nil {
				t.Fatal("expected Provider.Load() to return error for non-existent path")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestLoad_CustomPath_InvalidCUE_ReturnsError(t *testing.T) {
	t.Parallel()
	// Create a temp directory with an invalid config file
	tmpDir := t.TempDir()
	customConfigPath := filepath.Join(tmpDir, "invalid-config.cue")

	// Write invalid CUE content
	invalidConfig := `this is not valid CUE syntax {{{{`
	if err := os.WriteFile(customConfigPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// loadWithOptions should fail with an actionable error
	_, _, err := loadWithOptions(t.Context(), LoadOptions{
		ConfigFilePath: types.FilesystemPath(customConfigPath),
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
