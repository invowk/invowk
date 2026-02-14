// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvFile_BasicKeyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "simple key value",
			content:  "FOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "multiple key values",
			content:  "FOO=bar\nBAZ=qux",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:     "empty value",
			content:  "EMPTY=",
			expected: map[string]string{"EMPTY": ""},
		},
		{
			name:     "value with equals sign",
			content:  "URL=https://example.com?foo=bar",
			expected: map[string]string{"URL": "https://example.com?foo=bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := make(map[string]string)
			err := ParseEnvFile(env, []byte(tt.content), "test.env")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, v := range tt.expected {
				if env[k] != v {
					t.Errorf("expected %s=%q, got %s=%q", k, v, k, env[k])
				}
			}
		})
	}
}

func TestParseEnvFile_Comments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "comment line",
			content:  "# This is a comment\nFOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "inline comment unquoted",
			content:  "FOO=bar # this is an inline comment",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "no inline comment without space",
			content:  "FOO=bar#not-a-comment",
			expected: map[string]string{"FOO": "bar#not-a-comment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := make(map[string]string)
			err := ParseEnvFile(env, []byte(tt.content), "test.env")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, v := range tt.expected {
				if env[k] != v {
					t.Errorf("expected %s=%q, got %s=%q", k, v, k, env[k])
				}
			}
		})
	}
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "double quoted",
			content:  `FOO="hello world"`,
			expected: map[string]string{"FOO": "hello world"},
		},
		{
			name:     "single quoted",
			content:  `FOO='hello world'`,
			expected: map[string]string{"FOO": "hello world"},
		},
		{
			name:     "double quoted with escape sequences",
			content:  `FOO="hello\nworld"`,
			expected: map[string]string{"FOO": "hello\nworld"},
		},
		{
			name:     "single quoted preserves escapes",
			content:  `FOO='hello\nworld'`,
			expected: map[string]string{"FOO": `hello\nworld`},
		},
		{
			name:     "double quoted with escaped quote",
			content:  `FOO="hello \"world\""`,
			expected: map[string]string{"FOO": `hello "world"`},
		},
		{
			name:     "double quoted with escaped backslash",
			content:  `FOO="path\\to\\file"`,
			expected: map[string]string{"FOO": `path\to\file`},
		},
		{
			name:     "double quoted with tab",
			content:  `FOO="hello\tworld"`,
			expected: map[string]string{"FOO": "hello\tworld"},
		},
		{
			name:     "double quoted with carriage return",
			content:  `FOO="hello\rworld"`,
			expected: map[string]string{"FOO": "hello\rworld"},
		},
		{
			name:     "double quoted with dollar escape",
			content:  `FOO="price is \$100"`,
			expected: map[string]string{"FOO": "price is $100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := make(map[string]string)
			err := ParseEnvFile(env, []byte(tt.content), "test.env")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, v := range tt.expected {
				if env[k] != v {
					t.Errorf("expected %s=%q, got %s=%q", k, v, k, env[k])
				}
			}
		})
	}
}

func TestParseEnvFile_ExportPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "export prefix",
			content:  "export FOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "export prefix with quotes",
			content:  `export FOO="bar"`,
			expected: map[string]string{"FOO": "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := make(map[string]string)
			err := ParseEnvFile(env, []byte(tt.content), "test.env")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, v := range tt.expected {
				if env[k] != v {
					t.Errorf("expected %s=%q, got %s=%q", k, v, k, env[k])
				}
			}
		})
	}
}

func TestParseEnvFile_Whitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "leading whitespace",
			content:  "  FOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "trailing whitespace",
			content:  "FOO=bar  ",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "whitespace around equals",
			content:  "FOO = bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "empty lines ignored",
			content:  "FOO=bar\n\n\nBAZ=qux",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:     "windows line endings",
			content:  "FOO=bar\r\nBAZ=qux\r\n",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := make(map[string]string)
			err := ParseEnvFile(env, []byte(tt.content), "test.env")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, v := range tt.expected {
				if env[k] != v {
					t.Errorf("expected %s=%q, got %s=%q", k, v, k, env[k])
				}
			}
		})
	}
}

