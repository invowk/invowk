// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/fang"

	"github.com/invowk/invowk/internal/config"
)

func TestColorFlag(t *testing.T) {
	t.Parallel()

	t.Run("valid values", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			args        []string
			wantMode    colorMode
			wantProfile colorprofile.Profile
		}{
			{
				name:        "default auto in buffer",
				args:        []string{"init", "--help"},
				wantMode:    colorModeAuto,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "inline always",
				args:        []string{"--ivk-color=always", "init", "--help"},
				wantMode:    colorModeAlways,
				wantProfile: colorprofile.TrueColor,
			},
			{
				name:        "separate always",
				args:        []string{"--ivk-color", "always", "init", "--help"},
				wantMode:    colorModeAlways,
				wantProfile: colorprofile.TrueColor,
			},
			{
				name:        "inline never",
				args:        []string{"--ivk-color=never", "init", "--help"},
				wantMode:    colorModeNever,
				wantProfile: colorprofile.NoTTY,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var stdout, stderr bytes.Buffer
				app, err := NewApp(Dependencies{
					Config: &staticConfigProvider{cfg: config.DefaultConfig()},
					Stdout: &stdout,
					Stderr: &stderr,
					Getenv: func(string) string { return "" },
				})
				if err != nil {
					t.Fatalf("NewApp() error = %v", err)
				}

				rootCmd := NewRootCommand(app)
				rootCmd.SetOut(&stdout)
				rootCmd.SetErr(&stderr)
				rootCmd.SetArgs(tt.args)
				if err := rootCmd.ExecuteContext(t.Context()); err != nil {
					t.Fatalf("ExecuteContext(%v) error = %v", tt.args, err)
				}
				if gotMode := colorModeFromCmd(rootCmd); gotMode != tt.wantMode {
					t.Errorf("colorModeFromCmd() = %q, want %q", gotMode, tt.wantMode)
				}
				if app.uiOut.Profile != tt.wantProfile {
					t.Errorf("app.uiOut.Profile = %v, want %v", app.uiOut.Profile, tt.wantProfile)
				}
			})
		}
	})

	t.Run("invalid values return usage error exit 2", func(t *testing.T) {
		t.Parallel()

		for _, rawArgs := range [][]string{
			{"--ivk-color=bogus", "init"},
			{"--ivk-color=", "init"},
			{"--ivk-color=bogus", "--help"},
		} {
			var stdout, stderr bytes.Buffer
			app, err := NewApp(Dependencies{
				Config: &staticConfigProvider{cfg: config.DefaultConfig()},
				Stdout: &stdout,
				Stderr: &stderr,
				Getenv: func(string) string { return "" },
			})
			if err != nil {
				t.Fatalf("NewApp() error = %v", err)
			}

			rootCmd := NewRootCommand(app)
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			rootCmd.SetArgs(rawArgs)
			runErr := rootCmd.ExecuteContext(t.Context())
			if runErr == nil {
				t.Fatalf("ExecuteContext(%v) = nil, want UsageError", rawArgs)
			}
			if got := exitCodeForError(runErr); got != ExitCodeUsage {
				t.Errorf("exitCodeForError(%v) = %d, want %d", runErr, got, ExitCodeUsage)
			}
			if !errors.Is(runErr, errInvalidColorMode) {
				t.Errorf("errors.Is(%v, errInvalidColorMode) = false, want true", runErr)
			}
			if !strings.Contains(runErr.Error(), "--ivk-color") {
				t.Errorf("error %q does not mention --ivk-color", runErr.Error())
			}
		}
	})
}

