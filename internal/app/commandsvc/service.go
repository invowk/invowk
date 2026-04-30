// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// Service is the command execution orchestration service. It manages the full
	// execution pipeline: config loading, command discovery, input validation,
	// runtime resolution, host-access lifecycle, execution context construction,
	// and dispatch. It returns raw typed errors (not styled ServiceErrors).
	Service struct {
		config            config.Provider
		discovery         CommandDiscovery
		hostAccess        HostAccess
		registryFactory   RuntimeRegistryFactory
		interactive       InteractiveExecutor
		observer          ExecutionObserver
		capabilityChecker deps.CapabilityChecker
		hostProbe         deps.HostProbe
		userEnvFunc       UserEnvFunc
		configFallback    ConfigFallbackFunc
	}

	// ConfigFallbackFunc loads configuration with fallback to defaults on failure.
	// The CLI layer provides the implementation that emits diagnostics.
	ConfigFallbackFunc func(ctx context.Context, provider config.Provider, configPath string) (*config.Config, []discovery.Diagnostic)
)

// New creates a command execution service.
//
// The userEnvFunc callback captures the host environment when Request.UserEnv
// is nil. The configFallback function loads configuration with fallback behavior.
// Both are provided by the CLI layer to avoid the service importing cmd/.
func New(
	configProvider config.Provider,
	disc CommandDiscovery,
	userEnvFunc UserEnvFunc,
	configFallback ConfigFallbackFunc,
) *Service {
	return NewWithPorts(configProvider, disc, userEnvFunc, configFallback, nil, nil, nil, nil, nil)
}

// NewWithPorts creates a command execution service with explicit infrastructure
// adapters. Nil ports fall back to no-op/default adapters for tests.
func NewWithPorts(
	configProvider config.Provider,
	disc CommandDiscovery,
	userEnvFunc UserEnvFunc,
	configFallback ConfigFallbackFunc,
	hostAccess HostAccess,
	registryFactory RuntimeRegistryFactory,
	interactive InteractiveExecutor,
	observer ExecutionObserver,
	hostProbe deps.HostProbe,
) *Service {
	svc := &Service{
		config:          configProvider,
		discovery:       disc,
		hostAccess:      noopHostAccess{},
		registryFactory: defaultRuntimeRegistryFactory{},
		interactive:     defaultInteractiveExecutor{},
		observer:        noopExecutionObserver{},
		userEnvFunc:     userEnvFunc,
		configFallback:  configFallback,
	}
	if hostAccess != nil {
		svc.hostAccess = hostAccess
	}
	if registryFactory != nil {
		svc.registryFactory = registryFactory
	}
	if interactive != nil {
		svc.interactive = interactive
	}
	if observer != nil {
		svc.observer = observer
	}
	if hostProbe != nil {
		svc.hostProbe = hostProbe
	}
	return svc
}

// Execute executes an invowk command through the full orchestration pipeline:
//  1. Validates the request struct fields.
//  2. Loads config and discovers the target command by name.
//  3. Validates inputs: flags, arguments, platform compatibility, and runtime compatibility.
//  4. Manages host-access lifecycle when the container runtime needs host callbacks.
//  5. Builds execution context with env var projection (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn).
//  6. Propagates incoming context for timeout and cancellation signals.
//  7. Dry-run intercept: if DryRun is set, returns structured data for rendering.
//  8. Dispatches execution (timeout → dep validation → runtime).
func (s *Service) Execute(ctx context.Context, req Request) (Result, []discovery.Diagnostic, error) {
	// Validate typed fields before any downstream work to catch programmatic misuse early.
	if err := req.Validate(); err != nil {
		return Result{}, nil, err
	}

	// Capture the host environment early, before any downstream code could
	// potentially modify it via os.Setenv. Tests can pre-populate req.UserEnv
	// to inject a controlled environment.
	if req.UserEnv == nil && s.userEnvFunc != nil {
		req.UserEnv = s.userEnvFunc()
	}
	if req.Platform == "" {
		req.Platform = invowkfile.CurrentPlatform()
	}

	cfg, cmdInfo, req, diags, err := s.discoverCommand(ctx, req)
	if err != nil {
		return Result{}, diags, err
	}

	defs := s.resolveDefinitions(req, cmdInfo)

	if validErr := s.validateInputs(req, cmdInfo, defs); validErr != nil {
		return Result{}, diags, validErr
	}

	resolved, err := s.resolveRuntime(req, cmdInfo, cfg)
	if err != nil {
		return Result{}, diags, err
	}

	execCtx, err := s.buildExecContext(ctx, req, cmdInfo, defs, resolved)
	if err != nil {
		return Result{}, diags, err
	}

	// Dry-run mode returns structured data for the CLI adapter to render.
	// It is intentionally before dependency validation and host/runtime setup
	// so planning a command does not start infrastructure or touch containers.
	if req.DryRun {
		return Result{
			ExitCode: 0,
			DryRunData: &DryRunData{
				SourceID:  cmdInfo.SourceID,
				Selection: resolved,
				ExecCtx:   execCtx,
			},
		}, diags, nil
	}

	// Track whether we are the caller that starts host access so that only this
	// Execute() invocation owns cleanup. If the adapter is already running when
	// we enter, we skip the defer to avoid premature shutdown.
	hostAccessWasRunning := s.hostAccess.Running()
	if sshErr := s.ensureSSHIfNeeded(ctx, resolved); sshErr != nil {
		return Result{}, diags, sshErr
	}
	if !hostAccessWasRunning && s.hostAccess.Running() {
		defer s.hostAccess.Stop()
	}

	return s.dispatchExecution(req, execCtx, cmdInfo, cfg, diags)
}

