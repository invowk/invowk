// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestModuleRefKey(t *testing.T) {
	tests := []struct {
		name     string
		req      ModuleRef
		expected ModuleRefKey
	}{
		{
			name: "simple URL",
			req: ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			expected: "https://github.com/user/repo.git",
		},
		{
			name: "URL with path",
			req: ModuleRef{
				GitURL:  "https://github.com/user/monorepo.git",
				Version: "^1.0.0",
				Path:    "packages/module1",
			},
			expected: "https://github.com/user/monorepo.git#packages/module1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.req.Key()
			if result != tt.expected {
				t.Errorf("Key() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestModuleRefString(t *testing.T) {
	tests := []struct {
		name     string
		req      ModuleRef
		contains []string
	}{
		{
			name: "simple requirement",
			req: ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			contains: []string{"github.com/user/repo.git", "^1.0.0"},
		},
		{
			name: "with alias",
			req: ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Alias:   "myalias",
			},
			contains: []string{"github.com/user/repo.git", "^1.0.0", "alias:", "myalias"},
		},
		{
			name: "with path",
			req: ModuleRef{
				GitURL:  "https://github.com/user/monorepo.git",
				Version: "~2.0.0",
				Path:    "packages/module1",
			},
			contains: []string{"github.com/user/monorepo.git", "#packages/module1", "~2.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.req.String()
			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("String() = %q, should contain %q", result, c)
				}
			}
		})
	}
}

func TestGetDefaultCacheDir(t *testing.T) {
	t.Parallel()

	t.Run("with env var", func(t *testing.T) {
		t.Parallel()

		customPath := "/custom/path/to/modules"
		result, err := GetDefaultCacheDirWith(func(key string) string {
			if key == ModuleCachePathEnv {
				return customPath
			}
			return ""
		})
		if err != nil {
			t.Fatalf("GetDefaultCacheDirWith() error = %v", err)
		}
		if result != customPath {
			t.Errorf("GetDefaultCacheDirWith() = %q, want %q", result, customPath)
		}
	})

	t.Run("without env var", func(t *testing.T) {
		t.Parallel()

		result, err := GetDefaultCacheDirWith(func(string) string { return "" })
		if err != nil {
			t.Fatalf("GetDefaultCacheDirWith() error = %v", err)
		}

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".invowk", DefaultModulesDir)
		if result != expected {
			t.Errorf("GetDefaultCacheDirWith() = %q, want %q", result, expected)
		}
	})
}

func TestNewResolver(t *testing.T) {
	t.Run("with valid directories", func(t *testing.T) {
		workDir := t.TempDir()
		cacheDir := t.TempDir()

		mgr, err := NewResolver(workDir, cacheDir)
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}
		if mgr == nil {
			t.Fatal("NewResolver() returned nil")
		}
		if string(mgr.WorkingDir()) != workDir {
			t.Errorf("WorkingDir() = %q, want %q", mgr.WorkingDir(), workDir)
		}
		if string(mgr.CacheDir()) != cacheDir {
			t.Errorf("CacheDir() = %q, want %q", mgr.CacheDir(), cacheDir)
		}
	})

	t.Run("with empty working dir", func(t *testing.T) {
		cacheDir := t.TempDir()

		mgr, err := NewResolver("", cacheDir)
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}
		if mgr.WorkingDir() == "" {
			t.Error("WorkingDir() should not be empty")
		}
	})
}

