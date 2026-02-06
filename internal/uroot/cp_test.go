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

func TestCpCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newCpCommand()
	if got := cmd.Name(); got != "cp" {
		t.Errorf("Name() = %q, want %q", got, "cp")
	}
}

func TestCpCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newCpCommand()
	flags := cmd.SupportedFlags()

	// Should have -r and -f flags at minimum
	hasRecursive := false
	hasForce := false
	for _, f := range flags {
		if f.Name == "r" || f.ShortName == "r" || f.Name == "R" {
			hasRecursive = true
		}
		if f.Name == "f" || f.ShortName == "f" {
			hasForce = true
		}
	}
	if !hasRecursive {
		t.Error("SupportedFlags() should include -r flag")
	}
	if !hasForce {
		t.Error("SupportedFlags() should include -f flag")
	}
}

func TestCpCommand_Run_SingleFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")
	content := "hello world\n"

	if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	err := cmd.Run(ctx, []string{"cp", srcFile, dstFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Verify destination exists and has correct content
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != content {
		t.Errorf("destination content = %q, want %q", string(got), content)
	}

	// Verify source still exists
	if _, err := os.Stat(srcFile); err != nil {
		t.Error("source file should still exist after cp")
	}
}

func TestCpCommand_Run_ToDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstDir := filepath.Join(tmpDir, "destdir")
	content := "content\n"

	if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("failed to create destination directory: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	err := cmd.Run(ctx, []string{"cp", srcFile, dstDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// File should be copied into directory with same name
	dstFile := filepath.Join(dstDir, "source.txt")
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != content {
		t.Errorf("destination content = %q, want %q", string(got), content)
	}
}

func TestCpCommand_Run_Recursive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	dstDir := filepath.Join(tmpDir, "dstdir")

	// Create source directory with nested content
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("file1"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("file2"), 0o644); err != nil {
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

	cmd := newCpCommand()
	err := cmd.Run(ctx, []string{"cp", "-r", srcDir, dstDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Verify nested content was copied
	content1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatalf("failed to read file1 in destination: %v", err)
	}
	if string(content1) != "file1" {
		t.Errorf("file1 content = %q, want %q", string(content1), "file1")
	}

	content2, err := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
	if err != nil {
		t.Fatalf("failed to read file2 in destination: %v", err)
	}
	if string(content2) != "file2" {
		t.Errorf("file2 content = %q, want %q", string(content2), "file2")
	}
}

func TestCpCommand_Run_DirectoryWithoutRecursive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	dstDir := filepath.Join(tmpDir, "dstdir")

	if err := os.Mkdir(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	// Without -r, copying a directory writes a warning to stderr and skips
	_ = cmd.Run(ctx, []string{"cp", srcDir, dstDir})

	// Should write warning to stderr about omitting directory
	if stderr.Len() == 0 {
		t.Error("cp on directory without -r should write warning to stderr")
	}

	// Destination should not be created
	if _, err := os.Stat(dstDir); err == nil {
		t.Error("destination directory should not have been created without -r")
	}
}

func TestCpCommand_Run_Overwrite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")
	srcContent := "new content\n"
	dstContent := "old content\n"

	if err := os.WriteFile(srcFile, []byte(srcContent), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if err := os.WriteFile(dstFile, []byte(dstContent), 0o644); err != nil {
		t.Fatalf("failed to create destination file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	err := cmd.Run(ctx, []string{"cp", srcFile, dstFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != srcContent {
		t.Errorf("destination content = %q, want %q", string(got), srcContent)
	}
}

func TestCpCommand_Run_SourceNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	err := cmd.Run(ctx, []string{"cp", "/nonexistent/source.txt", "dest.txt"})
	if err == nil {
		t.Error("cp with nonexistent source should error")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] cp:") {
		t.Errorf("error should have [uroot] cp: prefix, got: %v", err)
	}
}

func TestCpCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	err := cmd.Run(ctx, []string{"cp"})
	if err == nil {
		t.Error("cp with no arguments should error")
	}
}

func TestCpCommand_Run_RelativePaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	content := "relative path content\n"

	if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newCpCommand()
	// Use relative paths
	err := cmd.Run(ctx, []string{"cp", "source.txt", "dest.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "dest.txt"))
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != content {
		t.Errorf("destination content = %q, want %q", string(got), content)
	}
}
