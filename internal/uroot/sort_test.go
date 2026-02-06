// SPDX-License-Identifier: MPL-2.0

//nolint:goconst // Test files naturally repeat string literals across test functions
package uroot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSortCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newSortCommand()
	if got := cmd.Name(); got != "sort" {
		t.Errorf("Name() = %q, want %q", got, "sort")
	}
}

func TestSortCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newSortCommand()
	flags := cmd.SupportedFlags()

	// Should include common flags
	expectedFlags := map[string]bool{"r": false, "n": false, "u": false}
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

func TestSortCommand_Run_Basic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "cherry\napple\nbanana\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}

	// Should be sorted alphabetically
	if lines[0] != "apple" || lines[1] != "banana" || lines[2] != "cherry" {
		t.Errorf("lines not sorted correctly: %v", lines)
	}
}

func TestSortCommand_Run_Reverse(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple\nbanana\ncherry\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", "-r", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// Should be reverse sorted
	if lines[0] != "cherry" || lines[1] != "banana" || lines[2] != "apple" {
		t.Errorf("lines not reverse sorted: %v", lines)
	}
}

func TestSortCommand_Run_Numeric(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "10\n2\n1\n20\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", "-n", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// Should be numerically sorted
	if lines[0] != "1" || lines[1] != "2" || lines[2] != "10" || lines[3] != "20" {
		t.Errorf("lines not numerically sorted: %v", lines)
	}
}

func TestSortCommand_Run_Unique(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple\nbanana\napple\ncherry\nbanana\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", "-u", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d unique lines, want 3", len(lines))
	}
}

func TestSortCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	stdinContent := "cherry\napple\nbanana\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if lines[0] != "apple" {
		t.Errorf("first line = %q, want %q", lines[0], "apple")
	}
}

func TestSortCommand_Run_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("cherry\napple\n"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("banana\ndate\n"), 0o644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("got %d lines, want 4", len(lines))
	}

	// All lines from both files should be sorted together
	if lines[0] != "apple" || lines[3] != "date" {
		t.Errorf("lines from multiple files not sorted correctly: %v", lines)
	}
}

func TestSortCommand_Run_FileNotFound(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] sort:") {
		t.Errorf("error should have [uroot] sort: prefix, got: %v", err)
	}
}

func TestSortCommand_Run_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	if err := os.WriteFile(testFile, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty file, got %q", stdout.String())
	}
}

func TestSortCommand_Run_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "Banana\napple\nCherry\n"
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newSortCommand()
	err := cmd.Run(ctx, []string{"sort", "-f", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// Should be case-insensitive sorted
	if lines[0] != "apple" {
		t.Errorf("first line = %q, want %q (case-insensitive)", lines[0], "apple")
	}
}