func TestParseEnvFile_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name:    "missing equals sign",
			content: "FOOBAR",
			errMsg:  "invalid format",
		},
		{
			name:    "empty variable name",
			content: "=value",
			errMsg:  "empty variable name",
		},
		{
			name:    "unterminated double quote",
			content: `FOO="hello world`,
			errMsg:  "unterminated double quote",
		},
		{
			name:    "unterminated single quote",
			content: `FOO='hello world`,
			errMsg:  "unterminated single quote",
		},
		{
			name:    "double quote only opening",
			content: `BAR="`,
			errMsg:  "unterminated double quote",
		},
		{
			name:    "single quote only opening",
			content: `BAR='`,
			errMsg:  "unterminated single quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := make(map[string]string)
			err := ParseEnvFile(env, []byte(tt.content), "test.env")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestParseEnvFile_Precedence(t *testing.T) {
	t.Parallel()

	// Later values override earlier values
	env := make(map[string]string)
	content := "FOO=first\nFOO=second"

	err := ParseEnvFile(env, []byte(content), "test.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["FOO"] != "second" {
		t.Errorf("expected FOO=second, got FOO=%s", env["FOO"])
	}
}

func TestLoadEnvFile_RelativePath(t *testing.T) {
	t.Parallel()

	// Create a temp directory with a .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")
	if err := os.WriteFile(envFile, []byte("FOO=bar\nBAZ=qux"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	env := make(map[string]string)
	err := LoadEnvFile(env, "test.env", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got BAZ=%s", env["BAZ"])
	}
}

func TestLoadEnvFile_AbsolutePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "absolute.env")
	if err := os.WriteFile(envFile, []byte("ABSOLUTE=true"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	env := make(map[string]string)
	err := LoadEnvFile(env, envFile, "/some/other/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["ABSOLUTE"] != "true" {
		t.Errorf("expected ABSOLUTE=true, got ABSOLUTE=%s", env["ABSOLUTE"])
	}
}

func TestLoadEnvFile_OptionalMissing(t *testing.T) {
	t.Parallel()

	env := make(map[string]string)
	// Optional file (suffixed with ?) should not error when missing
	err := LoadEnvFile(env, "nonexistent.env?", "/nonexistent/path")
	if err != nil {
		t.Errorf("expected no error for optional missing file, got: %v", err)
	}
}

func TestLoadEnvFile_RequiredMissing(t *testing.T) {
	t.Parallel()

	env := make(map[string]string)
	// Required file should error when missing
	err := LoadEnvFile(env, "nonexistent.env", "/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing required file, got nil")
	}
}

func TestLoadEnvFile_OptionalExists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "optional.env")
	if err := os.WriteFile(envFile, []byte("OPTIONAL=yes"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	env := make(map[string]string)
	// Optional file should be loaded if it exists
	err := LoadEnvFile(env, "optional.env?", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["OPTIONAL"] != "yes" {
		t.Errorf("expected OPTIONAL=yes, got OPTIONAL=%s", env["OPTIONAL"])
	}
}

func TestLoadEnvFile_ForwardSlashPath(t *testing.T) {
	t.Parallel()

	// Test that forward slashes work on all platforms
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "config")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	envFile := filepath.Join(subDir, "app.env")
	if err := os.WriteFile(envFile, []byte("SUBDIR=true"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	env := make(map[string]string)
	// Use forward slashes (common in CUE files)
	err := LoadEnvFile(env, "config/app.env", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["SUBDIR"] != "true" {
		t.Errorf("expected SUBDIR=true, got SUBDIR=%s", env["SUBDIR"])
	}
}

func TestLoadEnvFileFromCwd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create .env file in temp directory
	if writeErr := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("CWD_VAR=hello"), 0o644); writeErr != nil {
		t.Fatalf("failed to create test file: %v", writeErr)
	}

	env := make(map[string]string)
	// Pass cwd explicitly instead of using MustChdir
	if err := LoadEnvFileFromCwd(env, ".env", tmpDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["CWD_VAR"] != "hello" {
		t.Errorf("expected CWD_VAR=hello, got CWD_VAR=%s", env["CWD_VAR"])
	}
}

func TestLoadEnvFileFromCwd_OptionalMissing(t *testing.T) {
	t.Parallel()

	env := make(map[string]string)
	// Optional file should not error when missing — pass a known directory as cwd
	err := LoadEnvFileFromCwd(env, "nonexistent.env?", t.TempDir())
	if err != nil {
		t.Errorf("expected no error for optional missing file, got: %v", err)
	}
}

func TestParseEnvFile_MergesIntoExisting(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"EXISTING": "value",
		"OVERRIDE": "old",
	}

	content := "NEW=added\nOVERRIDE=new"
	err := ParseEnvFile(env, []byte(content), "test.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["EXISTING"] != "value" {
		t.Errorf("expected EXISTING=value, got EXISTING=%s", env["EXISTING"])
	}
	if env["NEW"] != "added" {
		t.Errorf("expected NEW=added, got NEW=%s", env["NEW"])
	}
	if env["OVERRIDE"] != "new" {
		t.Errorf("expected OVERRIDE=new, got OVERRIDE=%s", env["OVERRIDE"])
	}
}
