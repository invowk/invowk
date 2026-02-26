// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newCatCommand()
	if got := cmd.Name(); got != "cat" {
		t.Errorf("Name() = %q, want %q", got, "cat")
	}
}

func TestCatCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newCatCommand()
	flags := cmd.SupportedFlags()

	// Should return at least basic flags
	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}
}

func TestCatCommand_Run_SingleFile(t *testing.T) {
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

	cmd := newCatCommand()
	err := cmd.Run(ctx, []string{"cat", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if got := stdout.String(); got != content {
		t.Errorf("stdout = %q, want %q", got, content)
	}

	if stderr.Len() > 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestCatCommand_Run_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content1 := "first\n"
	content2 := "second\n"

	if err := os.WriteFile(file1, []byte(content1), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(content2), 0o644); err != nil {
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

	cmd := newCatCommand()
	err := cmd.Run(ctx, []string{"cat", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := content1 + content2
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	stdinContent := "from stdin\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCatCommand()
	// No file arguments - should read from stdin
	err := cmd.Run(ctx, []string{"cat"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if got := stdout.String(); got != stdinContent {
		t.Errorf("stdout = %q, want %q", got, stdinContent)
	}
}

func TestCatCommand_Run_StdinDash(t *testing.T) {
	t.Parallel()

	stdinContent := "from stdin via dash\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCatCommand()
	// Explicit "-" for stdin
	err := cmd.Run(ctx, []string{"cat", "-"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if got := stdout.String(); got != stdinContent {
		t.Errorf("stdout = %q, want %q", got, stdinContent)
	}
}

func TestCatCommand_Run_FileNotFound(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCatCommand()
	err := cmd.Run(ctx, []string{"cat", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] cat:") {
		t.Errorf("error should have [uroot] cat: prefix, got: %v", err)
	}
}

func TestCatCommand_Run_RelativePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "relative.txt")
	content := "relative path content\n"

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

	cmd := newCatCommand()
	// Use relative path
	err := cmd.Run(ctx, []string{"cat", "relative.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if got := stdout.String(); got != content {
		t.Errorf("stdout = %q, want %q", got, content)
	}
}
