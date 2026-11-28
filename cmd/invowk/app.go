// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const serviceErrorLabel = "Error:"

type (
	//goplint:mutable
	//
	// App wires CLI services and shared dependencies. It is the composition root for
	// the CLI layer — all Cobra command handlers receive an App reference and delegate
	// business logic through its service interfaces (Commands, Discovery, Config).
	App struct {
		Config        config.Provider
		Discovery     DiscoveryService
		Commands      CommandService
		Watchers      WatchRunnerCreator
		Diagnostics   DiagnosticRenderer
		WatchTerminal WatchTerminalOpener
		// Getenv is the only way CLI handlers read INVOWK_CONFIG and
		// INVOWK_NO_CWD, so tests never depend on the process environment.
		Getenv           func(string) string
		stdout           io.Writer
		stderr           io.Writer
		uiOut            *colorprofile.Writer
		uiErr            *colorprofile.Writer
		resolveUIProfile func(io.Writer) colorprofile.Profile
		alwaysColor      bool
	}

	// Dependencies defines the injection points for building an App. Nil fields are
	// replaced with production defaults by NewApp. Tests can supply mock implementations
	// to isolate specific service behavior.
	Dependencies struct {
		Config        config.Provider
		Discovery     DiscoveryService
		Commands      CommandService
		Watchers      WatchRunnerCreator
		Diagnostics   DiagnosticRenderer
		WatchTerminal WatchTerminalOpener
		// Getenv reads INVOWK_CONFIG and INVOWK_NO_CWD; nil means os.Getenv.
		// The default config provider is built from the same function.
		Getenv func(string) string
		Stdout io.Writer
		Stderr io.Writer
	}

	// ExecuteRequest is the CLI-facing alias for the command service request.
	// Cobra handlers construct it, while commandsvc owns validation and execution
	// semantics so the data contract has one source of truth.
	ExecuteRequest = commandsvc.Request

	//goplint:validate-all
	//
	// ExecuteResult contains command execution outcomes.
	ExecuteResult struct {
		ExitCode types.ExitCode
	}

	// CommandService executes a resolved command request and returns user-renderable
	// diagnostics. Implementations must not write directly to stdout/stderr; diagnostics
	// are returned as structured data for the CLI layer to render.
	CommandService interface {
		Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error)
		ResolveCommand(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error)
		ResolveWatchPlan(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, commandsvc.WatchPlan, []discovery.Diagnostic, error)
		ResolveFromSource(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error)
	}

	// DiscoveryService discovers invowk commands and diagnostics.
	// DiscoverCommandSet lists all available commands (for completion, listing).
	// DiscoverAndValidateCommandSet lists and validates the command tree (for registration).
	// GetCommand looks up a single command by name (for execution).
	DiscoveryService interface {
		DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
		DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
		DiscoverModules(ctx context.Context) (discovery.ModuleListResult, error)
		GetCommand(ctx context.Context, name string) (discovery.LookupResult, error)
	}

	// DiagnosticRenderer renders structured diagnostics.
	DiagnosticRenderer interface {
		Render(ctx context.Context, diags []discovery.Diagnostic, stderr io.Writer)
	}

	defaultDiagnosticRenderer struct{}

	// cliCommandAdapter wraps commandsvc.Service with CLI rendering.
	// It translates raw domain errors from the service into styled ServiceErrors
	// for CLI output, and handles dry-run rendering.
	cliCommandAdapter struct {
		svc    *commandsvc.Service
		stdout io.Writer
	}

	cliExecutionObserver struct {
		stdout io.Writer
	}
)

