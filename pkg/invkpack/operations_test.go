// SPDX-License-Identifier: EPL-2.0

package invkpack

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

func TestIsPack(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string // returns path to test
		expected bool
	}{
		{
			name: "valid pack with simple name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: true,
		},
		{
			name: "valid pack with RDNS name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "com.example.mycommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: true,
		},
		{
			name: "invalid - missing suffix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: false,
		},
		{
			name: "invalid - wrong suffix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.wrong")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: false,
		},
		{
			name: "invalid - starts with number",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "123commands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: false,
		},
		{
			name: "invalid - hidden folder prefix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Note: folder name itself doesn't start with dot, but the name part does
				// This tests ".hidden.invkpack" - the prefix is ".hidden" which is invalid
				packPath := filepath.Join(dir, ".hidden.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: false,
		},
		{
			name: "invalid - file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invkpack")
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
				return "/nonexistent/path/mycommands.invkpack"
			},
			expected: false,
		},
		{
			name: "invalid - contains hyphen in name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "my-commands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: false,
		},
		{
			name: "invalid - contains underscore in name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "my_commands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: false,
		},
		{
			name: "valid - segment starts with uppercase",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "Com.Example.MyCommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			result := IsPack(path)
			if result != tt.expected {
				t.Errorf("IsPack(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestParsePackName(t *testing.T) {
	tests := []struct {
		name        string
		folderName  string
		expectedOK  bool
		expectedVal string
	}{
		{
			name:        "simple name",
			folderName:  "mycommands.invkpack",
			expectedOK:  true,
			expectedVal: "mycommands",
		},
		{
			name:        "RDNS name",
			folderName:  "com.example.mycommands.invkpack",
			expectedOK:  true,
			expectedVal: "com.example.mycommands",
		},
		{
			name:        "single letter segments",
			folderName:  "a.b.c.invkpack",
			expectedOK:  true,
			expectedVal: "a.b.c",
		},
		{
			name:        "alphanumeric segments",
			folderName:  "com.example123.mytools.invkpack",
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
			folderName: ".invkpack",
			expectedOK: false,
		},
		{
			name:       "starts with number",
			folderName: "123commands.invkpack",
			expectedOK: false,
		},
		{
			name:       "segment starts with number",
			folderName: "com.123example.invkpack",
			expectedOK: false,
		},
		{
			name:       "contains hyphen",
			folderName: "my-commands.invkpack",
			expectedOK: false,
		},
		{
			name:       "contains underscore",
			folderName: "my_commands.invkpack",
			expectedOK: false,
		},
		{
			name:       "starts with dot (hidden)",
			folderName: ".hidden.invkpack",
			expectedOK: false,
		},
		{
			name:       "double dots",
			folderName: "com..example.invkpack",
			expectedOK: false,
		},
		{
			name:       "ends with dot before suffix",
			folderName: "com.example..invkpack",
			expectedOK: false,
		},
		{
			name:       "empty segment",
			folderName: "com.example..tools.invkpack",
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePackName(tt.folderName)
			if tt.expectedOK {
				if err != nil {
					t.Errorf("ParsePackName(%q) returned error: %v, expected %q", tt.folderName, err, tt.expectedVal)
				}
				if result != tt.expectedVal {
					t.Errorf("ParsePackName(%q) = %q, want %q", tt.folderName, result, tt.expectedVal)
				}
			} else if err == nil {
				t.Errorf("ParsePackName(%q) = %q, expected error", tt.folderName, result)
			}
		})
	}
}

// Helper function to create a valid pack with both invkpack.cue and invkfile.cue
func createValidPack(t *testing.T, dir, folderName, packID string) string {
	t.Helper()
	packPath := filepath.Join(dir, folderName)
	if err := os.Mkdir(packPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create invkpack.cue with metadata
	invkpackPath := filepath.Join(packPath, "invkpack.cue")
	invkpackContent := fmt.Sprintf(`pack: "%s"
version: "1.0"
`, packID)
	if err := os.WriteFile(invkpackPath, []byte(invkpackContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create invkfile.cue with commands
	invkfilePath := filepath.Join(packPath, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	return packPath
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) string // returns path to pack
		expectValid    bool
		expectIssues   int
		checkIssueType string // optional: check that at least one issue has this type
	}{
		{
			name: "valid pack with invkpack.cue and invkfile.cue",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return createValidPack(t, dir, "mycommands.invkpack", "mycommands")
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "valid RDNS pack",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return createValidPack(t, dir, "com.example.mycommands.invkpack", "com.example.mycommands")
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "library-only pack (no invkfile.cue)",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mylib.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Only create invkpack.cue (no invkfile.cue)
				invkpackPath := filepath.Join(packPath, "invkpack.cue")
				if err := os.WriteFile(invkpackPath, []byte(`pack: "mylib"
version: "1.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "missing invkpack.cue (required)",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Only create invkfile.cue (missing invkpack.cue)
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "pack field mismatches folder name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Create invkpack.cue with WRONG pack ID
				invkpackPath := filepath.Join(packPath, "invkpack.cue")
				if err := os.WriteFile(invkpackPath, []byte(`pack: "wrongname"
version: "1.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
		{
			name: "invkpack.cue is a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				invkpackDir := filepath.Join(packPath, "invkpack.cue")
				if err := os.Mkdir(invkpackDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "nested pack not allowed",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := createValidPack(t, dir, "mycommands.invkpack", "mycommands")
				// Create nested pack
				nestedPath := filepath.Join(packPath, "nested.invkpack")
				if err := os.Mkdir(nestedPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "invalid folder name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "123invalid.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				invkpackPath := filepath.Join(packPath, "invkpack.cue")
				if err := os.WriteFile(invkpackPath, []byte(`pack: "test"
version: "1.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
		{
			name: "path does not exist",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/mycommands.invkpack"
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "path is a file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invkpack")
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
			name: "multiple issues - missing invkpack.cue and nested pack",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Create nested pack (but no invkpack.cue)
				nestedPath := filepath.Join(packPath, "nested.invkpack")
				if err := os.Mkdir(nestedPath, 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:  false,
			expectIssues: 2,
		},
		{
			name: "pack with script files - valid structure",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := createValidPack(t, dir, "mycommands.invkpack", "mycommands")
				// Create scripts directory
				scriptsDir := filepath.Join(packPath, "scripts")
				if err := os.Mkdir(scriptsDir, 0o755); err != nil {
					t.Fatal(err)
				}
				scriptPath := filepath.Join(scriptsDir, "build.sh")
				if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
					t.Fatal(err)
				}
				return packPath
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
	t.Run("loads valid pack", func(t *testing.T) {
		dir := t.TempDir()
		packPath := createValidPack(t, dir, "com.example.test.invkpack", "com.example.test")

		pack, err := Load(packPath)
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if pack.Name() != "com.example.test" {
			t.Errorf("pack.Name() = %q, want %q", pack.Name(), "com.example.test")
		}

		// Verify invkpack.cue path is set
		expectedInvkpackPath := filepath.Join(packPath, "invkpack.cue")
		if pack.InvkpackPath() != expectedInvkpackPath {
			t.Errorf("pack.InvkpackPath() = %q, want %q", pack.InvkpackPath(), expectedInvkpackPath)
		}

		// Verify invkfile.cue path is set
		expectedInvkfilePath := filepath.Join(packPath, "invkfile.cue")
		if pack.InvkfilePath() != expectedInvkfilePath {
			t.Errorf("pack.InvkfilePath() = %q, want %q", pack.InvkfilePath(), expectedInvkfilePath)
		}
	})

	t.Run("loads library-only pack", func(t *testing.T) {
		dir := t.TempDir()
		packPath := filepath.Join(dir, "mylib.invkpack")
		if err := os.Mkdir(packPath, 0o755); err != nil {
			t.Fatal(err)
		}
		// Only create invkpack.cue (no invkfile.cue)
		invkpackPath := filepath.Join(packPath, "invkpack.cue")
		if err := os.WriteFile(invkpackPath, []byte(`pack: "mylib"
version: "1.0"
`), 0o644); err != nil {
			t.Fatal(err)
		}

		pack, err := Load(packPath)
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if pack.Name() != "mylib" {
			t.Errorf("pack.Name() = %q, want %q", pack.Name(), "mylib")
		}

		if !pack.IsLibraryOnly {
			t.Error("pack.IsLibraryOnly should be true for library-only pack")
		}
	})

	t.Run("fails for pack missing invkpack.cue", func(t *testing.T) {
		dir := t.TempDir()
		packPath := filepath.Join(dir, "mycommands.invkpack")
		if err := os.Mkdir(packPath, 0o755); err != nil {
			t.Fatal(err)
		}
		// Only create invkfile.cue (missing invkpack.cue)
		invkfilePath := filepath.Join(packPath, "invkfile.cue")
		if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(packPath)
		if err == nil {
			t.Error("Load() expected error for pack missing invkpack.cue, got nil")
		}
	})
}

func TestPack_ResolveScriptPath(t *testing.T) {
	packPath := filepath.Join(string(filepath.Separator), "home", "user", "mycommands.invkpack")
	pack := &Pack{
		Path: packPath,
	}

	tests := []struct {
		name       string
		scriptPath string
		expected   string
	}{
		{
			name:       "relative path with forward slashes",
			scriptPath: "scripts/build.sh",
			expected:   filepath.Join(packPath, "scripts", "build.sh"),
		},
		{
			name:       "relative path in root",
			scriptPath: "run.sh",
			expected:   filepath.Join(packPath, "run.sh"),
		},
		{
			name:       "nested path",
			scriptPath: "lib/utils/helper.sh",
			expected:   filepath.Join(packPath, "lib", "utils", "helper.sh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pack.ResolveScriptPath(tt.scriptPath)
			if result != tt.expected {
				t.Errorf("ResolveScriptPath(%q) = %q, want %q", tt.scriptPath, result, tt.expected)
			}
		})
	}
}

func TestPack_ValidateScriptPath(t *testing.T) {
	pack := &Pack{
		Path: "/home/user/mycommands.invkpack",
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
			name:       "path escapes pack with ..",
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
			err := pack.ValidateScriptPath(tt.scriptPath)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateScriptPath(%q) expected error, got nil", tt.scriptPath)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateScriptPath(%q) unexpected error: %v", tt.scriptPath, err)
			}
		})
	}
}

func TestPack_ContainsPath(t *testing.T) {
	// Create a real temp directory for this test
	dir := t.TempDir()
	packPath := filepath.Join(dir, "mycommands.invkpack")
	if err := os.Mkdir(packPath, 0o755); err != nil {
		t.Fatal(err)
	}

	pack := &Pack{
		Path: packPath,
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "file in pack root",
			path:     filepath.Join(packPath, "invkfile.cue"),
			expected: true,
		},
		{
			name:     "file in subdirectory",
			path:     filepath.Join(packPath, "scripts", "build.sh"),
			expected: true,
		},
		{
			name:     "pack path itself",
			path:     packPath,
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
			result := pack.ContainsPath(tt.path)
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
				Message: "nested packs are not allowed",
				Path:    "nested.invkpack",
			},
			expected: "[structure] nested.invkpack: nested packs are not allowed",
		},
		{
			name: "issue without path",
			issue: ValidationIssue{
				Type:    "naming",
				Message: "pack name is invalid",
				Path:    "",
			},
			expected: "[naming] pack name is invalid",
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
		name      string
		packName  string
		expectErr bool
	}{
		{
			name:      "valid simple name",
			packName:  "mycommands",
			expectErr: false,
		},
		{
			name:      "valid RDNS name",
			packName:  "com.example.mycommands",
			expectErr: false,
		},
		{
			name:      "valid single letter segments",
			packName:  "a.b.c",
			expectErr: false,
		},
		{
			name:      "valid with uppercase",
			packName:  "Com.Example.MyCommands",
			expectErr: false,
		},
		{
			name:      "valid with numbers",
			packName:  "com.example123.tools",
			expectErr: false,
		},
		{
			name:      "empty name",
			packName:  "",
			expectErr: true,
		},
		{
			name:      "starts with dot",
			packName:  ".hidden",
			expectErr: true,
		},
		{
			name:      "starts with number",
			packName:  "123invalid",
			expectErr: true,
		},
		{
			name:      "contains hyphen",
			packName:  "my-commands",
			expectErr: true,
		},
		{
			name:      "contains underscore",
			packName:  "my_commands",
			expectErr: true,
		},
		{
			name:      "double dots",
			packName:  "com..example",
			expectErr: true,
		},
		{
			name:      "segment starts with number",
			packName:  "com.123example",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.packName)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateName(%q) expected error, got nil", tt.packName)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateName(%q) unexpected error: %v", tt.packName, err)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name      string
		opts      CreateOptions
		expectErr bool
		validate  func(t *testing.T, packPath string)
	}{
		{
			name: "create simple pack",
			opts: CreateOptions{
				Name: "mycommands",
			},
			expectErr: false,
			validate: func(t *testing.T, packPath string) {
				// Check pack directory exists
				info, err := os.Stat(packPath)
				if err != nil {
					t.Fatalf("pack directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("pack path is not a directory")
				}

				// Check invkpack.cue exists (required)
				invkpackPath := filepath.Join(packPath, "invkpack.cue")
				if _, statErr := os.Stat(invkpackPath); statErr != nil {
					t.Errorf("invkpack.cue not created: %v", statErr)
				}

				// Check invkfile.cue exists
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if _, statErr := os.Stat(invkfilePath); statErr != nil {
					t.Errorf("invkfile.cue not created: %v", statErr)
				}

				// Verify pack is valid
				_, err = Load(packPath)
				if err != nil {
					t.Errorf("created pack is not valid: %v", err)
				}
			},
		},
		{
			name: "create RDNS pack",
			opts: CreateOptions{
				Name: "com.example.mytools",
			},
			expectErr: false,
			validate: func(t *testing.T, packPath string) {
				if !strings.HasSuffix(packPath, "com.example.mytools.invkpack") {
					t.Errorf("unexpected pack path: %s", packPath)
				}
				// Verify invkpack.cue contains correct pack ID
				content, err := os.ReadFile(filepath.Join(packPath, "invkpack.cue"))
				if err != nil {
					t.Fatalf("failed to read invkpack.cue: %v", err)
				}
				if !strings.Contains(string(content), `pack: "com.example.mytools"`) {
					t.Error("pack ID not set correctly in invkpack.cue")
				}
			},
		},
		{
			name: "create pack with scripts directory",
			opts: CreateOptions{
				Name:             "mytools",
				CreateScriptsDir: true,
			},
			expectErr: false,
			validate: func(t *testing.T, packPath string) {
				scriptsDir := filepath.Join(packPath, "scripts")
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
			name: "create pack with custom pack identifier",
			opts: CreateOptions{
				Name: "mytools",
				Pack: "custom.pack",
			},
			expectErr: false,
			validate: func(t *testing.T, packPath string) {
				// Custom pack ID should be in invkpack.cue (not invkfile.cue)
				content, err := os.ReadFile(filepath.Join(packPath, "invkpack.cue"))
				if err != nil {
					t.Fatalf("failed to read invkpack.cue: %v", err)
				}
				if !strings.Contains(string(content), `pack: "custom.pack"`) {
					t.Error("custom pack not set in invkpack.cue")
				}
			},
		},
		{
			name: "create pack with custom description",
			opts: CreateOptions{
				Name:        "mytools",
				Description: "My custom description",
			},
			expectErr: false,
			validate: func(t *testing.T, packPath string) {
				// Description should be in invkpack.cue (not invkfile.cue)
				content, err := os.ReadFile(filepath.Join(packPath, "invkpack.cue"))
				if err != nil {
					t.Fatalf("failed to read invkpack.cue: %v", err)
				}
				if !strings.Contains(string(content), `description: "My custom description"`) {
					t.Error("custom description not set in invkpack.cue")
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

			packPath, err := Create(opts)
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
				tt.validate(t, packPath)
			}
		})
	}
}

func TestCreate_ExistingPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pack first time
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
		t.Error("Create() expected error for existing pack, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestPack(t *testing.T) {
	t.Run("pack valid pack", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a pack first
		packPath, err := Create(CreateOptions{
			Name:             "mytools",
			ParentDir:        tmpDir,
			CreateScriptsDir: true,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Add a script file
		scriptPath := filepath.Join(packPath, "scripts", "test.sh")
		if writeErr := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0o755); writeErr != nil {
			t.Fatalf("failed to write script: %v", writeErr)
		}

		// Archive the pack
		outputPath := filepath.Join(tmpDir, "output.zip")
		zipPath, err := Archive(packPath, outputPath)
		if err != nil {
			t.Fatalf("Pack() failed: %v", err)
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
			t.Errorf("Pack() returned %q, expected %q", zipPath, outputPath)
		}
	})

	t.Run("pack with default output path", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a pack
		packPath, err := Create(CreateOptions{
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
		zipPath, err := Archive(packPath, "")
		if err != nil {
			t.Fatalf("Pack() failed: %v", err)
		}

		// Verify default name
		expectedName := "com.example.tools.invkpack.zip"
		if filepath.Base(zipPath) != expectedName {
			t.Errorf("default ZIP name = %q, expected %q", filepath.Base(zipPath), expectedName)
		}
	})

	t.Run("pack invalid pack fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid pack (no invkfile)
		packPath := filepath.Join(tmpDir, "invalid.invkpack")
		if err := os.Mkdir(packPath, 0o755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		_, err := Archive(packPath, "")
		if err == nil {
			t.Error("Archive() expected error for invalid pack, got nil")
		}
	})
}

func TestUnpack(t *testing.T) {
	t.Run("unpack valid pack from ZIP", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and pack a pack
		packPath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "pack.zip")
		_, err = Archive(packPath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Remove original pack
		testutil.MustRemoveAll(t, packPath)

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

		// Verify extracted pack is valid
		b, err := Load(extractedPath)
		if err != nil {
			t.Fatalf("extracted pack is invalid: %v", err)
		}

		if b.Name() != "mytools" {
			t.Errorf("extracted pack name = %q, expected %q", b.Name(), "mytools")
		}
	})

	t.Run("unpack fails for existing pack without overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and pack a pack
		packPath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "pack.zip")
		_, err = Archive(packPath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Try to unpack to same directory (pack already exists)
		_, err = Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   tmpDir,
			Overwrite: false,
		})
		if err == nil {
			t.Error("Unpack() expected error for existing pack, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("unpack with overwrite replaces existing pack", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and pack a pack
		packPath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "pack.zip")
		_, err = Archive(packPath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Modify the existing pack
		markerFile := filepath.Join(packPath, "marker.txt")
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

		// Verify marker file is gone (pack was replaced)
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

	t.Run("unpack fails for ZIP without pack", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a ZIP file without a pack
		zipPath := filepath.Join(tmpDir, "nopack.zip")
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
			t.Error("Unpack() expected error for ZIP without pack, got nil")
		}
		if !strings.Contains(err.Error(), "no valid pack found") {
			t.Errorf("expected 'no valid pack found' error, got: %v", err)
		}
	})
}

func TestVendoredPacksDir(t *testing.T) {
	if VendoredPacksDir != "invk_packs" {
		t.Errorf("VendoredPacksDir = %q, want %q", VendoredPacksDir, "invk_packs")
	}
}

func TestGetVendoredPacksDir(t *testing.T) {
	packPath := "/path/to/mypack.invkpack"
	expected := filepath.Join(packPath, "invk_packs")
	result := GetVendoredPacksDir(packPath)
	if result != expected {
		t.Errorf("GetVendoredPacksDir() = %q, want %q", result, expected)
	}
}

func TestHasVendoredPacks(t *testing.T) {
	t.Run("no vendored packs directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		packPath := createValidPack(t, tmpDir, "mypack.invkpack", "mypack")

		if HasVendoredPacks(packPath) {
			t.Error("HasVendoredPacks() should return false when invk_packs/ doesn't exist")
		}
	})

	t.Run("empty vendored packs directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		packPath := createValidPack(t, tmpDir, "mypack.invkpack", "mypack")
		vendoredDir := filepath.Join(packPath, VendoredPacksDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if HasVendoredPacks(packPath) {
			t.Error("HasVendoredPacks() should return false when invk_packs/ is empty")
		}
	})

	t.Run("with vendored packs", func(t *testing.T) {
		tmpDir := t.TempDir()
		packPath := createValidPack(t, tmpDir, "mypack.invkpack", "mypack")
		vendoredDir := filepath.Join(packPath, VendoredPacksDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Create a vendored pack using new format
		createValidPack(t, vendoredDir, "vendor.invkpack", "vendor")

		if !HasVendoredPacks(packPath) {
			t.Error("HasVendoredPacks() should return true when invk_packs/ has packs")
		}
	})
}

func TestListVendoredPacks(t *testing.T) {
	t.Run("no vendored packs", func(t *testing.T) {
		tmpDir := t.TempDir()
		packPath := filepath.Join(tmpDir, "mypack.invkpack")
		if err := os.Mkdir(packPath, 0o755); err != nil {
			t.Fatal(err)
		}

		packs, err := ListVendoredPacks(packPath)
		if err != nil {
			t.Fatalf("ListVendoredPacks() error: %v", err)
		}
		if len(packs) != 0 {
			t.Errorf("ListVendoredPacks() returned %d packs, want 0", len(packs))
		}
	})

	t.Run("with vendored packs", func(t *testing.T) {
		tmpDir := t.TempDir()
		packPath := filepath.Join(tmpDir, "mypack.invkpack")
		if err := os.Mkdir(packPath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(packPath, VendoredPacksDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create two vendored packs using new format
		createValidPack(t, vendoredDir, "vendor1.invkpack", "vendor1")
		createValidPack(t, vendoredDir, "vendor2.invkpack", "vendor2")

		packs, err := ListVendoredPacks(packPath)
		if err != nil {
			t.Fatalf("ListVendoredPacks() error: %v", err)
		}
		if len(packs) != 2 {
			t.Errorf("ListVendoredPacks() returned %d packs, want 2", len(packs))
		}

		// Check pack names
		names := make(map[string]bool)
		for _, p := range packs {
			names[p.Name()] = true
		}
		if !names["vendor1"] || !names["vendor2"] {
			t.Errorf("ListVendoredPacks() missing expected packs, got: %v", names)
		}
	})

	t.Run("skips invalid packs", func(t *testing.T) {
		tmpDir := t.TempDir()
		packPath := filepath.Join(tmpDir, "mypack.invkpack")
		if err := os.Mkdir(packPath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(packPath, VendoredPacksDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a valid pack using new format
		createValidPack(t, vendoredDir, "valid.invkpack", "valid")

		// Create an invalid pack (no invkpack.cue)
		invalidPack := filepath.Join(vendoredDir, "invalid.invkpack")
		if err := os.Mkdir(invalidPack, 0o755); err != nil {
			t.Fatal(err)
		}

		packs, err := ListVendoredPacks(packPath)
		if err != nil {
			t.Fatalf("ListVendoredPacks() error: %v", err)
		}
		if len(packs) != 1 {
			t.Errorf("ListVendoredPacks() returned %d packs, want 1 (should skip invalid)", len(packs))
		}
		if len(packs) > 0 && packs[0].Name() != "valid" {
			t.Errorf("ListVendoredPacks() returned wrong pack: %s", packs[0].Name())
		}
	})
}

func TestValidate_AllowsNestedPacksInVendoredDir(t *testing.T) {
	tmpDir := t.TempDir()
	packPath := createValidPack(t, tmpDir, "mycommands.invkpack", "mycommands")

	// Create invk_packs directory with a nested pack
	vendoredDir := filepath.Join(packPath, VendoredPacksDir)
	if err := os.Mkdir(vendoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createValidPack(t, vendoredDir, "vendored.invkpack", "vendored")

	result, err := Validate(packPath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Validate() should return valid for pack with nested packs in invk_packs/. Issues: %v", result.Issues)
	}
}

func TestValidate_StillRejectsNestedPacksOutsideVendoredDir(t *testing.T) {
	tmpDir := t.TempDir()
	packPath := createValidPack(t, tmpDir, "mycommands.invkpack", "mycommands")

	// Create a nested pack NOT in invk_packs
	nestedPack := filepath.Join(packPath, "nested.invkpack")
	if err := os.Mkdir(nestedPack, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Validate(packPath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if result.Valid {
		t.Error("Validate() should reject nested packs outside of invk_packs/")
	}

	// Check that the issue mentions nested pack
	foundNestedIssue := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "nested") {
			foundNestedIssue = true
			break
		}
	}
	if !foundNestedIssue {
		t.Error("Validate() should report issue about nested pack")
	}
}

func TestValidate_DetectsSymlinks(t *testing.T) {
	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	packPath := createValidPack(t, tmpDir, "mycommands.invkpack", "mycommands")

	// Create a file outside the pack
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the pack pointing outside
	symlinkPath := filepath.Join(packPath, "link_to_outside")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := Validate(packPath)
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
		t.Error("Validate() should report security issue about symlink pointing outside pack")
	}
}

func TestValidate_DetectsWindowsReservedFilenames(t *testing.T) {
	tmpDir := t.TempDir()
	packPath := createValidPack(t, tmpDir, "mycommands.invkpack", "mycommands")

	// Create a file with a Windows reserved name
	reservedFile := filepath.Join(packPath, "CON")
	if err := os.WriteFile(reservedFile, []byte("test"), 0o644); err != nil {
		// On Windows, this might fail - that's expected
		if runtime.GOOS == "windows" {
			t.Skip("Cannot create reserved filename on Windows")
		}
		t.Fatal(err)
	}

	result, err := Validate(packPath)
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
	packPath := createValidPack(t, tmpDir, "mycommands.invkpack", "mycommands")

	// Create scripts directory
	scriptsDir := filepath.Join(packPath, "scripts")
	if err := os.Mkdir(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file inside the pack
	internalFile := filepath.Join(scriptsDir, "original.sh")
	if err := os.WriteFile(internalFile, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the pack pointing to another file inside the pack
	symlinkPath := filepath.Join(packPath, "link_to_internal")
	if err := os.Symlink(internalFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := Validate(packPath)
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
