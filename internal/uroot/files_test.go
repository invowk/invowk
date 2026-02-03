// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFilesOrStdin_EmptyArgs_UsesStdin(t *testing.T) {
	stdinContent := "stdin data\n"

	var called bool
	var gotFilename string
	var gotIndex, gotTotal int

	processor := func(r io.Reader, filename string, index, total int) error {
		called = true
		gotFilename = filename
		gotIndex = index
		gotTotal = total

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if string(data) != stdinContent {
			t.Errorf("stdin content = %q, want %q", string(data), stdinContent)
		}
		return nil
	}

	err := ProcessFilesOrStdin(
		[]string{},
		strings.NewReader(stdinContent),
		t.TempDir(),
		"test",
		processor,
	)
	if err != nil {
		t.Fatalf("ProcessFilesOrStdin() returned error: %v", err)
	}
	if !called {
		t.Error("processor was not called")
	}
	if gotFilename != "-" {
		t.Errorf("filename = %q, want %q", gotFilename, "-")
	}
	if gotIndex != 0 {
		t.Errorf("index = %d, want 0", gotIndex)
	}
	if gotTotal != 0 {
		t.Errorf("total = %d, want 0", gotTotal)
	}
}

func TestProcessFilesOrStdin_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	fileContent := "file content\n"

	if err := os.WriteFile(testFile, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var gotFilename string
	var gotIndex, gotTotal int
	var gotContent string

	processor := func(r io.Reader, filename string, index, total int) error {
		gotFilename = filename
		gotIndex = index
		gotTotal = total

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		gotContent = string(data)
		return nil
	}

	err := ProcessFilesOrStdin(
		[]string{testFile},
		strings.NewReader("unused stdin"),
		tmpDir,
		"test",
		processor,
	)
	if err != nil {
		t.Fatalf("ProcessFilesOrStdin() returned error: %v", err)
	}
	if gotFilename != testFile {
		t.Errorf("filename = %q, want %q", gotFilename, testFile)
	}
	if gotIndex != 0 {
		t.Errorf("index = %d, want 0", gotIndex)
	}
	if gotTotal != 1 {
		t.Errorf("total = %d, want 1", gotTotal)
	}
	if gotContent != fileContent {
		t.Errorf("content = %q, want %q", gotContent, fileContent)
	}
}

func TestProcessFilesOrStdin_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}
	if err := os.WriteFile(file3, []byte("content3"), 0o644); err != nil {
		t.Fatalf("failed to create file3: %v", err)
	}

	type call struct {
		filename string
		index    int
		total    int
		content  string
	}
	var calls []call

	processor := func(r io.Reader, filename string, index, total int) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		calls = append(calls, call{filename, index, total, string(data)})
		return nil
	}

	err := ProcessFilesOrStdin(
		[]string{file1, file2, file3},
		strings.NewReader("unused stdin"),
		tmpDir,
		"test",
		processor,
	)
	if err != nil {
		t.Fatalf("ProcessFilesOrStdin() returned error: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("processor called %d times, want 3", len(calls))
	}

	// Verify calls in order
	expected := []call{
		{file1, 0, 3, "content1"},
		{file2, 1, 3, "content2"},
		{file3, 2, 3, "content3"},
	}
	for i, want := range expected {
		got := calls[i]
		if got.filename != want.filename {
			t.Errorf("call[%d].filename = %q, want %q", i, got.filename, want.filename)
		}
		if got.index != want.index {
			t.Errorf("call[%d].index = %d, want %d", i, got.index, want.index)
		}
		if got.total != want.total {
			t.Errorf("call[%d].total = %d, want %d", i, got.total, want.total)
		}
		if got.content != want.content {
			t.Errorf("call[%d].content = %q, want %q", i, got.content, want.content)
		}
	}
}

