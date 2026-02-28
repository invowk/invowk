// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWcCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newWcCommand()
	if got := cmd.Name(); got != "wc" {
		t.Errorf("Name() = %q, want %q", got, "wc")
	}
}

func TestWcCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newWcCommand()
	flags := cmd.SupportedFlags()

	// Should include -l, -w, -c flags
	expectedFlags := map[string]bool{"l": false, "w": false, "c": false}
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

func TestWcCommand_Run_AllCounts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file: 3 lines, 6 words, 24 bytes (including newlines)
	content := "hello world\nfoo bar\nbaz qux\n"
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

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Default output should include lines, words, bytes
	// Format: "  3   6  24 test.txt" (numbers vary in padding)
	if !strings.Contains(output, "3") {
		t.Errorf("output should contain line count 3, got: %q", output)
	}
	if !strings.Contains(output, "6") {
		t.Errorf("output should contain word count 6, got: %q", output)
	}
	if !strings.Contains(output, testFile) || !strings.Contains(output, "test.txt") {
		t.Errorf("output should contain filename, got: %q", output)
	}
}

func TestWcCommand_Run_LinesOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3\n"
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

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "-l", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	// Should only have line count and filename
	if !strings.HasPrefix(output, "3") && !strings.Contains(output, " 3 ") {
		t.Errorf("output should start with line count 3, got: %q", output)
	}
}

func TestWcCommand_Run_WordsOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "one two three\nfour five\n"
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

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "-w", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	// Should contain word count 5
	if !strings.Contains(output, "5") {
		t.Errorf("output should contain word count 5, got: %q", output)
	}
}

func TestWcCommand_Run_BytesOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "hello\n" // 6 bytes
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

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "-c", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	// Should contain byte count 6
	if !strings.Contains(output, "6") {
		t.Errorf("output should contain byte count 6, got: %q", output)
	}
}

func TestWcCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	stdinContent := "one two three\nfour five\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "-l"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	// Should have 2 lines
	if !strings.Contains(output, "2") {
		t.Errorf("output should contain line count 2, got: %q", output)
	}
}

func TestWcCommand_Run_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("a b\nc d\n"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("e f g\n"), 0o644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "-l", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Should have total line
	if !strings.Contains(output, "total") {
		t.Error("multiple files should include total line")
	}
	// Total should be 3 (2 + 1)
	if !strings.Contains(output, "3") {
		t.Errorf("total should be 3, output: %q", output)
	}
}

func TestWcCommand_Run_FileNotFound(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] wc:") {
		t.Errorf("error should have [uroot] wc: prefix, got: %v", err)
	}
}

func TestWcCommand_Run_EmptyFile(t *testing.T) {
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

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// All counts should be 0
	if !strings.Contains(output, "0") {
		t.Errorf("empty file should have 0 counts, got: %q", output)
	}
}

func TestWcCommand_Run_CombinedFlags(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "hello world\n" // 1 line, 2 words, 12 bytes
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

	cmd := newWcCommand()
	err := cmd.Run(ctx, []string{"wc", "-lw", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Should have line and word count
	if !strings.Contains(output, "1") || !strings.Contains(output, "2") {
		t.Errorf("output should contain line count 1 and word count 2, got: %q", output)
	}
}
