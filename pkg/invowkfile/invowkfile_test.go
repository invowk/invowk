package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			cmd := &Command{Script: tt.script}
			result := cmd.IsScriptFile()
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
			cmd := &Command{Script: tt.script}
			result := cmd.GetScriptFilePath(invowkfilePath)
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
			cmd := &Command{Name: "test", Script: tt.script}
			result, err := cmd.ResolveScript("/fake/path/invowkfile.toml")
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
		cmd := &Command{Name: "test", Script: "./test.sh"}
		result, err := cmd.ResolveScript(invowkfilePath)
		if err != nil {
			t.Errorf("ResolveScript() error = %v", err)
			return
		}
		if result != scriptContent {
			t.Errorf("ResolveScript() = %q, want %q", result, scriptContent)
		}
	})

	t.Run("resolve script with absolute path", func(t *testing.T) {
		cmd := &Command{Name: "test", Script: scriptPath}
		result, err := cmd.ResolveScript(invowkfilePath)
		if err != nil {
			t.Errorf("ResolveScript() error = %v", err)
			return
		}
		if result != scriptContent {
			t.Errorf("ResolveScript() = %q, want %q", result, scriptContent)
		}
	})

	t.Run("error on missing script file", func(t *testing.T) {
		cmd := &Command{Name: "test", Script: "./nonexistent.sh"}
		_, err := cmd.ResolveScript(invowkfilePath)
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
		cmd := &Command{Name: "build", Script: "./scripts/build.sh"}
		result, err := cmd.ResolveScriptWithFS(invowkfilePath, readFile)
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
		cmd := &Command{Name: "inline", Script: "echo hello world"}
		result, err := cmd.ResolveScriptWithFS(invowkfilePath, readFile)
		if err != nil {
			t.Errorf("ResolveScriptWithFS() error = %v", err)
			return
		}
		if result != "echo hello world" {
			t.Errorf("ResolveScriptWithFS() = %q, want %q", result, "echo hello world")
		}
	})

	t.Run("error on missing file in virtual fs", func(t *testing.T) {
		cmd := &Command{Name: "missing", Script: "./scripts/nonexistent.sh"}
		_, err := cmd.ResolveScriptWithFS(invowkfilePath, readFile)
		if err == nil {
			t.Error("ResolveScriptWithFS() expected error for missing file, got nil")
		}
	})
}

