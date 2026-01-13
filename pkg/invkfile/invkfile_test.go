// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testCommand creates a Command with a single script for testing purposes
func testCommand(name string, script string) Command {
	return Command{
		Name: name,
		Implementations: []Implementation{
			{Script: script, Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}},
		},
	}
}

// testCommandWithDeps creates a Command with a single script and dependencies for testing
func testCommandWithDeps(name string, script string, deps *DependsOn) Command {
	return Command{
		Name:            name,
		Implementations: []Implementation{{Script: script, Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}},
		DependsOn:       deps,
	}
}

func TestIsScriptFile(t *testing.T) {
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
			s := &Implementation{Script: tt.script, Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
			result := s.IsScriptFile()
			if result != tt.expected {
				t.Errorf("IsScriptFile() = %v, want %v for script %q", result, tt.expected, tt.script)
			}
		})
	}
}

func TestGetScriptFilePath(t *testing.T) {
	invkfilePath := "/home/user/project/invkfile.cue"

	tests := []struct {
		name           string
		script         string
		expectedPath   string
		expectedResult bool // true if path should be non-empty
	}{
		{"inline script", "echo hello", "", false},
		{"relative path", "./scripts/build.sh", "/home/user/project/scripts/build.sh", true},
		{"absolute path", "/usr/local/bin/script.sh", "/usr/local/bin/script.sh", true},
		{"simple filename", "build.sh", "/home/user/project/build.sh", true},
		{"nested relative path", "scripts/deploy/prod.sh", "/home/user/project/scripts/deploy/prod.sh", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Implementation{Script: tt.script, Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
			result := s.GetScriptFilePath(invkfilePath)
			if tt.expectedResult {
				if result != tt.expectedPath {
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
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{"simple inline", "echo hello", "echo hello"},
		{"multi-line inline", "echo hello\necho world", "echo hello\necho world"},
		{"inline with variables", "echo $HOME", "echo $HOME"},
		{"complex multi-line", "#!/bin/bash\nset -e\necho 'Starting...'\ngo build ./...\necho 'Done!'",
			"#!/bin/bash\nset -e\necho 'Starting...'\ngo build ./...\necho 'Done!'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Implementation{Script: tt.script, Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
			result, err := s.ResolveScript("/fake/path/invkfile.cue")
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
	defer os.RemoveAll(tmpDir)

	// Create a test script file
	scriptContent := "#!/bin/bash\necho 'Hello from script file!'"
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	// Create invkfile path
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	t.Run("resolve script from file", func(t *testing.T) {
		s := &Implementation{Script: "./test.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		result, err := s.ResolveScript(invkfilePath)
		if err != nil {
			t.Errorf("ResolveScript() error = %v", err)
			return
		}
		if result != scriptContent {
			t.Errorf("ResolveScript() = %q, want %q", result, scriptContent)
		}
	})

	t.Run("resolve script with absolute path", func(t *testing.T) {
		s := &Implementation{Script: scriptPath, Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		result, err := s.ResolveScript(invkfilePath)
		if err != nil {
			t.Errorf("ResolveScript() error = %v", err)
			return
		}
		if result != scriptContent {
			t.Errorf("ResolveScript() = %q, want %q", result, scriptContent)
		}
	})

	t.Run("error on missing script file", func(t *testing.T) {
		s := &Implementation{Script: "./nonexistent.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		_, err := s.ResolveScript(invkfilePath)
		if err == nil {
			t.Error("ResolveScript() expected error for missing file, got nil")
		}
	})
}

func TestResolveScriptWithFS(t *testing.T) {
	// Virtual filesystem
	virtualFS := map[string]string{
		"/project/scripts/build.sh":  "#!/bin/bash\ngo build ./...",
		"/project/scripts/deploy.sh": "#!/bin/bash\nkubectl apply -f k8s/",
	}

	readFile := func(path string) ([]byte, error) {
		if content, ok := virtualFS[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}

	invkfilePath := "/project/invkfile.cue"

	t.Run("resolve script from virtual fs", func(t *testing.T) {
		s := &Implementation{Script: "./scripts/build.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		result, err := s.ResolveScriptWithFS(invkfilePath, readFile)
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
		s := &Implementation{Script: "echo hello world", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		result, err := s.ResolveScriptWithFS(invkfilePath, readFile)
		if err != nil {
			t.Errorf("ResolveScriptWithFS() error = %v", err)
			return
		}
		if result != "echo hello world" {
			t.Errorf("ResolveScriptWithFS() = %q, want %q", result, "echo hello world")
		}
	})

	t.Run("error on missing file in virtual fs", func(t *testing.T) {
		s := &Implementation{Script: "./scripts/nonexistent.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		_, err := s.ResolveScriptWithFS(invkfilePath, readFile)
		if err == nil {
			t.Error("ResolveScriptWithFS() expected error for missing file, got nil")
		}
	})
}

func TestMultiLineScriptParsing(t *testing.T) {
	// Test that CUE multi-line strings are properly parsed
	cueContent := `
group: "test"
version: "1.0"

commands: [
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
				target: {
					runtimes: [{name: "native"}]
				}
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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
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
	if !strings.Contains(cmd.Implementations[0].Script, "Line 1") || !strings.Contains(cmd.Implementations[0].Script, "Line 2") {
		t.Errorf("Multi-line script parsing failed.\nGot: %q", cmd.Implementations[0].Script)
	}

	// Verify resolution works too
	resolved, err := cmd.Implementations[0].ResolveScript(invkfilePath)
	if err != nil {
		t.Errorf("ResolveScript() error = %v", err)
	}
	if !strings.Contains(resolved, "Line 1") {
		t.Errorf("ResolveScript() missing expected content, got: %q", resolved)
	}
}

func TestScriptCaching(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test script file
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte("original content"), 0755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	s := &Implementation{Script: "./test.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}

	// First resolution
	result1, err := s.ResolveScript(invkfilePath)
	if err != nil {
		t.Fatalf("First ResolveScript() error = %v", err)
	}
	if result1 != "original content" {
		t.Errorf("First ResolveScript() = %q, want %q", result1, "original content")
	}

	// Modify the file
	if err := os.WriteFile(scriptPath, []byte("modified content"), 0755); err != nil {
		t.Fatalf("Failed to modify script file: %v", err)
	}

	// Second resolution should return cached content
	result2, err := s.ResolveScript(invkfilePath)
	if err != nil {
		t.Fatalf("Second ResolveScript() error = %v", err)
	}
	if result2 != "original content" {
		t.Errorf("Caching failed: second ResolveScript() = %q, want cached %q", result2, "original content")
	}
}

func TestCommand_HasDependencies(t *testing.T) {
	tests := []struct {
		name     string
		cmd      Command
		expected bool
	}{
		{
			name:     "nil DependsOn",
			cmd:      testCommand("test", "echo"),
			expected: false,
		},
		{
			name:     "empty DependsOn",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{}),
			expected: false,
		},
		{
			name:     "only tools",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Tools: []ToolDependency{{Alternatives: []string{"git"}}}}),
			expected: true,
		},
		{
			name:     "only commands",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"build"}}}}),
			expected: true,
		},
		{
			name: "both tools and commands",
			cmd: testCommandWithDeps("test", "echo", &DependsOn{
				Tools:    []ToolDependency{{Alternatives: []string{"git"}}},
				Commands: []CommandDependency{{Alternatives: []string{"build"}}},
			}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.HasDependencies()
			if result != tt.expected {
				t.Errorf("HasDependencies() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommand_GetCommandDependencies(t *testing.T) {
	tests := []struct {
		name     string
		cmd      Command
		expected []string
	}{
		{
			name:     "nil DependsOn",
			cmd:      testCommand("test", "echo"),
			expected: nil,
		},
		{
			name:     "empty DependsOn",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{}),
			expected: []string{},
		},
		{
			name:     "single command",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Commands: []CommandDependency{{Alternatives: []string{"build"}}}}),
			expected: []string{"build"},
		},
		{
			name: "multiple commands",
			cmd: testCommandWithDeps("test", "echo", &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []string{"clean"}},
					{Alternatives: []string{"build"}},
					{Alternatives: []string{"test unit"}},
				},
			}),
			expected: []string{"clean", "build", "test unit"},
		},
		{
			name:     "only tools no commands",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Tools: []ToolDependency{{Alternatives: []string{"git"}}}}),
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetCommandDependencies()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("GetCommandDependencies() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("GetCommandDependencies() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, name := range result {
				if name != tt.expected[i] {
					t.Errorf("GetCommandDependencies()[%d] = %q, want %q", i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestParseDependsOn(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "release"
		implementations: [
			{
				script: "echo releasing"
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["git"]},
				{alternatives: ["docker"]},
			]
			commands: [
				{alternatives: ["build"]},
				{alternatives: ["test unit"]},
			]
			custom_checks: [
				{name: "docker-version", check_script: "docker --version", expected_output: "Docker"},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	// Check tools
	if len(cmd.DependsOn.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(cmd.DependsOn.Tools))
	}

	if len(cmd.DependsOn.Tools[0].Alternatives) == 0 || cmd.DependsOn.Tools[0].Alternatives[0] != "git" {
		t.Errorf("First tool alternatives = %v, want [git]", cmd.DependsOn.Tools[0].Alternatives)
	}

	if len(cmd.DependsOn.Tools[1].Alternatives) == 0 || cmd.DependsOn.Tools[1].Alternatives[0] != "docker" {
		t.Errorf("Second tool alternatives = %v, want [docker]", cmd.DependsOn.Tools[1].Alternatives)
	}

	// Check custom_checks
	if len(cmd.DependsOn.CustomChecks) != 1 {
		t.Errorf("Expected 1 custom_check, got %d", len(cmd.DependsOn.CustomChecks))
	}

	checks := cmd.DependsOn.CustomChecks[0].GetChecks()
	if len(checks) == 0 {
		t.Fatal("Expected at least one check from CustomCheckDependency")
	}

	if checks[0].Name != "docker-version" {
		t.Errorf("First custom_check name = %q, want %q", checks[0].Name, "docker-version")
	}

	if checks[0].CheckScript != "docker --version" {
		t.Errorf("First custom_check check_script = %q, want %q", checks[0].CheckScript, "docker --version")
	}

	if checks[0].ExpectedOutput != "Docker" {
		t.Errorf("First custom_check expected_output = %q, want %q", checks[0].ExpectedOutput, "Docker")
	}

	// Check commands
	if len(cmd.DependsOn.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(cmd.DependsOn.Commands))
	}

	if len(cmd.DependsOn.Commands[0].Alternatives) == 0 || cmd.DependsOn.Commands[0].Alternatives[0] != "build" {
		t.Errorf("First command alternatives = %v, want [build]", cmd.DependsOn.Commands[0].Alternatives)
	}

	if len(cmd.DependsOn.Commands[1].Alternatives) == 0 || cmd.DependsOn.Commands[1].Alternatives[0] != "test unit" {
		t.Errorf("Second command alternatives = %v, want [test unit]", cmd.DependsOn.Commands[1].Alternatives)
	}
}

func TestParseDependsOn_ToolsOnly(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["make"]},
				{alternatives: ["gcc"]},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(cmd.DependsOn.Tools))
	}

	if len(cmd.DependsOn.Commands) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(cmd.DependsOn.Commands))
	}
}

func TestParseDependsOn_CommandsOnly(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "release"
		implementations: [
			{
				script: "echo release"
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			commands: [
				{alternatives: ["build"]},
				{alternatives: ["test"]},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(cmd.DependsOn.Tools))
	}

	if len(cmd.DependsOn.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(cmd.DependsOn.Commands))
	}
}

func TestParseDependsOn_WithCustomChecks(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["make"]},
				{alternatives: ["go"]},
			]
			custom_checks: [
				{
					name: "go-version"
					check_script: "go version"
					expected_code: 0
					expected_output: "go1\\."
				},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(cmd.DependsOn.Tools))
	}

	// First tool - simple
	if len(cmd.DependsOn.Tools[0].Alternatives) == 0 || cmd.DependsOn.Tools[0].Alternatives[0] != "make" {
		t.Errorf("First tool alternatives = %v, want [make]", cmd.DependsOn.Tools[0].Alternatives)
	}

	// Second tool - simple
	if len(cmd.DependsOn.Tools[1].Alternatives) == 0 || cmd.DependsOn.Tools[1].Alternatives[0] != "go" {
		t.Errorf("Second tool alternatives = %v, want [go]", cmd.DependsOn.Tools[1].Alternatives)
	}

	// Custom check with validation
	if len(cmd.DependsOn.CustomChecks) != 1 {
		t.Fatalf("Expected 1 custom_check, got %d", len(cmd.DependsOn.CustomChecks))
	}

	checks := cmd.DependsOn.CustomChecks[0].GetChecks()
	if len(checks) == 0 {
		t.Fatal("Expected at least one check from CustomCheckDependency")
	}
	goCheck := checks[0]
	if goCheck.Name != "go-version" {
		t.Errorf("Custom check name = %q, want %q", goCheck.Name, "go-version")
	}
	if goCheck.CheckScript != "go version" {
		t.Errorf("Custom check check_script = %q, want %q", goCheck.CheckScript, "go version")
	}
	if goCheck.ExpectedCode == nil {
		t.Error("Custom check expected_code should not be nil")
	} else if *goCheck.ExpectedCode != 0 {
		t.Errorf("Custom check expected_code = %d, want 0", *goCheck.ExpectedCode)
	}
	if goCheck.ExpectedOutput != "go1\\." {
		t.Errorf("Custom check expected_output = %q, want %q", goCheck.ExpectedOutput, "go1\\.")
	}
}

func TestParseDependsOn_WithFilepaths(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["config.yaml"]},
				{alternatives: ["secrets.env"], readable: true},
				{alternatives: ["output"], writable: true},
				{alternatives: ["scripts/deploy.sh"], executable: true},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Filepaths) != 4 {
		t.Fatalf("Expected 4 filepaths, got %d", len(cmd.DependsOn.Filepaths))
	}

	// First filepath - simple existence check
	fp0 := cmd.DependsOn.Filepaths[0]
	if len(fp0.Alternatives) != 1 || fp0.Alternatives[0] != "config.yaml" {
		t.Errorf("First filepath alternatives = %v, want [config.yaml]", fp0.Alternatives)
	}
	if fp0.Readable || fp0.Writable || fp0.Executable {
		t.Error("First filepath should have no permission checks")
	}

	// Second filepath - readable
	fp1 := cmd.DependsOn.Filepaths[1]
	if len(fp1.Alternatives) != 1 || fp1.Alternatives[0] != "secrets.env" {
		t.Errorf("Second filepath alternatives = %v, want [secrets.env]", fp1.Alternatives)
	}
	if !fp1.Readable {
		t.Error("Second filepath should be readable")
	}

	// Third filepath - writable
	fp2 := cmd.DependsOn.Filepaths[2]
	if len(fp2.Alternatives) != 1 || fp2.Alternatives[0] != "output" {
		t.Errorf("Third filepath alternatives = %v, want [output]", fp2.Alternatives)
	}
	if !fp2.Writable {
		t.Error("Third filepath should be writable")
	}

	// Fourth filepath - executable
	fp3 := cmd.DependsOn.Filepaths[3]
	if len(fp3.Alternatives) != 1 || fp3.Alternatives[0] != "scripts/deploy.sh" {
		t.Errorf("Fourth filepath alternatives = %v, want [scripts/deploy.sh]", fp3.Alternatives)
	}
	if !fp3.Executable {
		t.Error("Fourth filepath should be executable")
	}
}

func TestCommand_HasDependencies_WithFilepaths(t *testing.T) {
	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}}},
		DependsOn: &DependsOn{
			Filepaths: []FilepathDependency{{Alternatives: []string{"config.yaml"}}},
		},
	}

	if !cmd.HasDependencies() {
		t.Error("HasDependencies() should return true when filepaths are present")
	}
}

func TestGenerateCUE_WithFilepaths(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name:            "deploy",
				Implementations: []Implementation{{Script: "echo deploy", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}, {Name: PlatformMac}}}}},
				DependsOn: &DependsOn{
					Filepaths: []FilepathDependency{
						{Alternatives: []string{"config.yaml"}},
						{Alternatives: []string{"secrets.env"}, Readable: true},
						{Alternatives: []string{"output"}, Writable: true},
						{Alternatives: []string{"deploy.sh"}, Executable: true},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check that filepaths structure is present
	if !strings.Contains(output, "filepaths:") {
		t.Error("GenerateCUE should contain 'filepaths:'")
	}

	if !strings.Contains(output, "alternatives:") {
		t.Error("GenerateCUE should contain 'alternatives:'")
	}

	if !strings.Contains(output, `"config.yaml"`) {
		t.Error("GenerateCUE should contain config.yaml")
	}

	if !strings.Contains(output, "readable: true") {
		t.Error("GenerateCUE should contain readable flag")
	}

	if !strings.Contains(output, "writable: true") {
		t.Error("GenerateCUE should contain writable flag")
	}

	if !strings.Contains(output, "executable: true") {
		t.Error("GenerateCUE should contain executable flag")
	}
}

func TestParsePlatforms(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				target: { runtimes: [{name: "native"}] }
				// No platforms = all platforms
			}
		]
	},
	{
		name: "deploy"
		implementations: [
			{
				script: "deploy.sh"
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(inv.Commands))
	}

	// First command - all platforms (no platforms specified)
	cmd1 := inv.Commands[0]
	platforms1 := cmd1.GetSupportedPlatforms()
	if len(platforms1) != 3 {
		t.Errorf("Expected 3 platforms for first command, got %d", len(platforms1))
	}

	// Second command - linux only
	cmd2 := inv.Commands[1]
	platforms2 := cmd2.GetSupportedPlatforms()
	if len(platforms2) != 1 {
		t.Errorf("Expected 1 platform for second command, got %d", len(platforms2))
	}
	if platforms2[0] != HostLinux {
		t.Errorf("First platform = %q, want %q", platforms2[0], HostLinux)
	}
}

func TestGenerateCUE_WithPlatforms(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "build",
				Implementations: []Implementation{
					{Script: "make build", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}},
				},
			},
			{
				Name: "clean",
				Implementations: []Implementation{
					{Script: "rm -rf bin/", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}, {Name: PlatformMac}}}},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check that scripts structure is present
	if !strings.Contains(output, "implementations:") {
		t.Error("GenerateCUE should contain 'implementations:'")
	}

	if !strings.Contains(output, "target: {") {
		t.Error("GenerateCUE should contain 'target: {'")
	}

	if !strings.Contains(output, `"linux"`) {
		t.Error("GenerateCUE should contain 'linux'")
	}

	if !strings.Contains(output, `"macos"`) {
		t.Error("GenerateCUE should contain 'macos'")
	}
}

// Tests for enable_host_ssh functionality

func TestParseEnableHostSSH(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "container-ssh"
		description: "Container command with host SSH enabled"
		implementations: [
			{
				script: "echo hello"
				target: {
					runtimes: [{name: "container", image: "alpine:latest", enable_host_ssh: true}]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Implementations) != 1 {
		t.Fatalf("Expected 1 implementation, got %d", len(cmd.Implementations))
	}

	impl := cmd.Implementations[0]
	if len(impl.Target.Runtimes) != 1 {
		t.Fatalf("Expected 1 runtime, got %d", len(impl.Target.Runtimes))
	}

	rt := impl.Target.Runtimes[0]
	if rt.Name != RuntimeContainer {
		t.Errorf("Runtime name = %q, want %q", rt.Name, RuntimeContainer)
	}

	if !rt.EnableHostSSH {
		t.Error("EnableHostSSH should be true")
	}

	if rt.Image != "alpine:latest" {
		t.Errorf("Image = %q, want %q", rt.Image, "alpine:latest")
	}
}

func TestParseEnableHostSSH_DefaultFalse(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "container-no-ssh"
		implementations: [
			{
				script: "echo hello"
				target: {
					runtimes: [{name: "container", image: "alpine:latest"}]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	rt := cmd.Implementations[0].Target.Runtimes[0]

	if rt.EnableHostSSH {
		t.Error("EnableHostSSH should be false by default")
	}
}

func TestScript_HasHostSSH(t *testing.T) {
	tests := []struct {
		name     string
		script   Implementation
		expected bool
	}{
		{
			name: "container with enable_host_ssh true",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeContainer, EnableHostSSH: true, Image: "alpine:latest"},
					},
				},
			},
			expected: true,
		},
		{
			name: "container with enable_host_ssh false",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeContainer, EnableHostSSH: false, Image: "alpine:latest"},
					},
				},
			},
			expected: false,
		},
		{
			name: "native runtime (no enable_host_ssh)",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeNative},
					},
				},
			},
			expected: false,
		},
		{
			name: "multiple runtimes, one with enable_host_ssh",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeNative},
						{Name: RuntimeContainer, EnableHostSSH: true, Image: "alpine:latest"},
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple container runtimes, none with enable_host_ssh",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeContainer, EnableHostSSH: false, Image: "alpine:latest"},
						{Name: RuntimeNative},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.script.HasHostSSH()
			if result != tt.expected {
				t.Errorf("HasHostSSH() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScript_GetHostSSHForRuntime(t *testing.T) {
	tests := []struct {
		name     string
		script   Implementation
		runtime  RuntimeMode
		expected bool
	}{
		{
			name: "container runtime with enable_host_ssh true",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeContainer, EnableHostSSH: true, Image: "alpine:latest"},
					},
				},
			},
			runtime:  RuntimeContainer,
			expected: true,
		},
		{
			name: "container runtime with enable_host_ssh false",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeContainer, EnableHostSSH: false, Image: "alpine:latest"},
					},
				},
			},
			runtime:  RuntimeContainer,
			expected: false,
		},
		{
			name: "native runtime always returns false",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeNative},
					},
				},
			},
			runtime:  RuntimeNative,
			expected: false,
		},
		{
			name: "virtual runtime always returns false",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeVirtual},
					},
				},
			},
			runtime:  RuntimeVirtual,
			expected: false,
		},
		{
			name: "runtime not found returns false",
			script: Implementation{
				Script: "echo test",
				Target: Target{
					Runtimes: []RuntimeConfig{
						{Name: RuntimeNative},
					},
				},
			},
			runtime:  RuntimeContainer,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.script.GetHostSSHForRuntime(tt.runtime)
			if result != tt.expected {
				t.Errorf("GetHostSSHForRuntime(%s) = %v, want %v", tt.runtime, result, tt.expected)
			}
		})
	}
}

func TestValidateEnableHostSSH_InvalidForNonContainer(t *testing.T) {
	// Test that enable_host_ssh is rejected for non-container runtimes
	// This tests the Go validation, not the CUE schema (CUE schema only allows enable_host_ssh for container)

	rt := &RuntimeConfig{
		Name:          RuntimeNative,
		EnableHostSSH: true, // Invalid for native runtime
	}

	err := validateRuntimeConfig(rt, "test-cmd", 1)
	if err == nil {
		t.Error("Expected error for enable_host_ssh on native runtime, got nil")
	}

	if !strings.Contains(err.Error(), "enable_host_ssh") {
		t.Errorf("Error should mention enable_host_ssh, got: %v", err)
	}
}

func TestValidateEnableHostSSH_ValidForContainer(t *testing.T) {
	rt := &RuntimeConfig{
		Name:          RuntimeContainer,
		EnableHostSSH: true,
		Image:         "alpine:latest",
	}

	err := validateRuntimeConfig(rt, "test-cmd", 1)
	if err != nil {
		t.Errorf("Unexpected error for enable_host_ssh on container runtime: %v", err)
	}
}

func TestGenerateCUE_WithEnableHostSSH(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "container-ssh",
				Implementations: []Implementation{
					{
						Script: "echo hello",
						Target: Target{
							Runtimes: []RuntimeConfig{
								{Name: RuntimeContainer, EnableHostSSH: true, Image: "alpine:latest"},
							},
						},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, "enable_host_ssh: true") {
		t.Error("GenerateCUE should contain 'enable_host_ssh: true'")
	}

	if !strings.Contains(output, `image: "alpine:latest"`) {
		t.Error("GenerateCUE should contain image specification")
	}
}

func TestGenerateCUE_WithEnableHostSSH_False(t *testing.T) {
	// When enable_host_ssh is false (default), it should not appear in the output
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "container-no-ssh",
				Implementations: []Implementation{
					{
						Script: "echo hello",
						Target: Target{
							Runtimes: []RuntimeConfig{
								{Name: RuntimeContainer, EnableHostSSH: false, Image: "alpine:latest"},
							},
						},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if strings.Contains(output, "enable_host_ssh") {
		t.Error("GenerateCUE should not contain 'enable_host_ssh' when it's false")
	}
}

func TestParseContainerRuntimeWithAllOptions(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "full-container"
		implementations: [
			{
				script: "echo hello"
				target: {
					runtimes: [{
						name: "container"
						image: "golang:1.21"
						enable_host_ssh: true
						volumes: ["./data:/data", "/tmp:/tmp:ro"]
						ports: ["8080:80", "3000:3000"]
					}]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	rt := cmd.Implementations[0].Target.Runtimes[0]

	if rt.Name != RuntimeContainer {
		t.Errorf("Runtime name = %q, want %q", rt.Name, RuntimeContainer)
	}

	if rt.Image != "golang:1.21" {
		t.Errorf("Image = %q, want %q", rt.Image, "golang:1.21")
	}

	if !rt.EnableHostSSH {
		t.Error("EnableHostSSH should be true")
	}

	if len(rt.Volumes) != 2 {
		t.Errorf("Volumes length = %d, want 2", len(rt.Volumes))
	} else {
		if rt.Volumes[0] != "./data:/data" {
			t.Errorf("Volumes[0] = %q, want %q", rt.Volumes[0], "./data:/data")
		}
		if rt.Volumes[1] != "/tmp:/tmp:ro" {
			t.Errorf("Volumes[1] = %q, want %q", rt.Volumes[1], "/tmp:/tmp:ro")
		}
	}

	if len(rt.Ports) != 2 {
		t.Errorf("Ports length = %d, want 2", len(rt.Ports))
	} else {
		if rt.Ports[0] != "8080:80" {
			t.Errorf("Ports[0] = %q, want %q", rt.Ports[0], "8080:80")
		}
		if rt.Ports[1] != "3000:3000" {
			t.Errorf("Ports[1] = %q, want %q", rt.Ports[1], "3000:3000")
		}
	}
}

func TestParseDependsOn_WithCapabilities(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "rsync -avz ./dist/ user@server:/var/www/"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["local-area-network"]},
				{alternatives: ["internet"]},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d", len(cmd.DependsOn.Capabilities))
	}

	// First capability - local-area-network
	cap0 := cmd.DependsOn.Capabilities[0]
	if len(cap0.Alternatives) == 0 || cap0.Alternatives[0] != CapabilityLocalAreaNetwork {
		t.Errorf("First capability alternatives = %v, want [%s]", cap0.Alternatives, CapabilityLocalAreaNetwork)
	}

	// Second capability - internet
	cap1 := cmd.DependsOn.Capabilities[1]
	if len(cap1.Alternatives) == 0 || cap1.Alternatives[0] != CapabilityInternet {
		t.Errorf("Second capability alternatives = %v, want [%s]", cap1.Alternatives, CapabilityInternet)
	}
}

func TestParseDependsOn_CapabilitiesAtImplementationLevel(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "sync"
		implementations: [
			{
				script: "rsync -avz ./dist/ user@server:/var/www/"
				target: { runtimes: [{name: "native"}] }
				depends_on: {
					capabilities: [
						{alternatives: ["internet"]},
					]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Implementations) != 1 {
		t.Fatalf("Expected 1 implementation, got %d", len(cmd.Implementations))
	}

	impl := cmd.Implementations[0]
	if impl.DependsOn == nil {
		t.Fatal("Implementation DependsOn should not be nil")
	}

	if len(impl.DependsOn.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(impl.DependsOn.Capabilities))
	}

	if len(impl.DependsOn.Capabilities[0].Alternatives) == 0 || impl.DependsOn.Capabilities[0].Alternatives[0] != CapabilityInternet {
		t.Errorf("Capability alternatives = %v, want [%s]", impl.DependsOn.Capabilities[0].Alternatives, CapabilityInternet)
	}
}

func TestCommand_HasDependencies_WithCapabilities(t *testing.T) {
	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
		},
	}

	if !cmd.HasDependencies() {
		t.Error("HasDependencies() should return true when capabilities are present")
	}
}

func TestCommand_HasCommandLevelDependencies_WithCapabilities(t *testing.T) {
	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}}},
		},
	}

	if !cmd.HasCommandLevelDependencies() {
		t.Error("HasCommandLevelDependencies() should return true when capabilities are present")
	}
}

func TestScript_HasDependencies_WithCapabilities(t *testing.T) {
	impl := Implementation{
		Script: "echo test",
		Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
		},
	}

	if !impl.HasDependencies() {
		t.Error("Implementation.HasDependencies() should return true when capabilities are present")
	}
}

func TestMergeDependsOn_WithCapabilities(t *testing.T) {
	cmdDeps := &DependsOn{
		Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}}},
	}

	scriptDeps := &DependsOn{
		Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
	}

	merged := MergeDependsOn(cmdDeps, scriptDeps)

	if merged == nil {
		t.Fatal("MergeDependsOn should return non-nil result")
	}

	if len(merged.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities after merge, got %d", len(merged.Capabilities))
	}

	// Command-level capabilities should come first
	if len(merged.Capabilities[0].Alternatives) == 0 || merged.Capabilities[0].Alternatives[0] != CapabilityLocalAreaNetwork {
		t.Errorf("First capability alternatives = %v, want [%s]", merged.Capabilities[0].Alternatives, CapabilityLocalAreaNetwork)
	}

	if len(merged.Capabilities[1].Alternatives) == 0 || merged.Capabilities[1].Alternatives[0] != CapabilityInternet {
		t.Errorf("Second capability alternatives = %v, want [%s]", merged.Capabilities[1].Alternatives, CapabilityInternet)
	}
}

func TestGenerateCUE_WithCapabilities(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "deploy",
				Implementations: []Implementation{
					{
						Script: "rsync deploy",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
				DependsOn: &DependsOn{
					Capabilities: []CapabilityDependency{
						{Alternatives: []CapabilityName{CapabilityInternet}},
						{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Check that capabilities section is present
	if !strings.Contains(result, "capabilities:") {
		t.Error("GenerateCUE should include 'capabilities:' section")
	}

	if !strings.Contains(result, `"internet"`) {
		t.Error("GenerateCUE should include internet capability")
	}

	if !strings.Contains(result, `"local-area-network"`) {
		t.Error("GenerateCUE should include local-area-network capability")
	}
}

func TestGenerateCUE_WithCapabilitiesAtImplementationLevel(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "sync",
				Implementations: []Implementation{
					{
						Script: "rsync sync",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
						DependsOn: &DependsOn{
							Capabilities: []CapabilityDependency{
								{Alternatives: []CapabilityName{CapabilityInternet}},
							},
						},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Check that capabilities section is present at implementation level
	if !strings.Contains(result, "capabilities:") {
		t.Error("GenerateCUE should include 'capabilities:' section at implementation level")
	}

	if !strings.Contains(result, `"internet"`) {
		t.Error("GenerateCUE should include internet capability")
	}
}

// TestCUESchema_RejectsToolDependencyWithName verifies that the CUE schema rejects
// tool dependencies that use the old 'name' field instead of 'alternatives'
func TestCUESchema_RejectsToolDependencyWithName(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			tools: [
				{name: "git"},
			]
		}
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject tool dependency with 'name' field instead of 'alternatives'")
	}
	if !strings.Contains(err.Error(), "field not allowed") {
		t.Errorf("Error should mention 'field not allowed', got: %v", err)
	}
}

// TestCUESchema_RejectsCustomCheckWithBothNameAndAlternatives verifies that the CUE schema
// rejects custom checks that have both direct fields (name, check_script) AND alternatives
func TestCUESchema_RejectsCustomCheckWithBothNameAndAlternatives(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			custom_checks: [
				{
					name: "should-not-have-both"
					check_script: "echo test"
					alternatives: [
						{name: "alt1", check_script: "echo alt1"}
					]
				}
			]
		}
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject custom check with both direct fields and alternatives")
	}
	// The error could be about conflicting fields or disjunction not matching
	if !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "not allowed") {
		t.Logf("Warning: Error message may not be ideal, got: %v", err)
	}
}

// TestCUESchema_RejectsCapabilityDependencyWithName verifies that the CUE schema rejects
// capability dependencies that use the old 'name' field instead of 'alternatives'
func TestCUESchema_RejectsCapabilityDependencyWithName(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			capabilities: [
				{name: "internet"},
			]
		}
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject capability dependency with 'name' field instead of 'alternatives'")
	}
	if !strings.Contains(err.Error(), "field not allowed") {
		t.Errorf("Error should mention 'field not allowed', got: %v", err)
	}
}

// TestCUESchema_RejectsCommandDependencyWithName verifies that the CUE schema rejects
// command dependencies that use the old 'name' field instead of 'alternatives'
func TestCUESchema_RejectsCommandDependencyWithName(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		depends_on: {
			commands: [
				{name: "build"},
			]
		}
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject command dependency with 'name' field instead of 'alternatives'")
	}
	if !strings.Contains(err.Error(), "field not allowed") {
		t.Errorf("Error should mention 'field not allowed', got: %v", err)
	}
}

// Tests for the group field

func TestParseGroup_Valid(t *testing.T) {
	tests := []struct {
		name  string
		group string
	}{
		{"simple lowercase", "mygroup"},
		{"simple uppercase", "MyGroup"},
		{"with numbers", "group1"},
		{"dotted two parts", "my.group"},
		{"dotted three parts", "my.nested.group"},
		{"single letter", "a"},
		{"single letter with dotted", "a.b.c"},
		{"mixed case with dots", "My.Nested.Group1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "` + tt.group + `"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if inv.Group != tt.group {
				t.Errorf("Group = %q, want %q", inv.Group, tt.group)
			}
		})
	}
}

func TestParseGroup_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		group string
	}{
		{"starts with dot", ".group"},
		{"ends with dot", "group."},
		{"consecutive dots", "my..group"},
		{"starts with number", "1group"},
		{"contains hyphen", "my-group"},
		{"contains underscore", "my_group"},
		{"contains space", "my group"},
		{"empty string", ""},
		{"only dots", "..."},
		{"dot then number", "a.1b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "` + tt.group + `"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err = Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject invalid group %q", tt.group)
			}
		})
	}
}

func TestParseGroup_Missing(t *testing.T) {
	cueContent := `
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject invkfile without group field")
	}
}

func TestGetFullCommandName(t *testing.T) {
	inv := &Invkfile{
		Group: "my.group",
	}

	tests := []struct {
		name     string
		cmdName  string
		expected string
	}{
		{"simple command", "build", "my.group build"},
		{"subcommand with space", "test unit", "my.group test unit"},
		{"nested subcommand", "db migrate up", "my.group db migrate up"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inv.GetFullCommandName(tt.cmdName)
			if result != tt.expected {
				t.Errorf("GetFullCommandName(%q) = %q, want %q", tt.cmdName, result, tt.expected)
			}
		})
	}
}

func TestListCommands_WithGroup(t *testing.T) {
	inv := &Invkfile{
		Group: "mygroup",
		Commands: []Command{
			{Name: "build"},
			{Name: "test"},
			{Name: "deploy prod"},
		},
	}

	names := inv.ListCommands()

	expected := []string{"mygroup build", "mygroup test", "mygroup deploy prod"}
	if len(names) != len(expected) {
		t.Fatalf("ListCommands() returned %d names, want %d", len(names), len(expected))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ListCommands()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestFlattenCommands_WithGroup(t *testing.T) {
	inv := &Invkfile{
		Group: "mygroup",
		Commands: []Command{
			{Name: "build", Description: "Build command"},
			{Name: "test unit", Description: "Unit tests"},
		},
	}

	flat := inv.FlattenCommands()

	if len(flat) != 2 {
		t.Fatalf("FlattenCommands() returned %d commands, want 2", len(flat))
	}

	// Check that keys are prefixed with group
	if _, ok := flat["mygroup build"]; !ok {
		t.Error("FlattenCommands() should have key 'mygroup build'")
	}

	if _, ok := flat["mygroup test unit"]; !ok {
		t.Error("FlattenCommands() should have key 'mygroup test unit'")
	}

	// Check that commands are correct
	if flat["mygroup build"].Description != "Build command" {
		t.Errorf("flat['mygroup build'].Description = %q, want %q", flat["mygroup build"].Description, "Build command")
	}
}

func TestGenerateCUE_WithGroup(t *testing.T) {
	inv := &Invkfile{
		Group:   "my.group",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "test",
				Implementations: []Implementation{
					{Script: "echo test", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, `group: "my.group"`) {
		t.Error("GenerateCUE should contain 'group: \"my.group\"'")
	}

	// Group should appear before version
	groupIdx := strings.Index(output, "group:")
	versionIdx := strings.Index(output, "version:")
	if groupIdx > versionIdx {
		t.Error("GenerateCUE should output group before version")
	}
}

// ============================================================================
// Tests for Flags Feature
// ============================================================================

func TestParseFlags(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "env", description: "Target environment"},
			{name: "dry-run", description: "Perform a dry run without making changes", default_value: "false"},
			{name: "verbose", description: "Enable verbose output"},
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Flags) != 3 {
		t.Fatalf("Expected 3 flags, got %d", len(cmd.Flags))
	}

	// First flag - no default value
	flag0 := cmd.Flags[0]
	if flag0.Name != "env" {
		t.Errorf("Flag[0].Name = %q, want %q", flag0.Name, "env")
	}
	if flag0.Description != "Target environment" {
		t.Errorf("Flag[0].Description = %q, want %q", flag0.Description, "Target environment")
	}
	if flag0.DefaultValue != "" {
		t.Errorf("Flag[0].DefaultValue = %q, want empty string", flag0.DefaultValue)
	}

	// Second flag - with default value
	flag1 := cmd.Flags[1]
	if flag1.Name != "dry-run" {
		t.Errorf("Flag[1].Name = %q, want %q", flag1.Name, "dry-run")
	}
	if flag1.Description != "Perform a dry run without making changes" {
		t.Errorf("Flag[1].Description = %q, want %q", flag1.Description, "Perform a dry run without making changes")
	}
	if flag1.DefaultValue != "false" {
		t.Errorf("Flag[1].DefaultValue = %q, want %q", flag1.DefaultValue, "false")
	}

	// Third flag
	flag2 := cmd.Flags[2]
	if flag2.Name != "verbose" {
		t.Errorf("Flag[2].Name = %q, want %q", flag2.Name, "verbose")
	}
}

func TestParseFlagsValidation_InvalidName(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"starts with number", "1flag"},
		{"contains space", "my flag"},
		{"special characters", "flag@name"},
		{"empty name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "` + tt.flagName + `", description: "Test flag"},
		]
	}
]
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err = Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject flag with invalid name %q", tt.flagName)
			}
		})
	}
}

func TestParseFlagsValidation_ValidNames(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"simple lowercase", "verbose"},
		{"with hyphen", "dry-run"},
		{"with underscore", "output_file"},
		{"with numbers", "retry3"},
		{"mixed case", "outputFile"},
		{"uppercase start", "Verbose"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "` + tt.flagName + `", description: "Test flag"},
		]
	}
]
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Errorf("Parse() should accept flag with valid name %q, got error: %v", tt.flagName, err)
				return
			}

			if len(inv.Commands[0].Flags) != 1 {
				t.Errorf("Expected 1 flag, got %d", len(inv.Commands[0].Flags))
			}
		})
	}
}

