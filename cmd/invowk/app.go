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
	"sync"

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
	configPathContextKey            struct{}
	discoveryRequestCacheContextKey struct{}

	//goplint:mutable
	//
	// App wires CLI services and shared dependencies. It is the composition root for
	// the CLI layer — all Cobra command handlers receive an App reference and delegate
	// business logic through its service interfaces (Commands, Discovery, Config).
	App struct {
		Config      config.Provider
		Discovery   DiscoveryService
		Commands    CommandService
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
	}

	// DiscoveryService discovers invowk commands and diagnostics.
	// DiscoverCommandSet lists all available commands (for completion, listing).
	// DiscoverAndValidateCommandSet lists and validates the command tree (for registration).
	// GetCommand looks up a single command by name (for execution).
	DiscoveryService interface {
		DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
		DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
		GetCommand(ctx context.Context, name string) (discovery.LookupResult, error)
	}

	// DiagnosticRenderer renders structured diagnostics.
	DiagnosticRenderer interface {
		Render(ctx context.Context, diags []discovery.Diagnostic, stderr io.Writer)
	}

	// appDiscoveryService implements DiscoveryService with per-request memoization.
	// On cache miss, it creates a discovery.Discovery instance, runs the operation,
	// and caches the result. Configuration diagnostics are prepended on every path
	// since the config may vary by context path.
	appDiscoveryService struct {
		config config.Provider
	}

	// lookupCacheEntry holds a memoized GetCommand result and its associated error.
	lookupCacheEntry struct {
		result discovery.LookupResult
		err    error
	}

	// discoveryRequestCache stores discovery results for a single command/request
	// context to avoid repeated filesystem scans/parsing during one invocation.
	discoveryRequestCache struct {
		mu sync.Mutex

		hasConfig bool
		cfg       *config.Config
		cfgDiags  []discovery.Diagnostic

		hasCommandSet bool
		commandSet    discovery.CommandSetResult
		commandSetErr error

		hasValidatedSet bool
		validatedSet    discovery.CommandSetResult
		validatedSetErr error

		lookups map[string]lookupCacheEntry
	}

	defaultDiagnosticRenderer struct{}

	// cliCommandAdapter wraps commandsvc.Service with CLI rendering.
	// It translates raw domain errors from the service into styled ServiceErrors
	// for CLI output, and handles dry-run rendering.
	cliCommandAdapter struct {
		svc    *commandsvc.Service
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
		d.Discovery = &appDiscoveryService{config: d.Config}
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
			d.Stdout,
			d.Stderr,
			captureUserEnv,
			loadConfigWithFallback,
			hostAccess,
			registryFactory,
			interactiveExecutor,
		)
		d.Commands = &cliCommandAdapter{svc: svc, stdout: d.Stdout}
	}

	return &App{
		Config:      d.Config,
		Discovery:   d.Discovery,
		Commands:    d.Commands,
		Diagnostics: d.Diagnostics,
		stdout:      d.Stdout,
		stderr:      d.Stderr,
	}, nil
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
		return newServiceError(classified.Err, classified.IssueID, styledMsg)
	}

	return err
}

// contextWithConfigPath attaches the explicit --ivk-config value and a per-request
// discovery cache to the context. The RunE handler calls this once; all downstream
// callees (runWorkspaceValidation, registerDiscoveredCommands, checkAmbiguousCommand,
// listCommands, executeRequest, runDisambiguatedCommand, and runWatchMode) share the
// same cache.
func contextWithConfigPath(ctx context.Context, configPath string) context.Context {
	ctx = contextWithDiscoveryRequestCache(ctx)
	return context.WithValue(ctx, configPathContextKey{}, configPath)
}

// configPathFromContext extracts the explicit config path from context.
func configPathFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(configPathContextKey{}).(string); ok {
		return v
	}

	return ""
}