// NewApp creates an App with defaults for omitted dependencies.
func NewApp(d Dependencies) (*App, error) {
	if d.Stdout == nil {
		d.Stdout = os.Stdout
	}
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
	if d.Getenv == nil {
		d.Getenv = os.Getenv
	}
	uiOut := &colorprofile.Writer{
		Forward: d.Stdout,
		Profile: resolveColorProfile(colorModeAuto, d.Stdout, d.Getenv),
	}
	uiErr := &colorprofile.Writer{
		Forward: d.Stderr,
		Profile: resolveColorProfile(colorModeAuto, d.Stderr, d.Getenv),
	}
	if d.Config == nil {
		d.Config = config.NewProviderWithEnv(d.Getenv)
	}
	if d.Discovery == nil {
		d.Discovery = commandadapters.NewDiscoveryService(d.Config)
	}
	if d.Diagnostics == nil {
		d.Diagnostics = &defaultDiagnosticRenderer{}
	}
	if d.Commands == nil {
		hostAccess, err := commandadapters.NewHostAccess()
		if err != nil {
			return nil, err
		}
		registryFactory, err := commandadapters.NewRuntimeRegistryFactory()
		if err != nil {
			return nil, err
		}
		interactiveExecutor, err := commandadapters.NewInteractiveExecutor()
		if err != nil {
			return nil, err
		}
		requestScope, err := commandadapters.NewDiscoveryRequestScope()
		if err != nil {
			return nil, err
		}
		svc := commandsvc.New(
			d.Config,
			d.Discovery,
			captureUserEnv,
			commandsvc.LoadConfigWithFallback,
			commandsvc.NewPorts(
				hostAccess,
				registryFactory,
				interactiveExecutor,
				&cliExecutionObserver{stdout: d.Stdout},
				requestScope.Begin,
				commandadapters.NewDependencyCapabilityChecker(),
				commandadapters.NewDependencyHostProbe(),
				commandadapters.NewDependencyLockProvider(),
				commandadapters.NewDependencyScriptFileReader(),
			),
		)
		d.Commands = &cliCommandAdapter{svc: svc, stdout: uiOut}
	}
	if d.Watchers == nil {
		d.Watchers = productionWatchRunnerCreator{}
	}
	if d.WatchTerminal == nil {
		d.WatchTerminal = defaultWatchTerminalOpener{stdout: d.Stdout}
	}

	return &App{
		Config:           d.Config,
		Discovery:        d.Discovery,
		Commands:         d.Commands,
		Watchers:         d.Watchers,
		Diagnostics:      d.Diagnostics,
		WatchTerminal:    d.WatchTerminal,
		Getenv:           d.Getenv,
		stdout:           d.Stdout,
		stderr:           d.Stderr,
		uiOut:            uiOut,
		uiErr:            uiErr,
		resolveUIProfile: func(w io.Writer) colorprofile.Profile { return resolveColorProfile(colorModeAuto, w, d.Getenv) },
	}, nil
}

func (a *App) setColorMode(mode colorMode) {
	if a == nil {
		return
	}
	a.alwaysColor = mode == colorModeAlways
	a.resolveUIProfile = func(w io.Writer) colorprofile.Profile {
		return resolveColorProfile(mode, w, a.Getenv)
	}
	if a.uiOut != nil {
		a.uiOut.Profile = a.resolveUIProfile(a.stdout)
	}
	if a.uiErr != nil {
		a.uiErr.Profile = a.resolveUIProfile(a.stderr)
	}
}

func (a *App) outUI() io.Writer {
	if a != nil && a.uiOut != nil {
		return a.uiOut
	}
	if a != nil && a.stdout != nil {
		return a.stdout
	}
	return os.Stdout
}

func (a *App) errUI() io.Writer {
	if a != nil && a.uiErr != nil {
		return a.uiErr
	}
	if a != nil && a.stderr != nil {
		return a.stderr
	}
	return os.Stderr
}

func (a *App) uiWriterForOut(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		if out := cmd.OutOrStdout(); out != os.Stdout {
			return &colorprofile.Writer{
				Forward: out,
				Profile: a.profileForWriter(out),
			}
		}
	}
	if a != nil && a.uiOut != nil {
		return a.uiOut
	}
	return os.Stdout
}

func (a *App) uiWriterForErr(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		if errW := cmd.ErrOrStderr(); errW != os.Stderr {
			return &colorprofile.Writer{
				Forward: errW,
				Profile: a.profileForWriter(errW),
			}
		}
	}
	if a != nil && a.uiErr != nil {
		return a.uiErr
	}
	return os.Stderr
}

