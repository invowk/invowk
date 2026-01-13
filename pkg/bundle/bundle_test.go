package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBundle(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string // returns path to test
		expected bool
	}{
		{
			name: "valid bundle with simple name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: true,
		},
		{
			name: "valid bundle with RDNS name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "com.example.mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: true,
		},
		{
			name: "invalid - missing suffix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: false,
		},
		{
			name: "invalid - wrong suffix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.bundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: false,
		},
		{
			name: "invalid - starts with number",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "123commands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: false,
		},
		{
			name: "invalid - hidden folder prefix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Note: folder name itself doesn't start with dot, but the name part does
				// This tests ".hidden.invowkbundle" - the prefix is ".hidden" which is invalid
				bundlePath := filepath.Join(dir, ".hidden.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: false,
		},
		{
			name: "invalid - file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invowkbundle")
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
				return "/nonexistent/path/mycommands.invowkbundle"
			},
			expected: false,
		},
		{
			name: "invalid - contains hyphen in name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "my-commands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: false,
		},
		{
			name: "invalid - contains underscore in name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "my_commands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: false,
		},
		{
			name: "valid - segment starts with uppercase",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "Com.Example.MyCommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			result := IsBundle(path)
			if result != tt.expected {
				t.Errorf("IsBundle(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestParseBundleName(t *testing.T) {
	tests := []struct {
		name        string
		folderName  string
		expectedOK  bool
		expectedVal string
	}{
		{
			name:        "simple name",
			folderName:  "mycommands.invowkbundle",
			expectedOK:  true,
			expectedVal: "mycommands",
		},
		{
			name:        "RDNS name",
			folderName:  "com.example.mycommands.invowkbundle",
			expectedOK:  true,
			expectedVal: "com.example.mycommands",
		},
		{
			name:        "single letter segments",
			folderName:  "a.b.c.invowkbundle",
			expectedOK:  true,
			expectedVal: "a.b.c",
		},
		{
			name:        "alphanumeric segments",
			folderName:  "com.example123.mytools.invowkbundle",
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
			folderName: "mycommands.bundle",
			expectedOK: false,
		},
		{
			name:       "empty prefix",
			folderName: ".invowkbundle",
			expectedOK: false,
		},
		{
			name:       "starts with number",
			folderName: "123commands.invowkbundle",
			expectedOK: false,
		},
		{
			name:       "segment starts with number",
			folderName: "com.123example.invowkbundle",
			expectedOK: false,
		},
		{
			name:       "contains hyphen",
			folderName: "my-commands.invowkbundle",
			expectedOK: false,
		},
		{
			name:       "contains underscore",
			folderName: "my_commands.invowkbundle",
			expectedOK: false,
		},
		{
			name:       "starts with dot (hidden)",
			folderName: ".hidden.invowkbundle",
			expectedOK: false,
		},
		{
			name:       "double dots",
			folderName: "com..example.invowkbundle",
			expectedOK: false,
		},
		{
			name:       "ends with dot before suffix",
			folderName: "com.example..invowkbundle",
			expectedOK: false,
		},
		{
			name:       "empty segment",
			folderName: "com.example..tools.invowkbundle",
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseBundleName(tt.folderName)
			if tt.expectedOK {
				if err != nil {
					t.Errorf("ParseBundleName(%q) returned error: %v, expected %q", tt.folderName, err, tt.expectedVal)
				}
				if result != tt.expectedVal {
					t.Errorf("ParseBundleName(%q) = %q, want %q", tt.folderName, result, tt.expectedVal)
				}
			} else {
				if err == nil {
					t.Errorf("ParseBundleName(%q) = %q, expected error", tt.folderName, result)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) string // returns path to bundle
		expectValid    bool
		expectIssues   int
		checkIssueType string // optional: check that at least one issue has this type
	}{
		{
			name: "valid bundle with invowkfile",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "valid RDNS bundle",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "com.example.mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "missing invowkfile.cue",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "invowkfile.cue is a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				invowkfileDir := filepath.Join(bundlePath, "invowkfile.cue")
				if err := os.Mkdir(invowkfileDir, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "nested bundle not allowed",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				// Create nested bundle
				nestedPath := filepath.Join(bundlePath, "nested.invowkbundle")
				if err := os.Mkdir(nestedPath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "invalid folder name",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "123invalid.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
		{
			name: "path does not exist",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/mycommands.invowkbundle"
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "path is a file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invowkbundle")
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
			name: "multiple issues - missing invowkfile and nested bundle",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				// Create nested bundle (but no invowkfile)
				nestedPath := filepath.Join(bundlePath, "nested.invowkbundle")
				if err := os.Mkdir(nestedPath, 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
			},
			expectValid:  false,
			expectIssues: 2,
		},
		{
			name: "bundle with script files - valid structure",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
				if err := os.Mkdir(bundlePath, 0755); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
					t.Fatal(err)
				}
				// Create scripts directory
				scriptsDir := filepath.Join(bundlePath, "scripts")
				if err := os.Mkdir(scriptsDir, 0755); err != nil {
					t.Fatal(err)
				}
				scriptPath := filepath.Join(scriptsDir, "build.sh")
				if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
					t.Fatal(err)
				}
				return bundlePath
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
	t.Run("loads valid bundle", func(t *testing.T) {
		dir := t.TempDir()
		bundlePath := filepath.Join(dir, "com.example.test.invowkbundle")
		if err := os.Mkdir(bundlePath, 0755); err != nil {
			t.Fatal(err)
		}
		invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
		if err := os.WriteFile(invowkfilePath, []byte("group: \"test\"\ncommands: []"), 0644); err != nil {
			t.Fatal(err)
		}

		bundle, err := Load(bundlePath)
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if bundle.Name != "com.example.test" {
			t.Errorf("bundle.Name = %q, want %q", bundle.Name, "com.example.test")
		}

		if bundle.InvowkfilePath != invowkfilePath {
			t.Errorf("bundle.InvowkfilePath = %q, want %q", bundle.InvowkfilePath, invowkfilePath)
		}
	})

	t.Run("fails for invalid bundle", func(t *testing.T) {
		dir := t.TempDir()
		bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
		if err := os.Mkdir(bundlePath, 0755); err != nil {
			t.Fatal(err)
		}
		// No invowkfile.cue

		_, err := Load(bundlePath)
		if err == nil {
			t.Error("Load() expected error for invalid bundle, got nil")
		}
	})
}

func TestBundle_ResolveScriptPath(t *testing.T) {
	bundle := &Bundle{
		Path:           "/home/user/mycommands.invowkbundle",
		Name:           "mycommands",
		InvowkfilePath: "/home/user/mycommands.invowkbundle/invowkfile.cue",
	}

	tests := []struct {
		name       string
		scriptPath string
		expected   string
	}{
		{
			name:       "relative path with forward slashes",
			scriptPath: "scripts/build.sh",
			expected:   filepath.Join("/home/user/mycommands.invowkbundle", "scripts", "build.sh"),
		},
		{
			name:       "relative path in root",
			scriptPath: "run.sh",
			expected:   filepath.Join("/home/user/mycommands.invowkbundle", "run.sh"),
		},
		{
			name:       "nested path",
			scriptPath: "lib/utils/helper.sh",
			expected:   filepath.Join("/home/user/mycommands.invowkbundle", "lib", "utils", "helper.sh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bundle.ResolveScriptPath(tt.scriptPath)
			if result != tt.expected {
				t.Errorf("ResolveScriptPath(%q) = %q, want %q", tt.scriptPath, result, tt.expected)
			}
		})
	}
}

func TestBundle_ValidateScriptPath(t *testing.T) {
	bundle := &Bundle{
		Path:           "/home/user/mycommands.invowkbundle",
		Name:           "mycommands",
		InvowkfilePath: "/home/user/mycommands.invowkbundle/invowkfile.cue",
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
			name:       "path escapes bundle with ..",
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
			err := bundle.ValidateScriptPath(tt.scriptPath)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateScriptPath(%q) expected error, got nil", tt.scriptPath)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateScriptPath(%q) unexpected error: %v", tt.scriptPath, err)
			}
		})
	}
}

func TestBundle_ContainsPath(t *testing.T) {
	// Create a real temp directory for this test
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "mycommands.invowkbundle")
	if err := os.Mkdir(bundlePath, 0755); err != nil {
		t.Fatal(err)
	}

	bundle := &Bundle{
		Path: bundlePath,
		Name: "mycommands",
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "file in bundle root",
			path:     filepath.Join(bundlePath, "invowkfile.cue"),
			expected: true,
		},
		{
			name:     "file in subdirectory",
			path:     filepath.Join(bundlePath, "scripts", "build.sh"),
			expected: true,
		},
		{
			name:     "bundle path itself",
			path:     bundlePath,
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
			result := bundle.ContainsPath(tt.path)
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
				Message: "nested bundles are not allowed",
				Path:    "nested.invowkbundle",
			},
			expected: "[structure] nested.invowkbundle: nested bundles are not allowed",
		},
		{
			name: "issue without path",
			issue: ValidationIssue{
				Type:    "naming",
				Message: "bundle name is invalid",
				Path:    "",
			},
			expected: "[naming] bundle name is invalid",
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
