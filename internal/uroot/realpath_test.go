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

func TestRealpathCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newRealpathCommand()
	if got := cmd.Name(); got != "realpath" {
		t.Errorf("Name() = %q, want %q", got, "realpath")
	}
}

func TestRealpathCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newRealpathCommand()
	flags := cmd.SupportedFlags()
	if len(flags) != 0 {
		t.Errorf("SupportedFlags() returned %d flags, want 0", len(flags))
	}
}

func TestRealpathCommand_Run_AbsolutePath(t *testing.T) {
	t.Parallel()

	// Resolve the temp dir to handle OS symlinks (e.g., macOS /var → /private/var,
	// Windows 8.3 short names like RUNNER~1 → runneradmin)
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve temp dir: %v", err)
	}
	testFile := filepath.Join(tmpDir, "file.txt")
	err = os.WriteFile(testFile, []byte("content"), 0o644)
	if err != nil {
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

	cmd := newRealpathCommand()
	err = cmd.Run(ctx, []string{"realpath", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := filepath.ToSlash(testFile)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRealpathCommand_Run_RelativePath(t *testing.T) {
	t.Parallel()

	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve temp dir: %v", err)
	}
	testFile := filepath.Join(tmpDir, "file.txt")
	err = os.WriteFile(testFile, []byte("content"), 0o644)
	if err != nil {
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

	cmd := newRealpathCommand()
	err = cmd.Run(ctx, []string{"realpath", "file.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := filepath.ToSlash(testFile)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRealpathCommand_Run_SymlinkResolution(t *testing.T) {
	t.Parallel()

	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve temp dir: %v", err)
	}
	target := filepath.Join(tmpDir, "actual.txt")
	symlink := filepath.Join(tmpDir, "link.txt")

	err = os.WriteFile(target, []byte("content"), 0o644)
	if err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
	err = os.Symlink(target, symlink)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newRealpathCommand()
	err = cmd.Run(ctx, []string{"realpath", symlink})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := filepath.ToSlash(target)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRealpathCommand_Run_MultipleArgs(t *testing.T) {
	t.Parallel()

	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve temp dir: %v", err)
	}
	file1 := filepath.Join(tmpDir, "a.txt")
	file2 := filepath.Join(tmpDir, "b.txt")

	for _, f := range []string{file1, file2} {
		err = os.WriteFile(f, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newRealpathCommand()
	err = cmd.Run(ctx, []string{"realpath", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want1 := filepath.ToSlash(file1)
	want2 := filepath.ToSlash(file2)
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0] != want1 {
		t.Errorf("line 0 = %q, want %q", lines[0], want1)
	}
	if lines[1] != want2 {
		t.Errorf("line 1 = %q, want %q", lines[1], want2)
	}
}

func TestRealpathCommand_Run_NonexistentPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newRealpathCommand()
	err := cmd.Run(ctx, []string{"realpath", "/nonexistent/path/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent path")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] realpath:") {
		t.Errorf("error should have [uroot] realpath: prefix, got: %v", err)
	}
}

func TestRealpathCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newRealpathCommand()
	err := cmd.Run(ctx, []string{"realpath"})

	if err == nil {
		t.Fatal("Run() should return error for missing operand")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] realpath:") {
		t.Errorf("error should have [uroot] realpath: prefix, got: %v", err)
	}
}

func TestRealpathCommand_Run_Directory(t *testing.T) {
	t.Parallel()

	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve temp dir: %v", err)
	}
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newRealpathCommand()
	err = cmd.Run(ctx, []string{"realpath", "subdir"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := filepath.ToSlash(subDir)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