func TestParseFlagsValidation_EmptyDescription(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "verbose", description: "   "},
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject flag with empty/whitespace-only description")
	}
}

func TestParseFlagsValidation_DuplicateNames(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output"},
			{name: "verbose", description: "Duplicate flag"},
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err = Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject duplicate flag names")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Error should mention 'duplicate', got: %v", err)
	}
}

func TestGenerateCUE_WithFlags(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "deploy",
				Implementations: []Implementation{
					{Script: "echo deploy", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}},
				},
				Flags: []Flag{
					{Name: "env", Description: "Target environment"},
					{Name: "dry-run", Description: "Perform dry run", DefaultValue: "false"},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, "flags:") {
		t.Error("GenerateCUE should contain 'flags:'")
	}

	if !strings.Contains(output, `name: "env"`) {
		t.Error("GenerateCUE should contain flag name 'env'")
	}

	if !strings.Contains(output, `description: "Target environment"`) {
		t.Error("GenerateCUE should contain flag description")
	}

	if !strings.Contains(output, `default_value: "false"`) {
		t.Error("GenerateCUE should contain default_value for flags that have one")
	}
}

func TestGenerateCUE_WithoutFlags(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "build",
				Implementations: []Implementation{
					{Script: "echo build", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if strings.Contains(output, "flags:") {
		t.Error("GenerateCUE should not contain 'flags:' when there are no flags")
	}
}

func TestParseFlags_EmptyList(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: []
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands[0].Flags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(inv.Commands[0].Flags))
	}
}

