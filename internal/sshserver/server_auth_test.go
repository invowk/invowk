// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/ssh"

	"github.com/invowk/invowk/internal/testutil"
)

// stubSSHContext implements ssh.Context for testing auth handlers
// without starting a real SSH server.
type stubSSHContext struct {
	context.Context
	*sync.Mutex
	user   string
	values map[any]any
}

func newStubSSHContext(ctx context.Context) *stubSSHContext {
	return &stubSSHContext{
		Context: ctx,
		Mutex:   &sync.Mutex{},
		user:    "test-user",
		values:  make(map[any]any),
	}
}

func (s *stubSSHContext) User() string          { return s.user }
func (s *stubSSHContext) SessionID() string     { return "test-session-id" }
func (s *stubSSHContext) ClientVersion() string { return "SSH-2.0-test" }
func (s *stubSSHContext) ServerVersion() string { return "SSH-2.0-invowk-test" }

func (s *stubSSHContext) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (s *stubSSHContext) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}
}

func (s *stubSSHContext) Permissions() *ssh.Permissions { return &ssh.Permissions{} }

func (s *stubSSHContext) SetValue(key, value any) {
	s.Lock()
	defer s.Unlock()
	s.values[key] = value
}

func (s *stubSSHContext) getValue(key any) (any, bool) {
	s.Lock()
	defer s.Unlock()
	v, ok := s.values[key]
	return v, ok
}

func TestPasswordHandler(t *testing.T) {
	t.Parallel()

	t.Run("valid token returns true and sets context", func(t *testing.T) {
		t.Parallel()

		srv := mustNew(t, DefaultConfig())
		token, err := srv.GenerateToken(CommandID("cmd-123"))
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		ctx := newStubSSHContext(t.Context())
		result := srv.passwordHandler(ctx, string(token.Value))

		if !result {
			t.Error("passwordHandler() = false, want true for valid token")
		}

		// Verify context values were set
		storedToken, ok := ctx.getValue("token")
		if !ok {
			t.Fatal("context 'token' not set")
		}
		if storedToken.(*Token).CommandID != CommandID("cmd-123") {
			t.Errorf("stored token CommandID = %q, want %q", storedToken.(*Token).CommandID, "cmd-123")
		}

		storedCmdID, ok := ctx.getValue("commandID")
		if !ok {
			t.Fatal("context 'commandID' not set")
		}
		if storedCmdID != CommandID("cmd-123") {
			t.Errorf("stored commandID = %q, want %q", storedCmdID, "cmd-123")
		}
	})

	t.Run("invalid token format returns false", func(t *testing.T) {
		t.Parallel()

		srv := mustNew(t, DefaultConfig())
		ctx := newStubSSHContext(t.Context())

		// Empty string fails TokenValue.Validate()
		if srv.passwordHandler(ctx, "") {
			t.Error("passwordHandler() = true, want false for empty token")
		}

		// Whitespace-only fails TokenValue.Validate()
		if srv.passwordHandler(ctx, "   ") {
			t.Error("passwordHandler() = true, want false for whitespace token")
		}
	})

	t.Run("unknown token returns false", func(t *testing.T) {
		t.Parallel()

		srv := mustNew(t, DefaultConfig())
		ctx := newStubSSHContext(t.Context())

		// Valid format but not registered
		if srv.passwordHandler(ctx, "abcdef1234567890abcdef1234567890") {
			t.Error("passwordHandler() = true, want false for unknown token")
		}
	})

	t.Run("expired token returns false", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultConfig()
		cfg.TokenTTL = 1 * time.Hour

		clock := testutil.NewFakeClock(time.Now())
		srv, err := NewWithClock(cfg, clock)
		if err != nil {
			t.Fatalf("NewWithClock() error = %v", err)
		}

		token, err := srv.GenerateToken(CommandID("cmd-456"))
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		// Advance past TTL
		clock.Advance(cfg.TokenTTL + time.Second)

		ctx := newStubSSHContext(t.Context())
		if srv.passwordHandler(ctx, string(token.Value)) {
			t.Error("passwordHandler() = true, want false for expired token")
		}
	})

	t.Run("revoked token returns false", func(t *testing.T) {
		t.Parallel()

		srv := mustNew(t, DefaultConfig())
		token, err := srv.GenerateToken(CommandID("cmd-789"))
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		srv.RevokeToken(token.Value)

		ctx := newStubSSHContext(t.Context())
		if srv.passwordHandler(ctx, string(token.Value)) {
			t.Error("passwordHandler() = true, want false for revoked token")
		}
	})
}

func TestPublicKeyHandler(t *testing.T) {
	t.Parallel()

	srv := mustNew(t, DefaultConfig())
	ctx := newStubSSHContext(t.Context())

	// publicKeyHandler always rejects — only token-based auth is supported
	if srv.publicKeyHandler(ctx, nil) {
		t.Error("publicKeyHandler() = true, want false (public key auth not supported)")
	}
}
