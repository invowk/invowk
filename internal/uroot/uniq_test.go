// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniqCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newUniqCommand()
	if got := cmd.Name(); got != "uniq" {
		t.Errorf("Name() = %q, want %q", got, "uniq")
	}
}

func TestUniqCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newUniqCommand()
	flags := cmd.SupportedFlags()

	// Should include common flags
	expectedFlags := map[string]bool{"c": false, "d": false, "u": false}
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

func TestUniqCommand_Run_Basic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Adjacent duplicates should be removed
	content := "apple\napple\nbanana\nbanana\nbanana\ncherry\n"
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d unique lines, want 3", len(lines))
	}

	if lines[0] != "apple" || lines[1] != "banana" || lines[2] != "cherry" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestUniqCommand_Run_NonAdjacent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Non-adjacent duplicates are NOT removed (uniq only checks adjacent lines)
	content := "apple\nbanana\napple\n"
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// All 3 lines should appear since they are not adjacent duplicates
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3 (non-adjacent duplicates preserved)", len(lines))
	}
}

func TestUniqCommand_Run_Count(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple\napple\napple\nbanana\ncherry\ncherry\n"
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", "-c", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Should have counts prefixed
	if !strings.Contains(output, "3") { // apple appears 3 times
		t.Errorf("output should contain count 3 for apple, got: %q", output)
	}
	if !strings.Contains(output, "1") && !strings.Contains(output, " 1 ") { // banana appears once
		t.Errorf("output should contain count for banana, got: %q", output)
	}
}

func TestUniqCommand_Run_DuplicatesOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple\napple\nbanana\ncherry\ncherry\n"
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", "-d", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// Only duplicated lines (apple and cherry)
	if len(lines) != 2 {
		t.Errorf("got %d duplicated lines, want 2", len(lines))
	}
}

func TestUniqCommand_Run_UniqueOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "apple\napple\nbanana\ncherry\ncherry\n"
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", "-u", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	// Only unique line (banana)
	if output != "banana" {
		t.Errorf("output = %q, want %q", output, "banana")
	}
}

func TestUniqCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	stdinContent := "apple\napple\nbanana\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}
}

func TestUniqCommand_Run_FileNotFound(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] uniq:") {
		t.Errorf("error should have [uroot] uniq: prefix, got: %v", err)
	}
}

func TestUniqCommand_Run_EmptyFile(t *testing.T) {
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty file, got %q", stdout.String())
	}
}

func TestUniqCommand_Run_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "Apple\napple\nAPPLE\nbanana\n"
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

	cmd := newUniqCommand()
	err := cmd.Run(ctx, []string{"uniq", "-i", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	// With -i, all "apple" variants should collapse to one
	if len(lines) != 2 {
		t.Errorf("got %d lines with -i, want 2", len(lines))
	}
}