func (a *App) profileForWriter(w io.Writer) colorprofile.Profile {
	if a != nil && a.resolveUIProfile != nil {
		return a.resolveUIProfile(w)
	}
	var getenv func(string) string
	if a != nil {
		getenv = a.Getenv
	}
	return resolveColorProfile(colorModeAuto, w, getenv)
}

func (a *App) glamourStyle(ctx context.Context, rootFlags *rootFlagValues) glamourTheme {
	if a == nil || a.uiErr == nil || !colorEnabled(a.uiErr.Profile) {
		return glamourThemeNotty
	}
	scheme := config.ColorSchemeAuto
	if cfg, err := loadUIConfig(ctx, a, rootFlags); err == nil && cfg != nil {
		scheme = cfg.UI.ColorScheme
	}
	style := resolveGlamourStyle(true, scheme)
	// Glamour's "auto" style checks os.Stdout.Fd() and falls back to "notty"
	// when stdout is piped; use "dark" when color is forced/enabled on stderr
	// while stdout is non-TTY so styling is not silently dropped.
	if style == glamourThemeAuto && !stdoutIsTerminal(a.stdout) {
		return glamourThemeDark
	}
	return style
}

func (o *cliExecutionObserver) CommandStarting(name invowkfile.CommandName) {
	fmt.Fprintf(o.stdout, "-> Running '%s'...\n", name)
}

func (o *cliExecutionObserver) InteractiveFallback(runtimeName invowkfile.RuntimeMode) {
	fmt.Fprintf(o.stdout, "! Runtime '%s' does not support interactive mode, using standard execution\n", runtimeName)
}

// Execute translates an ExecuteRequest into a commandsvc.Request, delegates
// to the underlying service, and wraps raw domain errors into styled
// ServiceErrors for CLI rendering. Dry-run results are rendered here.
func (a *cliCommandAdapter) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	if err := req.Validate(); err != nil {
		return ExecuteResult{}, nil, err
	}
	return a.executeValidated(ctx, req)
}

// ResolveFromSource delegates source-filtered command selection to the command service.
func (a *cliCommandAdapter) ResolveFromSource(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	if err := req.Validate(); err != nil {
		return nil, ExecuteRequest{}, nil, err
	}
	cmdInfo, resolvedReq, commandDiags, err := a.svc.ResolveFromSource(ctx, req)
	return cmdInfo, resolvedReq, convertCommandDiagnostics(commandDiags), err
}

// ResolveCommand delegates command selection to the command service.
func (a *cliCommandAdapter) ResolveCommand(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	if err := req.Validate(); err != nil {
		return nil, ExecuteRequest{}, nil, err
	}
	cmdInfo, resolvedReq, commandDiags, err := a.svc.ResolveCommand(ctx, req)
	return cmdInfo, resolvedReq, convertCommandDiagnostics(commandDiags), err
}

// ResolveWatchPlan delegates watch command selection and planning to the command service.
func (a *cliCommandAdapter) ResolveWatchPlan(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, commandsvc.WatchPlan, []discovery.Diagnostic, error) {
	if err := req.Validate(); err != nil {
		return nil, ExecuteRequest{}, commandsvc.WatchPlan{}, nil, err
	}
	cmdInfo, resolvedReq, plan, commandDiags, err := a.svc.ResolveWatchPlan(ctx, req)
	return cmdInfo, resolvedReq, plan, convertCommandDiagnostics(commandDiags), err
}

func (a *cliCommandAdapter) executeValidated(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	result, commandDiags, err := a.svc.Execute(ctx, req)
	diags := convertCommandDiagnostics(commandDiags)

	// Handle dry-run rendering: the service returns structured data;
	// the CLI adapter renders it with lipgloss styles.
	if result.DryRunData != nil {
		renderDryRun(a.stdout, result.DryRunData.Plan)
		return ExecuteResult{ExitCode: result.ExitCode}, diags, nil
	}

	if err != nil {
		err = renderAndWrapServiceError(err, req, diags)
	}
	return ExecuteResult{ExitCode: result.ExitCode}, diags, err
}

