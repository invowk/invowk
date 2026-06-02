// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRegexPatternBoundaryAndWrapping(t *testing.T) {
	t.Parallel()

	if err := ValidateRegexPattern(strings.Repeat("a", MaxRegexPatternLength)); err != nil {
		t.Fatalf("ValidateRegexPattern(max length) error = %v, want nil", err)
	}

	err := ValidateRegexPattern(strings.Repeat("a", MaxRegexPatternLength+1))
	if err == nil {
		t.Fatal("ValidateRegexPattern(over max length) error = nil, want length error")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Fatalf("ValidateRegexPattern(over max length) error = %v, want too long", err)
	}

	err = ValidateRegexPattern("[z-a]")
	if err == nil {
		t.Fatal("ValidateRegexPattern(invalid regex) error = nil, want wrapped compile error")
	}
	if !strings.Contains(err.Error(), "invalid regex") {
		t.Fatalf("ValidateRegexPattern(invalid regex) error = %v, want invalid regex", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("ValidateRegexPattern(invalid regex) error = %v, want wrapped cause", err)
	}
}

func TestValidateRequiredDescriptionTextDelegatesValidation(t *testing.T) {
	t.Parallel()

	if err := validateRequiredDescriptionText("Valid description"); err != nil {
		t.Fatalf("validateRequiredDescriptionText(valid) error = %v, want nil", err)
	}

	err := validateRequiredDescriptionText(" \t\n ")
	if err == nil {
		t.Fatal("validateRequiredDescriptionText(whitespace) error = nil, want required error")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("validateRequiredDescriptionText(whitespace) error = %v, want required error", err)
	}

	err = validateRequiredDescriptionText(DescriptionText(strings.Repeat("a", MaxDescriptionLength+1)))
	if err == nil {
		t.Fatal("validateRequiredDescriptionText(too long) error = nil, want delegated validation error")
	}
	if strings.Contains(err.Error(), "description is required") {
		t.Fatalf("validateRequiredDescriptionText(too long) error = %v, want delegated validation error", err)
	}
}

func TestHasOverlappingAlternationEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{name: "second group overlaps", pattern: "(cat|dog)+(ab|ac)+", want: true},
		{name: "empty alternative ignored", pattern: "(a|)+", want: false},
		{name: "regex token prefix overlaps", pattern: `(\\d|\\d+)+`, want: true},
		{name: "same literal start overlaps", pattern: "(ab|ac)+", want: true},
		{name: "left prefix overlaps", pattern: "(ab|a)+", want: true},
		{name: "empty left alternative ignored", pattern: "(|a)+", want: false},
		{name: "empty middle alternative does not stop later overlap", pattern: "(a||aa)+", want: true},
		{name: "same literal start before metacharacter overlaps", pattern: "(a.|ab)+", want: true},
		{name: "same metacharacter start is not literal overlap", pattern: "(.*|.+)+", want: false},
		{name: "different literal starts do not overlap", pattern: "(a|b)+", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasOverlappingAlternation(tt.pattern); got != tt.want {
				t.Fatalf("hasOverlappingAlternation(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestCheckNestingDepthEscapesAndBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "exact max nested groups",
			pattern: strings.Repeat("(", MaxNestedGroups) + "x" + strings.Repeat(")", MaxNestedGroups),
		},
		{
			name:    "over max nested groups",
			pattern: strings.Repeat("(", MaxNestedGroups+1) + "x" + strings.Repeat(")", MaxNestedGroups+1),
			wantErr: true,
		},
		{
			name:    "escaped opening parens do not count",
			pattern: strings.Repeat(`\(`, MaxNestedGroups+1),
		},
		{
			name:    "escaped paren resets before later nesting",
			pattern: `\(` + strings.Repeat("(", MaxNestedGroups+1) + "x" + strings.Repeat(")", MaxNestedGroups+1),
			wantErr: true,
		},
		{
			name:    "unmatched closing parens do not go negative",
			pattern: strings.Repeat(")", MaxNestedGroups+1) + strings.Repeat("(", MaxNestedGroups) + "x",
		},
		{
			name:    "unmatched close before over max nesting",
			pattern: ")" + strings.Repeat("(", MaxNestedGroups+1) + "x" + strings.Repeat(")", MaxNestedGroups+1),
			wantErr: true,
		},
		{
			name:    "sequential shallow groups stay shallow",
			pattern: strings.Repeat("()", MaxNestedGroups+1),
		},
		{
			name: "sequential exact max groups reset depth",
			pattern: strings.Repeat("(", MaxNestedGroups) + "x" + strings.Repeat(")", MaxNestedGroups) +
				strings.Repeat("(", MaxNestedGroups) + "y" + strings.Repeat(")", MaxNestedGroups),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkNestingDepth(tt.pattern)
			if tt.wantErr && err == nil {
				t.Fatal("checkNestingDepth() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("checkNestingDepth() error = %v, want nil", err)
			}
		})
	}
}

func TestCheckQuantifierCountEscapesClassesAndBraces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{name: "exact max simple quantifiers", pattern: strings.Repeat("a+", MaxQuantifierRepeats)},
		{name: "over max simple quantifiers", pattern: strings.Repeat("a+", MaxQuantifierRepeats+1), wantErr: true},
		{name: "over max star quantifiers", pattern: strings.Repeat("a*", MaxQuantifierRepeats+1), wantErr: true},
		{name: "over max question quantifiers", pattern: strings.Repeat("a?", MaxQuantifierRepeats+1), wantErr: true},
		{name: "escaped pluses ignored", pattern: strings.Repeat(`\+`, MaxQuantifierRepeats+1)},
		{name: "escaped plus resets before later quantifiers", pattern: `\+` + strings.Repeat("+", MaxQuantifierRepeats+1), wantErr: true},
		{name: "character class quantifiers ignored", pattern: strings.Repeat("[+*?{}]", MaxQuantifierRepeats+1)},
		{name: "character class closes before later quantifiers", pattern: strings.Repeat("[a]+", MaxQuantifierRepeats+1), wantErr: true},
		{name: "brace quantifiers counted", pattern: strings.Repeat("a{1}", MaxQuantifierRepeats+1), wantErr: true},
		{name: "brace ranges counted", pattern: strings.Repeat("a{1,2}", MaxQuantifierRepeats+1), wantErr: true},
		{name: "brace zero quantifiers counted", pattern: strings.Repeat("a{0}", MaxQuantifierRepeats+1), wantErr: true},
		{name: "brace nine quantifiers counted", pattern: strings.Repeat("a{9}", MaxQuantifierRepeats+1), wantErr: true},
		{name: "extra closing brace after quantifier ignored", pattern: strings.Repeat("a{1}}", MaxQuantifierRepeats)},
		{name: "literal invalid braces ignored", pattern: strings.Repeat("a{b}", MaxQuantifierRepeats+1)},
		{name: "low non-digit invalid braces ignored", pattern: strings.Repeat("a{/}", MaxQuantifierRepeats+1)},
		{name: "unterminated braces ignored", pattern: strings.Repeat("a{12", MaxQuantifierRepeats+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkQuantifierCount(tt.pattern)
			if tt.wantErr && err == nil {
				t.Fatal("checkQuantifierCount() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("checkQuantifierCount() error = %v, want nil", err)
			}
		})
	}
}
