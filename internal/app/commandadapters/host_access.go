// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/invowk/invowk/internal/sshserver"
)

// HostAccess provides goroutine-safe SSH server lifecycle management scoped to
// command execution. It lazily starts the SSH server on first demand and stops
// it when the owning command execution completes.
//
//goplint:ignore -- infrastructure adapter has no domain invariants; zero value is valid.
type HostAccess struct {
	mu       sync.Mutex
	instance *sshserver.Server
}

// NewHostAccess creates an SSH-backed host-access adapter.
func NewHostAccess() (*HostAccess, error) {
	hostAccess := &HostAccess{}
	if err := hostAccess.Validate(); err != nil {
		return nil, err
	}
	return hostAccess, nil
}

// Validate returns nil because zero-value HostAccess is a valid lazy adapter.
func (h *HostAccess) Validate() error {
	return nil
}

// Ensure lazily starts the SSH server if not already running. It blocks until
// the server is ready to accept connections.
func (h *HostAccess) Ensure(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.instance != nil && h.instance.IsRunning() {
		return nil
	}

	srv, err := sshserver.New(sshserver.DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SSH server: %w", err)
	}

	h.instance = srv
	return nil
}

// Running reports whether the SSH server is currently active.
func (h *HostAccess) Running() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.instance != nil && h.instance.IsRunning()
}

// Stop shuts down the SSH server if running. This is a best-effort operation
// called via defer after command execution completes.
func (h *HostAccess) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.instance != nil {
		if err := h.instance.Stop(); err != nil {
			slog.Debug("SSH server stop failed during cleanup", "error", err)
		}
		h.instance = nil
	}
}

// SSHServer returns the active SSH server instance, or nil if not started. The
// runtime registry adapter uses this to pass host-access coordinates to the
// container runtime without exposing SSH details to commandsvc.
func (h *HostAccess) SSHServer() *sshserver.Server {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.instance
}
