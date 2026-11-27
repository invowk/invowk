// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	cueerrors "cuelang.org/go/cue/errors"
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

func TestCompileSchema(t *testing.T) {
	t.Parallel()

	t.Run("valid schema compiles successfully", func(t *testing.T) {
		t.Parallel()

		compiled, err := CompileSchema([]byte(testSchema), "#TestConfig")
		if err != nil {
			t.Fatalf("CompileSchema() error = %v", err)
		}
		if compiled == nil || !compiled.compiled {
			t.Fatalf("CompileSchema() = %#v, want initialized CompiledSchema", compiled)
		}
	})

	t.Run("empty or whitespace schema path returns ErrInvalidCUEPath", func(t *testing.T) {
		t.Parallel()

		for _, rawPath := range []CUEPath{"", "   \t "} {
			_, err := CompileSchema([]byte(testSchema), rawPath)
			if !errors.Is(err, ErrInvalidCUEPath) {
				t.Errorf("CompileSchema(%q) error = %v, want ErrInvalidCUEPath", rawPath, err)
			}
		}
	})

	t.Run("malformed schema syntax wraps CUE error", func(t *testing.T) {
		t.Parallel()

		_, err := CompileSchema([]byte(`#TestConfig: {name: `), "#TestConfig")
		if err == nil || !strings.Contains(err.Error(), "internal error: failed to compile schema:") {
			t.Fatalf("CompileSchema() error = %v, want internal compile schema error", err)
		}
		if _, ok := errors.AsType[cueerrors.Error](err); !ok {
			t.Fatalf("CompileSchema() error = %v, want wrapped CUE error", err)
		}
	})

	t.Run("missing schema definition wraps CUE error", func(t *testing.T) {
		t.Parallel()

		_, err := CompileSchema([]byte(testSchema), "#Missing")
		if err == nil || !strings.Contains(err.Error(), "internal error: schema definition #Missing not found:") {
			t.Fatalf("CompileSchema() error = %v, want missing definition error", err)
		}
		if _, ok := errors.AsType[cueerrors.Error](err); !ok {
			t.Fatalf("CompileSchema() error = %v, want wrapped CUE error", err)
		}
	})
}

