// SPDX-License-Identifier: MPL-2.0

package vhsnorm

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	// ansiPattern matches ANSI escape sequences.
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// unstableFramePathOutputPattern detects lines where a path (like "./bin/") is immediately
	// followed by an uppercase letter, indicating command/output content bleeding together.
	// Example: "./bin/Hello from invowk!" instead of separate lines.
	unstableFramePathOutputPattern = regexp.MustCompile(`^\./bin/[A-Z]`)

	// unstableFramePromptMergePattern detects lines with lowercase-to-uppercase transitions
	// that suggest output bleeding into a prompt line.
	// Example: "> ./bin/invowk cmd helloHello from" (command merged with output).
	unstableFramePromptMergePattern = regexp.MustCompile(`[a-z][A-Z][a-z]+ from`)
)

// Normalizer processes VHS output to produce deterministic, comparable text.
type Normalizer struct {
	cfg             *Config
	rules           []compiledRule
	separatorSet    map[rune]struct{}
	promptLineRegex *regexp.Regexp // Compiled prompt line pattern (nil if not enabled)
}

// NewNormalizer creates a new normalizer with the given configuration.
// It compiles all regex patterns upfront for efficiency.
func NewNormalizer(cfg *Config) (*Normalizer, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Compile substitution rules
	rules := make([]compiledRule, len(cfg.Substitutions))
	for i, rule := range cfg.Substitutions {
		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			// This shouldn't happen since ValidateConfig already checks,
			// but handle it gracefully
			return nil, fmt.Errorf("failed to compile pattern %q: %w", rule.Name, err)
		}
		rules[i] = compiledRule{
			name:        rule.Name,
			regex:       regex,
			replacement: rule.Replacement,
		}
	}

	// Build separator character set for fast lookup
	separatorSet := make(map[rune]struct{})
	for _, char := range cfg.VHSArtifacts.SeparatorChars {
		for _, r := range char {
			separatorSet[r] = struct{}{}
		}
	}

	// Compile prompt line pattern if enabled
	var promptLineRegex *regexp.Regexp
	if cfg.VHSArtifacts.StripPromptLines && cfg.VHSArtifacts.PromptLinePattern != "" {
		// Pattern already validated by ValidateConfig
		promptLineRegex = regexp.MustCompile(cfg.VHSArtifacts.PromptLinePattern)
	}

	return &Normalizer{
		cfg:             cfg,
		rules:           rules,
		separatorSet:    separatorSet,
		promptLineRegex: promptLineRegex,
	}, nil
}

// Normalize reads from r, normalizes the content, and writes to w.
// The processing pipeline:
//  1. Read all lines
//  2. Strip ANSI escape codes (if enabled)
//  3. Filter VHS artifacts (frame separators, empty prompts, unstable frames, prompt lines)
//  4. Apply substitution rules in order
//  5. Remove empty lines (if enabled) - before deduplication to ensure proper consecutive detection
//  6. Deduplicate consecutive identical lines (if enabled)
//  7. Write output
func (n *Normalizer) Normalize(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	var lines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Step 2: Strip ANSI escape codes
		if n.cfg.Filters.StripANSI {
			line = n.stripANSI(line)
		}

		// Step 3: Filter VHS artifacts
		if n.cfg.VHSArtifacts.StripFrameSeparators && n.isFrameSeparator(line) {
			continue
		}
		if n.cfg.VHSArtifacts.StripEmptyPrompts && n.isEmptyPrompt(line) {
			continue
		}
		if n.cfg.VHSArtifacts.StripUnstableFrames && n.isUnstableFrame(line) {
			continue
		}
		if n.cfg.VHSArtifacts.StripPromptLines && n.isPromptLine(line) {
			continue
		}

		// Step 4: Apply substitution rules
		for _, rule := range n.rules {
			line = rule.regex.ReplaceAllString(line, rule.replacement)
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	// Step 5: Remove empty lines (if enabled) - BEFORE deduplication
	// This ensures empty lines don't prevent consecutive identical lines from being deduplicated
	if n.cfg.Filters.StripEmpty {
		lines = n.removeEmpty(lines)
	}

	// Step 6: Deduplicate consecutive identical lines
	if n.cfg.VHSArtifacts.Deduplicate {
		lines = n.deduplicate(lines)
	}

	// Step 7: Write output
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("error writing output: %w", err)
		}
	}

	return nil
}

// NormalizeString is a convenience method that normalizes a string.
func (n *Normalizer) NormalizeString(input string) (string, error) {
	var buf strings.Builder
	if err := n.Normalize(strings.NewReader(input), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// stripANSI removes ANSI escape sequences from a line.
func (n *Normalizer) stripANSI(line string) string {
	return ansiPattern.ReplaceAllString(line, "")
}

// isFrameSeparator returns true if the line consists only of box-drawing characters.
// This detects VHS frame dividers like "────────────────────────────────────────".
func (n *Normalizer) isFrameSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	for _, r := range trimmed {
		if _, ok := n.separatorSet[r]; !ok {
			return false
		}
	}
	return true
}

// isEmptyPrompt returns true if the trimmed line equals just the prompt character.
// This detects VHS artifacts like "> " (empty prompt lines).
func (n *Normalizer) isEmptyPrompt(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == n.cfg.VHSArtifacts.PromptChar
}

// isUnstableFrame detects lines with intermixed command/output content.
// This happens when VHS captures a frame while the terminal is in a transitional state,
// causing the command text to bleed into the output line.
//
// Detection patterns:
//  1. Path followed by uppercase: "./bin/Hello from invowk!" (path bleeding into output)
//  2. Prompt with merged content: "> ./bin/invowk cmd helloHello from" (command merged with output)
func (n *Normalizer) isUnstableFrame(line string) bool {
	// Pattern 1: Standalone path followed by uppercase (output bleeding)
	// Matches: "./bin/Hello", "./bin/invowk Hello", etc.
	if unstableFramePathOutputPattern.MatchString(line) {
		return true
	}

	// Pattern 2: Prompt line with partial command merged with uppercase output
	// Matches: "> ./bin/invowk cmd helloHello from"
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "> ") {
		// Check for lowercase-to-uppercase transition indicating output bleeding in
		if unstableFramePromptMergePattern.MatchString(line) {
			return true
		}
	}

	return false
}

// isPromptLine returns true if the line matches the prompt line pattern.
// This strips all command lines (e.g., "> ./bin/invowk cmd hello") to eliminate
// timing-dependent duplication from VHS frame capture.
func (n *Normalizer) isPromptLine(line string) bool {
	if n.promptLineRegex == nil {
		return false
	}
	return n.promptLineRegex.MatchString(line)
}

// deduplicate removes consecutive duplicate lines.
// Unlike shell `uniq`, this operates on the entire slice for better results.
func (n *Normalizer) deduplicate(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	result := make([]string, 0, len(lines))
	result = append(result, lines[0])

	for i := 1; i < len(lines); i++ {
		if lines[i] != lines[i-1] {
			result = append(result, lines[i])
		}
	}

	return result
}

// removeEmpty removes empty or whitespace-only lines.
func (n *Normalizer) removeEmpty(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}
