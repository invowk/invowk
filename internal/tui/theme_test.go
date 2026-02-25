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

func TestTheme_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		theme Theme
		want  string
	}{
		{ThemeDefault, "default"},
		{ThemeCharm, "charm"},
		{ThemeDracula, "dracula"},
		{ThemeCatppuccin, "catppuccin"},
		{ThemeBase16, "base16"},
		{"custom", "custom"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.theme.String()
			if got != tt.want {
				t.Errorf("Theme(%q).String() = %q, want %q", tt.theme, got, tt.want)
			}
		})
	}
}

func TestTheme_String_FmtStringer(t *testing.T) {
	t.Parallel()

	// Verify Theme implements fmt.Stringer.
	got := ThemeCharm.String()
	if got != "charm" {
		t.Errorf("ThemeCharm.String() = %q, want %q", got, "charm")
	}
}

func TestTUIConfig_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       Config
		want      bool
		wantErr   bool
		wantCount int // expected number of field errors
	}{
		{
			"all valid",
			Config{Theme: ThemeDefault, Accessible: false, Width: 80},
			true, false, 0,
		},
		{
			"valid charm theme",
			Config{Theme: ThemeCharm},
			true, false, 0,
		},
		{
			"invalid theme (empty)",
			Config{Theme: Theme("")},
			false, true, 1,
		},
		{
			"invalid theme (unknown)",
			Config{Theme: Theme("nope")},
			false, true, 1,
		},
		{
			"zero value struct",
			Config{},
			false, true, 1, // zero Theme is ""  which is invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cfg.IsValid()
			if isValid != tt.want {
				t.Errorf("Config.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Config.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidTUIConfig) {
					t.Errorf("error should wrap ErrInvalidTUIConfig, got: %v", errs[0])
				}
				var cfgErr *InvalidTUIConfigError
				if !errors.As(errs[0], &cfgErr) {
					t.Fatalf("error should be *InvalidTUIConfigError, got: %T", errs[0])
				}
				if len(cfgErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(cfgErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("Config.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}
