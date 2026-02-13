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

func TestBase64Command_Name(t *testing.T) {
	t.Parallel()

	cmd := newBase64Command()
	if got := cmd.Name(); got != "base64" {
		t.Errorf("Name() = %q, want %q", got, "base64")
	}
}

func TestBase64Command_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newBase64Command()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasDecode := false
	for _, f := range flags {
		if f.Name == "d" {
			hasDecode = true
		}
	}
	if !hasDecode {
		t.Error("SupportedFlags() should include -d flag")
	}
}

func TestBase64Command_Run_EncodeStdin(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader("hello world"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBase64Command()
	err := cmd.Run(ctx, []string{"base64"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "aGVsbG8gd29ybGQ="
	if got != want {
		t.Errorf("encoded output = %q, want %q", got, want)
	}
}

func TestBase64Command_Run_DecodeStdin(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader("aGVsbG8gd29ybGQ="),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBase64Command()
	err := cmd.Run(ctx, []string{"base64", "-d"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := stdout.String()
	want := "hello world"
	if got != want {
		t.Errorf("decoded output = %q, want %q", got, want)
	}
}

func TestBase64Command_Run_EncodeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "input.txt")

	if err := os.WriteFile(testFile, []byte("test data"), 0o644); err != nil {
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

	cmd := newBase64Command()
	err := cmd.Run(ctx, []string{"base64", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "dGVzdCBkYXRh"
	if got != want {
		t.Errorf("encoded output = %q, want %q", got, want)
	}
}

func TestBase64Command_Run_DecodeInvalid(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader("!!!invalid-base64!!!"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newBase64Command()
	err := cmd.Run(ctx, []string{"base64", "-d"})

	if err == nil {
		t.Fatal("Run() should return error for corrupted base64 input")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] base64:") {
		t.Errorf("error should have [uroot] base64: prefix, got: %v", err)
	}
}

func TestBase64Command_Run_DecodeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "encoded.txt")

	if err := os.WriteFile(testFile, []byte("dGVzdCBkYXRh"), 0o644); err != nil {
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

	cmd := newBase64Command()
	err := cmd.Run(ctx, []string{"base64", "-d", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := stdout.String()
	want := "test data"
	if got != want {
		t.Errorf("decoded output = %q, want %q", got, want)
	}
}
