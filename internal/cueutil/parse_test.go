// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
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
	t.Run("valid config parses successfully", func(t *testing.T) {
		data := []byte(`
name: "test"
count: 42
enabled: true
description: "A test config"
`)
		result, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
		if err != nil {
			t.Fatalf("ParseAndDecode failed: %v", err)
		}

		if result.Value.Name != "test" {
			t.Errorf("expected name='test', got %q", result.Value.Name)
		}
		if result.Value.Count != 42 {
			t.Errorf("expected count=42, got %d", result.Value.Count)
		}
		if !result.Value.Enabled {
			t.Error("expected enabled=true")
		}
		if result.Value.Description != "A test config" {
			t.Errorf("expected description='A test config', got %q", result.Value.Description)
		}
	})

	t.Run("optional field can be omitted", func(t *testing.T) {
		data := []byte(`
name: "minimal"
count: 1
enabled: false
`)
		result, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
		if err != nil {
			t.Fatalf("ParseAndDecode failed: %v", err)
		}

		if result.Value.Name != "minimal" {
			t.Errorf("expected name='minimal', got %q", result.Value.Name)
		}
		if result.Value.Description != "" {
			t.Errorf("expected empty description, got %q", result.Value.Description)
		}
	})

	t.Run("invalid type returns error", func(t *testing.T) {
		data := []byte(`
name: "test"
count: "not a number"  // Should be int
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
		if err == nil {
			t.Error("expected error for invalid type")
		}
	})

	t.Run("missing required field returns error", func(t *testing.T) {
		data := []byte(`
name: "test"
// count is missing
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
		if err == nil {
			t.Error("expected error for missing required field")
		}
	})

	t.Run("WithFilename sets filename in errors", func(t *testing.T) {
		data := []byte(`
name: "test"
count: "invalid"
enabled: true
`)
		_, err := ParseAndDecode[TestConfig](
			[]byte(testSchema),
			data,
			"#TestConfig",
			WithFilename("my-config.cue"),
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "my-config.cue") {
			t.Errorf("error should contain filename, got: %v", err)
		}
	})
}

// T017: Tests for Invkmod type parsing (simulated)
func TestParseInvkmodType(t *testing.T) {
	// Simulated invkmod schema for testing
	invkmodSchema := `
#Invkmod: {
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
	type Invkmod struct {
		Module      string        `json:"module"`
		Version     string        `json:"version,omitempty"`
		Description string        `json:"description,omitempty"`
		Requires    []Requirement `json:"requires,omitempty"`
	}

	t.Run("valid invkmod parses successfully", func(t *testing.T) {
		data := []byte(`
module: "io.example.mymodule"
version: "1.0.0"
description: "My test module"
requires: [
	{module: "io.example.dep1"},
	{module: "io.example.dep2", alias: "d2"},
]
`)
		result, err := ParseAndDecode[Invkmod]([]byte(invkmodSchema), data, "#Invkmod")
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

	t.Run("minimal invkmod parses successfully", func(t *testing.T) {
		data := []byte(`
module: "io.example.minimal"
`)
		result, err := ParseAndDecode[Invkmod]([]byte(invkmodSchema), data, "#Invkmod")
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
	// Simulated config schema with optional fields
	configSchema := `
#Config: {
	container_engine?: "docker" | "podman"
	search_paths?: [...string]
	default_runtime?: "native" | "virtual" | "container"
}
`

	type Config struct {
		ContainerEngine string   `json:"container_engine,omitempty"`
		SearchPaths     []string `json:"search_paths,omitempty"`
		DefaultRuntime  string   `json:"default_runtime,omitempty"`
	}

	t.Run("full config parses successfully", func(t *testing.T) {
		data := []byte(`
container_engine: "podman"
search_paths: ["./", "~/.config/invowk"]
default_runtime: "virtual"
`)
		result, err := ParseAndDecode[Config]([]byte(configSchema), data, "#Config")
		if err != nil {
			t.Fatalf("ParseAndDecode failed: %v", err)
		}

		if result.Value.ContainerEngine != "podman" {
			t.Errorf("expected container_engine='podman', got %q", result.Value.ContainerEngine)
		}
		if len(result.Value.SearchPaths) != 2 {
			t.Errorf("expected 2 search_paths, got %d", len(result.Value.SearchPaths))
		}
	})

	t.Run("empty config parses with WithConcrete(false)", func(t *testing.T) {
		data := []byte(`{}`)
		result, err := ParseAndDecode[Config](
			[]byte(configSchema),
			data,
			"#Config",
			WithConcrete(false),
		)
		if err != nil {
			t.Fatalf("ParseAndDecode failed: %v", err)
		}

		if result.Value.ContainerEngine != "" {
			t.Errorf("expected empty container_engine, got %q", result.Value.ContainerEngine)
		}
	})

	t.Run("invalid enum value returns error", func(t *testing.T) {
		data := []byte(`
container_engine: "kubernetes"  // Invalid: not docker or podman
`)
		_, err := ParseAndDecode[Config]([]byte(configSchema), data, "#Config")
		if err == nil {
			t.Error("expected error for invalid enum value")
		}
	})
}

// T019: File size limit enforcement tests
func TestFileSizeLimit(t *testing.T) {
	t.Run("file within limit parses successfully", func(t *testing.T) {
		data := []byte(`
name: "test"
count: 1
enabled: true
`)
		_, err := ParseAndDecode[TestConfig](
			[]byte(testSchema),
			data,
			"#TestConfig",
			WithMaxFileSize(1024), // 1KB limit
		)
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
	})

	t.Run("file exceeding limit returns error", func(t *testing.T) {
		// Create data larger than the limit
		data := make([]byte, 200)
		for i := range data {
			data[i] = 'a'
		}

		_, err := ParseAndDecode[TestConfig](
			[]byte(testSchema),
			data,
			"#TestConfig",
			WithMaxFileSize(100), // 100 byte limit
		)
		if err == nil {
			t.Error("expected error for oversized file")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("error should mention size limit, got: %v", err)
		}
	})

	t.Run("default limit is applied", func(t *testing.T) {
		// Create data well under default limit
		data := []byte(`name: "test"
count: 1
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
		if err != nil {
			t.Errorf("expected success with default limit, got error: %v", err)
		}
	})
}

// Test ParseAndDecodeString convenience function
func TestParseAndDecodeString(t *testing.T) {
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
