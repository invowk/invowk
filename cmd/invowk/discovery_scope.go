// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
)

const (
	// noCwdFlagName is the root persistent flag that disables current-directory
	// command sources (the cwd invowkfile and sibling modules).
	noCwdFlagName = "ivk-no-cwd"
	// envNoCwd is the environment form of --ivk-no-cwd, parsed with strconv.ParseBool.
	envNoCwd = "INVOWK_NO_CWD"

	longNoCwdFlag       = "--" + noCwdFlagName
	longNoCwdFlagInline = longNoCwdFlag + "="

	// noCwdOriginFlag means --ivk-no-cwd decided the switch.
	noCwdOriginFlag noCwdOrigin = longNoCwdFlag
	// noCwdOriginEnv means INVOWK_NO_CWD decided the switch (or nothing set it).
	noCwdOriginEnv noCwdOrigin = envNoCwd

	noCwdAcceptedValues = "1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False"

	errMsgInvalidNoCwdEnv    = "invalid INVOWK_NO_CWD value"
	errMsgInvalidNoCwdOrigin = "invalid no-cwd origin"
	errMsgVerifyNeedsCwd     = "--verify needs current-directory discovery, which --ivk-no-cwd (or INVOWK_NO_CWD) disables; drop --verify or the switch"
)

var (
	// ErrInvalidNoCwdEnv marks an INVOWK_NO_CWD value that strconv.ParseBool rejects.
	ErrInvalidNoCwdEnv = errors.New(errMsgInvalidNoCwdEnv)
	// ErrInvalidNoCwdOrigin is returned when a noCwdOrigin value is not recognized.
	ErrInvalidNoCwdOrigin = errors.New(errMsgInvalidNoCwdOrigin)
	// ErrVerifyNeedsCwd marks `agent cmd create|change --verify` under --ivk-no-cwd.
	ErrVerifyNeedsCwd = errors.New(errMsgVerifyNeedsCwd)

	// The production discovery adapter must keep satisfying the UI config
	// loader; a signature drift would silently bypass the request cache.
	_ uiConfigLoader = (*commandadapters.DiscoveryService)(nil)
)

type (
	// noCwdOrigin names what decided the --ivk-no-cwd switch. The empty value
	// means the switch is off for the current request.
	noCwdOrigin string

	// InvalidNoCwdEnvError reports an INVOWK_NO_CWD value that is neither empty
	// nor accepted by strconv.ParseBool. It wraps ErrInvalidNoCwdEnv.
	InvalidNoCwdEnvError struct {
		Value string //goplint:ignore -- raw environment value echoed in the error message.
	}

	// InvalidNoCwdOriginError is returned when a noCwdOrigin value is not recognized.
	InvalidNoCwdOriginError struct {
		Value noCwdOrigin
	}

	// uiConfigLoader is the request-cached config loader that the discovery
	// adapter offers; loadUIConfig uses it so UI flags share the request's
	// config (and its explicit-source error).
	uiConfigLoader interface {
		LoadConfig(ctx context.Context) (*config.Config, []discovery.Diagnostic, error)
	}
)

// Error implements the error interface for InvalidNoCwdEnvError.
func (e *InvalidNoCwdEnvError) Error() string {
	return fmt.Sprintf("%s %q: accepted values are %s (empty means off)", errMsgInvalidNoCwdEnv, e.Value, noCwdAcceptedValues)
}

// Unwrap returns ErrInvalidNoCwdEnv for errors.Is() compatibility.
func (e *InvalidNoCwdEnvError) Unwrap() error { return ErrInvalidNoCwdEnv }

// Error implements the error interface for InvalidNoCwdOriginError.
func (e *InvalidNoCwdOriginError) Error() string {
	return fmt.Sprintf("%s %q", errMsgInvalidNoCwdOrigin, e.Value)
}

// Unwrap returns ErrInvalidNoCwdOrigin for errors.Is() compatibility.
func (e *InvalidNoCwdOriginError) Unwrap() error { return ErrInvalidNoCwdOrigin }

func (o noCwdOrigin) String() string { return string(o) }

// Validate accepts the two origins and the empty "switch off" value.
func (o noCwdOrigin) Validate() error {
	switch o {
	case "", noCwdOriginFlag, noCwdOriginEnv:
		return nil
	default:
		return &InvalidNoCwdOriginError{Value: o}
	}
}

// resolveNoCwd returns the effective --ivk-no-cwd switch. An explicitly given
// flag decides on its own and INVOWK_NO_CWD is then neither parsed nor
// validated (same "explicit flag beats env" gate as llmconfig ChangedFlags).
// Otherwise INVOWK_NO_CWD is parsed with strconv.ParseBool; empty means off.
// Only flows that discover command sources call it, so an invalid env value
// cannot break --version or `config path`.
func resolveNoCwd(flagSet *pflag.FlagSet, rootFlags *rootFlagValues, getenv func(string) string) (on bool, origin noCwdOrigin, err error) {
	if flagSet != nil && rootFlags != nil && flagSet.Changed(noCwdFlagName) {
		return rootFlags.noCwd, noCwdOriginFlag, nil
	}
	return noCwdFromEnv(getenv)
}

// explicitNoCwdFromArgs scans raw argv for --ivk-no-cwd before Cobra parses
// flags, so dynamic command registration sees the switch whether it appears
// before or after the `cmd` token. A `--` ends the scan: anything after it is
// an argument to the user command. The last occurrence wins, as in pflag.
//
//goplint:ignore -- parses raw process argv tokens at CLI startup boundary.
func explicitNoCwdFromArgs(args []string) (value, set bool, err error) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		switch {
		case arg == longNoCwdFlag:
			value, set = true, true
		case strings.HasPrefix(arg, longNoCwdFlagInline):
			parsed, parseErr := strconv.ParseBool(strings.TrimPrefix(arg, longNoCwdFlagInline))
			if parseErr != nil {
				return false, true, fmt.Errorf("invalid %s value: %w", longNoCwdFlag, parseErr)
			}
			value, set = parsed, true
		}
	}
	return value, set, nil
}

