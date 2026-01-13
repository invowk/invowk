package invkfile

import (
	"reflect"
	"testing"
)

func TestParseShebang(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected ShebangInfo
	}{
		// Basic shebangs
		{
			name:    "bash shebang",
			content: "#!/bin/bash\necho hello",
			expected: ShebangInfo{
				Interpreter: "/bin/bash",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "sh shebang",
			content: "#!/bin/sh\necho hello",
			expected: ShebangInfo{
				Interpreter: "/bin/sh",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "shebang with space after #!",
			content: "#! /bin/sh\necho hello",
			expected: ShebangInfo{
				Interpreter: "/bin/sh",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "shebang with interpreter args",
			content: "#!/usr/bin/perl -w\nprint 'hello';",
			expected: ShebangInfo{
				Interpreter: "/usr/bin/perl",
				Args:        []string{"-w"},
				Found:       true,
			},
		},
		{
			name:    "shebang with multiple interpreter args",
			content: "#!/usr/bin/perl -w -T\nprint 'hello';",
			expected: ShebangInfo{
				Interpreter: "/usr/bin/perl",
				Args:        []string{"-w", "-T"},
				Found:       true,
			},
		},

		// env-based shebangs
		{
			name:    "env python3",
			content: "#!/usr/bin/env python3\nprint('hello')",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "env ruby",
			content: "#!/usr/bin/env ruby\nputs 'hello'",
			expected: ShebangInfo{
				Interpreter: "ruby",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "env node",
			content: "#!/usr/bin/env node\nconsole.log('hello')",
			expected: ShebangInfo{
				Interpreter: "node",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "/bin/env python3",
			content: "#!/bin/env python3\nprint('hello')",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},

		// env with -S flag (split string mode)
		{
			name:    "env -S with interpreter args",
			content: "#!/usr/bin/env -S python3 -u\nprint('hello')",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u"},
				Found:       true,
			},
		},
		{
			name:    "env -S with multiple interpreter args",
			content: "#!/usr/bin/env -S python3 -u -B\nprint('hello')",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u", "-B"},
				Found:       true,
			},
		},
		{
			name:    "env -S node with args",
			content: "#!/usr/bin/env -S node --experimental-modules\nconsole.log('hello')",
			expected: ShebangInfo{
				Interpreter: "node",
				Args:        []string{"--experimental-modules"},
				Found:       true,
			},
		},

		// No shebang / invalid cases
		{
			name:    "no shebang",
			content: "echo hello\nworld",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name:    "empty content",
			content: "",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name:    "comment but not shebang",
			content: "# This is a comment\necho hello",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name:    "shebang-like but not at start",
			content: "echo hello\n#!/bin/bash",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name:    "empty shebang",
			content: "#!\necho hello",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name:    "env without interpreter",
			content: "#!/usr/bin/env\necho hello",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name:    "env -S without interpreter",
			content: "#!/usr/bin/env -S\necho hello",
			expected: ShebangInfo{
				Found: false,
			},
		},

		// Edge cases
		{
			name:    "Windows line endings",
			content: "#!/bin/bash\r\necho hello",
			expected: ShebangInfo{
				Interpreter: "/bin/bash",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "single line script with shebang",
			content: "#!/bin/bash",
			expected: ShebangInfo{
				Interpreter: "/bin/bash",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:    "shebang with tabs",
			content: "#!/bin/bash\t-x",
			expected: ShebangInfo{
				Interpreter: "/bin/bash",
				Args:        []string{"-x"},
				Found:       true,
			},
		},
		{
			name:    "shebang with extra whitespace",
			content: "#!   /bin/bash   -x   \necho hello",
			expected: ShebangInfo{
				Interpreter: "/bin/bash",
				Args:        []string{"-x"},
				Found:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseShebang(tt.content)

			if result.Found != tt.expected.Found {
				t.Errorf("ParseShebang() Found = %v, want %v", result.Found, tt.expected.Found)
				return
			}

			if result.Found {
				if result.Interpreter != tt.expected.Interpreter {
					t.Errorf("ParseShebang() Interpreter = %q, want %q", result.Interpreter, tt.expected.Interpreter)
				}

				if !reflect.DeepEqual(result.Args, tt.expected.Args) {
					// Handle nil vs empty slice
					if len(result.Args) == 0 && len(tt.expected.Args) == 0 {
						return
					}
					t.Errorf("ParseShebang() Args = %v, want %v", result.Args, tt.expected.Args)
				}
			}
		})
	}
}

func TestParseInterpreterString(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		expected ShebangInfo
	}{
		// Basic interpreters
		{
			name: "simple interpreter",
			spec: "python3",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name: "interpreter with path",
			spec: "/usr/bin/python3",
			expected: ShebangInfo{
				Interpreter: "/usr/bin/python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name: "interpreter with args",
			spec: "python3 -u",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u"},
				Found:       true,
			},
		},
		{
			name: "interpreter with multiple args",
			spec: "python3 -u -B",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u", "-B"},
				Found:       true,
			},
		},
		{
			name: "interpreter with path and args",
			spec: "/usr/bin/python3 -u -B",
			expected: ShebangInfo{
				Interpreter: "/usr/bin/python3",
				Args:        []string{"-u", "-B"},
				Found:       true,
			},
		},

		// env-based specifications
		{
			name: "env python3",
			spec: "/usr/bin/env python3",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name: "env shorthand",
			spec: "env python3",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name: "env with args",
			spec: "/usr/bin/env python3 -u",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u"},
				Found:       true,
			},
		},

		// Empty / auto cases
		{
			name: "empty string",
			spec: "",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name: "whitespace only",
			spec: "   ",
			expected: ShebangInfo{
				Found: false,
			},
		},
		{
			name: "auto keyword",
			spec: "auto",
			expected: ShebangInfo{
				Found: false,
			},
		},

		// Edge cases
		{
			name: "interpreter with extra whitespace",
			spec: "  python3   -u  ",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u"},
				Found:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseInterpreterString(tt.spec)

			if result.Found != tt.expected.Found {
				t.Errorf("ParseInterpreterString(%q) Found = %v, want %v", tt.spec, result.Found, tt.expected.Found)
				return
			}

			if result.Found {
				if result.Interpreter != tt.expected.Interpreter {
					t.Errorf("ParseInterpreterString(%q) Interpreter = %q, want %q", tt.spec, result.Interpreter, tt.expected.Interpreter)
				}

				if !reflect.DeepEqual(result.Args, tt.expected.Args) {
					// Handle nil vs empty slice
					if len(result.Args) == 0 && len(tt.expected.Args) == 0 {
						return
					}
					t.Errorf("ParseInterpreterString(%q) Args = %v, want %v", tt.spec, result.Args, tt.expected.Args)
				}
			}
		})
	}
}

