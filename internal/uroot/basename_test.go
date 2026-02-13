// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestBasenameCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newBasenameCommand()
	if got := cmd.Name(); got != "basename" {
		t.Errorf("Name() = %q, want %q", got, "basename")
	}
}

func TestBasenameCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newBasenameCommand()
	flags := cmd.SupportedFlags()
	if len(flags) != 0 {
		t.Errorf("SupportedFlags() returned %d flags, want 0", len(flags))
	}
}

func TestBasenameCommand_Run_BasicPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	err := cmd.Run(ctx, []string{"basename", "/foo/bar"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "bar" {
		t.Errorf("got %q, want %q", got, "bar")
	}
}

func TestBasenameCommand_Run_SuffixStripping(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	err := cmd.Run(ctx, []string{"basename", "/path/to/bar.txt", ".txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "bar" {
		t.Errorf("got %q, want %q", got, "bar")
	}
}

func TestBasenameCommand_Run_SuffixNoMatch(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	// Suffix doesn't match, so the base should be returned as-is
	err := cmd.Run(ctx, []string{"basename", "file.go", ".txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "file.go" {
		t.Errorf("got %q, want %q", got, "file.go")
	}
}

func TestBasenameCommand_Run_SuffixEqualsBase(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	// When suffix equals the entire basename, it should NOT be stripped
	err := cmd.Run(ctx, []string{"basename", "/path/.txt", ".txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != ".txt" {
		t.Errorf("got %q, want %q", got, ".txt")
	}
}

func TestBasenameCommand_Run_RootPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	err := cmd.Run(ctx, []string{"basename", "/"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "/" {
		t.Errorf("got %q, want %q", got, "/")
	}
}

func TestBasenameCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	err := cmd.Run(ctx, []string{"basename"})

	if err == nil {
		t.Fatal("Run() should return error for missing operand")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] basename:") {
		t.Errorf("error should have [uroot] basename: prefix, got: %v", err)
	}
}

func TestBasenameCommand_Run_SimpleFilename(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	// A simple filename with no directory component
	err := cmd.Run(ctx, []string{"basename", "myfile.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "myfile.txt" {
		t.Errorf("got %q, want %q", got, "myfile.txt")
	}
}

func TestBasenameCommand_Run_BackslashIsNotSeparator(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBasenameCommand()
	// In the POSIX virtual shell, backslash is NOT a path separator.
	// path.Base treats '\' as a regular character, so the entire string is the base.
	err := cmd.Run(ctx, []string{"basename", `C:\Users\foo`})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != `C:\Users\foo` {
		t.Errorf("got %q, want %q (backslash should not be treated as separator)", got, `C:\Users\foo`)
	}
}
