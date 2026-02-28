// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLnCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newLnCommand()
	if got := cmd.Name(); got != "ln" {
		t.Errorf("Name() = %q, want %q", got, "ln")
	}
}

func TestLnCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newLnCommand()
	flags := cmd.SupportedFlags()

	flagNames := make(map[string]bool)
	for _, f := range flags {
		flagNames[f.Name] = true
	}

	if !flagNames["s"] {
		t.Error("SupportedFlags() should include -s flag")
	}
	if !flagNames["f"] {
		t.Error("SupportedFlags() should include -f flag")
	}
}

func TestLnCommand_Run_SymbolicLink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	linkName := filepath.Join(tmpDir, "link.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	err := cmd.Run(ctx, []string{"ln", "-s", target, linkName})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Verify symlink exists and points to target
	resolved, err := os.Readlink(linkName)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if resolved != target {
		t.Errorf("symlink points to %q, want %q", resolved, target)
	}
}

func TestLnCommand_Run_HardLink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	linkName := filepath.Join(tmpDir, "hardlink.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	err := cmd.Run(ctx, []string{"ln", target, linkName})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Verify hard link: both should have the same inode
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat target: %v", err)
	}
	linkInfo, err := os.Stat(linkName)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}

	if !os.SameFile(targetInfo, linkInfo) {
		t.Error("hard link and target should reference the same file")
	}
}

func TestLnCommand_Run_ForceOverwrite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	linkName := filepath.Join(tmpDir, "link.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
	// Create existing file at the link location
	if err := os.WriteFile(linkName, []byte("existing"), 0o644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	// -s and -f should remove the existing file before creating symlink
	err := cmd.Run(ctx, []string{"ln", "-s", "-f", target, linkName})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	resolved, err := os.Readlink(linkName)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if resolved != target {
		t.Errorf("symlink points to %q, want %q", resolved, target)
	}
}

func TestLnCommand_Run_RelativeSymlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	// Relative symlink target should be stored as-is (not resolved to absolute)
	err := cmd.Run(ctx, []string{"ln", "-s", "target.txt", "rellink.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	linkPath := filepath.Join(tmpDir, "rellink.txt")
	resolved, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}

	// The symlink must store the relative target string, not an absolute path
	if resolved != "target.txt" {
		t.Errorf("symlink target = %q, want %q (relative target should be preserved)", resolved, "target.txt")
	}

	// Verify the symlink actually resolves to the correct file
	content, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("failed to read through symlink: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("content through symlink = %q, want %q", string(content), "content")
	}
}

func TestLnCommand_Run_RelativeHardLink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	// Relative hard link target should be resolved to absolute (hard links need real paths)
	err := cmd.Run(ctx, []string{"ln", "target.txt", "hardrellink.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	linkPath := filepath.Join(tmpDir, "hardrellink.txt")
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat target: %v", err)
	}
	linkInfo, err := os.Stat(linkPath)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}

	if !os.SameFile(targetInfo, linkInfo) {
		t.Error("hard link and target should reference the same file")
	}
}

func TestLnCommand_Run_WithoutForce_ExistingFails(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	linkName := filepath.Join(tmpDir, "link.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
	if err := os.WriteFile(linkName, []byte("existing"), 0o644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	// Without -f, should fail because linkName already exists
	err := cmd.Run(ctx, []string{"ln", "-s", target, linkName})
	if err == nil {
		t.Fatal("Run() should return error when link destination exists")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] ln:") {
		t.Errorf("error should have [uroot] ln: prefix, got: %v", err)
	}
}

func TestLnCommand_Run_HardLinkNonexistentTarget(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	linkName := filepath.Join(tmpDir, "hardlink.txt")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	err := cmd.Run(ctx, []string{"ln", filepath.Join(tmpDir, "nonexistent.txt"), linkName})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent hard link target")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] ln:") {
		t.Errorf("error should have [uroot] ln: prefix, got: %v", err)
	}
}

func TestLnCommand_Run_MissingOperand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newLnCommand()
	err := cmd.Run(ctx, []string{"ln", "-s", "only_target"})

	if err == nil {
		t.Fatal("Run() should return error for missing file operand")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] ln:") {
		t.Errorf("error should have [uroot] ln: prefix, got: %v", err)
	}
}
