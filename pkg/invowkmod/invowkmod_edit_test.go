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

func TestAddRequirement(t *testing.T) {
	t.Parallel()

	t.Run("add to existing requires block", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
]
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		req := ModuleRef{
			GitURL:  "https://github.com/user/new.git",
			Version: "^2.0.0",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if !strings.Contains(got, `git_url: "https://github.com/user/existing.git"`) {
			t.Error("existing entry should be preserved")
		}
		if !strings.Contains(got, `git_url: "https://github.com/user/new.git"`) {
			t.Error("new entry should be added")
		}
		if !strings.Contains(got, `version: "^2.0.0"`) {
			t.Error("new entry version should be present")
		}
	})

	t.Run("add when no requires block exists", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
description: "Test module"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		req := ModuleRef{
			GitURL:  "https://github.com/user/tools.git",
			Version: "^1.0.0",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if !strings.Contains(got, "requires: [") {
			t.Error("requires block should be appended")
		}
		if !strings.Contains(got, `git_url: "https://github.com/user/tools.git"`) {
			t.Error("new entry should be present")
		}
		if !strings.Contains(got, `module: "mymodule"`) {
			t.Error("original content should be preserved")
		}
	})

	t.Run("duplicate detection", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		req := ModuleRef{
			GitURL:  "https://github.com/user/tools.git",
			Version: "^2.0.0", // Different version, but same git_url
		}

		err := AddRequirement(types.FilesystemPath(path), req)
		if err == nil {
			t.Fatal("expected duplicate error, got nil")
		}
		if !errors.Is(err, ErrModuleAlreadyExists) {
			t.Errorf("expected ErrModuleAlreadyExists, got: %v", err)
		}
	})

	t.Run("reject declaration-invalid version before editing file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		req := ModuleRef{
			GitURL:  "https://github.com/user/tools.git",
			Version: "v1.0.0",
		}

		err := AddRequirement(types.FilesystemPath(path), req)
		if err == nil {
			t.Fatal("expected invalid version error, got nil")
		}
		var refErr *InvalidModuleRefError
		if !errors.As(err, &refErr) {
			t.Fatalf("expected InvalidModuleRefError, got: %v", err)
		}
		if len(refErr.FieldErrors) != 1 || !errors.Is(refErr.FieldErrors[0], ErrInvalidSemVerConstraint) {
			t.Fatalf("field errors = %v, want ErrInvalidSemVerConstraint", refErr.FieldErrors)
		}

		result, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed to read result: %v", readErr)
		}
		if string(result) != content {
			t.Fatalf("file changed after invalid requirement:\n%s", result)
		}
	})

	t.Run("preserves existing comments", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `// This is a module comment
module: "mymodule"
version: "1.0.0"
// This comment should survive
description: "A test module"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		req := ModuleRef{
			GitURL:  "https://github.com/user/tools.git",
			Version: "^1.0.0",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if !strings.Contains(got, "// This is a module comment") {
			t.Error("header comment should be preserved")
		}
		if !strings.Contains(got, "// This comment should survive") {
			t.Error("inline comment should be preserved")
		}
	})

	t.Run("with alias and path fields", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		req := ModuleRef{
			GitURL:  "https://github.com/user/monorepo.git",
			Version: "^1.0.0",
			Alias:   "myalias",
			Path:    "packages/utils",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if !strings.Contains(got, `alias:   "myalias"`) {
			t.Error("alias field should be present")
		}
		if !strings.Contains(got, `path:    "packages/utils"`) {
			t.Error("path field should be present")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent.cue")

		req := ModuleRef{
			GitURL:  "https://github.com/user/tools.git",
			Version: "^1.0.0",
		}

		err := AddRequirement(types.FilesystemPath(path), req)
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})
}

func TestRemoveRequirement(t *testing.T) {
	t.Parallel()

	t.Run("remove single entry from multi-entry requires", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
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
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if strings.Contains(got, "tools.git") {
			t.Error("removed entry should not be present")
		}
		if !strings.Contains(got, "utils.git") {
			t.Error("remaining entry should still be present")
		}
		if !strings.Contains(got, "requires:") {
			t.Error("requires block should still exist")
		}
	})

	t.Run("remove the only entry removes entire block", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if strings.Contains(got, "requires") {
			t.Error("entire requires block should be removed")
		}
		if !strings.Contains(got, `module: "mymodule"`) {
			t.Error("other content should be preserved")
		}
	})

	t.Run("remove with path matching (monorepo)", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
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
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Remove only packages/a (same git_url but different path)
		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/monorepo.git", "packages/a"); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if strings.Contains(got, "packages/a") {
			t.Error("removed entry (packages/a) should not be present")
		}
		if !strings.Contains(got, "packages/b") {
			t.Error("remaining entry (packages/b) should still be present")
		}
	})

	t.Run("preserve comments before and after requires block", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `// Header comment
module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]

