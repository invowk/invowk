// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestRegexPatternValidate(t *testing.T) {
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

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("RegexPattern(%q).Validate() error = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("RegexPattern(%q).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("RegexPattern.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidRegexPattern) {
					t.Errorf("error does not wrap ErrInvalidRegexPattern: %v", err)
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
