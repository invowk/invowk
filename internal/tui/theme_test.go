// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestTheme_Validate(t *testing.T) {
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
			err := tt.theme.Validate()
			if (err == nil) != tt.want {
				t.Errorf("Theme(%q).Validate() err = %v, wantValid %v", tt.theme, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Theme(%q).Validate() returned nil, want error", tt.theme)
				}
				if !errors.Is(err, ErrInvalidTheme) {
					t.Errorf("error should wrap ErrInvalidTheme, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("Theme(%q).Validate() returned unexpected error: %v", tt.theme, err)
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

func TestTUIConfig_Validate(t *testing.T) {
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
			err := tt.cfg.Validate()
			if (err == nil) != tt.want {
				t.Errorf("Config.Validate() err = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Config.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidTUIConfig) {
					t.Errorf("error should wrap ErrInvalidTUIConfig, got: %v", err)
				}
				var cfgErr *InvalidTUIConfigError
				if !errors.As(err, &cfgErr) {
					t.Fatalf("error should be *InvalidTUIConfigError, got: %T", err)
				}
				if len(cfgErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(cfgErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("Config.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
