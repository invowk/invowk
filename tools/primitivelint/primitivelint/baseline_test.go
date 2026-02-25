// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBaseline(t *testing.T) {
	t.Parallel()

	t.Run("empty path returns empty baseline", func(t *testing.T) {
		t.Parallel()
		bl, err := loadBaseline("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bl.Count() != 0 {
			t.Errorf("expected 0 entries, got %d", bl.Count())
		}
		// Empty baseline should match nothing.
		if bl.Contains(CategoryPrimitive, "anything") {
			t.Error("empty baseline should not match anything")
		}
	})

	t.Run("nonexistent file returns empty baseline", func(t *testing.T) {
		t.Parallel()
		bl, err := loadBaseline("/nonexistent/path/baseline.toml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bl.Count() != 0 {
			t.Errorf("expected 0 entries, got %d", bl.Count())
		}
	})

	t.Run("valid TOML parses correctly", func(t *testing.T) {
		t.Parallel()
		content := `
[primitive]
messages = [
    "struct field pkg.Foo.Bar uses primitive type string",
    "parameter \"name\" of pkg.Func uses primitive type string",
]

[missing-isvalid]
messages = [
    "named type pkg.MyType has no IsValid() method",
]

[missing-constructor]
messages = [
    "exported struct pkg.Config has no NewConfig() constructor",
]
`
		path := writeTempFile(t, "baseline.toml", content)
		bl, err := loadBaseline(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if bl.Count() != 4 {
			t.Errorf("expected 4 entries, got %d", bl.Count())
		}
	})

	t.Run("invalid TOML returns error", func(t *testing.T) {
		t.Parallel()
		path := writeTempFile(t, "bad.toml", "[[invalid toml")
		_, err := loadBaseline(path)
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
	})
}

func TestBaselineContains(t *testing.T) {
	t.Parallel()

	content := `
[primitive]
messages = [
    "struct field pkg.Foo.Bar uses primitive type string",
    "return value of pkg.Func uses primitive type int",
]

[missing-isvalid]
messages = [
    "named type pkg.MyType has no IsValid() method",
]

[missing-stringer]
messages = [
    "named type pkg.MyType has no String() method",
]

[missing-constructor]
messages = [
    "exported struct pkg.Config has no NewConfig() constructor",
]

[wrong-constructor-sig]
messages = [
    "constructor NewFoo() for pkg.Foo returns Bar, expected Foo",
]

[missing-func-options]
messages = [
    "constructor NewBig() for pkg.Big has 5 non-option parameters; consider using functional options",
]

[missing-immutability]
messages = [
    "struct pkg.Svc has NewSvc() constructor but field Addr is exported",
]
`
	path := writeTempFile(t, "baseline.toml", content)
	bl, err := loadBaseline(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		category string
		message  string
		want     bool
	}{
		{
			name:     "primitive match",
			category: CategoryPrimitive,
			message:  "struct field pkg.Foo.Bar uses primitive type string",
			want:     true,
		},
		{
			name:     "primitive no match",
			category: CategoryPrimitive,
			message:  "struct field pkg.Foo.Baz uses primitive type string",
			want:     false,
		},
		{
			name:     "wrong category",
			category: CategoryMissingIsValid,
			message:  "struct field pkg.Foo.Bar uses primitive type string",
			want:     false,
		},
		{
			name:     "missing-isvalid match",
			category: CategoryMissingIsValid,
			message:  "named type pkg.MyType has no IsValid() method",
			want:     true,
		},
		{
			name:     "missing-stringer match",
			category: CategoryMissingStringer,
			message:  "named type pkg.MyType has no String() method",
			want:     true,
		},
		{
			name:     "missing-constructor match",
			category: CategoryMissingConstructor,
			message:  "exported struct pkg.Config has no NewConfig() constructor",
			want:     true,
		},
		{
			name:     "wrong-constructor-sig match",
			category: CategoryWrongConstructorSig,
			message:  "constructor NewFoo() for pkg.Foo returns Bar, expected Foo",
			want:     true,
		},
		{
			name:     "missing-func-options match",
			category: CategoryMissingFuncOptions,
			message:  "constructor NewBig() for pkg.Big has 5 non-option parameters; consider using functional options",
			want:     true,
		},
		{
			name:     "missing-immutability match",
			category: CategoryMissingImmutability,
			message:  "struct pkg.Svc has NewSvc() constructor but field Addr is exported",
			want:     true,
		},
		{
			name:     "nil baseline",
			category: CategoryPrimitive,
			message:  "anything",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var target *BaselineConfig
			if tt.name != "nil baseline" {
				target = bl
			}
			got := target.Contains(tt.category, tt.message)
			if got != tt.want {
				t.Errorf("Contains(%q, %q) = %v, want %v", tt.category, tt.message, got, tt.want)
			}
		})
	}
}

