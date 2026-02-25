// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestRegexPatternIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     RegexPattern
		wantValid bool
	}{
		{name: "empty is valid", value: "", wantValid: true},
		{name: "simple pattern", value: "^hello$", wantValid: true},
		{name: "character class", value: "[a-zA-Z0-9]+", wantValid: true},
		{name: "semver pattern", value: `^v[0-9]+\.[0-9]+`, wantValid: true},
		{name: "unclosed bracket", value: "[a-z", wantValid: false},
		{name: "dangerous backtracking pattern", value: "(a+)+", wantValid: false},
		{name: "dangerous alternation pattern", value: "(a|aa)+", wantValid: false},
		{name: "invalid range", value: "[z-a]", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.wantValid {
				t.Errorf("RegexPattern(%q).IsValid() = %v, want %v", tt.value, isValid, tt.wantValid)
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("RegexPattern(%q).IsValid() returned errors for valid value: %v", tt.value, errs)
				}
			} else {
				if len(errs) == 0 {
					t.Error("RegexPattern.IsValid() returned no errors for invalid value")
				}
				if !errors.Is(errs[0], ErrInvalidRegexPattern) {
					t.Errorf("error does not wrap ErrInvalidRegexPattern: %v", errs[0])
				}
			}
		})
	}
}

func TestRegexPatternString(t *testing.T) {
	t.Parallel()

	r := RegexPattern("^hello$")
	if got := r.String(); got != "^hello$" {
		t.Errorf("RegexPattern.String() = %q, want %q", got, "^hello$")
	}
}