func TestColorProfileWriter(t *testing.T) {
	t.Parallel()

	t.Run("precedence matrix", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			mode        colorMode
			env         map[string]string
			wantEnabled bool
			wantProfile colorprofile.Profile
			wantAtLeast bool
		}{
			{
				name:        "non-TTY auto omits color",
				mode:        colorModeAuto,
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "NO_COLOR=1 disables color",
				mode:        colorModeAuto,
				env:         map[string]string{"NO_COLOR": "1", "TTY_FORCE": "1", "TERM": "xterm-256color"},
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "non-bool NO_COLOR disables color",
				mode:        colorModeAuto,
				env:         map[string]string{"NO_COLOR": "yes", "TTY_FORCE": "1", "TERM": "xterm-256color"},
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "NO_COLOR beats CLICOLOR_FORCE",
				mode:        colorModeAuto,
				env:         map[string]string{"NO_COLOR": "1", "CLICOLOR_FORCE": "1"},
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "CLICOLOR_FORCE enables color in non-TTY",
				mode:        colorModeAuto,
				env:         map[string]string{"CLICOLOR_FORCE": "1"},
				wantEnabled: true,
				// colorprofile.Detect returns ANSI on POSIX without TERM and TrueColor on Windows console.
				wantProfile: colorprofile.ANSI,
				wantAtLeast: true,
			},
			{
				name:        "CLICOLOR_FORCE=0 does not force color",
				mode:        colorModeAuto,
				env:         map[string]string{"CLICOLOR_FORCE": "0"},
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "TERM=dumb disables color on TTY",
				mode:        colorModeAuto,
				env:         map[string]string{"TTY_FORCE": "1", "TERM": "dumb"},
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
			{
				name:        "TTY_FORCE with xterm-256color enables color",
				mode:        colorModeAuto,
				env:         map[string]string{"TTY_FORCE": "1", "TERM": "xterm-256color"},
				wantEnabled: true,
				wantProfile: colorprofile.ANSI256,
			},
			{
				name:        "ivk-color=always overrides NO_COLOR and TERM=dumb",
				mode:        colorModeAlways,
				env:         map[string]string{"NO_COLOR": "1", "TERM": "dumb"},
				wantEnabled: true,
				wantProfile: colorprofile.TrueColor,
			},
			{
				name:        "ivk-color=never overrides CLICOLOR_FORCE and TTY_FORCE",
				mode:        colorModeNever,
				env:         map[string]string{"CLICOLOR_FORCE": "1", "TTY_FORCE": "1", "TERM": "xterm-256color"},
				wantEnabled: false,
				wantProfile: colorprofile.NoTTY,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var buf bytes.Buffer
				getenv := func(k string) string { return tt.env[k] }
				profile := resolveColorProfile(tt.mode, &buf, getenv)
				if tt.wantAtLeast {
					if profile < tt.wantProfile {
						t.Errorf("resolveColorProfile(%q) = %v, want >= %v", tt.mode, profile, tt.wantProfile)
					}
				} else if profile != tt.wantProfile {
					t.Errorf("resolveColorProfile(%q) = %v, want %v", tt.mode, profile, tt.wantProfile)
				}
				if got := colorEnabled(profile); got != tt.wantEnabled {
					t.Errorf("colorEnabled(%v) = %v, want %v", profile, got, tt.wantEnabled)
				}

				w := &colorprofile.Writer{Forward: &buf, Profile: profile}
				if _, err := w.WriteString(TitleStyle.Render("Title")); err != nil {
					t.Fatalf("WriteString() error = %v", err)
				}
				hasANSI := strings.Contains(buf.String(), "\x1b[")
				if hasANSI != tt.wantEnabled {
					t.Errorf("rendered ANSI = %v (output=%q), want %v", hasANSI, buf.String(), tt.wantEnabled)
				}
			})
		}
	})

	t.Run("App keeps raw stdout and stderr unwrapped", func(t *testing.T) {
		t.Parallel()

		var rawOut, rawErr bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &rawOut,
			Stderr: &rawErr,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}
		if app.stdout != &rawOut || app.stderr != &rawErr {
			t.Fatal("App.stdout / App.stderr must remain raw unwrapped writers")
		}
		if app.uiOut == nil || app.uiOut.Forward != &rawOut {
			t.Fatal("App.uiOut must forward to App.stdout")
		}
		if app.uiErr == nil || app.uiErr.Forward != &rawErr {
			t.Fatal("App.uiErr must forward to App.stderr")
		}
	})
}

