// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTarCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newTarCommand()
	if got := cmd.Name(); got != "tar" {
		t.Errorf("Name() = %q, want %q", got, "tar")
	}
}

func TestTarCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newTarCommand()
	flags := cmd.SupportedFlags()

	if len(flags) == 0 {
		t.Error("SupportedFlags() returned empty slice")
	}

	hasCreate := false
	hasExtract := false
	hasFile := false
	for _, f := range flags {
		switch f.Name {
		case "c":
			hasCreate = true
		case "x":
			hasExtract = true
		case "f":
			hasFile = true
		}
	}
	if !hasCreate {
		t.Error("SupportedFlags() should include -c flag")
	}
	if !hasExtract {
		t.Error("SupportedFlags() should include -x flag")
	}
	if !hasFile {
		t.Error("SupportedFlags() should include -f flag")
	}
}

// createTestArchive creates a tar archive at archivePath containing the given
// file entries (name -> content). This helper creates archives directly using
// Go's archive/tar to avoid the known limitation in u-root's tarutil where
// filepath.Rel fails with absolute paths when ChangeDirectory is empty.
func createTestArchive(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive file: %v", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			t.Fatalf("failed to close archive: %v", closeErr)
		}
	}()

	tw := tar.NewWriter(f)
	defer func() {
		if closeErr := tw.Close(); closeErr != nil {
			t.Fatalf("failed to close tar writer: %v", closeErr)
		}
	}()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header for %q: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content for %q: %v", name, err)
		}
	}
}

func TestTarCommand_Run_ListArchive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "archive.tar")

	// Create an archive using Go's archive/tar directly
	createTestArchive(t, archivePath, map[string]string{
		"file1.txt": "first",
		"file2.txt": "second",
	})

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTarCommand()
	err := cmd.Run(ctx, []string{"tar", "-t", "-f", archivePath})
	if err != nil {
		t.Fatalf("tar list returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "file1.txt") {
		t.Errorf("tar list should contain file1.txt, got: %q", output)
	}
	if !strings.Contains(output, "file2.txt") {
		t.Errorf("tar list should contain file2.txt, got: %q", output)
	}
}

func TestTarCommand_Run_ExtractArchive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "archive.tar")
	extractDir := filepath.Join(tmpDir, "extracted")

	if err := os.Mkdir(extractDir, 0o755); err != nil {
		t.Fatalf("failed to create extract dir: %v", err)
	}

	content := "archive content\n"
	createTestArchive(t, archivePath, map[string]string{
		"data.txt": content,
	})

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       extractDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTarCommand()
	err := cmd.Run(ctx, []string{"tar", "-x", "-f", archivePath, extractDir})
	if err != nil {
		t.Fatalf("tar extract returned error: %v", err)
	}

	// Verify extracted file
	got, err := os.ReadFile(filepath.Join(extractDir, "data.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(got) != content {
		t.Errorf("extracted content = %q, want %q", string(got), content)
	}
}

func TestTarCommand_Run_NonexistentArchive(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTarCommand()
	err := cmd.Run(ctx, []string{"tar", "-t", "-f", "/nonexistent/archive.tar"})

	if err == nil {
		t.Fatal("Run() should return error for nonexistent archive")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] tar:") {
		t.Errorf("error should have [uroot] tar: prefix, got: %v", err)
	}
}

func TestTarCommand_Run_CreateMode_KnownBug(t *testing.T) {
	t.Parallel()
	t.Skip("upstream u-root tar has filepath.Rel bug in create mode — revisit when fixed")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile.txt")
	archivePath := filepath.Join(tmpDir, "output.tar")

	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
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

	cmd := newTarCommand()
	err := cmd.Run(ctx, []string{"tar", "-c", "-f", archivePath, testFile})
	if err != nil {
		t.Fatalf("tar create returned error: %v", err)
	}

	// If this test runs (skip removed), verify the archive was created
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive was not created: %v", err)
	}
}

func TestTarCommand_Run_NoFlags(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTarCommand()
	err := cmd.Run(ctx, []string{"tar"})

	if err == nil {
		t.Error("tar with no flags should error")
	}
}
