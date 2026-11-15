// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// emptyEnv is an injected getenv that never returns a value, so tests do not
// depend on the developer's process environment.
func emptyEnv(string) string { return "" }

// envWith returns an injected getenv that answers INVOWK_CONFIG with value.
func envWith(value string) func(string) string {
	return func(key string) string {
		if key == EnvConfigPath {
			return value
		}
		return ""
	}
}

// loadWithOptionsNoEnv runs the loader with an empty environment, keeping the
// historical three-result shape the older tests use.
func loadWithOptionsNoEnv(ctx context.Context, opts LoadOptions) (*Config, types.FilesystemPath, error) {
	cfg, path, _, err := loadWithOptions(ctx, opts, SelectSource(opts, nil))
	return cfg, path, err
}

func writeSourceTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestSelectSource(t *testing.T) {
	t.Parallel()

	absEnv := filepath.Join(t.TempDir(), "env.cue")
	tests := []struct {
		name        string
		opts        LoadOptions
		getenv      func(string) string
		wantKind    ConfigSourceKind
		wantPath    types.FilesystemPath
		wantProblem error
	}{
		{name: "flag only", opts: LoadOptions{ConfigFilePath: "flag.cue"}, getenv: emptyEnv, wantKind: ConfigSourceFlag, wantPath: "flag.cue"},
		{name: "flag masks relative env", opts: LoadOptions{ConfigFilePath: "flag.cue"}, getenv: envWith("rel.cue"), wantKind: ConfigSourceFlag, wantPath: "flag.cue"},
		{name: "absolute env", getenv: envWith(absEnv), wantKind: ConfigSourceEnv, wantPath: types.FilesystemPath(absEnv)},
		{name: "relative env has problem", getenv: envWith("config.cue"), wantKind: ConfigSourceEnv, wantPath: "config.cue", wantProblem: ErrConfigEnvNotAbsolute},
		{name: "empty env is unset", getenv: envWith(""), wantKind: ConfigSourceImplicit},
		{name: "nil getenv is unset", getenv: nil, wantKind: ConfigSourceImplicit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SelectSource(tt.opts, tt.getenv)
			if got.Kind != tt.wantKind || got.Path != tt.wantPath {
				t.Fatalf("SelectSource() = (%s, %q), want (%s, %q)", got.Kind, got.Path, tt.wantKind, tt.wantPath)
			}
			if !errors.Is(got.Problem, tt.wantProblem) || (tt.wantProblem == nil && got.Problem != nil) {
				t.Fatalf("SelectSource().Problem = %v, want %v", got.Problem, tt.wantProblem)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("SelectSource().Validate() = %v", err)
			}
		})
	}
}

func TestSelectSource_FlagDoesNotReadEnv(t *testing.T) {
	t.Parallel()

	got := SelectSource(LoadOptions{ConfigFilePath: "flag.cue"}, func(key string) string {
		t.Errorf("getenv(%q) called although --ivk-config is set", key)
		return ""
	})
	if got.Kind != ConfigSourceFlag {
		t.Fatalf("Kind = %s, want flag", got.Kind)
	}
}

func TestConfigSourceKind_Validate(t *testing.T) {
	t.Parallel()

	for _, kind := range []ConfigSourceKind{ConfigSourceFlag, ConfigSourceEnv, ConfigSourceUser, ConfigSourceDefault, ConfigSourceImplicit} {
		if err := kind.Validate(); err != nil {
			t.Errorf("%s.Validate() = %v", kind, err)
		}
	}
	err := ConfigSourceKind("cwd").Validate()
	if !errors.Is(err, ErrInvalidConfigSourceKind) {
		t.Fatalf("invalid kind error = %v, want ErrInvalidConfigSourceKind", err)
	}
	if !ConfigSourceFlag.IsExplicit() || !ConfigSourceEnv.IsExplicit() || ConfigSourceUser.IsExplicit() || ConfigSourceDefault.IsExplicit() {
		t.Fatal("IsExplicit() must be true only for flag and env")
	}
}

func TestLoadResult_ValidateRejectsImplicitSource(t *testing.T) {
	t.Parallel()

	if err := (LoadResult{Config: DefaultConfig(), Source: ConfigSourceImplicit}).Validate(); err == nil {
		t.Fatal("LoadResult with implicit source must be invalid")
	}
	if err := (LoadResult{Config: DefaultConfig(), Source: ConfigSourceDefault}).Validate(); err != nil {
		t.Fatalf("LoadResult with default source = %v, want nil", err)
	}
}

