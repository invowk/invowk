// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsScriptFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		expected bool
	}{
		// Inline scripts (should return false)
		{"empty script", "", false},
		{"simple inline", "echo hello", false},
		{"multi-line inline", "echo hello\necho world", false},
		{"inline with semicolons", "echo hello; echo world", false},
		{"inline command", "go build ./...", false},

		// File paths (should return true)
		{"relative path with ./", "./script.sh", true},
		{"relative path with ../", "../scripts/build.sh", true},
		{"absolute unix path", "/usr/local/bin/script.sh", true},
		{"shell script extension", "build.sh", true},
		{"bash script extension", "build.bash", true},
		{"powershell extension", "build.ps1", true},
		{"batch file extension", "deploy.bat", true},
		{"cmd file extension", "deploy.cmd", true},
		{"python script", "script.py", true},
		{"ruby script", "script.rb", true},
		{"perl script", "script.pl", true},
		{"zsh script", "script.zsh", true},
		{"fish script", "script.fish", true},
		{"relative path to script", "scripts/build.sh", true},

		// Edge cases
		{"script with .sh in name but not extension", "my.sh.backup", false},
		{"script path with spaces", "./my script.sh", true},
		{"uppercase extension", "BUILD.SH", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &Implementation{Script: ScriptContent(tt.script), Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
			result := s.IsScriptFile()
			if result != tt.expected {
				t.Errorf("IsScriptFile() = %v, want %v for script %q", result, tt.expected, tt.script)
			}
		})
	}
}

