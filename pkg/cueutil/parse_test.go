// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

// Simple test schema for parsing tests
const testSchema = `
#TestConfig: {
	name:        string
	count:       int
	enabled:     bool
	description?: string
}
`

// TestConfig is a simple struct for testing generic parsing
type TestConfig struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
}

// T016: Tests for basic CUE parsing
func TestParseAndDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		data        string
		options     []Option
		want        TestConfig
		wantErr     bool
		wantErrText string
	}{
		{name: "valid config parses successfully", data: `
name: "test"
count: 42
enabled: true
description: "A test config"
`, want: TestConfig{Name: "test", Count: 42, Enabled: true, Description: "A test config"}},
		{name: "optional field can be omitted", data: `
name: "minimal"
count: 1
enabled: false
`, want: TestConfig{Name: "minimal", Count: 1}},
		{name: "invalid type returns error", data: `
name: "test"
count: "not a number"  // Should be int
enabled: true
`, wantErr: true},
		{name: "missing required field returns error", data: `
name: "test"
// count is missing
enabled: true
`, wantErr: true},
		{name: "WithFilename sets filename in errors", data: `
name: "test"
count: "invalid"
enabled: true
`, options: []Option{WithFilename("my-config.cue")}, wantErr: true, wantErrText: "my-config.cue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseAndDecode[TestConfig]([]byte(testSchema), []byte(tt.data), "#TestConfig", tt.options...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAndDecode() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("error = %v, want text %q", err, tt.wantErrText)
			}
			if !tt.wantErr && *result.Value != tt.want {
				t.Errorf("ParseAndDecode() value = %#v, want %#v", *result.Value, tt.want)
			}
		})
	}
}

// T017: Tests for Invowkmod type parsing (simulated)
func TestParseInvowkmodType(t *testing.T) {
	t.Parallel()

	// Simulated invowkmod schema for testing
	invowkmodSchema := `
#Invowkmod: {
	module:       string
	version?:     string
	description?: string
	requires?: [...{
		module: string
		alias?: string
	}]
}
`

	type Requirement struct {
		Module string `json:"module"`
		Alias  string `json:"alias,omitempty"`
	}
	type Invowkmod struct {
		Module      string        `json:"module"`
		Version     string        `json:"version,omitempty"`
		Description string        `json:"description,omitempty"`
		Requires    []Requirement `json:"requires,omitempty"`
	}

	t.Run("valid invowkmod parses successfully", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
module: "io.example.mymodule"
version: "1.0.0"
description: "My test module"
requires: [
	{module: "io.example.dep1"},
	{module: "io.example.dep2", alias: "d2"},
]
`)
		result, err := ParseAndDecode[Invowkmod]([]byte(invowkmodSchema), data, "#Invowkmod")
		if err != nil {
			t.Fatalf("ParseAndDecode failed: %v", err)
		}

		if result.Value.Module != "io.example.mymodule" {
			t.Errorf("expected module='io.example.mymodule', got %q", result.Value.Module)
		}
		if len(result.Value.Requires) != 2 {
			t.Errorf("expected 2 requires, got %d", len(result.Value.Requires))
		}
	})

	t.Run("minimal invowkmod parses successfully", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
module: "io.example.minimal"
`)
		result, err := ParseAndDecode[Invowkmod]([]byte(invowkmodSchema), data, "#Invowkmod")
		if err != nil {
			t.Fatalf("ParseAndDecode failed: %v", err)
		}

		if result.Value.Module != "io.example.minimal" {
			t.Errorf("expected module='io.example.minimal', got %q", result.Value.Module)
		}
	})
}

// T018: Tests for Config type parsing (simulated)
func TestParseConfigType(t *testing.T) {
	t.Parallel()

	// Simulated config schema with optional fields
	configSchema := `
#Config: {
	container_engine?: "docker" | "podman"
	includes?: [...string]
	default_runtime?: "native" | "virtual-sh" | "virtual-lua" | "container"
}
`

	type Config struct {
		ContainerEngine string   `json:"container_engine,omitempty"`
		Includes        []string `json:"includes,omitempty"`
		DefaultRuntime  string   `json:"default_runtime,omitempty"`
	}

	tests := []struct {
		name    string
		data    string
		options []Option
		want    Config
		wantErr bool
	}{
		{name: "full config parses successfully", data: `
container_engine: "podman"
includes: ["./", "~/.config/invowk"]
default_runtime: "virtual-sh"
`, want: Config{ContainerEngine: "podman", Includes: []string{"./", "~/.config/invowk"}, DefaultRuntime: "virtual-sh"}},
		{name: "empty config parses with WithConcrete(false)", data: `{}`, options: []Option{WithConcrete(false)}},
		{name: "invalid enum value returns error", data: `
container_engine: "kubernetes"  // Invalid: not docker or podman
`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseAndDecode[Config]([]byte(configSchema), []byte(tt.data), "#Config", tt.options...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAndDecode() error = %v, wantErr %t", err, tt.wantErr)
			}
			if !tt.wantErr && (result.Value.ContainerEngine != tt.want.ContainerEngine ||
				!slices.Equal(result.Value.Includes, tt.want.Includes) ||
				result.Value.DefaultRuntime != tt.want.DefaultRuntime) {
				t.Errorf("ParseAndDecode() value = %#v, want %#v", result.Value, tt.want)
			}
		})
	}
}

// T019: File size limit enforcement tests
func TestFileSizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		data         []byte
		maxFileSize  int64
		useDefault   bool
		wantErr      bool
		wantSentinel error
	}{
		{name: "file within limit parses successfully", data: []byte(`
name: "test"
count: 1
enabled: true
`), maxFileSize: 1024},
		{name: "file exceeding limit returns error", data: bytes.Repeat([]byte{'a'}, 200), maxFileSize: 100, wantErr: true, wantSentinel: ErrFileSizeExceeded},
		{name: "default limit is applied", data: []byte(`name: "test"
count: 1
enabled: true
`), useDefault: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := []Option{}
			if !tt.useDefault {
				options = append(options, WithMaxFileSize(tt.maxFileSize))
			}
			_, err := ParseAndDecode[TestConfig]([]byte(testSchema), tt.data, "#TestConfig", options...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAndDecode() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
				t.Errorf("ParseAndDecode() error = %v, want %v", err, tt.wantSentinel)
			}
		})
	}
}

// Test ParseAndDecodeString convenience function
func TestParseAndDecodeString(t *testing.T) {
	t.Parallel()

	data := []byte(`
name: "test"
count: 42
enabled: true
`)
	result, err := ParseAndDecodeString[TestConfig](testSchema, data, "#TestConfig")
	if err != nil {
		t.Fatalf("ParseAndDecodeString failed: %v", err)
	}

	if result.Value.Name != "test" {
		t.Errorf("expected name='test', got %q", result.Value.Name)
	}
}

// Test that Unified value is accessible
func TestUnifiedValueAccess(t *testing.T) {
	t.Parallel()

	data := []byte(`
name: "test"
count: 42
enabled: true
`)
	result, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
	if err != nil {
		t.Fatalf("ParseAndDecode failed: %v", err)
	}

	// Verify we can access the unified value
	if result.Unified.Err() != nil {
		t.Errorf("unified value has error: %v", result.Unified.Err())
	}
}
