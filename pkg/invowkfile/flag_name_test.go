// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestFlagNameIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     FlagName
		wantValid bool
	}{
		{name: "simple name", value: "env", wantValid: true},
		{name: "with hyphens", value: "dry-run", wantValid: true},
		{name: "with underscores", value: "max_retries", wantValid: true},
		{name: "with digits", value: "level2", wantValid: true},
		{name: "single letter", value: "v", wantValid: true},
		{name: "empty is invalid", value: "", wantValid: false},
		{name: "starts with digit", value: "2fast", wantValid: false},
		{name: "starts with hyphen", value: "-verbose", wantValid: false},
		{name: "contains space", value: "my flag", wantValid: false},
		{name: "too long", value: FlagName(strings.Repeat("a", MaxNameLength+1)), wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.wantValid {
				t.Errorf("FlagName(%q).IsValid() = %v, want %v", tt.value, isValid, tt.wantValid)
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("FlagName(%q).IsValid() returned errors for valid value: %v", tt.value, errs)
				}
			} else {
				if len(errs) == 0 {
					t.Error("FlagName.IsValid() returned no errors for invalid value")
				}
				if !errors.Is(errs[0], ErrInvalidFlagName) {
					t.Errorf("error does not wrap ErrInvalidFlagName: %v", errs[0])
				}
			}
		})
	}
}

func TestFlagShorthandIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     FlagShorthand
		wantValid bool
	}{
		{name: "empty is valid", value: "", wantValid: true},
		{name: "lowercase letter", value: "v", wantValid: true},
		{name: "uppercase letter", value: "V", wantValid: true},
		{name: "digit is invalid", value: "1", wantValid: false},
		{name: "two letters is invalid", value: "ab", wantValid: false},
		{name: "special char is invalid", value: "-", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.wantValid {
				t.Errorf("FlagShorthand(%q).IsValid() = %v, want %v", tt.value, isValid, tt.wantValid)
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("FlagShorthand(%q).IsValid() returned errors for valid value: %v", tt.value, errs)
				}
			} else {
				if len(errs) == 0 {
					t.Error("FlagShorthand.IsValid() returned no errors for invalid value")
				}
				if !errors.Is(errs[0], ErrInvalidFlagShorthand) {
					t.Errorf("error does not wrap ErrInvalidFlagShorthand: %v", errs[0])
				}
			}
		})
	}
}

func TestFlagNameString(t *testing.T) {
	t.Parallel()

	n := FlagName("verbose")
	if got := n.String(); got != "verbose" {
		t.Errorf("FlagName.String() = %q, want %q", got, "verbose")
	}
}

func TestFlagShorthandString(t *testing.T) {
	t.Parallel()

	s := FlagShorthand("v")
	if got := s.String(); got != "v" {
		t.Errorf("FlagShorthand.String() = %q, want %q", got, "v")
	}
}

func TestArgumentNameIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     ArgumentName
		wantValid bool
	}{
		{name: "simple name", value: "source", wantValid: true},
		{name: "with hyphens", value: "input-file", wantValid: true},
		{name: "with underscores", value: "output_dir", wantValid: true},
		{name: "with digits", value: "arg1", wantValid: true},
		{name: "single letter", value: "n", wantValid: true},
		{name: "empty is invalid", value: "", wantValid: false},
		{name: "starts with digit", value: "1st", wantValid: false},
		{name: "contains space", value: "my arg", wantValid: false},
		{name: "too long", value: ArgumentName(strings.Repeat("a", MaxNameLength+1)), wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.wantValid {
				t.Errorf("ArgumentName(%q).IsValid() = %v, want %v", tt.value, isValid, tt.wantValid)
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("ArgumentName(%q).IsValid() returned errors for valid value: %v", tt.value, errs)
				}
			} else {
				if len(errs) == 0 {
					t.Error("ArgumentName.IsValid() returned no errors for invalid value")
				}
				if !errors.Is(errs[0], ErrInvalidArgumentName) {
					t.Errorf("error does not wrap ErrInvalidArgumentName: %v", errs[0])
				}
			}
		})
	}
}

func TestCommandCategoryIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     CommandCategory
		wantValid bool
	}{
		{name: "empty is valid", value: "", wantValid: true},
		{name: "normal category", value: "Build Tools", wantValid: true},
		{name: "single word", value: "testing", wantValid: true},
		{name: "whitespace only is invalid", value: "   ", wantValid: false},
		{name: "tab only is invalid", value: "\t", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.wantValid {
				t.Errorf("CommandCategory(%q).IsValid() = %v, want %v", tt.value, isValid, tt.wantValid)
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("CommandCategory(%q).IsValid() returned errors for valid value: %v", tt.value, errs)
				}
			} else {
				if len(errs) == 0 {
					t.Error("CommandCategory.IsValid() returned no errors for invalid value")
				}
				if !errors.Is(errs[0], ErrInvalidCommandCategory) {
					t.Errorf("error does not wrap ErrInvalidCommandCategory: %v", errs[0])
				}
			}
		})
	}
}
