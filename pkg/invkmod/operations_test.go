// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
				modulePath := filepath.Join(dir, "mycommands.invkmod")
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
				modulePath := filepath.Join(dir, "com.example.mycommands.invkmod")
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
				modulePath := filepath.Join(dir, "123commands.invkmod")
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
				// This tests ".hidden.invkmod" - the prefix is ".hidden" which is invalid
				modulePath := filepath.Join(dir, ".hidden.invkmod")
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
				filePath := filepath.Join(dir, "mycommands.invkmod")
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
				return "/nonexistent/path/mycommands.invkmod"
			},
			expected: false,
		},
		{
			name: "invalid - contains hyphen in name",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "my-commands.invkmod")
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
				modulePath := filepath.Join(dir, "my_commands.invkmod")
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
				modulePath := filepath.Join(dir, "Com.Example.MyCommands.invkmod")
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
			result := IsModule(path)
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
			folderName:  "mycommands.invkmod",
			expectedOK:  true,
			expectedVal: "mycommands",
		},
		{
			name:        "RDNS name",
			folderName:  "com.example.mycommands.invkmod",
			expectedOK:  true,
			expectedVal: "com.example.mycommands",
		},
		{
			name:        "single letter segments",
			folderName:  "a.b.c.invkmod",
			expectedOK:  true,
			expectedVal: "a.b.c",
		},
		{
			name:        "alphanumeric segments",
			folderName:  "com.example123.mytools.invkmod",
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
			folderName: ".invkmod",
			expectedOK: false,
		},
		{
			name:       "starts with number",
			folderName: "123commands.invkmod",
			expectedOK: false,
		},
		{
			name:       "segment starts with number",
			folderName: "com.123example.invkmod",
			expectedOK: false,
		},
		{
			name:       "contains hyphen",
			folderName: "my-commands.invkmod",
			expectedOK: false,
		},
		{
			name:       "contains underscore",
			folderName: "my_commands.invkmod",
			expectedOK: false,
		},
		{
			name:       "starts with dot (hidden)",
			folderName: ".hidden.invkmod",
			expectedOK: false,
		},
		{
			name:       "double dots",
			folderName: "com..example.invkmod",
			expectedOK: false,
		},
		{
			name:       "ends with dot before suffix",
			folderName: "com.example..invkmod",
			expectedOK: false,
		},
		{
			name:       "empty segment",
			folderName: "com.example..tools.invkmod",
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
				if result != tt.expectedVal {
					t.Errorf("ParseModuleName(%q) = %q, want %q", tt.folderName, result, tt.expectedVal)
				}
			} else if err == nil {
				t.Errorf("ParseModuleName(%q) = %q, expected error", tt.folderName, result)
			}
		})
	}
}

// Helper function to create a valid module with both invkmod.cue and invkfile.cue
func createValidModule(t *testing.T, dir, folderName, moduleID string) string {
	t.Helper()
	modulePath := filepath.Join(dir, folderName)
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create invkmod.cue with metadata
	invkmodPath := filepath.Join(modulePath, "invkmod.cue")
	invkmodContent := fmt.Sprintf(`module: "%s"
version: "1.0"
`, moduleID)
	if err := os.WriteFile(invkmodPath, []byte(invkmodContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create invkfile.cue with commands
	invkfilePath := filepath.Join(modulePath, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	return modulePath
}

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("loads valid module", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modulePath := createValidModule(t, dir, "com.example.test.invkmod", "com.example.test")

		module, err := Load(modulePath)
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if module.Name() != "com.example.test" {
			t.Errorf("module.Name() = %q, want %q", module.Name(), "com.example.test")
		}

		// Verify invkmod.cue path is set
		expectedInvkmodPath := filepath.Join(modulePath, "invkmod.cue")
		if module.InvkmodPath() != expectedInvkmodPath {
			t.Errorf("module.InvkmodPath() = %q, want %q", module.InvkmodPath(), expectedInvkmodPath)
		}

		// Verify invkfile.cue path is set
		expectedInvkfilePath := filepath.Join(modulePath, "invkfile.cue")
		if module.InvkfilePath() != expectedInvkfilePath {
			t.Errorf("module.InvkfilePath() = %q, want %q", module.InvkfilePath(), expectedInvkfilePath)
		}
	})

	t.Run("loads library-only module", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modulePath := filepath.Join(dir, "mylib.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		// Only create invkmod.cue (no invkfile.cue)
		invkmodPath := filepath.Join(modulePath, "invkmod.cue")
		if err := os.WriteFile(invkmodPath, []byte(`module: "mylib"
version: "1.0"
`), 0o644); err != nil {
			t.Fatal(err)
		}

		module, err := Load(modulePath)
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

	t.Run("fails for module missing invkmod.cue", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		modulePath := filepath.Join(dir, "mycommands.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		// Only create invkfile.cue (missing invkmod.cue)
		invkfilePath := filepath.Join(modulePath, "invkfile.cue")
		if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(modulePath)
		if err == nil {
			t.Error("Load() expected error for module missing invkmod.cue, got nil")
		}
	})
}

func TestModule_ResolveScriptPath(t *testing.T) {
	t.Parallel()

	modulePath := filepath.Join(string(filepath.Separator), "home", "user", "mycommands.invkmod")
	module := &Module{
		Path: modulePath,
	}

	tests := []struct {
		name       string
		scriptPath string
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
			if result != tt.expected {
				t.Errorf("ResolveScriptPath(%q) = %q, want %q", tt.scriptPath, result, tt.expected)
			}
		})
	}
}

func TestModule_ValidateScriptPath(t *testing.T) {
	t.Parallel()

	module := &Module{
		Path: "/home/user/mycommands.invkmod",
	}

	tests := []struct {
		name       string
		scriptPath string
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
			scriptPath: func() string {
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
	modulePath := filepath.Join(dir, "mycommands.invkmod")
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatal(err)
	}

	module := &Module{
		Path: modulePath,
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "file in module root",
			path:     filepath.Join(modulePath, "invkfile.cue"),
			expected: true,
		},
		{
			name:     "file in subdirectory",
			path:     filepath.Join(modulePath, "scripts", "build.sh"),
			expected: true,
		},
		{
			name:     "module path itself",
			path:     modulePath,
			expected: true,
		},
		{
			name:     "parent directory",
			path:     dir,
			expected: false,
		},
		{
			name:     "sibling directory",
			path:     filepath.Join(dir, "other"),
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