// ResolveFromSource resolves a source-filtered command request without executing it.
func (s *Service) ResolveFromSource(ctx context.Context, req Request) (*discovery.CommandInfo, Request, []discovery.Diagnostic, error) {
	_, cmdInfo, resolvedReq, diags, err := s.discoverCommandFromSource(ctx, nil, req)
	return cmdInfo, resolvedReq, diags, err
}

// ResolveCommand resolves a command request without executing it.
func (s *Service) ResolveCommand(ctx context.Context, req Request) (*discovery.CommandInfo, Request, []discovery.Diagnostic, error) {
	if err := req.Validate(); err != nil {
		return nil, req, nil, err
	}
	if req.Platform == "" {
		req.Platform = invowkfile.CurrentPlatform()
	}
	_, cmdInfo, resolvedReq, diags, err := s.discoverCommand(ctx, req)
	return cmdInfo, resolvedReq, diags, err
}

// discoverCommand loads configuration and discovers the target command by name.
// It returns the config, discovered command info, accumulated diagnostics, and
// any error. It returns a ClassifiedError when the command is not found.
//
// Discovery is routed through CommandDiscovery so the per-request cache (attached
// to the context by the RunE handler) is shared across validateCommandTree,
// checkAmbiguousCommand, and this method — avoiding duplicate filesystem scans.
// Config is loaded separately because downstream callers need it for runtime
// registry construction and env builder configuration.
func (s *Service) discoverCommand(ctx context.Context, req Request) (*config.Config, *discovery.CommandInfo, Request, []discovery.Diagnostic, error) {
	cfg, configDiags := s.loadConfig(ctx, string(req.ConfigPath))
	req = applyUIConfigDefaults(req, cfg)
	if req.ResolvedCommand != nil {
		return cfg, req.ResolvedCommand, req, configDiags, nil
	}

	if req.FromSource != "" {
		foundCfg, cmdInfo, resolvedReq, diags, err := s.discoverCommandFromSource(ctx, cfg, req)
		diags = appendDiagnostics(configDiags, diags...)
		return foundCfg, cmdInfo, resolvedReq, diags, err
	}

	result, err := s.discovery.DiscoverCommandSet(ctx)
	diags := appendDiagnostics(configDiags, result.Diagnostics...)
	if err != nil {
		return nil, nil, req, diags, err
	}

	if result.Set == nil {
		return s.discoverCommandByLookup(ctx, cfg, req, diags)
	}
	cmdInfo, resolvedReq, ambiguousName := resolveCommandFromSet(result.Set, req)
	if ambiguousName != "" {
		return nil, nil, req, diags, &ClassifiedError{
			Err: &AmbiguousCommandError{
				CommandName: ambiguousName,
				Sources:     ambiguousSourcesFor(result.Set, ambiguousName),
			},
			Kind: ErrorKindCommandAmbiguous,
		}
	}
	if cmdInfo == nil {
		return nil, nil, req, diags, &ClassifiedError{
			Err:  fmt.Errorf("command '%s' not found", req.Name),
			Kind: ErrorKindCommandNotFound,
		}
	}

	return cfg, cmdInfo, resolvedReq, diags, nil
}

func applyUIConfigDefaults(req Request, cfg *config.Config) Request {
	if cfg == nil {
		return req
	}
	if !req.VerboseSet {
		req.Verbose = cfg.UI.Verbose
	}
	if !req.InteractiveSet {
		req.Interactive = cfg.UI.Interactive
	}
	return req
}

func (s *Service) discoverCommandByLookup(ctx context.Context, cfg *config.Config, req Request, diags []discovery.Diagnostic) (*config.Config, *discovery.CommandInfo, Request, []discovery.Diagnostic, error) {
	lookup, err := s.discovery.GetCommand(ctx, req.Name)
	diags = append(diags, lookup.Diagnostics...)
	if err != nil {
		return nil, nil, req, diags, err
	}
	if lookup.Command == nil {
		return nil, nil, req, diags, &ClassifiedError{
			Err:  fmt.Errorf("command '%s' not found", req.Name),
			Kind: ErrorKindCommandNotFound,
		}
	}
	return cfg, lookup.Command, req, diags, nil
}

