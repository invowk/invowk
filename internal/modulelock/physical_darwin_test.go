// SPDX-License-Identifier: MPL-2.0

//go:build darwin

package modulelock_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/types"
)

// TestPhysicalDirDarwinCaseNormalization checks that uppercase, lowercase, and
// mixed-case spellings of a directory on a case-insensitive macOS volume all
// resolve to the same canonical on-disk case path via F_GETPATH.
func TestPhysicalDirDarwinCaseNormalization(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const baseName = "MixedCaseProjectDir"
	dir := filepath.Join(root, baseName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	lowercasePath := filepath.Join(root, strings.ToLower(baseName))
	if _, statErr := os.Stat(lowercasePath); errors.Is(statErr, fs.ErrNotExist) {
		t.Skipf("volume at %s is case-sensitive", root)
	} else if statErr != nil {
		t.Fatalf("Stat(%s) error = %v", lowercasePath, statErr)
	}

	locker := modulelock.NewFileLocker()
	want, err := locker.PhysicalDir(types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("PhysicalDir(%s) error = %v", dir, err)
	}
	if filepath.Base(string(want)) != baseName {
		t.Fatalf("PhysicalDir(%s) = %s, want canonical base name %q", dir, want, baseName)
	}

	variants := []struct {
		name string
		path string
	}{
		{name: "lowercase", path: lowercasePath},
		{name: "uppercase", path: filepath.Join(root, strings.ToUpper(baseName))},
		{name: "mixed_case", path: filepath.Join(root, "mIxEdCaSePrOjEcTdIr")},
	}
	for _, tt := range variants {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, varErr := locker.PhysicalDir(types.FilesystemPath(tt.path))
			if varErr != nil {
				t.Fatalf("PhysicalDir(%s) error = %v", tt.path, varErr)
			}
			if got != want {
				t.Fatalf("PhysicalDir(%s) = %s, want %s", tt.path, got, want)
			}
		})
	}
}
