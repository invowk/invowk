// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestIsModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(t *testing.T) string // returns path to test
		expected bool
	}{
		{
			name: "valid module with simple name",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: true,
		},
		{
			name: "valid module with RDNS name",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "com.example.mycommands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: true,
		},
		{
			name: "invalid - missing suffix",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: false,
		},
		{
			name: "invalid - wrong suffix",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.wrong")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: false,
		},
		{
			name: "invalid - starts with number",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "123commands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: false,
		},
		{
			name: "invalid - hidden folder prefix",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				// Note: folder name itself doesn't start with dot, but the name part does
				// This tests ".hidden.invowkmod" - the prefix is ".hidden" which is invalid
				modulePath := filepath.Join(dir, ".hidden.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: false,
		},
		{
			name: "invalid - file not directory",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invowkmod")
				if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			expected: false,
		},
		{
			name: "invalid - path does not exist",
			setup: func(t *testing.T) string {
				t.Helper()
				return "/nonexistent/path/mycommands.invowkmod"
			},
			expected: false,
		},
		{
			name: "invalid - contains hyphen in name",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "my-commands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: false,
		},
		{
			name: "invalid - contains underscore in name",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "my_commands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: false,
		},
		{
			name: "valid - segment starts with uppercase",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "Com.Example.MyCommands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := tt.setup(t)
			result := IsModule(types.FilesystemPath(path))
			if result != tt.expected {
				t.Errorf("IsModule(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestParseModuleName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		folderName  string
		expectedOK  bool
		expectedVal string
	}{
		{
			name:        "simple name",
			folderName:  "mycommands.invowkmod",
			expectedOK:  true,
			expectedVal: "mycommands",
		},
		{
			name:        "RDNS name",
			folderName:  "com.example.mycommands.invowkmod",
			expectedOK:  true,
			expectedVal: "com.example.mycommands",
		},
		{
			name:        "single letter segments",
			folderName:  "a.b.c.invowkmod",
			expectedOK:  true,
			expectedVal: "a.b.c",
		},
		{
			name:        "alphanumeric segments",
			folderName:  "com.example123.mytools.invowkmod",
			expectedOK:  true,
			expectedVal: "com.example123.mytools",
		},
		{
			name:       "missing suffix",
			folderName: "mycommands",
			expectedOK: false,
		},
		{
			name:       "wrong suffix",
			folderName: "mycommands.wrong",
			expectedOK: false,
		},
		{
			name:       "empty prefix",
			folderName: ".invowkmod",
			expectedOK: false,
		},
		{
			name:       "starts with number",
			folderName: "123commands.invowkmod",
			expectedOK: false,
		},
		{
			name:       "segment starts with number",
			folderName: "com.123example.invowkmod",
			expectedOK: false,
		},
		{
			name:       "contains hyphen",
			folderName: "my-commands.invowkmod",
			expectedOK: false,
		},
		{
			name:       "contains underscore",
			folderName: "my_commands.invowkmod",
			expectedOK: false,
		},
		{
			name:       "starts with dot (hidden)",
			folderName: ".hidden.invowkmod",
			expectedOK: false,
		},
		{
			name:       "double dots",
			folderName: "com..example.invowkmod",
			expectedOK: false,
		},
		{
			name:       "ends with dot before suffix",
			folderName: "com.example..invowkmod",
			expectedOK: false,
		},
		{
			name:       "empty segment",
			folderName: "com.example..tools.invowkmod",
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseModuleName(tt.folderName)
			if tt.expectedOK {
				if err != nil {
					t.Errorf("ParseModuleName(%q) returned error: %v, expected %q", tt.folderName, err, tt.expectedVal)
				}
				if string(result) != tt.expectedVal {
					t.Errorf("ParseModuleName(%q) = %q, want %q", tt.folderName, result, tt.expectedVal)
				}
			} else if err == nil {
				t.Errorf("ParseModuleName(%q) = %q, expected error", tt.folderName, result)
			}
		})
	}
}

// Helper function to create a valid module with both invowkmod.cue and invowkfile.cue
func createValidModule(t *testing.T, dir, folderName, moduleID string) string {
	t.Helper()
	modulePath := filepath.Join(dir, folderName)
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create invowkmod.cue with metadata
	invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
	invowkmodContent := fmt.Sprintf(`module: "%s"
version: "1.0.0"
`, moduleID)
	if err := os.WriteFile(invowkmodPath, []byte(invowkmodContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create invowkfile.cue with commands
	invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte("cmds: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	return modulePath
}

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("loads valid module", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modulePath := createValidModule(t, dir, "com.example.test.invowkmod", "com.example.test")

		module, err := Load(types.FilesystemPath(modulePath))
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if module.Name() != "com.example.test" {
			t.Errorf("module.Name() = %q, want %q", module.Name(), "com.example.test")
		}

		// Verify invowkmod.cue path is set
		expectedInvowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
		if string(module.InvowkmodPath()) != expectedInvowkmodPath {
			t.Errorf("module.InvowkmodPath() = %q, want %q", module.InvowkmodPath(), expectedInvowkmodPath)
		}

		// Verify invowkfile.cue path is set
		expectedInvowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
		if string(module.InvowkfilePath()) != expectedInvowkfilePath {
			t.Errorf("module.InvowkfilePath() = %q, want %q", module.InvowkfilePath(), expectedInvowkfilePath)
		}
	})

	t.Run("loads library-only module", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modulePath := filepath.Join(dir, "mylib.invowkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		// Only create invowkmod.cue (no invowkfile.cue)
		invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
		if err := os.WriteFile(invowkmodPath, []byte(`module: "mylib"
version: "1.0.0"
`), 0o644); err != nil {
			t.Fatal(err)
		}

		module, err := Load(types.FilesystemPath(modulePath))
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if module.Name() != "mylib" {
			t.Errorf("module.Name() = %q, want %q", module.Name(), "mylib")
		}

		if !module.IsLibraryOnly {
			t.Error("module.IsLibraryOnly should be true for library-only module")
		}
	})

	t.Run("fails for module missing invowkmod.cue", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modulePath := filepath.Join(dir, "mycommands.invowkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		// Only create invowkfile.cue (missing invowkmod.cue)
		invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
		if err := os.WriteFile(invowkfilePath, []byte("cmds: []"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(types.FilesystemPath(modulePath))
		if err == nil {
			t.Error("Load() expected error for module missing invowkmod.cue, got nil")
		}
	})
}

func TestModule_ResolveScriptPath(t *testing.T) {
	t.Parallel()

	modulePath := filepath.Join(string(filepath.Separator), "home", "user", "mycommands.invowkmod")
	module := &Module{
		Path: types.FilesystemPath(modulePath),
	}

	tests := []struct {
		name       string
		scriptPath types.FilesystemPath
		expected   string
	}{
		{
			name:       "relative path with forward slashes",
			scriptPath: "scripts/build.sh",
			expected:   filepath.Join(modulePath, "scripts", "build.sh"),
		},
		{
			name:       "relative path in root",
			scriptPath: "run.sh",
			expected:   filepath.Join(modulePath, "run.sh"),
		},
		{
			name:       "nested path",
			scriptPath: "lib/utils/helper.sh",
			expected:   filepath.Join(modulePath, "lib", "utils", "helper.sh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := module.ResolveScriptPath(tt.scriptPath)
			if string(result) != tt.expected {
				t.Errorf("ResolveScriptPath(%q) = %q, want %q", tt.scriptPath, result, tt.expected)
			}
		})
	}
}

func TestModule_ValidateScriptPath(t *testing.T) {
	t.Parallel()

	module := &Module{
		Path: "/home/user/mycommands.invowkmod",
	}

	tests := []struct {
		name       string
		scriptPath types.FilesystemPath
		expectErr  bool
	}{
		{
			name:       "valid relative path",
			scriptPath: "scripts/build.sh",
			expectErr:  false,
		},
		{
			name:       "valid path in root",
			scriptPath: "run.sh",
			expectErr:  false,
		},
		{
			name:       "empty path",
			scriptPath: "",
			expectErr:  true,
		},
		{
			name: "absolute path not allowed",
			scriptPath: func() types.FilesystemPath {
				if runtime.GOOS == "windows" {
					return `C:\Windows\System32\cmd.exe`
				}
				return "/usr/bin/bash"
			}(),
			expectErr: true,
		},
		{
			name:       "path escapes module with ..",
			scriptPath: "../other/script.sh",
			expectErr:  true,
		},
		{
			name:       "path with multiple .. escapes",
			scriptPath: "scripts/../../other.sh",
			expectErr:  true,
		},
		{
			name:       "valid path with . component",
			scriptPath: "./scripts/build.sh",
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := module.ValidateScriptPath(tt.scriptPath)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateScriptPath(%q) expected error, got nil", tt.scriptPath)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateScriptPath(%q) unexpected error: %v", tt.scriptPath, err)
			}
		})
	}
}

func TestModule_ContainsPath(t *testing.T) {
	t.Parallel()

	// Create a real temp directory for this test
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "mycommands.invowkmod")
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatal(err)
	}

	module := &Module{
		Path: types.FilesystemPath(modulePath),
	}

	tests := []struct {
		name     string
		path     types.FilesystemPath
		expected bool
	}{
		{
			name:     "file in module root",
			path:     types.FilesystemPath(filepath.Join(modulePath, "invowkfile.cue")),
			expected: true,
		},
		{
			name:     "file in subdirectory",
			path:     types.FilesystemPath(filepath.Join(modulePath, "scripts", "build.sh")),
			expected: true,
		},
		{
			name:     "module path itself",
			path:     types.FilesystemPath(modulePath),
			expected: true,
		},
		{
			name:     "parent directory",
			path:     types.FilesystemPath(dir),
			expected: false,
		},
		{
			name:     "sibling directory",
			path:     types.FilesystemPath(filepath.Join(dir, "other")),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := module.ContainsPath(tt.path)
			if result != tt.expected {
				t.Errorf("ContainsPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
