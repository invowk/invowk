// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestImplementationScriptExplicitMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		script     ImplementationScript
		wantFile   bool
		wantInline bool
	}{
		{"simple inline", ImplementationScript{Content: "echo hello"}, false, true},
		{"file-like inline content", ImplementationScript{Content: "scripts/build.sh"}, false, true},
		{"extensionless file", ImplementationScript{File: filesystemPathPtr("scripts/build")}, true, false},
		{"relative path with ./", ImplementationScript{File: filesystemPathPtr("./script.sh")}, true, false},
		{"absolute unix path", ImplementationScript{File: filesystemPathPtr("/usr/local/bin/script.sh")}, true, false},
		{"uppercase file extension is still explicit", ImplementationScript{File: filesystemPathPtr("BUILD.SH")}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.script.IsFile(); got != tt.wantFile {
				t.Errorf("IsFile() = %v, want %v for script %#v", got, tt.wantFile, tt.script)
			}
			if got := tt.script.IsContent(); got != tt.wantInline {
				t.Errorf("IsContent() = %v, want %v for script %#v", got, tt.wantInline, tt.script)
			}
		})
	}
}

func TestGetScriptFilePath_NonModuleReturnsEmpty(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	invowkfilePath := filepath.Join(baseDir, "invowkfile.cue")
	absScriptPath := filepath.Join(t.TempDir(), "bin", "script.sh")

	tests := []struct {
		name           string
		script         string
		expectedResult bool // true if path should be non-empty
	}{
		{"inline script", "echo hello", false},
		{"relative file", "./scripts/build.sh", true},
		{"absolute file", absScriptPath, true},
		{"simple file", "build.sh", true},
		{"nested relative file", "scripts/deploy/prod.sh", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			script := ImplementationScript{Content: ScriptContent(tt.script)}
			if tt.expectedResult {
				script = ImplementationScript{File: filesystemPathPtr(tt.script)}
			}
			s := &Implementation{Script: script, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
			result := s.GetScriptFilePath(FilesystemPath(invowkfilePath))
			if result != "" {
				t.Errorf("GetScriptFilePath() = %q, want empty string for non-module context", result)
			}
		})
	}
}

