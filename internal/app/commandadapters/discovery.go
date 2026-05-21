// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/provisionenv"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	configPathContextKey            struct{}
	discoveryRequestCacheContextKey struct{}

	// DiscoveryService implements request-scoped discovery/config memoization
	// for command execution frontends.
	//goplint:ignore -- adapter state is validated by construction; zero-value config provider is allowed in tests.
	DiscoveryService struct {
		config      config.Loader
		baseDir     *types.FilesystemPath
		commandsDir *types.FilesystemPath

		provisionedModules       discovery.ProvisionedModuleEntries
		provisionedGlobalModules discovery.ProvisionedModuleEntries
		provisionedDiagnostics   []discovery.Diagnostic
	}

	// DiscoveryRequestScope attaches CLI discovery request state for command
	// service entry points.
	DiscoveryRequestScope struct{}

	//goplint:ignore -- request cache entry holds discovery results produced by the discovery domain.
	lookupCacheEntry struct {
		result discovery.LookupResult
		err    error
	}

	discoveryRequestCache struct {
		mu sync.Mutex

		hasConfig              bool
		cfg                    *config.Config
		cfgDiags               []discovery.Diagnostic
		cfgDiagsOwnedByCommand bool

		hasCommandSet bool
		commandSet    discovery.CommandSetResult
		commandSetErr error

		hasValidatedSet bool
		validatedSet    discovery.CommandSetResult
		validatedSetErr error

		lookups map[string]lookupCacheEntry //goplint:ignore -- cache key is the CLI lookup text at the adapter boundary.
	}
)

// NewDiscoveryService creates a request-cached discovery adapter.
func NewDiscoveryService(provider config.Loader) *DiscoveryService {
	provisionedModules, moduleDiags := provisionedModuleEntriesFromEnvironment(
		provisionenv.ModuleManifestName,
		provisionenv.ModulePathName,
	)
	provisionedGlobalModules, globalDiags := provisionedModuleEntriesFromEnvironment(
		provisionenv.GlobalModuleManifestName,
		provisionenv.GlobalModulePathName,
	)
	return &DiscoveryService{
		config:                   provider,
		provisionedModules:       provisionedModules,
		provisionedGlobalModules: provisionedGlobalModules,
		provisionedDiagnostics:   append(moduleDiags, globalDiags...),
	}
}

// NewDiscoveryServiceWithDirs creates a discovery adapter with explicit
// filesystem roots. Passing an empty commandsDir disables user-dir discovery.
func NewDiscoveryServiceWithDirs(provider config.Loader, baseDir, commandsDir types.FilesystemPath) (*DiscoveryService, error) {
	service := &DiscoveryService{
		config:      provider,
		baseDir:     &baseDir,
		commandsDir: &commandsDir,
	}
	if err := service.Validate(); err != nil {
		return nil, err
	}
	return service, nil
}

