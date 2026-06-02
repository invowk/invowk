// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestConfigMutationLoadErrorContracts(t *testing.T) {
	t.Parallel()

	t.Run("missing explicit path preserves requested path", func(t *testing.T) {
		t.Parallel()

		missingPath := types.FilesystemPath(filepath.Join(t.TempDir(), "missing-config.cue"))
		_, _, err := loadWithOptions(t.Context(), LoadOptions{ConfigFilePath: missingPath})
		if err == nil {
			t.Fatal("loadWithOptions() error = nil, want missing explicit config error")
		}
		var notFound *FileNotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("loadWithOptions() error type = %T, want *FileNotFoundError", err)
		}
		if notFound.Path != missingPath {
			t.Fatalf("FileNotFoundError.Path = %q, want %q", notFound.Path, missingPath)
		}
		if !strings.Contains(err.Error(), string(missingPath)) {
			t.Fatalf("FileNotFoundError.Error() = %q, want requested path", err.Error())
		}
	})

	t.Run("cue load error preserves source path and cause", func(t *testing.T) {
		t.Parallel()

		cfgPath := writeConfigMutationFile(t, t.TempDir(), "invalid.cue", `this is not valid CUE syntax {{{`)
		_, _, err := loadWithOptions(t.Context(), LoadOptions{ConfigFilePath: types.FilesystemPath(cfgPath)})
		if err == nil {
			t.Fatal("loadWithOptions() error = nil, want invalid CUE load error")
		}
		if !errors.Is(err, ErrConfigLoadFailed) {
			t.Fatalf("loadWithOptions() error = %v, want ErrConfigLoadFailed", err)
		}
		var loadErr *LoadError
		if !errors.As(err, &loadErr) {
			t.Fatalf("loadWithOptions() error type = %T, want *LoadError", err)
		}
		if loadErr.Path != types.FilesystemPath(cfgPath) {
			t.Fatalf("LoadError.Path = %q, want %q", loadErr.Path, cfgPath)
		}
		if loadErr.Err == nil {
			t.Fatal("LoadError.Err = nil, want underlying CUE error")
		}
	})

	t.Run("decode errors include file name", func(t *testing.T) {
		t.Parallel()

		cfgPath := writeConfigMutationFile(t, t.TempDir(), "named-invalid.cue", `this is not valid CUE syntax {{{`)
		_, err := decodeCUEConfigFile(types.FilesystemPath(cfgPath))
		if err == nil {
			t.Fatal("decodeCUEConfigFile() error = nil, want invalid CUE error")
		}
		if !strings.Contains(err.Error(), cfgPath) {
			t.Fatalf("decodeCUEConfigFile() error = %q, want file path %q", err.Error(), cfgPath)
		}
	})
}

func TestConfigMutationLoadSourcePrecedence(t *testing.T) {
	t.Parallel()

	t.Run("local fallback loads config contents", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		cfgPath := writeConfigMutationFile(t, baseDir, ConfigFileName+"."+ConfigFileExt, `
container_engine: "docker"
default_runtime: "virtual-sh"
`)
		cfg, resolvedPath, err := loadWithOptions(t.Context(), LoadOptions{
			ConfigDirPath: types.FilesystemPath(t.TempDir()),
			BaseDir:       types.FilesystemPath(baseDir),
		})
		if err != nil {
			t.Fatalf("loadWithOptions() error = %v, want nil", err)
		}
		if resolvedPath != types.FilesystemPath(cfgPath) {
			t.Fatalf("resolvedPath = %q, want %q", resolvedPath, cfgPath)
		}
		if cfg.ContainerEngine != ContainerEngineDocker || cfg.DefaultRuntime != RuntimeVirtualSh {
			t.Fatalf("loaded config = %+v, want local fallback values", cfg)
		}
	})

	t.Run("local fallback invalid CUE keeps local path", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		cfgPath := writeConfigMutationFile(t, baseDir, ConfigFileName+"."+ConfigFileExt, `this is not valid CUE syntax {{{`)
		_, _, err := loadWithOptions(t.Context(), LoadOptions{
			ConfigDirPath: types.FilesystemPath(t.TempDir()),
			BaseDir:       types.FilesystemPath(baseDir),
		})
		if err == nil {
			t.Fatal("loadWithOptions() error = nil, want local fallback CUE error")
		}
		var loadErr *LoadError
		if !errors.As(err, &loadErr) {
			t.Fatalf("loadWithOptions() error type = %T, want *LoadError", err)
		}
		if loadErr.Path != types.FilesystemPath(cfgPath) {
			t.Fatalf("LoadError.Path = %q, want %q", loadErr.Path, cfgPath)
		}
	})
}

