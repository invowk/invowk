// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestContainerSafeTempDirFallsBackWhenDefaultBaseIsFile(t *testing.T) { //nolint:paralleltest // SetHomeDir mutates process home env.
	homeDir := t.TempDir()
	defer SetHomeDir(t, homeDir)()

	baseFile := filepath.Join(homeDir, "invowk-test")
	if err := os.WriteFile(baseFile, []byte("occupied"), 0o644); err != nil {
		t.Fatalf("failed to write occupied base path: %v", err)
	}

	tmpDir := ContainerSafeTempDir(t, "container-test")

	wantBaseDir := filepath.Join(homeDir, fmt.Sprintf("invowk-test-%d", os.Getpid()))
	if gotBaseDir := filepath.Dir(tmpDir); gotBaseDir != wantBaseDir {
		t.Fatalf("ContainerSafeTempDir() parent = %q, want fallback %q", gotBaseDir, wantBaseDir)
	}

	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("ContainerSafeTempDir() path is not stat-able: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("ContainerSafeTempDir() path is not a directory: %s", tmpDir)
	}

	content, err := os.ReadFile(baseFile)
	if err != nil {
		t.Fatalf("default base file was removed or changed: %v", err)
	}
	if string(content) != "occupied" {
		t.Fatalf("default base file content = %q, want %q", string(content), "occupied")
	}
}
