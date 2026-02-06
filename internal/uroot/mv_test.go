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

func TestMvCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newMvCommand()
	if got := cmd.Name(); got != "mv" {
		t.Errorf("Name() = %q, want %q", got, "mv")
	}
}

func TestMvCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newMvCommand()
	flags := cmd.SupportedFlags()

	// Should have -f and -n flags at minimum
	hasForce := false
	hasNoClobber := false
	for _, f := range flags {
		if f.Name == "f" || f.ShortName == "f" {
			hasForce = true
		}
		if f.Name == "n" || f.ShortName == "n" {
			hasNoClobber = true
		}
	}
	if !hasForce {
		t.Error("SupportedFlags() should include -f flag")
	}
	if !hasNoClobber {
		t.Error("SupportedFlags() should include -n flag")
	}
}

func TestMvCommand_Run_SingleFile(t *testing.T) {
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

	cmd := newMvCommand()
	err := cmd.Run(ctx, []string{"mv", srcFile, dstFile})
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

	// Verify source no longer exists
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source file should have been removed after mv")
	}
}

func TestMvCommand_Run_ToDirectory(t *testing.T) {
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

	cmd := newMvCommand()
	err := cmd.Run(ctx, []string{"mv", srcFile, dstDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// File should be moved into directory with same name
	dstFile := filepath.Join(dstDir, "source.txt")
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != content {
		t.Errorf("destination content = %q, want %q", string(got), content)
	}

	// Verify source no longer exists
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source file should have been removed after mv")
	}
}

func TestMvCommand_Run_RenameDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	dstDir := filepath.Join(tmpDir, "dstdir")

	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newMvCommand()
	err := cmd.Run(ctx, []string{"mv", srcDir, dstDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Verify directory was renamed
	info, err := os.Stat(dstDir)
	if err != nil {
		t.Fatalf("destination directory doesn't exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("destination should be a directory")
	}

	// Verify content was preserved
	got, err := os.ReadFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read file in destination: %v", err)
	}
	if string(got) != "content" {
		t.Errorf("file content = %q, want %q", string(got), "content")
	}

	// Verify source no longer exists
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("source directory should have been removed after mv")
	}
}

func TestMvCommand_Run_Overwrite(t *testing.T) {
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

	cmd := newMvCommand()
	err := cmd.Run(ctx, []string{"mv", srcFile, dstFile})
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

func TestMvCommand_Run_NoClobber(t *testing.T) {
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

	cmd := newMvCommand()
	// With -n, destination should not be overwritten
	_ = cmd.Run(ctx, []string{"mv", "-n", srcFile, dstFile})
	// Note: -n behavior varies - some implementations silently skip, others error
	// We'll check the destination wasn't overwritten
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != dstContent {
		t.Errorf("destination should not have been overwritten with -n, got %q", string(got))
	}
}

func TestMvCommand_Run_SourceNotFound(t *testing.T) {
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

	cmd := newMvCommand()
	err := cmd.Run(ctx, []string{"mv", "/nonexistent/source.txt", "dest.txt"})
	if err == nil {
		t.Error("mv with nonexistent source should error")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] mv:") {
		t.Errorf("error should have [uroot] mv: prefix, got: %v", err)
	}
}

func TestMvCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newMvCommand()
	err := cmd.Run(ctx, []string{"mv"})
	if err == nil {
		t.Error("mv with no arguments should error")
	}
}

func TestMvCommand_Run_RelativePaths(t *testing.T) {
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

	cmd := newMvCommand()
	// Use relative paths
	err := cmd.Run(ctx, []string{"mv", "source.txt", "dest.txt"})
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