func TestMultiLineScriptParsing(t *testing.T) {
	// Test that CUE multi-line strings are properly parsed
	cueContent := `
version: "1.0"
default_runtime: "native"

commands: [
	{
		name: "multiline-test"
		description: "Test multi-line script"
		runtimes: ["native"]
		script: """
			#!/bin/bash
			set -e
			echo "Line 1"
			echo "Line 2"
			echo "Line 3"
			"""
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
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
	if !strings.Contains(cmd.Script, "Line 1") || !strings.Contains(cmd.Script, "Line 2") {
		t.Errorf("Multi-line script parsing failed.\nGot: %q", cmd.Script)
	}

	// Verify resolution works too
	resolved, err := cmd.ResolveScript(invowkfilePath)
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
	cmd := &Command{Name: "test", Script: "./test.sh"}

	// First resolution
	result1, err := cmd.ResolveScript(invowkfilePath)
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
	result2, err := cmd.ResolveScript(invowkfilePath)
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
			cmd:      Command{Name: "test", Script: "echo"},
			expected: false,
		},
		{
			name:     "empty DependsOn",
			cmd:      Command{Name: "test", Script: "echo", DependsOn: &DependsOn{}},
			expected: false,
		},
		{
			name: "only tools",
			cmd: Command{
				Name:   "test",
				Script: "echo",
				DependsOn: &DependsOn{
					Tools: []ToolDependency{{Name: "git"}},
				},
			},
			expected: true,
		},
		{
			name: "only commands",
			cmd: Command{
				Name:   "test",
				Script: "echo",
				DependsOn: &DependsOn{
					Commands: []CommandDependency{{Name: "build"}},
				},
			},
			expected: true,
		},
		{
			name: "both tools and commands",
			cmd: Command{
				Name:   "test",
				Script: "echo",
				DependsOn: &DependsOn{
					Tools:    []ToolDependency{{Name: "git"}},
					Commands: []CommandDependency{{Name: "build"}},
				},
			},
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
			cmd:      Command{Name: "test", Script: "echo"},
			expected: nil,
		},
		{
			name:     "empty DependsOn",
			cmd:      Command{Name: "test", Script: "echo", DependsOn: &DependsOn{}},
			expected: []string{},
		},
		{
			name: "single command",
			cmd: Command{
				Name:   "test",
				Script: "echo",
				DependsOn: &DependsOn{
					Commands: []CommandDependency{{Name: "build"}},
				},
			},
			expected: []string{"build"},
		},
		{
			name: "multiple commands",
			cmd: Command{
				Name:   "test",
				Script: "echo",
				DependsOn: &DependsOn{
					Commands: []CommandDependency{
						{Name: "clean"},
						{Name: "build"},
						{Name: "test unit"},
					},
				},
			},
			expected: []string{"clean", "build", "test unit"},
		},
		{
			name: "only tools no commands",
			cmd: Command{
				Name:   "test",
				Script: "echo",
				DependsOn: &DependsOn{
					Tools: []ToolDependency{{Name: "git"}},
				},
			},
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
default_runtime: "native"

commands: [
	{
		name: "release"
		runtimes: ["native"]
		script: "echo releasing"
		works_on: {
			hosts: ["linux", "mac"]
		}
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
default_runtime: "native"

commands: [
	{
		name: "build"
runtimes: ["native"]
		script: "make build"
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
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
default_runtime: "native"

commands: [
	{
		name: "release"
runtimes: ["native"]
		script: "echo release"
		works_on: {
			hosts: ["linux", "mac"]
		}
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
default_runtime: "native"

commands: [
	{
		name: "build"
runtimes: ["native"]
		script: "make build"
		works_on: {
			hosts: ["linux", "mac"]
		}
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
default_runtime: "native"

commands: [
	{
		name: "deploy"
runtimes: ["native"]
		script: "echo deploying"
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
		depends_on: {
			filepaths: [
				{path: "config.yaml"},
				{path: "secrets.env", readable: true},
				{path: "output", writable: true},
				{path: "scripts/deploy.sh", executable: true},
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
	if fp0.Path != "config.yaml" {
		t.Errorf("First filepath path = %q, want %q", fp0.Path, "config.yaml")
	}
	if fp0.Readable || fp0.Writable || fp0.Executable {
		t.Error("First filepath should have no permission checks")
	}

	// Second filepath - readable
	fp1 := cmd.DependsOn.Filepaths[1]
	if fp1.Path != "secrets.env" {
		t.Errorf("Second filepath path = %q, want %q", fp1.Path, "secrets.env")
	}
	if !fp1.Readable {
		t.Error("Second filepath should be readable")
	}

	// Third filepath - writable
	fp2 := cmd.DependsOn.Filepaths[2]
	if fp2.Path != "output" {
		t.Errorf("Third filepath path = %q, want %q", fp2.Path, "output")
	}
	if !fp2.Writable {
		t.Error("Third filepath should be writable")
	}

	// Fourth filepath - executable
	fp3 := cmd.DependsOn.Filepaths[3]
	if fp3.Path != "scripts/deploy.sh" {
		t.Errorf("Fourth filepath path = %q, want %q", fp3.Path, "scripts/deploy.sh")
	}
	if !fp3.Executable {
		t.Error("Fourth filepath should be executable")
	}
}

func TestCommand_HasDependencies_WithFilepaths(t *testing.T) {
	cmd := Command{
		Name:    "test",
		Script:  "echo",
		WorksOn: WorksOn{Hosts: []HostOS{HostLinux}},
		DependsOn: &DependsOn{
			Filepaths: []FilepathDependency{{Path: "config.yaml"}},
		},
	}

	if !cmd.HasDependencies() {
		t.Error("HasDependencies() should return true when filepaths are present")
	}
}

func TestGenerateCUE_WithFilepaths(t *testing.T) {
	inv := &Invowkfile{
		Version:        "1.0",
		DefaultRuntime: RuntimeNative,
		Commands: []Command{
			{
				Name:    "deploy",
				Script:  "echo deploy",
				WorksOn: WorksOn{Hosts: []HostOS{HostLinux, HostMac}},
				DependsOn: &DependsOn{
					Filepaths: []FilepathDependency{
						{Path: "config.yaml"},
						{Path: "secrets.env", Readable: true},
						{Path: "output", Writable: true},
						{Path: "deploy.sh", Executable: true},
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

	if !strings.Contains(output, `path: "config.yaml"`) {
		t.Error("GenerateCUE should contain config.yaml path")
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

func TestParseWorksOn(t *testing.T) {
	cueContent := `
version: "1.0"
default_runtime: "native"

commands: [
	{
		name: "build"
runtimes: ["native"]
		script: "make build"
		works_on: {
			hosts: ["linux", "mac", "windows"]
		}
	},
	{
		name: "deploy"
runtimes: ["native"]
		script: "deploy.sh"
		works_on: {
			hosts: ["linux"]
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

	if len(inv.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(inv.Commands))
	}

	// First command - all hosts
	cmd1 := inv.Commands[0]
	if len(cmd1.WorksOn.Hosts) != 3 {
		t.Errorf("Expected 3 hosts for first command, got %d", len(cmd1.WorksOn.Hosts))
	}
	if cmd1.WorksOn.Hosts[0] != HostLinux {
		t.Errorf("First host = %q, want %q", cmd1.WorksOn.Hosts[0], HostLinux)
	}

	// Second command - linux only
	cmd2 := inv.Commands[1]
	if len(cmd2.WorksOn.Hosts) != 1 {
		t.Errorf("Expected 1 host for second command, got %d", len(cmd2.WorksOn.Hosts))
	}
	if cmd2.WorksOn.Hosts[0] != HostLinux {
		t.Errorf("First host = %q, want %q", cmd2.WorksOn.Hosts[0], HostLinux)
	}
}

func TestGenerateCUE_WithWorksOn(t *testing.T) {
	inv := &Invowkfile{
		Version:        "1.0",
		DefaultRuntime: RuntimeNative,
		Commands: []Command{
			{
				Name:    "build",
				Script:  "make build",
				WorksOn: WorksOn{Hosts: []HostOS{HostLinux, HostMac, HostWindows}},
			},
			{
				Name:    "clean",
				Script:  "rm -rf bin/",
				WorksOn: WorksOn{Hosts: []HostOS{HostLinux, HostMac}},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check that works_on structure is present
	if !strings.Contains(output, "works_on:") {
		t.Error("GenerateCUE should contain 'works_on:'")
	}

	if !strings.Contains(output, "hosts:") {
		t.Error("GenerateCUE should contain 'hosts:'")
	}

	if !strings.Contains(output, `"linux"`) {
		t.Error("GenerateCUE should contain 'linux'")
	}

	if !strings.Contains(output, `"mac"`) {
		t.Error("GenerateCUE should contain 'mac'")
	}

	if !strings.Contains(output, `"windows"`) {
		t.Error("GenerateCUE should contain 'windows'")
	}
}
