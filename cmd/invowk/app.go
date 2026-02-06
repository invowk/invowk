// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invkfile"
)

type (
	configPathContextKey struct{}

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
		// SourceFilter is the source filter string (deprecated, use FromSource).
		SourceFilter string
		// Runtime is the --runtime override (e.g., "container", "virtual").
		Runtime string
		// Interactive enables alternate screen buffer with TUI server.
		Interactive bool
		// Verbose enables verbose diagnostic output.
		Verbose bool
		// FromSource is the --from flag value for source disambiguation.
		FromSource string
		// ForceRebuild forces container image rebuilds, bypassing cache.
		ForceRebuild bool
		// Workdir overrides the working directory for the command.
		Workdir string
		// EnvFiles are dotenv file paths from --env-file flags.
		EnvFiles []string
		// EnvVars are KEY=VALUE pairs from --env-var flags (highest env priority).
		EnvVars map[string]string
		// ConfigPath is the explicit --config flag value.
		ConfigPath string
		// FlagValues are parsed flag values from Cobra state (key: flag name).
		FlagValues map[string]string
		// FlagDefs are the command's flag definitions from the invkfile.
		FlagDefs []invkfile.Flag
		// ArgDefs are the command's argument definitions from the invkfile.
		ArgDefs []invkfile.Argument
		// EnvInheritMode overrides the runtime config env inherit mode.
		EnvInheritMode string
		// EnvInheritAllow overrides the runtime config env allowlist.
		EnvInheritAllow []string
		// EnvInheritDeny overrides the runtime config env denylist.
		EnvInheritDeny []string
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

	// ConfigProvider loads configuration using explicit options rather than global state.
	// This abstraction replaces the previous global config accessor and enables
	// testing with custom config sources.
	ConfigProvider interface {
		Load(ctx context.Context, opts config.LoadOptions) (*config.Config, error)
	}

	// appDiscoveryService implements DiscoveryService by creating a fresh
	// discovery.Discovery instance per call, prepending configuration diagnostics.
	appDiscoveryService struct {
		config ConfigProvider
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
		deps.Commands = newCommandService(deps.Config, deps.Stdout, deps.Stderr)
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

// contextWithConfigPath attaches the explicit --config value to the request context.
// Discovery and execution services use this value to load configuration from the same
// source as the originating Cobra command.
func contextWithConfigPath(ctx context.Context, configPath string) context.Context {
	return context.WithValue(ctx, configPathContextKey{}, configPath)
}

// configPathFromContext extracts the explicit config path from context.
func configPathFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(configPathContextKey{}).(string); ok {
		return v
	}

	return ""
}

// DiscoverCommandSet discovers commands and prepends configuration diagnostics.
// Each call creates a fresh discovery.Discovery instance rather than caching results.
// This is intentionally stateless: it avoids shared mutable caches and invalidation
// complexity, which is appropriate for a CLI tool where each process invocation is
// short-lived. If per-process caching is needed in the future, it should be scoped
// to the App lifetime rather than introduced as package-level state.
func (s *appDiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	result, err := discovery.New(cfg).DiscoverCommandSet(ctx)
	result.Diagnostics = append(cfgDiags, result.Diagnostics...)

	return result, err
}

// DiscoverAndValidateCommandSet discovers commands, validates the command tree,
// and prepends configuration diagnostics.
func (s *appDiscoveryService) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	result, err := discovery.New(cfg).DiscoverAndValidateCommandSet(ctx)
	result.Diagnostics = append(cfgDiags, result.Diagnostics...)

	return result, err
}

// GetCommand looks up a command by name and prepends configuration diagnostics.
func (s *appDiscoveryService) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	result, err := discovery.New(cfg).GetCommand(ctx, name)
	result.Diagnostics = append(cfgDiags, result.Diagnostics...)

	return result, err
}

// loadConfig returns configuration for discovery calls. On load failure, it keeps
// discovery operational with defaults and emits a warning diagnostic for the CLI.
func (s *appDiscoveryService) loadConfig(ctx context.Context) (*config.Config, []discovery.Diagnostic) {
	configPath := configPathFromContext(ctx)
	return loadConfigWithFallback(ctx, s.config, configPath)
}

// loadConfigWithFallback loads configuration via the provider. On failure it
// returns defaults and a warning diagnostic so callers stay operational.
func loadConfigWithFallback(ctx context.Context, provider ConfigProvider, configPath string) (*config.Config, []discovery.Diagnostic) {
	cfg, err := provider.Load(ctx, config.LoadOptions{ConfigFilePath: configPath})
	if err == nil {
		return cfg, nil
	}

	return config.DefaultConfig(), []discovery.Diagnostic{{
		Severity: discovery.SeverityWarning,
		Code:     "config_load_failed",
		Message:  fmt.Sprintf("failed to load config, using defaults: %v", err),
		Path:     configPath,
		Cause:    err,
	}}
}

// Render writes structured diagnostics to stderr.
func (r *defaultDiagnosticRenderer) Render(_ context.Context, diags []discovery.Diagnostic, stderr io.Writer) {
	for _, diag := range diags {
		prefix := "Warning"
		if diag.Severity == discovery.SeverityError {
			prefix = "Error"
		}

		if diag.Path != "" {
			_, _ = fmt.Fprintf(stderr, "%s: %s (%s)\n", prefix, diag.Message, diag.Path)
			continue
		}

		_, _ = fmt.Fprintf(stderr, "%s: %s\n", prefix, diag.Message)
	}
}
