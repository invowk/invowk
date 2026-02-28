// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestDescriptionText_Validate(t *testing.T) {
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
			err := tt.desc.Validate()
			if (err == nil) != tt.want {
				t.Errorf("DescriptionText(%q).Validate() error = %v, want valid=%v", tt.desc, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DescriptionText(%q).Validate() returned nil, want error", tt.desc)
				}
				if !errors.Is(err, ErrInvalidDescriptionText) {
					t.Errorf("error should wrap ErrInvalidDescriptionText, got: %v", err)
				}
				var dtErr *InvalidDescriptionTextError
				if !errors.As(err, &dtErr) {
					t.Errorf("error should be *InvalidDescriptionTextError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("DescriptionText(%q).Validate() returned unexpected error: %v", tt.desc, err)
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
