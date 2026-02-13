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

func TestTeeCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newTeeCommand()
	if got := cmd.Name(); got != "tee" {
		t.Errorf("Name() = %q, want %q", got, "tee")
	}
}

func TestTeeCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newTeeCommand()
	flags := cmd.SupportedFlags()

	found := false
	for _, f := range flags {
		if f.Name == "a" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SupportedFlags() should include -a flag")
	}
}

func TestTeeCommand_Run_StdoutAndFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.txt")
	input := "hello, tee\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(input),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	err := cmd.Run(ctx, []string{"tee", outFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// stdout should contain the input
	if stdout.String() != input {
		t.Errorf("stdout = %q, want %q", stdout.String(), input)
	}

	// File should also contain the input
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(content) != input {
		t.Errorf("file content = %q, want %q", string(content), input)
	}
}

func TestTeeCommand_Run_AppendMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.txt")

	// Write initial content
	if err := os.WriteFile(outFile, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("failed to write initial content: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader("second\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	err := cmd.Run(ctx, []string{"tee", "-a", outFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	want := "first\nsecond\n"
	if string(content) != want {
		t.Errorf("file content = %q, want %q", string(content), want)
	}
}

func TestTeeCommand_Run_OverwriteMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.txt")

	// Write initial content
	if err := os.WriteFile(outFile, []byte("old content\n"), 0o644); err != nil {
		t.Fatalf("failed to write initial content: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader("new content\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	// Without -a, file should be overwritten
	err := cmd.Run(ctx, []string{"tee", outFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	want := "new content\n"
	if string(content) != want {
		t.Errorf("file content = %q, want %q", string(content), want)
	}
}

func TestTeeCommand_Run_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "out1.txt")
	file2 := filepath.Join(tmpDir, "out2.txt")
	input := "multi-file test\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(input),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	err := cmd.Run(ctx, []string{"tee", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	for _, path := range []string{file1, file2} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		if string(content) != input {
			t.Errorf("file %s content = %q, want %q", path, string(content), input)
		}
	}

	if stdout.String() != input {
		t.Errorf("stdout = %q, want %q", stdout.String(), input)
	}
}

func TestTeeCommand_Run_RelativePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	input := "relative path test\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(input),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	// Use a relative filename; should resolve against hc.Dir
	err := cmd.Run(ctx, []string{"tee", "relative.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "relative.txt"))
	if err != nil {
		t.Fatalf("failed to read relative file: %v", err)
	}
	if string(content) != input {
		t.Errorf("file content = %q, want %q", string(content), input)
	}
}

func TestTeeCommand_Run_InvalidFilePath(t *testing.T) {
	t.Parallel()

	input := "stdin content\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(input),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	// Write to a path in a nonexistent directory
	err := cmd.Run(ctx, []string{"tee", "/nonexistent/dir/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for invalid file path")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] tee:") {
		t.Errorf("error should have [uroot] tee: prefix, got: %v", err)
	}
}

func TestTeeCommand_Run_PartialOpenFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goodFile := filepath.Join(tmpDir, "good.txt")
	badFile := filepath.Join(tmpDir, "nonexistent", "bad.txt") // Parent dir doesn't exist
	input := "partial test\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(input),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	// Second file will fail to open, triggering cleanup of the first
	err := cmd.Run(ctx, []string{"tee", goodFile, badFile})

	if err == nil {
		t.Fatal("Run() should return error when a file cannot be opened")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] tee:") {
		t.Errorf("error should have [uroot] tee: prefix, got: %v", err)
	}

	// The good file should not exist (or be empty) since the open-failure
	// cleanup path closes already-opened files
	if content, readErr := os.ReadFile(goodFile); readErr == nil && len(content) > 0 {
		t.Errorf("good file should be empty after cleanup, got content: %q", string(content))
	}
}

func TestTeeCommand_Run_StdinOnly(t *testing.T) {
	t.Parallel()

	input := "just stdout\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(input),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTeeCommand()
	// No file args: tee should just pass stdin to stdout
	err := cmd.Run(ctx, []string{"tee"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.String() != input {
		t.Errorf("stdout = %q, want %q", stdout.String(), input)
	}
}