func TestParseFlags_NoFlagsField(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Commands[0].Flags != nil && len(inv.Commands[0].Flags) != 0 {
		t.Errorf("Expected nil or empty flags, got %v", inv.Commands[0].Flags)
	}
}

// ============================================================================
// Tests for Enhanced Flags Feature (type, required, short, validation)
// ============================================================================

func TestParseFlags_WithType(t *testing.T) {
	tests := []struct {
		name         string
		flagType     string
		defaultValue string
		wantType     FlagType
	}{
		{"string type explicit", "string", "hello", FlagTypeString},
		{"bool type with true", "bool", "true", FlagTypeBool},
		{"bool type with false", "bool", "false", FlagTypeBool},
		{"int type with positive", "int", "42", FlagTypeInt},
		{"int type with zero", "int", "0", FlagTypeInt},
		{"int type with negative", "int", "-10", FlagTypeInt},
		{"float type with positive", "float", "3.14", FlagTypeFloat},
		{"float type with negative", "float", "-2.5", FlagTypeFloat},
		{"float type with integer-like", "float", "10.0", FlagTypeFloat},
		{"float type with scientific notation", "float", "1.5e-3", FlagTypeFloat},
		{"float type with zero", "float", "0.0", FlagTypeFloat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := fmt.Sprintf(`
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.flagType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			flag := inv.Commands[0].Flags[0]
			if flag.GetType() != tt.wantType {
				t.Errorf("Flag.GetType() = %v, want %v", flag.GetType(), tt.wantType)
			}
			if flag.DefaultValue != tt.defaultValue {
				t.Errorf("Flag.DefaultValue = %v, want %v", flag.DefaultValue, tt.defaultValue)
			}
		})
	}
}

func TestParseFlags_TypeDefaultsToString(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.Type != "" {
		t.Errorf("Flag.Type should be empty (unset), got %q", flag.Type)
	}
	if flag.GetType() != FlagTypeString {
		t.Errorf("Flag.GetType() should default to 'string', got %v", flag.GetType())
	}
}

func TestParseFlagsValidation_InvalidType(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", type: "invalid"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject invalid type")
	}
}

func TestParseFlagsValidation_TypeIncompatibleWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		flagType     string
		defaultValue string
	}{
		{"bool with non-bool value", "bool", "yes"},
		{"bool with number", "bool", "1"},
		{"int with non-number", "int", "abc"},
		{"int with float", "int", "3.14"},
		{"float with non-number", "float", "abc"},
		{"float with multiple dots", "float", "3.14.15"},
		{"float with invalid suffix", "float", "3.14abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := fmt.Sprintf(`
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.flagType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject flag with type %q and incompatible default_value %q", tt.flagType, tt.defaultValue)
			}
		})
	}
}

