// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// Service is the command execution orchestration service. It manages the full
	// execution pipeline: config loading, command discovery, input validation,
	// runtime resolution, SSH lifecycle, execution context construction, and
	// dispatch. It returns raw typed errors (not styled ServiceErrors).
	Service struct {
		config         config.Provider
		discovery      CommandDiscovery
		stdout         io.Writer
		stderr         io.Writer
		ssh            *sshServerController
		userEnvFunc    UserEnvFunc
		configFallback ConfigFallbackFunc
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
	stdout, stderr io.Writer,
	userEnvFunc UserEnvFunc,
	configFallback ConfigFallbackFunc,
) *Service {
	return &Service{
		config:         configProvider,
		discovery:      disc,
		stdout:         stdout,
		stderr:         stderr,
		ssh:            &sshServerController{},
		userEnvFunc:    userEnvFunc,
		configFallback: configFallback,
	}
}

// Execute executes an invowk command through the full orchestration pipeline:
//  1. Loads config and discovers the target command by name.
//  2. Validates inputs: flags, arguments, platform compatibility, and runtime compatibility.
//  3. Manages SSH server lifecycle when the container runtime needs host access.
//  4. Builds execution context with env var projection (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn).
//  5. Propagates incoming context for timeout and cancellation signals.
//  6. Dry-run intercept: if DryRun is set, returns structured data for rendering.
//  7. Dispatches execution (timeout → dep validation → runtime).
func (s *Service) Execute(ctx context.Context, req Request) (Result, []discovery.Diagnostic, error) {
	// Capture the host environment early, before any downstream code could
	// potentially modify it via os.Setenv. Tests can pre-populate req.UserEnv
	// to inject a controlled environment.
	if req.UserEnv == nil && s.userEnvFunc != nil {
		req.UserEnv = s.userEnvFunc()
	}

	cfg, cmdInfo, diags, err := s.discoverCommand(ctx, req)
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

	// Track whether we are the caller that starts the SSH server so that
	// only this Execute() invocation owns cleanup. If the server is already
	// running when we enter, we skip the defer to avoid premature shutdown.
	sshWasRunning := s.ssh.current() != nil
	if sshErr := s.ensureSSHIfNeeded(ctx, resolved); sshErr != nil {
		return Result{}, diags, sshErr
	}
	if !sshWasRunning && s.ssh.current() != nil {
		defer s.ssh.stop()
	}

	execCtx, err := s.buildExecContext(ctx, req, cmdInfo, defs, resolved)
	if err != nil {
		return Result{}, diags, err
	}

	// Dry-run mode: return structured data for the CLI adapter to render.
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

	return s.dispatchExecution(req, execCtx, cmdInfo, cfg, diags)
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
func (s *Service) discoverCommand(ctx context.Context, req Request) (*config.Config, *discovery.CommandInfo, []discovery.Diagnostic, error) {
	cfg, _ := s.loadConfig(ctx, string(req.ConfigPath))

	lookup, err := s.discovery.GetCommand(ctx, req.Name)
	diags := slices.Clone(lookup.Diagnostics)
	if err != nil {
		return nil, nil, diags, err
	}

	if lookup.Command == nil {
		return nil, nil, diags, &ClassifiedError{
			Err:     fmt.Errorf("command '%s' not found", req.Name),
			IssueID: issue.CommandNotFoundId,
			Message: "",
		}
	}

	return cfg, lookup.Command, diags, nil
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
