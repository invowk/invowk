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

	// App wires CLI services and shared dependencies.
	App struct {
		Config      ConfigProvider
		Discovery   DiscoveryService
		Commands    CommandService
		Diagnostics DiagnosticRenderer
		stdout      io.Writer
		stderr      io.Writer
	}

	// Dependencies defines the dependencies used to build an App.
	Dependencies struct {
		Config      ConfigProvider
		Discovery   DiscoveryService
		Commands    CommandService
		Diagnostics DiagnosticRenderer
		Stdout      io.Writer
		Stderr      io.Writer
	}

	// ExecuteRequest captures CLI execution inputs.
	ExecuteRequest struct {
		Name            string
		Args            []string
		SourceFilter    string
		Runtime         string
		Interactive     bool
		Verbose         bool
		FromSource      string
		ForceRebuild    bool
		Workdir         string
		EnvFiles        []string
		EnvVars         map[string]string
		ConfigPath      string
		FlagValues      map[string]string
		FlagDefs        []invkfile.Flag
		ArgDefs         []invkfile.Argument
		EnvInheritMode  string
		EnvInheritAllow []string
		EnvInheritDeny  []string
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
	ConfigProvider interface {
		Load(ctx context.Context, opts config.LoadOptions) (*config.Config, error)
	}

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
