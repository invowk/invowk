// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Module Validation Tests
// ============================================================================

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setup          func(t *testing.T) string // returns path to module
		expectValid    bool
		expectIssues   int
		checkIssueType string // optional: check that at least one issue has this type
	}{
		{
			name: "valid module with invowkmod.cue and invowkfile.cue",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				return createValidModule(t, dir, "mycommands.invowkmod", "mycommands")
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "valid RDNS module",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				return createValidModule(t, dir, "com.example.mycommands.invowkmod", "com.example.mycommands")
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "library-only module (no invowkfile.cue)",
			setup: func(t *testing.T) string {
				t.Helper()
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
				return modulePath
			},
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name: "missing invowkmod.cue (required)",
			setup: func(t *testing.T) string {
				t.Helper()
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
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "module field mismatches folder name",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Create invowkmod.cue with WRONG module ID
				invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
				if err := os.WriteFile(invowkmodPath, []byte(`module: "wrongname"
version: "1.0.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("cmds: []"), 0o644); err != nil {
					t.Fatal(err)
				}
				return modulePath
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
		{
			name: "invowkmod.cue is a directory",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				invowkmodDir := filepath.Join(modulePath, "invowkmod.cue")
				if err := os.Mkdir(invowkmodDir, 0o755); err != nil {
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
				t.Helper()
				dir := t.TempDir()
				modulePath := createValidModule(t, dir, "mycommands.invowkmod", "mycommands")
				// Create nested module
				nestedPath := filepath.Join(modulePath, "nested.invowkmod")
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
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "123invalid.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
				if err := os.WriteFile(invowkmodPath, []byte(`module: "test"
version: "1.0.0"
`), 0o644); err != nil {
					t.Fatal(err)
				}
				invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
				if err := os.WriteFile(invowkfilePath, []byte("cmds: []"), 0o644); err != nil {
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
				t.Helper()
				return "/nonexistent/path/mycommands.invowkmod"
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "structure",
		},
		{
			name: "path is a file not directory",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				filePath := filepath.Join(dir, "mycommands.invowkmod")
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
			name: "multiple issues - missing invowkmod.cue and nested module",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				modulePath := filepath.Join(dir, "mycommands.invowkmod")
				if err := os.Mkdir(modulePath, 0o755); err != nil {
					t.Fatal(err)
				}
				// Create nested module (but no invowkmod.cue)
				nestedPath := filepath.Join(modulePath, "nested.invowkmod")
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
				t.Helper()
				dir := t.TempDir()
				modulePath := createValidModule(t, dir, "mycommands.invowkmod", "mycommands")
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
		{
			name: "reserved module name 'invowkfile' rejected (FR-015)",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				// Create module with reserved name "invowkfile"
				return createValidModule(t, dir, "invowkfile.invowkmod", "invowkfile")
			},
			expectValid:    false,
			expectIssues:   1,
			checkIssueType: "naming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

// ============================================================================
// ValidationIssue Tests
// ============================================================================

func TestValidationIssue_Error(t *testing.T) {
	t.Parallel()

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
				Path:    "nested.invowkmod",
			},
			expected: "[structure] nested.invowkmod: nested modules are not allowed",
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
			t.Parallel()

			result := tt.issue.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// ValidateName Tests
// ============================================================================

func TestValidateName(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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
