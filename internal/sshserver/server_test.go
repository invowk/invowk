// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/core/serverbase"
	"github.com/invowk/invowk/internal/testutil"
)

func TestGenerateToken(t *testing.T) {
	t.Parallel()

	srv := New(DefaultConfig())

	token, err := srv.GenerateToken("test-command")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token.Value == "" {
		t.Error("Token value should not be empty")
	}
	if token.CommandID != "test-command" {
		t.Errorf("CommandID = %q, want %q", token.CommandID, "test-command")
	}
	if token.ExpiresAt.Before(time.Now()) {
		t.Error("Token should not be expired immediately")
	}
}

func TestValidateToken(t *testing.T) {
	t.Parallel()

	srv := New(DefaultConfig())

	token, err := srv.GenerateToken("test-command")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Valid token
	validated, ok := srv.ValidateToken(token.Value)
	if !ok {
		t.Error("Token should be valid")
	}
	if validated.CommandID != "test-command" {
		t.Errorf("CommandID = %q, want %q", validated.CommandID, "test-command")
	}

	// Invalid token
	_, ok = srv.ValidateToken("invalid-token")
	if ok {
		t.Error("Invalid token should not be valid")
	}
}

func TestRevokeToken(t *testing.T) {
	t.Parallel()

	srv := New(DefaultConfig())

	token, err := srv.GenerateToken("test-command")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Token should be valid
	_, ok := srv.ValidateToken(token.Value)
	if !ok {
		t.Error("Token should be valid before revocation")
	}

	// Revoke
	srv.RevokeToken(token.Value)

	// Token should be invalid now
	_, ok = srv.ValidateToken(token.Value)
	if ok {
		t.Error("Token should be invalid after revocation")
	}
}

func TestRevokeTokensForCommand(t *testing.T) {
	t.Parallel()

	srv := New(DefaultConfig())

	// Generate multiple tokens for same command
	token1, _ := srv.GenerateToken("command-1")
	token2, _ := srv.GenerateToken("command-1")
	token3, _ := srv.GenerateToken("command-2")

	// All should be valid
	if _, ok := srv.ValidateToken(token1.Value); !ok {
		t.Error("token1 should be valid")
	}
	if _, ok := srv.ValidateToken(token2.Value); !ok {
		t.Error("token2 should be valid")
	}
	if _, ok := srv.ValidateToken(token3.Value); !ok {
		t.Error("token3 should be valid")
	}

	// Revoke tokens for command-1
	srv.RevokeTokensForCommand("command-1")

	// token1 and token2 should be invalid
	if _, ok := srv.ValidateToken(token1.Value); ok {
		t.Error("token1 should be invalid after revocation")
	}
	if _, ok := srv.ValidateToken(token2.Value); ok {
		t.Error("token2 should be invalid after revocation")
	}

	// token3 should still be valid
	if _, ok := srv.ValidateToken(token3.Value); !ok {
		t.Error("token3 should still be valid")
	}
}

