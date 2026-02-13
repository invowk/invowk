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

func TestGzipCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newGzipCommand()
	if got := cmd.Name(); got != "gzip" {
		t.Errorf("Name() = %q, want %q", got, "gzip")
	}
}

func TestGzipCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newGzipCommand()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasDecompress := false
	hasStdout := false
	for _, f := range flags {
		if f.Name == "d" {
			hasDecompress = true
		}
		if f.Name == "c" {
			hasStdout = true
		}
	}
	if !hasDecompress {
		t.Error("SupportedFlags() should include -d flag")
	}
	if !hasStdout {
		t.Error("SupportedFlags() should include -c flag")
	}
}

func TestGzipCommand_Run_CompressFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world, this is some content to compress\n"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
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

	cmd := newGzipCommand()
	// Use -f to force compression (upstream gzip rejects non-terminal stdout without -f)
	err := cmd.Run(ctx, []string{"gzip", "-f", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// gzip replaces the original file with a .gz file
	gzFile := testFile + ".gz"
	if _, err := os.Stat(gzFile); err != nil {
		t.Fatalf("compressed file %q was not created: %v", gzFile, err)
	}

	// Original file should have been removed
	if _, err := os.Stat(testFile); err == nil {
		t.Error("original file should have been removed after compression")
	}
}

func TestGzipCommand_Run_DecompressFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world, this is some content to compress\n"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// First compress the file
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newGzipCommand()
	err := cmd.Run(ctx, []string{"gzip", "-f", testFile})
	if err != nil {
		t.Fatalf("gzip compress returned error: %v", err)
	}

	gzFile := testFile + ".gz"

	// Now decompress
	stdout.Reset()
	stderr.Reset()
	ctx = WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	err = cmd.Run(ctx, []string{"gzip", "-d", gzFile})
	if err != nil {
		t.Fatalf("gzip decompress returned error: %v", err)
	}

	// Original file should be restored
	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read decompressed file: %v", err)
	}
	if string(got) != content {
		t.Errorf("decompressed content = %q, want %q", string(got), content)
	}

	// .gz file should have been removed
	if _, err := os.Stat(gzFile); err == nil {
		t.Error(".gz file should have been removed after decompression")
	}
}

func TestGzipCommand_Run_CompressToStdout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "content for stdout\n"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
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

	cmd := newGzipCommand()
	err := cmd.Run(ctx, []string{"gzip", "-c", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// With -c, compressed data goes to stdout
	if stdout.Len() == 0 {
		t.Error("gzip -c should write compressed data to stdout")
	}

	// With -c, original file should still exist
	if _, err := os.Stat(testFile); err != nil {
		t.Error("original file should still exist when using -c")
	}
}

func TestGzipCommand_Run_NonexistentFile(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newGzipCommand()
	// Upstream gzip writes the error to stderr and continues (returns nil)
	// rather than returning an error for nonexistent files, so we check stderr
	_ = cmd.Run(ctx, []string{"gzip", "/nonexistent/file.txt"})

	if stderr.Len() == 0 {
		t.Error("gzip with nonexistent file should write error to stderr")
	}
}
