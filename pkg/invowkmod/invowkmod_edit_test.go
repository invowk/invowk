// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// removeRequirementCase is one row of the table that TestRemoveRequirement and
// TestRemoveRequirementIfPresent share. wantWritten says whether the removal
// replaces the file; a row without a write must leave it byte-identical.
type removeRequirementCase struct {
	name            string
	content         string
	missingFile     bool
	gitURL          GitURL
	path            SubdirectoryPath
	wantWritten     bool
	wantContains    []string
	wantNotContains []string
}

func TestAddRequirement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		content           string
		req               ModuleRef
		missingFile       bool
		wantErr           bool
		wantSentinel      error
		wantFieldSentinel error
		wantContains      []string
		wantUnchanged     bool
	}{
		{name: "add to existing requires block", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
]
`, req: ModuleRef{GitURL: "https://github.com/user/new.git", Version: "^2.0.0"}, wantContains: []string{`git_url: "https://github.com/user/existing.git"`, `git_url: "https://github.com/user/new.git"`, `version: "^2.0.0"`}},
		{name: "add when no requires block exists", content: `module: "mymodule"
version: "1.0.0"
description: "Test module"
`, req: ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"}, wantContains: []string{"requires: [", `git_url: "https://github.com/user/tools.git"`, `module: "mymodule"`}},
		{name: "duplicate detection", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`, req: ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^2.0.0"}, wantErr: true, wantSentinel: ErrModuleAlreadyExists},
		{name: "reject declaration-invalid version before editing file", content: `module: "mymodule"
version: "1.0.0"
`, req: ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "v1.0.0"}, wantErr: true, wantFieldSentinel: ErrInvalidSemVerConstraint, wantUnchanged: true},
		{name: "preserves existing comments", content: `// This is a module comment
module: "mymodule"
version: "1.0.0"
// This comment should survive
description: "A test module"
`, req: ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"}, wantContains: []string{"// This is a module comment", "// This comment should survive"}},
		{name: "with alias and path fields", content: `module: "mymodule"
version: "1.0.0"
`, req: ModuleRef{GitURL: "https://github.com/user/monorepo.git", Version: "^1.0.0", Alias: "myalias", Path: "packages/utils"}, wantContains: []string{`alias:   "myalias"`, `path:    "packages/utils"`}},
		{name: "file not found", missingFile: true, req: ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "invowkmod.cue")
			if tt.missingFile {
				path = filepath.Join(t.TempDir(), "nonexistent.cue")
			} else if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			err := AddRequirement(types.FilesystemPath(path), tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddRequirement() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
				t.Errorf("AddRequirement() error = %v, want %v", err, tt.wantSentinel)
			}
			if tt.wantFieldSentinel != nil {
				var refErr *InvalidModuleRefError
				if !errors.As(err, &refErr) {
					t.Fatalf("AddRequirement() error = %T %v, want InvalidModuleRefError", err, err)
				}
				if len(refErr.FieldErrors) != 1 || !errors.Is(refErr.FieldErrors[0], tt.wantFieldSentinel) {
					t.Fatalf("field errors = %v, want %v", refErr.FieldErrors, tt.wantFieldSentinel)
				}
			}
			if tt.missingFile {
				return
			}
			result, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read result: %v", readErr)
			}
			got := string(result)
			if tt.wantUnchanged && got != tt.content {
				t.Fatalf("file changed after invalid requirement:\n%s", result)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("result does not contain %q:\n%s", want, got)
				}
			}
		})
	}
}

func removeRequirementCases() []removeRequirementCase {
	return []removeRequirementCase{
		{name: "remove single entry from multi-entry requires", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/user/utils.git"
		version: "^2.0.0"
	},
]
`, gitURL: "https://github.com/user/tools.git", wantWritten: true, wantContains: []string{"utils.git", "requires:"}, wantNotContains: []string{"tools.git"}},
		{name: "remove the only entry removes entire block", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`, gitURL: "https://github.com/user/tools.git", wantWritten: true, wantContains: []string{`module: "mymodule"`}, wantNotContains: []string{"requires"}},
		{name: "remove with path matching (monorepo)", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path:    "packages/a"
	},
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path:    "packages/b"
	},
]
`, gitURL: "https://github.com/user/monorepo.git", path: "packages/a", wantWritten: true, wantContains: []string{"packages/b"}, wantNotContains: []string{"packages/a"}},
		{name: "preserve comments before and after requires block", content: `// Header comment
module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]

// Footer comment
`, gitURL: "https://github.com/user/tools.git", wantWritten: true, wantContains: []string{"// Header comment", "// Footer comment"}},
		{name: "no match is a no-op", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`, gitURL: "https://github.com/other/repo.git", wantContains: []string{"tools.git"}},
		{name: "no requires block is a no-op", content: `module: "mymodule"
version: "1.0.0"
`, gitURL: "https://github.com/user/tools.git", wantContains: []string{`module: "mymodule"`}},
		{name: "file does not exist is a no-op", missingFile: true, gitURL: "https://github.com/user/tools.git"},
	}
}

func TestRemoveRequirement(t *testing.T) {
	t.Parallel()

	for _, tt := range removeRequirementCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "invowkmod.cue")
			if tt.missingFile {
				path = filepath.Join(t.TempDir(), "nonexistent.cue")
			} else if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			if err := RemoveRequirement(types.FilesystemPath(path), tt.gitURL, tt.path); err != nil {
				t.Fatalf("RemoveRequirement() error = %v", err)
			}
			if tt.missingFile {
				return
			}
			assertRemoveRequirementContent(t, path, tt)
		})
	}
}