// Footer comment
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if !strings.Contains(got, "// Header comment") {
			t.Error("header comment should be preserved")
		}
		if !strings.Contains(got, "// Footer comment") {
			t.Error("footer comment should be preserved")
		}
	})

	t.Run("no match is a no-op", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/other/repo.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() should be idempotent, got error = %v", err)
		}

		result, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		got := string(result)
		if !strings.Contains(got, "tools.git") {
			t.Error("original entry should still be present")
		}
	})

	t.Run("file does not exist is a no-op", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent.cue")

		err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", "")
		if err != nil {
			t.Fatalf("RemoveRequirement() on missing file should return nil, got error = %v", err)
		}
	})
}

func TestAddRequirementMutationBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("path duplicate matches last field and reports qualified identifier", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/monorepo.git"
		version: "^1.0.0"
		path:    "packages/tools"
	},
]
`)

		req := ModuleRef{
			GitURL:  "https://github.com/user/monorepo.git",
			Version: "^2.0.0",
			Path:    "packages/tools",
		}

		err := AddRequirement(types.FilesystemPath(path), req)
		if err == nil {
			t.Fatal("AddRequirement() = nil, want duplicate error")
		}
		if !errors.Is(err, ErrModuleAlreadyExists) {
			t.Fatalf("AddRequirement() error = %v, want ErrModuleAlreadyExists", err)
		}
		if !strings.Contains(err.Error(), "https://github.com/user/monorepo.git#packages/tools") {
			t.Fatalf("AddRequirement() error = %q, want path-qualified identifier", err)
		}
	})

	t.Run("insert preserves closing bracket and trailing content exactly", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
]

description: "after requires"
`)

		req := ModuleRef{
			GitURL:  "https://github.com/user/new.git",
			Version: "^2.0.0",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
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
`)
	})

	t.Run("append trims trailing blank lines and writes one complete block", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, "module: \"mymodule\"\nversion: \"1.0.0\"\n\n\n")

		req := ModuleRef{
			GitURL:  "https://github.com/user/tools.git",
			Version: "^1.0.0",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]
`)
	})

	t.Run("commented requires does not hide later real block", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `module: "mymodule"
version: "1.0.0"
// requires: [

requires: [
	{
		git_url: "https://github.com/user/existing.git"
		version: "^1.0.0"
	},
]
`)

		req := ModuleRef{
			GitURL:  "https://github.com/user/new.git",
			Version: "^2.0.0",
		}

		if err := AddRequirement(types.FilesystemPath(path), req); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
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
`)
	})
}

func TestRemoveRequirementMutationBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("missing block is exact no-op", func(t *testing.T) {
		t.Parallel()

		content := `module: "mymodule"
version: "1.0.0"
description: "no deps"
`
		path := writeInvowkmodEditFixture(t, content)

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v, want nil", err)
		}

		assertInvowkmodEditFile(t, path, content)
	})

	t.Run("single entry removes surrounding blank lines exactly", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `module: "mymodule"
version: "1.0.0"

requires: [
	{
		git_url: "https://github.com/user/tools.git"
		version: "^1.0.0"
	},
]

description: "after requires"
`)

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/tools.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
version: "1.0.0"
description: "after requires"
`)
	})

	t.Run("single entry at top keeps following content", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `requires: [
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

	t.Run("multi entry removal deletes whole selected entry only", func(t *testing.T) {
		t.Parallel()

		path := writeInvowkmodEditFixture(t, `module: "mymodule"
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
`)

		if err := RemoveRequirement(types.FilesystemPath(path), "https://github.com/user/utils.git", ""); err != nil {
			t.Fatalf("RemoveRequirement() error = %v", err)
		}

		assertInvowkmodEditFile(t, path, `module: "mymodule"
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
`)
	})
}

func TestInvowkmodEditHelperMutationBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("find requires block skips comments and reports malformed block as absent", func(t *testing.T) {
		t.Parallel()

		lines := strings.Split(`// requires: [
module: "mymodule"
requires: [
`, "\n")

		startLine, endLine, found := findRequiresBlock(lines)
		if found {
			t.Fatalf("findRequiresBlock() found block at (%d, %d), want absent", startLine, endLine)
		}
		if startLine != 0 || endLine != 0 {
			t.Fatalf("findRequiresBlock() = (%d, %d, false), want (0, 0, false)", startLine, endLine)
		}
	})

	t.Run("find entry bounds includes entry ending at block end", func(t *testing.T) {
		t.Parallel()

		lines := []string{
			"{",
			`	git_url: "https://github.com/user/tools.git"`,
			"}",
		}

		entries := findEntryBounds(lines, 0, 2)
		if len(entries) != 1 {
			t.Fatalf("findEntryBounds() returned %d entries, want 1", len(entries))
		}
		if entries[0] != (entryBounds{start: 0, end: 2}) {
			t.Fatalf("findEntryBounds()[0] = %+v, want start=0 end=2", entries[0])
		}
	})
}

func writeInvowkmodEditFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "invowkmod.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
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