// contextWithDiscoveryRequestCache attaches a per-request discovery cache.
func contextWithDiscoveryRequestCache(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(discoveryRequestCacheContextKey{}).(*discoveryRequestCache); ok {
		return ctx
	}
	return context.WithValue(ctx, discoveryRequestCacheContextKey{}, &discoveryRequestCache{
		lookups: make(map[string]lookupCacheEntry),
	})
}

func discoveryCacheFromContext(ctx context.Context) *discoveryRequestCache {
	if cache, ok := ctx.Value(discoveryRequestCacheContextKey{}).(*discoveryRequestCache); ok {
		return cache
	}
	return nil
}

// Validate verifies cached typed values when present.
func (c *discoveryRequestCache) Validate() error {
	if c == nil {
		return nil
	}

	var errs []error
	if c.cfg != nil {
		if err := c.cfg.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, diag := range c.cfgDiags {
		if err := diag.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// prependDiags returns a new slice with prefix diagnostics before existing ones.
func prependDiags(existing, prefix []discovery.Diagnostic) []discovery.Diagnostic {
	if len(prefix) == 0 {
		return existing
	}
	return append(append(make([]discovery.Diagnostic, 0, len(prefix)+len(existing)), prefix...), existing...)
}

func prependCommandSetDiagnostics(result discovery.CommandSetResult, cfgDiags []discovery.Diagnostic) discovery.CommandSetResult {
	result.Diagnostics = prependDiags(result.Diagnostics, cfgDiags)
	return result
}

func prependLookupDiagnostics(result discovery.LookupResult, cfgDiags []discovery.Diagnostic) discovery.LookupResult {
	result.Diagnostics = prependDiags(result.Diagnostics, cfgDiags)
	return result
}

// DiscoverCommandSet discovers commands and prepends configuration diagnostics.
// Results are memoized within the request context when available.
func (s *appDiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		if cache.hasCommandSet {
			result := cache.commandSet
			err := cache.commandSetErr
			cache.mu.Unlock()
			return prependCommandSetDiagnostics(result, cfgDiags), err
		}
		cache.mu.Unlock()
	}

	result, err := discovery.New(cfg).DiscoverCommandSet(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		cache.hasCommandSet = true
		cache.commandSet = result
		cache.commandSetErr = err
		cache.mu.Unlock()
	}

	return prependCommandSetDiagnostics(result, cfgDiags), err
}

// DiscoverAndValidateCommandSet discovers commands, validates the command tree,
// and prepends configuration diagnostics.
func (s *appDiscoveryService) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		if cache.hasValidatedSet {
			result := cache.validatedSet
			err := cache.validatedSetErr
			cache.mu.Unlock()
			return prependCommandSetDiagnostics(result, cfgDiags), err
		}
		cache.mu.Unlock()
	}

	result, err := discovery.New(cfg).DiscoverAndValidateCommandSet(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		cache.hasValidatedSet = true
		cache.validatedSet = result
		cache.validatedSetErr = err
		// Cross-populate: the validated set can satisfy DiscoverCommandSet() calls,
		// avoiding a redundant discovery pass. Only populate when discovery succeeded
		// (result.Set != nil); tree validation errors are orthogonal to discovery,
		// so commandSetErr stays nil. When discovery itself fails, result.Set is nil
		// and we must not cache a zero-value result that would mask the real error.
		if !cache.hasCommandSet && result.Set != nil {
			cache.hasCommandSet = true
			cache.commandSet = result
			cache.commandSetErr = nil
		}
		cache.mu.Unlock()
	}

	return prependCommandSetDiagnostics(result, cfgDiags), err
}

