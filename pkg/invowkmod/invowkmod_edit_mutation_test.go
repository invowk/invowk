// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestInvowkmodEditMutationReadErrorContracts(t *testing.T) {
	t.Parallel()

	t.Run("add requirement wraps missing file read error", func(t *testing.T) {
		t.Parallel()

		err := AddRequirement(types.FilesystemPath(filepath.Join(t.TempDir(), "missing.cue")), testEditModuleRef())
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("AddRequirement() error = %v, want wrapped os.ErrNotExist", err)
		}
	})

	t.Run("remove requirement reports non-missing read errors", func(t *testing.T) {
		t.Parallel()

		err := RemoveRequirement(types.FilesystemPath(t.TempDir()), "https://github.com/user/tools.git", "")
		if err == nil {
			t.Fatal("RemoveRequirement() error = nil for directory path, want read error")
		}
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("RemoveRequirement() error = %v, want non-missing read error", err)
		}
	})
}

func TestAddRequirementMutationEmptyFileAppend(t *testing.T) {
	t.Parallel()

	path := writeInvowkmodEditFixture(t, "")

	if err := AddRequirement(types.FilesystemPath(path), testEditModuleRef()); err != nil {
		t.Fatalf("AddRequirement() error = %v", err)
	}

	assertInvowkmodEditFile(t, path, `
requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`)
}

func TestRemoveRequirementMutationEOFAndBlankLineBounds(t *testing.T) {
	t.Parallel()

	t.Run("single entry at EOF preserves preceding nonblank content", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `module: "mymodule"
version: "1.0.0"
requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]`)

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
version: "1.0.0"`)
	})

	t.Run("single entry keeps leading blank line trimmed", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `
requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
module: "mymodule"
`)

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
`)
	})
}

func TestRemoveRequirementMutationRemovesFirstDuplicateMatch(t *testing.T) {
	t.Parallel()

	path := writeInvowkmodEditFixture(t, `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
		alias:   "first"
	},
	{
		git_url: "https://github.com/user/tools.git"
		version: "^2.0.0"
		alias:   "second"
	},
]
`)

	if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
		t.Fatalf("RemoveRequirement() error = %v", err)
	}

	assertInvowkmodEditFile(t, path, `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^2.0.0"
		alias:   "second"
	},
]
`)
}

func testEditModuleRef() ModuleRef {
	return ModuleRef{
		GitURL:  "https://github.com/user/tools.git",
		Version: "^1.0.0",
	}
}
