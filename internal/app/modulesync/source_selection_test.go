// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestSelectSourceModule(t *testing.T) {
	t.Parallel()

	t.Run("ordinary root repository", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		writeSourceModule(t, repoDir, "io.example.tools")

		selected, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{})
		if err != nil {
			t.Fatalf("selectSourceModule() error = %v", err)
		}
		if selected.Path() != types.FilesystemPath(repoDir) {
			t.Fatalf("selected path = %q, want %q", selected.Path(), repoDir)
		}
		if selected.metadata.Module != "io.example.tools" {
			t.Fatalf("module = %q, want io.example.tools", selected.metadata.Module)
		}
	})

	t.Run("invowkmod git repository root", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		repoDir := filepath.Join(parent, "io.example.tools.invowkmod.git")
		writeSourceModule(t, repoDir, "io.example.tools")

		selected, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{})
		if err != nil {
			t.Fatalf("selectSourceModule() error = %v", err)
		}
		if selected.Path() != types.FilesystemPath(repoDir) {
			t.Fatalf("selected path = %q, want %q", selected.Path(), repoDir)
		}
	})

	t.Run("explicit subpath", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		moduleDir := filepath.Join(repoDir, "modules", "io.example.tools.invowkmod")
		writeSourceModule(t, moduleDir, "io.example.tools")

		selected, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{Path: "modules/io.example.tools.invowkmod"})
		if err != nil {
			t.Fatalf("selectSourceModule() error = %v", err)
		}
		if selected.Path() != types.FilesystemPath(moduleDir) {
			t.Fatalf("selected path = %q, want %q", selected.Path(), moduleDir)
		}
	})

	t.Run("child modules require explicit path", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		writeSourceModule(t, filepath.Join(repoDir, "io.example.tools.invowkmod"), "io.example.tools")

		_, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{})
		if err == nil {
			t.Fatal("selectSourceModule() error = nil, want ambiguous module source")
		}
		if !errors.Is(err, ErrAmbiguousModuleSource) {
			t.Fatalf("selectSourceModule() error = %v, want ErrAmbiguousModuleSource", err)
		}
		if !strings.Contains(err.Error(), "set path to one of [io.example.tools.invowkmod]") {
			t.Fatalf("error = %q, want corrective path guidance", err)
		}
	})

	t.Run("subpath must end with module suffix", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		writeSourceModule(t, filepath.Join(repoDir, "modules", "tools"), "io.example.tools")

		_, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{Path: "modules/tools"})
		if err == nil {
			t.Fatal("selectSourceModule() error = nil, want invalid module subpath")
		}
		if !errors.Is(err, ErrInvalidModuleSubpath) {
			t.Fatalf("selectSourceModule() error = %v, want ErrInvalidModuleSubpath", err)
		}
	})

	t.Run("traversal is rejected", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		_, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{Path: "../io.example.tools.invowkmod"})
		if err == nil {
			t.Fatal("selectSourceModule() error = nil, want invalid module subpath")
		}
		if !errors.Is(err, ErrInvalidModuleSubpath) {
			t.Fatalf("selectSourceModule() error = %v, want ErrInvalidModuleSubpath", err)
		}
	})

	t.Run("subpath basename must match module identity", func(t *testing.T) {
		t.Parallel()

		repoDir := t.TempDir()
		moduleDir := filepath.Join(repoDir, "io.example.tools.invowkmod")
		writeSourceModule(t, moduleDir, "io.example.other")

		_, err := selectSourceModule(types.FilesystemPath(repoDir), ModuleRef{Path: "io.example.tools.invowkmod"})
		if err == nil {
			t.Fatal("selectSourceModule() error = nil, want identity mismatch")
		}
		if !errors.Is(err, ErrModuleSubpathIdentityMismatch) {
			t.Fatalf("selectSourceModule() error = %v, want ErrModuleSubpathIdentityMismatch", err)
		}
	})
}

func writeSourceModule(t *testing.T, dir string, moduleID ModuleID) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	invowkmodContent := `module: "` + moduleID.String() + `"
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte("cmds: {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}
}