func TestIsShellInterpreter(t *testing.T) {
	tests := []struct {
		interpreter string
		expected    bool
	}{
		// Shell interpreters
		{"/bin/sh", true},
		{"/bin/bash", true},
		{"/usr/bin/bash", true},
		{"bash", true},
		{"sh", true},
		{"zsh", true},
		{"/bin/zsh", true},
		{"dash", true},
		{"ash", true},
		{"ksh", true},
		{"mksh", true},

		// Non-shell interpreters
		{"python3", false},
		{"/usr/bin/python3", false},
		{"ruby", false},
		{"perl", false},
		{"node", false},
		{"php", false},

		// Edge cases
		{"bash.exe", true}, // Windows
		{"", false},
		{"fish", false}, // fish is not POSIX-compatible
		{"pwsh", false}, // PowerShell is not POSIX-compatible
	}

	for _, tt := range tests {
		t.Run(tt.interpreter, func(t *testing.T) {
			result := IsShellInterpreter(tt.interpreter)
			if result != tt.expected {
				t.Errorf("IsShellInterpreter(%q) = %v, want %v", tt.interpreter, result, tt.expected)
			}
		})
	}
}

func TestGetExtensionForInterpreter(t *testing.T) {
	tests := []struct {
		interpreter string
		expected    string
	}{
		// Python
		{"python3", ".py"},
		{"python", ".py"},
		{"python2", ".py"},
		{"/usr/bin/python3", ".py"},

		// Ruby
		{"ruby", ".rb"},
		{"/usr/bin/ruby", ".rb"},

		// Perl
		{"perl", ".pl"},

		// Node.js
		{"node", ".js"},

		// Shell
		{"bash", ".sh"},
		{"sh", ".sh"},
		{"zsh", ".zsh"},

		// PowerShell
		{"pwsh", ".ps1"},
		{"powershell", ".ps1"},

		// Other
		{"fish", ".fish"},
		{"php", ".php"},
		{"lua", ".lua"},
		{"Rscript", ".R"},

		// Unknown
		{"unknown", ""},
		{"", ""},

		// Windows executables
		{"python3.exe", ".py"},
		{"bash.exe", ".sh"},
	}

	for _, tt := range tests {
		t.Run(tt.interpreter, func(t *testing.T) {
			result := GetExtensionForInterpreter(tt.interpreter)
			if result != tt.expected {
				t.Errorf("GetExtensionForInterpreter(%q) = %q, want %q", tt.interpreter, result, tt.expected)
			}
		})
	}
}

