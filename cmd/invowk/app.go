// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	configPathContextKey            struct{}
	discoveryRequestCacheContextKey struct{}

	// App wires CLI services and shared dependencies. It is the composition root for
	// the CLI layer — all Cobra command handlers receive an App reference and delegate
	// business logic through its service interfaces (Commands, Discovery, Config).
	App struct {
		Config      ConfigProvider
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
		Config      ConfigProvider
		Discovery   DiscoveryService
		Commands    CommandService
		Diagnostics DiagnosticRenderer
		Stdout      io.Writer
		Stderr      io.Writer
	}

	// ExecuteRequest captures all CLI execution inputs as an immutable value.
	// It is the request-scoped data contract between the CLI layer (Cobra handlers)
	// and the CommandService implementation.
	ExecuteRequest struct {
		// Name is the fully-qualified command name (e.g., "io.invowk.sample build").
		Name string
		// Args are positional arguments to pass to the command script ($1, $2, etc.).
		Args []string
		// Runtime is the --ivk-runtime override (e.g., RuntimeContainer, RuntimeVirtual).
		// Zero value ("") means no override.
		Runtime invowkfile.RuntimeMode
		// Interactive enables alternate screen buffer with TUI server.
		Interactive bool
		// Verbose enables verbose diagnostic output.
		Verbose bool
		// FromSource is the --ivk-from flag value for source disambiguation.
		FromSource string
		// ForceRebuild forces container image rebuilds, bypassing cache.
		ForceRebuild bool
		// Workdir overrides the working directory for the command.
		Workdir string
		// EnvFiles are dotenv file paths from --ivk-env-file flags.
		EnvFiles []string
		// EnvVars are KEY=VALUE pairs from --ivk-env-var flags (highest env priority).
		EnvVars map[string]string
		// ConfigPath is the explicit --ivk-config flag value.
		ConfigPath string
		// FlagValues are parsed flag values from Cobra state (key: flag name).
		FlagValues map[string]string
		// FlagDefs are the command's flag definitions from the invowkfile.
		FlagDefs []invowkfile.Flag
		// ArgDefs are the command's argument definitions from the invowkfile.
		ArgDefs []invowkfile.Argument
		// EnvInheritMode overrides the runtime config env inherit mode.
		// Zero value ("") means no override.
		EnvInheritMode invowkfile.EnvInheritMode
		// EnvInheritAllow overrides the runtime config env allowlist.
		EnvInheritAllow []string
		// EnvInheritDeny overrides the runtime config env denylist.
		EnvInheritDeny []string
		// DryRun enables dry-run mode: prints what would be executed without executing.
		DryRun bool
	}

	// ExecuteResult contains command execution outcomes.
	ExecuteResult struct {
		ExitCode int
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

	// ConfigProvider loads configuration using explicit options.
	// This abstraction enables testing with custom config sources or mock implementations.
	ConfigProvider interface {
		Load(ctx context.Context, opts config.LoadOptions) (*config.Config, error)
	}

	// appDiscoveryService implements DiscoveryService with per-request memoization.
	// On cache miss, it creates a discovery.Discovery instance, runs the operation,
	// and caches the result. Configuration diagnostics are prepended on every path
	// since the config may vary by context path.
	appDiscoveryService struct {
		config ConfigProvider
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

		hasCommandSet bool
		commandSet    discovery.CommandSetResult
		commandSetErr error

		hasValidatedSet bool
		validatedSet    discovery.CommandSetResult
		validatedSetErr error

		lookups map[string]lookupCacheEntry
	}

	defaultDiagnosticRenderer struct{}
)

// NewApp creates an App with defaults for omitted dependencies.
func NewApp(deps Dependencies) (*App, error) {
	if deps.Stdout == nil {
		deps.Stdout = os.Stdout
	}
	if deps.Stderr == nil {
		deps.Stderr = os.Stderr
	}
	if deps.Config == nil {
		deps.Config = config.NewProvider()
	}
	if deps.Discovery == nil {
		deps.Discovery = &appDiscoveryService{config: deps.Config}
	}
	if deps.Diagnostics == nil {
		deps.Diagnostics = &defaultDiagnosticRenderer{}
	}
	if deps.Commands == nil {
		deps.Commands = newCommandService(deps.Config, deps.Discovery, deps.Stdout, deps.Stderr)
	}

	return &App{
		Config:      deps.Config,
		Discovery:   deps.Discovery,
		Commands:    deps.Commands,
		Diagnostics: deps.Diagnostics,
		stdout:      deps.Stdout,
		stderr:      deps.Stderr,
	}, nil
}

// contextWithConfigPath attaches the explicit --ivk-config value and a per-request
// discovery cache to the context. The RunE handler calls this once; all downstream
// callees (validateCommandTree, checkAmbiguousCommand, listCommands, executeRequest,
// and commandService.discoverCommand via DiscoveryService) share the same cache.
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

func prependCommandSetDiagnostics(result discovery.CommandSetResult, cfgDiags []discovery.Diagnostic) discovery.CommandSetResult {
	if len(cfgDiags) == 0 {
		return result
	}

	out := result
	out.Diagnostics = append(append(make([]discovery.Diagnostic, 0, len(cfgDiags)+len(result.Diagnostics)), cfgDiags...), result.Diagnostics...)
	return out
}

func prependLookupDiagnostics(result discovery.LookupResult, cfgDiags []discovery.Diagnostic) discovery.LookupResult {
	if len(cfgDiags) == 0 {
		return result
	}

	out := result
	out.Diagnostics = append(append(make([]discovery.Diagnostic, 0, len(cfgDiags)+len(result.Diagnostics)), cfgDiags...), result.Diagnostics...)
	return out
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
	return loadConfigWithFallback(ctx, s.config, configPath)
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
func loadConfigWithFallback(ctx context.Context, provider ConfigProvider, configPath string) (*config.Config, []discovery.Diagnostic) {
	cfg, err := provider.Load(ctx, config.LoadOptions{ConfigFilePath: configPath})
	if err == nil {
		return cfg, nil
	}

	// When the user explicitly specified a config path, do not silently fall back
	// to defaults — surface the error as a diagnostic so downstream callers can
	// decide whether to abort.
	if configPath != "" {
		return config.DefaultConfig(), []discovery.Diagnostic{{
			Severity: discovery.SeverityError,
			Code:     discovery.CodeConfigLoadFailed,
			Message:  fmt.Sprintf("failed to load config from %s: %v", configPath, err),
			Path:     configPath,
			Cause:    err,
		}}
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

	return config.DefaultConfig(), []discovery.Diagnostic{{
		Severity: severity,
		Code:     discovery.CodeConfigLoadFailed,
		Message:  fmt.Sprintf("failed to load config, using defaults: %v", err),
		Cause:    err,
	}}
}

// Render writes structured diagnostics to stderr with lipgloss styling.
func (r *defaultDiagnosticRenderer) Render(_ context.Context, diags []discovery.Diagnostic, stderr io.Writer) {
	for _, diag := range diags {
		prefix := WarningStyle.Render("warning")
		if diag.Severity == discovery.SeverityError {
			prefix = ErrorStyle.Render("error")
		}

		if diag.Path != "" {
			_, _ = fmt.Fprintf(stderr, "%s: %s (%s)\n", prefix, diag.Message, diag.Path)
			continue
		}

		_, _ = fmt.Fprintf(stderr, "%s: %s\n", prefix, diag.Message)
	}
}
