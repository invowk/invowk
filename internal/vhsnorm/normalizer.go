// SPDX-License-Identifier: EPL-2.0

package vhsnorm

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ansiPattern matches ANSI escape sequences.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// Normalizer processes VHS output to produce deterministic, comparable text.
type Normalizer struct {
	cfg          *Config
	rules        []compiledRule
	separatorSet map[rune]struct{}
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

	return &Normalizer{
		cfg:          cfg,
		rules:        rules,
		separatorSet: separatorSet,
	}, nil
}

// Normalize reads from r, normalizes the content, and writes to w.
// The processing pipeline:
//  1. Read all lines
//  2. Strip ANSI escape codes (if enabled)
//  3. Filter VHS artifacts (frame separators, empty prompts)
//  4. Apply substitution rules in order
//  5. Deduplicate consecutive identical lines (if enabled)
//  6. Remove empty lines (if enabled)
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

		// Step 4: Apply substitution rules
		for _, rule := range n.rules {
			line = rule.regex.ReplaceAllString(line, rule.replacement)
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	// Step 5: Deduplicate consecutive identical lines
	if n.cfg.VHSArtifacts.Deduplicate {
		lines = n.deduplicate(lines)
	}

	// Step 6: Remove empty lines (if enabled)
	if n.cfg.Filters.StripEmpty {
		lines = n.removeEmpty(lines)
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
