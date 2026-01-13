package pack

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
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
			} else {
				if err == nil {
					t.Errorf("ParsePackName(%q) = %q, expected error", tt.folderName, result)
				}
			}
		})
	}
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
			name: "valid pack with invkfile",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "valid RDNS pack",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "com.example.mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "missing invkfile.cue",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				return packPath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "invkfile.cue is a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				invkfileDir := filepath.Join(packPath, "invkfile.cue")
				if err := os.Mkdir(invkfileDir, 0755); err != nil {
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
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				// Create nested pack
				nestedPath := filepath.Join(packPath, "nested.invkpack")
				if err := os.Mkdir(nestedPath, 0755); err != nil {
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
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
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
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "multiple issues - missing invkfile and nested pack",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				// Create nested pack (but no invkfile)
				nestedPath := filepath.Join(packPath, "nested.invkpack")
				if err := os.Mkdir(nestedPath, 0755); err != nil {
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
				packPath := filepath.Join(dir, "mycommands.invkpack")
				if err := os.Mkdir(packPath, 0755); err != nil {
					t.Fatal(err)
				}
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if err := os.WriteFile(invkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				// Create scripts directory
				scriptsDir := filepath.Join(packPath, "scripts")
				if err := os.Mkdir(scriptsDir, 0755); err != nil {
					t.Fatal(err)
				}
				scriptPath := filepath.Join(scriptsDir, "build.sh")
				if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
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
		packPath := filepath.Join(dir, "com.example.test.invkpack")
		if err := os.Mkdir(packPath, 0755); err != nil {
			t.Fatal(err)
		}
		invkfilePath := filepath.Join(packPath, "invkfile.cue")
		if err := os.WriteFile(invkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
			t.Fatal(err)
		}

		pack, err := Load(packPath)
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if pack.Name != "com.example.test" {
			t.Errorf("pack.Name = %q, want %q", pack.Name, "com.example.test")
		}

		if pack.InvkfilePath != invkfilePath {
			t.Errorf("pack.InvkfilePath = %q, want %q", pack.InvkfilePath, invkfilePath)
		}
	})

	t.Run("fails for invalid pack", func(t *testing.T) {
		dir := t.TempDir()
		packPath := filepath.Join(dir, "mycommands.invkpack")
		if err := os.Mkdir(packPath, 0755); err != nil {
			t.Fatal(err)
		}
		// No invkfile.cue

		_, err := Load(packPath)
		if err == nil {
			t.Error("Load() expected error for invalid pack, got nil")
		}
	})
}

func TestPack_ResolveScriptPath(t *testing.T) {
	pack := &Pack{
		Path:         "/home/user/mycommands.invkpack",
		Name:         "mycommands",
		InvkfilePath: "/home/user/mycommands.invkpack/invkfile.cue",
	}

	tests := []struct {
		name       string
		scriptPath string
		expected   string
	}{
		{
			name:       "relative path with forward slashes",
			scriptPath: "scripts/build.sh",
			expected:   filepath.Join("/home/user/mycommands.invkpack", "scripts", "build.sh"),
		},
		{
			name:       "relative path in root",
			scriptPath: "run.sh",
			expected:   filepath.Join("/home/user/mycommands.invkpack", "run.sh"),
		},
		{
			name:       "nested path",
			scriptPath: "lib/utils/helper.sh",
			expected:   filepath.Join("/home/user/mycommands.invkpack", "lib", "utils", "helper.sh"),
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
		Path:         "/home/user/mycommands.invkpack",
		Name:         "mycommands",
		InvkfilePath: "/home/user/mycommands.invkpack/invkfile.cue",
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
			name:       "absolute path not allowed",
			scriptPath: "/usr/bin/bash",
			expectErr:  true,
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
	if err := os.Mkdir(packPath, 0755); err != nil {
		t.Fatal(err)
	}

	pack := &Pack{
		Path: packPath,
		Name: "mycommands",
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

				// Check invkfile.cue exists
				invkfilePath := filepath.Join(packPath, "invkfile.cue")
				if _, err := os.Stat(invkfilePath); err != nil {
					t.Errorf("invkfile.cue not created: %v", err)
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
			name: "create pack with custom group",
			opts: CreateOptions{
				Name:  "mytools",
				Group: "custom-group",
			},
			expectErr: false,
			validate: func(t *testing.T, packPath string) {
				content, err := os.ReadFile(filepath.Join(packPath, "invkfile.cue"))
				if err != nil {
					t.Fatalf("failed to read invkfile: %v", err)
				}
				if !strings.Contains(string(content), `group: "custom-group"`) {
					t.Error("custom group not set in invkfile")
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
				content, err := os.ReadFile(filepath.Join(packPath, "invkfile.cue"))
				if err != nil {
					t.Fatalf("failed to read invkfile: %v", err)
				}
				if !strings.Contains(string(content), `description: "My custom description"`) {
					t.Error("custom description not set in invkfile")
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
		if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
			t.Fatalf("failed to write script: %v", err)
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
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(tmpDir)

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
		if err := os.Mkdir(packPath, 0755); err != nil {
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
		os.RemoveAll(packPath)

		// Unpack to a different directory
		unpackDir := filepath.Join(tmpDir, "unpacked")
		if err := os.Mkdir(unpackDir, 0755); err != nil {
			t.Fatalf("failed to create unpack dir: %v", err)
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

		if b.Name != "mytools" {
			t.Errorf("extracted pack name = %q, expected %q", b.Name, "mytools")
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
		if err := os.WriteFile(markerFile, []byte("marker"), 0644); err != nil {
			t.Fatalf("failed to create marker file: %v", err)
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
		if _, err := os.Stat(filepath.Join(extractedPath, "marker.txt")); !os.IsNotExist(err) {
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
		if err := os.WriteFile(invalidZip, []byte("not a zip file"), 0644); err != nil {
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
		w.Write([]byte("content"))
		zipWriter.Close()
		zipFile.Close()

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