func TestWriteBaseline(t *testing.T) {
	t.Parallel()

	t.Run("writes sorted TOML with header", func(t *testing.T) {
		t.Parallel()
		outPath := filepath.Join(t.TempDir(), "baseline.toml")
		findings := map[string][]string{
			CategoryPrimitive: {
				"struct field pkg.Foo.Baz uses primitive type int",
				"struct field pkg.Foo.Bar uses primitive type string",
			},
			CategoryMissingConstructor: {
				"exported struct pkg.Config has no NewConfig() constructor",
			},
		}

		if err := WriteBaseline(outPath, findings); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading output: %v", err)
		}
		content := string(data)

		// Verify SPDX header.
		if !containsStr(content, "SPDX-License-Identifier: MPL-2.0") {
			t.Error("missing SPDX header")
		}

		// Verify total count.
		if !containsStr(content, "Total: 3 findings") {
			t.Error("missing or wrong total count")
		}

		// Verify sorting — Bar should come before Baz.
		barIdx := indexOfStr(content, "pkg.Foo.Bar")
		bazIdx := indexOfStr(content, "pkg.Foo.Baz")
		if barIdx < 0 || bazIdx < 0 || barIdx > bazIdx {
			t.Error("primitive messages not sorted alphabetically")
		}
	})

	t.Run("empty categories omitted", func(t *testing.T) {
		t.Parallel()
		outPath := filepath.Join(t.TempDir(), "baseline.toml")
		findings := map[string][]string{
			CategoryMissingIsValid: {
				"named type pkg.MyType has no IsValid() method",
			},
		}

		if err := WriteBaseline(outPath, findings); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading output: %v", err)
		}
		content := string(data)

		if containsStr(content, "[primitive]") {
			t.Error("empty [primitive] section should be omitted")
		}
		if !containsStr(content, "[missing-isvalid]") {
			t.Error("non-empty [missing-isvalid] section should be present")
		}
	})
}

func TestBaselineRoundTrip(t *testing.T) {
	t.Parallel()

	findings := map[string][]string{
		CategoryPrimitive: {
			"struct field pkg.Foo.Bar uses primitive type string",
			`parameter "name" of pkg.Func uses primitive type string`,
		},
		CategoryMissingIsValid: {
			"named type pkg.MyType has no IsValid() method",
		},
		CategoryMissingStringer: {
			"named type pkg.MyType has no String() method",
		},
		CategoryMissingConstructor: {
			"exported struct pkg.Config has no NewConfig() constructor",
		},
		CategoryWrongConstructorSig: {
			"constructor NewFoo() for pkg.Foo returns Bar, expected Foo",
		},
		CategoryMissingFuncOptions: {
			"constructor NewBig() for pkg.Big has 5 non-option parameters; consider using functional options",
		},
		CategoryMissingImmutability: {
			"struct pkg.Svc has NewSvc() constructor but field Addr is exported",
		},
	}

	outPath := filepath.Join(t.TempDir(), "baseline.toml")
	if err := WriteBaseline(outPath, findings); err != nil {
		t.Fatalf("write error: %v", err)
	}

	bl, err := loadBaseline(outPath)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	// Verify all original findings are present.
	for cat, msgs := range findings {
		for _, msg := range msgs {
			if !bl.Contains(cat, msg) {
				t.Errorf("round-trip lost: category=%q, message=%q", cat, msg)
			}
		}
	}

	// Verify count matches.
	total := 0
	for _, msgs := range findings {
		total += len(msgs)
	}
	if bl.Count() != total {
		t.Errorf("count mismatch: got %d, want %d", bl.Count(), total)
	}
}

func TestQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", `"hello"`},
		{"with quotes", `say "hi"`, `"say \"hi\""`},
		{"with backslash", `path\to\file`, `"path\\to\\file"`},
		{"with newline", "line1\nline2", `"line1\nline2"`},
		{"with tab", "col1\tcol2", `"col1\tcol2"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := quote(tt.input)
			if got != tt.want {
				t.Errorf("quote(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

// writeTempFile creates a temporary file with the given content and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

func indexOfStr(s, substr string) int {
	return strings.Index(s, substr)
}
