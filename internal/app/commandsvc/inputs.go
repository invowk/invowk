// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/internal/app/deps"
	appexec "github.com/invowk/invowk/internal/app/execute"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// validateInputs validates flag values, positional arguments, and platform compatibility.
// It returns raw typed errors (ArgumentValidationError, ClassifiedError) — no ServiceError wrapping.
func (s *Service) validateInputs(req Request, cmdInfo *discovery.CommandInfo, defs resolvedDefinitions) error {
	if err := deps.ValidateFlagValues(req.Name, defs.flagValues, defs.flagDefs); err != nil {
		return err
	}

	if err := deps.ValidateArguments(req.Name, req.Args, defs.argDefs); err != nil {
		// Return the raw ArgumentValidationError; the CLI adapter wraps it with rendering.
		return err
	}

	platform := requestPlatform(req)
	if !cmdInfo.Command.CanRunOnPlatform(platform) {
		return &UnsupportedPlatformError{
			CommandName: invowkfile.CommandName(req.Name), //goplint:ignore -- service request name validated by discovery
			Current:     platform,
			Supported:   cmdInfo.Command.GetSupportedPlatforms(),
		}
	}

	return nil
}

// resolveRuntime determines the selected runtime and implementation for the current
// platform using 3-tier precedence:
//  1. CLI flag (--ivk-runtime) — hard override, errors if incompatible.
//  2. Config default runtime (cfg.DefaultRuntime) — soft, silently falls back if incompatible.
//  3. Per-command default — first runtime of the first matching implementation.
//
// It returns service-owned typed errors (RuntimeNotAllowedError, etc.) — no ServiceError wrapping.
func (s *Service) resolveRuntime(req Request, cmdInfo *discovery.CommandInfo, cfg *config.Config) (appexec.RuntimeSelection, error) {
	cmdName := invowkfile.CommandName(req.Name) //goplint:ignore -- CLI boundary, validated by discovery lookup
	selection, err := appexec.ResolveRuntime(cmdInfo.Command, cmdName, req.Runtime, cfg, requestPlatform(req))
	if err != nil {
		if notAllowed, ok := errors.AsType[*appexec.RuntimeNotAllowedError](err); ok {
			return appexec.RuntimeSelection{}, &RuntimeNotAllowedError{
				CommandName: notAllowed.CommandName,
				Runtime:     notAllowed.Runtime,
				Platform:    notAllowed.Platform,
				Allowed:     notAllowed.Allowed,
			}
		}
		return appexec.RuntimeSelection{}, fmt.Errorf("%w: resolve runtime for '%s': %w", ErrRuntimeResolution, req.Name, err)
	}

	return selection, nil
}

// ensureSSHIfNeeded conditionally starts the SSH server when the selected runtime
// implementation requires host SSH access (used by container runtime for host callbacks).
// Cleanup is handled by the caller (Execute) via a "started-by-me" guard.
func (s *Service) ensureSSHIfNeeded(ctx context.Context, resolved appexec.RuntimeSelection) error {
	if !resolved.Impl().GetHostSSHForRuntime(resolved.Mode()) {
		return nil
	}

	if err := s.hostAccess.Ensure(ctx); err != nil {
		return fmt.Errorf("failed to start SSH server for host access: %w", err)
	}
	return nil
}

// buildExecContext constructs the runtime execution context from the request,
// discovered command info, resolved definitions, and selected runtime. It projects
// flags and arguments into environment variables following the INVOWK_FLAG_*,
// INVOWK_ARG_*, ARGn, and ARGC conventions.
func (s *Service) buildExecContext(ctx context.Context, req Request, cmdInfo *discovery.CommandInfo, defs resolvedDefinitions, resolved appexec.RuntimeSelection) (*runtime.ExecutionContext, error) {
	return appexec.BuildExecutionContext(ctx, appexec.BuildExecutionContextOptions{
		Command:         cmdInfo.Command,
		Invowkfile:      cmdInfo.Invowkfile,
		Selection:       resolved,
		Args:            req.Args,
		Verbose:         req.Verbose,
		Workdir:         req.Workdir,
		ForceRebuild:    req.ForceRebuild,
		EnvFiles:        req.EnvFiles,
		EnvVars:         req.EnvVars,
		FlagValues:      defs.flagValues,
		ArgDefs:         defs.argDefs,
		EnvInheritMode:  req.EnvInheritMode,
		EnvInheritAllow: req.EnvInheritAllow,
		EnvInheritDeny:  req.EnvInheritDeny,
		SourceID:        cmdInfo.SourceID,
		Platform:        requestPlatform(req),
	})
}

func requestPlatform(req Request) invowkfile.Platform {
	if req.Platform != "" {
		return req.Platform
	}
	return invowkfile.CurrentPlatform()
}

// validateDeps validates command dependencies and returns raw typed errors
// (e.g., *deps.DependencyError). The CLI adapter inspects the error type and
// applies rendering. Discovery is routed through s.discovery so the per-request
// cache avoids redundant filesystem scans.
func (s *Service) validateDeps(cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext, registry *runtime.Registry, userEnv map[string]string) error {
	return deps.ValidateDependenciesWithPorts(s.discovery, cmdInfo, registry, execCtx, userEnv, s.capabilityChecker, s.hostProbe, s.lockProvider)
}