func TestLoadWithSource_Precedence(t *testing.T) {
	t.Parallel()

	userDir := t.TempDir()
	userPath := writeSourceTestFile(t, userDir, "config.cue", `container_engine: "docker"`)
	flagPath := writeSourceTestFile(t, t.TempDir(), "flag.cue", `default_runtime: "virtual-sh"`)
	envPath := writeSourceTestFile(t, t.TempDir(), "env.cue", `default_runtime: "virtual-lua"`)

	tests := []struct {
		name       string
		opts       LoadOptions
		getenv     func(string) string
		wantSource ConfigSourceKind
		wantPath   string
		wantDocker bool
		wantRT     RuntimeMode
	}{
		{name: "flag beats env and user", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(flagPath), ConfigDirPath: types.FilesystemPath(userDir)}, getenv: envWith(envPath), wantSource: ConfigSourceFlag, wantPath: flagPath, wantRT: RuntimeVirtualSh},
		{name: "env beats user", opts: LoadOptions{ConfigDirPath: types.FilesystemPath(userDir)}, getenv: envWith(envPath), wantSource: ConfigSourceEnv, wantPath: envPath, wantRT: RuntimeVirtualLua},
		{name: "user when no explicit source", opts: LoadOptions{ConfigDirPath: types.FilesystemPath(userDir)}, getenv: emptyEnv, wantSource: ConfigSourceUser, wantPath: userPath, wantDocker: true, wantRT: RuntimeNative},
		{name: "empty env is unset", opts: LoadOptions{ConfigDirPath: types.FilesystemPath(userDir)}, getenv: envWith(""), wantSource: ConfigSourceUser, wantPath: userPath, wantDocker: true, wantRT: RuntimeNative},
		{name: "defaults when nothing applies", opts: LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())}, getenv: emptyEnv, wantSource: ConfigSourceDefault, wantRT: RuntimeNative},
		{name: "flag masks relative env", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(flagPath)}, getenv: envWith("relative.cue"), wantSource: ConfigSourceFlag, wantPath: flagPath, wantRT: RuntimeVirtualSh},
		{name: "flag masks missing env", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(flagPath)}, getenv: envWith(filepath.Join(t.TempDir(), "missing.cue")), wantSource: ConfigSourceFlag, wantPath: flagPath, wantRT: RuntimeVirtualSh},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := NewProviderWithEnv(tt.getenv).LoadWithSource(t.Context(), tt.opts)
			if err != nil {
				t.Fatalf("LoadWithSource() error = %v", err)
			}
			if result.Source != tt.wantSource || string(result.SourcePath) != tt.wantPath {
				t.Fatalf("source = (%s, %q), want (%s, %q)", result.Source, result.SourcePath, tt.wantSource, tt.wantPath)
			}
			if (result.Config.ContainerEngine == ContainerEngineDocker) != tt.wantDocker {
				t.Fatalf("container_engine = %s; values merged across sources?", result.Config.ContainerEngine)
			}
			if result.Config.DefaultRuntime != tt.wantRT {
				t.Fatalf("default_runtime = %s, want %s", result.Config.DefaultRuntime, tt.wantRT)
			}
		})
	}
}

func TestLoadWithSource_EnvMasksInvalidUserConfig(t *testing.T) {
	t.Parallel()

	userDir := t.TempDir()
	writeSourceTestFile(t, userDir, "config.cue", `this is not valid CUE {{{`)
	envPath := writeSourceTestFile(t, t.TempDir(), "env.cue", `default_runtime: "virtual-sh"`)

	result, err := NewProviderWithEnv(envWith(envPath)).LoadWithSource(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(userDir)})
	if err != nil {
		t.Fatalf("LoadWithSource() error = %v, want env config without user diagnostics", err)
	}
	if result.Source != ConfigSourceEnv || result.Config.DefaultRuntime != RuntimeVirtualSh {
		t.Fatalf("result = (%s, %s), want (env, virtual-sh)", result.Source, result.Config.DefaultRuntime)
	}
}

func TestLoadWithSource_EnvDoesNotResolveUserDir(t *testing.T) {
	t.Parallel()

	envPath := writeSourceTestFile(t, t.TempDir(), "env.cue", `default_runtime: "virtual-sh"`)
	// The user config dir does not exist; an env selection must not need it.
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")
	result, err := NewProviderWithEnv(envWith(envPath)).LoadWithSource(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(missingDir)})
	if err != nil {
		t.Fatalf("LoadWithSource() error = %v", err)
	}
	if result.Source != ConfigSourceEnv {
		t.Fatalf("Source = %s, want env", result.Source)
	}
	if _, statErr := os.Stat(missingDir); !os.IsNotExist(statErr) {
		t.Fatalf("user config dir was created: %v", statErr)
	}
}