func TestProcessFilesOrStdin_FileNotFound(t *testing.T) {
	err := ProcessFilesOrStdin(
		[]string{"/nonexistent/file.txt"},
		strings.NewReader("unused stdin"),
		t.TempDir(),
		"testcmd",
		func(r io.Reader, filename string, index, total int) error {
			t.Error("processor should not be called for nonexistent file")
			return nil
		},
	)

	if err == nil {
		t.Fatal("ProcessFilesOrStdin() should return error for nonexistent file")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] testcmd:") {
		t.Errorf("error should have [uroot] testcmd: prefix, got: %v", err)
	}
}

func TestProcessFilesOrStdin_ProcessorError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	expectedErr := errors.New("processor failed")

	err := ProcessFilesOrStdin(
		[]string{testFile},
		strings.NewReader("unused stdin"),
		tmpDir,
		"test",
		func(r io.Reader, filename string, index, total int) error {
			return expectedErr
		},
	)

	if err == nil {
		t.Fatal("ProcessFilesOrStdin() should return processor error")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestProcessFilesOrStdin_StopsOnFirstError(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	callCount := 0
	expectedErr := errors.New("processor failed")

	err := ProcessFilesOrStdin(
		[]string{file1, file2},
		strings.NewReader("unused stdin"),
		tmpDir,
		"test",
		func(r io.Reader, filename string, index, total int) error {
			callCount++
			return expectedErr
		},
	)

	if err == nil {
		t.Fatal("ProcessFilesOrStdin() should return error")
	}
	if callCount != 1 {
		t.Errorf("processor called %d times, want 1 (should stop on first error)", callCount)
	}
}

func TestProcessFilesOrStdin_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	fileContent := "relative path content\n"

	if err := os.WriteFile(testFile, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var gotContent string

	// Use relative path "test.txt" with workDir set to tmpDir
	err := ProcessFilesOrStdin(
		[]string{"test.txt"},
		strings.NewReader("unused stdin"),
		tmpDir, // workDir for resolving relative paths
		"test",
		func(r io.Reader, filename string, index, total int) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			gotContent = string(data)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ProcessFilesOrStdin() returned error: %v", err)
	}
	if gotContent != fileContent {
		t.Errorf("content = %q, want %q", gotContent, fileContent)
	}
}

func TestProcessFilesOrStdin_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	fileContent := "absolute path content\n"

	if err := os.WriteFile(testFile, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var gotContent string

	// Use absolute path, workDir should be ignored
	err := ProcessFilesOrStdin(
		[]string{testFile}, // absolute path
		strings.NewReader("unused stdin"),
		"/some/other/dir", // should be ignored for absolute paths
		"test",
		func(r io.Reader, filename string, index, total int) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			gotContent = string(data)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ProcessFilesOrStdin() returned error: %v", err)
	}
	if gotContent != fileContent {
		t.Errorf("content = %q, want %q", gotContent, fileContent)
	}
}

func TestProcessFilesOrStdin_StdinProcessorError(t *testing.T) {
	expectedErr := errors.New("stdin processor failed")

	err := ProcessFilesOrStdin(
		[]string{}, // empty args = stdin
		strings.NewReader("stdin content"),
		t.TempDir(),
		"test",
		func(r io.Reader, filename string, index, total int) error {
			return expectedErr
		},
	)

	if err == nil {
		t.Fatal("ProcessFilesOrStdin() should return processor error for stdin")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestProcessFilesOrStdin_PreservesFilenameArgument(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file in subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use relative path argument
	relPath := filepath.Join("subdir", "test.txt")
	var gotFilename string

	err := ProcessFilesOrStdin(
		[]string{relPath},
		strings.NewReader("unused"),
		tmpDir,
		"test",
		func(r io.Reader, filename string, index, total int) error {
			gotFilename = filename
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ProcessFilesOrStdin() returned error: %v", err)
	}

	// The filename passed to processor should be the original argument,
	// not the resolved absolute path
	if gotFilename != relPath {
		t.Errorf("filename = %q, want %q (should preserve original argument)", gotFilename, relPath)
	}
}
