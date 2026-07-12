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

func TestInvowkmodEditMutationLeadingWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "mixed leading whitespace",
			line: "\t  {",
			want: "\t  ",
		},
		{
			name: "all whitespace",
			line: "\t ",
			want: "\t ",
		},
		{
			name: "no leading whitespace",
			line: "field",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := leadingWhitespace(tt.line); got != tt.want {
				t.Fatalf("leadingWhitespace(%q) = %q, want %q", tt.line, got, tt.want)
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