func TestFangErrorHandler_QuietExitError(t *testing.T) {
	t.Parallel()

	styles := fang.Styles{}

	t.Run("bare ExitError suppresses banner even when color enabled", func(t *testing.T) {
		t.Parallel()

		var out, errBuf bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &out,
			Stderr: &errBuf,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}
		app.setColorMode(colorModeAlways)
		rootCmd := NewRootCommand(app)

		handler := newFangErrorHandler(app, rootCmd)
		handler(&errBuf, styles, &ExitError{Code: 42})

		if errBuf.Len() != 0 {
			t.Fatalf("stderr = %q, want empty for bare ExitError", errBuf.String())
		}
	})

	t.Run("bare ExitError with --ivk-verbose prints single plain line", func(t *testing.T) {
		t.Parallel()

		var out, errBuf bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &out,
			Stderr: &errBuf,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}
		rootCmd := NewRootCommand(app)
		if err := rootCmd.PersistentFlags().Set("ivk-verbose", "true"); err != nil {
			t.Fatalf("Set(ivk-verbose) error = %v", err)
		}

		handler := newFangErrorHandler(app, rootCmd)
		handler(&errBuf, styles, &ExitError{Code: 42})

		got := errBuf.String()
		if got != "invowk: command exited with status 42\n" {
			t.Fatalf("stderr = %q, want single status line", got)
		}
	})

	t.Run("UsageError still renders message and Try --help hint without ANSI in non-TTY", func(t *testing.T) {
		t.Parallel()

		var out, errBuf bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &out,
			Stderr: &errBuf,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}
		rootCmd := NewRootCommand(app)

		handler := newFangErrorHandler(app, rootCmd)
		handler(&errBuf, styles, newUsageError(errors.New("unknown flag: --bogus")))

		got := errBuf.String()
		if !strings.Contains(got, "unknown flag: --bogus") {
			t.Errorf("stderr = %q, want unknown flag message", got)
		}
		if !strings.Contains(got, "Try --help for usage.") {
			t.Errorf("stderr = %q, want Try --help hint", got)
		}
		if strings.Contains(got, "\x1b[") {
			t.Errorf("stderr = %q, want no ANSI escapes in non-TTY", got)
		}
	})
}

func TestFangHelp_ColorControl_NonTTYAuto(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	app, err := NewApp(Dependencies{
		Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		Stdout: &out,
		Stderr: &errBuf,
		Getenv: func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	rootCmd := NewRootCommand(app)
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--help"})

	if err := executeWithFang(t.Context(), app, rootCmd, []string{"--help"}); err != nil {
		t.Fatalf("executeWithFang() error = %v", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("help stdout contains ANSI escapes in non-TTY auto mode: %q", out.String())
	}
	if !strings.Contains(out.String(), "invowk") {
		t.Errorf("help stdout missing invowk text: %q", out.String())
	}
}

func TestFangHelp_ColorControl_NeverOverridesCLICOLORForce(t *testing.T) {
	var out, errBuf bytes.Buffer
	t.Setenv("CLICOLOR_FORCE", "1")
	app, err := NewApp(Dependencies{
		Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		Stdout: &out,
		Stderr: &errBuf,
		Getenv: os.Getenv,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	rootCmd := NewRootCommand(app)
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errBuf)
	args := []string{"--ivk-color=never", "--help"}
	rootCmd.SetArgs(args)

	if err := executeWithFang(t.Context(), app, rootCmd, args); err != nil {
		t.Fatalf("executeWithFang() error = %v", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("help stdout contains ANSI escapes under --ivk-color=never: %q", out.String())
	}
}

func TestFangHelp_ColorControl_AlwaysOverridesNoColor(t *testing.T) {
	var out, errBuf bytes.Buffer
	t.Setenv("NO_COLOR", "1")
	app, err := NewApp(Dependencies{
		Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		Stdout: &out,
		Stderr: &errBuf,
		Getenv: os.Getenv,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	rootCmd := NewRootCommand(app)
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errBuf)
	args := []string{"--ivk-color=always", "--help"}
	rootCmd.SetArgs(args)

	if err := executeWithFang(t.Context(), app, rootCmd, args); err != nil {
		t.Fatalf("executeWithFang() error = %v", err)
	}
	if !strings.Contains(out.String(), "\x1b[") {
		t.Errorf("help stdout missing ANSI escapes under --ivk-color=always: %q", out.String())
	}
}
