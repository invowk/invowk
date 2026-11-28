// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
)

const (
	colorFlagName       = "ivk-color"
	longColorFlag       = "--" + colorFlagName
	longColorFlagInline = longColorFlag + "="

	colorModeAuto   colorMode = "auto"
	colorModeAlways colorMode = "always"
	colorModeNever  colorMode = "never"

	glamourThemeUnset glamourTheme = ""
	glamourThemeAuto  glamourTheme = "auto"
	glamourThemeDark  glamourTheme = "dark"
	glamourThemeLight glamourTheme = "light"
	glamourThemeNotty glamourTheme = "notty"

	errMsgInvalidColorMode    = "invalid --ivk-color value"
	errMsgInvalidGlamourTheme = "invalid glamour theme"
)

var (
	// errInvalidColorMode is returned when --ivk-color is not auto, always, or never.
	errInvalidColorMode = errors.New(errMsgInvalidColorMode)

	// errInvalidGlamourTheme is returned when a glamourTheme value is unrecognized.
	errInvalidGlamourTheme = errors.New(errMsgInvalidGlamourTheme)

	// colorEnvKeys lists environment variables consulted by colorprofile detection.
	colorEnvKeys = []string{
		"NO_COLOR",
		"CLICOLOR",
		"CLICOLOR_FORCE",
		"TTY_FORCE",
		"COLORTERM",
		"TERM",
		"WT_SESSION",
		"GOOGLE_CLOUD_SHELL",
		"TMUX",
	}
)

type (
	// colorMode controls CLI ANSI color enablement (--ivk-color=auto|always|never).
	colorMode string

	// glamourTheme identifies a Glamour markdown rendering style.
	glamourTheme string

	// colorModeFlag adapts colorMode to pflag.Value so Cobra validates --ivk-color
	// during flag parsing (returning a UsageError before help or RunE) and updates
	// App's UI writers immediately.
	colorModeFlag struct {
		raw *string //goplint:ignore -- Cobra pflag.Value binding target validated by Set and parseColorMode.
		app *App
	}

	// invalidColorModeError reports an unrecognized --ivk-color value.
	invalidColorModeError struct {
		value colorMode
	}

	// invalidGlamourThemeError reports an unrecognized Glamour theme value.
	invalidGlamourThemeError struct {
		value glamourTheme
	}
)

// Error implements the error interface for invalidColorModeError.
func (e *invalidColorModeError) Error() string {
	return fmt.Sprintf("%s %q: must be one of auto, always, never", errMsgInvalidColorMode, e.value)
}

// Unwrap returns errInvalidColorMode for errors.Is() compatibility.
func (e *invalidColorModeError) Unwrap() error { return errInvalidColorMode }

// Error implements the error interface for invalidGlamourThemeError.
func (e *invalidGlamourThemeError) Error() string {
	return fmt.Sprintf("%s %q: must be one of '', auto, dark, light, notty", errMsgInvalidGlamourTheme, e.value)
}

// Unwrap returns errInvalidGlamourTheme for errors.Is() compatibility.
func (e *invalidGlamourThemeError) Unwrap() error { return errInvalidGlamourTheme }

// String returns the string representation of the colorMode.
func (m colorMode) String() string { return string(m) }

// Validate ensures colorMode is one of auto, always, or never.
func (m colorMode) Validate() error {
	switch m {
	case colorModeAuto, colorModeAlways, colorModeNever:
		return nil
	default:
		return &invalidColorModeError{value: m}
	}
}

// String returns the string representation of the glamourTheme.
func (t glamourTheme) String() string { return string(t) }

// Validate ensures glamourTheme is one of "", auto, dark, light, or notty.
func (t glamourTheme) Validate() error {
	switch t {
	case glamourThemeUnset, glamourThemeAuto, glamourThemeDark, glamourThemeLight, glamourThemeNotty:
		return nil
	default:
		return &invalidGlamourThemeError{value: t}
	}
}

//goplint:ignore -- implements pflag.Value.Set boundary for --ivk-color.
func (f *colorModeFlag) Set(raw string) error {
	parsed, err := parseColorMode(raw)
	if err != nil {
		return err
	}
	if f.raw != nil {
		*f.raw = parsed.String()
	}
	if f.app != nil {
		f.app.setColorMode(parsed)
	}
	return nil
}