// Validate returns nil when optional discovery paths are valid.
func (s *DiscoveryService) Validate() error {
	if s == nil {
		return nil
	}
	var errs []error
	if s.baseDir != nil {
		if err := s.baseDir.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("discovery base dir: %w", err))
		}
	}
	if s.commandsDir != nil && *s.commandsDir != "" {
		if err := s.commandsDir.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("discovery commands dir: %w", err))
		}
	}
	if err := s.provisionedModules.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("provisioned modules: %w", err))
	}
	if err := s.provisionedGlobalModules.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("provisioned global modules: %w", err))
	}
	for _, diag := range s.provisionedDiagnostics {
		if err := diag.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("provisioned diagnostic: %w", err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// NewDiscoveryRequestScope creates a request-scope adapter for command services.
func NewDiscoveryRequestScope() (DiscoveryRequestScope, error) {
	scope := DiscoveryRequestScope{}
	if err := scope.Validate(); err != nil {
		return DiscoveryRequestScope{}, err
	}
	return scope, nil
}

// Begin attaches the configured path and discovery cache for one service request.
func (DiscoveryRequestScope) Begin(ctx context.Context, configPath types.FilesystemPath) context.Context {
	return ContextWithConfigPath(ctx, string(configPath))
}

// Validate returns nil because DiscoveryRequestScope is stateless.
func (DiscoveryRequestScope) Validate() error {
	return nil
}

func (e lookupCacheEntry) Validate() error {
	return e.result.Validate()
}

// ContextWithConfigPath attaches the explicit config path and request cache.
//
//goplint:ignore -- config path is a CLI adapter boundary value.
func ContextWithConfigPath(ctx context.Context, configPath string) context.Context {
	ctx = ContextWithDiscoveryRequestCache(ctx)
	return context.WithValue(ctx, configPathContextKey{}, configPath)
}

// ConfigPathFromContext extracts the explicit config path from context.
//
//goplint:ignore -- config path is a CLI adapter boundary value.
func ConfigPathFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(configPathContextKey{}).(string); ok {
		return v
	}
	return ""
}

// ContextWithDiscoveryRequestCache attaches a per-request discovery cache.
func ContextWithDiscoveryRequestCache(ctx context.Context) context.Context {
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

func prependModuleListDiagnostics(result discovery.ModuleListResult, cfgDiags []discovery.Diagnostic) discovery.ModuleListResult {
	result.Diagnostics = prependDiags(result.Diagnostics, cfgDiags)
	return result
}

// LoadConfig loads the request config through the same cache used by discovery
// without claiming ownership of config diagnostics.
func (s *DiscoveryService) LoadConfig(ctx context.Context) (*config.Config, []discovery.Diagnostic) {
	return s.loadConfig(ctx)
}

// LoadConfigForCommand loads the request config through the same cache used by
// discovery, and marks config diagnostics as owned by the command service for
// this request.
func (s *DiscoveryService) LoadConfigForCommand(ctx context.Context) (*config.Config, []discovery.Diagnostic) {
	cfg, cfgDiags := s.loadConfig(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		cache.cfgDiagsOwnedByCommand = true
		cache.mu.Unlock()
	}
	return cfg, cfgDiags
}

// DiscoverCommandSet discovers commands and prepends configuration diagnostics.
func (s *DiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	cfgDiags = unconsumedConfigDiagnostics(ctx, cfgDiags)
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

	result, err := s.newDiscovery(cfg).DiscoverCommandSet(ctx)
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
func (s *DiscoveryService) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	cfgDiags = unconsumedConfigDiagnostics(ctx, cfgDiags)
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

	result, err := s.newDiscovery(cfg).DiscoverAndValidateCommandSet(ctx)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		cache.hasValidatedSet = true
		cache.validatedSet = result
		cache.validatedSetErr = err
		if !cache.hasCommandSet && result.Set != nil {
			cache.hasCommandSet = true
			cache.commandSet = result
			cache.commandSetErr = nil
		}
		cache.mu.Unlock()
	}

	return prependCommandSetDiagnostics(result, cfgDiags), err
}

// DiscoverModules discovers modules and prepends configuration diagnostics.
func (s *DiscoveryService) DiscoverModules(ctx context.Context) (discovery.ModuleListResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	cfgDiags = unconsumedConfigDiagnostics(ctx, cfgDiags)
	result, err := s.newDiscovery(cfg).DiscoverModules()
	return prependModuleListDiagnostics(result, cfgDiags), err
}

// GetCommand looks up a command by name and prepends configuration diagnostics.
//
//goplint:ignore -- command lookup name is received at the adapter boundary.
func (s *DiscoveryService) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
	cfgDiags = unconsumedConfigDiagnostics(ctx, cfgDiags)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		if entry, ok := cache.lookups[name]; ok {
			cache.mu.Unlock()
			return prependLookupDiagnostics(entry.result, cfgDiags), entry.err
		}
		if cache.hasCommandSet && cache.commandSetErr == nil && cache.commandSet.Set != nil {
			result, err := lookupFromCommandSet(cache.commandSet, invowkfile.CommandName(name)) //goplint:ignore -- lookup input validated by helper
			cache.lookups[name] = lookupCacheEntry{result: result, err: err}
			cache.mu.Unlock()
			return prependLookupDiagnostics(result, cfgDiags), err
		}
		cache.mu.Unlock()
	}

	result, err := s.newDiscovery(cfg).GetCommand(ctx, name)
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		cache.lookups[name] = lookupCacheEntry{result: result, err: err}
		cache.mu.Unlock()
	}

	return prependLookupDiagnostics(result, cfgDiags), err
}