func TestParseFlags_RequiredFlag(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", required: true},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if !flag.Required {
		t.Errorf("Flag.Required = false, want true")
	}
}

func TestParseFlagsValidation_RequiredWithDefaultValue(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", required: true, default_value: "foo"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject flag that is both required and has default_value")
	}
	if err != nil && !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "default_value") {
		t.Errorf("Error message should mention required and default_value conflict, got: %v", err)
	}
}

func TestParseFlags_ShortAlias(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", short: "v"},
			{name: "quiet", description: "Quiet mode", short: "q"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flags := inv.Commands[0].Flags
	if flags[0].Short != "v" {
		t.Errorf("Flag[0].Short = %q, want %q", flags[0].Short, "v")
	}
	if flags[1].Short != "q" {
		t.Errorf("Flag[1].Short = %q, want %q", flags[1].Short, "q")
	}
}

func TestParseFlagsValidation_InvalidShortAlias(t *testing.T) {
	tests := []struct {
		name  string
		short string
	}{
		{"multiple chars", "ab"},
		{"digit", "1"},
		{"special char", "-"},
		{"empty string is valid", ""}, // Should NOT cause error - empty means no short alias
	}

	for _, tt := range tests {
		if tt.short == "" {
			continue // Skip empty - it's valid
		}
		t.Run(tt.name, func(t *testing.T) {
			cueContent := fmt.Sprintf(`
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", short: "%s"},
		]
	}
]
`, tt.short)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject invalid short alias %q", tt.short)
			}
		})
	}
}

func TestParseFlagsValidation_DuplicateShortAlias(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", short: "v"},
			{name: "version", description: "Show version", short: "v"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject duplicate short alias")
	}
	if err != nil && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "short") {
		t.Errorf("Error message should mention duplicate short alias, got: %v", err)
	}
}

func TestParseFlags_ValidationRegex(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Flag.Validation = %q, want %q", flag.Validation, "^(dev|staging|prod)$")
	}
}

func TestParseFlagsValidation_InvalidRegex(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", validation: "[invalid(regex"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject invalid validation regex")
	}
}

func TestParseFlagsValidation_DefaultNotMatchingValidation(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "invalid"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject default_value that doesn't match validation pattern")
	}
}

func TestParseFlags_DefaultMatchesValidation(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "prod"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() should accept default_value that matches validation, got error: %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.DefaultValue != "prod" {
		t.Errorf("Flag.DefaultValue = %q, want %q", flag.DefaultValue, "prod")
	}
}

func TestValidateFlagValue(t *testing.T) {
	tests := []struct {
		name       string
		flag       Flag
		value      string
		wantErr    bool
		errContain string
	}{
		{
			name:    "string type accepts any value",
			flag:    Flag{Name: "test", Type: FlagTypeString},
			value:   "hello world",
			wantErr: false,
		},
		{
			name:    "bool type accepts true",
			flag:    Flag{Name: "test", Type: FlagTypeBool},
			value:   "true",
			wantErr: false,
		},
		{
			name:    "bool type accepts false",
			flag:    Flag{Name: "test", Type: FlagTypeBool},
			value:   "false",
			wantErr: false,
		},
		{
			name:       "bool type rejects invalid",
			flag:       Flag{Name: "test", Type: FlagTypeBool},
			value:      "yes",
			wantErr:    true,
			errContain: "must be 'true' or 'false'",
		},
		{
			name:    "int type accepts positive",
			flag:    Flag{Name: "test", Type: FlagTypeInt},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "int type accepts zero",
			flag:    Flag{Name: "test", Type: FlagTypeInt},
			value:   "0",
			wantErr: false,
		},
		{
			name:    "int type accepts negative",
			flag:    Flag{Name: "test", Type: FlagTypeInt},
			value:   "-10",
			wantErr: false,
		},
		{
			name:       "int type rejects non-integer",
			flag:       Flag{Name: "test", Type: FlagTypeInt},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:       "int type rejects float",
			flag:       Flag{Name: "test", Type: FlagTypeInt},
			value:      "3.14",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:    "float type accepts positive",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "3.14",
			wantErr: false,
		},
		{
			name:    "float type accepts negative",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "-2.5",
			wantErr: false,
		},
		{
			name:    "float type accepts zero",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "0.0",
			wantErr: false,
		},
		{
			name:    "float type accepts integer-like",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "float type accepts scientific notation",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "1.5e-3",
			wantErr: false,
		},
		{
			name:       "float type rejects non-number",
			flag:       Flag{Name: "test", Type: FlagTypeFloat},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid floating-point number",
		},
		{
			name:       "float type rejects multiple dots",
			flag:       Flag{Name: "test", Type: FlagTypeFloat},
			value:      "3.14.15",
			wantErr:    true,
			errContain: "must be a valid floating-point number",
		},
		{
			name:    "validation regex passes",
			flag:    Flag{Name: "env", Type: FlagTypeString, Validation: "^(dev|staging|prod)$"},
			value:   "prod",
			wantErr: false,
		},
		{
			name:       "validation regex fails",
			flag:       Flag{Name: "env", Type: FlagTypeString, Validation: "^(dev|staging|prod)$"},
			value:      "invalid",
			wantErr:    true,
			errContain: "does not match required pattern",
		},
		{
			name:    "empty type defaults to string",
			flag:    Flag{Name: "test"},
			value:   "anything",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.flag.ValidateFlagValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateFlagValue() should return error")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateFlagValue() error = %v, should contain %q", err, tt.errContain)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFlagValue() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseFlags_AllEnhancedFeatures(t *testing.T) {
	// Test a flag with all enhanced features together
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{
				name: "environment"
				description: "Target environment"
				type: "string"
				required: true
				short: "t"
				validation: "^(dev|staging|prod)$"
			},
			{
				name: "dry-run"
				description: "Perform a dry run"
				type: "bool"
				default_value: "false"
				short: "d"
			},
			{
				name: "replicas"
				description: "Number of replicas"
				type: "int"
				default_value: "3"
				short: "r"
			},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flags := inv.Commands[0].Flags
	if len(flags) != 3 {
		t.Fatalf("Expected 3 flags, got %d", len(flags))
	}

	// Check environment flag
	envFlag := flags[0]
	if envFlag.Name != "environment" {
		t.Errorf("Flag[0].Name = %q, want %q", envFlag.Name, "environment")
	}
	if envFlag.GetType() != FlagTypeString {
		t.Errorf("Flag[0].GetType() = %v, want %v", envFlag.GetType(), FlagTypeString)
	}
	if !envFlag.Required {
		t.Errorf("Flag[0].Required = false, want true")
	}
	if envFlag.Short != "t" {
		t.Errorf("Flag[0].Short = %q, want %q", envFlag.Short, "t")
	}
	if envFlag.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Flag[0].Validation = %q, want %q", envFlag.Validation, "^(dev|staging|prod)$")
	}

	// Check dry-run flag
	dryRunFlag := flags[1]
	if dryRunFlag.GetType() != FlagTypeBool {
		t.Errorf("Flag[1].GetType() = %v, want %v", dryRunFlag.GetType(), FlagTypeBool)
	}
	if dryRunFlag.DefaultValue != "false" {
		t.Errorf("Flag[1].DefaultValue = %q, want %q", dryRunFlag.DefaultValue, "false")
	}
	if dryRunFlag.Short != "d" {
		t.Errorf("Flag[1].Short = %q, want %q", dryRunFlag.Short, "d")
	}

	// Check replicas flag
	replicasFlag := flags[2]
	if replicasFlag.GetType() != FlagTypeInt {
		t.Errorf("Flag[2].GetType() = %v, want %v", replicasFlag.GetType(), FlagTypeInt)
	}
	if replicasFlag.DefaultValue != "3" {
		t.Errorf("Flag[2].DefaultValue = %q, want %q", replicasFlag.DefaultValue, "3")
	}
}

// ============================================================================
// Tests for Positional Arguments Feature
// ============================================================================

func TestParseArgs_Basic(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "greet"
		description: "Greet a person"
		implementations: [
			{
				script: "echo Hello $INVOWK_ARG_NAME"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "name", description: "The name to greet", required: true},
			{name: "greeting", description: "The greeting to use", default_value: "Hello"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(cmd.Args))
	}

	// First arg - required
	arg0 := cmd.Args[0]
	if arg0.Name != "name" {
		t.Errorf("Arg[0].Name = %q, want %q", arg0.Name, "name")
	}
	if arg0.Description != "The name to greet" {
		t.Errorf("Arg[0].Description = %q, want %q", arg0.Description, "The name to greet")
	}
	if !arg0.Required {
		t.Error("Arg[0].Required = false, want true")
	}

	// Second arg - optional with default
	arg1 := cmd.Args[1]
	if arg1.Name != "greeting" {
		t.Errorf("Arg[1].Name = %q, want %q", arg1.Name, "greeting")
	}
	if arg1.Required {
		t.Error("Arg[1].Required = true, want false")
	}
	if arg1.DefaultValue != "Hello" {
		t.Errorf("Arg[1].DefaultValue = %q, want %q", arg1.DefaultValue, "Hello")
	}
}

func TestParseArgs_WithTypes(t *testing.T) {
	tests := []struct {
		name         string
		argType      string
		defaultValue string
		wantType     ArgumentType
	}{
		{"string type explicit", "string", "hello", ArgumentTypeString},
		{"int type with positive", "int", "42", ArgumentTypeInt},
		{"int type with zero", "int", "0", ArgumentTypeInt},
		{"int type with negative", "int", "-10", ArgumentTypeInt},
		{"float type with positive", "float", "3.14", ArgumentTypeFloat},
		{"float type with negative", "float", "-2.5", ArgumentTypeFloat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := fmt.Sprintf(`
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "Test arg", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.argType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			arg := inv.Commands[0].Args[0]
			if arg.GetType() != tt.wantType {
				t.Errorf("Arg.GetType() = %v, want %v", arg.GetType(), tt.wantType)
			}
			if arg.DefaultValue != tt.defaultValue {
				t.Errorf("Arg.DefaultValue = %v, want %v", arg.DefaultValue, tt.defaultValue)
			}
		})
	}
}

func TestParseArgs_TypeDefaultsToString(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "Test arg"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	arg := inv.Commands[0].Args[0]
	if arg.Type != "" {
		t.Errorf("Arg.Type should be empty (unset), got %q", arg.Type)
	}
	if arg.GetType() != ArgumentTypeString {
		t.Errorf("Arg.GetType() should default to 'string', got %v", arg.GetType())
	}
}

func TestParseArgs_Variadic(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "copy"
		description: "Copy files to a destination"
		implementations: [
			{
				script: "cp $INVOWK_ARG_FILES $INVOWK_ARG_DEST"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "dest", description: "Destination directory", required: true},
			{name: "files", description: "Source files", required: true, variadic: true},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	args := inv.Commands[0].Args
	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}

	// First arg - non-variadic
	if args[0].Variadic {
		t.Error("Arg[0].Variadic = true, want false")
	}

	// Second arg - variadic
	if !args[1].Variadic {
		t.Error("Arg[1].Variadic = false, want true")
	}
}

func TestParseArgs_ValidationRegex(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying to $INVOWK_ARG_ENV"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "env", description: "Target environment", required: true, validation: "^(dev|staging|prod)$"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	arg := inv.Commands[0].Args[0]
	if arg.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Arg.Validation = %q, want %q", arg.Validation, "^(dev|staging|prod)$")
	}
}

func TestParseArgsValidation_InvalidName(t *testing.T) {
	tests := []struct {
		name    string
		argName string
	}{
		{"starts with number", "1arg"},
		{"contains space", "my arg"},
		{"special characters", "arg@name"},
		{"empty name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "` + tt.argName + `", description: "Test arg"},
		]
	}
]
`
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject arg with invalid name %q", tt.argName)
			}
		})
	}
}