func (f *colorModeFlag) String() string {
	if f.raw == nil || *f.raw == "" {
		return colorModeAuto.String()
	}
	return *f.raw
}

//goplint:ignore -- implements pflag.Value.Type.
func (f *colorModeFlag) Type() string { return "string" }

//goplint:ignore -- CLI boundary parses raw --ivk-color flag text into typed colorMode.
func parseColorMode(raw string) (colorMode, error) {
	mode := colorMode(raw)
	if err := mode.Validate(); err != nil {
		return "", err
	}
	return mode, nil
}

// resolveColorProfile resolves the effective colorprofile.Profile using the
// precedence: --ivk-color > NO_COLOR / CLICOLOR_FORCE / TERM=dumb > TTY detection.
// Non-empty NO_COLOR and --ivk-color=never map to NoTTY so ANSI formatting
// (including bold/underline decorations) is completely stripped.
func resolveColorProfile(mode colorMode, w io.Writer, getenv func(string) string) colorprofile.Profile {
	if mode == colorModeNever {
		return colorprofile.NoTTY
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if mode == colorModeAlways {
		env := buildColorEnv(getenv, false)
		if p := colorprofile.Env(env); p >= colorprofile.ANSI {
			return p
		}
		return colorprofile.TrueColor
	}

	// Standard no-color.org semantics: any non-empty NO_COLOR disables ANSI
	// output unless --ivk-color=always was explicitly requested.
	if getenv("NO_COLOR") != "" {
		return colorprofile.NoTTY
	}

	return colorprofile.Detect(w, buildColorEnv(getenv, true))
}

//goplint:ignore -- converts process color environment variables into KEY=VALUE pairs for colorprofile.
func buildColorEnv(getenv func(string) string, includeNoColor bool) []string {
	env := make([]string, 0, len(colorEnvKeys))
	for _, key := range colorEnvKeys {
		if key == "NO_COLOR" && !includeNoColor {
			continue
		}
		val := getenv(key)
		if val == "" {
			continue
		}
		if key == "CLICOLOR_FORCE" && val != "0" {
			// colorprofile.Detect only parses CLICOLOR_FORCE with strconv.ParseBool;
			// normalize any non-empty, non-"0" value (e.g. "2") per bixense clicolors.
			if _, err := strconv.ParseBool(val); err != nil {
				val = "1"
			}
		}
		env = append(env, key+"="+val)
	}
	return env
}

func colorEnabled(profile colorprofile.Profile) bool {
	return profile > colorprofile.ASCII
}

func colorEnabledForWriter(w io.Writer) bool {
	if cpw, ok := w.(*colorprofile.Writer); ok {
		return colorEnabled(cpw.Profile)
	}
	return colorEnabled(resolveColorProfile(colorModeAuto, w, os.Getenv))
}

// resolveGlamourStyle selects the Glamour theme orthogonally to color enablement:
// when color is disabled, Glamour renders with "notty" regardless of ui.color_scheme;
// when color is enabled, it maps ui.color_scheme ("auto", "dark", "light").
func resolveGlamourStyle(enabled bool, scheme config.ColorScheme) glamourTheme {
	if !enabled {
		return glamourThemeNotty
	}
	switch scheme {
	case config.ColorSchemeDark:
		return glamourThemeDark
	case config.ColorSchemeLight:
		return glamourThemeLight
	case config.ColorSchemeAuto, "":
		return glamourThemeAuto
	default:
		return glamourThemeAuto
	}
}

// explicitColorModeFromArgs scans raw argv before `--` for --ivk-color so help
// and error rendering honor the flag even when Cobra fails before flag parsing
// completes (for example `invowk --bogus --ivk-color=always`).
//
//goplint:ignore -- parses raw process argv tokens at CLI startup boundary.
func explicitColorModeFromArgs(args []string) (colorMode, bool) {
	var (
		mode colorMode
		set  bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		switch {
		case arg == longColorFlag:
			if i+1 < len(args) {
				if parsed, err := parseColorMode(args[i+1]); err == nil {
					mode = parsed
					set = true
				} else {
					mode = ""
					set = false
				}
				i++
			}
		case strings.HasPrefix(arg, longColorFlagInline):
			if parsed, err := parseColorMode(strings.TrimPrefix(arg, longColorFlagInline)); err == nil {
				mode = parsed
				set = true
			} else {
				mode = ""
				set = false
			}
		}
	}
	return mode, set
}

func colorModeFromCmd(cmd *cobra.Command) colorMode {
	if cmd == nil {
		return colorModeAuto
	}
	flag := cmd.Root().PersistentFlags().Lookup(colorFlagName)
	if flag == nil {
		return colorModeAuto
	}
	if parsed, err := parseColorMode(flag.Value.String()); err == nil {
		return parsed
	}
	return colorModeAuto
}

// isHelpInvocation reports whether args before `--` will trigger Cobra/fang help
// output (without executing any user command).
//
//goplint:ignore -- scans raw process argv tokens at CLI startup boundary.
func isHelpInvocation(args []string) bool {
	var (
		positionals    []string
		hasUnknownFlag bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		switch {
		case arg == "--help", arg == "-h":
			return true
		case arg == "--version":
			return false
		case arg == "--ivk-verbose", arg == "-v", arg == "--ivk-interactive", arg == "-i", arg == "--ivk-no-cwd",
			strings.HasPrefix(arg, "--ivk-verbose="), strings.HasPrefix(arg, "--ivk-interactive="), strings.HasPrefix(arg, "--ivk-no-cwd="):
			continue
		case arg == longColorFlag, arg == "--ivk-config", arg == "-c":
			if i+1 < len(args) {
				i++
			}
		case strings.HasPrefix(arg, longColorFlagInline), strings.HasPrefix(arg, "--ivk-config="),
			strings.HasPrefix(arg, "-c="), (strings.HasPrefix(arg, "-c") && len(arg) > 2):
			continue
		case strings.HasPrefix(arg, "-"):
			hasUnknownFlag = true
		default:
			positionals = append(positionals, arg)
		}
	}

	if hasUnknownFlag && len(positionals) == 0 {
		return false
	}

	if len(positionals) > 0 && positionals[0] == "help" {
		return true
	}

	switch len(positionals) {
	case 0:
		return true
	case 1:
		switch positionals[0] {
		case "module", "mod", "config", "tui", "agent":
			return true
		}
	case 2:
		if positionals[0] == "agent" && (positionals[1] == "cmd" || positionals[1] == "mod") {
			return true
		}
	}
	return false
}

// isTUIStyleOrFormatInvocation reports whether args invoke `invowk tui style`
// or `invowk tui format`, which remain opt-in to ANSI output unless
// --ivk-color=never is passed.
//
//goplint:ignore -- scans raw process argv tokens at CLI startup boundary.
func isTUIStyleOrFormatInvocation(args []string) bool {
	first, rest := firstTopLevelToken(args)
	if first != "tui" {
		return false
	}
	second, _ := firstTopLevelToken(rest)
	return second == "style" || second == "format"
}

// forceFangHelpColor temporarily sets CLICOLOR_FORCE=1 and clears NO_COLOR when
// --ivk-color=always is used for a help invocation so fang's internal
// colorprofile.NewWriter(c.OutOrStdout(), os.Environ()) emits ANSI styling in
// non-TTY streams without altering the environment of user commands.
func forceFangHelpColor() func() {
	origForce, hadForce := os.LookupEnv("CLICOLOR_FORCE")
	origNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	if err := os.Setenv("CLICOLOR_FORCE", "1"); err != nil {
		slog.Warn("failed to set CLICOLOR_FORCE for help color", "error", err)
	}
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		slog.Warn("failed to unset NO_COLOR for help color", "error", err)
	}

	return func() {
		if hadForce {
			if err := os.Setenv("CLICOLOR_FORCE", origForce); err != nil {
				slog.Warn("failed to restore CLICOLOR_FORCE", "error", err)
			}
		} else if err := os.Unsetenv("CLICOLOR_FORCE"); err != nil {
			slog.Warn("failed to unset CLICOLOR_FORCE", "error", err)
		}
		if hadNoColor {
			if err := os.Setenv("NO_COLOR", origNoColor); err != nil {
				slog.Warn("failed to restore NO_COLOR", "error", err)
			}
		} else if err := os.Unsetenv("NO_COLOR"); err != nil {
			slog.Warn("failed to unset NO_COLOR", "error", err)
		}
	}
}
