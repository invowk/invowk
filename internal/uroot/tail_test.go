// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newTailCommand()
	if got := cmd.Name(); got != "tail" {
		t.Errorf("Name() = %q, want %q", got, "tail")
	}
}

func TestTailCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newTailCommand()
	flags := cmd.SupportedFlags()

	// Should include -n flag
	found := false
	for _, f := range flags {
		if f.Name == "n" {
			found = true
			if !f.TakesValue {
				t.Error("-n flag should take a value")
			}
			break
		}
	}
	if !found {
		t.Error("SupportedFlags() should include -n flag")
	}
}

func TestTailCommand_Run_DefaultLines(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with 15 lines
	var content strings.Builder
	for i := 1; i <= 15; i++ {
		content.WriteString("line " + itoa(i) + "\n")
	}
	if err := os.WriteFile(testFile, []byte(content.String()), 0o644); err != nil {
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

	cmd := newTailCommand()
	err := cmd.Run(ctx, []string{"tail", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Default is 10 lines
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 10 {
		t.Errorf("got %d lines, want 10", len(lines))
	}

	// First line should be "line 6" (last 10 of 15)
	if lines[0] != "line 6" {
		t.Errorf("first line = %q, want %q", lines[0], "line 6")
	}

	// Last line should be "line 15"
	if lines[9] != "line 15" {
		t.Errorf("last line = %q, want %q", lines[9], "line 15")
	}
}

func TestTailCommand_Run_CustomLines(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with 10 lines
	var content strings.Builder
	for i := 1; i <= 10; i++ {
		content.WriteString("line " + itoa(i) + "\n")
	}
	if err := os.WriteFile(testFile, []byte(content.String()), 0o644); err != nil {
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

	cmd := newTailCommand()
	err := cmd.Run(ctx, []string{"tail", "-n", "3", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}

	// Should be last 3 lines: line 8, line 9, line 10
	if lines[0] != "line 8" {
		t.Errorf("first line = %q, want %q", lines[0], "line 8")
	}
}

func TestTailCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	// Create stdin with 5 lines
	var stdinContent strings.Builder
	for i := 1; i <= 5; i++ {
		stdinContent.WriteString("stdin line " + itoa(i) + "\n")
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent.String()),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTailCommand()
	err := cmd.Run(ctx, []string{"tail", "-n", "2"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}

	// Should be last 2 lines
	if lines[0] != "stdin line 4" {
		t.Errorf("first line = %q, want %q", lines[0], "stdin line 4")
	}
	if lines[1] != "stdin line 5" {
		t.Errorf("second line = %q, want %q", lines[1], "stdin line 5")
	}
}

func TestTailCommand_Run_FewerLinesThanRequested(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with only 3 lines
	content := "line 1\nline 2\nline 3\n"
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

	cmd := newTailCommand()
	// Request 10 lines but file only has 3
	err := cmd.Run(ctx, []string{"tail", "-n", "10", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
}

func TestTailCommand_Run_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("a1\na2\na3\n"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("b1\nb2\nb3\n"), 0o644); err != nil {
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

	cmd := newTailCommand()
	err := cmd.Run(ctx, []string{"tail", "-n", "2", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Should include file headers for multiple files
	if !strings.Contains(output, "==> ") || !strings.Contains(output, " <==") {
		t.Error("multiple files should include headers")
	}

	// Should have last lines from both files
	if !strings.Contains(output, "a2") || !strings.Contains(output, "b2") {
		t.Error("output should contain last lines from both files")
	}
}

func TestTailCommand_Run_FileNotFound(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTailCommand()
	err := cmd.Run(ctx, []string{"tail", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] tail:") {
		t.Errorf("error should have [uroot] tail: prefix, got: %v", err)
	}
}

func TestTailCommand_Run_EmptyFile(t *testing.T) {
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

	cmd := newTailCommand()
	err := cmd.Run(ctx, []string{"tail", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty file, got %q", stdout.String())
	}
}

func TestTailCommand_Run_PlusN(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with 5 lines
	var content strings.Builder
	for i := 1; i <= 5; i++ {
		content.WriteString("line " + itoa(i) + "\n")
	}
	if err := os.WriteFile(testFile, []byte(content.String()), 0o644); err != nil {
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

	cmd := newTailCommand()
	// +3 means starting from line 3
	err := cmd.Run(ctx, []string{"tail", "-n", "+3", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3 (lines 3, 4, 5)", len(lines))
	}

	if lines[0] != "line 3" {
		t.Errorf("first line = %q, want %q", lines[0], "line 3")
	}
}
