// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// GenerateCUE tests — direct coverage of conditional rendering branches

func TestGenerateCUE_DefaultConfig(t *testing.T) {
	t.Parallel()
	output := GenerateCUE(DefaultConfig())

	for _, want := range []string{"container_engine:", "default_runtime:", "virtual_shell:", "ui:", "container:"} {
		if !strings.Contains(output, want) {
			t.Errorf("GenerateCUE(defaults) missing %q", want)
		}
	}
	// Default config has no includes — top-level "includes:" section should be absent.
	// The auto_provision section has "inherit_includes:" which is a different field.
	if strings.Contains(output, "\nincludes:") {
		t.Error("GenerateCUE(defaults) should not contain top-level 'includes:' when Includes is empty")
	}
	// Default config has empty BinaryPath and CacheDir — these conditional fields should be omitted.
	if strings.Contains(output, "binary_path:") {
		t.Error("GenerateCUE(defaults) should omit binary_path when empty")
	}
	if strings.Contains(output, "cache_dir:") {
		t.Error("GenerateCUE(defaults) should omit cache_dir when empty")
	}
}

func TestGenerateCUE_IncludesWithAndWithoutAliases(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	pathOne := filepath.Join(tmpDir, "path", "one.invowkmod")
	pathTwo := filepath.Join(tmpDir, "path", "two.invowkmod")

	cfg := DefaultConfig()
	cfg.Includes = []IncludeEntry{
		{Path: ModuleIncludePath(pathOne), Alias: "one"},
		{Path: ModuleIncludePath(pathTwo)},
	}
	output := GenerateCUE(cfg)

	if !strings.Contains(output, `alias: "one"`) {
		t.Error("GenerateCUE should render alias when set")
	}
	if !strings.Contains(output, fmt.Sprintf("path: %q", pathTwo)) {
		t.Error("GenerateCUE should render path without alias")
	}
	// Entry without alias should NOT have an alias field
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "two.invowkmod") && strings.Contains(line, "alias:") {
			t.Error("entry without alias should not render alias field")
		}
	}
}

func TestGenerateCUE_ConditionalBinaryPathAndCacheDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "bin", "invowk")
	cacheDir := filepath.Join(tmpDir, "invowk-cache")
	modPath := filepath.Join(tmpDir, "modules", "one.invowkmod")

	cfg := DefaultConfig()
	cfg.Container.AutoProvision.BinaryPath = BinaryFilePath(binaryPath)
	cfg.Container.AutoProvision.CacheDir = CacheDirPath(cacheDir)
	cfg.Container.AutoProvision.Includes = []IncludeEntry{
		{Path: ModuleIncludePath(modPath), Alias: "prov-one"},
	}
	output := GenerateCUE(cfg)

	if !strings.Contains(output, fmt.Sprintf("binary_path: %q", binaryPath)) {
		t.Error("GenerateCUE should render binary_path when non-empty")
	}
	if !strings.Contains(output, fmt.Sprintf("cache_dir: %q", cacheDir)) {
		t.Error("GenerateCUE should render cache_dir when non-empty")
	}
	if !strings.Contains(output, `alias: "prov-one"`) {
		t.Error("GenerateCUE should render auto_provision includes with alias")
	}
}

func TestGenerateCUE_Roundtrip(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ContainerEngine: ContainerEngineDocker,
		DefaultRuntime:  "virtual",
		VirtualShell:    VirtualShellConfig{EnableUrootUtils: false},
		UI:              UIConfig{ColorScheme: "dark", Verbose: true, Interactive: true},
		Container: ContainerConfig{
			AutoProvision: AutoProvisionConfig{
				Enabled:         false,
				InheritIncludes: false,
			},
		},
	}

	cueContent := GenerateCUE(cfg)

	// Write to temp file, reload, and verify fields match
	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, AppName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(cueContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, _, err := loadWithOptions(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(cfgDir)})
	if err != nil {
		t.Fatalf("loadWithOptions() roundtrip error: %v", err)
	}
	if loaded.ContainerEngine != cfg.ContainerEngine {
		t.Errorf("roundtrip ContainerEngine = %s, want %s", loaded.ContainerEngine, cfg.ContainerEngine)
	}
	if loaded.DefaultRuntime != cfg.DefaultRuntime {
		t.Errorf("roundtrip DefaultRuntime = %s, want %s", loaded.DefaultRuntime, cfg.DefaultRuntime)
	}
	if loaded.UI.ColorScheme != cfg.UI.ColorScheme {
		t.Errorf("roundtrip UI.ColorScheme = %s, want %s", loaded.UI.ColorScheme, cfg.UI.ColorScheme)
	}
	if loaded.UI.Verbose != cfg.UI.Verbose {
		t.Errorf("roundtrip UI.Verbose = %v, want %v", loaded.UI.Verbose, cfg.UI.Verbose)
	}
	if loaded.VirtualShell.EnableUrootUtils != cfg.VirtualShell.EnableUrootUtils {
		t.Errorf("roundtrip EnableUrootUtils = %v, want %v", loaded.VirtualShell.EnableUrootUtils, cfg.VirtualShell.EnableUrootUtils)
	}
	if loaded.UI.Interactive != cfg.UI.Interactive {
		t.Errorf("roundtrip UI.Interactive = %v, want %v", loaded.UI.Interactive, cfg.UI.Interactive)
	}
	if loaded.Container.AutoProvision.Enabled != cfg.Container.AutoProvision.Enabled {
		t.Errorf("roundtrip AutoProvision.Enabled = %v, want %v", loaded.Container.AutoProvision.Enabled, cfg.Container.AutoProvision.Enabled)
	}
}

// TestNoGlobalConfigAccess guards against re-introduction of global config
// accessors in production code paths. The config package has no global mutable
// state; this test ensures the pattern is not reintroduced.
func TestNoGlobalConfigAccess(t *testing.T) {
	t.Parallel()
	// Derive project root from this test file's location (internal/config/).
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path via runtime.Caller")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	// Patterns that must not appear in production Go source files.
	prohibited := []struct {
		pattern string
		reason  string
	}{
		{"config.Get()", "use config.Provider.Load() with explicit LoadOptions instead"},
	}

	dirs := []string{
		filepath.Join(projectRoot, "cmd"),
		filepath.Join(projectRoot, "internal"),
	}

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Skip test files — they may reference patterns for assertion purposes.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			content, readErr := os.ReadFile(path) //nolint:gosec // G122 — test walks project source tree; no symlink TOCTOU risk
			if readErr != nil {
				t.Errorf("failed to read %s: %v", path, readErr)
				return nil
			}

			src := string(content)
			rel, _ := filepath.Rel(projectRoot, path)

			for _, p := range prohibited {
				if strings.Contains(src, p.pattern) {
					t.Errorf("%s: contains prohibited pattern %q — %s", rel, p.pattern, p.reason)
				}
			}

			return nil
		})
		if err != nil {
			t.Fatalf("failed to walk %s: %v", dir, err)
		}
	}
}
