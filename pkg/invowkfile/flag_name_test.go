// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestFlagNameValidate(t *testing.T) {
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

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("FlagName(%q).Validate() error = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("FlagName(%q).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("FlagName.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidFlagName) {
					t.Errorf("error does not wrap ErrInvalidFlagName: %v", err)
				}
			}
		})
	}
}

func TestFlagShorthandValidate(t *testing.T) {
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

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("FlagShorthand(%q).Validate() error = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("FlagShorthand(%q).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("FlagShorthand.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidFlagShorthand) {
					t.Errorf("error does not wrap ErrInvalidFlagShorthand: %v", err)
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

func TestArgumentName_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value ArgumentName
		want  string
	}{
		{"normal name", ArgumentName("source"), "source"},
		{"empty", ArgumentName(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.value.String(); got != tt.want {
				t.Errorf("ArgumentName(%q).String() = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestArgumentNameValidate(t *testing.T) {
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

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("ArgumentName(%q).Validate() error = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("ArgumentName(%q).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("ArgumentName.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidArgumentName) {
					t.Errorf("error does not wrap ErrInvalidArgumentName: %v", err)
				}
			}
		})
	}
}

func TestCommandCategory_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value CommandCategory
		want  string
	}{
		{"normal category", CommandCategory("Build Tools"), "Build Tools"},
		{"empty", CommandCategory(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.value.String(); got != tt.want {
				t.Errorf("CommandCategory(%q).String() = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestCommandCategoryValidate(t *testing.T) {
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

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("CommandCategory(%q).Validate() error = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("CommandCategory(%q).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("CommandCategory.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidCommandCategory) {
					t.Errorf("error does not wrap ErrInvalidCommandCategory: %v", err)
				}
			}
		})
	}
}