func TestParseArgsValidation_ValidNames(t *testing.T) {
	tests := []struct {
		name    string
		argName string
	}{
		{"simple lowercase", "name"},
		{"with hyphen", "output-file"},
		{"with underscore", "output_file"},
		{"with numbers", "file1"},
		{"mixed case", "outputFile"},
		{"uppercase start", "Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "` + tt.argName + `", description: "Test arg"},
		]
	}
]
`
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Errorf("Parse() should accept arg with valid name %q, got error: %v", tt.argName, err)
				return
			}

			if len(inv.Commands[0].Args) != 1 {
				t.Errorf("Expected 1 arg, got %d", len(inv.Commands[0].Args))
			}
		})
	}
}

func TestParseArgsValidation_EmptyDescription(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "   "},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject arg with empty/whitespace-only description")
	}
}

func TestParseArgsValidation_DuplicateNames(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "name", description: "First argument"},
			{name: "name", description: "Duplicate argument"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject duplicate arg names")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Error should mention 'duplicate', got: %v", err)
	}
}

func TestParseArgsValidation_RequiredAfterOptional(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "optional", description: "Optional arg"},
			{name: "required", description: "Required arg", required: true},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject required arg after optional arg")
	}
	if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "optional") {
		t.Errorf("Error should mention required/optional ordering, got: %v", err)
	}
}

func TestParseArgsValidation_VariadicNotLast(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "files", description: "Source files", required: true, variadic: true},
			{name: "dest", description: "Destination", required: true},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject variadic arg that is not last")
	}
	if !strings.Contains(err.Error(), "variadic") {
		t.Errorf("Error should mention variadic constraint, got: %v", err)
	}
}

func TestParseArgsValidation_RequiredWithDefaultValue(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "Test arg", required: true, default_value: "foo"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject arg that is both required and has default_value")
	}
	if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "default_value") {
		t.Errorf("Error should mention required/default_value conflict, got: %v", err)
	}
}

func TestParseArgsValidation_InvalidType(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "Test arg", type: "invalid"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject invalid arg type")
	}
}

func TestParseArgsValidation_TypeIncompatibleWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		argType      string
		defaultValue string
	}{
		{"int with non-number", "int", "abc"},
		{"int with float", "int", "3.14"},
		{"float with non-number", "float", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := fmt.Sprintf(`
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "Test arg", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.argType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject arg with type %q and incompatible default_value %q", tt.argType, tt.defaultValue)
			}
		})
	}
}

func TestParseArgsValidation_InvalidRegex(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "myarg", description: "Test arg", validation: "[invalid(regex"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject invalid validation regex")
	}
}

func TestParseArgsValidation_DefaultNotMatchingValidation(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "invalid"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject default_value that doesn't match validation pattern")
	}
}

func TestValidateArgumentValue(t *testing.T) {
	tests := []struct {
		name       string
		arg        Argument
		value      string
		wantErr    bool
		errContain string
	}{
		{
			name:    "string type accepts any value",
			arg:     Argument{Name: "test", Type: ArgumentTypeString},
			value:   "hello world",
			wantErr: false,
		},
		{
			name:    "int type accepts positive",
			arg:     Argument{Name: "test", Type: ArgumentTypeInt},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "int type accepts zero",
			arg:     Argument{Name: "test", Type: ArgumentTypeInt},
			value:   "0",
			wantErr: false,
		},
		{
			name:    "int type accepts negative",
			arg:     Argument{Name: "test", Type: ArgumentTypeInt},
			value:   "-10",
			wantErr: false,
		},
		{
			name:       "int type rejects non-integer",
			arg:        Argument{Name: "test", Type: ArgumentTypeInt},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:       "int type rejects float",
			arg:        Argument{Name: "test", Type: ArgumentTypeInt},
			value:      "3.14",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:    "float type accepts positive",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "3.14",
			wantErr: false,
		},
		{
			name:    "float type accepts negative",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "-2.5",
			wantErr: false,
		},
		{
			name:    "float type accepts zero",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "0.0",
			wantErr: false,
		},
		{
			name:    "float type accepts integer-like",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "42",
			wantErr: false,
		},
		{
			name:       "float type rejects non-number",
			arg:        Argument{Name: "test", Type: ArgumentTypeFloat},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid floating-point number",
		},
		{
			name:    "validation regex passes",
			arg:     Argument{Name: "env", Type: ArgumentTypeString, Validation: "^(dev|staging|prod)$"},
			value:   "prod",
			wantErr: false,
		},
		{
			name:       "validation regex fails",
			arg:        Argument{Name: "env", Type: ArgumentTypeString, Validation: "^(dev|staging|prod)$"},
			value:      "invalid",
			wantErr:    true,
			errContain: "does not match required pattern",
		},
		{
			name:    "empty type defaults to string",
			arg:     Argument{Name: "test"},
			value:   "anything",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.arg.ValidateArgumentValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateArgumentValue() should return error")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateArgumentValue() error = %v, should contain %q", err, tt.errContain)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateArgumentValue() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseArgs_EmptyList(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: []
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands[0].Args) != 0 {
		t.Errorf("Expected 0 args, got %d", len(inv.Commands[0].Args))
	}
}

func TestParseArgs_NoArgsField(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Commands[0].Args != nil && len(inv.Commands[0].Args) != 0 {
		t.Errorf("Expected nil or empty args, got %v", inv.Commands[0].Args)
	}
}

func TestParseArgs_AllFeatures(t *testing.T) {
	// Test an arg with all features together
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying to $INVOWK_ARG_ENV with $INVOWK_ARG_REPLICAS replicas"
				target: { runtimes: [{name: "native"}] }
			}
		]
		args: [
			{
				name: "env"
				description: "Target environment"
				type: "string"
				required: true
				validation: "^(dev|staging|prod)$"
			},
			{
				name: "replicas"
				description: "Number of replicas"
				type: "int"
				default_value: "3"
			},
			{
				name: "extra-args"
				description: "Extra arguments to pass"
				variadic: true
			},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	args := inv.Commands[0].Args
	if len(args) != 3 {
		t.Fatalf("Expected 3 args, got %d", len(args))
	}

	// Check env arg
	envArg := args[0]
	if envArg.Name != "env" {
		t.Errorf("Arg[0].Name = %q, want %q", envArg.Name, "env")
	}
	if envArg.GetType() != ArgumentTypeString {
		t.Errorf("Arg[0].GetType() = %v, want %v", envArg.GetType(), ArgumentTypeString)
	}
	if !envArg.Required {
		t.Error("Arg[0].Required = false, want true")
	}
	if envArg.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Arg[0].Validation = %q, want %q", envArg.Validation, "^(dev|staging|prod)$")
	}

	// Check replicas arg
	replicasArg := args[1]
	if replicasArg.GetType() != ArgumentTypeInt {
		t.Errorf("Arg[1].GetType() = %v, want %v", replicasArg.GetType(), ArgumentTypeInt)
	}
	if replicasArg.DefaultValue != "3" {
		t.Errorf("Arg[1].DefaultValue = %q, want %q", replicasArg.DefaultValue, "3")
	}
	if replicasArg.Required {
		t.Error("Arg[1].Required = true, want false")
	}

	// Check extra-args arg
	extraArg := args[2]
	if !extraArg.Variadic {
		t.Error("Arg[2].Variadic = false, want true")
	}
}

func TestGenerateCUE_WithArgs(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy to environment",
				Implementations: []Implementation{
					{
						Script: "echo deploying",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
				Args: []Argument{
					{
						Name:        "env",
						Description: "Target environment",
						Required:    true,
						Validation:  "^(dev|staging|prod)$",
					},
					{
						Name:         "replicas",
						Description:  "Number of replicas",
						Type:         ArgumentTypeInt,
						DefaultValue: "3",
					},
					{
						Name:        "services",
						Description: "Services to deploy",
						Variadic:    true,
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check args section exists
	if !strings.Contains(output, "args: [") {
		t.Error("GenerateCUE should contain 'args: ['")
	}

	// Check required arg
	if !strings.Contains(output, `name: "env"`) {
		t.Error("GenerateCUE should contain arg name 'env'")
	}
	if !strings.Contains(output, `description: "Target environment"`) {
		t.Error("GenerateCUE should contain arg description")
	}
	if !strings.Contains(output, "required: true") {
		t.Error("GenerateCUE should contain required: true for required arg")
	}
	if !strings.Contains(output, `validation: "^(dev|staging|prod)$"`) {
		t.Error("GenerateCUE should contain validation pattern")
	}

	// Check typed arg with default
	if !strings.Contains(output, `name: "replicas"`) {
		t.Error("GenerateCUE should contain arg name 'replicas'")
	}
	if !strings.Contains(output, `type: "int"`) {
		t.Error("GenerateCUE should contain type: int")
	}
	if !strings.Contains(output, `default_value: "3"`) {
		t.Error("GenerateCUE should contain default_value: 3")
	}

	// Check variadic arg
	if !strings.Contains(output, `name: "services"`) {
		t.Error("GenerateCUE should contain arg name 'services'")
	}
	if !strings.Contains(output, "variadic: true") {
		t.Error("GenerateCUE should contain variadic: true")
	}
}

func TestGenerateCUE_WithArgs_StringTypeNotIncluded(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name:        "greet",
				Description: "Greet someone",
				Implementations: []Implementation{
					{
						Script: "echo hello",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
				Args: []Argument{
					{
						Name:        "name",
						Description: "Name to greet",
						Type:        ArgumentTypeString, // Default type
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// String type should NOT be explicitly included (it's the default)
	if strings.Contains(output, `type: "string"`) {
		t.Error("GenerateCUE should NOT include type: string (it's the default)")
	}

	// But the arg should still be there
	if !strings.Contains(output, `name: "name"`) {
		t.Error("GenerateCUE should contain the arg")
	}
}

func TestGenerateCUE_WithArgs_RoundTrip(t *testing.T) {
	// Create an invkfile with args, generate CUE, parse it back, and verify
	original := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
				Args: []Argument{
					{
						Name:        "env",
						Description: "Target environment",
						Required:    true,
					},
					{
						Name:         "count",
						Description:  "Replica count",
						Type:         ArgumentTypeInt,
						DefaultValue: "1",
					},
					{
						Name:        "extras",
						Description: "Extra params",
						Variadic:    true,
					},
				},
			},
		},
	}

	// Generate CUE
	cueContent := GenerateCUE(original)

	// Write to temp file and parse back
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	// Verify parsed args match original
	if len(parsed.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(parsed.Commands))
	}

	args := parsed.Commands[0].Args
	if len(args) != 3 {
		t.Fatalf("Expected 3 args, got %d", len(args))
	}

	// Check first arg
	if args[0].Name != "env" {
		t.Errorf("Args[0].Name = %q, want %q", args[0].Name, "env")
	}
	if !args[0].Required {
		t.Error("Args[0].Required should be true")
	}

	// Check second arg
	if args[1].Name != "count" {
		t.Errorf("Args[1].Name = %q, want %q", args[1].Name, "count")
	}
	if args[1].GetType() != ArgumentTypeInt {
		t.Errorf("Args[1].Type = %q, want %q", args[1].GetType(), ArgumentTypeInt)
	}
	if args[1].DefaultValue != "1" {
		t.Errorf("Args[1].DefaultValue = %q, want %q", args[1].DefaultValue, "1")
	}

	// Check third arg
	if args[2].Name != "extras" {
		t.Errorf("Args[2].Name = %q, want %q", args[2].Name, "extras")
	}
	if !args[2].Variadic {
		t.Error("Args[2].Variadic should be true")
	}
}

// ============================================================================
// Tests for Interpreter Validation (empty/whitespace rejection)
// ============================================================================

// TestCUESchema_RejectsEmptyInterpreter verifies that the CUE schema rejects
// empty interpreter values when the field is explicitly declared.
func TestCUESchema_RejectsEmptyInterpreter(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native", interpreter: ""}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject empty interpreter when explicitly declared")
	}
}

// TestCUESchema_RejectsWhitespaceOnlyInterpreter verifies that the CUE schema rejects
// whitespace-only interpreter values when the field is explicitly declared.
func TestCUESchema_RejectsWhitespaceOnlyInterpreter(t *testing.T) {
	tests := []struct {
		name        string
		interpreter string
	}{
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tab", "\t"},
		{"mixed whitespace", "  \t  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native", interpreter: "` + tt.interpreter + `"}] }
			}
		]
	}
]
`
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject whitespace-only interpreter %q", tt.interpreter)
			}
		})
	}
}

// TestCUESchema_RejectsEmptyInterpreterForContainer verifies that the CUE schema
// rejects empty interpreter for container runtime as well.
func TestCUESchema_RejectsEmptyInterpreterForContainer(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "container", image: "alpine:latest", interpreter: ""}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Error("Parse() should reject empty interpreter for container runtime")
	}
}

// TestValidateRuntimeConfig_RejectsEmptyInterpreter tests the Go-level validation
// as a fallback for empty/whitespace interpreter rejection.
func TestValidateRuntimeConfig_RejectsEmptyInterpreter(t *testing.T) {
	tests := []struct {
		name        string
		interpreter string
		wantErr     bool
	}{
		{"empty string is empty so no validation triggered", "", false}, // Empty means field was not declared
		{"whitespace only - space", " ", true},
		{"whitespace only - tabs", "\t\t", true},
		{"whitespace only - mixed", "  \t  ", true},
		{"valid interpreter - auto", "auto", false},
		{"valid interpreter - python3", "python3", false},
		{"valid interpreter - with leading space", " python3", false}, // Has non-whitespace content
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &RuntimeConfig{
				Name:        RuntimeNative,
				Interpreter: tt.interpreter,
			}

			err := validateRuntimeConfig(rt, "test-cmd", 1)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateRuntimeConfig() should return error for interpreter %q", tt.interpreter)
				}
				if err != nil && !strings.Contains(err.Error(), "interpreter") {
					t.Errorf("Error should mention 'interpreter', got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("validateRuntimeConfig() unexpected error for interpreter %q: %v", tt.interpreter, err)
				}
			}
		})
	}
}

// TestParseInterpreter_ValidValues verifies that valid interpreter values work correctly.
func TestParseInterpreter_ValidValues(t *testing.T) {
	tests := []struct {
		name        string
		interpreter string
	}{
		{"auto", "auto"},
		{"simple command", "python3"},
		{"with path", "/usr/bin/python3"},
		{"with args", "python3 -u"},
		{"shebang-style", "/usr/bin/env python3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "print('hello')"
				target: { runtimes: [{name: "native", interpreter: "` + tt.interpreter + `"}] }
			}
		]
	}
]
`
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Fatalf("Parse() should accept valid interpreter %q, got error: %v", tt.interpreter, err)
			}

			rt := inv.Commands[0].Implementations[0].Target.Runtimes[0]
			if rt.Interpreter != tt.interpreter {
				t.Errorf("RuntimeConfig.Interpreter = %q, want %q", rt.Interpreter, tt.interpreter)
			}
		})
	}
}

