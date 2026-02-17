// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"log/slog"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/sshserver"
)

type (
	// BuildRegistryOptions configures runtime registry construction.
	BuildRegistryOptions struct {
		// Config controls runtime behavior and feature flags.
		Config *config.Config
		// SSHServer is forwarded to the container runtime for host callbacks.
		SSHServer *sshserver.Server
	}

	// InitDiagnostic reports non-fatal runtime initialization details.
	InitDiagnostic struct {
		Code    string
		Message string
		Cause   error
	}

	// RegistryBuildResult contains the built registry, cleanup hook, diagnostics,
	// and any container-runtime initialization error.
	// Registry and Cleanup are always non-nil after BuildRegistry returns.
	// Callers should defer Cleanup() after use.
	RegistryBuildResult struct {
		Registry         *Registry
		Cleanup          func()
		Diagnostics      []InitDiagnostic
		ContainerInitErr error
	}
)

// BuildRegistry creates and populates the runtime registry.
// Native and virtual runtimes are always registered. Container runtime
// registration is best-effort and reported via Diagnostics/ContainerInitErr.
func BuildRegistry(opts BuildRegistryOptions) RegistryBuildResult {
	cfg := opts.Config
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	result := RegistryBuildResult{
		Registry: NewRegistry(),
		Cleanup:  func() {},
	}

	result.Registry.Register(RuntimeTypeNative, NewNativeRuntime())
	result.Registry.Register(RuntimeTypeVirtual, NewVirtualRuntime(cfg.VirtualShell.EnableUrootUtils))

	containerRT, err := NewContainerRuntime(cfg)
	if err != nil {
		result.ContainerInitErr = err
		result.Diagnostics = append(result.Diagnostics, InitDiagnostic{
			Code:    "container_runtime_init_failed",
			Message: fmt.Sprintf("container runtime unavailable: %v", err),
			Cause:   err,
		})
	} else {
		if opts.SSHServer != nil && opts.SSHServer.IsRunning() {
			containerRT.SetSSHServer(opts.SSHServer)
		}
		result.Registry.Register(RuntimeTypeContainer, containerRT)
		result.Cleanup = func() {
			if closeErr := containerRT.Close(); closeErr != nil {
				slog.Warn("container runtime cleanup failed", "error", closeErr)
			}
		}
	}

	return result
}
