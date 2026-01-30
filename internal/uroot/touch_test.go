// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTouchCommand_Name(t *testing.T) {
	cmd := newTouchCommand()
	if got := cmd.Name(); got != "touch" {
		t.Errorf("Name() = %q, want %q", got, "touch")
	}
}

func TestTouchCommand_SupportedFlags(t *testing.T) {
	cmd := newTouchCommand()
	flags := cmd.SupportedFlags()

	// Should have -c flag at minimum
	hasNoCreate := false
	for _, f := range flags {
		if f.Name == "c" || f.ShortName == "c" {
			hasNoCreate = true
			break
		}
	}
	if !hasNoCreate {
		t.Error("SupportedFlags() should include -c flag")
	}
}

func TestTouchCommand_Run_CreateFile(t *testing.T) {
	tmpDir := t.TempDir()
	newFile := filepath.Join(tmpDir, "newfile.txt")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTouchCommand()
	err := cmd.Run(ctx, []string{"touch", newFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	info, err := os.Stat(newFile)
	if err != nil {
		t.Fatalf("file was not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("new file should be empty, got size %d", info.Size())
	}
}

func TestTouchCommand_Run_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTouchCommand()
	err := cmd.Run(ctx, []string{"touch", file1, file2})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	for _, f := range []string{file1, file2} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("file %s was not created: %v", f, err)
		}
	}
}

func TestTouchCommand_Run_UpdateTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "existing.txt")

	// Create file with old timestamp
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(testFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set old timestamp: %v", err)
	}

	// Get the old modification time
	infoBefore, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	modTimeBefore := infoBefore.ModTime()

	// Small delay to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTouchCommand()
	err = cmd.Run(ctx, []string{"touch", testFile})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	infoAfter, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file after touch: %v", err)
	}

	if !infoAfter.ModTime().After(modTimeBefore) {
		t.Errorf("modification time was not updated: before=%v, after=%v",
			modTimeBefore, infoAfter.ModTime())
	}
}

func TestTouchCommand_Run_NoCreate(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTouchCommand()
	// With -c, touching nonexistent file should not create it
	err := cmd.Run(ctx, []string{"touch", "-c", nonexistentFile})
	if err != nil {
		t.Errorf("touch -c on nonexistent file should not error, got: %v", err)
	}

	if _, err := os.Stat(nonexistentFile); !os.IsNotExist(err) {
		t.Error("file should not have been created with -c flag")
	}
}

func TestTouchCommand_Run_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTouchCommand()
	err := cmd.Run(ctx, []string{"touch"})
	if err == nil {
		t.Error("touch with no arguments should error")
	}
}

func TestTouchCommand_Run_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       tmpDir,
		LookupEnv: os.LookupEnv,
	})

	cmd := newTouchCommand()
	// Use relative path
	err := cmd.Run(ctx, []string{"touch", "relative.txt"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "relative.txt")); err != nil {
		t.Errorf("file was not created: %v", err)
	}
}