func TestConfigMutationIncludeValidationContracts(t *testing.T) {
	t.Parallel()

	t.Run("load wraps include validation in root config error", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		firstPath := filepath.Join(tmpDir, "first.invowkmod")
		secondPath := filepath.Join(tmpDir, "second.invowkmod")
		cfgPath := writeConfigMutationFile(t, tmpDir, ConfigFileName+"."+ConfigFileExt, fmt.Sprintf(`
includes: [
	{path: %q, alias: "same"},
	{path: %q, alias: "same"},
]
`, firstPath, secondPath))

		_, _, err := loadWithOptions(t.Context(), LoadOptions{ConfigFilePath: types.FilesystemPath(cfgPath)})
		if err == nil {
			t.Fatal("loadWithOptions() error = nil, want duplicate include alias error")
		}
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("loadWithOptions() error = %v, want ErrInvalidConfig wrapper", err)
		}
		if !errors.Is(err, ErrInvalidIncludeCollection) {
			t.Fatalf("loadWithOptions() error = %v, want ErrInvalidIncludeCollection", err)
		}
		var configErr *InvalidConfigError
		if !errors.As(err, &configErr) {
			t.Fatalf("loadWithOptions() error type = %T, want *InvalidConfigError", err)
		}
		if len(configErr.FieldErrors) != 1 {
			t.Fatalf("InvalidConfigError.FieldErrors length = %d, want 1", len(configErr.FieldErrors))
		}
	})

	t.Run("invalid entry reports field index and wraps entry cause", func(t *testing.T) {
		t.Parallel()

		err := validateIncludes(IncludeCollectionRoot, []IncludeEntry{{Path: ""}})
		if err == nil {
			t.Fatal("validateIncludes() error = nil, want invalid include entry error")
		}
		if !errors.Is(err, ErrInvalidIncludeEntry) {
			t.Fatalf("validateIncludes() error = %v, want ErrInvalidIncludeEntry", err)
		}
		var includeErr *InvalidIncludeCollectionError
		if !errors.As(err, &includeErr) {
			t.Fatalf("validateIncludes() error type = %T, want *InvalidIncludeCollectionError", err)
		}
		if includeErr.Field != IncludeCollectionRoot {
			t.Fatalf("InvalidIncludeCollectionError.Field = %q, want %q", includeErr.Field, IncludeCollectionRoot)
		}
		if includeErr.Cause == nil {
			t.Fatal("InvalidIncludeCollectionError.Cause = nil, want entry validation cause")
		}
		if !strings.Contains(includeErr.Cause.Error(), "includes[0]") {
			t.Fatalf("include cause = %q, want indexed field label", includeErr.Cause.Error())
		}
	})

	t.Run("duplicate path is rejected before alias-disambiguated short-name collision", func(t *testing.T) {
		t.Parallel()

		duplicatePath := filepath.Join(t.TempDir(), "shared.invowkmod")
		err := validateIncludes(IncludeCollectionRoot, []IncludeEntry{
			{Path: ModuleIncludePath(duplicatePath), Alias: "one"},
			{Path: ModuleIncludePath(duplicatePath), Alias: "two"},
		})
		if err == nil {
			t.Fatal("validateIncludes() error = nil, want duplicate path error")
		}
		if !strings.Contains(err.Error(), "duplicate path") || !strings.Contains(err.Error(), "same as includes[0]") {
			t.Fatalf("validateIncludes() error = %q, want duplicate path diagnostic", err.Error())
		}
	})

	t.Run("short-name collision reports exact other-entry count", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		err := validateIncludes(IncludeCollectionRoot, []IncludeEntry{
			{Path: ModuleIncludePath(filepath.Join(tmpDir, "a", "same.invowkmod"))},
			{Path: ModuleIncludePath(filepath.Join(tmpDir, "b", "same.invowkmod"))},
			{Path: ModuleIncludePath(filepath.Join(tmpDir, "c", "same.invowkmod"))},
		})
		if err == nil {
			t.Fatal("validateIncludes() error = nil, want short-name collision error")
		}
		if !strings.Contains(err.Error(), "with 2 other entry(ies)") {
			t.Fatalf("validateIncludes() error = %q, want exact collision count", err.Error())
		}
	})
}

