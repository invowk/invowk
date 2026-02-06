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

func TestMkdirCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newMkdirCommand()
	if got := cmd.Name(); got != "mkdir" {
		t.Errorf("Name() = %q, want %q", got, "mkdir")
	}
}

func TestMkdirCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newMkdirCommand()
	flags := cmd.SupportedFlags()

	// Should have -p flag at minimum
	hasParents := false
	for _, f := range flags {
		if f.Name == "p" || f.ShortName == "p" {
			hasParents = true
			break
		}
	}
	if !hasParents {
		t.Error("SupportedFlags() should include -p flag")
	}
}

func TestMkdirCommand_Run_SingleDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "newdir")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newMkdirCommand()
	err := cmd.Run(ctx, []string{"mkdir", newDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestMkdirCommand_Run_MultipleDirs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newMkdirCommand()
	err := cmd.Run(ctx, []string{"mkdir", dir1, dir2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	for _, dir := range []string{dir1, dir2} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s was not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestMkdirCommand_Run_Parents(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	deepDir := filepath.Join(tmpDir, "a", "b", "c")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newMkdirCommand()
	err := cmd.Run(ctx, []string{"mkdir", "-p", deepDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(deepDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestMkdirCommand_Run_ParentsExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")

	// Create the directory first
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatalf("failed to create existing directory: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newMkdirCommand()
	// With -p, creating an existing directory should not error
	err := cmd.Run(ctx, []string{"mkdir", "-p", existingDir})
	if err != nil {
		t.Errorf("mkdir -p on existing directory should not error, got: %v", err)
	}
}

func TestMkdirCommand_Run_ExistingWithoutParents(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")

	// Create the directory first
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatalf("failed to create existing directory: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newMkdirCommand()
	// Without -p, creating an existing directory writes to stderr but continues
	// (similar to standard mkdir behavior with multiple directories)
	_ = cmd.Run(ctx, []string{"mkdir", existingDir})

	// The error message should be written to stderr
	if stderr.Len() == 0 {
		t.Error("mkdir on existing directory without -p should write error to stderr")
	}
}

func TestMkdirCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newMkdirCommand()
	err := cmd.Run(ctx, []string{"mkdir"})
	if err == nil {
		t.Error("mkdir with no arguments should error")
	}
}

func TestMkdirCommand_Run_RelativePath(t *testing.T) {
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

	cmd := newMkdirCommand()
	// Use relative path
	err := cmd.Run(ctx, []string{"mkdir", "reldir"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "reldir"))
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}