func TestComputeNamespace(t *testing.T) {
	tests := []struct {
		name       string
		moduleName ModuleShortName
		version    string
		alias      ModuleAlias
		expected   ModuleNamespace
	}{
		{
			name:       "without alias",
			moduleName: "mymodule",
			version:    "1.2.3",
			alias:      "",
			expected:   "mymodule@1.2.3",
		},
		{
			name:       "with alias",
			moduleName: "mymodule",
			version:    "1.2.3",
			alias:      "mp",
			expected:   "mp",
		},
		{
			name:       "version with v prefix",
			moduleName: "tools",
			version:    "v2.0.0",
			alias:      "",
			expected:   "tools@v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeNamespace(tt.moduleName, tt.version, tt.alias)
			if result != tt.expected {
				t.Errorf("computeNamespace() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractModuleName(t *testing.T) {
	tests := []struct {
		name     string
		key      ModuleRefKey
		expected ModuleShortName
	}{
		{
			name:     "github URL",
			key:      "github.com/user/mymodule",
			expected: "mymodule",
		},
		{
			name:     "with .git suffix",
			key:      "github.com/user/mymodule.git",
			expected: "mymodule",
		},
		{
			name:     "with subpath",
			key:      "github.com/user/monorepo#packages/module1",
			expected: "monorepo",
		},
		{
			name:     "simple name",
			key:      "mymodule",
			expected: "mymodule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractModuleName(tt.key)
			if result != tt.expected {
				t.Errorf("extractModuleName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractModuleFromInvowkmod(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "simple module",
			content: `module: "mymodule"
cmds: []`,
			expected: "mymodule",
		},
		{
			name: "dotted module (RDNS)",
			content: `module: "com.example.mymodule"
version: "1.0.0"
cmds: []`,
			expected: "com.example.mymodule",
		},
		{
			name:     "no module",
			content:  `cmds: []`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractModuleFromInvowkmod(tt.content)
			if result != tt.expected {
				t.Errorf("extractModuleFromInvowkmod() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dest")

	// Create test files
	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	subFile := filepath.Join(subDir, "sub.txt")
	if err := os.WriteFile(subFile, []byte("sub content"), 0o644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Copy
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir() error = %v", err)
	}

	// Verify
	dstFile := filepath.Join(dstDir, "test.txt")
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Copied content = %q, want %q", string(content), "test content")
	}

	dstSubFile := filepath.Join(dstDir, "subdir", "sub.txt")
	subContent, err := os.ReadFile(dstSubFile)
	if err != nil {
		t.Fatalf("Failed to read copied sub file: %v", err)
	}
	if string(subContent) != "sub content" {
		t.Errorf("Copied sub content = %q, want %q", string(subContent), "sub content")
	}
}

func TestIsGitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "https URL", s: "https://github.com/user/repo.git", want: true},
		{name: "git@ URL", s: "git@github.com:user/repo.git", want: true},
		{name: "ssh:// URL", s: "ssh://git@github.com/user/repo.git", want: true},
		{name: "bare name", s: "mymodule", want: false},
		{name: "namespace with version", s: "mymodule@1.2.3", want: false},
		{name: "alias", s: "myalias", want: false},
		{name: "empty string", s: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isGitURL(tt.s)
			if got != tt.want {
				t.Errorf("isGitURL(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestResolveIdentifier(t *testing.T) {
	t.Parallel()

	modules := map[ModuleRefKey]LockedModule{
		"https://github.com/user/tools.git": {
			GitURL:    "https://github.com/user/tools.git",
			Version:   "^1.0.0",
			Namespace: "tools@1.2.3",
		},
		"https://github.com/user/utils.git": {
			GitURL:    "https://github.com/user/utils.git",
			Version:   "^2.0.0",
			Namespace: "myalias",
			Alias:     "myalias",
		},
		"https://github.com/user/monorepo.git#packages/a": {
			GitURL:    "https://github.com/user/monorepo.git",
			Version:   "^1.0.0",
			Namespace: "pkga@1.0.0",
			Path:      "packages/a",
		},
		"https://github.com/user/monorepo.git#packages/b": {
			GitURL:    "https://github.com/user/monorepo.git",
			Version:   "^1.0.0",
			Namespace: "pkgb@1.0.0",
			Path:      "packages/b",
		},
	}

	tests := []struct {
		name       string
		identifier string
		wantKeys   []ModuleRefKey
		wantErr    bool
		wantAmbig  bool
	}{
		{
			name:       "git URL exact match",
			identifier: "https://github.com/user/tools.git",
			wantKeys:   []ModuleRefKey{"https://github.com/user/tools.git"},
		},
		{
			name:       "git URL prefix matches monorepo entries",
			identifier: "https://github.com/user/monorepo.git",
			wantKeys: []ModuleRefKey{
				"https://github.com/user/monorepo.git#packages/a",
				"https://github.com/user/monorepo.git#packages/b",
			},
		},
		{
			name:       "exact namespace (alias)",
			identifier: "myalias",
			wantKeys:   []ModuleRefKey{"https://github.com/user/utils.git"},
		},
		{
			name:       "namespace prefix (bare module name)",
			identifier: "tools",
			wantKeys:   []ModuleRefKey{"https://github.com/user/tools.git"},
		},
		{
			name:       "exact lock key",
			identifier: "https://github.com/user/monorepo.git#packages/a",
			wantKeys:   []ModuleRefKey{"https://github.com/user/monorepo.git#packages/a"},
		},
		{
			name:       "no match",
			identifier: "nonexistent",
			wantErr:    true,
		},
		{
			name:       "no match git URL",
			identifier: "https://github.com/other/repo.git",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			keys, err := resolveIdentifier(tt.identifier, modules)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveIdentifier(%q) = %v, want error", tt.identifier, keys)
				}
				if tt.wantAmbig {
					var ambigErr *AmbiguousIdentifierError
					if !errors.As(err, &ambigErr) {
						t.Errorf("expected AmbiguousIdentifierError, got %T: %v", err, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveIdentifier(%q) error = %v", tt.identifier, err)
			}

			// Sort both for stable comparison
			slices.Sort(keys)
			slices.Sort(tt.wantKeys)
			if !slices.Equal(keys, tt.wantKeys) {
				t.Errorf("resolveIdentifier(%q) = %v, want %v", tt.identifier, keys, tt.wantKeys)
			}
		})
	}
}

func TestResolveIdentifierAmbiguous(t *testing.T) {
	t.Parallel()

	// Two modules with same namespace prefix
	modules := map[ModuleRefKey]LockedModule{
		"https://github.com/orgA/tools.git": {
			GitURL:    "https://github.com/orgA/tools.git",
			Namespace: "tools@1.0.0",
		},
		"https://github.com/orgB/tools.git": {
			GitURL:    "https://github.com/orgB/tools.git",
			Namespace: "tools@2.0.0",
		},
	}

	_, err := resolveIdentifier("tools", modules)
	if err == nil {
		t.Fatal("expected ambiguous error, got nil")
	}

	var ambigErr *AmbiguousIdentifierError
	if !errors.As(err, &ambigErr) {
		t.Fatalf("expected AmbiguousIdentifierError, got %T: %v", err, err)
	}
	if ambigErr.Identifier != "tools" {
		t.Errorf("Identifier = %q, want %q", ambigErr.Identifier, "tools")
	}
	if len(ambigErr.Matches) != 2 {
		t.Errorf("len(Matches) = %d, want 2", len(ambigErr.Matches))
	}
}

func TestRemoveByNamespace(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cacheDir := t.TempDir()

	// Write a lock file with known entries
	lock := NewLockFile()
	lock.Modules["https://github.com/user/tools.git"] = LockedModule{
		GitURL:          "https://github.com/user/tools.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.2.3",
		GitCommit:       "abc123",
		Namespace:       "tools@1.2.3",
	}
	lock.Modules["https://github.com/user/utils.git"] = LockedModule{
		GitURL:          "https://github.com/user/utils.git",
		Version:         "^2.0.0",
		ResolvedVersion: "2.0.0",
		GitCommit:       "def456",
		Alias:           "myalias",
		Namespace:       "myalias",
	}

	lockPath := filepath.Join(workDir, LockFileName)
	if err := lock.Save(lockPath); err != nil {
		t.Fatalf("failed to save test lock file: %v", err)
	}

	resolver, err := NewResolver(workDir, cacheDir)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	// Remove by alias namespace
	results, err := resolver.Remove(context.Background(), "myalias")
	if err != nil {
		t.Fatalf("Remove(myalias) error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Remove(myalias) returned %d results, want 1", len(results))
	}
	if results[0].RemovedEntry.Namespace != "myalias" {
		t.Errorf("RemovedEntry.Namespace = %q, want %q", results[0].RemovedEntry.Namespace, "myalias")
	}

	// Verify lock file was updated
	reloaded, err := LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	if len(reloaded.Modules) != 1 {
		t.Errorf("lock file has %d modules after remove, want 1", len(reloaded.Modules))
	}
	if _, ok := reloaded.Modules["https://github.com/user/utils.git"]; ok {
		t.Error("utils module should have been removed from lock file")
	}
	if _, ok := reloaded.Modules["https://github.com/user/tools.git"]; !ok {
		t.Error("tools module should still be in lock file")
	}
}

func TestAddWritesLockFile(t *testing.T) {
	t.Parallel()

	t.Run("writes to fresh lock file", func(t *testing.T) {
		t.Parallel()

		workDir := t.TempDir()
		lockPath := filepath.Join(workDir, LockFileName)

		// Simulate the lock-file-write path that Add() executes after resolution.
		// Add() calls resolveOne() (which requires real Git), then persists via
		// LoadLockFile → AddModule → Save. We test this persistence path directly.
		resolved := &ResolvedModule{
			ModuleRef: ModuleRef{
				GitURL:  "https://github.com/user/tools.git",
				Version: "^1.0.0",
			},
			ResolvedVersion: "1.2.3",
			GitCommit:       "abc123def456",
			Namespace:       "tools@1.2.3",
			ModuleName:      "tools",
		}

		lock, err := LoadLockFile(lockPath)
		if err != nil {
			t.Fatalf("LoadLockFile() error = %v", err)
		}
		lock.AddModule(resolved)
		if saveErr := lock.Save(lockPath); saveErr != nil {
			t.Fatalf("Save() error = %v", saveErr)
		}

		// Verify the lock file was created and contains the expected entry
		reloaded, err := LoadLockFile(lockPath)
		if err != nil {
			t.Fatalf("LoadLockFile() after save error = %v", err)
		}
		if len(reloaded.Modules) != 1 {
			t.Fatalf("lock file has %d modules, want 1", len(reloaded.Modules))
		}

		key := ModuleRefKey("https://github.com/user/tools.git")
		entry, ok := reloaded.Modules[key]
		if !ok {
			t.Fatalf("lock file missing key %q", key)
		}
		if entry.GitURL != "https://github.com/user/tools.git" {
			t.Errorf("GitURL = %q, want %q", entry.GitURL, "https://github.com/user/tools.git")
		}
		if entry.ResolvedVersion != "1.2.3" {
			t.Errorf("ResolvedVersion = %q, want %q", entry.ResolvedVersion, "1.2.3")
		}
		if entry.GitCommit != "abc123def456" {
			t.Errorf("GitCommit = %q, want %q", entry.GitCommit, "abc123def456")
		}
		if entry.Namespace != "tools@1.2.3" {
			t.Errorf("Namespace = %q, want %q", entry.Namespace, "tools@1.2.3")
		}
	})

	t.Run("appends to existing lock file", func(t *testing.T) {
		t.Parallel()

		workDir := t.TempDir()
		lockPath := filepath.Join(workDir, LockFileName)

		// Pre-populate lock file with an existing entry
		existing := NewLockFile()
		existing.Modules["https://github.com/user/utils.git"] = LockedModule{
			GitURL:          "https://github.com/user/utils.git",
			Version:         "^2.0.0",
			ResolvedVersion: "2.1.0",
			GitCommit:       "existing789",
			Namespace:       "utils@2.1.0",
		}
		if err := existing.Save(lockPath); err != nil {
			t.Fatalf("failed to save existing lock file: %v", err)
		}

		// Now simulate Add() writing a second module
		resolved := &ResolvedModule{
			ModuleRef: ModuleRef{
				GitURL:  "https://github.com/user/tools.git",
				Version: "^1.0.0",
				Alias:   "mytools",
				Path:    "packages/core",
			},
			ResolvedVersion: "1.5.0",
			GitCommit:       "newcommit456",
			Namespace:       "mytools",
			ModuleName:      "tools",
		}

		lock, err := LoadLockFile(lockPath)
		if err != nil {
			t.Fatalf("LoadLockFile() error = %v", err)
		}
		lock.AddModule(resolved)
		if saveErr := lock.Save(lockPath); saveErr != nil {
			t.Fatalf("Save() error = %v", saveErr)
		}

		// Verify both entries are present
		reloaded, err := LoadLockFile(lockPath)
		if err != nil {
			t.Fatalf("LoadLockFile() after save error = %v", err)
		}
		if len(reloaded.Modules) != 2 {
			t.Fatalf("lock file has %d modules, want 2", len(reloaded.Modules))
		}

		// Verify existing entry is preserved
		utilsKey := ModuleRefKey("https://github.com/user/utils.git")
		utils, ok := reloaded.Modules[utilsKey]
		if !ok {
			t.Fatal("existing utils entry should be preserved")
		}
		if utils.ResolvedVersion != "2.1.0" {
			t.Errorf("utils ResolvedVersion = %q, want %q", utils.ResolvedVersion, "2.1.0")
		}

		// Verify new entry with alias and path
		toolsKey := ModuleRefKey("https://github.com/user/tools.git#packages/core")
		tools, ok := reloaded.Modules[toolsKey]
		if !ok {
			t.Fatalf("new tools entry missing, expected key %q", toolsKey)
		}
		if tools.Alias != "mytools" {
			t.Errorf("Alias = %q, want %q", tools.Alias, "mytools")
		}
		if tools.Path != "packages/core" {
			t.Errorf("Path = %q, want %q", tools.Path, "packages/core")
		}
		if tools.Namespace != "mytools" {
			t.Errorf("Namespace = %q, want %q", tools.Namespace, "mytools")
		}
	})
}
