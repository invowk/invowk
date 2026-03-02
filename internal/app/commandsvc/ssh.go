// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/invowk/invowk/internal/sshserver"
)

// sshServerController provides goroutine-safe SSH server lifecycle management
// scoped to a single Service instance. It lazily starts the SSH server
// on first demand and stops it when the owning command execution completes.
type sshServerController struct {
	mu       sync.Mutex
	instance *sshserver.Server
}

// ensure lazily starts the SSH server if not already running. It blocks until
// the server is ready to accept connections. The server is reused across
// multiple calls within the same command execution. The started server is
// stored internally and accessed via current().
func (s *sshServerController) ensure(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil && s.instance.IsRunning() {
		return nil
	}

	// Start blocks until SSH server is ready to accept connections.
	srv, err := sshserver.New(sshserver.DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SSH server: %w", err)
	}

	s.instance = srv
	return nil
}

// stop shuts down the SSH server if running. This is a best-effort operation
// called via defer after command execution completes.
func (s *sshServerController) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil {
		// Best-effort shutdown: execution already completed at this point.
		if err := s.instance.Stop(); err != nil {
			slog.Debug("SSH server stop failed during cleanup", "error", err)
		}
		s.instance = nil
	}
}

// current returns the active SSH server instance, or nil if not started.
// Used to pass the server reference to the container runtime for host access.
func (s *sshServerController) current() *sshserver.Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.instance
}