// TestParseInterpreter_OmittedFieldIsValid verifies that omitting the interpreter
// field entirely is valid (defaults to auto-detection).
func TestParseInterpreter_OmittedFieldIsValid(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo hello"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() should accept omitted interpreter field, got error: %v", err)
	}

	rt := inv.Commands[0].Implementations[0].Target.Runtimes[0]
	if rt.Interpreter != "" {
		t.Errorf("RuntimeConfig.Interpreter should be empty when omitted, got %q", rt.Interpreter)
	}
}

// ============================================================================
// Tests for Env Feature (env.files and env.vars)
// ============================================================================

func TestParseEnv_CommandLevelFiles(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		env: {
			files: [".env", "config/app.env", ".env.local?"]
		}
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.Env == nil {
		t.Fatalf("Expected cmd.Env to be non-nil")
	}
	if len(cmd.Env.Files) != 3 {
		t.Fatalf("Expected 3 env.files, got %d", len(cmd.Env.Files))
	}

	expectedFiles := []string{".env", "config/app.env", ".env.local?"}
	for i, expected := range expectedFiles {
		if cmd.Env.Files[i] != expected {
			t.Errorf("Env.Files[%d] = %q, want %q", i, cmd.Env.Files[i], expected)
		}
	}
}

func TestParseEnv_ImplementationLevelFiles(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
				env: {
					files: ["impl.env", "secrets.env?"]
				}
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	impl := inv.Commands[0].Implementations[0]
	if impl.Env == nil {
		t.Fatalf("Expected impl.Env to be non-nil")
	}
	if len(impl.Env.Files) != 2 {
		t.Fatalf("Expected 2 env.files, got %d", len(impl.Env.Files))
	}

	expectedFiles := []string{"impl.env", "secrets.env?"}
	for i, expected := range expectedFiles {
		if impl.Env.Files[i] != expected {
			t.Errorf("Env.Files[%d] = %q, want %q", i, impl.Env.Files[i], expected)
		}
	}
}

func TestParseEnv_BothLevels(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
				env: {
					files: ["impl.env"]
				}
			}
		]
		env: {
			files: ["cmd.env"]
		}
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	impl := cmd.Implementations[0]

	if cmd.Env == nil || len(cmd.Env.Files) != 1 || cmd.Env.Files[0] != "cmd.env" {
		t.Errorf("Command Env.Files = %v, want [cmd.env]", cmd.Env.GetFiles())
	}

	if impl.Env == nil || len(impl.Env.Files) != 1 || impl.Env.Files[0] != "impl.env" {
		t.Errorf("Implementation Env.Files = %v, want [impl.env]", impl.Env.GetFiles())
	}
}

func TestParseEnv_EmptyFiles(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		env: {
			files: []
		}
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.Env != nil && len(cmd.Env.Files) != 0 {
		t.Errorf("Expected 0 env.files, got %d", len(cmd.Env.Files))
	}
}

func TestParseEnv_NoField(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.Env != nil {
		t.Errorf("Expected nil Env when field is omitted, got %+v", cmd.Env)
	}

	impl := cmd.Implementations[0]
	if impl.Env != nil {
		t.Errorf("Expected nil Env when field is omitted, got %+v", impl.Env)
	}
}

func TestParseEnv_WithVars(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
				env: {
					vars: {
						IMPL_VAR: "impl_value"
					}
				}
			}
		]
		env: {
			files: [".env"]
			vars: {
				CMD_VAR: "cmd_value"
				DEBUG: "true"
			}
		}
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.Env == nil {
		t.Fatalf("Expected cmd.Env to be non-nil")
	}
	if len(cmd.Env.Files) != 1 || cmd.Env.Files[0] != ".env" {
		t.Errorf("Command Env.Files = %v, want [\".env\"]", cmd.Env.Files)
	}
	if len(cmd.Env.Vars) != 2 {
		t.Fatalf("Expected 2 command env vars, got %d", len(cmd.Env.Vars))
	}
	if cmd.Env.Vars["CMD_VAR"] != "cmd_value" {
		t.Errorf("CMD_VAR = %q, want %q", cmd.Env.Vars["CMD_VAR"], "cmd_value")
	}
	if cmd.Env.Vars["DEBUG"] != "true" {
		t.Errorf("DEBUG = %q, want %q", cmd.Env.Vars["DEBUG"], "true")
	}

	impl := cmd.Implementations[0]
	if impl.Env == nil {
		t.Fatalf("Expected impl.Env to be non-nil")
	}
	if len(impl.Env.Vars) != 1 {
		t.Fatalf("Expected 1 implementation env var, got %d", len(impl.Env.Vars))
	}
	if impl.Env.Vars["IMPL_VAR"] != "impl_value" {
		t.Errorf("IMPL_VAR = %q, want %q", impl.Env.Vars["IMPL_VAR"], "impl_value")
	}
}

func TestGenerateCUE_WithEnv(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
						Env: &EnvConfig{
							Files: []string{"impl.env", "secrets.env?"},
						},
					},
				},
				Env: &EnvConfig{
					Files: []string{".env", "config/app.env"},
				},
			},
		},
	}

	cue := GenerateCUE(inv)

	// Check command-level env with files
	if !strings.Contains(cue, `env: {`) {
		t.Errorf("GenerateCUE() should include env block, got:\n%s", cue)
	}
	if !strings.Contains(cue, `files: [".env", "config/app.env"]`) {
		t.Errorf("GenerateCUE() should include command-level env.files, got:\n%s", cue)
	}

	// Check implementation-level env with files
	if !strings.Contains(cue, `files: ["impl.env", "secrets.env?"]`) {
		t.Errorf("GenerateCUE() should include implementation-level env.files, got:\n%s", cue)
	}
}

func TestGenerateCUE_EnvRoundTrip(t *testing.T) {
	original := &Invkfile{
		Group:   "test.roundtrip",
		Version: "1.0",
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
						Env: &EnvConfig{
							Files: []string{"impl.env"},
						},
					},
				},
				Env: &EnvConfig{
					Files: []string{".env", "config/app.env?"},
				},
			},
		},
	}

	// Generate CUE
	cue := GenerateCUE(original)

	// Write to temp file
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cue), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	// Parse it back
	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify command-level env.files
	if parsed.Commands[0].Env == nil {
		t.Fatalf("Expected non-nil Env after roundtrip")
	}
	if len(parsed.Commands[0].Env.Files) != 2 {
		t.Fatalf("Expected 2 command env.files, got %d", len(parsed.Commands[0].Env.Files))
	}
	if parsed.Commands[0].Env.Files[0] != ".env" {
		t.Errorf("Command Env.Files[0] = %q, want %q", parsed.Commands[0].Env.Files[0], ".env")
	}
	if parsed.Commands[0].Env.Files[1] != "config/app.env?" {
		t.Errorf("Command Env.Files[1] = %q, want %q", parsed.Commands[0].Env.Files[1], "config/app.env?")
	}

	// Verify implementation-level env.files
	if parsed.Commands[0].Implementations[0].Env == nil {
		t.Fatalf("Expected non-nil impl.Env after roundtrip")
	}
	if len(parsed.Commands[0].Implementations[0].Env.Files) != 1 {
		t.Fatalf("Expected 1 implementation env.files, got %d", len(parsed.Commands[0].Implementations[0].Env.Files))
	}
	if parsed.Commands[0].Implementations[0].Env.Files[0] != "impl.env" {
		t.Errorf("Implementation Env.Files[0] = %q, want %q", parsed.Commands[0].Implementations[0].Env.Files[0], "impl.env")
	}
}

func TestParseEnv_RootLevelFiles(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

env: {
	files: ["global.env", "shared.env?"]
}

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Env == nil {
		t.Fatalf("Expected inv.Env to be non-nil")
	}
	if len(inv.Env.Files) != 2 {
		t.Fatalf("Expected 2 env.files, got %d", len(inv.Env.Files))
	}

	expectedFiles := []string{"global.env", "shared.env?"}
	for i, expected := range expectedFiles {
		if inv.Env.Files[i] != expected {
			t.Errorf("Env.Files[%d] = %q, want %q", i, inv.Env.Files[i], expected)
		}
	}
}

func TestParseEnv_RootLevelVars(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

env: {
	vars: {
		GLOBAL_VAR: "global_value"
		APP_ENV: "production"
	}
}

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Env == nil {
		t.Fatalf("Expected inv.Env to be non-nil")
	}
	if len(inv.Env.Vars) != 2 {
		t.Fatalf("Expected 2 env.vars, got %d", len(inv.Env.Vars))
	}

	if inv.Env.Vars["GLOBAL_VAR"] != "global_value" {
		t.Errorf("GLOBAL_VAR = %q, want %q", inv.Env.Vars["GLOBAL_VAR"], "global_value")
	}
	if inv.Env.Vars["APP_ENV"] != "production" {
		t.Errorf("APP_ENV = %q, want %q", inv.Env.Vars["APP_ENV"], "production")
	}
}

func TestParseEnv_AllThreeLevels(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

env: {
	files: ["global.env"]
	vars: {
		LEVEL: "root"
	}
}

commands: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
				env: {
					files: ["impl.env"]
					vars: {
						LEVEL: "implementation"
					}
				}
			}
		]
		env: {
			files: ["cmd.env"]
			vars: {
				LEVEL: "command"
			}
		}
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Check root-level env
	if inv.Env == nil {
		t.Fatalf("Expected inv.Env to be non-nil")
	}
	if len(inv.Env.Files) != 1 || inv.Env.Files[0] != "global.env" {
		t.Errorf("Root Env.Files = %v, want [global.env]", inv.Env.Files)
	}
	if inv.Env.Vars["LEVEL"] != "root" {
		t.Errorf("Root LEVEL = %q, want %q", inv.Env.Vars["LEVEL"], "root")
	}

	// Check command-level env
	cmd := inv.Commands[0]
	if cmd.Env == nil {
		t.Fatalf("Expected cmd.Env to be non-nil")
	}
	if len(cmd.Env.Files) != 1 || cmd.Env.Files[0] != "cmd.env" {
		t.Errorf("Command Env.Files = %v, want [cmd.env]", cmd.Env.Files)
	}
	if cmd.Env.Vars["LEVEL"] != "command" {
		t.Errorf("Command LEVEL = %q, want %q", cmd.Env.Vars["LEVEL"], "command")
	}

	// Check implementation-level env
	impl := cmd.Implementations[0]
	if impl.Env == nil {
		t.Fatalf("Expected impl.Env to be non-nil")
	}
	if len(impl.Env.Files) != 1 || impl.Env.Files[0] != "impl.env" {
		t.Errorf("Implementation Env.Files = %v, want [impl.env]", impl.Env.Files)
	}
	if impl.Env.Vars["LEVEL"] != "implementation" {
		t.Errorf("Implementation LEVEL = %q, want %q", impl.Env.Vars["LEVEL"], "implementation")
	}
}

func TestGenerateCUE_WithRootLevelEnv(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Env: &EnvConfig{
			Files: []string{"global.env", "shared.env?"},
			Vars: map[string]string{
				"GLOBAL_VAR": "global_value",
			},
		},
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	cue := GenerateCUE(inv)

	// Check root-level env block
	if !strings.Contains(cue, `env: {`) {
		t.Errorf("GenerateCUE() should include env block, got:\n%s", cue)
	}
	if !strings.Contains(cue, `files: ["global.env", "shared.env?"]`) {
		t.Errorf("GenerateCUE() should include root-level env.files, got:\n%s", cue)
	}
	if !strings.Contains(cue, `GLOBAL_VAR: "global_value"`) {
		t.Errorf("GenerateCUE() should include root-level env.vars, got:\n%s", cue)
	}
}