func (s *DiscoveryService) loadConfig(ctx context.Context) (*config.Config, []discovery.Diagnostic) {
	configPath := ConfigPathFromContext(ctx)
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

	cfg, commandDiags := commandsvc.LoadConfigWithFallback(ctx, s.config, configPath)
	diags := discoveryDiagnosticsFromCommand(commandDiags)
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

func unconsumedConfigDiagnostics(ctx context.Context, diags []discovery.Diagnostic) []discovery.Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	if cache := discoveryCacheFromContext(ctx); cache != nil {
		cache.mu.Lock()
		ownedByCommand := cache.cfgDiagsOwnedByCommand
		cache.mu.Unlock()
		if ownedByCommand {
			return nil
		}
	}
	return diags
}

func (s *DiscoveryService) newDiscovery(cfg *config.Config) *discovery.Discovery {
	var opts []discovery.Option
	if s.baseDir != nil {
		opts = append(opts, discovery.WithBaseDir(*s.baseDir))
	}
	if s.commandsDir != nil {
		opts = append(opts, discovery.WithCommandsDir(*s.commandsDir))
	}
	if len(s.provisionedModules) > 0 {
		opts = append(opts, discovery.WithProvisionedModuleEntries(s.provisionedModules))
	}
	if len(s.provisionedGlobalModules) > 0 {
		opts = append(opts, discovery.WithProvisionedGlobalModuleEntries(s.provisionedGlobalModules))
	}
	if len(s.provisionedDiagnostics) > 0 {
		opts = append(opts, discovery.WithInitialDiagnostics(s.provisionedDiagnostics))
	}
	return discovery.New(cfg, opts...)
}

func provisionedModuleEntriesFromEnvironment(manifestEnv, pathEnv provisionenv.Name) (discovery.ProvisionedModuleEntries, []discovery.Diagnostic) {
	entries, err := provisionenv.ParseEnvironment(readProvisionedEnv(manifestEnv), readProvisionedEnv(pathEnv))
	if err != nil {
		return nil, []discovery.Diagnostic{provisionedManifestDiagnostic(manifestEnv, err)}
	}
	return provisionedEntriesToDiscovery(entries), nil
}

func readProvisionedEnv(name provisionenv.Name) provisionenv.Value {
	envValue := provisionenv.Value(os.Getenv(name.String()))
	if err := envValue.Validate(); err != nil {
		return ""
	}
	return envValue
}

func provisionedEntriesToDiscovery(entries provisionenv.Entries) discovery.ProvisionedModuleEntries {
	result := make(discovery.ProvisionedModuleEntries, 0, len(entries))
	for _, entry := range entries {
		result = append(result, discovery.ProvisionedModuleEntry{
			Path:             types.FilesystemPath(entry.Path.String()), //goplint:ignore -- validated provisioned container manifest path.
			CommandNamespace: entry.CommandNamespace,
		})
	}
	return result
}

func provisionedManifestDiagnostic(name provisionenv.Name, cause error) discovery.Diagnostic {
	diag, err := discovery.NewDiagnosticWithCause(
		discovery.SeverityWarning,
		discovery.CodeProvisionedModuleManifestInvalid,
		fmt.Sprintf("invalid provisioned module manifest in %s: %v", name, cause),
		"",
		cause,
	)
	if err != nil {
		panic(err)
	}
	return diag
}

func discoveryDiagnosticsFromCommand(diags []commandsvc.Diagnostic) []discovery.Diagnostic {
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
			continue
		}
		result = append(result, converted)
	}
	return result
}

func lookupFromCommandSet(commandSetResult discovery.CommandSetResult, cmdName invowkfile.CommandName) (discovery.LookupResult, error) {
	if err := cmdName.Validate(); err != nil {
		return discovery.LookupResult{}, fmt.Errorf("invalid command name: %w", err)
	}

	diagnostics := slices.Clone(commandSetResult.Diagnostics)
	if commandSetResult.Set == nil {
		return discovery.LookupResult{Diagnostics: diagnostics}, nil
	}
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
	return discovery.LookupResult{
		Diagnostics: diagnostics,
	}, nil
}
