// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestDirnameCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newDirnameCommand()
	if got := cmd.Name(); got != "dirname" {
		t.Errorf("Name() = %q, want %q", got, "dirname")
	}
}

func TestDirnameCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newDirnameCommand()
	flags := cmd.SupportedFlags()
	if len(flags) != 0 {
		t.Errorf("SupportedFlags() returned %d flags, want 0", len(flags))
	}
}

func TestDirnameCommand_Run_BasicPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	err := cmd.Run(ctx, []string{"dirname", "/foo/bar"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "/foo" {
		t.Errorf("got %q, want %q", got, "/foo")
	}
}

func TestDirnameCommand_Run_NestedPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	err := cmd.Run(ctx, []string{"dirname", "/a/b/c/d"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "/a/b/c" {
		t.Errorf("got %q, want %q", got, "/a/b/c")
	}
}

func TestDirnameCommand_Run_TrailingSlash(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	// path.Dir normalizes trailing slashes
	err := cmd.Run(ctx, []string{"dirname", "/foo/bar/"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "/foo/bar" {
		t.Errorf("got %q, want %q", got, "/foo/bar")
	}
}

func TestDirnameCommand_Run_RootPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	err := cmd.Run(ctx, []string{"dirname", "/foo"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "/" {
		t.Errorf("got %q, want %q", got, "/")
	}
}

func TestDirnameCommand_Run_MultipleArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	err := cmd.Run(ctx, []string{"dirname", "/a/b", "/c/d/e"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0] != "/a" {
		t.Errorf("line 0 = %q, want %q", lines[0], "/a")
	}
	if lines[1] != "/c/d" {
		t.Errorf("line 1 = %q, want %q", lines[1], "/c/d")
	}
}

func TestDirnameCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	err := cmd.Run(ctx, []string{"dirname"})

	if err == nil {
		t.Fatal("Run() should return error for missing operand")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] dirname:") {
		t.Errorf("error should have [uroot] dirname: prefix, got: %v", err)
	}
}

func TestDirnameCommand_Run_PlainFilename(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	// A plain filename with no directory => dirname returns "."
	err := cmd.Run(ctx, []string{"dirname", "file.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "." {
		t.Errorf("got %q, want %q", got, ".")
	}
}

func TestDirnameCommand_Run_BackslashIsNotSeparator(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newDirnameCommand()
	// In the POSIX virtual shell, backslash is NOT a path separator.
	// path.Dir treats '\' as a regular character, so the entire input has no
	// directory component and dirname returns ".".
	err := cmd.Run(ctx, []string{"dirname", `C:\Users\foo`})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "." {
		t.Errorf("got %q, want %q (backslash should not be treated as separator)", got, ".")
	}
}