// GetCommand looks up a command by name and prepends configuration diagnostics.
func (s *appDiscoveryService) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		if entry, ok := cache.lookups[name]; ok {
			cache.mu.Unlock()
			return prependLookupDiagnostics(entry.result, cfgDiags), entry.err
		}
		if cache.hasCommandSet && cache.commandSetErr == nil && cache.commandSet.Set != nil {
			result, err := lookupFromCommandSet(cache.commandSet, invowkfile.CommandName(name)) //goplint:ignore -- CLI lookup input, validated in helper
			cache.lookups[name] = lookupCacheEntry{result: result, err: err}
			cache.mu.Unlock()
			return prependLookupDiagnostics(result, cfgDiags), err
		}
		cache.mu.Unlock()
	}

	result, err := discovery.New(cfg).GetCommand(ctx, name)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		cache.lookups[name] = lookupCacheEntry{result: result, err: err}
		cache.mu.Unlock()
	}

	return prependLookupDiagnostics(result, cfgDiags), err
}

// loadConfig returns configuration for discovery calls. On load failure, it keeps
// discovery operational with defaults and emits a warning diagnostic for the CLI.
func (s *appDiscoveryService) loadConfig(ctx context.Context) (*config.Config, []discovery.Diagnostic) {
	configPath := configPathFromContext(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		if cache.hasConfig {
			cfg := cache.cfg
			diags := slices.Clone(cache.cfgDiags)
			cache.mu.Unlock()
			return cfg, diags
		}
		cache.mu.Unlock()
	}

	cfg, diags := loadConfigWithFallback(ctx, s.config, configPath)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		if !cache.hasConfig {
			cache.hasConfig = true
			cache.cfg = cfg
			cache.cfgDiags = slices.Clone(diags)
		}
		cache.mu.Unlock()
	}

	return cfg, diags
}

// loadConfigWithFallback loads configuration via the provider. On failure it
// returns defaults with a diagnostic so callers stay operational.
//
// Diagnostic severity depends on the failure mode:
//   - Explicit --ivk-config path: always SeverityError (user-specified file must work).
//   - Default path with existing but malformed file: SeverityError (syntax errors
//     in a file the user created should not be silently downgraded to a warning).
//   - Default path with missing config dir or similar infrastructure error:
//     SeverityWarning (common on fresh installs, defaults are appropriate).
func loadConfigWithFallback(ctx context.Context, provider config.Provider, configPath string) (*config.Config, []discovery.Diagnostic) {
	cfg, err := provider.Load(ctx, config.LoadOptions{ConfigFilePath: types.FilesystemPath(configPath)})
	if err == nil {
		return cfg, nil
	}

	// When the user explicitly specified a config path, do not silently fall back
	// to defaults — surface the error as a diagnostic so downstream callers can
	// decide whether to abort.
	if configPath != "" {
		diag, diagErr := discovery.NewDiagnosticWithCause(
			discovery.SeverityError,
			discovery.CodeConfigLoadFailed,
			fmt.Sprintf("failed to load config from %s: %v", configPath, err),
			types.FilesystemPath(configPath),
			err,
		)
		if diagErr != nil {
			slog.Error("BUG: failed to create config-load diagnostic", "error", diagErr)
			return config.DefaultConfig(), nil
		}
		return config.DefaultConfig(), []discovery.Diagnostic{diag}
	}

	// Default config path: differentiate "file exists but is broken" (syntax error,
	// schema violation) from "cannot determine config dir" (missing HOME, etc.).
	// The config loader only returns errors for existing files; missing files silently
	// return defaults. So if we got an error here, a config file likely exists but
	// is malformed — use SeverityError to surface it clearly.
	severity := discovery.SeverityError
	if errors.Is(err, os.ErrNotExist) {
		severity = discovery.SeverityWarning
	}

	diag, diagErr := discovery.NewDiagnosticWithCause(
		severity,
		discovery.CodeConfigLoadFailed,
		fmt.Sprintf("failed to load config, using defaults: %v", err),
		"",
		err,
	)
	if diagErr != nil {
		slog.Error("BUG: failed to create config-load diagnostic", "error", diagErr)
		return config.DefaultConfig(), nil
	}
	return config.DefaultConfig(), []discovery.Diagnostic{diag}
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