func TestGenerateCUE_RootEnvRoundTrip(t *testing.T) {
	original := &Invkfile{
		Group:   "test.roundtrip",
		Version: "1.0",
		Env: &EnvConfig{
			Files: []string{"global.env", "shared.env?"},
			Vars: map[string]string{
				"GLOBAL_VAR": "global_value",
			},
		},
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	// Generate CUE
	cue := GenerateCUE(original)

	// Write to temp file
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cue), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	// Parse it back
	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify root-level env.files
	if parsed.Env == nil {
		t.Fatalf("Expected non-nil Env after roundtrip")
	}
	if len(parsed.Env.Files) != 2 {
		t.Fatalf("Expected 2 root env.files, got %d", len(parsed.Env.Files))
	}
	if parsed.Env.Files[0] != "global.env" {
		t.Errorf("Root Env.Files[0] = %q, want %q", parsed.Env.Files[0], "global.env")
	}
	if parsed.Env.Files[1] != "shared.env?" {
		t.Errorf("Root Env.Files[1] = %q, want %q", parsed.Env.Files[1], "shared.env?")
	}

	// Verify root-level env.vars
	if len(parsed.Env.Vars) != 1 {
		t.Fatalf("Expected 1 root env.vars, got %d", len(parsed.Env.Vars))
	}
	if parsed.Env.Vars["GLOBAL_VAR"] != "global_value" {
		t.Errorf("GLOBAL_VAR = %q, want %q", parsed.Env.Vars["GLOBAL_VAR"], "global_value")
	}
}

func TestValidateFlags_ReservedEnvFileName(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "env-file", description: "This should fail - reserved flag name"}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Fatal("Parse() should fail for reserved flag name 'env-file', got nil error")
	}

	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention 'reserved', got: %v", err)
	}
}

func TestValidateFlags_ReservedShortAliasE(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				target: { runtimes: [{name: "native"}] }
			}
		]
		flags: [
			{name: "environment", short: "e", description: "This should fail - reserved short alias"}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Fatal("Parse() should fail for reserved short alias 'e', got nil error")
	}

	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention 'reserved', got: %v", err)
	}
}

// ============================================================================
// Tests for GetEffectiveWorkDir (Working Directory Hierarchy)
// ============================================================================

func TestGetEffectiveWorkDir_DefaultToInvkfileDir(t *testing.T) {
	// When no workdir is specified at any level, defaults to invkfile directory
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")

	if result != tmpDir {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, tmpDir)
	}
}

func TestGetEffectiveWorkDir_RootLevel(t *testing.T) {
	// When only root-level workdir is specified
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "build",
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "build")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_CommandLevel(t *testing.T) {
	// Command-level workdir overrides root-level
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "cmd-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_ImplementationLevel(t *testing.T) {
	// Implementation-level workdir overrides command and root levels
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}
	impl := &Implementation{Script: "echo test", WorkDir: "impl-workdir"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "impl-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_CLIOverride(t *testing.T) {
	// CLI override takes highest precedence
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}
	impl := &Implementation{Script: "echo test", WorkDir: "impl-workdir"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "cli-workdir")
	expected := filepath.Join(tmpDir, "cli-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_AbsolutePath(t *testing.T) {
	// Absolute paths should be returned as-is (not joined with invkfile dir)
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	absPath := filepath.Join(t.TempDir(), "absolute-workdir")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  absPath,
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")

	if result != absPath {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, absPath)
	}
}

