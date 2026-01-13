package invowkfile

import (
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
	invowkfilePath := "/home/user/project/invowkfile.cue"

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
			result := s.GetScriptFilePath(invowkfilePath)
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
			result, err := s.ResolveScript("/fake/path/invowkfile.toml")
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

	// Create invowkfile path
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	t.Run("resolve script from file", func(t *testing.T) {
		s := &Implementation{Script: "./test.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		result, err := s.ResolveScript(invowkfilePath)
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
		result, err := s.ResolveScript(invowkfilePath)
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
		_, err := s.ResolveScript(invowkfilePath)
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

	invowkfilePath := "/project/invowkfile.toml"

	t.Run("resolve script from virtual fs", func(t *testing.T) {
		s := &Implementation{Script: "./scripts/build.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}
		result, err := s.ResolveScriptWithFS(invowkfilePath, readFile)
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
		result, err := s.ResolveScriptWithFS(invowkfilePath, readFile)
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
		_, err := s.ResolveScriptWithFS(invowkfilePath, readFile)
		if err == nil {
			t.Error("ResolveScriptWithFS() expected error for missing file, got nil")
		}
	})
}

func TestMultiLineScriptParsing(t *testing.T) {
	// Test that CUE multi-line strings are properly parsed
	cueContent := `
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
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
	if !strings.Contains(cmd.Implementations[0].Script, "Line 1") || !strings.Contains(cmd.Implementations[0].Script, "Line 2") {
		t.Errorf("Multi-line script parsing failed.\nGot: %q", cmd.Implementations[0].Script)
	}

	// Verify resolution works too
	resolved, err := cmd.Implementations[0].ResolveScript(invowkfilePath)
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")
	s := &Implementation{Script: "./test.sh", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}}}

	// First resolution
	result1, err := s.ResolveScript(invowkfilePath)
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
	result2, err := s.ResolveScript(invowkfilePath)
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
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Tools: []ToolDependency{{Name: "git"}}}),
			expected: true,
		},
		{
			name:     "only commands",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Commands: []CommandDependency{{Name: "build"}}}),
			expected: true,
		},
		{
			name: "both tools and commands",
			cmd: testCommandWithDeps("test", "echo", &DependsOn{
				Tools:    []ToolDependency{{Name: "git"}},
				Commands: []CommandDependency{{Name: "build"}},
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
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Commands: []CommandDependency{{Name: "build"}}}),
			expected: []string{"build"},
		},
		{
			name: "multiple commands",
			cmd: testCommandWithDeps("test", "echo", &DependsOn{
				Commands: []CommandDependency{
					{Name: "clean"},
					{Name: "build"},
					{Name: "test unit"},
				},
			}),
			expected: []string{"clean", "build", "test unit"},
		},
		{
			name:     "only tools no commands",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Tools: []ToolDependency{{Name: "git"}}}),
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
				{name: "git"},
				{name: "docker", check_script: "docker --version", expected_output: "Docker"},
			]
			commands: [
				{name: "build"},
				{name: "test unit"},
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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

	if cmd.DependsOn.Tools[0].Name != "git" {
		t.Errorf("First tool name = %q, want %q", cmd.DependsOn.Tools[0].Name, "git")
	}

	if cmd.DependsOn.Tools[1].Name != "docker" {
		t.Errorf("Second tool name = %q, want %q", cmd.DependsOn.Tools[1].Name, "docker")
	}

	if cmd.DependsOn.Tools[1].CheckScript != "docker --version" {
		t.Errorf("Second tool check_script = %q, want %q", cmd.DependsOn.Tools[1].CheckScript, "docker --version")
	}

	if cmd.DependsOn.Tools[1].ExpectedOutput != "Docker" {
		t.Errorf("Second tool expected_output = %q, want %q", cmd.DependsOn.Tools[1].ExpectedOutput, "Docker")
	}

	// Check commands
	if len(cmd.DependsOn.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(cmd.DependsOn.Commands))
	}

	if cmd.DependsOn.Commands[0].Name != "build" {
		t.Errorf("First command name = %q, want %q", cmd.DependsOn.Commands[0].Name, "build")
	}

	if cmd.DependsOn.Commands[1].Name != "test unit" {
		t.Errorf("Second command name = %q, want %q", cmd.DependsOn.Commands[1].Name, "test unit")
	}
}

func TestParseDependsOn_ToolsOnly(t *testing.T) {
	cueContent := `
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
				{name: "make"},
				{name: "gcc"},
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
				{name: "build"},
				{name: "test"},
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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

func TestParseDependsOn_WithCustomValidation(t *testing.T) {
	cueContent := `
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
				{name: "make"},
				{
					name: "go"
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
	if cmd.DependsOn.Tools[0].Name != "make" {
		t.Errorf("First tool name = %q, want %q", cmd.DependsOn.Tools[0].Name, "make")
	}
	if cmd.DependsOn.Tools[0].CheckScript != "" {
		t.Errorf("First tool check_script should be empty, got %q", cmd.DependsOn.Tools[0].CheckScript)
	}

	// Second tool - with custom validation
	goTool := cmd.DependsOn.Tools[1]
	if goTool.Name != "go" {
		t.Errorf("Second tool name = %q, want %q", goTool.Name, "go")
	}
	if goTool.CheckScript != "go version" {
		t.Errorf("Second tool check_script = %q, want %q", goTool.CheckScript, "go version")
	}
	if goTool.ExpectedCode == nil {
		t.Error("Second tool expected_code should not be nil")
	} else if *goTool.ExpectedCode != 0 {
		t.Errorf("Second tool expected_code = %d, want 0", *goTool.ExpectedCode)
	}
	if goTool.ExpectedOutput != "go1\\." {
		t.Errorf("Second tool expected_output = %q, want %q", goTool.ExpectedOutput, "go1\\.")
	}
}

func TestParseDependsOn_WithFilepaths(t *testing.T) {
	cueContent := `
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
	inv := &Invowkfile{
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
	inv := &Invowkfile{
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
	inv := &Invowkfile{
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
	inv := &Invowkfile{
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
				{name: "local-area-network"},
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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
	if cap0.Name != CapabilityLocalAreaNetwork {
		t.Errorf("First capability name = %q, want %q", cap0.Name, CapabilityLocalAreaNetwork)
	}

	// Second capability - internet
	cap1 := cmd.DependsOn.Capabilities[1]
	if cap1.Name != CapabilityInternet {
		t.Errorf("Second capability name = %q, want %q", cap1.Name, CapabilityInternet)
	}
}

func TestParseDependsOn_CapabilitiesAtImplementationLevel(t *testing.T) {
	cueContent := `
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
						{name: "internet"},
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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	inv, err := Parse(invowkfilePath)
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

	if impl.DependsOn.Capabilities[0].Name != CapabilityInternet {
		t.Errorf("Capability name = %q, want %q", impl.DependsOn.Capabilities[0].Name, CapabilityInternet)
	}
}

func TestCommand_HasDependencies_WithCapabilities(t *testing.T) {
	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Target: Target{Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Name: CapabilityInternet}},
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
			Capabilities: []CapabilityDependency{{Name: CapabilityLocalAreaNetwork}},
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
			Capabilities: []CapabilityDependency{{Name: CapabilityInternet}},
		},
	}

	if !impl.HasDependencies() {
		t.Error("Implementation.HasDependencies() should return true when capabilities are present")
	}
}

func TestMergeDependsOn_WithCapabilities(t *testing.T) {
	cmdDeps := &DependsOn{
		Capabilities: []CapabilityDependency{{Name: CapabilityLocalAreaNetwork}},
	}

	scriptDeps := &DependsOn{
		Capabilities: []CapabilityDependency{{Name: CapabilityInternet}},
	}

	merged := MergeDependsOn(cmdDeps, scriptDeps)

	if merged == nil {
		t.Fatal("MergeDependsOn should return non-nil result")
	}

	if len(merged.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities after merge, got %d", len(merged.Capabilities))
	}

	// Command-level capabilities should come first
	if merged.Capabilities[0].Name != CapabilityLocalAreaNetwork {
		t.Errorf("First capability = %q, want %q", merged.Capabilities[0].Name, CapabilityLocalAreaNetwork)
	}

	if merged.Capabilities[1].Name != CapabilityInternet {
		t.Errorf("Second capability = %q, want %q", merged.Capabilities[1].Name, CapabilityInternet)
	}
}

func TestGenerateCUE_WithCapabilities(t *testing.T) {
	inv := &Invowkfile{
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
						{Name: CapabilityInternet},
						{Name: CapabilityLocalAreaNetwork},
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

	if !strings.Contains(result, `name: "internet"`) {
		t.Error("GenerateCUE should include internet capability")
	}

	if !strings.Contains(result, `name: "local-area-network"`) {
		t.Error("GenerateCUE should include local-area-network capability")
	}
}

func TestGenerateCUE_WithCapabilitiesAtImplementationLevel(t *testing.T) {
	inv := &Invowkfile{
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
								{Name: CapabilityInternet},
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

	if !strings.Contains(result, `name: "internet"`) {
		t.Error("GenerateCUE should include internet capability")
	}
}