func appendDiagnostics(base []discovery.Diagnostic, extra ...discovery.Diagnostic) []discovery.Diagnostic {
	result := slices.Clone(base)
	return append(result, extra...)
}

func ambiguousSourcesFor(commandSet *discovery.DiscoveredCommandSet, name invowkfile.CommandName) []discovery.SourceID {
	var sources []discovery.SourceID
	if commandSet == nil {
		return sources
	}
	for _, cmd := range commandSet.BySimpleName[name] {
		if !slices.Contains(sources, cmd.SourceID) {
			sources = append(sources, cmd.SourceID)
		}
	}
	return sources
}

func resolveCommandFromSet(commandSet *discovery.DiscoveredCommandSet, req Request) (*discovery.CommandInfo, Request, invowkfile.CommandName) {
	if commandSet == nil {
		return nil, req, ""
	}

	tokens := strings.Fields(req.Name)
	tokens = append(tokens, req.Args...)
	for i := len(tokens); i > 0; i-- {
		candidate := invowkfile.CommandName(strings.Join(tokens[:i], " ")) //goplint:ignore -- request tokens validated by Request.Validate()
		if commandSet.AmbiguousNames[candidate] {
			return nil, req, candidate
		}
		if target := commandSet.ByName[candidate]; target != nil {
			req.Name = string(target.Name)
			req.Args = slices.Clone(tokens[i:])
			req.ResolvedCommand = target
			return target, req, ""
		}
		if simpleMatches := commandSet.BySimpleName[candidate]; len(simpleMatches) == 1 {
			target := simpleMatches[0]
			req.Name = string(target.Name)
			req.Args = slices.Clone(tokens[i:])
			req.ResolvedCommand = target
			return target, req, ""
		}
	}

	return nil, req, ""
}

func (s *Service) discoverCommandFromSource(ctx context.Context, cfg *config.Config, req Request) (*config.Config, *discovery.CommandInfo, Request, []discovery.Diagnostic, error) {
	result, err := s.discovery.DiscoverCommandSet(ctx)
	if err != nil {
		return nil, nil, req, nil, err
	}
	diags := slices.Clone(result.Diagnostics)
	var availableSources []discovery.SourceID
	if result.Set != nil {
		availableSources = result.Set.SourceOrder
	}
	if result.Set == nil || !slices.Contains(availableSources, req.FromSource) {
		return nil, nil, req, diags, &ClassifiedError{
			Err: &SourceNotFoundError{
				Source:           req.FromSource,
				AvailableSources: slices.Clone(availableSources),
			},
			Kind: ErrorKindCommandNotFound,
		}
	}

	tokens := strings.Fields(req.Name)
	tokens = append(tokens, req.Args...)
	var target *discovery.CommandInfo
	matchLen := 0
	for i := len(tokens); i > 0; i-- {
		candidate := strings.Join(tokens[:i], " ")
		for _, cmd := range result.Set.BySource[req.FromSource] {
			if string(cmd.SimpleName) == candidate || string(cmd.Name) == candidate {
				target = cmd
				matchLen = i
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		return nil, nil, req, diags, &ClassifiedError{
			Err:  fmt.Errorf("command '%s' not found in source '%s'", req.Name, req.FromSource),
			Kind: ErrorKindCommandNotFound,
		}
	}

	req.Name = string(target.Name)
	req.Args = slices.Clone(tokens[matchLen:])
	req.ResolvedCommand = target
	return cfg, target, req, diags, nil
}

// resolveDefinitions resolves flag/arg definitions and flag values by applying
// fallbacks from the command's invowkfile definitions when the request does not
// supply them. This supports both the Cobra-parsed path (defs provided) and the
// direct-call path (only command name + args).
func (s *Service) resolveDefinitions(req Request, cmdInfo *discovery.CommandInfo) resolvedDefinitions {
	flagDefs := req.FlagDefs
	// Fallback path for requests that only supply command name + args.
	if flagDefs == nil {
		flagDefs = cmdInfo.Command.Flags
	}
	argDefs := req.ArgDefs
	if argDefs == nil {
		argDefs = cmdInfo.Command.Args
	}

	flagValues := req.FlagValues
	// Apply command defaults when the caller did not provide parsed flag values.
	if flagValues == nil && len(flagDefs) > 0 {
		flagValues = make(map[invowkfile.FlagName]string)
		for _, flag := range flagDefs {
			if flag.DefaultValue != "" {
				flagValues[flag.Name] = flag.DefaultValue
			}
		}
	}

	return resolvedDefinitions{
		flagDefs:   flagDefs,
		argDefs:    argDefs,
		flagValues: flagValues,
	}
}

// loadConfig loads configuration via the configFallback callback. On failure it
// returns defaults with diagnostics so callers stay operational.
func (s *Service) loadConfig(ctx context.Context, configPath string) (*config.Config, []discovery.Diagnostic) {
	return s.configFallback(ctx, s.config, configPath)
}