func TestGetEffectiveWorkDir_ForwardSlashConversion(t *testing.T) {
	// Forward slashes in CUE should be converted to native path separator
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "nested/deep/path", // Forward slashes (CUE format)
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "nested", "deep", "path")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_NilCommand(t *testing.T) {
	// Should handle nil command gracefully
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "root-workdir",
	}

	result := inv.GetEffectiveWorkDir(nil, nil, "")
	expected := filepath.Join(tmpDir, "root-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_NilImplementation(t *testing.T) {
	// Should handle nil implementation gracefully
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}

	result := inv.GetEffectiveWorkDir(cmd, nil, "")
	expected := filepath.Join(tmpDir, "cmd-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_EmptyCommandWorkDir(t *testing.T) {
	// Empty command workdir should fall through to root level
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: ""} // Empty command workdir
	impl := &Implementation{Script: "echo test", WorkDir: ""}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "root-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_ParentDirectory(t *testing.T) {
	// Relative paths with .. should work correctly
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	invkfilePath := filepath.Join(subDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  "../sibling", // Go up and into sibling directory
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(subDir, "..", "sibling")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_CurrentDirectory(t *testing.T) {
	// "." should resolve to invkfile directory
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		WorkDir:  ".",
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, ".")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_PackPath(t *testing.T) {
	// When loaded from a pack, paths should resolve against pack directory
	tmpDir := t.TempDir()
	packDir := filepath.Join(tmpDir, "mypack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("Failed to create pack dir: %v", err)
	}
	invkfilePath := filepath.Join(packDir, "invkfile.cue")

	inv := &Invkfile{
		Group:    "test",
		FilePath: invkfilePath,
		PackPath: packDir, // Loaded from pack
		WorkDir:  "scripts",
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	// Should resolve against pack directory (via GetScriptBasePath)
	expected := filepath.Join(packDir, "scripts")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestParseWorkDir_RootLevel(t *testing.T) {
	// Test parsing workdir from CUE at root level
	cueContent := `
group: "test"
version: "1.0"
workdir: "build/output"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.WorkDir != "build/output" {
		t.Errorf("Invkfile.WorkDir = %q, want %q", inv.WorkDir, "build/output")
	}
}

func TestParseWorkDir_CommandLevel(t *testing.T) {
	// Test parsing workdir from CUE at command level
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		workdir: "cmd-specific"
		implementations: [
			{
				script: "echo test"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Commands[0].WorkDir != "cmd-specific" {
		t.Errorf("Command.WorkDir = %q, want %q", inv.Commands[0].WorkDir, "cmd-specific")
	}
}

func TestParseWorkDir_ImplementationLevel(t *testing.T) {
	// Test parsing workdir from CUE at implementation level
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				workdir: "impl-specific"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Commands[0].Implementations[0].WorkDir != "impl-specific" {
		t.Errorf("Implementation.WorkDir = %q, want %q", inv.Commands[0].Implementations[0].WorkDir, "impl-specific")
	}
}

func TestParseWorkDir_AllLevels(t *testing.T) {
	// Test parsing workdir at all levels and verify precedence
	cueContent := `
group: "test"
version: "1.0"
workdir: "root-dir"

commands: [
	{
		name: "test"
		workdir: "cmd-dir"
		implementations: [
			{
				script: "echo test"
				workdir: "impl-dir"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify all levels are parsed correctly
	if inv.WorkDir != "root-dir" {
		t.Errorf("Invkfile.WorkDir = %q, want %q", inv.WorkDir, "root-dir")
	}
	if inv.Commands[0].WorkDir != "cmd-dir" {
		t.Errorf("Command.WorkDir = %q, want %q", inv.Commands[0].WorkDir, "cmd-dir")
	}
	if inv.Commands[0].Implementations[0].WorkDir != "impl-dir" {
		t.Errorf("Implementation.WorkDir = %q, want %q", inv.Commands[0].Implementations[0].WorkDir, "impl-dir")
	}

	// Verify precedence using GetEffectiveWorkDir
	cmd := &inv.Commands[0]
	impl := &inv.Commands[0].Implementations[0]

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "impl-dir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGenerateCUE_WithWorkDir(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		WorkDir: "build",
		Commands: []Command{
			{
				Name:    "deploy",
				WorkDir: "deploy-dir",
				Implementations: []Implementation{
					{
						Script:  "echo deploying",
						WorkDir: "impl-dir",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check root-level workdir
	if !strings.Contains(output, `workdir: "build"`) {
		t.Error("GenerateCUE should contain root-level 'workdir: \"build\"'")
	}

	// Check command-level workdir
	if !strings.Contains(output, `workdir: "deploy-dir"`) {
		t.Error("GenerateCUE should contain command-level 'workdir: \"deploy-dir\"'")
	}

	// Check implementation-level workdir
	if !strings.Contains(output, `workdir: "impl-dir"`) {
		t.Error("GenerateCUE should contain implementation-level 'workdir: \"impl-dir\"'")
	}
}

func TestGenerateCUE_WithWorkDir_RoundTrip(t *testing.T) {
	// Create an invkfile with workdir at all levels, generate CUE, parse it back, and verify
	original := &Invkfile{
		Group:   "test",
		Version: "1.0",
		WorkDir: "root-workdir",
		Commands: []Command{
			{
				Name:    "build",
				WorkDir: "cmd-workdir",
				Implementations: []Implementation{
					{
						Script:  "echo building",
						WorkDir: "impl-workdir",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	// Generate CUE
	cueContent := GenerateCUE(original)

	// Write to temp file and parse back
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	// Verify parsed workdir values match original
	if parsed.WorkDir != "root-workdir" {
		t.Errorf("Invkfile.WorkDir = %q, want %q", parsed.WorkDir, "root-workdir")
	}
	if parsed.Commands[0].WorkDir != "cmd-workdir" {
		t.Errorf("Command.WorkDir = %q, want %q", parsed.Commands[0].WorkDir, "cmd-workdir")
	}
	if parsed.Commands[0].Implementations[0].WorkDir != "impl-workdir" {
		t.Errorf("Implementation.WorkDir = %q, want %q", parsed.Commands[0].Implementations[0].WorkDir, "impl-workdir")
	}
}

// TestParse_RootLevelDependsOn verifies that root-level depends_on is parsed correctly
func TestParse_RootLevelDependsOn(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

depends_on: {
	tools: [{alternatives: ["sh"]}]
	capabilities: [{alternatives: ["internet"]}]
	filepaths: [{alternatives: ["/etc/hosts"], readable: true}]
	env_vars: [{alternatives: [{name: "HOME"}]}]
}

commands: [
	{
		name: "hello"
		implementations: [
			{
				script: "echo hello"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invkfile: %v", err)
	}

	// Verify root-level depends_on was parsed
	if parsed.DependsOn == nil {
		t.Fatal("Invkfile.DependsOn should not be nil")
	}

	if len(parsed.DependsOn.Tools) != 1 {
		t.Fatalf("Expected 1 tool dependency, got %d", len(parsed.DependsOn.Tools))
	}
	if parsed.DependsOn.Tools[0].Alternatives[0] != "sh" {
		t.Errorf("Tool alternative = %q, want %q", parsed.DependsOn.Tools[0].Alternatives[0], "sh")
	}

	if len(parsed.DependsOn.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability dependency, got %d", len(parsed.DependsOn.Capabilities))
	}
	if parsed.DependsOn.Capabilities[0].Alternatives[0] != CapabilityInternet {
		t.Errorf("Capability alternative = %v, want %v", parsed.DependsOn.Capabilities[0].Alternatives[0], CapabilityInternet)
	}

	if len(parsed.DependsOn.Filepaths) != 1 {
		t.Fatalf("Expected 1 filepath dependency, got %d", len(parsed.DependsOn.Filepaths))
	}
	if parsed.DependsOn.Filepaths[0].Alternatives[0] != "/etc/hosts" {
		t.Errorf("Filepath alternative = %q, want %q", parsed.DependsOn.Filepaths[0].Alternatives[0], "/etc/hosts")
	}
	if !parsed.DependsOn.Filepaths[0].Readable {
		t.Error("Filepath.Readable should be true")
	}

	if len(parsed.DependsOn.EnvVars) != 1 {
		t.Fatalf("Expected 1 env_var dependency, got %d", len(parsed.DependsOn.EnvVars))
	}
	if parsed.DependsOn.EnvVars[0].Alternatives[0].Name != "HOME" {
		t.Errorf("EnvVar alternative name = %q, want %q", parsed.DependsOn.EnvVars[0].Alternatives[0].Name, "HOME")
	}
}

// TestInvkfile_HasRootLevelDependencies verifies the helper method works correctly
func TestInvkfile_HasRootLevelDependencies(t *testing.T) {
	tests := []struct {
		name     string
		deps     *DependsOn
		expected bool
	}{
		{
			name:     "nil depends_on",
			deps:     nil,
			expected: false,
		},
		{
			name:     "empty depends_on",
			deps:     &DependsOn{},
			expected: false,
		},
		{
			name: "with tools",
			deps: &DependsOn{
				Tools: []ToolDependency{{Alternatives: []string{"sh"}}},
			},
			expected: true,
		},
		{
			name: "with capabilities",
			deps: &DependsOn{
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
			},
			expected: true,
		},
		{
			name: "with env_vars",
			deps: &DependsOn{
				EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := &Invkfile{
				Group:     "test",
				DependsOn: tt.deps,
				Commands:  []Command{{Name: "test", Implementations: []Implementation{{Script: "echo", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}}}},
			}
			if got := inv.HasRootLevelDependencies(); got != tt.expected {
				t.Errorf("HasRootLevelDependencies() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestMergeDependsOnAll verifies three-way merge works correctly
func TestMergeDependsOnAll(t *testing.T) {
	rootDeps := &DependsOn{
		Tools:        []ToolDependency{{Alternatives: []string{"sh"}}},
		Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}}},
	}
	cmdDeps := &DependsOn{
		Tools:     []ToolDependency{{Alternatives: []string{"bash"}}},
		Filepaths: []FilepathDependency{{Alternatives: []string{"/etc/hosts"}}},
	}
	implDeps := &DependsOn{
		Tools:   []ToolDependency{{Alternatives: []string{"python3"}}},
		EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}},
	}

	merged := MergeDependsOnAll(rootDeps, cmdDeps, implDeps)

	if merged == nil {
		t.Fatal("MergeDependsOnAll should return non-nil result")
	}

	// Verify tools are merged in order: root -> command -> impl
	if len(merged.Tools) != 3 {
		t.Fatalf("Expected 3 tools after merge, got %d", len(merged.Tools))
	}
	if merged.Tools[0].Alternatives[0] != "sh" {
		t.Errorf("First tool = %q, want %q", merged.Tools[0].Alternatives[0], "sh")
	}
	if merged.Tools[1].Alternatives[0] != "bash" {
		t.Errorf("Second tool = %q, want %q", merged.Tools[1].Alternatives[0], "bash")
	}
	if merged.Tools[2].Alternatives[0] != "python3" {
		t.Errorf("Third tool = %q, want %q", merged.Tools[2].Alternatives[0], "python3")
	}

	// Verify capabilities from root
	if len(merged.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(merged.Capabilities))
	}
	if merged.Capabilities[0].Alternatives[0] != CapabilityLocalAreaNetwork {
		t.Errorf("Capability = %v, want %v", merged.Capabilities[0].Alternatives[0], CapabilityLocalAreaNetwork)
	}

	// Verify filepaths from command
	if len(merged.Filepaths) != 1 {
		t.Fatalf("Expected 1 filepath, got %d", len(merged.Filepaths))
	}
	if merged.Filepaths[0].Alternatives[0] != "/etc/hosts" {
		t.Errorf("Filepath = %q, want %q", merged.Filepaths[0].Alternatives[0], "/etc/hosts")
	}

	// Verify env_vars from impl
	if len(merged.EnvVars) != 1 {
		t.Fatalf("Expected 1 env_var, got %d", len(merged.EnvVars))
	}
	if merged.EnvVars[0].Alternatives[0].Name != "HOME" {
		t.Errorf("EnvVar name = %q, want %q", merged.EnvVars[0].Alternatives[0].Name, "HOME")
	}
}

// TestMergeDependsOnAll_NilInputs verifies three-way merge handles nil inputs
func TestMergeDependsOnAll_NilInputs(t *testing.T) {
	tests := []struct {
		name     string
		rootDeps *DependsOn
		cmdDeps  *DependsOn
		implDeps *DependsOn
		expected bool // true if result should be non-nil
	}{
		{
			name:     "all nil",
			rootDeps: nil,
			cmdDeps:  nil,
			implDeps: nil,
			expected: false,
		},
		{
			name:     "only root",
			rootDeps: &DependsOn{Tools: []ToolDependency{{Alternatives: []string{"sh"}}}},
			cmdDeps:  nil,
			implDeps: nil,
			expected: true,
		},
		{
			name:     "only command",
			rootDeps: nil,
			cmdDeps:  &DependsOn{Tools: []ToolDependency{{Alternatives: []string{"bash"}}}},
			implDeps: nil,
			expected: true,
		},
		{
			name:     "only impl",
			rootDeps: nil,
			cmdDeps:  nil,
			implDeps: &DependsOn{Tools: []ToolDependency{{Alternatives: []string{"python3"}}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeDependsOnAll(tt.rootDeps, tt.cmdDeps, tt.implDeps)
			if tt.expected && result == nil {
				t.Error("MergeDependsOnAll should return non-nil result")
			}
			if !tt.expected && result != nil {
				t.Error("MergeDependsOnAll should return nil")
			}
		})
	}
}

// TestGenerateCUE_WithRootLevelDependsOn verifies GenerateCUE produces valid CUE for root-level depends_on
func TestGenerateCUE_WithRootLevelDependsOn(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		DependsOn: &DependsOn{
			Tools:        []ToolDependency{{Alternatives: []string{"sh", "bash"}}},
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
			Filepaths:    []FilepathDependency{{Alternatives: []string{"/etc/hosts"}, Readable: true}},
			EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}},
		},
		Commands: []Command{
			{
				Name: "hello",
				Implementations: []Implementation{
					{
						Script: "echo hello",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Verify depends_on section is present at root level
	if !strings.Contains(result, "depends_on:") {
		t.Error("GenerateCUE should include 'depends_on:' section at root level")
	}
	if !strings.Contains(result, "tools:") {
		t.Error("GenerateCUE should include 'tools:' in depends_on")
	}
	if !strings.Contains(result, `"sh"`) {
		t.Error("GenerateCUE should include 'sh' tool")
	}
	if !strings.Contains(result, `"bash"`) {
		t.Error("GenerateCUE should include 'bash' tool")
	}
	if !strings.Contains(result, "capabilities:") {
		t.Error("GenerateCUE should include 'capabilities:' in depends_on")
	}
	if !strings.Contains(result, `"internet"`) {
		t.Error("GenerateCUE should include 'internet' capability")
	}
	if !strings.Contains(result, "filepaths:") {
		t.Error("GenerateCUE should include 'filepaths:' in depends_on")
	}
	if !strings.Contains(result, `"/etc/hosts"`) {
		t.Error("GenerateCUE should include filepath")
	}
	if !strings.Contains(result, "readable: true") {
		t.Error("GenerateCUE should include 'readable: true'")
	}
	if !strings.Contains(result, "env_vars:") {
		t.Error("GenerateCUE should include 'env_vars:' in depends_on")
	}
	if !strings.Contains(result, `"HOME"`) {
		t.Error("GenerateCUE should include HOME env var")
	}

	// Verify the generated CUE is parseable
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(result), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	// Verify parsed root-level depends_on matches original
	if parsed.DependsOn == nil {
		t.Fatal("Parsed Invkfile.DependsOn should not be nil")
	}
	if len(parsed.DependsOn.Tools) != 1 {
		t.Errorf("Expected 1 tool dependency, got %d", len(parsed.DependsOn.Tools))
	}
	if len(parsed.DependsOn.Capabilities) != 1 {
		t.Errorf("Expected 1 capability dependency, got %d", len(parsed.DependsOn.Capabilities))
	}
	if len(parsed.DependsOn.Filepaths) != 1 {
		t.Errorf("Expected 1 filepath dependency, got %d", len(parsed.DependsOn.Filepaths))
	}
	if len(parsed.DependsOn.EnvVars) != 1 {
		t.Errorf("Expected 1 env_var dependency, got %d", len(parsed.DependsOn.EnvVars))
	}
}

// TestGenerateCUE_WithRootLevelDependsOn_CustomChecks verifies GenerateCUE handles custom_checks at root level
func TestGenerateCUE_WithRootLevelDependsOn_CustomChecks(t *testing.T) {
	expectedCode := 0
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		DependsOn: &DependsOn{
			CustomChecks: []CustomCheckDependency{
				{
					Name:         "check-version",
					CheckScript:  "sh --version",
					ExpectedCode: &expectedCode,
				},
				{
					Alternatives: []CustomCheck{
						{Name: "alt1", CheckScript: "echo 1"},
						{Name: "alt2", CheckScript: "echo 2"},
					},
				},
			},
		},
		Commands: []Command{
			{
				Name: "hello",
				Implementations: []Implementation{
					{
						Script: "echo hello",
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Verify custom_checks section is present at root level
	if !strings.Contains(result, "custom_checks:") {
		t.Error("GenerateCUE should include 'custom_checks:' section at root level")
	}
	if !strings.Contains(result, `"check-version"`) {
		t.Error("GenerateCUE should include 'check-version' custom check name")
	}
	if !strings.Contains(result, `"sh --version"`) {
		t.Error("GenerateCUE should include 'sh --version' check_script")
	}
	if !strings.Contains(result, "expected_code: 0") {
		t.Error("GenerateCUE should include 'expected_code: 0'")
	}
	if !strings.Contains(result, "alternatives:") {
		t.Error("GenerateCUE should include alternatives for custom checks")
	}
}

// TestParse_RootLevelDependsOn_CustomChecks verifies custom_checks parsing at root level
func TestParse_RootLevelDependsOn_CustomChecks(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

depends_on: {
	custom_checks: [
		{
			name: "version-check"
			check_script: "sh --version"
			expected_code: 0
		},
		{
			alternatives: [
				{name: "bash-check", check_script: "bash --version"},
				{name: "sh-check", check_script: "sh --version"}
			]
		}
	]
}

commands: [
	{
		name: "hello"
		implementations: [
			{
				script: "echo hello"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invkfile: %v", err)
	}

	if parsed.DependsOn == nil {
		t.Fatal("Invkfile.DependsOn should not be nil")
	}

	if len(parsed.DependsOn.CustomChecks) != 2 {
		t.Fatalf("Expected 2 custom checks, got %d", len(parsed.DependsOn.CustomChecks))
	}

	// First check is direct (not alternatives)
	check1 := parsed.DependsOn.CustomChecks[0]
	if check1.IsAlternatives() {
		t.Error("First custom check should not be alternatives format")
	}
	if check1.Name != "version-check" {
		t.Errorf("First check name = %q, want %q", check1.Name, "version-check")
	}
	if check1.CheckScript != "sh --version" {
		t.Errorf("First check script = %q, want %q", check1.CheckScript, "sh --version")
	}

	// Second check uses alternatives
	check2 := parsed.DependsOn.CustomChecks[1]
	if !check2.IsAlternatives() {
		t.Error("Second custom check should be alternatives format")
	}
	if len(check2.Alternatives) != 2 {
		t.Fatalf("Expected 2 alternatives in second check, got %d", len(check2.Alternatives))
	}
}

// TestParse_RootLevelDependsOn_CommandDeps verifies command dependencies parsing at root level
func TestParse_RootLevelDependsOn_CommandDeps(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

depends_on: {
	commands: [
		{alternatives: ["test setup"]},
		{alternatives: ["test init", "test bootstrap"]}
	]
}

commands: [
	{
		name: "hello"
		implementations: [
			{
				script: "echo hello"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invkfile: %v", err)
	}

	if parsed.DependsOn == nil {
		t.Fatal("Invkfile.DependsOn should not be nil")
	}

	if len(parsed.DependsOn.Commands) != 2 {
		t.Fatalf("Expected 2 command dependencies, got %d", len(parsed.DependsOn.Commands))
	}

	// First command dependency has one alternative
	if len(parsed.DependsOn.Commands[0].Alternatives) != 1 {
		t.Errorf("Expected 1 alternative in first command dep, got %d", len(parsed.DependsOn.Commands[0].Alternatives))
	}
	if parsed.DependsOn.Commands[0].Alternatives[0] != "test setup" {
		t.Errorf("First command dep alternative = %q, want %q", parsed.DependsOn.Commands[0].Alternatives[0], "test setup")
	}

	// Second command dependency has two alternatives
	if len(parsed.DependsOn.Commands[1].Alternatives) != 2 {
		t.Errorf("Expected 2 alternatives in second command dep, got %d", len(parsed.DependsOn.Commands[1].Alternatives))
	}
}

// ============================================================================
// Tests for TTY Field (Interactive Mode)
// ============================================================================

func TestParseTTYField(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "interactive"
		description: "Command requiring TTY"
		implementations: [
			{
				script: "vim file.txt"
				tty: true
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Implementations) != 1 {
		t.Fatalf("Expected 1 implementation, got %d", len(cmd.Implementations))
	}

	impl := cmd.Implementations[0]
	if !impl.TTY {
		t.Error("TTY should be true")
	}

	if !impl.RequiresTTY() {
		t.Error("RequiresTTY() should return true when TTY is true")
	}
}

func TestParseTTYField_DefaultFalse(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "simple"
		implementations: [
			{
				script: "echo hello"
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	impl := cmd.Implementations[0]

	if impl.TTY {
		t.Error("TTY should be false by default")
	}

	if impl.RequiresTTY() {
		t.Error("RequiresTTY() should return false when TTY is not set")
	}
}

func TestParseTTYField_ExplicitFalse(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "simple"
		implementations: [
			{
				script: "echo hello"
				tty: false
				target: { runtimes: [{name: "native"}] }
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	impl := cmd.Implementations[0]

	if impl.TTY {
		t.Error("TTY should be false when explicitly set to false")
	}

	if impl.RequiresTTY() {
		t.Error("RequiresTTY() should return false when TTY is explicitly false")
	}
}

func TestImplementation_RequiresTTY(t *testing.T) {
	tests := []struct {
		name     string
		impl     Implementation
		expected bool
	}{
		{
			name: "tty true",
			impl: Implementation{
				Script: "vim file.txt",
				TTY:    true,
				Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}},
			},
			expected: true,
		},
		{
			name: "tty false",
			impl: Implementation{
				Script: "echo hello",
				TTY:    false,
				Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}},
			},
			expected: false,
		},
		{
			name: "tty not set (default)",
			impl: Implementation{
				Script: "echo hello",
				Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.impl.RequiresTTY()
			if result != tt.expected {
				t.Errorf("RequiresTTY() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateCUE_WithTTY(t *testing.T) {
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "interactive",
				Implementations: []Implementation{
					{
						Script: "vim file.txt",
						TTY:    true,
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, "tty: true") {
		t.Error("GenerateCUE should contain 'tty: true'")
	}
}

func TestGenerateCUE_WithTTY_False(t *testing.T) {
	// When tty is false (default), it should not appear in the output
	inv := &Invkfile{
		Group:   "test",
		Version: "1.0",
		Commands: []Command{
			{
				Name: "simple",
				Implementations: []Implementation{
					{
						Script: "echo hello",
						TTY:    false,
						Target: Target{
							Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
						},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if strings.Contains(output, "tty:") {
		t.Error("GenerateCUE should not contain 'tty:' when it's false")
	}
}

func TestParseTTYField_MultipleImplementations(t *testing.T) {
	cueContent := `
group: "test"
version: "1.0"

commands: [
	{
		name: "edit"
		implementations: [
			{
				script: "vim file.txt"
				tty: true
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			},
			{
				script: "notepad file.txt"
				tty: true
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "windows"}]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Implementations) != 2 {
		t.Fatalf("Expected 2 implementations, got %d", len(cmd.Implementations))
	}

	// Both implementations should have TTY set to true
	for i, impl := range cmd.Implementations {
		if !impl.TTY {
			t.Errorf("Implementation %d: TTY should be true", i)
		}
		if !impl.RequiresTTY() {
			t.Errorf("Implementation %d: RequiresTTY() should return true", i)
		}
	}
}
