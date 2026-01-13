package invowkfile

import (
	"os"
	"path/filepath"
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
	invowkfilePath := "/home/user/project/invowkfile.toml"

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
	// Test that TOML multi-line strings are properly parsed
	tomlContent := `
version = "1.0"
default_runtime = "native"

[[commands]]
name = "multiline-test"
description = "Test multi-line script"
script = '''
#!/bin/bash
set -e
echo "Line 1"
echo "Line 2"
echo "Line 3"
'''
`

	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")
	if err := os.WriteFile(invowkfilePath, []byte(tomlContent), 0644); err != nil {
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
	expectedScript := "#!/bin/bash\nset -e\necho \"Line 1\"\necho \"Line 2\"\necho \"Line 3\"\n"
	if cmd.Script != expectedScript {
		t.Errorf("Multi-line script parsing failed.\nGot: %q\nWant: %q", cmd.Script, expectedScript)
	}

	// Verify resolution works too
	resolved, err := cmd.ResolveScript(invowkfilePath)
	if err != nil {
		t.Errorf("ResolveScript() error = %v", err)
	}
	if resolved != expectedScript {
		t.Errorf("ResolveScript() = %q, want %q", resolved, expectedScript)
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
