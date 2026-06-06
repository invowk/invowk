// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

const (
	validationInputFloatErr   = "must be a valid floating-point number"
	validationInputIntegerErr = "must be a valid integer"
)

func TestValidateValueTypeMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		kind    FlagType
		wantErr string
	}{
		{
			name:  "integer accepts leading negative sign",
			value: "-42",
			kind:  FlagTypeInt,
		},
		{
			name:  "integer accepts digit nine",
			value: "9",
			kind:  FlagTypeInt,
		},
		{
			name:    "integer rejects sign without digits",
			value:   "-",
			kind:    FlagTypeInt,
			wantErr: validationInputIntegerErr,
		},
		{
			name:    "integer rejects empty value",
			value:   "",
			kind:    FlagTypeInt,
			wantErr: validationInputIntegerErr,
		},
		{
			name:    "integer rejects embedded negative sign",
			value:   "1-2",
			kind:    FlagTypeInt,
			wantErr: validationInputIntegerErr,
		},
		{
			name:    "integer rejects invalid first rune",
			value:   "x2",
			kind:    FlagTypeInt,
			wantErr: validationInputIntegerErr,
		},
		{
			name:    "integer rejects non-digit after negative sign",
			value:   "-x",
			kind:    FlagTypeInt,
			wantErr: validationInputIntegerErr,
		},
		{
			name:  "float accepts value outside float32 range",
			value: "1e39",
			kind:  FlagTypeFloat,
		},
		{
			name:    "float rejects empty value",
			value:   "",
			kind:    FlagTypeFloat,
			wantErr: validationInputFloatErr,
		},
		{
			name:  "string accepts any value",
			value: "anything",
			kind:  FlagTypeString,
		},
		{
			name:    "unknown type rejects programmatic misuse",
			value:   "anything",
			kind:    "duration",
			wantErr: `unknown flag type "duration"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateValueType(tt.value, tt.kind)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateValueType(%q, %q) error = %v, want nil", tt.value, tt.kind, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateValueType(%q, %q) error = nil, want %q", tt.value, tt.kind, tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateValueType(%q, %q) error = %q, want %q", tt.value, tt.kind, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateValueWithRegexMutationContracts(t *testing.T) {
	t.Parallel()

	if err := validateValueWithRegex("input", "anything", ""); err != nil {
		t.Fatalf("validateValueWithRegex(empty pattern) error = %v, want nil", err)
	}

	invalidErr := validateValueWithRegex("input", "anything", "[")
	if invalidErr == nil {
		t.Fatal("validateValueWithRegex(invalid pattern) error = nil, want error")
	}
	if !strings.Contains(invalidErr.Error(), "input") || !strings.Contains(invalidErr.Error(), "invalid validation pattern") {
		t.Fatalf("validateValueWithRegex(invalid pattern) error = %q, want named invalid-pattern diagnostic", invalidErr)
	}

	mismatchErr := validateValueWithRegex("input", "dev", "^prod$")
	if mismatchErr == nil {
		t.Fatal("validateValueWithRegex(mismatch) error = nil, want error")
	}
	if !strings.Contains(mismatchErr.Error(), "dev") || !strings.Contains(mismatchErr.Error(), "^prod$") {
		t.Fatalf("validateValueWithRegex(mismatch) error = %q, want value and pattern diagnostic", mismatchErr)
	}
}