func TestGetScriptFilePath(t *testing.T) {
	t.Parallel()

	// Use platform-native paths for testing
	var invowkfilePath, baseDir, absScriptPath string
	if runtime.GOOS == "windows" {
		invowkfilePath = `C:\Users\user\project\invowkfile.cue`
		baseDir = `C:\Users\user\project`
		absScriptPath = `C:\scripts\build.sh`
	} else {
		invowkfilePath = "/home/user/project/invowkfile.cue"
		baseDir = "/home/user/project"
		absScriptPath = "/usr/local/bin/script.sh"
	}

	tests := []struct {
		name           string
		script         string
		expectedPath   string
		expectedResult bool // true if path should be non-empty
	}{
		{"inline script", "echo hello", "", false},
		{"relative path", "./scripts/build.sh", filepath.Join(baseDir, "scripts", "build.sh"), true},
		{"absolute path", absScriptPath, absScriptPath, true},
		{"simple filename", "build.sh", filepath.Join(baseDir, "build.sh"), true},
		{"nested relative path", "scripts/deploy/prod.sh", filepath.Join(baseDir, "scripts", "deploy", "prod.sh"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &Implementation{Script: ScriptContent(tt.script), Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
			result := s.GetScriptFilePath(FilesystemPath(invowkfilePath))
			if tt.expectedResult {
				if string(result) != tt.expectedPath {
					t.Errorf("GetScriptFilePath() = %q, want %q", result, tt.expectedPath)
				}
			} else {
				if result != "" {
					t.Errorf("GetScriptFilePath() = %q, want empty string", result)
				}
			}
		})
	}
}

func TestResolveScript_Inline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{"simple inline", "echo hello", "echo hello"},
		{"multi-line inline", "echo hello\necho world", "echo hello\necho world"},
		{"inline with variables", "echo $HOME", "echo $HOME"},
		{
			"complex multi-line", "#!/bin/bash\nset -e\necho 'Starting...'\ngo build ./...\necho 'Done!'",
			"#!/bin/bash\nset -e\necho 'Starting...'\ngo build ./...\necho 'Done!'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &Implementation{Script: ScriptContent(tt.script), Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
			result, err := s.ResolveScript(FilesystemPath("/fake/path/invowkfile.cue"))
			if err != nil {
				t.Errorf("ResolveScript() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("ResolveScript() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestResolveScript_FromFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	// Create a test script file
	scriptContent := "#!/bin/bash\necho 'Hello from script file!'"
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	// Create invowkfile path
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	t.Run("resolve script from file", func(t *testing.T) {
		s := &Implementation{Script: "./test.sh", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
		result, err := s.ResolveScript(FilesystemPath(invowkfilePath))
		if err != nil {
			t.Errorf("ResolveScript() error = %v", err)
			return
		}
		if result != scriptContent {
			t.Errorf("ResolveScript() = %q, want %q", result, scriptContent)
		}
	})

	t.Run("resolve script with absolute path", func(t *testing.T) {
		s := &Implementation{Script: ScriptContent(scriptPath), Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
		result, err := s.ResolveScript(FilesystemPath(invowkfilePath))
		if err != nil {
			t.Errorf("ResolveScript() error = %v", err)
			return
		}
		if result != scriptContent {
			t.Errorf("ResolveScript() = %q, want %q", result, scriptContent)
		}
	})

	t.Run("error on missing script file", func(t *testing.T) {
		s := &Implementation{Script: "./nonexistent.sh", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
		_, err := s.ResolveScript(FilesystemPath(invowkfilePath))
		if err == nil {
			t.Error("ResolveScript() expected error for missing file, got nil")
		}
	})
}

func TestResolveScriptWithFS(t *testing.T) {
	t.Parallel()

	// Use platform-native paths for the virtual filesystem
	var projectDir string
	if runtime.GOOS == "windows" {
		projectDir = `C:\project`
	} else {
		projectDir = "/project"
	}

	virtualFS := map[string]string{
		filepath.Join(projectDir, "scripts", "build.sh"):  "#!/bin/bash\ngo build ./...",
		filepath.Join(projectDir, "scripts", "deploy.sh"): "#!/bin/bash\nkubectl apply -f k8s/",
	}

	readFile := func(path string) ([]byte, error) {
		if content, ok := virtualFS[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}

	invowkfilePath := filepath.Join(projectDir, "invowkfile.cue")

	t.Run("resolve script from virtual fs", func(t *testing.T) {
		t.Parallel()

		s := &Implementation{Script: "./scripts/build.sh", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
		result, err := s.ResolveScriptWithFS(FilesystemPath(invowkfilePath), readFile)
		if err != nil {
			t.Errorf("ResolveScriptWithFS() error = %v", err)
			return
		}
		expected := "#!/bin/bash\ngo build ./..."
		if result != expected {
			t.Errorf("ResolveScriptWithFS() = %q, want %q", result, expected)
		}
	})

	t.Run("inline script bypasses fs", func(t *testing.T) {
		t.Parallel()

		s := &Implementation{Script: "echo hello world", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
		result, err := s.ResolveScriptWithFS(FilesystemPath(invowkfilePath), readFile)
		if err != nil {
			t.Errorf("ResolveScriptWithFS() error = %v", err)
			return
		}
		if result != "echo hello world" {
			t.Errorf("ResolveScriptWithFS() = %q, want %q", result, "echo hello world")
		}
	})

	t.Run("error on missing file in virtual fs", func(t *testing.T) {
		t.Parallel()

		s := &Implementation{Script: "./scripts/nonexistent.sh", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
		_, err := s.ResolveScriptWithFS(FilesystemPath(invowkfilePath), readFile)
		if err == nil {
			t.Error("ResolveScriptWithFS() expected error for missing file, got nil")
		}
	})
}

func TestMultiLineScriptParsing(t *testing.T) {
	t.Parallel()

	// Test that CUE multi-line strings are properly parsed
	cueContent := `
cmds: [
	{
		name: "multiline-test"
		description: "Test multi-line script"
		implementations: [
			{
				script: """
					#!/bin/bash
					set -e
					echo "Line 1"
					echo "Line 2"
					echo "Line 3"
					"""

				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`

	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	// CUE multi-line strings preserve the content with tabs stripped based on first line indent
	if len(cmd.Implementations) == 0 {
		t.Fatal("Expected at least 1 script")
	}
	if !strings.Contains(string(cmd.Implementations[0].Script), "Line 1") || !strings.Contains(string(cmd.Implementations[0].Script), "Line 2") {
		t.Errorf("Multi-line script parsing failed.\nGot: %q", cmd.Implementations[0].Script)
	}

	// Verify resolution works too
	resolved, err := cmd.Implementations[0].ResolveScript(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Errorf("ResolveScript() error = %v", err)
	}
	if !strings.Contains(resolved, "Line 1") {
		t.Errorf("ResolveScript() missing expected content, got: %q", resolved)
	}
}

// TestContainerfilePathCUEValidation verifies that the CUE schema correctly validates
// containerfile paths. This is a regression test for a bug where CUE's strings.HasPrefix
// and strings.Contains functions were used incorrectly, causing valid containerfile paths
// to fail validation with "invalid operation !strings.HasPrefix(...)" errors.
func TestContainerfilePathCUEValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		containerfile  string
		shouldError    bool
		errorSubstring string
	}{
		// Valid paths - should parse successfully
		{name: "simple filename", containerfile: "Containerfile", shouldError: false},
		{name: "subdirectory", containerfile: "docker/Containerfile", shouldError: false},
		{name: "dot prefix", containerfile: "./Containerfile", shouldError: false},
		{name: "deep path", containerfile: "a/b/c/Dockerfile", shouldError: false},

		// Invalid paths - should be rejected by CUE schema
		{name: "absolute path", containerfile: "/etc/Containerfile", shouldError: true, errorSubstring: "out of bound"},
		{name: "path traversal parent", containerfile: "../Containerfile", shouldError: true, errorSubstring: "out of bound"},
		{name: "path traversal middle", containerfile: "foo/../bar/Containerfile", shouldError: true, errorSubstring: "out of bound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir, err := os.MkdirTemp("", "invowkfile-containerfile-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			// Create invowkfile.cue with container runtime using the test containerfile path
			cueContent := `cmds: [
	{
		name: "test-cmd"
		description: "Test command"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "container", containerfile: "` + tt.containerfile + `"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]`
			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invowkfile: %v", writeErr)
			}

			// Also create the containerfile if it's a valid relative path (for Go validation to pass)
			if !tt.shouldError {
				containerfilePath := filepath.Join(tmpDir, tt.containerfile)
				if mkdirErr := os.MkdirAll(filepath.Dir(containerfilePath), 0o755); mkdirErr != nil {
					t.Fatalf("Failed to create containerfile dir: %v", mkdirErr)
				}
				if writeErr := os.WriteFile(containerfilePath, []byte("FROM debian:stable-slim\n"), 0o644); writeErr != nil {
					t.Fatalf("Failed to write containerfile: %v", writeErr)
				}
			}

			_, parseErr := Parse(invowkfilePath)

			if tt.shouldError {
				if parseErr == nil {
					t.Errorf("Expected CUE validation error for containerfile %q, but parsing succeeded", tt.containerfile)
				} else if tt.errorSubstring != "" && !strings.Contains(parseErr.Error(), tt.errorSubstring) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorSubstring, parseErr)
				}
			} else {
				if parseErr != nil {
					t.Errorf("Unexpected error for valid containerfile %q: %v", tt.containerfile, parseErr)
				}
			}
		})
	}
}

func TestScriptCaching(t *testing.T) {
	t.Parallel()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	// Create a test script file
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if writeErr := os.WriteFile(scriptPath, []byte("original content"), 0o755); writeErr != nil {
		t.Fatalf("Failed to write script file: %v", writeErr)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	s := &Implementation{Script: "./test.sh", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}

	// First resolution
	result1, err := s.ResolveScript(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("First ResolveScript() error = %v", err)
	}
	if result1 != "original content" {
		t.Errorf("First ResolveScript() = %q, want %q", result1, "original content")
	}

	// Modify the file
	if writeErr := os.WriteFile(scriptPath, []byte("modified content"), 0o755); writeErr != nil {
		t.Fatalf("Failed to modify script file: %v", writeErr)
	}

	// Second resolution should return cached content
	result2, err := s.ResolveScript(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Second ResolveScript() error = %v", err)
	}
	if result2 != "original content" {
		t.Errorf("Caching failed: second ResolveScript() = %q, want cached %q", result2, "original content")
	}
}
