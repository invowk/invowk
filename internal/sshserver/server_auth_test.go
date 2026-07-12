// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"net"
	"slices"
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

func TestPasswordHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		commandID   CommandID
		rawTokens   []string
		useCount    int
		revokeAfter int
		expire      bool
		want        []bool
	}{
		{name: "valid token returns true", commandID: "cmd-123", useCount: 1, want: []bool{true}},
		{name: "valid token can be reused until revoked", commandID: "cmd-reuse", useCount: 3, revokeAfter: 2, want: []bool{true, true, false}},
		{name: "invalid token format returns false", rawTokens: []string{"", "   "}, want: []bool{false, false}},
		{name: "unknown token returns false", rawTokens: []string{"abcdef1234567890abcdef1234567890"}, want: []bool{false}},
		{name: "expired token returns false", commandID: "cmd-456", useCount: 1, expire: true, want: []bool{false}},
		{name: "revoked token returns false", commandID: "cmd-789", useCount: 1, revokeAfter: -1, want: []bool{false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultConfig()
			cfg.TokenTTL = time.Hour
			clock := testutil.NewFakeClock(time.Now())
			srv, err := NewWithClock(cfg, clock)
			if err != nil {
				t.Fatalf("NewWithClock() error = %v", err)
			}

			tokens := append([]string(nil), tt.rawTokens...)
			var generated TokenValue
			if tt.commandID != "" {
				token, generateErr := srv.GenerateToken(tt.commandID)
				if generateErr != nil {
					t.Fatalf("GenerateToken() error = %v", generateErr)
				}
				generated = token.Value
				if tt.revokeAfter < 0 {
					srv.RevokeToken(generated)
				}
				if tt.expire {
					clock.Advance(cfg.TokenTTL + time.Second)
				}
				for range tt.useCount {
					tokens = append(tokens, string(generated))
				}
			}

			got := make([]bool, 0, len(tokens))
			for i, token := range tokens {
				if tt.revokeAfter > 0 && i == tt.revokeAfter {
					srv.RevokeToken(generated)
				}
				got = append(got, srv.passwordHandler(newStubSSHContext(t.Context()), token))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("passwordHandler() results = %v, want %v", got, tt.want)
			}
		})
	}
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
