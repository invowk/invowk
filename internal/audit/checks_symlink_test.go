// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestSymlinkChecker_NoModules(t *testing.T) {
	t.Parallel()

	sc := &ScanContext{}
	checker := NewSymlinkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("no modules should produce 0 findings, got %d", len(findings))
	}
}

func TestSymlinkChecker_DetectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests require Unix")
	}
	t.Parallel()

	// Create a module directory with a symlink.
	modDir := t.TempDir()
	target := filepath.Join(modDir, "real.txt")
	link := filepath.Join(modDir, "link.txt")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	sc := newSymlinkTestScanContext(t, modDir)

	checker := NewSymlinkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasSymlink := false
	for _, f := range findings {
		if f.Title == "Symlink found in module directory" {
			hasSymlink = true
		}
	}
	if !hasSymlink {
		t.Error("expected symlink detection finding")
	}
}

func TestSymlinkChecker_DetectsExternalTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests require Unix")
	}
	t.Parallel()

	// Create a module directory with a symlink pointing outside.
	modDir := t.TempDir()
	externalDir := t.TempDir()
	externalFile := filepath.Join(externalDir, "external.txt")
	link := filepath.Join(modDir, "escape.txt")

	if err := os.WriteFile(externalFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalFile, link); err != nil {
		t.Fatal(err)
	}

	sc := newSymlinkTestScanContext(t, modDir)

	checker := NewSymlinkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasBoundaryEscape := false
	for _, f := range findings {
		if f.Title == "Symlink points outside module boundary" {
			hasBoundaryEscape = true
		}
	}
	if !hasBoundaryEscape {
		t.Error("expected boundary escape finding")
	}
}

func TestSymlinkChecker_DanglingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests require Unix")
	}
	t.Parallel()

	modDir := t.TempDir()
	link := filepath.Join(modDir, "dangling.txt")
	if err := os.Symlink(filepath.Join(modDir, "nonexistent.txt"), link); err != nil {
		t.Fatal(err)
	}

	sc := newSymlinkTestScanContext(t, modDir)

	checker := NewSymlinkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasDangling := false
	for _, f := range findings {
		if f.Title == "Dangling symlink in module directory" {
			hasDangling = true
		}
	}
	if !hasDangling {
		t.Error("expected dangling symlink finding")
	}
}

func TestSymlinkChecker_SymlinkChain(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests require Unix")
	}

	t.Run("long_chain_fires", func(t *testing.T) {
		t.Parallel()

		// Create a chain of 11 symlinks (exceeds maxSymlinkChainDepth of 10).
		modDir := t.TempDir()
		target := filepath.Join(modDir, "real.txt")
		if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Build the chain: link0 -> link1 -> ... -> link10 -> real.txt
		// Start from the end so each link points to the next.
		prev := target
		for i := 10; i >= 0; i-- {
			link := filepath.Join(modDir, fmt.Sprintf("link%d", i))
			if err := os.Symlink(prev, link); err != nil {
				t.Fatal(err)
			}
			prev = link
		}

		sc := newSymlinkTestScanContext(t, modDir)

		checker := NewSymlinkChecker()
		findings, err := checker.Check(t.Context(), sc)
		if err != nil {
			t.Fatal(err)
		}

		hasChain := false
		for _, f := range findings {
			if f.Title == "Symlink chain detected" {
				hasChain = true
			}
		}
		if !hasChain {
			t.Error("expected symlink chain finding for 11-link chain")
		}
	})

	t.Run("short_chain_does_not_fire", func(t *testing.T) {
		t.Parallel()

		// Create a chain of 5 symlinks (under maxSymlinkChainDepth of 10).
		modDir := t.TempDir()
		target := filepath.Join(modDir, "real.txt")
		if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		prev := target
		for i := 4; i >= 0; i-- {
			link := filepath.Join(modDir, fmt.Sprintf("link%d", i))
			if err := os.Symlink(prev, link); err != nil {
				t.Fatal(err)
			}
			prev = link
		}

		sc := newSymlinkTestScanContext(t, modDir)

		checker := NewSymlinkChecker()
		findings, err := checker.Check(t.Context(), sc)
		if err != nil {
			t.Fatal(err)
		}

		for _, f := range findings {
			if f.Title == "Symlink chain detected" {
				t.Error("unexpected symlink chain finding for 5-link chain")
			}
		}
	})
}

func TestSymlinkChecker_NoSymlinks(t *testing.T) {
	t.Parallel()

	modDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modDir, "script.sh"), []byte("echo ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	sc := newSymlinkTestScanContext(t, modDir)

	checker := NewSymlinkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("no symlinks should produce 0 findings, got %d", len(findings))
	}
}

func newSymlinkTestScanContext(t *testing.T, modDir string) *ScanContext {
	t.Helper()

	symlinks, scanErr := scanModuleSymlinks(types.FilesystemPath(modDir))
	return &ScanContext{
		modules: []*ScannedModule{{
			Path:           types.FilesystemPath(modDir),
			SurfaceID:      "testmod",
			Module:         &invowkmod.Module{Metadata: &invowkmod.Invowkmod{}},
			Symlinks:       symlinks,
			SymlinkScanErr: scanErr,
		}},
	}
}
