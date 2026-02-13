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

func TestMktempCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newMktempCommand()
	if got := cmd.Name(); got != "mktemp" {
		t.Errorf("Name() = %q, want %q", got, "mktemp")
	}
}

func TestMktempCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newMktempCommand()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasDir := false
	hasPrefix := false
	for _, f := range flags {
		if f.Name == "d" {
			hasDir = true
		}
		if f.Name == "p" {
			hasPrefix = true
		}
	}
	if !hasDir {
		t.Error("SupportedFlags() should include -d flag")
	}
	if !hasPrefix {
		t.Error("SupportedFlags() should include -p flag")
	}
}

func TestMktempCommand_Run_TempFile(t *testing.T) {
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

	cmd := newMktempCommand()
	err := cmd.Run(ctx, []string{"mktemp", "-p", tmpDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		t.Fatal("mktemp should output the path of the created file")
	}

	// Verify the file was actually created
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("created temp file does not exist at %q: %v", output, err)
	}
	if info.IsDir() {
		t.Errorf("mktemp without -d should create a file, not a directory")
	}
}

func TestMktempCommand_Run_TempDir(t *testing.T) {
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

	cmd := newMktempCommand()
	err := cmd.Run(ctx, []string{"mktemp", "-d", "-p", tmpDir})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		t.Fatal("mktemp -d should output the path of the created directory")
	}

	// Verify the directory was actually created
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("created temp directory does not exist at %q: %v", output, err)
	}
	if !info.IsDir() {
		t.Errorf("mktemp -d should create a directory, not a file")
	}
}

func TestMktempCommand_Run_Default(t *testing.T) {
	t.Parallel()

	// No Windows skip: this custom implementation uses os.TempDir()
	// which returns the correct temp directory on all platforms.

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newMktempCommand()
	err := cmd.Run(ctx, []string{"mktemp"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		t.Fatal("mktemp should output the path of the created file")
	}

	// Verify the file exists
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("created temp file does not exist at %q: %v", output, err)
	}
}

func TestMktempCommand_Run_Template(t *testing.T) {
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

	cmd := newMktempCommand()
	err := cmd.Run(ctx, []string{"mktemp", "-p", tmpDir, "myprefix.XXXXXX"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	base := filepath.Base(output)
	if !strings.HasPrefix(base, "myprefix.") {
		t.Errorf("created file should have prefix 'myprefix.', got %q", base)
	}
}

func TestMktempCommand_Run_QuietSuppressesError(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newMktempCommand()
	// Use a nonexistent parent directory to trigger an error
	err := cmd.Run(ctx, []string{"mktemp", "-q", "-p", "/nonexistent/path/for/mktemp"})
	if err != nil {
		t.Errorf("Run() with -q should suppress errors, got: %v", err)
	}
}

func TestMktempCommand_Run_InvalidDir(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newMktempCommand()
	err := cmd.Run(ctx, []string{"mktemp", "-p", "/nonexistent/path/for/mktemp"})
	if err == nil {
		t.Fatal("Run() should return error for nonexistent parent directory")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] mktemp:") {
		t.Errorf("error should have [uroot] mktemp: prefix, got: %v", err)
	}
}

func TestMktempCommand_Run_RelativeDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
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

	cmd := newMktempCommand()
	// Relative -p should resolve against Dir
	err := cmd.Run(ctx, []string{"mktemp", "-p", "subdir"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(output, subDir) {
		t.Errorf("created file should be under %q, got %q", subDir, output)
	}
}

func TestMktempCommand_Run_TMPDIR(t *testing.T) {
	t.Parallel()

	customTmpDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    t.TempDir(),
		LookupEnv: func(key string) (string, bool) {
			if key == "TMPDIR" {
				return customTmpDir, true
			}
			return os.LookupEnv(key)
		},
	})

	cmd := newMktempCommand()
	err := cmd.Run(ctx, []string{"mktemp"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(output, customTmpDir) {
		t.Errorf("with TMPDIR set, file should be under %q, got %q", customTmpDir, output)
	}
}