func TestConfigMutationFileAndCreateContracts(t *testing.T) {
	t.Parallel()

	t.Run("fileExists distinguishes missing files directories and regular files", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		missingPath := filepath.Join(tmpDir, "missing.cue")
		filePath := writeConfigMutationFile(t, tmpDir, "config.cue", "{}")

		if fileExists(missingPath) {
			t.Fatalf("fileExists(%q) = true, want false for missing file", missingPath)
		}
		if fileExists(tmpDir) {
			t.Fatalf("fileExists(%q) = true, want false for directory", tmpDir)
		}
		if !fileExists(filePath) {
			t.Fatalf("fileExists(%q) = false, want true for regular file", filePath)
		}
	})

	t.Run("fileExists treats stat errors as absent files", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("chmod-based stat denial is Unix-specific")
		}

		parent := filepath.Join(t.TempDir(), "protected")
		if err := os.Mkdir(parent, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
		if err := os.Chmod(parent, 0); err != nil {
			t.Fatalf("Chmod() error = %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(parent, 0o755); err != nil {
				t.Errorf("restore protected directory permissions: %v", err)
			}
		})

		deniedPath := filepath.Join(parent, "config.cue")
		if fileExists(deniedPath) {
			t.Fatalf("fileExists(%q) = true, want false for stat error", deniedPath)
		}
	})

	t.Run("create default config preserves existing file", func(t *testing.T) {
		t.Parallel()

		cfgDir := t.TempDir()
		cfgPath := writeConfigMutationFile(t, cfgDir, ConfigFileName+"."+ConfigFileExt, `container_engine: "docker"`)
		if err := CreateDefaultConfig(types.FilesystemPath(cfgDir)); err != nil {
			t.Fatalf("CreateDefaultConfig() error = %v, want nil for existing file", err)
		}
		got, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(got) != `container_engine: "docker"` {
			t.Fatalf("CreateDefaultConfig() overwrote existing file with %q", string(got))
		}
	})

	t.Run("create default config reports directory creation failure", func(t *testing.T) {
		t.Parallel()

		blocker := writeConfigMutationFile(t, t.TempDir(), "not-a-directory", "blocker")
		err := CreateDefaultConfig(types.FilesystemPath(filepath.Join(blocker, "child")))
		if err == nil {
			t.Fatal("CreateDefaultConfig() error = nil, want directory creation failure")
		}
		if !strings.Contains(err.Error(), "failed to create config directory") {
			t.Fatalf("CreateDefaultConfig() error = %v, want directory creation wrapper", err)
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("CreateDefaultConfig() error = %v, want wrapped *os.PathError", err)
		}
	})

	t.Run("create default config reports write failure", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlinked write-failure fixture is Unix-specific")
		}

		cfgDir := t.TempDir()
		cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)
		missingTarget := filepath.Join(t.TempDir(), "missing-parent", "target.cue")
		if err := os.Symlink(missingTarget, cfgPath); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}

		err := CreateDefaultConfig(types.FilesystemPath(cfgDir))
		if err == nil {
			t.Fatal("CreateDefaultConfig() error = nil, want write failure")
		}
		if !strings.Contains(err.Error(), "failed to write config file") {
			t.Fatalf("CreateDefaultConfig() error = %v, want write wrapper", err)
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("CreateDefaultConfig() error = %v, want wrapped *os.PathError", err)
		}
	})
}

func TestConfigMutationSaveWrapsValidationErrors(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.DefaultRuntime = RuntimeMode("bad-runtime")

	err := Save(cfg, types.FilesystemPath(t.TempDir()))
	if err == nil {
		t.Fatal("Save() error = nil, want invalid config")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Save() error = %v, want ErrInvalidConfig", err)
	}
	if !errors.Is(err, ErrInvalidConfigRuntimeMode) {
		t.Fatalf("Save() error = %v, want ErrInvalidConfigRuntimeMode", err)
	}
}

func TestConfigMutationGenerateCUEExactRendering(t *testing.T) {
	t.Parallel()

	defaultOutput := GenerateCUE(DefaultConfig())
	wantHeader := "// Invowk Configuration File\n// See https://github.com/invowk/invowk for documentation.\n\n"
	if !strings.HasPrefix(defaultOutput, wantHeader) {
		t.Fatalf("GenerateCUE() prefix = %q, want %q", defaultOutput[:min(len(defaultOutput), len(wantHeader))], wantHeader)
	}
	if strings.Contains(defaultOutput, "\tconcurrency: 0\n") {
		t.Fatalf("GenerateCUE(defaults) rendered zero LLM concurrency:\n%s", defaultOutput)
	}

	cfg := DefaultConfig()
	cfg.LLM = LLMConfig{Provider: LLMProviderCodex, Model: "gpt-5", Concurrency: 1}
	output := GenerateCUE(cfg)
	if !strings.Contains(output, "\tconcurrency: 1\n") {
		t.Fatalf("GenerateCUE() missing concurrency 1:\n%s", output)
	}
}

func writeConfigMutationFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
