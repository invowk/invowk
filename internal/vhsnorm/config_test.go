// SPDX-License-Identifier: MPL-2.0

package vhsnorm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check VHS artifacts defaults
	if !cfg.VHSArtifacts.StripFrameSeparators {
		t.Error("StripFrameSeparators should be true by default")
	}
	if !cfg.VHSArtifacts.StripEmptyPrompts {
		t.Error("StripEmptyPrompts should be true by default")
	}
	if !cfg.VHSArtifacts.Deduplicate {
		t.Error("Deduplicate should be true by default")
	}
	if cfg.VHSArtifacts.PromptChar != ">" {
		t.Errorf("PromptChar should be '>', got %q", cfg.VHSArtifacts.PromptChar)
	}
	if len(cfg.VHSArtifacts.SeparatorChars) == 0 {
		t.Error("SeparatorChars should not be empty")
	}

	// Check filters defaults
	if !cfg.Filters.StripANSI {
		t.Error("StripANSI should be true by default")
	}
	if !cfg.Filters.StripEmpty {
		t.Error("StripEmpty should be true by default")
	}

	// Check substitutions exist
	if len(cfg.Substitutions) == 0 {
		t.Error("Substitutions should not be empty")
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(*testing.T, *Config)
		wantErr bool
	}{
		{
			name: "valid minimal config",
			content: `
vhs_artifacts: {
	strip_frame_separators: false
	strip_empty_prompts: true
	deduplicate: true
}
substitutions: []
filters: {
	strip_ansi: true
	strip_empty: false
}
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.VHSArtifacts.StripFrameSeparators {
					t.Error("StripFrameSeparators should be false")
				}
				if !cfg.VHSArtifacts.StripEmptyPrompts {
					t.Error("StripEmptyPrompts should be true")
				}
				if cfg.Filters.StripEmpty {
					t.Error("StripEmpty should be false")
				}
			},
		},
		{
			name: "valid config with substitutions",
			content: `
vhs_artifacts: {
	strip_frame_separators: true
	strip_empty_prompts: true
	deduplicate: true
	prompt_char: "$"
	separator_chars: ["─", "═"]
}
substitutions: [
	{name: "test", pattern: "foo", replacement: "bar"},
]
filters: {
	strip_ansi: true
	strip_empty: true
}
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.VHSArtifacts.PromptChar != "$" {
					t.Errorf("PromptChar should be '$', got %q", cfg.VHSArtifacts.PromptChar)
				}
				if len(cfg.VHSArtifacts.SeparatorChars) != 2 {
					t.Errorf("SeparatorChars should have 2 items, got %d", len(cfg.VHSArtifacts.SeparatorChars))
				}
				if len(cfg.Substitutions) != 1 {
					t.Errorf("Substitutions should have 1 item, got %d", len(cfg.Substitutions))
				}
				if cfg.Substitutions[0].Name != "test" {
					t.Errorf("Substitution name should be 'test', got %q", cfg.Substitutions[0].Name)
				}
			},
		},
		{
			name: "invalid CUE syntax",
			content: `
vhs_artifacts: {
	invalid syntax here
}
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "test.cue")
			if err := os.WriteFile(cfgPath, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			cfg, err := LoadConfig(cfgPath)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.cue")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid custom config",
			cfg: &Config{
				Substitutions: []SubstitutionRule{
					{Name: "test", Pattern: `\d+`, Replacement: "[NUM]"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex pattern",
			cfg: &Config{
				Substitutions: []SubstitutionRule{
					{Name: "bad", Pattern: `[invalid`, Replacement: "x"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty pattern",
			cfg: &Config{
				Substitutions: []SubstitutionRule{
					{Name: "empty", Pattern: "", Replacement: "x"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	// Config with empty prompt_char should get default
	cfg := Config{
		VHSArtifacts: VHSArtifactsConfig{
			PromptChar:     "",
			SeparatorChars: nil,
		},
	}

	result := applyDefaults(cfg)

	if result.VHSArtifacts.PromptChar != ">" {
		t.Errorf("PromptChar should default to '>', got %q", result.VHSArtifacts.PromptChar)
	}
	if len(result.VHSArtifacts.SeparatorChars) == 0 {
		t.Error("SeparatorChars should have defaults applied")
	}
}
