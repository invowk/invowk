// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestConfigSchemaDefaultsMatchDefaultConfig(t *testing.T) {
	t.Parallel()

	got, err := decodeCUEConfigSource(configCUESource{
		data:     configCUEData("{}"),
		filename: configCUEFilename("empty-config.cue"),
	})
	if err != nil {
		t.Fatalf("decodeCUEConfigSource() error = %v, want nil", err)
	}
	want := DefaultConfig()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty config decoded through #Config = %+v, want DefaultConfig() = %+v", got, want)
	}
}

func TestConfigSchemaAppliesNestedDefaultsWithOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := decodeCUEConfigSource(configCUESource{
		data: configCUEData(`
container_engine: "docker"
ui: {
	color_scheme: "dark"
	verbose: true
}
container: {
	auto_provision: {
		enabled: false
	}
}
`),
		filename: configCUEFilename("overrides.cue"),
	})
	if err != nil {
		t.Fatalf("decodeCUEConfigSource() error = %v, want nil", err)
	}

	if cfg.ContainerEngine != ContainerEngineDocker {
		t.Errorf("ContainerEngine = %s, want %s", cfg.ContainerEngine, ContainerEngineDocker)
	}
	if cfg.DefaultRuntime != RuntimeNative {
		t.Errorf("DefaultRuntime = %s, want %s", cfg.DefaultRuntime, RuntimeNative)
	}
	if !cfg.Virtual.Utilities.Enabled {
		t.Error("Virtual.Utilities.Enabled = false, want schema default true")
	}
	if cfg.UI.ColorScheme != ColorSchemeDark || !cfg.UI.Verbose || cfg.UI.Interactive {
		t.Errorf("UI = %+v, want dark verbose=true interactive=false", cfg.UI)
	}
	if cfg.Container.AutoProvision.Enabled {
		t.Error("AutoProvision.Enabled = true, want override false")
	}
	if cfg.Container.AutoProvision.Strict {
		t.Error("AutoProvision.Strict = true, want schema default false")
	}
	if !cfg.Container.AutoProvision.InheritIncludes {
		t.Error("AutoProvision.InheritIncludes = false, want schema default true")
	}
}

func TestConfigSchemaRejectsInvalidOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  string
	}{
		{name: "invalid enum", cue: `container_engine: "nerdctl"`},
		{name: "invalid nested bool", cue: `ui: {verbose: "yes"}`},
		{name: "invalid duration syntax", cue: `llm: {provider: "codex", timeout: "soon"}`},
		{name: "relative include path", cue: `includes: [{path: "relative/example.invowkmod"}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := loadConfigFromContent(t, tt.cue)
			if err == nil {
				t.Fatal("loadWithOptions() succeeded, want invalid override error")
			}
		})
	}
}

func TestConfigSchemaRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  string
	}{
		{name: "root unknown", cue: `future_field: true`},
		{name: "nested ui unknown", cue: `ui: {accent_color: "blue"}`},
		{name: "nested auto provision unknown", cue: `container: {auto_provision: {extra: true}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := loadConfigFromContent(t, tt.cue)
			if err == nil {
				t.Fatal("loadWithOptions() succeeded, want closed schema error")
			}
			if !errors.Is(err, ErrConfigLoadFailed) {
				t.Fatalf("loadWithOptions() error = %v, want ErrConfigLoadFailed", err)
			}
		})
	}
}

func TestConfigSchemaPreservesLLMAPIBlockPresence(t *testing.T) {
	t.Parallel()

	cfg, err := decodeCUEConfigSource(configCUESource{
		data:     configCUEData(`llm: {api: {}}`),
		filename: configCUEFilename("empty-api.cue"),
	})
	if err != nil {
		t.Fatalf("decodeCUEConfigSource() error = %v, want nil", err)
	}
	if !cfg.LLM.HasAPIBackend() {
		t.Fatal("LLM.HasAPIBackend() = false, want true for explicit llm.api block")
	}
	if cfg.LLM.API.HasConfig() {
		t.Fatal("LLM.API.HasConfig() = true, want false for empty llm.api block")
	}
	if err := cfg.LLM.Validate(); !errors.Is(err, ErrInvalidLLMAPIConfig) {
		t.Fatalf("LLM.Validate() error = %v, want ErrInvalidLLMAPIConfig", err)
	}
}

func TestLoadRejectsEmptyLLMAPIConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cue     string
		wantErr error
	}{
		{
			name:    "empty api",
			cue:     `llm: {api: {}}`,
			wantErr: ErrInvalidLLMAPIConfig,
		},
		{
			name:    "provider plus empty api",
			cue:     `llm: {provider: "codex", api: {}}`,
			wantErr: ErrConfigLoadFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := loadConfigFromContent(t, tt.cue)
			if err == nil {
				t.Fatal("loadWithOptions() succeeded, want config load error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("loadWithOptions() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadPreservesLLMPostDecodeValidation(t *testing.T) {
	t.Parallel()

	err := loadConfigFromContent(t, `llm: {api: {base_url: "not-a-url"}}`)
	if err == nil {
		t.Fatal("loadWithOptions() succeeded, want post-decode LLM URL validation error")
	}
	if !errors.Is(err, ErrInvalidLLMBaseURL) {
		t.Fatalf("loadWithOptions() error = %v, want ErrInvalidLLMBaseURL", err)
	}
}

func loadConfigFromContent(t *testing.T, content string) error {
	t.Helper()

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := loadWithOptions(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(cfgDir)})
	return err
}