// TestRemoveRequirementIfPresent covers spec "Removing a requires entry
// reports whether it wrote": a write is reported only when the file was
// replaced, and a row without a write leaves the file byte-identical (or still
// absent).
func TestRemoveRequirementIfPresent(t *testing.T) {
	t.Parallel()

	for _, tt := range removeRequirementCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "invowkmod.cue")
			if !tt.missingFile {
				path = writeInvowkmodEditFixture(t, tt.content)
			}
			written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), tt.gitURL, tt.path)
			if err != nil {
				t.Fatalf("RemoveRequirementIfPresent() error = %v", err)
			}
			if written != tt.wantWritten {
				t.Fatalf("RemoveRequirementIfPresent() written = %t, want %t", written, tt.wantWritten)
			}
			switch {
			case tt.missingFile:
				if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("missing invowkmod.cue was created (stat error %v)", statErr)
				}
			case !tt.wantWritten:
				assertInvowkmodEditFile(t, path, tt.content)
			default:
				assertRemoveRequirementContent(t, path, tt)
			}
		})
	}

	t.Run("read error is not a write", func(t *testing.T) {
		t.Parallel()

		// A directory at the invowkmod.cue path fails the read with an error
		// other than "not exist".
		path := filepath.Join(t.TempDir(), "invowkmod.cue")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), "https://github.com/user/tools.git", "")
		if err == nil {
			t.Fatal("RemoveRequirementIfPresent() error = nil, want a read error")
		}
		if written {
			t.Fatal("RemoveRequirementIfPresent() written = true on a read error")
		}
		if delegateErr := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); delegateErr == nil {
			t.Fatal("RemoveRequirement() error = nil, want the delegate's read error")
		}
	})
}

func TestAddRequirementMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		req         ModuleRef
		wantErr     error
		wantErrText string
		wantContent string
	}{
		{name: "path duplicate matches last field and reports qualified identifier", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path:    "packages/tools"
	},
]
`, req: ModuleRef{GitURL: "https://github.com/user/monorepo.git", Version: "^2.0.0", Path: "packages/tools"}, wantErr: ErrModuleAlreadyExists, wantErrText: "https://github.com/user/monorepo.git#packages/tools"},
		{name: "insert preserves closing bracket and trailing content exactly", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
]

description: "after requires"
`, req: ModuleRef{GitURL: "https://github.com/user/new.git", Version: "^2.0.0"}, wantContent: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/user/new.git"
		version: "^2.0.0"
	},
]

description: "after requires"
`},
		{name: "append trims trailing blank lines and writes one complete block", content: "module: \"mymodule\"\nversion: \"1.0.0\"\n\n\n", req: ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"}, wantContent: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`},
		{name: "commented requires does not hide later real block", content: `module: "mymodule"
version: "1.0.0"
// requires: [

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
]
`, req: ModuleRef{GitURL: "https://github.com/user/new.git", Version: "^2.0.0"}, wantContent: `module: "mymodule"
version: "1.0.0"
// requires: [

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/user/new.git"
		version: "^2.0.0"
	},
]
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeInvowkmodEditFixture(t, tt.content)
			err := AddRequirement(types.FilesystemPath(path), tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AddRequirement() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("AddRequirement() error = %q, want text %q", err, tt.wantErrText)
			}
			if tt.wantContent != "" {
				assertInvowkmodEditFile(t, path, tt.wantContent)
			}
		})
	}
}

func TestRemoveRequirementMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		gitURL      GitURL
		path        SubdirectoryPath
		wantContent string
	}{
		{name: "missing block is exact no-op", content: `module: "mymodule"
version: "1.0.0"
description: "no deps"
`, gitURL: "https://github.com/user/tools.git", wantContent: `module: "mymodule"
version: "1.0.0"
description: "no deps"
`},
		{name: "single entry removes surrounding blank lines exactly", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]

description: "after requires"
`, gitURL: "https://github.com/user/tools.git", wantContent: `module: "mymodule"
version: "1.0.0"
description: "after requires"
`},
		{name: "single entry at top keeps following content", content: `requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
module: "mymodule"
`, gitURL: "https://github.com/user/tools.git", wantContent: "module: \"mymodule\"\n"},
		{name: "multi entry removal deletes whole selected entry only", content: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/user/utils.git"
		version: "^2.0.0"
	},
	{
		git_url: "https://github.com/user/extra.git"
		version: "^3.0.0"
	},
]
`, gitURL: "https://github.com/user/utils.git", wantContent: `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
	{
		git_url: "https://github.com/user/extra.git"
		version: "^3.0.0"
	},
]
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeInvowkmodEditFixture(t, tt.content)
			if err := RemoveRequirement(types.FilesystemPath(path), tt.gitURL, tt.path); err != nil {
				t.Fatalf("RemoveRequirement() error = %v", err)
			}
			assertInvowkmodEditFile(t, path, tt.wantContent)
		})
	}
}

func writeInvowkmodEditFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "invowkmod.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// assertRemoveRequirementContent checks the file at path against the row's
// wantContains and wantNotContains.
func assertRemoveRequirementContent(t *testing.T, path string, tt removeRequirementCase) {
	t.Helper()

	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	got := string(result)
	for _, want := range tt.wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("result does not contain %q:\n%s", want, got)
		}
	}
	for _, unwanted := range tt.wantNotContains {
		if strings.Contains(got, unwanted) {
			t.Errorf("result unexpectedly contains %q:\n%s", unwanted, got)
		}
	}
}

func assertInvowkmodEditFile(t *testing.T, path, want string) {
	t.Helper()

	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(result) != want {
		t.Fatalf("file content mismatch\ngot:\n%s\nwant:\n%s", result, want)
	}
}
