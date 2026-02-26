// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCutCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newCutCommand()
	if got := cmd.Name(); got != "cut" {
		t.Errorf("Name() = %q, want %q", got, "cut")
	}
}

func TestCutCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newCutCommand()
	flags := cmd.SupportedFlags()

	// Should include -d, -f, -c flags
	expectedFlags := map[string]bool{"d": false, "f": false, "c": false}
	for _, f := range flags {
		if _, exists := expectedFlags[f.Name]; exists {
			expectedFlags[f.Name] = true
		}
	}

	for name, found := range expectedFlags {
		if !found {
			t.Errorf("SupportedFlags() should include -%s flag", name)
		}
	}
}

func TestCutCommand_Run_Fields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple:red:fruit\nbanana:yellow:fruit\ncarrot:orange:vegetable\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-d", ":", "-f", "1", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}

	if lines[0] != "apple" || lines[1] != "banana" || lines[2] != "carrot" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestCutCommand_Run_MultipleFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple:red:fruit\nbanana:yellow:fruit\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-d", ":", "-f", "1,3", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// Fields 1 and 3 separated by delimiter
	if lines[0] != "apple:fruit" {
		t.Errorf("first line = %q, want %q", lines[0], "apple:fruit")
	}
}

func TestCutCommand_Run_FieldRange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "a:b:c:d:e\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-d", ":", "-f", "2-4", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "b:c:d" {
		t.Errorf("output = %q, want %q", output, "b:c:d")
	}
}

func TestCutCommand_Run_Characters(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "hello world\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-c", "1-5", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "hello" {
		t.Errorf("output = %q, want %q", output, "hello")
	}
}

func TestCutCommand_Run_TabDelimiter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple\tred\nbanana\tyellow\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	// Default delimiter is tab
	err := cmd.Run(ctx, []string{"cut", "-f", "2", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if lines[0] != "red" || lines[1] != "yellow" {
		t.Errorf("unexpected output with tab delimiter: %v", lines)
	}
}

func TestCutCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	stdinContent := "apple:red\nbanana:yellow\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-d", ":", "-f", "2"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if lines[0] != "red" || lines[1] != "yellow" {
		t.Errorf("unexpected output from stdin: %v", lines)
	}
}

func TestCutCommand_Run_OnlyDelimited(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Mix of lines with and without delimiter
	content := "apple:red\nnodelimieter\nbanana:yellow\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	// -s suppresses lines without delimiter
	err := cmd.Run(ctx, []string{"cut", "-d", ":", "-f", "1", "-s", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// Should only have 2 lines (ones with delimiter)
	if len(lines) != 2 {
		t.Errorf("got %d lines with -s, want 2", len(lines))
	}
}

func TestCutCommand_Run_FileNotFound(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-f", "1", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] cut:") {
		t.Errorf("error should have [uroot] cut: prefix, got: %v", err)
	}
}

func TestCutCommand_Run_NoFieldOrChar(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader("test\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	// Must specify -f or -c
	err := cmd.Run(ctx, []string{"cut"})

	if err == nil {
		t.Fatal("Run() should return error when no -f or -c specified")
	}
}

func TestCutCommand_Run_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	if err := os.WriteFile(testFile, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCutCommand()
	err := cmd.Run(ctx, []string{"cut", "-f", "1", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty file, got %q", stdout.String())
	}
}
