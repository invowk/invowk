package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
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

func TestValidateName(t *testing.T) {
	tests := []struct {
		name       string
		bundleName string
		expectErr  bool
	}{
		{
			name:       "valid simple name",
			bundleName: "mycommands",
			expectErr:  false,
		},
		{
			name:       "valid RDNS name",
			bundleName: "com.example.mycommands",
			expectErr:  false,
		},
		{
			name:       "valid single letter segments",
			bundleName: "a.b.c",
			expectErr:  false,
		},
		{
			name:       "valid with uppercase",
			bundleName: "Com.Example.MyCommands",
			expectErr:  false,
		},
		{
			name:       "valid with numbers",
			bundleName: "com.example123.tools",
			expectErr:  false,
		},
		{
			name:       "empty name",
			bundleName: "",
			expectErr:  true,
		},
		{
			name:       "starts with dot",
			bundleName: ".hidden",
			expectErr:  true,
		},
		{
			name:       "starts with number",
			bundleName: "123invalid",
			expectErr:  true,
		},
		{
			name:       "contains hyphen",
			bundleName: "my-commands",
			expectErr:  true,
		},
		{
			name:       "contains underscore",
			bundleName: "my_commands",
			expectErr:  true,
		},
		{
			name:       "double dots",
			bundleName: "com..example",
			expectErr:  true,
		},
		{
			name:       "segment starts with number",
			bundleName: "com.123example",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.bundleName)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateName(%q) expected error, got nil", tt.bundleName)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateName(%q) unexpected error: %v", tt.bundleName, err)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name      string
		opts      CreateOptions
		expectErr bool
		validate  func(t *testing.T, bundlePath string)
	}{
		{
			name: "create simple bundle",
			opts: CreateOptions{
				Name: "mycommands",
			},
			expectErr: false,
			validate: func(t *testing.T, bundlePath string) {
				// Check bundle directory exists
				info, err := os.Stat(bundlePath)
				if err != nil {
					t.Fatalf("bundle directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("bundle path is not a directory")
				}

				// Check invowkfile.cue exists
				invowkfilePath := filepath.Join(bundlePath, "invowkfile.cue")
				if _, err := os.Stat(invowkfilePath); err != nil {
					t.Errorf("invowkfile.cue not created: %v", err)
				}

				// Verify bundle is valid
				_, err = Load(bundlePath)
				if err != nil {
					t.Errorf("created bundle is not valid: %v", err)
				}
			},
		},
		{
			name: "create RDNS bundle",
			opts: CreateOptions{
				Name: "com.example.mytools",
			},
			expectErr: false,
			validate: func(t *testing.T, bundlePath string) {
				if !strings.HasSuffix(bundlePath, "com.example.mytools.invowkbundle") {
					t.Errorf("unexpected bundle path: %s", bundlePath)
				}
			},
		},
		{
			name: "create bundle with scripts directory",
			opts: CreateOptions{
				Name:             "mytools",
				CreateScriptsDir: true,
			},
			expectErr: false,
			validate: func(t *testing.T, bundlePath string) {
				scriptsDir := filepath.Join(bundlePath, "scripts")
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
			name: "create bundle with custom group",
			opts: CreateOptions{
				Name:  "mytools",
				Group: "custom-group",
			},
			expectErr: false,
			validate: func(t *testing.T, bundlePath string) {
				content, err := os.ReadFile(filepath.Join(bundlePath, "invowkfile.cue"))
				if err != nil {
					t.Fatalf("failed to read invowkfile: %v", err)
				}
				if !strings.Contains(string(content), `group: "custom-group"`) {
					t.Error("custom group not set in invowkfile")
				}
			},
		},
		{
			name: "create bundle with custom description",
			opts: CreateOptions{
				Name:        "mytools",
				Description: "My custom description",
			},
			expectErr: false,
			validate: func(t *testing.T, bundlePath string) {
				content, err := os.ReadFile(filepath.Join(bundlePath, "invowkfile.cue"))
				if err != nil {
					t.Fatalf("failed to read invowkfile: %v", err)
				}
				if !strings.Contains(string(content), `description: "My custom description"`) {
					t.Error("custom description not set in invowkfile")
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

			bundlePath, err := Create(opts)
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
				tt.validate(t, bundlePath)
			}
		})
	}
}

func TestCreate_ExistingBundle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create bundle first time
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
		t.Error("Create() expected error for existing bundle, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestPack(t *testing.T) {
	t.Run("pack valid bundle", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a bundle first
		bundlePath, err := Create(CreateOptions{
			Name:             "mytools",
			ParentDir:        tmpDir,
			CreateScriptsDir: true,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Add a script file
		scriptPath := filepath.Join(bundlePath, "scripts", "test.sh")
		if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
			t.Fatalf("failed to write script: %v", err)
		}

		// Pack the bundle
		outputPath := filepath.Join(tmpDir, "output.zip")
		zipPath, err := Pack(bundlePath, outputPath)
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

		// Create a bundle
		bundlePath, err := Create(CreateOptions{
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

		// Pack with empty output path
		zipPath, err := Pack(bundlePath, "")
		if err != nil {
			t.Fatalf("Pack() failed: %v", err)
		}

		// Verify default name
		expectedName := "com.example.tools.invowkbundle.zip"
		if filepath.Base(zipPath) != expectedName {
			t.Errorf("default ZIP name = %q, expected %q", filepath.Base(zipPath), expectedName)
		}
	})

	t.Run("pack invalid bundle fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid bundle (no invowkfile)
		bundlePath := filepath.Join(tmpDir, "invalid.invowkbundle")
		if err := os.Mkdir(bundlePath, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		_, err := Pack(bundlePath, "")
		if err == nil {
			t.Error("Pack() expected error for invalid bundle, got nil")
		}
	})
}

func TestUnpack(t *testing.T) {
	t.Run("unpack valid bundle from ZIP", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and pack a bundle
		bundlePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "bundle.zip")
		_, err = Pack(bundlePath, zipPath)
		if err != nil {
			t.Fatalf("Pack() failed: %v", err)
		}

		// Remove original bundle
		os.RemoveAll(bundlePath)

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

		// Verify extracted bundle is valid
		b, err := Load(extractedPath)
		if err != nil {
			t.Fatalf("extracted bundle is invalid: %v", err)
		}

		if b.Name != "mytools" {
			t.Errorf("extracted bundle name = %q, expected %q", b.Name, "mytools")
		}
	})

	t.Run("unpack fails for existing bundle without overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and pack a bundle
		bundlePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "bundle.zip")
		_, err = Pack(bundlePath, zipPath)
		if err != nil {
			t.Fatalf("Pack() failed: %v", err)
		}

		// Try to unpack to same directory (bundle already exists)
		_, err = Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   tmpDir,
			Overwrite: false,
		})
		if err == nil {
			t.Error("Unpack() expected error for existing bundle, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("unpack with overwrite replaces existing bundle", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and pack a bundle
		bundlePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "bundle.zip")
		_, err = Pack(bundlePath, zipPath)
		if err != nil {
			t.Fatalf("Pack() failed: %v", err)
		}

		// Modify the existing bundle
		markerFile := filepath.Join(bundlePath, "marker.txt")
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

		// Verify marker file is gone (bundle was replaced)
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

	t.Run("unpack fails for ZIP without bundle", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a ZIP file without a bundle
		zipPath := filepath.Join(tmpDir, "nobundle.zip")
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
			t.Error("Unpack() expected error for ZIP without bundle, got nil")
		}
		if !strings.Contains(err.Error(), "no valid bundle found") {
			t.Errorf("expected 'no valid bundle found' error, got: %v", err)
		}
	})
}
