// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/sshserver"
)

type (
	// HostAccess provides goroutine-safe SSH server lifecycle management scoped to
	// command execution. It lazily starts the SSH server on first demand and stops
	// it when the owning command execution completes.
	//
	//goplint:ignore -- infrastructure adapter has no domain invariants; zero value is valid.
	HostAccess struct {
		mu       sync.Mutex
		instance *sshserver.Server
	}

	sshHostCallbackServer struct {
		server *sshserver.Server
	}
)

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
func (h *HostAccess) SSHServer() runtime.HostCallbackServer {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.instance == nil {
		return nil
	}
	return sshHostCallbackServer{server: h.instance}
}

func (s sshHostCallbackServer) IsRunning() bool {
	return s.server != nil && s.server.IsRunning()
}

func (s sshHostCallbackServer) GetConnectionInfo(commandID runtime.HostCallbackCommandID) (*runtime.HostCallbackConnectionInfo, error) {
	info, err := s.server.GetConnectionInfo(commandID.String())
	if err != nil {
		return nil, err
	}
	host := runtime.HostCallbackHost(info.Host.String())
	if err := host.Validate(); err != nil {
		return nil, err
	}
	token := runtime.HostCallbackToken(info.Token.String())
	if err := token.Validate(); err != nil {
		return nil, err
	}
	port := info.Port
	if err := port.Validate(); err != nil {
		return nil, err
	}
	user := runtime.HostCallbackUser(info.User)
	if err := user.Validate(); err != nil {
		return nil, err
	}
	connInfo := &runtime.HostCallbackConnectionInfo{
		Host:  host,
		Port:  port,
		Token: token,
		User:  user,
	}
	if err := connInfo.Validate(); err != nil {
		return nil, err
	}
	return connInfo, nil
}

func (s sshHostCallbackServer) RevokeToken(token runtime.HostCallbackToken) {
	sshToken := sshserver.TokenValue(token.String())
	if err := sshToken.Validate(); err != nil {
		return
	}
	s.server.RevokeToken(sshToken)
}
