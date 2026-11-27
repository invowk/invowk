// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/fspath"
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

func TestRemoveRequirementMutationMissingBlockNoopWithEntryLikeComment(t *testing.T) {
	t.Parallel()

	content := `git_url: "https://github.com/user/tools.git" // {}
module: "mymodule"
`
	path := writeInvowkmodEditFixture(t, content)

	if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
		t.Fatalf("RemoveRequirement() error = %v", err)
	}

	assertInvowkmodEditFile(t, path, content)
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

func TestInvowkmodEditMutationCompactPathFieldMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		operation   string
		content     string
		req         ModuleRef
		gitURL      GitURL
		path        SubdirectoryPath
		wantErr     error
		wantContent string
	}{
		{name: "add detects duplicate when path shares closing brace line", operation: "add", content: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path: "packages/tools" },
]
`, req: ModuleRef{GitURL: "https://github.com/user/monorepo.git", Version: "^2.0.0", Path: "packages/tools"}, wantErr: ErrModuleAlreadyExists},
		{name: "add detects duplicate before compact adjacent entry", operation: "add", content: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/first.git"
		version: "^1.0.0"
	}, {
		git_url: "https://github.com/user/second.git"
		version: "^1.0.0"
	},
]
`, req: ModuleRef{GitURL: "https://github.com/user/first.git", Version: "^2.0.0"}, wantErr: ErrModuleAlreadyExists, wantContent: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/first.git"
		version: "^1.0.0"
	}, {
		git_url: "https://github.com/user/second.git"
		version: "^1.0.0"
	},
]
`},
		{name: "remove matches path when path shares closing brace line", operation: "remove", content: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path: "packages/tools" },
	{
		git_url: "https://github.com/user/other.git"
		version: "^1.0.0"
	},
]
`, gitURL: "https://github.com/user/monorepo.git", path: "packages/tools", wantContent: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/other.git"
		version: "^1.0.0"
	},
]
`},
		{name: "remove skips previous compact adjacent entry", operation: "remove", content: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/first.git"
		version: "^1.0.0"
	}, {
		git_url: "https://github.com/user/second.git"
		version: "^1.0.0"
	},
]
`, gitURL: "https://github.com/user/second.git", wantContent: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/first.git"
		version: "^1.0.0"
	},
]
`},
		{name: "remove preserves next compact adjacent entry indentation", operation: "remove", content: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/first.git"
		version: "^1.0.0"
	}, {
		git_url: "https://github.com/user/second.git"
		version: "^1.0.0"
	},
]
`, gitURL: "https://github.com/user/first.git", wantContent: `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/second.git"
		version: "^1.0.0"
	},
]
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeInvowkmodEditFixture(t, tt.content)
			var err error
			switch tt.operation {
			case "add":
				err = AddRequirement(types.FilesystemPath(path), tt.req)
			case "remove":
				err = RemoveRequirement(types.FilesystemPath(path), tt.gitURL, tt.path)
			default:
				t.Fatalf("unknown operation %q", tt.operation)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("%s error = %v, want %v", tt.operation, err, tt.wantErr)
			}
			if tt.wantContent != "" {
				assertInvowkmodEditFile(t, path, tt.wantContent)
			}
		})
	}
}

func testEditModuleRef() ModuleRef {
	return ModuleRef{
		GitURL:  "https://github.com/user/tools.git",
		Version: "^1.0.0",
	}
}

func TestInvowkmodEditMutationSymlinkContracts(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	t.Run("duplicate add through symlink leaves symlink and target untouched", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		content := `module: "mymodule"
requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`
		realPath := filepath.Join(root, "real.cue")
		if err := os.WriteFile(realPath, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		linkPath := filepath.Join(root, "invowkmod.cue")
		fstree.MustSymlink(t, realPath, linkPath)

		err := AddRequirement(types.FilesystemPath(linkPath), testEditModuleRef())
		if !errors.Is(err, ErrModuleAlreadyExists) {
			t.Fatalf("AddRequirement() error = %v, want ErrModuleAlreadyExists", err)
		}
		assertSymlinkPreserved(t, linkPath, realPath)
		assertInvowkmodEditFile(t, realPath, content)
	})

	t.Run("remove no-op through symlink leaves target untouched and returns written=false", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		content := `module: "mymodule"
version: "1.0.0"
`
		realPath := filepath.Join(root, "real.cue")
		if err := os.WriteFile(realPath, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		f, openErr := os.Open(realPath)
		if openErr != nil {
			t.Fatalf("Open() error = %v", openErr)
		}
		beforeInfo, statErr := f.Stat()
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
		if statErr != nil {
			t.Fatalf("Stat() error = %v", statErr)
		}

		linkPath := filepath.Join(root, "invowkmod.cue")
		fstree.MustSymlink(t, realPath, linkPath)

		written, err := RemoveRequirementIfPresent(types.FilesystemPath(linkPath), "https://github.com/user/tools.git", "")
		if err != nil {
			t.Fatalf("RemoveRequirementIfPresent() error = %v", err)
		}
		if written {
			t.Fatal("RemoveRequirementIfPresent() written = true on no-op")
		}
		assertSymlinkPreserved(t, linkPath, realPath)
		afterInfo, afterErr := os.Stat(realPath)
		if afterErr != nil {
			t.Fatalf("Stat(realPath) error = %v", afterErr)
		}
		if !os.SameFile(beforeInfo, afterInfo) {
			t.Fatal("no-op remove replaced symlink target file")
		}
	})

	t.Run("dangling symlink fails closed on both add and remove", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		missing := filepath.Join(root, "missing.cue")
		linkPath := filepath.Join(root, "invowkmod.cue")
		fstree.MustSymlink(t, missing, linkPath)

		addErr := AddRequirement(types.FilesystemPath(linkPath), testEditModuleRef())
		if !errors.Is(addErr, fs.ErrNotExist) {
			t.Fatalf("AddRequirement() error = %v, want fs.ErrNotExist", addErr)
		}
		written, rmErr := RemoveRequirementIfPresent(types.FilesystemPath(linkPath), "https://github.com/user/tools.git", "")
		if !errors.Is(rmErr, fs.ErrNotExist) || written {
			t.Fatalf("RemoveRequirementIfPresent() = (%t, %v), want (false, fs.ErrNotExist)", written, rmErr)
		}
		assertSymlinkPreserved(t, linkPath, missing)
	})

	t.Run("directory symlink fails closed on both add and remove", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		dir := filepath.Join(root, "dir")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
		linkPath := filepath.Join(root, "invowkmod.cue")
		fstree.MustSymlink(t, dir, linkPath)

		addErr := AddRequirement(types.FilesystemPath(linkPath), testEditModuleRef())
		if !errors.Is(addErr, fspath.ErrNonRegularTarget) {
			t.Fatalf("AddRequirement() error = %v, want fspath.ErrNonRegularTarget", addErr)
		}
		written, rmErr := RemoveRequirementIfPresent(types.FilesystemPath(linkPath), "https://github.com/user/tools.git", "")
		if !errors.Is(rmErr, fspath.ErrNonRegularTarget) || written {
			t.Fatalf("RemoveRequirementIfPresent() = (%t, %v), want (false, fspath.ErrNonRegularTarget)", written, rmErr)
		}
		assertSymlinkPreserved(t, linkPath, dir)
	})
}
