// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddRequirement(t *testing.T) {
	t.Parallel()

	t.Run("add to existing requires block", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := AddRequirement(path, req); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := AddRequirement(path, req); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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

		err := AddRequirement(path, req)
		if err == nil {
			t.Fatal("expected duplicate error, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention 'already exists', got: %v", err)
		}
	})

	t.Run("preserves existing comments", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := AddRequirement(path, req); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := AddRequirement(path, req); err != nil {
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

		err := AddRequirement(path, req)
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
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := RemoveRequirement(path, "https://github.com/user/tools.git", ""); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := RemoveRequirement(path, "https://github.com/user/tools.git", ""); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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
		if err := RemoveRequirement(path, "https://github.com/user/monorepo.git", "packages/a"); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := RemoveRequirement(path, "https://github.com/user/tools.git", ""); err != nil {
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
		path := filepath.Join(dir, "invkmod.cue")
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

		if err := RemoveRequirement(path, "https://github.com/other/repo.git", ""); err != nil {
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

		err := RemoveRequirement(path, "https://github.com/user/tools.git", "")
		if err != nil {
			t.Fatalf("RemoveRequirement() on missing file should return nil, got error = %v", err)
		}
	})
}
