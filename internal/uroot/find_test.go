// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newFindCommand()
	if got := cmd.Name(); got != "find" {
		t.Errorf("Name() = %q, want %q", got, "find")
	}
}

func TestFindCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newFindCommand()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasName := false
	hasType := false
	for _, f := range flags {
		if f.Name == "name" {
			hasName = true
		}
		if f.Name == "type" {
			hasType = true
		}
	}
	if !hasName {
		t.Error("SupportedFlags() should include -name flag")
	}
	if !hasType {
		t.Error("SupportedFlags() should include -type flag")
	}
}

func TestFindCommand_Run_ByName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a directory structure with files to search
	if err := os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to create hello.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "world.txt"), []byte("world"), 0o644); err != nil {
		t.Fatalf("failed to create world.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("failed to create readme.md: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newFindCommand()
	err := cmd.Run(ctx, []string{"find", tmpDir, "-name", "*.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "hello.txt") {
		t.Errorf("output should contain hello.txt, got: %q", output)
	}
	if !strings.Contains(output, "world.txt") {
		t.Errorf("output should contain world.txt, got: %q", output)
	}
	if strings.Contains(output, "readme.md") {
		t.Errorf("output should not contain readme.md when searching for *.txt, got: %q", output)
	}
}

func TestFindCommand_Run_ByTypeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create file.txt: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newFindCommand()
	err := cmd.Run(ctx, []string{"find", tmpDir, "-type", "f"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "file.txt") {
		t.Errorf("output should contain file.txt, got: %q", output)
	}
}

func TestFindCommand_Run_ByTypeDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "mydir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create mydir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create file.txt: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newFindCommand()
	err := cmd.Run(ctx, []string{"find", tmpDir, "-type", "d"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "mydir") {
		t.Errorf("output should contain mydir, got: %q", output)
	}
}

func TestFindCommand_Run_AllFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("failed to create a.txt: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newFindCommand()
	// Find with no filters lists all entries
	err := cmd.Run(ctx, []string{"find", tmpDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Error("find with no filters should produce output")
	}
}