func convertCommandDiagnostics(diags []commandsvc.Diagnostic) []discovery.Diagnostic {
	result := make([]discovery.Diagnostic, 0, len(diags))
	for _, diag := range diags {
		converted, err := discovery.NewDiagnosticWithCause(
			discovery.Severity(diag.Severity()),
			discovery.DiagnosticCode(diag.Code()),
			diag.Message().String(),
			diag.Path(),
			diag.Cause(),
		)
		if err != nil {
			slog.Error("BUG: failed to bridge command diagnostic to CLI diagnostic",
				"code", diag.Code(), "error", err)
			continue
		}
		if diag.FromCurrentDir() {
			converted = converted.WithCurrentDir()
		}
		result = append(result, converted)
	}
	return result
}

// renderAndWrapServiceError inspects the raw domain error from the service and
// applies CLI rendering to produce a styled ServiceError. The error type
// determines the issue catalog ID and rendering function. Errors caused by the
// shape of the command line (argument and flag validation, unknown or
// ambiguous command, unknown source, runtime not allowed) are additionally
// wrapped in UsageError so the process exits with ExitCodeUsage. diags are the
// discovery diagnostics of the same request: a not-found error stays exit 1
// when they report a source that failed to load (notFoundError).
//
//plint:render
func renderAndWrapServiceError(err error, req ExecuteRequest, diags []discovery.Diagnostic) error {
	if depErr, ok := errors.AsType[*deps.DependencyError](err); ok {
		return newServiceError(err, issue.DependenciesNotSatisfiedId, RenderDependencyError(depErr))
	}

	if flagErr, ok := errors.AsType[*deps.FlagValidationError](err); ok {
		return newUsageError(newServiceError(err, issue.InvalidArgumentId, RenderFlagValidationError(flagErr)))
	}

	if argErr, ok := errors.AsType[*deps.ArgumentValidationError](err); ok {
		return newUsageError(newServiceError(err, issue.InvalidArgumentId, RenderArgumentValidationError(argErr)))
	}

	if ambigErr, ok := errors.AsType[*commandsvc.AmbiguousCommandError](err); ok {
		styledMsg := RenderAmbiguousCommandError(&AmbiguousCommandError{
			CommandName: ambigErr.CommandName,
			Sources:     ambigErr.Sources,
		})
		return newUsageError(newServiceError(ambigErr, 0, styledMsg))
	}

	if sourceErr, ok := errors.AsType[*commandsvc.SourceNotFoundError](err); ok {
		styledMsg := RenderSourceNotFoundError(&SourceNotFoundError{
			Source:           sourceErr.Source,
			AvailableSources: sourceErr.AvailableSources,
		})
		return notFoundError(newServiceError(sourceErr, issue.CommandNotFoundId, styledMsg), diags, req.FromSource)
	}

	if platformErr, ok := errors.AsType[*commandsvc.UnsupportedPlatformError](err); ok {
		supported := make([]string, 0, len(platformErr.Supported))
		for i := range platformErr.Supported {
			supported = append(supported, string(platformErr.Supported[i]))
		}
		return newServiceError(
			err,
			issue.HostNotSupportedId,
			RenderHostNotSupportedError(string(platformErr.CommandName), string(platformErr.Current), strings.Join(supported, ", ")),
		)
	}

	if notAllowed, ok := errors.AsType[*commandsvc.RuntimeNotAllowedError](err); ok {
		var allowed []string
		for _, r := range notAllowed.Allowed {
			allowed = append(allowed, string(r))
		}
		return newUsageError(newServiceError(
			err,
			issue.InvalidRuntimeModeId,
			RenderRuntimeNotAllowedError(req.Name, string(req.Runtime), strings.Join(allowed, ", ")),
		))
	}

	if classified, ok := errors.AsType[*commandsvc.ClassifiedError](err); ok {
		// Re-create the styled message using the CLI-layer error formatter.
		var styledMsg string
		styledLabel := ErrorStyle.Render(serviceErrorLabel)
		switch classified.Message {
		case commandsvc.HintTimedOut:
			styledMsg = fmt.Sprintf("\n%s command timed out: %s\n", styledLabel, formatErrorForDisplay(classified.Err, req.Verbose))
		case commandsvc.HintCancelled:
			styledMsg = fmt.Sprintf("\n%s command was cancelled: %s\n", styledLabel, formatErrorForDisplay(classified.Err, req.Verbose))
		default:
			styledMsg = fmt.Sprintf("\n%s %s\n", styledLabel, formatErrorForDisplay(classified.Err, req.Verbose))
		}
		svcErr := newServiceError(classified.Err, issueIDForServiceErrorKind(classified.Kind), styledMsg)
		if classified.Kind == commandsvc.ErrorKindCommandNotFound {
			return notFoundError(svcErr, diags, req.FromSource)
		}
		if classified.Kind == commandsvc.ErrorKindCommandAmbiguous {
			return newUsageError(svcErr)
		}
		return svcErr
	}

	return err
}

