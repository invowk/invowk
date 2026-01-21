// SPDX-License-Identifier: EPL-2.0

package vhsnorm

import (
	"slices"
	"strings"
	"testing"
)

func TestNewNormalizer(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		n, err := NewNormalizer(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n == nil {
			t.Error("normalizer should not be nil")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := &Config{
			Substitutions: []SubstitutionRule{
				{Name: "bad", Pattern: `[invalid`, Replacement: "x"},
			},
		}
		_, err := NewNormalizer(cfg)
		if err == nil {
			t.Error("expected error for invalid config")
		}
	})
}

func TestNormalizerIsFrameSeparator(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"full line of dashes", "────────────────────────────────────────", true},
		{"mixed box chars", "━━━━━━━━━━━━━━━━━━━━", true},
		{"double lines", "════════════════════", true},
		{"empty line", "", false},
		{"whitespace only", "   ", false},
		{"prompt line", "> ls", false},
		{"text with separator", "Hello───World", false},
		{"normal text", "Hello World", false},
		{"command output", "drwxr-xr-x  2 user user 4096", false},
		{"separator with spaces", "  ────────  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.isFrameSeparator(tt.line)
			if result != tt.expected {
				t.Errorf("isFrameSeparator(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestNormalizerIsEmptyPrompt(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"just prompt", ">", true},
		{"prompt with space", "> ", true},
		{"prompt with multiple spaces", ">   ", true},
		{"prompt with leading space", " >", true},
		{"prompt with text", "> ls", false},
		{"different char", "$", false},
		{"empty line", "", false},
		{"double prompt", ">>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.isEmptyPrompt(tt.line)
			if result != tt.expected {
				t.Errorf("isEmptyPrompt(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestNormalizerStripANSI(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no ANSI", "Hello World", "Hello World"},
		{"bold text", "\x1b[1mBold\x1b[0m", "Bold"},
		{"colored text", "\x1b[31mRed\x1b[0m", "Red"},
		{"multiple codes", "\x1b[1;31mBold Red\x1b[0m Normal", "Bold Red Normal"},
		{"cursor movement", "\x1b[2JCleared", "Cleared"},
		{"mixed content", "Normal \x1b[32mGreen\x1b[0m Normal", "Normal Green Normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.stripANSI(tt.input)
			if result != tt.expected {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizerDeduplicate(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		lines    []string
		expected []string
	}{
		{
			"no duplicates",
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"consecutive duplicates",
			[]string{"a", "a", "b", "b", "b", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"non-consecutive duplicates",
			[]string{"a", "b", "a", "b"},
			[]string{"a", "b", "a", "b"},
		},
		{
			"all same",
			[]string{"x", "x", "x"},
			[]string{"x"},
		},
		{
			"empty slice",
			[]string{},
			[]string{},
		},
		{
			"single item",
			[]string{"only"},
			[]string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.deduplicate(tt.lines)
			if len(result) != len(tt.expected) {
				t.Errorf("deduplicate() returned %d items, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("deduplicate()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestNormalizerRemoveEmpty(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		lines    []string
		expected []string
	}{
		{
			"no empty lines",
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"some empty lines",
			[]string{"a", "", "b", "", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"whitespace lines",
			[]string{"a", "   ", "b", "\t", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"all empty",
			[]string{"", "", ""},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.removeEmpty(tt.lines)
			if len(result) != len(tt.expected) {
				t.Errorf("removeEmpty() returned %d items, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("removeEmpty()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestNormalizerIsUnstableFrame(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		// Clean lines that should NOT be detected as unstable
		{"clean command", "> ./bin/invowk cmd hello", false},
		{"clean output", "Hello from invowk!", false},
		{"clean prompt only", "> ", false},
		{"empty line", "", false},
		{"normal camelCase command", "> ./bin/invowk cmd camelCase", false},
		{"regular path", "./bin/invowk version", false},
		{"path with lowercase", "./bin/hello", false},
		{"non-prompt with uppercase", "PATH=/bin:/usr/bin", false},

		// Unstable frames that SHOULD be detected
		{"path-output bleed: Hello", "./bin/Hello from invowk!", true},
		{"path-output bleed: World", "./bin/World output here", true},
		{"path-output bleed: Error", "./bin/Error: something wrong", true},
		{"prompt merge: helloHello from", "> ./bin/invowk cmd helloHello from", true},
		{"prompt merge: testTest from", "> ./bin/invowk testTest from invowk", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.isUnstableFrame(tt.line)
			if result != tt.expected {
				t.Errorf("isUnstableFrame(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestNormalizerWithUnstableFrames(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VHSArtifacts.StripPromptLines = false // Disable prompt stripping for this test
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate the CI failure case: intermixed command/output content
	input := `> ./bin/invowk cmd hello
./bin/Hello from invowk!
> ./bin/invowk cmd hello
Hello from invowk!
`

	// Expected: the corrupted frame "./bin/Hello from invowk!" is removed
	expected := `> ./bin/invowk cmd hello
Hello from invowk!
`

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result) != strings.TrimSpace(expected) {
		t.Errorf("Unstable frame filtering failed.\nGot:\n%s\nWant:\n%s", result, expected)
	}
}

func TestNormalizerUnstableFramesDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VHSArtifacts.StripUnstableFrames = false
	cfg.VHSArtifacts.StripPromptLines = false // Disable prompt stripping for this test
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When disabled, the corrupted frame should be preserved
	input := `> ./bin/invowk cmd hello
./bin/Hello from invowk!
Hello from invowk!
`

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With filtering disabled, the unstable frame should remain
	if !strings.Contains(result, "./bin/Hello from invowk!") {
		t.Errorf("With unstable frame filtering disabled, corrupted frames should remain.\nGot: %q", result)
	}
}

func TestNormalizerSubstitutions(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			"ISO timestamp",
			"Started at 2024-01-15T10:30:45Z",
			"[TIMESTAMP]",
		},
		{
			"Linux home path",
			"/home/testuser/.config",
			"[HOME]",
		},
		{
			"Var home path",
			"/var/home/testuser/.config",
			"[HOME]",
		},
		{
			"macOS home path",
			"/Users/testuser/.config",
			"[HOME]",
		},
		{
			"tmp path",
			"/tmp/test-123.txt",
			"[TMPDIR]",
		},
		{
			"var tmp path",
			"/var/tmp/session.abc",
			"[TMPDIR]",
		},
		{
			"hostname",
			"hostname: myhost.local",
			"hostname: [HOSTNAME]",
		},
		{
			"version with v prefix",
			"invowk v1.2.3-beta",
			"invowk [VERSION]",
		},
		{
			"version word",
			"invowk version 1.2.3",
			"invowk version [VERSION]",
		},
		{
			"env USER",
			"USER = 'testuser'",
			"USER = '[USER]'",
		},
		{
			"env HOME",
			"HOME = '/home/testuser'",
			"HOME = '[HOME]'",
		},
		{
			"env PATH",
			"PATH = '/usr/bin:/bin'",
			"PATH = '[PATH]'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := n.NormalizeString(tt.input + "\n")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result, tt.contains) {
				t.Errorf("NormalizeString(%q) = %q, should contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestNormalizerFullPipeline(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate actual VHS output with all artifacts
	// Note: VHS captures progressive frames as typing happens, so we get
	// repeated commands at different stages. The normalizer removes prompt lines
	// entirely (to eliminate timing-dependent duplication), frame separators,
	// and consecutive duplicates.
	input := `>
────────────────────────────────────────────────────────────────────────────────
>
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd hello
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd hello
Hello from invowk!
>
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd hello
Hello from invowk!
> ./bin/invowk version
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd hello
Hello from invowk!
> ./bin/invowk version
invowk v1.2.3
`

	// Expected: prompt lines removed, artifacts removed, consecutive duplicates collapsed,
	// substitutions applied (version normalized)
	expected := `Hello from invowk!
invowk [VERSION]
`

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Full pipeline result mismatch.\nGot:\n%s\nWant:\n%s", result, expected)
	}
}

func TestNormalizerWithANSICodes(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Prompt lines are stripped (after ANSI codes are removed), so only output remains
	input := "\x1b[1m> \x1b[32m./bin/invowk\x1b[0m cmd hello\nHello World!\n"
	expected := "Hello World!\n"

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("ANSI stripping failed.\nGot:\n%q\nWant:\n%q", result, expected)
	}
}

func TestNormalizerDisabledFeatures(t *testing.T) {
	t.Run("deduplication disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.VHSArtifacts.Deduplicate = false
		n, err := NewNormalizer(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		input := "line1\nline1\nline2\n"
		result, err := n.NormalizeString(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// With deduplication disabled, duplicates should remain
		if !strings.Contains(result, "line1\nline1\n") {
			t.Errorf("With dedup disabled, duplicate lines should remain. Got: %q", result)
		}
	})

	t.Run("strip empty disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Filters.StripEmpty = false
		cfg.VHSArtifacts.StripEmptyPrompts = false
		cfg.VHSArtifacts.StripFrameSeparators = false
		cfg.VHSArtifacts.Deduplicate = false
		n, err := NewNormalizer(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		input := "line1\n\nline2\n"
		result, err := n.NormalizeString(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// With strip empty disabled, empty lines should remain
		lines := strings.Split(result, "\n")
		if !slices.Contains(lines, "") {
			t.Errorf("With strip empty disabled, empty lines should remain. Got: %q", result)
		}
	})

	t.Run("frame separators disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.VHSArtifacts.StripFrameSeparators = false
		cfg.VHSArtifacts.Deduplicate = false
		n, err := NewNormalizer(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		input := "line1\n────────────────────────\nline2\n"
		result, err := n.NormalizeString(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// With frame separator stripping disabled, separators should remain
		if !strings.Contains(result, "────") {
			t.Errorf("With frame separator stripping disabled, separators should remain. Got: %q", result)
		}
	})
}

func TestNormalizerCustomPromptChar(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VHSArtifacts.PromptChar = "$"
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ">" should NOT be treated as empty prompt with custom config
	if n.isEmptyPrompt(">") {
		t.Error("'>' should not be empty prompt when prompt_char is '$'")
	}

	// "$" SHOULD be treated as empty prompt
	if !n.isEmptyPrompt("$") {
		t.Error("'$' should be empty prompt when prompt_char is '$'")
	}
}

func TestNormalizerRealisticVHSOutput(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Real-world VHS output sample
	input := `>
────────────────────────────────────────────────────────────────────────────────
>
────────────────────────────────────────────────────────────────────────────────
>
────────────────────────────────────────────────────────────────────────────────
>
────────────────────────────────────────────────────────────────────────────────
>
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd 'env files basic'
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd 'env files basic'
────────────────────────────────────────────────────────────────────────────────
> ./bin/invowk cmd 'env files basic'
==========================================
  Basic env.files Demo
==========================================
Variables loaded from examples/.env:
  APP_NAME    = 'invowk-demo'
  APP_VERSION = '1.0.0'
  APP_ENV     = 'development'
  ENABLE_DEBUG= 'true'
  LOG_LEVEL   = 'info'
==========================================
>
────────────────────────────────────────────────────────────────────────────────
`

	// Expected: prompt lines stripped, only command output remains
	expected := `==========================================
  Basic env.files Demo
==========================================
Variables loaded from examples/.env:
  APP_NAME    = 'invowk-demo'
  APP_VERSION = '1.0.0'
  APP_ENV     = 'development'
  ENABLE_DEBUG= 'true'
  LOG_LEVEL   = 'info'
==========================================
`

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Realistic VHS output normalization failed.\nGot:\n%s\nWant:\n%s", result, expected)
	}
}

func TestNormalizerIsPromptLine(t *testing.T) {
	cfg := DefaultConfig()
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		// Lines that SHOULD be detected as prompt lines
		{"simple command", "> ./bin/invowk cmd hello", true},
		{"command with quotes", "> ./bin/invowk cmd 'env hierarchy'", true},
		{"command with leading space", "  > ./bin/invowk cmd hello", true},
		{"multiple spaces after prompt", ">   ./bin/invowk cmd hello", true},
		{"ls command", "> ls -la", true},
		{"echo command", "> echo hello", true},

		// Lines that should NOT be detected as prompt lines
		{"output only", "Hello from invowk!", false},
		{"empty prompt", ">", false},
		// Note: ">  " technically matches the pattern, but it's already handled by isEmptyPrompt
		// in the full pipeline, so this edge case doesn't matter in practice.
		{"prompt with only spaces", ">  ", true}, // matches pattern (handled by isEmptyPrompt first)
		{"command output with >", "redirect > file.txt", false},
		{"empty line", "", false},
		{"frame separator", "────────────────────────────────", false},
		{"version output", "invowk v1.2.3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.isPromptLine(tt.line)
			if result != tt.expected {
				t.Errorf("isPromptLine(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestNormalizerStripPromptLinesDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VHSArtifacts.StripPromptLines = false
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When disabled, prompt lines should be preserved
	input := `> ./bin/invowk cmd hello
Hello from invowk!
> ./bin/invowk version
invowk v1.2.3
`

	expected := `> ./bin/invowk cmd hello
Hello from invowk!
> ./bin/invowk version
invowk [VERSION]
`

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("With prompt line stripping disabled, prompt lines should remain.\nGot:\n%s\nWant:\n%s", result, expected)
	}
}

func TestNormalizerCustomPromptLinePattern(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VHSArtifacts.StripPromptLines = true
	cfg.VHSArtifacts.PromptLinePattern = `^\$\s+.*$` // Match "$ command" style prompts
	n, err := NewNormalizer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With custom pattern, only "$ ..." lines should be stripped
	input := `$ echo hello
Hello
> ./bin/invowk cmd hello
Hello from invowk!
`

	// "> ./bin/invowk cmd hello" should remain because pattern matches "$" not ">"
	expected := `Hello
> ./bin/invowk cmd hello
Hello from invowk!
`

	result, err := n.NormalizeString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Custom prompt pattern not working correctly.\nGot:\n%s\nWant:\n%s", result, expected)
	}
}
