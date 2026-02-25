// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// BaselineConfig holds accepted findings loaded from a baseline TOML file.
// Findings present in the baseline are suppressed during analysis, so only
// new regressions are reported. Use loadBaseline to parse from disk and
// writeBaseline to generate from collected findings.
type BaselineConfig struct {
	Primitive           BaselineCategory `toml:"primitive"`
	MissingIsValid      BaselineCategory `toml:"missing-isvalid"`
	MissingStringer     BaselineCategory `toml:"missing-stringer"`
	MissingConstructor  BaselineCategory `toml:"missing-constructor"`
	WrongConstructorSig BaselineCategory `toml:"wrong-constructor-sig"`
	MissingFuncOptions  BaselineCategory `toml:"missing-func-options"`
	MissingImmutability BaselineCategory `toml:"missing-immutability"`

	// lookup is an O(1) index built after loading. Keyed by category string,
	// each value is the set of accepted message strings for that category.
	lookup map[string]map[string]bool
}

// BaselineCategory holds the accepted diagnostic messages for one category.
type BaselineCategory struct {
	Messages []string `toml:"messages"`
}

// loadBaseline reads and parses a baseline TOML file, building an internal
// lookup index for fast Contains checks.
//
// Returns an empty baseline (matches nothing) if path is empty or the file
// does not exist. This graceful fallback lets old PRs without a baseline
// file pass the check.
func loadBaseline(path string) (*BaselineConfig, error) {
	if path == "" {
		return emptyBaseline(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyBaseline(), nil
		}
		return nil, fmt.Errorf("reading baseline: %w", err)
	}

	var cfg BaselineConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing baseline TOML: %w", err)
	}

	cfg.buildLookup()

	return &cfg, nil
}

// Contains reports whether a finding with the given category and message
// is present in the baseline (i.e., is a known/accepted finding).
func (b *BaselineConfig) Contains(category, message string) bool {
	if b == nil || b.lookup == nil {
		return false
	}
	msgs, ok := b.lookup[category]
	if !ok {
		return false
	}
	return msgs[message]
}

// Count returns the total number of baseline entries across all categories.
func (b *BaselineConfig) Count() int {
	if b == nil {
		return 0
	}
	return len(b.Primitive.Messages) +
		len(b.MissingIsValid.Messages) +
		len(b.MissingStringer.Messages) +
		len(b.MissingConstructor.Messages) +
		len(b.WrongConstructorSig.Messages) +
		len(b.MissingFuncOptions.Messages) +
		len(b.MissingImmutability.Messages)
}

// buildLookup populates the internal lookup maps from the parsed TOML data.
func (b *BaselineConfig) buildLookup() {
	b.lookup = map[string]map[string]bool{
		CategoryPrimitive:           toSet(b.Primitive.Messages),
		CategoryMissingIsValid:      toSet(b.MissingIsValid.Messages),
		CategoryMissingStringer:     toSet(b.MissingStringer.Messages),
		CategoryMissingConstructor:  toSet(b.MissingConstructor.Messages),
		CategoryWrongConstructorSig: toSet(b.WrongConstructorSig.Messages),
		CategoryMissingFuncOptions:  toSet(b.MissingFuncOptions.Messages),
		CategoryMissingImmutability: toSet(b.MissingImmutability.Messages),
	}
}

// WriteBaseline writes a baseline TOML file from categorized findings.
// The findings map is keyed by category constant, with sorted message slices.
// Empty categories are omitted from the output.
func WriteBaseline(path string, findings map[string][]string) error {
	var sb strings.Builder

	sb.WriteString("# SPDX-License-Identifier: MPL-2.0\n")
	sb.WriteString("#\n")
	sb.WriteString("# primitivelint baseline — accepted DDD compliance findings\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().UTC().Format("2006-01-02")))
	sb.WriteString("# Regenerate: make update-baseline\n")

	// Count total findings for the header comment.
	total := 0
	for _, msgs := range findings {
		total += len(msgs)
	}
	sb.WriteString(fmt.Sprintf("# Total: %d findings\n", total))

	// Write each category as a TOML section with a sorted messages array.
	// Order matches the diagnostic category constants for consistency.
	categories := []struct {
		key   string
		label string
	}{
		{CategoryPrimitive, "Bare primitive type usage"},
		{CategoryMissingIsValid, "Named types missing IsValid() method"},
		{CategoryMissingStringer, "Named types missing String() method"},
		{CategoryMissingConstructor, "Exported structs missing NewXxx() constructor"},
		{CategoryWrongConstructorSig, "Constructors with wrong return type"},
		{CategoryMissingFuncOptions, "Structs missing functional options pattern"},
		{CategoryMissingImmutability, "Structs with constructor but exported mutable fields"},
	}

	for _, cat := range categories {
		msgs := findings[cat.key]
		if len(msgs) == 0 {
			continue
		}

		// Sort for stable diffs.
		slices.Sort(msgs)

		sb.WriteString(fmt.Sprintf("\n# %s\n", cat.label))
		sb.WriteString(fmt.Sprintf("[%s]\n", cat.key))
		sb.WriteString("messages = [\n")
		for _, msg := range msgs {
			// quote() produces a TOML-compatible double-quoted string with
			// proper escaping for special characters.
			sb.WriteString(fmt.Sprintf("    %s,\n", quote(msg)))
		}
		sb.WriteString("]\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// emptyBaseline returns a baseline that matches nothing.
func emptyBaseline() *BaselineConfig {
	b := &BaselineConfig{}
	b.buildLookup()
	return b
}

// toSet converts a string slice to a set (map[string]bool) for O(1) lookups.
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// quote produces a TOML-compatible double-quoted string with proper escaping.
// TOML basic strings use the same escape sequences as Go string literals.
func quote(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