func TestServerStartStop(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0 // Auto-select port

	srv := New(cfg)

	// Initial state should be Created
	if srv.State() != serverbase.StateCreated {
		t.Errorf("State should be Created, got %s", srv.State())
	}

	if srv.IsRunning() {
		t.Error("Server should not be running before Start()")
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// State should be Running
	if srv.State() != serverbase.StateRunning {
		t.Errorf("State should be Running, got %s", srv.State())
	}

	if !srv.IsRunning() {
		t.Error("Server should be running after Start()")
	}

	if srv.Port() == 0 {
		t.Error("Server port should be assigned")
	}

	if srv.Address() == "" {
		t.Error("Server address should not be empty")
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// State should be Stopped
	if srv.State() != serverbase.StateStopped {
		t.Errorf("State should be Stopped, got %s", srv.State())
	}

	if srv.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}
}

func TestServerDoubleStart(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0

	srv := New(cfg)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer testutil.MustStop(t, srv)

	// Second Start() should fail
	err := srv.Start(ctx)
	if err == nil {
		t.Error("Second Start() should return error")
	}
}

func TestServerDoubleStop(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0

	srv := New(cfg)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// First Stop() should succeed
	if err := srv.Stop(); err != nil {
		t.Fatalf("First Stop() failed: %v", err)
	}

	// Second Stop() should be no-op (not error)
	if err := srv.Stop(); err != nil {
		t.Errorf("Second Stop() should not error, got: %v", err)
	}
}

func TestGetConnectionInfo(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0

	srv := New(cfg)

	// Should fail before server starts
	_, err := srv.GetConnectionInfo("test")
	if err == nil {
		t.Error("GetConnectionInfo should fail when server is not running")
	}

	ctx := context.Background()
	if startErr := srv.Start(ctx); startErr != nil {
		t.Fatalf("Failed to start server: %v", startErr)
	}
	defer testutil.MustStop(t, srv)

	// Should succeed after server starts
	info, err := srv.GetConnectionInfo("test-command")
	if err != nil {
		t.Fatalf("GetConnectionInfo failed: %v", err)
	}

	if info.Host == "" {
		t.Error("Host should not be empty")
	}
	if info.Port == 0 {
		t.Error("Port should not be 0")
	}
	if info.Token == "" {
		t.Error("Token should not be empty")
	}
	if info.User == "" {
		t.Error("User should not be empty")
	}
}

func TestExpiredToken(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TokenTTL = 1 * time.Hour // Use a reasonable TTL; we control time via FakeClock

	// Create a FakeClock for deterministic time control
	clock := testutil.NewFakeClock(time.Now())
	srv := NewWithClock(cfg, clock)

	token, err := srv.GenerateToken("test-command")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Token should be valid immediately after creation
	_, ok := srv.ValidateToken(token.Value)
	if !ok {
		t.Error("Token should be valid immediately after creation")
	}

	// Advance time past the token TTL
	clock.Advance(cfg.TokenTTL + time.Millisecond)

	// Token should now be expired
	_, ok = srv.ValidateToken(token.Value)
	if ok {
		t.Error("Expired token should not be valid")
	}
}

func TestServerState(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0

	srv := New(cfg)

	// Test state transitions
	if srv.State() != serverbase.StateCreated {
		t.Errorf("Initial state should be Created, got %s", srv.State())
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	if srv.State() != serverbase.StateRunning {
		t.Errorf("State after Start should be Running, got %s", srv.State())
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	if srv.State() != serverbase.StateStopped {
		t.Errorf("State after Stop should be Stopped, got %s", srv.State())
	}
}

func TestServerStartWithCancelledContext(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0

	srv := New(cfg)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := srv.Start(ctx)
	if err == nil {
		t.Error("Start with cancelled context should return error")
		testutil.MustStop(t, srv)
	}

	// State should be Failed
	if srv.State() != serverbase.StateFailed {
		t.Errorf("State should be Failed, got %s", srv.State())
	}
}

func TestStopWithoutStart(t *testing.T) {
	t.Parallel()

	srv := New(DefaultConfig())

	// Stop without Start should be safe
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop without Start should not error, got: %v", err)
	}

	// State should be Stopped
	if srv.State() != serverbase.StateStopped {
		t.Errorf("State should be Stopped, got %s", srv.State())
	}
}

func TestServerStateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state    serverbase.State
		expected string
	}{
		{serverbase.StateCreated, "created"},
		{serverbase.StateStarting, "starting"},
		{serverbase.StateRunning, "running"},
		{serverbase.StateStopping, "stopping"},
		{serverbase.StateStopped, "stopped"},
		{serverbase.StateFailed, "failed"},
		{serverbase.State(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("serverbase.State(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestIsClosedConnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("something"), false},
		{"closed conn OpError", &net.OpError{Op: "read", Err: errors.New("use of closed network connection")}, true},
		{"different OpError", &net.OpError{Op: "read", Err: errors.New("different error")}, false},
		{"non-OpError type", errors.New("use of closed network connection"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isClosedConnError(tt.err); got != tt.want {
				t.Errorf("isClosedConnError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerStartWithUsedPort(t *testing.T) {
	t.Parallel()

	cfg1 := DefaultConfig()
	cfg1.Port = 0
	srv1 := New(cfg1)

	ctx := context.Background()
	if err := srv1.Start(ctx); err != nil {
		t.Fatalf("Failed to start server1: %v", err)
	}
	defer testutil.MustStop(t, srv1)

	// Create server2 targeting the same port
	cfg2 := DefaultConfig()
	cfg2.Port = srv1.Port()
	srv2 := New(cfg2)

	err := srv2.Start(ctx)
	if err == nil {
		testutil.MustStop(t, srv2)
		t.Fatal("Start with used port should return error")
	}

	if srv2.State() != serverbase.StateFailed {
		t.Errorf("State should be Failed, got %s", srv2.State())
	}
}

func TestServerAccessorsAfterStart(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0
	srv := New(cfg)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer testutil.MustStop(t, srv)

	if !strings.Contains(srv.Address(), ":") {
		t.Errorf("Address() = %q, should contain ':'", srv.Address())
	}
	if srv.Port() <= 0 {
		t.Errorf("Port() = %d, should be > 0", srv.Port())
	}
	if srv.Host() != "127.0.0.1" {
		t.Errorf("Host() = %q, want %q", srv.Host(), "127.0.0.1")
	}
}

func TestServerWaitAfterStop(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0
	srv := New(cfg)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	if err := srv.Wait(); err != nil {
		t.Errorf("Wait() after Stop should return nil, got: %v", err)
	}
}

func TestServerWaitAfterFail(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 0
	srv := New(cfg)

	// Use an already-cancelled context to force Start to fail
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := srv.Start(ctx); err == nil {
		testutil.MustStop(t, srv)
		t.Fatal("Start with cancelled context should return error")
	}

	err := srv.Wait()
	if err == nil {
		t.Error("Wait() after failed Start should return non-nil error")
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 0 {
		t.Errorf("Port = %d, want 0", cfg.Port)
	}
	if cfg.TokenTTL != time.Hour {
		t.Errorf("TokenTTL = %v, want %v", cfg.TokenTTL, time.Hour)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
	if cfg.DefaultShell != "/bin/sh" {
		t.Errorf("DefaultShell = %q, want %q", cfg.DefaultShell, "/bin/sh")
	}
	if cfg.StartupTimeout != 5*time.Second {
		t.Errorf("StartupTimeout = %v, want %v", cfg.StartupTimeout, 5*time.Second)
	}
}

// Note: Server restart (Stop then Start on the same instance) is not supported.
// Server instances are single-use: once stopped, create a new instance.
// This simplifies the implementation and avoids complex state management.