func issueIDForServiceErrorKind(kind commandsvc.ErrorKind) issue.Id {
	switch kind {
	case commandsvc.ErrorKindCommandAmbiguous:
		return issue.CommandNotFoundId
	case commandsvc.ErrorKindCommandNotFound:
		return issue.CommandNotFoundId
	case commandsvc.ErrorKindContainerEngineNotFound:
		return issue.ContainerEngineNotFoundId
	case commandsvc.ErrorKindRuntimeNotAvailable:
		return issue.RuntimeNotAvailableId
	case commandsvc.ErrorKindPermissionDenied:
		return issue.PermissionDeniedId
	case commandsvc.ErrorKindShellNotFound:
		return issue.ShellNotFoundId
	case commandsvc.ErrorKindTUIExposureRefused:
		return issue.TUIExposureRefusedId
	case commandsvc.ErrorKindScriptExecutionFailed:
		return issue.ScriptExecutionFailedId
	default:
		return issue.ScriptExecutionFailedId
	}
}

// configPathFromContext extracts the explicit config path from context.
func configPathFromContext(ctx context.Context) string {
	return commandadapters.ConfigPathFromContext(ctx)
}

// contextWithDiscoveryRequestCache attaches a per-request discovery cache.
func contextWithDiscoveryRequestCache(ctx context.Context) context.Context {
	return commandadapters.ContextWithDiscoveryRequestCache(ctx)
}

// captureUserEnv captures the current environment as a map.
// This should be called at the start of execution to capture the user's
// actual environment before invowk sets any command-level env vars.
func captureUserEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if key, value, found := strings.Cut(e, "="); found {
			env[key] = value
		}
	}
	return env
}

// lookupFromCommandSet resolves a command lookup against an already-discovered
// command set, matching discovery.GetCommand behavior without repeating scans.
func lookupFromCommandSet(commandSetResult discovery.CommandSetResult, cmdName invowkfile.CommandName) (discovery.LookupResult, error) {
	if err := cmdName.Validate(); err != nil {
		return discovery.LookupResult{}, fmt.Errorf("invalid command name: %w", err)
	}

	diagnostics := slices.Clone(commandSetResult.Diagnostics)
	if cmd, ok := commandSetResult.Set.ByName[cmdName]; ok {
		return discovery.LookupResult{
			Command:     cmd,
			Diagnostics: diagnostics,
		}, nil
	}

	notFound, err := discovery.NewDiagnostic(
		discovery.SeverityError,
		discovery.CodeCommandNotFound,
		fmt.Sprintf("command '%s' not found", cmdName),
	)
	if err != nil {
		return discovery.LookupResult{}, fmt.Errorf("create command-not-found diagnostic: %w", err)
	}

	diagnostics = append(diagnostics, notFound)
	return discovery.LookupResult{Diagnostics: diagnostics}, nil
}

// Render writes structured diagnostics to stderr with lipgloss styling.
func (r *defaultDiagnosticRenderer) Render(_ context.Context, diags []discovery.Diagnostic, stderr io.Writer) {
	for _, diag := range diags {
		prefix := WarningStyle.Render("warning")
		if diag.Severity() == discovery.SeverityError {
			prefix = ErrorStyle.Render("error")
		}

		if diag.Path() != "" {
			_, _ = fmt.Fprintf(stderr, "%s: %s (%s)\n", prefix, diag.Message(), diag.Path())
			continue
		}

		_, _ = fmt.Fprintf(stderr, "%s: %s\n", prefix, diag.Message())
	}
}
