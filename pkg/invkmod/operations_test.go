// SPDX-License-Identifier: EPL-2.0

package invkmod

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"invowk-cli/internal/testutil"
)

func TestIsModule(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string // returns path to test
		expected bool
	}{
		{
			name: "valid module with simple name",
			setup: func(t *testing.T) string {
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
				return "/nonexistent/path/mycommands.invkmod"
			},
			expected: false,
		},
		{
			name: "invalid - contains hyphen in name",
			setup: func(t *testing.T) string {
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
			path := tt.setup(t)
			result := IsModule(path)
			if result != tt.expected {
				t.Errorf("IsModule(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestParseModuleName(t *testing.T) {
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

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) string // returns path to module
		expectValid    bool
		expectIssues   int
		checkIssueType string // optional: check that at least one issue has this type
	}{
		{
			name: "valid module with invkmod.cue and invkfile.cue",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return createValidModule(t, dir, "mycommands.invkmod", "mycommands")
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "valid RDNS module",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return createValidModule(t, dir, "com.example.mycommands.invkmod", "com.example.mycommands")
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "library-only module (no invkfile.cue)",
			setup: func(t *testing.T) string {
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
				return modulePath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "missing invkmod.cue (required)",
			setup: func(t *testing.T) string {
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
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "module field mismatches folder name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Create invkmod.cue with WRONG module ID
				invkmodPath := filepath.Join(modulePath, "invkmod.cue")
				if err := os.WriteFile(invkmodPath, []byte(`module: "wrongname"
version: "1.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(modulePath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
		{
			name: "invkmod.cue is a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				invkmodDir := filepath.Join(modulePath, "invkmod.cue")
				if err := os.Mkdir(invkmodDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "nested module not allowed",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				modulePath := createValidModule(t, dir, "mycommands.invkmod", "mycommands")
				// Create nested module
				nestedPath := filepath.Join(modulePath, "nested.invkmod")
				if err := os.Mkdir(nestedPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "invalid folder name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "123invalid.invkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				invkmodPath := filepath.Join(modulePath, "invkmod.cue")
				if err := os.WriteFile(invkmodPath, []byte(`module: "test"
version: "1.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(modulePath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
		{
			name: "path does not exist",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/mycommands.invkmod"
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "path is a file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invkmod")
				if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "multiple issues - missing invkmod.cue and nested module",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Create nested module (but no invkmod.cue)
				nestedPath := filepath.Join(modulePath, "nested.invkmod")
				if err := os.Mkdir(nestedPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:  false,
			expectIssues: 2,
		},
		{
			name: "module with script files - valid structure",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				modulePath := createValidModule(t, dir, "mycommands.invkmod", "mycommands")
				// Create scripts directory
				scriptsDir := filepath.Join(modulePath, "scripts")
				if err := os.Mkdir(scriptsDir, 0o755); err != nil {
					t.Fatal(err)
				}
				scriptPath := filepath.Join(scriptsDir, "build.sh")
				if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:  true,
			expectIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			result, err := Validate(path)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", path, err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Validate(%q).Valid = %v, want %v", path, result.Valid, tt.expectValid)
			}

			if len(result.Issues) != tt.expectIssues {
				t.Errorf("Validate(%q) has %d issues, want %d. Issues: %v", path, len(result.Issues), tt.expectIssues, result.Issues)
			}

			if tt.checkIssueType != "" && len(result.Issues) > 0 {
				found := false
				for _, issue := range result.Issues {
					if issue.Type == tt.checkIssueType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Validate(%q) expected issue type %q, but not found in %v", path, tt.checkIssueType, result.Issues)
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("loads valid module", func(t *testing.T) {
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
			result := module.ResolveScriptPath(tt.scriptPath)
			if result != tt.expected {
				t.Errorf("ResolveScriptPath(%q) = %q, want %q", tt.scriptPath, result, tt.expected)
			}
		})
	}
}

func TestModule_ValidateScriptPath(t *testing.T) {
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
			result := module.ContainsPath(tt.path)
			if result != tt.expected {
				t.Errorf("ContainsPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidationIssue_Error(t *testing.T) {
	tests := []struct {
		name     string
		issue    ValidationIssue
		expected string
	}{
		{
			name: "issue with path",
			issue: ValidationIssue{
				Type:    "structure",
				Message: "nested modules are not allowed",
				Path:    "nested.invkmod",
			},
			expected: "[structure] nested.invkmod: nested modules are not allowed",
		},
		{
			name: "issue without path",
			issue: ValidationIssue{
				Type:    "naming",
				Message: "module name is invalid",
				Path:    "",
			},
			expected: "[naming] module name is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.issue.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name       string
		moduleName string
		expectErr  bool
	}{
		{
			name:       "valid simple name",
			moduleName: "mycommands",
			expectErr:  false,
		},
		{
			name:       "valid RDNS name",
			moduleName: "com.example.mycommands",
			expectErr:  false,
		},
		{
			name:       "valid single letter segments",
			moduleName: "a.b.c",
			expectErr:  false,
		},
		{
			name:       "valid with uppercase",
			moduleName: "Com.Example.MyCommands",
			expectErr:  false,
		},
		{
			name:       "valid with numbers",
			moduleName: "com.example123.tools",
			expectErr:  false,
		},
		{
			name:       "empty name",
			moduleName: "",
			expectErr:  true,
		},
		{
			name:       "starts with dot",
			moduleName: ".hidden",
			expectErr:  true,
		},
		{
			name:       "starts with number",
			moduleName: "123invalid",
			expectErr:  true,
		},
		{
			name:       "contains hyphen",
			moduleName: "my-commands",
			expectErr:  true,
		},
		{
			name:       "contains underscore",
			moduleName: "my_commands",
			expectErr:  true,
		},
		{
			name:       "double dots",
			moduleName: "com..example",
			expectErr:  true,
		},
		{
			name:       "segment starts with number",
			moduleName: "com.123example",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.moduleName)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateName(%q) expected error, got nil", tt.moduleName)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateName(%q) unexpected error: %v", tt.moduleName, err)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name      string
		opts      CreateOptions
		expectErr bool
		validate  func(t *testing.T, modulePath string)
	}{
		{
			name: "create simple module",
			opts: CreateOptions{
				Name: "mycommands",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				// Check module directory exists
				info, err := os.Stat(modulePath)
				if err != nil {
					t.Fatalf("module directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("module path is not a directory")
				}

				// Check invkmod.cue exists (required)
				invkmodPath := filepath.Join(modulePath, "invkmod.cue")
				if _, statErr := os.Stat(invkmodPath); statErr != nil {
					t.Errorf("invkmod.cue not created: %v", statErr)
				}

				// Check invkfile.cue exists
				invkfilePath := filepath.Join(modulePath, "invkfile.cue")
				if _, statErr := os.Stat(invkfilePath); statErr != nil {
					t.Errorf("invkfile.cue not created: %v", statErr)
				}

				// Verify module is valid
				_, err = Load(modulePath)
				if err != nil {
					t.Errorf("created module is not valid: %v", err)
				}
			},
		},
		{
			name: "create RDNS module",
			opts: CreateOptions{
				Name: "com.example.mytools",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				if !strings.HasSuffix(modulePath, "com.example.mytools.invkmod") {
					t.Errorf("unexpected module path: %s", modulePath)
				}
				// Verify invkmod.cue contains correct module ID
				content, err := os.ReadFile(filepath.Join(modulePath, "invkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `module: "com.example.mytools"`) {
					t.Error("module ID not set correctly in invkmod.cue")
				}
			},
		},
		{
			name: "create module with scripts directory",
			opts: CreateOptions{
				Name:             "mytools",
				CreateScriptsDir: true,
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				scriptsDir := filepath.Join(modulePath, "scripts")
				info, err := os.Stat(scriptsDir)
				if err != nil {
					t.Fatalf("scripts directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("scripts path is not a directory")
				}

				// Check .gitkeep exists
				gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
				if _, err := os.Stat(gitkeepPath); err != nil {
					t.Errorf(".gitkeep not created: %v", err)
				}
			},
		},
		{
			name: "create module with custom module identifier",
			opts: CreateOptions{
				Name:   "mytools",
				Module: "custom.module",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				// Custom module ID should be in invkmod.cue (not invkfile.cue)
				content, err := os.ReadFile(filepath.Join(modulePath, "invkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `module: "custom.module"`) {
					t.Error("custom module not set in invkmod.cue")
				}
			},
		},
		{
			name: "create module with custom description",
			opts: CreateOptions{
				Name:        "mytools",
				Description: "My custom description",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				// Description should be in invkmod.cue (not invkfile.cue)
				content, err := os.ReadFile(filepath.Join(modulePath, "invkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `description: "My custom description"`) {
					t.Error("custom description not set in invkmod.cue")
				}
			},
		},
		{
			name: "empty name fails",
			opts: CreateOptions{
				Name: "",
			},
			expectErr: true,
		},
		{
			name: "invalid name fails",
			opts: CreateOptions{
				Name: "123invalid",
			},
			expectErr: true,
		},
		{
			name: "name with hyphen fails",
			opts: CreateOptions{
				Name: "my-commands",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory as parent
			tmpDir := t.TempDir()
			opts := tt.opts
			opts.ParentDir = tmpDir

			modulePath, err := Create(opts)
			if tt.expectErr {
				if err == nil {
					t.Error("Create() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Create() unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, modulePath)
			}
		})
	}
}

func TestCreate_ExistingModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create module first time
	opts := CreateOptions{
		Name:      "mytools",
		ParentDir: tmpDir,
	}

	_, err := Create(opts)
	if err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	// Try to create again - should fail
	_, err = Create(opts)
	if err == nil {
		t.Error("Create() expected error for existing module, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestArchive(t *testing.T) {
	t.Run("archive valid module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a module first
		modulePath, err := Create(CreateOptions{
			Name:             "mytools",
			ParentDir:        tmpDir,
			CreateScriptsDir: true,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Add a script file
		scriptPath := filepath.Join(modulePath, "scripts", "test.sh")
		if writeErr := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0o755); writeErr != nil {
			t.Fatalf("failed to write script: %v", writeErr)
		}

		// Archive the module
		outputPath := filepath.Join(tmpDir, "output.zip")
		zipPath, err := Archive(modulePath, outputPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Verify ZIP was created
		info, err := os.Stat(zipPath)
		if err != nil {
			t.Fatalf("ZIP file not created: %v", err)
		}
		if info.Size() == 0 {
			t.Error("ZIP file is empty")
		}

		// Verify ZIP path matches expected
		if zipPath != outputPath {
			t.Errorf("Archive() returned %q, expected %q", zipPath, outputPath)
		}
	})

	t.Run("archive with default output path", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a module
		modulePath, err := Create(CreateOptions{
			Name:      "com.example.tools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Change to temp dir to test default output
		restoreWd := testutil.MustChdir(t, tmpDir)
		defer restoreWd()

		// Archive with empty output path
		zipPath, err := Archive(modulePath, "")
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Verify default name
		expectedName := "com.example.tools.invkmod.zip"
		if filepath.Base(zipPath) != expectedName {
			t.Errorf("default ZIP name = %q, expected %q", filepath.Base(zipPath), expectedName)
		}
	})

	t.Run("archive invalid module fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid module (no invkfile)
		modulePath := filepath.Join(tmpDir, "invalid.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		_, err := Archive(modulePath, "")
		if err == nil {
			t.Error("Archive() expected error for invalid module, got nil")
		}
	})
}

func TestUnpack(t *testing.T) {
	t.Run("unpack valid module from ZIP", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(modulePath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Remove original module
		testutil.MustRemoveAll(t, modulePath)

		// Unpack to a different directory
		unpackDir := filepath.Join(tmpDir, "unpacked")
		if mkdirErr := os.Mkdir(unpackDir, 0o755); mkdirErr != nil {
			t.Fatalf("failed to create unpack dir: %v", mkdirErr)
		}

		extractedPath, err := Unpack(UnpackOptions{
			Source:  zipPath,
			DestDir: unpackDir,
		})
		if err != nil {
			t.Fatalf("Unpack() failed: %v", err)
		}

		// Verify extracted module is valid
		b, err := Load(extractedPath)
		if err != nil {
			t.Fatalf("extracted module is invalid: %v", err)
		}

		if b.Name() != "mytools" {
			t.Errorf("extracted module name = %q, expected %q", b.Name(), "mytools")
		}
	})

	t.Run("unpack fails for existing module without overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(modulePath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Try to unpack to same directory (module already exists)
		_, err = Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   tmpDir,
			Overwrite: false,
		})
		if err == nil {
			t.Error("Unpack() expected error for existing module, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("unpack with overwrite replaces existing module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(modulePath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Modify the existing module
		markerFile := filepath.Join(modulePath, "marker.txt")
		if writeErr := os.WriteFile(markerFile, []byte("marker"), 0o644); writeErr != nil {
			t.Fatalf("failed to create marker file: %v", writeErr)
		}

		// Unpack with overwrite
		extractedPath, err := Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   tmpDir,
			Overwrite: true,
		})
		if err != nil {
			t.Fatalf("Unpack() with overwrite failed: %v", err)
		}

		// Verify marker file is gone (module was replaced)
		if _, statErr := os.Stat(filepath.Join(extractedPath, "marker.txt")); !os.IsNotExist(statErr) {
			t.Error("marker file should not exist after overwrite")
		}
	})

	t.Run("unpack fails for empty source", func(t *testing.T) {
		_, err := Unpack(UnpackOptions{
			Source: "",
		})
		if err == nil {
			t.Error("Unpack() expected error for empty source, got nil")
		}
	})

	t.Run("unpack fails for invalid ZIP", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid ZIP file
		invalidZip := filepath.Join(tmpDir, "invalid.zip")
		if err := os.WriteFile(invalidZip, []byte("not a zip file"), 0o644); err != nil {
			t.Fatalf("failed to create invalid ZIP: %v", err)
		}

		_, err := Unpack(UnpackOptions{
			Source:  invalidZip,
			DestDir: tmpDir,
		})
		if err == nil {
			t.Error("Unpack() expected error for invalid ZIP, got nil")
		}
	})

	t.Run("unpack fails for ZIP without module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a ZIP file without a module
		zipPath := filepath.Join(tmpDir, "nomodule.zip")
		zipFile, err := os.Create(zipPath)
		if err != nil {
			t.Fatalf("failed to create ZIP file: %v", err)
		}
		zipWriter := zip.NewWriter(zipFile)
		w, _ := zipWriter.Create("somefile.txt")
		_, _ = w.Write([]byte("content")) // Test setup; error non-critical
		_ = zipWriter.Close()             // Test setup; error non-critical
		_ = zipFile.Close()               // Test setup; error non-critical

		_, err = Unpack(UnpackOptions{
			Source:  zipPath,
			DestDir: tmpDir,
		})
		if err == nil {
			t.Error("Unpack() expected error for ZIP without module, got nil")
		}
		if !strings.Contains(err.Error(), "no valid module found") {
			t.Errorf("expected 'no valid module found' error, got: %v", err)
		}
	})
}

func TestVendoredModulesDir(t *testing.T) {
	if VendoredModulesDir != "invk_modules" {
		t.Errorf("VendoredModulesDir = %q, want %q", VendoredModulesDir, "invk_modules")
	}
}

func TestGetVendoredModulesDir(t *testing.T) {
	modulePath := "/path/to/mymodule.invkmod"
	expected := filepath.Join(modulePath, "invk_modules")
	result := GetVendoredModulesDir(modulePath)
	if result != expected {
		t.Errorf("GetVendoredModulesDir() = %q, want %q", result, expected)
	}
}

func TestHasVendoredModules(t *testing.T) {
	t.Run("no vendored modules directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := createValidModule(t, tmpDir, "mymodule.invkmod", "mymodule")

		if HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return false when invk_modules/ doesn't exist")
		}
	})

	t.Run("empty vendored modules directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := createValidModule(t, tmpDir, "mymodule.invkmod", "mymodule")
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return false when invk_modules/ is empty")
		}
	})

	t.Run("with vendored modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := createValidModule(t, tmpDir, "mymodule.invkmod", "mymodule")
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Create a vendored module using new format
		createValidModule(t, vendoredDir, "vendor.invkmod", "vendor")

		if !HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return true when invk_modules/ has modules")
		}
	})
}

func TestListVendoredModules(t *testing.T) {
	t.Run("no vendored modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 0 {
			t.Errorf("ListVendoredModules() returned %d modules, want 0", len(modules))
		}
	})

	t.Run("with vendored modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create two vendored modules using new format
		createValidModule(t, vendoredDir, "vendor1.invkmod", "vendor1")
		createValidModule(t, vendoredDir, "vendor2.invkmod", "vendor2")

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 2 {
			t.Errorf("ListVendoredModules() returned %d modules, want 2", len(modules))
		}

		// Check module names
		names := make(map[string]bool)
		for _, p := range modules {
			names[p.Name()] = true
		}
		if !names["vendor1"] || !names["vendor2"] {
			t.Errorf("ListVendoredModules() missing expected modules, got: %v", names)
		}
	})

	t.Run("skips invalid modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a valid module using new format
		createValidModule(t, vendoredDir, "valid.invkmod", "valid")

		// Create an invalid module (no invkmod.cue)
		invalidModule := filepath.Join(vendoredDir, "invalid.invkmod")
		if err := os.Mkdir(invalidModule, 0o755); err != nil {
			t.Fatal(err)
		}

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 1 {
			t.Errorf("ListVendoredModules() returned %d modules, want 1 (should skip invalid)", len(modules))
		}
		if len(modules) > 0 && modules[0].Name() != "valid" {
			t.Errorf("ListVendoredModules() returned wrong module: %s", modules[0].Name())
		}
	})
}

func TestValidate_AllowsNestedModulesInVendoredDir(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := createValidModule(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create invk_modules directory with a nested module
	vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
	if err := os.Mkdir(vendoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createValidModule(t, vendoredDir, "vendored.invkmod", "vendored")

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Validate() should return valid for module with nested modules in invk_modules/. Issues: %v", result.Issues)
	}
}

func TestValidate_StillRejectsNestedModulesOutsideVendoredDir(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := createValidModule(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create a nested module NOT in invk_modules
	nestedModule := filepath.Join(modulePath, "nested.invkmod")
	if err := os.Mkdir(nestedModule, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if result.Valid {
		t.Error("Validate() should reject nested modules outside of invk_modules/")
	}

	// Check that the issue mentions nested module
	foundNestedIssue := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "nested") {
			foundNestedIssue = true
			break
		}
	}
	if !foundNestedIssue {
		t.Error("Validate() should report issue about nested module")
	}
}

func TestValidate_DetectsSymlinks(t *testing.T) {
	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	modulePath := createValidModule(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create a file outside the module
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the module pointing outside
	symlinkPath := filepath.Join(modulePath, "link_to_outside")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should report a security issue about the symlink
	foundSymlinkIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "security" && strings.Contains(strings.ToLower(issue.Message), "symlink") {
			foundSymlinkIssue = true
			break
		}
	}
	if !foundSymlinkIssue {
		t.Error("Validate() should report security issue about symlink pointing outside module")
	}
}

func TestValidate_DetectsWindowsReservedFilenames(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := createValidModule(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create a file with a Windows reserved name
	reservedFile := filepath.Join(modulePath, "CON")
	if err := os.WriteFile(reservedFile, []byte("test"), 0o644); err != nil {
		// On Windows, this might fail - that's expected
		if runtime.GOOS == "windows" {
			t.Skip("Cannot create reserved filename on Windows")
		}
		t.Fatal(err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should report a compatibility issue about the reserved filename
	foundReservedIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "compatibility" && strings.Contains(issue.Message, "reserved on Windows") {
			foundReservedIssue = true
			break
		}
	}
	if !foundReservedIssue {
		t.Error("Validate() should report compatibility issue about Windows reserved filename")
	}
}

func TestValidate_RejectsAllSymlinks(t *testing.T) {
	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	modulePath := createValidModule(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create scripts directory
	scriptsDir := filepath.Join(modulePath, "scripts")
	if err := os.Mkdir(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file inside the module
	internalFile := filepath.Join(scriptsDir, "original.sh")
	if err := os.WriteFile(internalFile, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the module pointing to another file inside the module
	symlinkPath := filepath.Join(modulePath, "link_to_internal")
	if err := os.Symlink(internalFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// ALL symlinks should be rejected as a security measure (even internal ones)
	// This is intentional to prevent zip slip attacks during archive extraction
	foundSecurityIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "security" && strings.Contains(strings.ToLower(issue.Message), "symlink") {
			foundSecurityIssue = true
			break
		}
	}
	if !foundSecurityIssue {
		t.Error("Validate() should report security issue for ALL symlinks (including internal ones)")
	}
}
