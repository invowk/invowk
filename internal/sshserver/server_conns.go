// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"errors"
	"net"
	"sync"

	"charm.land/ssh"
)

type (
	// tokenConnContextKey stores a connection's *tokenConn in its ssh.Context.
	tokenConnContextKey struct{}

	// tokenConn wraps an accepted connection so that revoking the token it
	// authenticated with closes it. SSH authenticates once per connection and
	// an authenticated client may open further sessions without logging in
	// again, so revocation must close the connection, not only its sessions
	// (finding F7 in formal/README.md).
	tokenConn struct {
		net.Conn
		server *Server
		// token and commandID are set under server.tokenMu when
		// authentication succeeds; commandID survives the token's expiry.
		token     TokenValue
		commandID CommandID
		// closed is set under server.tokenMu when Close begins; admitConn
		// then refuses the connection, so a login that completes after
		// teardown closed it is never indexed.
		closed    bool
		closeOnce sync.Once
		closeErr  error
	}
)

// Validate returns nil when the connection is unauthenticated (no token yet)
// or bound to a valid token and command.
func (c *tokenConn) Validate() error {
	if c.token == "" {
		return nil
	}
	return errors.Join(c.token.Validate(), c.commandID.Validate())
}

// Close forgets the connection and closes it; later calls return the first
// result, so revocation and the SSH server may both close it.
func (c *tokenConn) Close() error {
	c.closeOnce.Do(func() {
		c.server.forgetConn(c)
		c.closeErr = c.Conn.Close()
	})
	return c.closeErr
}

// trackConnections installs the connection wrapper that lets token revocation
// close the connections the token authenticated, and that lets Stop close
// every accepted connection and wait for its handling (finding F11).
//
// The library calls ConnCallback before the handshake, in the connection
// goroutine, and defers ConnectionCloseCallback exactly once for every
// connection whose ConnCallback returned non-nil, so connWg's Add and Done
// pair up; a refused connection (nil) is closed by the library and gets
// neither (charm.land/ssh v0.4.3 handleConn).
func (s *Server) trackConnections() ssh.Option {
	return func(srv *ssh.Server) error {
		srv.ConnCallback = func(ctx ssh.Context, conn net.Conn) net.Conn {
			tracked := &tokenConn{Conn: conn, server: s}
			if !s.acceptConn(tracked) {
				return nil
			}
			ctx.SetValue(tokenConnContextKey{}, tracked)
			return tracked
		}
		srv.ConnectionCloseCallback = func(net.Conn) {
			s.connWg.Done()
		}
		return nil
	}
}

// acceptConn records an accepted connection and counts its connection
// goroutine, unless a teardown began: then it refuses. The Add happens under
// connMu before teardown sets acceptClosed, and so before teardown's Wait.
func (s *Server) acceptConn(conn *tokenConn) bool {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.acceptClosed {
		return false
	}
	s.accepted[conn] = struct{}{}
	s.connWg.Add(1)
	return true
}

// closeAcceptedConns refuses further connections and closes every accepted
// one, authenticated or not. Only teardown calls it.
func (s *Server) closeAcceptedConns() {
	s.connMu.Lock()
	s.acceptClosed = true
	conns := make([]*tokenConn, 0, len(s.accepted))
	for conn := range s.accepted {
		conns = append(conns, conn)
	}
	s.connMu.Unlock()
	closeConns(conns)
}

// admitConn validates the token and, in the same critical section, binds the
// connection to it, so a concurrent revocation either rejects this login or
// closes this connection. Expired tokens are dropped without closing
// connections: expiry bounds new logins, execution end bounds sessions.
func (s *Server) admitConn(tokenValue TokenValue, conn *tokenConn) (*Token, bool) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if conn.closed {
		return nil, false
	}
	token, exists := s.tokens[tokenValue]
	if !exists {
		return nil, false
	}
	if s.clock.Now().After(token.ExpiresAt) {
		delete(s.tokens, tokenValue)
		return nil, false
	}
	conn.token, conn.commandID = tokenValue, token.CommandID
	if s.conns[tokenValue] == nil {
		s.conns[tokenValue] = make(map[*tokenConn]struct{})
	}
	s.conns[tokenValue][conn] = struct{}{}
	return cloneToken(token), true
}

// revokeLocked deletes the token and detaches its connections; the caller
// closes them after releasing tokenMu.
func (s *Server) revokeLocked(tokenValue TokenValue) []*tokenConn {
	delete(s.tokens, tokenValue)
	conns := make([]*tokenConn, 0, len(s.conns[tokenValue]))
	for conn := range s.conns[tokenValue] {
		conns = append(conns, conn)
	}
	delete(s.conns, tokenValue)
	return conns
}

// forgetConn marks the connection closed and removes it from its token's
// index and from the accepted set.
func (s *Server) forgetConn(conn *tokenConn) {
	s.tokenMu.Lock()
	conn.closed = true
	if conns, ok := s.conns[conn.token]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(s.conns, conn.token)
		}
	}
	s.tokenMu.Unlock()

	s.connMu.Lock()
	delete(s.accepted, conn)
	s.connMu.Unlock()
}

func closeConns(conns []*tokenConn) {
	for _, conn := range conns {
		_ = conn.Close() //nolint:errcheck // best effort: the peer may already be gone
	}
}