func TestParseAndDecodeCompiled(t *testing.T) {
	t.Parallel()

	compiled, err := CompileSchema([]byte(testSchema), "#TestConfig")
	if err != nil {
		t.Fatalf("CompileSchema() error = %v", err)
	}

	t.Run("nil and zero-value schemas fail with ErrInvalidCompiledSchema", func(t *testing.T) {
		t.Parallel()

		data := []byte(`name: "test"
count: 1
enabled: true
`)
		for _, tc := range []struct {
			name   string
			schema *CompiledSchema
		}{
			{name: "nil schema", schema: nil},
			{name: "zero-value schema", schema: &CompiledSchema{}},
		} {
			_, gotErr := ParseAndDecodeCompiled[TestConfig](tc.schema, data)
			if !errors.Is(gotErr, ErrInvalidCompiledSchema) {
				t.Errorf("%s: error = %v, want ErrInvalidCompiledSchema", tc.name, gotErr)
			}
		}
	})

	t.Run("matches ParseAndDecode parity across valid and invalid inputs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			data         string
			options      []Option
			want         TestConfig
			wantErr      bool
			wantErrText  string
			wantSentinel error
		}{
			{
				name: "valid config",
				data: `
name: "compiled"
count: 7
enabled: true
description: "ok"
`,
				want: TestConfig{Name: "compiled", Count: 7, Enabled: true, Description: "ok"},
			},
			{
				name: "optional omitted",
				data: `
name: "minimal"
count: 2
enabled: false
`,
				want: TestConfig{Name: "minimal", Count: 2},
			},
			{
				name: "syntax error formats filename",
				data: `name: [`,
				options: []Option{
					WithFilename("bad-syntax.cue"),
				},
				wantErr:     true,
				wantErrText: "bad-syntax.cue:",
			},
			{
				name: "concrete validation error formats path and filename",
				data: `
name: "missing-count"
enabled: true
`,
				options: []Option{
					WithFilename("incomplete.cue"),
				},
				wantErr:     true,
				wantErrText: "incomplete.cue: #TestConfig.count:",
			},
			{
				name: "size limit enforced",
				data: `name: "too-long"`,
				options: []Option{
					WithMaxFileSize(4),
					WithFilename("big.cue"),
				},
				wantErr:      true,
				wantErrText:  "big.cue:",
				wantSentinel: ErrFileSizeExceeded,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				result, gotErr := ParseAndDecodeCompiled[TestConfig](compiled, []byte(tt.data), tt.options...)
				legacyResult, legacyErr := ParseAndDecode[TestConfig]([]byte(testSchema), []byte(tt.data), "#TestConfig", tt.options...)
				if (gotErr != nil) != tt.wantErr {
					t.Fatalf("ParseAndDecodeCompiled() error = %v, wantErr %t", gotErr, tt.wantErr)
				}
				if (legacyErr != nil) != tt.wantErr {
					t.Fatalf("ParseAndDecode() error = %v, wantErr %t", legacyErr, tt.wantErr)
				}
				if tt.wantErr {
					if gotErr.Error() != legacyErr.Error() {
						t.Errorf("error mismatch:\n  compiled: %q\n  legacy:   %q", gotErr.Error(), legacyErr.Error())
					}
					if tt.wantErrText != "" && !strings.Contains(gotErr.Error(), tt.wantErrText) {
						t.Errorf("error = %v, want substring %q", gotErr, tt.wantErrText)
					}
					if tt.wantSentinel != nil && !errors.Is(gotErr, tt.wantSentinel) {
						t.Errorf("error = %v, want sentinel %v", gotErr, tt.wantSentinel)
					}
					return
				}

				if *result.Value != tt.want {
					t.Errorf("value = %#v, want %#v", *result.Value, tt.want)
				}
				if *result.Value != *legacyResult.Value {
					t.Errorf("value mismatch: compiled = %#v, legacy = %#v", *result.Value, *legacyResult.Value)
				}
				if result.Unified.Err() != nil {
					t.Errorf("unified value error = %v", result.Unified.Err())
				}
			})
		}
	})

	t.Run("decode error formats filename identically to ParseAndDecode", func(t *testing.T) {
		t.Parallel()

		const decodeSchema = `
#DecodeTarget: {
	count: string
}
`
		type decodeTarget struct {
			Count int `json:"count"`
		}

		decodeCompiled, decodeCompileErr := CompileSchema([]byte(decodeSchema), "#DecodeTarget")
		if decodeCompileErr != nil {
			t.Fatalf("CompileSchema() error = %v", decodeCompileErr)
		}

		data := []byte(`count: "forty-two"`)
		_, gotErr := ParseAndDecodeCompiled[decodeTarget](decodeCompiled, data, WithFilename("decode.cue"))
		_, legacyErr := ParseAndDecode[decodeTarget]([]byte(decodeSchema), data, "#DecodeTarget", WithFilename("decode.cue"))
		if gotErr == nil || legacyErr == nil {
			t.Fatalf("expected decode errors, got compiled=%v legacy=%v", gotErr, legacyErr)
		}
		if gotErr.Error() != legacyErr.Error() {
			t.Errorf("decode error mismatch:\n  compiled: %q\n  legacy:   %q", gotErr.Error(), legacyErr.Error())
		}
		if !strings.Contains(gotErr.Error(), "decode.cue:") {
			t.Errorf("error = %v, want decode.cue:", gotErr)
		}
	})

	t.Run("non-concrete option matches ParseAndDecode behavior", func(t *testing.T) {
		t.Parallel()

		const looseSchema = `
#Loose: {
	mode?:    "on" | "off"
	required: string
}
`
		type looseConfig struct {
			Mode string `json:"mode,omitempty"`
		}

		looseCompiled, looseErr := CompileSchema([]byte(looseSchema), "#Loose")
		if looseErr != nil {
			t.Fatalf("CompileSchema() error = %v", looseErr)
		}

		// WithConcrete(true) rejects missing required field; WithConcrete(false) skips concreteness check.
		if _, strictErr := ParseAndDecodeCompiled[looseConfig](looseCompiled, []byte(`mode: "on"`)); strictErr == nil {
			t.Fatal("expected default concrete validation to reject missing required field")
		}
		okRes, okErr := ParseAndDecodeCompiled[looseConfig](looseCompiled, []byte(`mode: "on"`), WithConcrete(false))
		if okErr != nil {
			t.Fatalf("non-concrete parse error = %v", okErr)
		}
		if okRes.Value.Mode != "on" {
			t.Errorf("Mode = %q, want %q", okRes.Value.Mode, "on")
		}

		_, badErr := ParseAndDecodeCompiled[looseConfig](looseCompiled, []byte(`mode: "bad"`), WithConcrete(false), WithFilename("loose.cue"))
		_, legacyBadErr := ParseAndDecode[looseConfig]([]byte(looseSchema), []byte(`mode: "bad"`), "#Loose", WithConcrete(false), WithFilename("loose.cue"))
		if badErr == nil || legacyBadErr == nil || badErr.Error() != legacyBadErr.Error() {
			t.Fatalf("invalid non-concrete error mismatch: compiled=%v legacy=%v", badErr, legacyBadErr)
		}
		if !strings.Contains(badErr.Error(), "loose.cue:") || !strings.Contains(badErr.Error(), "#Loose.mode:") {
			t.Fatalf("invalid non-concrete error = %v, want loose.cue and #Loose.mode", badErr)
		}
	})

	t.Run("stdlib strings import constraints work across parses", func(t *testing.T) {
		t.Parallel()

		const maxRunesSchema = `
import "strings"

#Bounded: {
	name: string & strings.MaxRunes(8)
}
`
		type bounded struct {
			Name string `json:"name"`
		}

		boundedSchema, compileErr := CompileSchema([]byte(maxRunesSchema), "#Bounded")
		if compileErr != nil {
			t.Fatalf("CompileSchema() error = %v", compileErr)
		}

		okRes, okErr := ParseAndDecodeCompiled[bounded](boundedSchema, []byte(`name: "eightchr"`), WithFilename("ok.cue"))
		if okErr != nil {
			t.Fatalf("expected valid 8-rune input to pass, got %v", okErr)
		}
		if okRes.Value.Name != "eightchr" {
			t.Errorf("Name = %q, want %q", okRes.Value.Name, "eightchr")
		}

		_, badErr := ParseAndDecodeCompiled[bounded](boundedSchema, []byte(`name: "ninechars"`), WithFilename("too-long.cue"))
		if badErr == nil {
			t.Fatal("expected 9-rune input to fail validation")
		}
		if !strings.Contains(badErr.Error(), "too-long.cue:") {
			t.Errorf("error = %v, want filename %q", badErr, "too-long.cue:")
		}
	})
}

