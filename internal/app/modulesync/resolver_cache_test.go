// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/modulecache"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestResolver_getCachePath(t *testing.T) {
	t.Parallel()

	resolver := newTestResolver(t)

	tests := []struct {
		name    string
		gitURL  string
		version string
		subPath string
		wantEnd string
	}{
		{
			name:    "https_url",
			gitURL:  "https://github.com/user/repo.git",
			version: "1.0.0",
			wantEnd: filepath.Join("github.com", "user", "repo", "1.0.0"),
		},
		{
			name:    "git_at_url",
			gitURL:  "git@github.com:user/repo.git",
			version: "2.0.0",
			wantEnd: filepath.Join("github.com", "user", "repo", "2.0.0"),
		},
		{
			name:    "with_subpath",
			gitURL:  "https://github.com/user/repo.git",
			version: "1.0.0",
			subPath: "modules/tools",
			wantEnd: filepath.Join("github.com", "user", "repo", "1.0.0", "modules", "tools"),
		},
		{
			name:    "without_git_suffix",
			gitURL:  "https://github.com/user/repo",
			version: "1.0.0",
			wantEnd: filepath.Join("github.com", "user", "repo", "1.0.0"),
		},
		{
			name:    "deeply_nested",
			gitURL:  "https://gitlab.com/org/group/subgroup/repo.git",
			version: "3.0.0",
			wantEnd: filepath.Join("gitlab.com", "org", "group", "subgroup", "repo", "3.0.0"),
		},
		{
			name:    "no_subpath",
			gitURL:  "https://github.com/user/repo.git",
			version: "1.0.0",
			subPath: "",
			wantEnd: filepath.Join("github.com", "user", "repo", "1.0.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolver.getCachePath(tt.gitURL, tt.version, tt.subPath)

			// The result should start with the cache directory
			absCacheDir := string(resolver.CacheDir())
			if !strings.HasPrefix(got, absCacheDir) {
				t.Errorf("result %q does not start with cache dir %q", got, absCacheDir)
			}

			// Check the suffix matches expected URL-to-path transformation
			if !strings.HasSuffix(got, tt.wantEnd) {
				t.Errorf("getCachePath() = %q, want suffix %q", got, tt.wantEnd)
			}
		})
	}
}

func TestFindModuleInDir(t *testing.T) {
	t.Parallel()

	t.Run("invowkmod_subdir_present", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modDir := filepath.Join(dir, "mymod.invowkmod")
		if err := os.MkdirAll(modDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}

		gotDir, gotName, err := modulecache.LocateModuleInDir(types.FilesystemPath(dir))
		if err != nil {
			t.Fatalf("findModuleInDir() error: %v", err)
		}
		if gotDir != types.FilesystemPath(modDir) {
			t.Errorf("dir = %q, want %q", gotDir, modDir)
		}
		if gotName != "mymod" {
			t.Errorf("name = %q, want %q", gotName, "mymod")
		}
	})

	t.Run("invowkmod_cue_at_root_with_suffix", func(t *testing.T) {
		t.Parallel()

		// Simulate a Git repo whose name ends with .invowkmod and has invowkmod.cue at root
		dir := filepath.Join(t.TempDir(), "tools.invowkmod")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte(`module: "tools"`), 0o644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		gotDir, gotName, err := modulecache.LocateModuleInDir(types.FilesystemPath(dir))
		if err != nil {
			t.Fatalf("findModuleInDir() error: %v", err)
		}
		if gotDir != types.FilesystemPath(dir) {
			t.Errorf("dir = %q, want %q", gotDir, dir)
		}
		if gotName != "tools" {
			t.Errorf("name = %q, want %q", gotName, "tools")
		}
	})

	t.Run("invowkmod_cue_at_root_without_suffix", func(t *testing.T) {
		t.Parallel()

		// Dir without .invowkmod suffix but containing invowkmod.cue
		dir := filepath.Join(t.TempDir(), "myproject")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte(`module: "proj"`), 0o644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		gotDir, gotName, err := modulecache.LocateModuleInDir(types.FilesystemPath(dir))
		if err != nil {
			t.Fatalf("findModuleInDir() error: %v", err)
		}
		if gotDir != types.FilesystemPath(dir) {
			t.Errorf("dir = %q, want %q", gotDir, dir)
		}
		// Falls back to directory name when no .invowkmod suffix
		if gotName != "myproject" {
			t.Errorf("name = %q, want %q", gotName, "myproject")
		}
	})

	t.Run("no_module_found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		_, _, err := modulecache.LocateModuleInDir(types.FilesystemPath(dir))
		if err == nil {
			t.Fatal("expected error for empty directory, got nil")
		}
		if !errors.Is(err, modulecache.ErrModuleNotFoundInDir) {
			t.Errorf("error should wrap ErrModuleNotFoundInDir, got: %v", err)
		}
	})

	t.Run("nonexistent_dir", func(t *testing.T) {
		t.Parallel()

		_, _, err := modulecache.LocateModuleInDir(types.FilesystemPath(filepath.Join(t.TempDir(), "nonexistent")))
		if err == nil {
			t.Fatal("expected error for nonexistent directory, got nil")
		}
	})
}

func TestIsSupportedGitURLPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want bool
	}{
		{"https://github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"ssh://git@github.com/user/repo.git", true},
		{"http://github.com/user/repo.git", false},
		{"ftp://example.com/repo.git", false},
		{"", false},
		{"github.com/user/repo.git", false},
		{"file:///local/repo", false},
	}

	for _, tt := range tests {
		name := tt.url
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := isSupportedGitURLPrefix(tt.url); got != tt.want {
				t.Errorf("isSupportedGitURLPrefix(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestResolver_cacheModule(t *testing.T) {
	t.Parallel()

	t.Run("already_cached", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t)

		// Create both source and destination
		srcDir := filepath.Join(t.TempDir(), "src")
		dstDir := filepath.Join(t.TempDir(), "dst")
		if mkdirErr := os.MkdirAll(srcDir, 0o755); mkdirErr != nil {
			t.Fatalf("MkdirAll(src) error: %v", mkdirErr)
		}
		if mkdirErr := os.MkdirAll(dstDir, 0o755); mkdirErr != nil {
			t.Fatalf("MkdirAll(dst) error: %v", mkdirErr)
		}

		// Should return hash when destination already exists.
		hash, cacheErr := resolver.cacheModule(srcDir, dstDir, "")
		if cacheErr != nil {
			t.Fatalf("cacheModule() error: %v", cacheErr)
		}
		if err := hash.Validate(); err != nil {
			t.Errorf("returned hash is invalid: %v", err)
		}
	})

	t.Run("fresh_cache", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t)

		srcDir := filepath.Join(t.TempDir(), "src")
		if mkdirErr := os.MkdirAll(srcDir, 0o755); mkdirErr != nil {
			t.Fatalf("MkdirAll() error: %v", mkdirErr)
		}
		if writeErr := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("hello"), 0o644); writeErr != nil {
			t.Fatalf("WriteFile() error: %v", writeErr)
		}

		dstDir := filepath.Join(t.TempDir(), "cache", "module")
		hash, cacheErr := resolver.cacheModule(srcDir, dstDir, "")
		if cacheErr != nil {
			t.Fatalf("cacheModule() error: %v", cacheErr)
		}
		if err := hash.Validate(); err != nil {
			t.Errorf("returned hash is invalid: %v", err)
		}

		// Verify file was copied
		data, readErr := os.ReadFile(filepath.Join(dstDir, "test.txt"))
		if readErr != nil {
			t.Fatalf("ReadFile() error: %v", readErr)
		}
		if string(data) != "hello" {
			t.Errorf("copied content = %q, want %q", string(data), "hello")
		}
	})

	t.Run("source_not_found", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t)

		srcDir := filepath.Join(t.TempDir(), "nonexistent")
		dstDir := filepath.Join(t.TempDir(), "dst")

		_, err := resolver.cacheModule(srcDir, dstDir, "")
		if err == nil {
			t.Fatal("expected error for nonexistent source, got nil")
		}
	})

	t.Run("hash_mismatch", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t)

		// Create a cached module directory with known content.
		dstDir := filepath.Join(t.TempDir(), "cached")
		if mkdirErr := os.MkdirAll(dstDir, 0o755); mkdirErr != nil {
			t.Fatalf("MkdirAll() error: %v", mkdirErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dstDir, "data.txt"), []byte("content"), 0o644); writeErr != nil {
			t.Fatalf("WriteFile() error: %v", writeErr)
		}

		// Pass an expected hash that doesn't match.
		wrongHash := ContentHash("sha256:0000000000000000000000000000000000000000000000000000000000000000")
		_, err := resolver.cacheModule("", dstDir, wrongHash)
		if err == nil {
			t.Fatal("expected ContentHashMismatchError, got nil")
		}
		if !errors.Is(err, invowkmod.ErrContentHashMismatch) {
			t.Errorf("expected ErrContentHashMismatch, got: %v", err)
		}
	})
}

// newTestResolver creates a Resolver with temporary directories for tests.
func newTestResolver(t *testing.T) *Resolver {
	t.Helper()
	resolver, err := NewResolver(
		types.FilesystemPath(t.TempDir()),
		types.FilesystemPath(t.TempDir()),
	)
	if err != nil {
		t.Fatalf("NewResolver() error: %v", err)
	}
	return resolver
}
