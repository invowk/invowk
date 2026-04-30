// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/app/deps"
	appexec "github.com/invowk/invowk/internal/app/execute"
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
		Config      config.Provider
		Discovery   DiscoveryService
		Commands    CommandService
		Watchers    WatchRunnerFactory
		Diagnostics DiagnosticRenderer
		stdout      io.Writer
		stderr      io.Writer
	}

	// Dependencies defines the injection points for building an App. Nil fields are
	// replaced with production defaults by NewApp. Tests can supply mock implementations
	// to isolate specific service behavior.
	Dependencies struct {
		Config      config.Provider
		Discovery   DiscoveryService
		Commands    CommandService
		Watchers    WatchRunnerFactory
		Diagnostics DiagnosticRenderer
		Stdout      io.Writer
		Stderr      io.Writer
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
	if d.Config == nil {
		d.Config = config.NewProvider()
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
		svc := commandsvc.NewWithPorts(
			d.Config,
			d.Discovery,
			captureUserEnv,
			commandsvc.LoadConfigWithFallback,
			hostAccess,
			registryFactory,
			interactiveExecutor,
			&cliExecutionObserver{stdout: d.Stdout},
			nil,
		)
		d.Commands = &cliCommandAdapter{svc: svc, stdout: d.Stdout}
	}
	if d.Watchers == nil {
		d.Watchers = productionWatchRunnerFactory{}
	}

	return &App{
		Config:      d.Config,
		Discovery:   d.Discovery,
		Commands:    d.Commands,
		Watchers:    d.Watchers,
		Diagnostics: d.Diagnostics,
		stdout:      d.Stdout,
		stderr:      d.Stderr,
	}, nil
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
	result, diags, err := a.svc.Execute(ctx, req)

	// Handle dry-run rendering: the service returns structured data;
	// the CLI adapter renders it with lipgloss styles.
	if result.DryRunData != nil {
		renderDryRun(
			a.stdout,
			req,
			&discovery.CommandInfo{SourceID: result.DryRunData.SourceID},
			result.DryRunData.ExecCtx,
			result.DryRunData.Selection,
		)
		return ExecuteResult{ExitCode: result.ExitCode}, diags, nil
	}

	if err != nil {
		err = renderAndWrapServiceError(err, req)
	}
	return ExecuteResult{ExitCode: result.ExitCode}, diags, err
}

// ResolveFromSource delegates source-filtered command selection to the command service.
func (a *cliCommandAdapter) ResolveFromSource(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return a.svc.ResolveFromSource(ctx, req)
}

// ResolveCommand delegates command selection to the command service.
func (a *cliCommandAdapter) ResolveCommand(ctx context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return a.svc.ResolveCommand(ctx, req)
}

// renderAndWrapServiceError inspects the raw domain error from the service and
// applies CLI rendering to produce a styled ServiceError. The error type
// determines the issue catalog ID and rendering function.
//
//plint:render
func renderAndWrapServiceError(err error, req ExecuteRequest) error {
	if depErr, ok := errors.AsType[*deps.DependencyError](err); ok {
		return newServiceError(err, issue.DependenciesNotSatisfiedId, RenderDependencyError(depErr))
	}

	if argErr, ok := errors.AsType[*deps.ArgumentValidationError](err); ok {
		return newServiceError(err, issue.InvalidArgumentId, RenderArgumentValidationError(argErr))
	}

	if notAllowed, ok := errors.AsType[*appexec.RuntimeNotAllowedError](err); ok {
		var allowed []string
		for _, r := range notAllowed.Allowed {
			allowed = append(allowed, string(r))
		}
		return newServiceError(
			err,
			issue.InvalidRuntimeModeId,
			RenderRuntimeNotAllowedError(req.Name, string(req.Runtime), strings.Join(allowed, ", ")),
		)
	}

	if classified, ok := errors.AsType[*commandsvc.ClassifiedError](err); ok {
		if ambigErr, ambigOK := errors.AsType[*commandsvc.AmbiguousCommandError](classified.Err); ambigOK {
			styledMsg := RenderAmbiguousCommandError(&AmbiguousCommandError{
				CommandName: ambigErr.CommandName,
				Sources:     ambigErr.Sources,
			})
			return newServiceError(classified.Err, 0, styledMsg)
		}
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
		return newServiceError(classified.Err, issueIDForServiceErrorKind(classified.Kind), styledMsg)
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
	case commandsvc.ErrorKindScriptExecutionFailed:
		return issue.ScriptExecutionFailedId
	default:
		return issue.ScriptExecutionFailedId
	}
}

// contextWithConfigPath attaches the explicit --ivk-config value and a per-request
// discovery cache to the context. The RunE handler calls this once; all downstream
// callees (runWorkspaceValidation, registerDiscoveredCommands, checkAmbiguousCommand,
// listCommands, executeRequest, runDisambiguatedCommand, and runWatchMode) share the
// same cache.
func contextWithConfigPath(ctx context.Context, configPath string) context.Context {
	return commandadapters.ContextWithConfigPath(ctx, configPath)
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
