// SPDX-License-Identifier: EPL-2.0

// Package vhsnorm provides VHS output normalization for deterministic test comparison.
//
// VHS (charmbracelet/vhs) captures terminal recordings, but its output contains
// non-deterministic content like timestamps, paths, and terminal frame artifacts.
// This package normalizes VHS output to enable reliable golden file comparisons.
package vhsnorm

import (
	"fmt"
	"os"
	"regexp"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

type (
	// Config holds the normalization configuration loaded from a CUE file.
	Config struct {
		// VHSArtifacts controls VHS-specific artifact filtering.
		VHSArtifacts VHSArtifactsConfig `json:"vhs_artifacts"`
		// Substitutions defines regex patterns and their replacements.
		Substitutions []SubstitutionRule `json:"substitutions"`
		// Filters controls general content filtering.
		Filters FiltersConfig `json:"filters"`
	}

	// VHSArtifactsConfig controls VHS-specific artifact filtering.
	VHSArtifactsConfig struct {
		// StripFrameSeparators removes lines consisting only of box-drawing characters.
		StripFrameSeparators bool `json:"strip_frame_separators"`
		// StripEmptyPrompts removes lines that only contain the prompt character.
		StripEmptyPrompts bool `json:"strip_empty_prompts"`
		// Deduplicate removes consecutive duplicate lines.
		Deduplicate bool `json:"deduplicate"`
		// PromptChar is the prompt character to detect empty prompts (default: ">").
		PromptChar string `json:"prompt_char"`
		// SeparatorChars are box-drawing characters that form frame separators.
		SeparatorChars []string `json:"separator_chars"`
	}

	// SubstitutionRule defines a regex pattern and its replacement.
	SubstitutionRule struct {
		// Name is a human-readable identifier for the rule (for debugging/logging).
		Name string `json:"name"`
		// Pattern is a regular expression to match.
		Pattern string `json:"pattern"`
		// Replacement is the string to substitute for matches.
		Replacement string `json:"replacement"`
	}

	// FiltersConfig controls general content filtering.
	FiltersConfig struct {
		// StripANSI removes ANSI escape codes from output.
		StripANSI bool `json:"strip_ansi"`
		// StripEmpty removes empty lines from output.
		StripEmpty bool `json:"strip_empty"`
	}

	// compiledRule is a substitution rule with a pre-compiled regex.
	compiledRule struct {
		name        string
		regex       *regexp.Regexp
		replacement string
	}
)

// DefaultConfig returns a configuration with sensible defaults.
// This is used when no config file is provided.
func DefaultConfig() *Config {
	return &Config{
		VHSArtifacts: VHSArtifactsConfig{
			StripFrameSeparators: true,
			StripEmptyPrompts:    true,
			Deduplicate:          true,
			PromptChar:           ">",
			SeparatorChars:       []string{"─", "━", "═", "│", "┃", "║"},
		},
		Substitutions: defaultSubstitutions(),
		Filters: FiltersConfig{
			StripANSI:  true,
			StripEmpty: true,
		},
	}
}

// defaultSubstitutions returns the default set of substitution rules,
// matching the patterns from the original normalize.sh script.
func defaultSubstitutions() []SubstitutionRule {
	return []SubstitutionRule{
		{
			Name:        "iso_timestamp",
			Pattern:     `[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[A-Z]?`,
			Replacement: "[TIMESTAMP]",
		},
		{
			Name:        "home_linux",
			Pattern:     `/home/[a-zA-Z0-9_-]+`,
			Replacement: "[HOME]",
		},
		{
			Name:        "home_var",
			Pattern:     `/var/home/[a-zA-Z0-9_-]+`,
			Replacement: "[HOME]",
		},
		{
			Name:        "home_macos",
			Pattern:     `/Users/[a-zA-Z0-9_-]+`,
			Replacement: "[HOME]",
		},
		{
			Name:        "tmp_dir",
			Pattern:     `/tmp/[a-zA-Z0-9._-]+`,
			Replacement: "[TMPDIR]",
		},
		{
			Name:        "var_tmp_dir",
			Pattern:     `/var/tmp/[a-zA-Z0-9._-]+`,
			Replacement: "[TMPDIR]",
		},
		{
			Name:        "hostname",
			Pattern:     `hostname: [a-zA-Z0-9._-]+`,
			Replacement: "hostname: [HOSTNAME]",
		},
		{
			Name:        "version_v_prefix",
			Pattern:     `invowk v[0-9]+\.[0-9]+\.[0-9]+[^ ]*`,
			Replacement: "invowk [VERSION]",
		},
		{
			Name:        "version_word",
			Pattern:     `invowk version [0-9]+\.[0-9]+\.[0-9]+[^ ]*`,
			Replacement: "invowk version [VERSION]",
		},
		{
			Name:        "env_user",
			Pattern:     `USER = '[a-zA-Z0-9_-]+'`,
			Replacement: "USER = '[USER]'",
		},
		{
			Name:        "env_home",
			Pattern:     `HOME = '[^']+'`,
			Replacement: "HOME = '[HOME]'",
		},
		{
			Name:        "env_path",
			Pattern:     `PATH = '[^']+'`,
			Replacement: "PATH = '[PATH]'",
		},
		{
			Name:        "env_path_truncated",
			Pattern:     `PATH = '[^']+' \(truncated\)`,
			Replacement: "PATH = '[PATH]' (truncated)",
		},
	}
}

// LoadConfig loads a normalization configuration from a CUE file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	ctx := cuecontext.New()
	value := ctx.CompileBytes(data, cue.Filename(path))
	if err := value.Err(); err != nil {
		return nil, fmt.Errorf("CUE parse error: %w", err)
	}

	// Validate the CUE value
	if err := value.Validate(cue.Concrete(false)); err != nil {
		return nil, fmt.Errorf("CUE validation error: %w", err)
	}

	var cfg Config
	if err := value.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("CUE decode error: %w", err)
	}

	// Apply defaults for missing fields
	cfg = applyDefaults(cfg)

	return &cfg, nil
}

// applyDefaults fills in missing fields with default values.
func applyDefaults(cfg Config) Config {
	defaults := DefaultConfig()

	// Apply VHS artifact defaults
	if cfg.VHSArtifacts.PromptChar == "" {
		cfg.VHSArtifacts.PromptChar = defaults.VHSArtifacts.PromptChar
	}
	if len(cfg.VHSArtifacts.SeparatorChars) == 0 {
		cfg.VHSArtifacts.SeparatorChars = defaults.VHSArtifacts.SeparatorChars
	}

	return cfg
}

// ValidateConfig validates the configuration and compiles all regex patterns.
// This is called internally by NewNormalizer to provide early error detection.
func ValidateConfig(cfg *Config) error {
	for i, rule := range cfg.Substitutions {
		if rule.Pattern == "" {
			return fmt.Errorf("substitution rule %d (%s): empty pattern", i, rule.Name)
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("substitution rule %d (%s): invalid regex: %w", i, rule.Name, err)
		}
	}
	return nil
}
