// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShasumCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newShasumCommand()
	if got := cmd.Name(); got != "shasum" {
		t.Errorf("Name() = %q, want %q", got, "shasum")
	}
}

func TestShasumCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newShasumCommand()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasAlgorithm := false
	for _, f := range flags {
		if f.Name == "a" {
			hasAlgorithm = true
		}
	}
	if !hasAlgorithm {
		t.Error("SupportedFlags() should include -a flag")
	}
}

func TestShasumCommand_Run_SHA256File(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world\n"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newShasumCommand()
	err := cmd.Run(ctx, []string{"shasum", "-a", "256", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// SHA-256 of "hello world\n" (12 bytes)
	wantHash := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	if !strings.Contains(output, wantHash) {
		t.Errorf("output should contain SHA-256 hash %q, got: %q", wantHash, output)
	}
}

func TestShasumCommand_Run_SHA1File(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world\n"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newShasumCommand()
	err := cmd.Run(ctx, []string{"shasum", "-a", "1", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// SHA-1 of "hello world\n" is 22596363b3de40b06f981fb85d82312e8c0ed511
	wantHash := "22596363b3de40b06f981fb85d82312e8c0ed511"
	if !strings.Contains(output, wantHash) {
		t.Errorf("output should contain SHA-1 hash %q, got: %q", wantHash, output)
	}
}

func TestShasumCommand_Run_DefaultAlgorithm(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world\n"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newShasumCommand()
	err := cmd.Run(ctx, []string{"shasum", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	// Without -a flag, shasum defaults to SHA-1 per POSIX convention
	wantHash := "22596363b3de40b06f981fb85d82312e8c0ed511"
	if !strings.Contains(output, wantHash) {
		t.Errorf("output should contain SHA-1 hash %q, got: %q", wantHash, output)
	}
}

func TestShasumCommand_Run_Stdin(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader("hello world\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newShasumCommand()
	err := cmd.Run(ctx, []string{"shasum", "-a", "256"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	wantHash := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	if !strings.Contains(output, wantHash) {
		t.Errorf("output should contain SHA-256 hash %q, got: %q", wantHash, output)
	}
}

func TestShasumCommand_Run_NonexistentFile(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newShasumCommand()
	err := cmd.Run(ctx, []string{"shasum", "-a", "256", "/nonexistent/file.txt"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent file")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] shasum:") {
		t.Errorf("error should have [uroot] shasum: prefix, got: %v", err)
	}
}