func TestParseAndDecodeCompiledConcurrent(t *testing.T) {
	t.Parallel()

	const concurrentSchema = `
import "strings"

#ConcurrentDoc: {
	name:  string & strings.MaxRunes(32)
	count: int
}
`
	type concurrentDoc struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	compiled, err := CompileSchema([]byte(concurrentSchema), "#ConcurrentDoc")
	if err != nil {
		t.Fatalf("CompileSchema() error = %v", err)
	}

	const (
		goroutines = 16
		iterations = 50
	)

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := range goroutines {
		wg.Go(func() {
			for i := range iterations {
				switch i % 4 {
				case 0:
					wantName := fmt.Sprintf("worker-%d-item-%d", g, i)
					data := fmt.Appendf(nil, "name: %q\ncount: %d\n", wantName, i)
					res, parseErr := ParseAndDecodeCompiled[concurrentDoc](compiled, data, WithFilename("valid.cue"))
					if parseErr != nil {
						errCh <- fmt.Errorf("valid case (%d,%d): %w", g, i, parseErr)
						return
					}
					if res.Value.Name != wantName || res.Value.Count != i {
						errCh <- fmt.Errorf("valid case (%d,%d): got %+v, want {%s %d}", g, i, *res.Value, wantName, i)
						return
					}
				case 1:
					data := fmt.Appendf(nil, "name: %q\ncount: %d\n", strings.Repeat("x", 40), i)
					if _, parseErr := ParseAndDecodeCompiled[concurrentDoc](compiled, data, WithFilename("too-long.cue")); parseErr == nil {
						errCh <- fmt.Errorf("constraint violation (%d,%d) unexpectedly succeeded", g, i)
						return
					}
				case 2:
					if _, parseErr := ParseAndDecodeCompiled[concurrentDoc](compiled, []byte(`name: [`), WithFilename("syntax.cue")); parseErr == nil {
						errCh <- fmt.Errorf("syntax error (%d,%d) unexpectedly succeeded", g, i)
						return
					}
				case 3:
					data := fmt.Appendf(nil, "import \"strings\"\nimport \"list\"\nname: strings.ToLower(\"W%d_%d\")\ncount: list.Sum([1, %d])\n", g, i, i)
					res, parseErr := ParseAndDecodeCompiled[concurrentDoc](compiled, data, WithFilename("user-imports.cue"))
					if parseErr != nil {
						errCh <- fmt.Errorf("user-imports case (%d,%d): %w", g, i, parseErr)
						return
					}
					wantName := fmt.Sprintf("w%d_%d", g, i)
					if res.Value.Name != wantName || res.Value.Count != i+1 {
						errCh <- fmt.Errorf("user-imports case (%d,%d): got %+v, want {%s %d}", g, i, *res.Value, wantName, i+1)
						return
					}
				}
			}
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

// Not parallel: measures process-wide HeapAlloc growth across many parses.
func TestParseAndDecodeCompiledBoundedMemory(t *testing.T) { //nolint:paralleltest // Measures process-wide HeapAlloc growth across sequential parses.
	const (
		boundedSchema = `
import "strings"

#LeakCheck: {
	name:    string & strings.MaxRunes(64)
	payload: string
}
`
		iterations         = 1500
		payloadSizeBytes   = 1024
		maxHeapGrowthBytes = 16 << 20
	)
	type leakCheck struct {
		Name    string `json:"name"`
		Payload string `json:"payload"`
	}

	compiled, err := CompileSchema([]byte(boundedSchema), "#LeakCheck")
	if err != nil {
		t.Fatalf("CompileSchema() error = %v", err)
	}

	padding := strings.Repeat("a", payloadSizeBytes)
	runBatch := func(offset int) {
		for i := range iterations {
			idx := offset + i
			data := fmt.Appendf(nil, "name: \"entry-%d\"\npayload: \"%06d-%s\"\n", idx, idx, padding)
			filename := fmt.Sprintf("file-%d.cue", idx)
			if _, parseErr := ParseAndDecodeCompiled[leakCheck](compiled, data, WithFilename(filename)); parseErr != nil {
				t.Fatalf("ParseAndDecodeCompiled(%d) error = %v", idx, parseErr)
			}
		}
	}

	// Warm up compiled schema and runtime allocations before measuring steady-state retention.
	runBatch(0)
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	runBatch(iterations)
	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if after.HeapAlloc > before.HeapAlloc && after.HeapAlloc-before.HeapAlloc > maxHeapGrowthBytes {
		t.Fatalf("live heap grew by %d bytes after %d parses (limit %d bytes)", after.HeapAlloc-before.HeapAlloc, iterations, maxHeapGrowthBytes)
	}
}