// noCwdForRegistration resolves the switch for the pre-parse registration step:
// the argv flag when present, otherwise INVOWK_NO_CWD.
//
//goplint:ignore -- parses raw process argv tokens at CLI startup boundary.
func noCwdForRegistration(args []string, getenv func(string) string) (bool, error) {
	value, set, err := explicitNoCwdFromArgs(args)
	if err != nil {
		return false, err
	}
	if set {
		return value, nil
	}
	on, _, err := noCwdFromEnv(getenv)
	return on, err
}

// contextWithDiscoveryScope attaches the request's discovery scope: the
// --ivk-no-cwd switch first, then the explicit config path and the per-request
// discovery cache. Every command-source discovery entry goes through it, so no
// entry can set the config path without the switch. The cache is only reused
// from a parent context with the same switch value.
func contextWithDiscoveryScope(ctx context.Context, configPath string, noCwd bool) context.Context {
	ctx = commandadapters.ContextWithNoCwd(ctx, noCwd)
	return commandadapters.ContextWithConfigPath(ctx, configPath)
}

// scopeDiscovery resolves --ivk-no-cwd for cmd and attaches the discovery scope
// to its context. The returned origin is empty when the switch is off.
func scopeDiscovery(cmd *cobra.Command, app *App, rootFlags *rootFlagValues) (context.Context, noCwdOrigin, error) {
	on, origin, err := resolveNoCwd(cmd.Root().PersistentFlags(), rootFlags, app.Getenv)
	if err != nil {
		return cmd.Context(), "", err
	}
	if err := origin.Validate(); err != nil {
		return cmd.Context(), "", err
	}
	ctx := contextWithDiscoveryScope(cmd.Context(), rootFlags.configPath, on)
	cmd.SetContext(ctx)
	if !on {
		return ctx, "", nil
	}
	return ctx, origin, nil
}

// activeNoCwdOrigin returns what turned the switch on for cmd's request, or
// the empty origin when cmd's context has the switch off.
func activeNoCwdOrigin(cmd *cobra.Command, app *App, rootFlags *rootFlagValues) noCwdOrigin {
	if !noCwdFromContext(cmd.Context()) {
		return ""
	}
	on, origin, err := resolveNoCwd(cmd.Root().PersistentFlags(), rootFlags, app.Getenv)
	if err != nil || !on {
		// The request was scoped with the switch on, so the decider can only
		// be the env when the flag form is absent.
		return noCwdOriginEnv
	}
	return origin
}

// noCwdFromContext reports the switch already attached to ctx.
func noCwdFromContext(ctx context.Context) bool {
	return commandadapters.NoCwdFromContext(ctx)
}

// noCwdDisabledLine is the stdout line that names the switch and its origin.
//
//plint:render
func noCwdDisabledLine(origin noCwdOrigin) string {
	return fmt.Sprintf("current-directory sources disabled (%s)", origin)
}

// verifyNoCwdError resolves the switch only when --verify is set and returns
// ErrVerifyNeedsCwd when it is on (or the resolution error itself), so
// `agent cmd create|change --verify` is rejected before any LLM call.
func verifyNoCwdError(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, verify bool) error {
	if !verify {
		return nil
	}
	on, _, err := resolveNoCwd(cmd.Root().PersistentFlags(), rootFlags, app.Getenv)
	if err != nil {
		return err
	}
	if on {
		return ErrVerifyNeedsCwd
	}
	return nil
}

// renderExplicitConfigError renders the explicit_config_load_failed issue
// entry (never the user-file config_load_failed advice to run `config init` or
// delete the user file) plus a per-source hint when err is a failed explicit
// config source, and returns err unchanged either way. Callers return the
// error from RunE as a plain error, so the process exits 1.
func renderExplicitConfigError(w io.Writer, err error) error {
	explicitErr, ok := errors.AsType[*config.ExplicitConfigError](err)
	if !ok {
		return err
	}
	printIssueCatalogEntry(w, issue.ExplicitConfigLoadFailedId)
	fmt.Fprintf(w, "%s %s\n", WarningStyle.Render("Hint:"), explicitConfigHint(explicitErr))
	return err
}

func noCwdFromEnv(getenv func(string) string) (bool, noCwdOrigin, error) {
	if getenv == nil {
		return false, noCwdOriginEnv, nil
	}
	raw := getenv(envNoCwd)
	if raw == "" {
		return false, noCwdOriginEnv, nil
	}
	on, err := strconv.ParseBool(raw)
	if err != nil {
		return false, noCwdOriginEnv, &InvalidNoCwdEnvError{Value: raw}
	}
	return on, noCwdOriginEnv, nil
}

// explicitConfigHint says how to fix a failed explicit source. The env hint
// mentions the absolute-path rule only when that rule is what failed.
//
//plint:render
func explicitConfigHint(explicitErr *config.ExplicitConfigError) string {
	if explicitErr.Source != config.ConfigSourceEnv {
		return "check the " + config.ConfigFlagName + " path"
	}
	if errors.Is(explicitErr, config.ErrConfigEnvNotAbsolute) {
		return "fix or unset " + config.EnvConfigPath + "; it must be an absolute path"
	}
	return "fix or unset " + config.EnvConfigPath
}
