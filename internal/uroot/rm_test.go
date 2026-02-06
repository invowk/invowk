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

func TestRmCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newRmCommand()
	if got := cmd.Name(); got != "rm" {
		t.Errorf("Name() = %q, want %q", got, "rm")
	}
}

func TestRmCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newRmCommand()
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

func TestRmCommand_Run_SingleFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
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

	cmd := newRmCommand()
	err := cmd.Run(ctx, []string{"rm", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestRmCommand_Run_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
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

	cmd := newRmCommand()
	err := cmd.Run(ctx, []string{"rm", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	for _, f := range []string{file1, file2} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("file %s should have been removed", f)
		}
	}
}

func TestRmCommand_Run_NonEmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	nestedFile := filepath.Join(subDir, "file.txt")

	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newRmCommand()
	// Without -r, removing a non-empty directory should fail
	err := cmd.Run(ctx, []string{"rm", subDir})
	if err == nil {
		t.Error("rm on non-empty directory without -r should error")
	}

	// Error should have [uroot] prefix
	if err != nil && !strings.HasPrefix(err.Error(), "[uroot] rm:") {
		t.Errorf("error should have [uroot] rm: prefix, got: %v", err)
	}
}

func TestRmCommand_Run_RecursiveDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	nestedFile := filepath.Join(subDir, "nested.txt")

	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("nested"), 0o644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newRmCommand()
	err := cmd.Run(ctx, []string{"rm", "-r", subDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("directory should have been removed")
	}
}

func TestRmCommand_Run_Force(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newRmCommand()
	// With -f, removing nonexistent file should not error
	err := cmd.Run(ctx, []string{"rm", "-f", "/nonexistent/file.txt"})
	if err != nil {
		t.Errorf("rm -f on nonexistent file should not error, got: %v", err)
	}
}

func TestRmCommand_Run_NonexistentWithoutForce(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newRmCommand()
	// Without -f, removing nonexistent file should error
	err := cmd.Run(ctx, []string{"rm", "/nonexistent/file.txt"})
	if err == nil {
		t.Error("rm on nonexistent file without -f should error")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] rm:") {
		t.Errorf("error should have [uroot] rm: prefix, got: %v", err)
	}
}

func TestRmCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newRmCommand()
	err := cmd.Run(ctx, []string{"rm"})
	if err == nil {
		t.Error("rm with no arguments should error")
	}
}

func TestRmCommand_Run_RelativePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "relative.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
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

	cmd := newRmCommand()
	// Use relative path
	err := cmd.Run(ctx, []string{"rm", "relative.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}
