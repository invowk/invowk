// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	configPathContextKey            struct{}
	discoveryRequestCacheContextKey struct{}

	// DiscoveryService implements request-scoped discovery/config memoization
	// for command execution frontends.
	//goplint:ignore -- adapter state is validated by construction; zero-value config provider is allowed in tests.
	DiscoveryService struct {
		config config.Provider
	}

	//goplint:ignore -- request cache entry holds discovery results produced by the discovery domain.
	lookupCacheEntry struct {
		result discovery.LookupResult
		err    error
	}

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

		lookups map[string]lookupCacheEntry //goplint:ignore -- cache key is the CLI lookup text at the adapter boundary.
	}
)

// NewDiscoveryService creates a request-cached discovery adapter.
func NewDiscoveryService(provider config.Provider) *DiscoveryService {
	return &DiscoveryService{config: provider}
}

// Validate returns nil because DiscoveryService has no intrinsic invariants.
func (s *DiscoveryService) Validate() error {
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

// DiscoverCommandSet discovers commands and prepends configuration diagnostics.
func (s *DiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
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
func (s *DiscoveryService) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
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
	result, err := discovery.New(cfg).DiscoverModules()
	return prependModuleListDiagnostics(result, cfgDiags), err
}

// GetCommand looks up a command by name and prepends configuration diagnostics.
//
//goplint:ignore -- command lookup name is received at the adapter boundary.
func (s *DiscoveryService) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	cfg, cfgDiags := s.loadConfig(ctx)
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

	result, err := discovery.New(cfg).GetCommand(ctx, name)
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

	cfg, diags := commandsvc.LoadConfigWithFallback(ctx, s.config, configPath)
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
