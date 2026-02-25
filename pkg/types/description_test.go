// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestDescriptionText_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    DescriptionText
		want    bool
		wantErr bool
	}{
		{"simple text", DescriptionText("Run the build"), true, false},
		{"multiline", DescriptionText("Line 1\nLine 2"), true, false},
		{"empty is valid (zero value)", DescriptionText(""), true, false},
		{"whitespace only is invalid", DescriptionText("   "), false, true},
		{"tab only is invalid", DescriptionText("\t"), false, true},
		{"newline only is invalid", DescriptionText("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.desc.IsValid()
			if isValid != tt.want {
				t.Errorf("DescriptionText(%q).IsValid() = %v, want %v", tt.desc, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("DescriptionText(%q).IsValid() returned no errors, want error", tt.desc)
				}
				if !errors.Is(errs[0], ErrInvalidDescriptionText) {
					t.Errorf("error should wrap ErrInvalidDescriptionText, got: %v", errs[0])
				}
				var dtErr *InvalidDescriptionTextError
				if !errors.As(errs[0], &dtErr) {
					t.Errorf("error should be *InvalidDescriptionTextError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("DescriptionText(%q).IsValid() returned unexpected errors: %v", tt.desc, errs)
			}
		})
	}
}

func TestDescriptionText_String(t *testing.T) {
	t.Parallel()
	d := DescriptionText("Run the build")
	if d.String() != "Run the build" {
		t.Errorf("DescriptionText.String() = %q, want %q", d.String(), "Run the build")
	}
}