func TestResolveInterpreter(t *testing.T) {
	tests := []struct {
		name          string
		interpreter   string
		scriptContent string
		expected      ShebangInfo
	}{
		// Empty interpreter (defaults to auto)
		{
			name:          "empty interpreter with shebang",
			interpreter:   "",
			scriptContent: "#!/usr/bin/env python3\nprint('hello')",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:          "empty interpreter without shebang",
			interpreter:   "",
			scriptContent: "echo hello world",
			expected: ShebangInfo{
				Found: false,
			},
		},

		// Explicit "auto"
		{
			name:          "auto with shebang",
			interpreter:   "auto",
			scriptContent: "#!/bin/bash\necho hello",
			expected: ShebangInfo{
				Interpreter: "/bin/bash",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:          "auto without shebang",
			interpreter:   "auto",
			scriptContent: "echo hello",
			expected: ShebangInfo{
				Found: false,
			},
		},

		// Explicit interpreter (ignores shebang)
		{
			name:          "explicit interpreter ignores shebang",
			interpreter:   "python3",
			scriptContent: "#!/bin/bash\necho hello",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{},
				Found:       true,
			},
		},
		{
			name:          "explicit interpreter with args",
			interpreter:   "python3 -u",
			scriptContent: "#!/bin/bash\necho hello",
			expected: ShebangInfo{
				Interpreter: "python3",
				Args:        []string{"-u"},
				Found:       true,
			},
		},
		{
			name:          "explicit full path interpreter",
			interpreter:   "/usr/local/bin/python3",
			scriptContent: "print('hello')",
			expected: ShebangInfo{
				Interpreter: "/usr/local/bin/python3",
				Args:        []string{},
				Found:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveInterpreter(tt.interpreter, tt.scriptContent)

			if result.Found != tt.expected.Found {
				t.Errorf("ResolveInterpreter(%q, ...) Found = %v, want %v", tt.interpreter, result.Found, tt.expected.Found)
				return
			}

			if result.Found {
				if result.Interpreter != tt.expected.Interpreter {
					t.Errorf("ResolveInterpreter(%q, ...) Interpreter = %q, want %q", tt.interpreter, result.Interpreter, tt.expected.Interpreter)
				}

				if !reflect.DeepEqual(result.Args, tt.expected.Args) {
					// Handle nil vs empty slice
					if len(result.Args) == 0 && len(tt.expected.Args) == 0 {
						return
					}
					t.Errorf("ResolveInterpreter(%q, ...) Args = %v, want %v", tt.interpreter, result.Args, tt.expected.Args)
				}
			}
		})
	}
}
