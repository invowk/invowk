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

func TestHeadCommand_Name(t *testing.T) {
	cmd := newHeadCommand()
	if got := cmd.Name(); got != "head" {
		t.Errorf("Name() = %q, want %q", got, "head")
	}
}

func TestHeadCommand_SupportedFlags(t *testing.T) {
	cmd := newHeadCommand()
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

func TestHeadCommand_Run_DefaultLines(t *testing.T) {
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
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newHeadCommand()
	err := cmd.Run(ctx, []string{"head", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Default is 10 lines
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 10 {
		t.Errorf("got %d lines, want 10", len(lines))
	}

	// First line should be "line 1"
	if lines[0] != "line 1" {
		t.Errorf("first line = %q, want %q", lines[0], "line 1")
	}

	// Last line should be "line 10"
	if lines[9] != "line 10" {
		t.Errorf("last line = %q, want %q", lines[9], "line 10")
	}
}

func TestHeadCommand_Run_CustomLines(t *testing.T) {
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
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newHeadCommand()
	err := cmd.Run(ctx, []string{"head", "-n", "3", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
}

func TestHeadCommand_Run_Stdin(t *testing.T) {
	// Create stdin with 5 lines
	var stdinContent strings.Builder
	for i := 1; i <= 5; i++ {
		stdinContent.WriteString("stdin line " + itoa(i) + "\n")
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent.String()),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newHeadCommand()
	err := cmd.Run(ctx, []string{"head", "-n", "2"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}

	if lines[0] != "stdin line 1" {
		t.Errorf("first line = %q, want %q", lines[0], "stdin line 1")
	}
}

func TestHeadCommand_Run_FewerLinesThanRequested(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with only 3 lines
	content := "line 1\nline 2\nline 3\n"
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

	cmd := newHeadCommand()
	// Request 10 lines but file only has 3
	err := cmd.Run(ctx, []string{"head", "-n", "10", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
}

func TestHeadCommand_Run_MultipleFiles(t *testing.T) {
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
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newHeadCommand()
	err := cmd.Run(ctx, []string{"head", "-n", "2", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Should include file headers for multiple files
	if !strings.Contains(output, "==> ") || !strings.Contains(output, " <==") {
		t.Error("multiple files should include headers")
	}

	// Should have lines from both files
	if !strings.Contains(output, "a1") || !strings.Contains(output, "b1") {
		t.Error("output should contain lines from both files")
	}
}

func TestHeadCommand_Run_FileNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newHeadCommand()
	err := cmd.Run(ctx, []string{"head", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] head:") {
		t.Errorf("error should have [uroot] head: prefix, got: %v", err)
	}
}

func TestHeadCommand_Run_EmptyFile(t *testing.T) {
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

	cmd := newHeadCommand()
	err := cmd.Run(ctx, []string{"head", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty file, got %q", stdout.String())
	}
}

// itoa is a simple integer to string conversion for test use.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var s string
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
