// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/testutil"
)

func TestModuleRefKey(t *testing.T) {
	tests := []struct {
		name     string
		req      ModuleRef
		expected string
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
				if !containsString(result, c) {
					t.Errorf("String() = %q, should contain %q", result, c)
				}
			}
		})
	}
}

func TestGetDefaultCacheDir(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv(ModuleCachePathEnv)
	defer func() { _ = os.Setenv(ModuleCachePathEnv, originalEnv) }() // Test cleanup; error non-critical

	t.Run("with env var", func(t *testing.T) {
		customPath := "/custom/path/to/modules"
		restoreEnv := testutil.MustSetenv(t, ModuleCachePathEnv, customPath)
		defer restoreEnv()

		result, err := GetDefaultCacheDir()
		if err != nil {
			t.Fatalf("GetDefaultCacheDir() error = %v", err)
		}
		if result != customPath {
			t.Errorf("GetDefaultCacheDir() = %q, want %q", result, customPath)
		}
	})

	t.Run("without env var", func(t *testing.T) {
		testutil.MustUnsetenv(t, ModuleCachePathEnv)

		result, err := GetDefaultCacheDir()
		if err != nil {
			t.Fatalf("GetDefaultCacheDir() error = %v", err)
		}

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".invowk", DefaultModulesDir)
		if result != expected {
			t.Errorf("GetDefaultCacheDir() = %q, want %q", result, expected)
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
		if mgr.WorkingDir != workDir {
			t.Errorf("WorkingDir = %q, want %q", mgr.WorkingDir, workDir)
		}
		if mgr.CacheDir != cacheDir {
			t.Errorf("CacheDir = %q, want %q", mgr.CacheDir, cacheDir)
		}
	})

	t.Run("with empty working dir", func(t *testing.T) {
		cacheDir := t.TempDir()

		mgr, err := NewResolver("", cacheDir)
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}
		if mgr.WorkingDir == "" {
			t.Error("WorkingDir should not be empty")
		}
	})
}

func TestComputeNamespace(t *testing.T) {
	tests := []struct {
		name       string
		moduleName string
		version    string
		alias      string
		expected   string
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
		key      string
		expected string
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

func TestExtractModuleFromInvkmod(t *testing.T) {
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
			result := extractModuleFromInvkmod(tt.content)
			if result != tt.expected {
				t.Errorf("extractModuleFromInvkmod() = %q, want %q", result, tt.expected)
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

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
