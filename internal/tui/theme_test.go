// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestTheme_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		theme   Theme
		want    bool
		wantErr bool
	}{
		{ThemeDefault, true, false},
		{ThemeCharm, true, false},
		{ThemeDracula, true, false},
		{ThemeCatppuccin, true, false},
		{ThemeBase16, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"DEFAULT", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.theme), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.theme.IsValid()
			if isValid != tt.want {
				t.Errorf("Theme(%q).IsValid() = %v, want %v", tt.theme, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Theme(%q).IsValid() returned no errors, want error", tt.theme)
				}
				if !errors.Is(errs[0], ErrInvalidTheme) {
					t.Errorf("error should wrap ErrInvalidTheme, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("Theme(%q).IsValid() returned unexpected errors: %v", tt.theme, errs)
			}
		})
	}
}
