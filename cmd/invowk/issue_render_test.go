// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/issue"
)

func TestFormatActionableError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *issue.ActionableError
		verbose  bool
		contains []string
		excludes []string
	}{
		{
			name: "simple error non-verbose",
			err: issue.NewErrorContext().
				WithOperation("load config").
				Build(),
			contains: []string{"failed to load config"},
		},
		{
			name: "error with suggestions",
			err: issue.NewErrorContext().
				WithOperation("load invowkfile").
				WithResource("./invowkfile.cue").
				WithSuggestions("Run 'invowk init'", "Check file permissions").
				Build(),
			contains: []string{
				"failed to load invowkfile",
				"./invowkfile.cue",
				"• Run 'invowk init'",
				"• Check file permissions",
			},
		},
		{
			name: "error chain in verbose mode",
			err: issue.NewErrorContext().
				WithOperation("parse config").
				Wrap(errors.New("syntax error")).
				Build(),
			verbose: true,
			contains: []string{
				"failed to parse config",
				"Error chain:",
				"1. syntax error",
			},
		},
		{
			name: "no error chain in non-verbose",
			err: issue.NewErrorContext().
				WithOperation("parse config").
				Wrap(errors.New("syntax error")).
				Build(),
			contains: []string{"failed to parse config: syntax error"},
			excludes: []string{"Error chain:"},
		},
		{
			name: "nested error chain verbose",
			err: issue.NewErrorContext().
				WithOperation("execute command").
				Wrap(issue.NewErrorContext().
					WithOperation("load script").
					Wrap(errors.New("file not found")).
					Build()).
				Build(),
			verbose: true,
			contains: []string{
				"Error chain:",
				"1. failed to load script: file not found",
				"2. file not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatActionableError(tt.err, tt.verbose).String()

			for _, expected := range tt.contains {
				if !strings.Contains(got, expected) {
					t.Errorf("formatActionableError() missing %q\ngot:\n%s", expected, got)
				}
			}

			for _, excluded := range tt.excludes {
				if strings.Contains(got, excluded) {
					t.Errorf("formatActionableError() should not contain %q\ngot:\n%s", excluded, got)
				}
			}
		})
	}
}

func TestIssueCatalogMarkdown(t *testing.T) {
	t.Parallel()

	catalogEntry := issue.Get(issue.InvowkfileNotFoundId)
	if catalogEntry == nil {
		t.Fatal("Get(InvowkfileNotFoundId) returned nil")
	}

	got := issueCatalogMarkdown(catalogEntry).String()
	if got == "" {
		t.Fatal("issueCatalogMarkdown() returned empty string")
	}
	if !strings.Contains(got, "invowkfile") {
		t.Errorf("issueCatalogMarkdown() should contain issue content, got: %s", got)
	}
}

func TestIssueRender_ColorSchemes(t *testing.T) {
	t.Parallel()

	t.Run("resolveGlamourStyle is orthogonal to color enablement", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			enabled bool
			scheme  config.ColorScheme
			want    glamourTheme
		}{
			{enabled: false, scheme: config.ColorSchemeAuto, want: glamourThemeNotty},
			{enabled: false, scheme: config.ColorSchemeDark, want: glamourThemeNotty},
			{enabled: false, scheme: config.ColorSchemeLight, want: glamourThemeNotty},
			{enabled: true, scheme: config.ColorSchemeAuto, want: glamourThemeAuto},
			{enabled: true, scheme: config.ColorSchemeDark, want: glamourThemeDark},
			{enabled: true, scheme: config.ColorSchemeLight, want: glamourThemeLight},
		}
		for _, tt := range tests {
			if got := resolveGlamourStyle(tt.enabled, tt.scheme); got != tt.want {
				t.Errorf("resolveGlamourStyle(%v, %q) = %q, want %q", tt.enabled, tt.scheme, got, tt.want)
			}
		}
	})

	t.Run("notty omits ANSI while dark and light differ", func(t *testing.T) {
		t.Parallel()

		entry := issue.Get(issue.InvowkfileNotFoundId)
		if entry == nil {
			t.Fatal("Get(InvowkfileNotFoundId) returned nil")
		}

		nottyOut, err := renderIssueCatalogEntry(entry, "notty")
		if err != nil {
			t.Fatalf("renderIssueCatalogEntry(notty) error = %v", err)
		}
		if strings.Contains(nottyOut, "\x1b[") {
			t.Errorf("notty output contains ANSI escapes: %q", nottyOut)
		}

		darkOut, err := renderIssueCatalogEntry(entry, "dark")
		if err != nil {
			t.Fatalf("renderIssueCatalogEntry(dark) error = %v", err)
		}
		lightOut, err := renderIssueCatalogEntry(entry, "light")
		if err != nil {
			t.Fatalf("renderIssueCatalogEntry(light) error = %v", err)
		}
		if !strings.Contains(darkOut, "\x1b[") || !strings.Contains(lightOut, "\x1b[") {
			t.Error("dark and light outputs should contain ANSI styling")
		}
		if darkOut == lightOut {
			t.Error("light Glamour output should differ from dark Glamour output")
		}
	})

	t.Run("App.glamourStyle honors ui.color_scheme and NO_COLOR", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		cfg.UI.ColorScheme = config.ColorSchemeLight

		var out, errBuf bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: cfg},
			Stdout: &out,
			Stderr: &errBuf,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		if got := app.glamourStyle(t.Context(), &rootFlagValues{}); got != "notty" {
			t.Errorf("glamourStyle(non-TTY auto) = %q, want notty", got)
		}

		app.setColorMode(colorModeAlways)
		if got := app.glamourStyle(t.Context(), &rootFlagValues{}); got != "light" {
			t.Errorf("glamourStyle(always + light) = %q, want light", got)
		}

		app.setColorMode(colorModeNever)
		if got := app.glamourStyle(t.Context(), &rootFlagValues{}); got != "notty" {
			t.Errorf("glamourStyle(never + light) = %q, want notty", got)
		}
	})
}