func TestResolveScript_Inline(t *testing.T) {
	t.Parallel()

	fakeInvowkfilePath := FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue"))

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

			s := &Implementation{Script: ImplementationScript{Content: ScriptContent(tt.script)}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
			result, err := s.ResolveScript(fakeInvowkfilePath)
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
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a test script file
	scriptContent := "#!/bin/bash\necho 'Hello from script file!'"
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	// Create invowkfile path
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	modulePath := FilesystemPath(tmpDir)

	tests := []struct {
		name         string
		file         string
		want         string
		wantErr      bool
		wantSentinel error
	}{
		{name: "resolve script from file", file: "./test.sh", want: scriptContent},
		{name: "resolve script with absolute path", file: scriptPath, wantErr: true, wantSentinel: ErrInvalidScriptFilePath},
		{name: "error on missing script file", file: "./nonexistent.sh", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &Implementation{Script: ImplementationScript{File: filesystemPathPtr(tt.file)}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}
			result, err := s.ResolveScriptWithFSAndModule(FilesystemPath(invowkfilePath), modulePath, os.ReadFile)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ResolveScriptWithFS() error = nil, want error")
				}
				if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
					t.Errorf("ResolveScriptWithFS() error = %v, want %v", err, tt.wantSentinel)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveScriptWithFS() error = %v", err)
			}
			if result != tt.want {
				t.Errorf("ResolveScriptWithFS() = %q, want %q", result, tt.want)
			}
		})
	}
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
	modulePath := FilesystemPath(projectDir)

	tests := []struct {
		name           string
		implementation Implementation
		withModule     bool
		want           string
		wantErr        bool
	}{
		{name: "resolve script from virtual fs", implementation: Implementation{Script: ImplementationScript{File: filesystemPathPtr("./scripts/build.sh")}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}, withModule: true, want: "#!/bin/bash\ngo build ./..."},
		{name: "inline script bypasses fs", implementation: Implementation{Script: ImplementationScript{Content: "echo hello world"}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}, want: "echo hello world"},
		{name: "error on missing file in virtual fs", implementation: Implementation{Script: ImplementationScript{File: filesystemPathPtr("./scripts/nonexistent.sh")}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}, withModule: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var result string
			var err error
			if tt.withModule {
				result, err = tt.implementation.ResolveScriptWithFSAndModule(FilesystemPath(invowkfilePath), modulePath, readFile)
			} else {
				result, err = tt.implementation.ResolveScriptWithFS(FilesystemPath(invowkfilePath), readFile)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveScriptWithFS() error = %v, wantErr %t", err, tt.wantErr)
			}
			if result != tt.want {
				t.Errorf("ResolveScriptWithFS() = %q, want %q", result, tt.want)
			}
		})
	}
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
					script: {
						content: """
							#!/bin/bash
							set -e
							echo "Line 1"
							echo "Line 2"
							echo "Line 3"
							"""
					}

				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(FilesystemPath(invowkfilePath))
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
	if !strings.Contains(string(cmd.Implementations[0].Script.Content), "Line 1") || !strings.Contains(string(cmd.Implementations[0].Script.Content), "Line 2") {
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
		{name: "dot segment", containerfile: "docker/./Containerfile", shouldError: false},
		{name: "consecutive dots in filename", containerfile: "Containerfile..backup", shouldError: false},
		{name: "consecutive dots in directory", containerfile: "docker/v1..2/Containerfile", shouldError: false},
		{name: "deep path", containerfile: "a/b/c/Dockerfile", shouldError: false},

		// Invalid paths are rejected by Go after CUE validates string shape.
		{name: "absolute path", containerfile: "/etc/Containerfile", shouldError: true, errorSubstring: "path must be relative"},
		{name: "path traversal parent", containerfile: "../Containerfile", shouldError: true, errorSubstring: "parent-directory segment"},
		{name: "path traversal middle", containerfile: "foo/../bar/Containerfile", shouldError: true, errorSubstring: "parent-directory segment"},
		{name: "path traversal backslash", containerfile: `foo\..\bar\Containerfile`, shouldError: true, errorSubstring: "parent-directory segment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			cueContainerfile := strings.ReplaceAll(tt.containerfile, "\\", "\\\\")

			// Create invowkfile.cue with container runtime using the test containerfile path
			cueContent := `cmds: [
	{
		name: "test-cmd"
		description: "Test command"
		implementations: [
			{
					script: {content: "echo test"}
				runtimes: [{name: "container", containerfile: "` + cueContainerfile + `"}]
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

			_, parseErr := Parse(FilesystemPath(invowkfilePath))

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

func TestResolveScriptWithFSReadsCurrentFileContent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a test script file
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if writeErr := os.WriteFile(scriptPath, []byte("original content"), 0o755); writeErr != nil {
		t.Fatalf("Failed to write script file: %v", writeErr)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	modulePath := FilesystemPath(tmpDir)
	s := &Implementation{Script: ImplementationScript{File: filesystemPathPtr("./test.sh")}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}

	// First resolution.
	result1, err := s.ResolveScriptWithFSAndModule(FilesystemPath(invowkfilePath), modulePath, os.ReadFile)
	if err != nil {
		t.Fatalf("First ResolveScriptWithFS() error = %v", err)
	}
	if result1 != "original content" {
		t.Errorf("First ResolveScriptWithFS() = %q, want %q", result1, "original content")
	}

	// Modify the file
	if writeErr := os.WriteFile(scriptPath, []byte("modified content"), 0o755); writeErr != nil {
		t.Fatalf("Failed to modify script file: %v", writeErr)
	}

	// Second resolution reads through the caller-owned filesystem boundary.
	result2, err := s.ResolveScriptWithFSAndModule(FilesystemPath(invowkfilePath), modulePath, os.ReadFile)
	if err != nil {
		t.Fatalf("Second ResolveScriptWithFS() error = %v", err)
	}
	if result2 != "modified content" {
		t.Errorf("Second ResolveScriptWithFS() = %q, want %q", result2, "modified content")
	}
}
