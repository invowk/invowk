// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	goruntime "runtime"
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

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: upstream u-root mktemp hardcodes /tmp which does not exist on Windows")
	}

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
