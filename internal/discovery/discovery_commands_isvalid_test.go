// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"testing"
)

func TestSourceID_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   SourceID
		want    bool
		wantErr bool
	}{
		{"invowkfile constant", SourceIDInvowkfile, true, false},
		{"simple name", SourceID("foo"), true, false},
		{"name with dot", SourceID("io.invowk.sample"), true, false},
		{"name with hyphen", SourceID("my-module"), true, false},
		{"name with underscore", SourceID("my_module"), true, false},
		{"mixed characters", SourceID("foo-bar.baz_42"), true, false},
		{"single letter", SourceID("x"), true, false},
		{"uppercase", SourceID("MyModule"), true, false},
		{"empty string", SourceID(""), false, true},
		{"starts with digit", SourceID("123abc"), false, true},
		{"contains space", SourceID("foo bar"), false, true},
		{"starts with dot", SourceID(".hidden"), false, true},
		{"starts with hyphen", SourceID("-invalid"), false, true},
		{"contains at sign", SourceID("@invalid"), false, true},
		{"contains slash", SourceID("path/to"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.want {
				t.Errorf("SourceID(%q).IsValid() = %v, want %v", tt.value, isValid, tt.want)
			}

			if tt.wantErr {
				if len(errs) == 0 {
					t.Error("expected validation errors, got none")
				} else if !errors.Is(errs[0], ErrInvalidSourceID) {
					t.Errorf("expected errors.Is(err, ErrInvalidSourceID), got %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("unexpected validation errors: %v", errs)
			}
		})
	}
}