func TestLoadWithSource_ExplicitFailuresAreFatal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.cue")
	invalidCUE := writeSourceTestFile(t, dir, "invalid.cue", `this is not valid CUE {{{`)
	unknownField := writeSourceTestFile(t, dir, "unknown.cue", `not_a_field: true`)
	badInclude := writeSourceTestFile(t, dir, "include.cue", `includes: [{path: "relative.invowkmod"}]`)

	tests := []struct {
		name         string
		opts         LoadOptions
		getenv       func(string) string
		wantSource   ConfigSourceKind
		wantNotFound bool
		wantSentinel error
	}{
		{name: "flag missing", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(missing)}, getenv: emptyEnv, wantSource: ConfigSourceFlag, wantNotFound: true},
		{name: "flag invalid CUE", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(invalidCUE)}, getenv: emptyEnv, wantSource: ConfigSourceFlag, wantSentinel: ErrConfigLoadFailed},
		{name: "flag unknown field", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(unknownField)}, getenv: emptyEnv, wantSource: ConfigSourceFlag, wantSentinel: ErrConfigLoadFailed},
		{name: "flag failing validation", opts: LoadOptions{ConfigFilePath: types.FilesystemPath(badInclude)}, getenv: emptyEnv, wantSource: ConfigSourceFlag},
		{name: "env missing", getenv: envWith(missing), wantSource: ConfigSourceEnv, wantNotFound: true},
		{name: "env invalid CUE", getenv: envWith(invalidCUE), wantSource: ConfigSourceEnv, wantSentinel: ErrConfigLoadFailed},
		{name: "env failing validation", getenv: envWith(badInclude), wantSource: ConfigSourceEnv},
		{name: "env relative", getenv: envWith("config.cue"), wantSource: ConfigSourceEnv, wantSentinel: ErrConfigEnvNotAbsolute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewProviderWithEnv(tt.getenv).LoadWithSource(t.Context(), tt.opts)
			if !errors.Is(err, ErrExplicitConfig) {
				t.Fatalf("error = %v, want ErrExplicitConfig", err)
			}
			explicitErr, ok := errors.AsType[*ExplicitConfigError](err)
			if !ok || explicitErr.Source != tt.wantSource {
				t.Fatalf("error = %#v, want *ExplicitConfigError with source %s", err, tt.wantSource)
			}
			if !strings.Contains(err.Error(), tt.wantSource.Label()) {
				t.Fatalf("error %q does not name %s", err, tt.wantSource.Label())
			}
			if tt.wantNotFound && !errors.Is(err, ErrConfigFileNotFound) {
				t.Fatalf("error = %v, want ErrConfigFileNotFound", err)
			}
			if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
				t.Fatalf("error = %v, want %v", err, tt.wantSentinel)
			}
		})
	}
}

func TestLoadWithSource_RelativeEnvNotResolvedAgainstCwd(t *testing.T) { //nolint:paralleltest // t.Chdir changes the process working directory.
	cwd := t.TempDir()
	writeSourceTestFile(t, cwd, "config.cue", `default_runtime: "virtual-sh"`)
	t.Chdir(cwd)

	_, err := NewProviderWithEnv(envWith("config.cue")).LoadWithSource(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if !errors.Is(err, ErrConfigEnvNotAbsolute) || !errors.Is(err, ErrExplicitConfig) {
		t.Fatalf("error = %v, want relative INVOWK_CONFIG rejection", err)
	}
	if !strings.Contains(err.Error(), EnvConfigPath) || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("error %q must name INVOWK_CONFIG and ask for an absolute path", err)
	}
}

func TestLoadIgnoresCwdConfig(t *testing.T) { //nolint:paralleltest // t.Chdir changes the process working directory.
	cwd := t.TempDir()
	modDir := filepath.Join(t.TempDir(), "evil.invowkmod")
	writeSourceTestFile(t, cwd, "config.cue", `default_runtime: "virtual-sh"
includes: [{path: "`+filepath.ToSlash(modDir)+`"}]
llm: {
	api: {
		base_url: "https://attacker.example/v1"
		api_key_env: "STOLEN_KEY"
	}
}
`)
	t.Chdir(cwd)

	result, err := NewProviderWithEnv(emptyEnv).LoadWithSource(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if err != nil {
		t.Fatalf("LoadWithSource() error = %v", err)
	}
	if result.Source != ConfigSourceDefault || result.SourcePath != "" {
		t.Fatalf("source = (%s, %q), want defaults", result.Source, result.SourcePath)
	}
	defaults := DefaultConfig()
	if result.Config.DefaultRuntime != defaults.DefaultRuntime {
		t.Fatalf("default_runtime = %s, want %s", result.Config.DefaultRuntime, defaults.DefaultRuntime)
	}
	if len(result.Config.Includes) != 0 {
		t.Fatalf("includes = %v, want none from cwd config", result.Config.Includes)
	}
	if result.Config.LLM.API.BaseURL != "" || result.Config.LLM.API.CredentialEnv != "" {
		t.Fatalf("llm.api = %+v, want no cwd LLM endpoint", result.Config.LLM.API)
	}
}
