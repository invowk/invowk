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

func TestSelectSourceModuleOrdinaryRootRepository(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeSourceModule(t, repoDir, "io.example.tools")

	selected := mustSelectSourceModule(t, repoDir, ModuleRef{})
	assertSelectedSourceModulePath(t, selected, repoDir)
	if selected.metadata.Module != "io.example.tools" {
		t.Fatalf("module = %q, want io.example.tools", selected.metadata.Module)
	}
}

func TestSelectSourceModuleInvowkmodGitRepositoryRoot(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	repoDir := filepath.Join(parent, "io.example.tools.invowkmod.git")
	writeSourceModule(t, repoDir, "io.example.tools")

	selected := mustSelectSourceModule(t, repoDir, ModuleRef{})
	assertSelectedSourceModulePath(t, selected, repoDir)
}

func TestSelectSourceModuleExplicitSubpath(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	moduleDir := filepath.Join(repoDir, "modules", "io.example.tools.invowkmod")
	writeSourceModule(t, moduleDir, "io.example.tools")

	selected := mustSelectSourceModule(t, repoDir, ModuleRef{Path: "modules/io.example.tools.invowkmod"})
	assertSelectedSourceModulePath(t, selected, moduleDir)
}

func TestSelectSourceModuleChildModulesRequireExplicitPath(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeSourceModule(t, filepath.Join(repoDir, "io.example.tools.invowkmod"), "io.example.tools")

	assertSelectSourceModuleError(t, repoDir, ModuleRef{}, ErrAmbiguousModuleSource, "set path to one of [io.example.tools.invowkmod]")
}

func TestSelectSourceModuleSubpathMustEndWithModuleSuffix(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeSourceModule(t, filepath.Join(repoDir, "modules", "tools"), "io.example.tools")

	assertSelectSourceModuleError(t, repoDir, ModuleRef{Path: "modules/tools"}, ErrInvalidModuleSubpath, "")
}

func TestSelectSourceModuleRejectsTraversal(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()

	assertSelectSourceModuleError(t, repoDir, ModuleRef{Path: "../io.example.tools.invowkmod"}, ErrInvalidModuleSubpath, "")
}

func TestSelectSourceModuleSubpathBasenameMustMatchModuleIdentity(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	moduleDir := filepath.Join(repoDir, "io.example.tools.invowkmod")
	writeSourceModule(t, moduleDir, "io.example.other")

	assertSelectSourceModuleError(t, repoDir, ModuleRef{Path: "io.example.tools.invowkmod"}, ErrModuleSubpathIdentityMismatch, "")
}

func mustSelectSourceModule(t *testing.T, repoDir string, ref ModuleRef) selectedSourceModule {
	t.Helper()

	selected, err := selectSourceModule(types.FilesystemPath(repoDir), ref)
	if err != nil {
		t.Fatalf("selectSourceModule() error = %v", err)
	}
	return selected
}

func assertSelectedSourceModulePath(t *testing.T, selected selectedSourceModule, want string) {
	t.Helper()

	if selected.Path() != types.FilesystemPath(want) {
		t.Fatalf("selected path = %q, want %q", selected.Path(), want)
	}
}

func assertSelectSourceModuleError(t *testing.T, repoDir string, ref ModuleRef, want error, wantMessage string) {
	t.Helper()

	_, err := selectSourceModule(types.FilesystemPath(repoDir), ref)
	if err == nil {
		t.Fatalf("selectSourceModule() error = nil, want %v", want)
	}
	if !errors.Is(err, want) {
		t.Fatalf("selectSourceModule() error = %v, want %v", err, want)
	}
	if wantMessage != "" && !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error = %q, want message containing %q", err, wantMessage)
	}
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
