// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

func TestChmodCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newChmodCommand()
	if got := cmd.Name(); got != "chmod" {
		t.Errorf("Name() = %q, want %q", got, "chmod")
	}
}

func TestChmodCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newChmodCommand()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasRecursive := false
	for _, f := range flags {
		if f.Name == "recursive" {
			hasRecursive = true
		}
	}
	if !hasRecursive {
		t.Error("SupportedFlags() should include --recursive flag")
	}
}

func TestChmodCommand_Run_OctalMode(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Windows does not support Unix permission bits")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
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

	cmd := newChmodCommand()
	err := cmd.Run(ctx, []string{"chmod", "0755", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// Check that the executable bits are set
	perm := info.Mode().Perm()
	if perm&0o111 == 0 {
		t.Errorf("expected executable bits set, got mode %o", perm)
	}
}

func TestChmodCommand_Run_SymbolicMode(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Windows does not support Unix permission bits")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
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

	cmd := newChmodCommand()
	err := cmd.Run(ctx, []string{"chmod", "+x", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm&0o111 == 0 {
		t.Errorf("expected executable bits set after +x, got mode %o", perm)
	}
}

func TestChmodCommand_Run_NonexistentFile(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newChmodCommand()
	err := cmd.Run(ctx, []string{"chmod", "0755", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] chmod:") {
		t.Errorf("error should have [uroot] chmod: prefix, got: %v", err)
	}
}

func TestChmodCommand_Run_Recursive(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Windows does not support Unix permission bits")
	}

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	nestedFile := filepath.Join(subDir, "nested.txt")
	if err := os.WriteFile(nestedFile, []byte("nested\n"), 0o644); err != nil {
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

	cmd := newChmodCommand()
	// Upstream u-root chmod uses --recursive (long flag), not -R
	err := cmd.Run(ctx, []string{"chmod", "--recursive", "0700", subDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(nestedFile)
	if err != nil {
		t.Fatalf("failed to stat nested file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		t.Errorf("expected group/other bits cleared after chmod -R 0700, got mode %o", perm)
	}
}

func TestChmodCommand_Run_RelativePath(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Windows does not support Unix permission bits")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "relative.txt")

	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
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

	cmd := newChmodCommand()
	err := cmd.Run(ctx, []string{"chmod", "0755", "relative.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm&0o111 == 0 {
		t.Errorf("expected executable bits set, got mode %o", perm)
	}
}
