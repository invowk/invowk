// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/charmbracelet/ssh"
)

// GenerateToken creates a new authentication token for a command.
func (s *Server) GenerateToken(commandID string) (*Token, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	tokenValue := TokenValue(hex.EncodeToString(tokenBytes))
	now := s.clock.Now()

	token := &Token{
		Value:     tokenValue,
		CreatedAt: now,
		ExpiresAt: now.Add(s.cfg.TokenTTL),
		CommandID: commandID,
		Used:      false,
	}

	s.tokenMu.Lock()
	s.tokens[tokenValue] = token
	s.tokenMu.Unlock()

	s.logger.Debug("Generated token", "commandID", commandID)

	return token, nil
}

// ValidateToken checks if a token is valid.
func (s *Server) ValidateToken(tokenValue TokenValue) (*Token, bool) {
	s.tokenMu.RLock()
	token, exists := s.tokens[tokenValue]
	s.tokenMu.RUnlock()

	if !exists {
		return nil, false
	}

	if s.clock.Now().After(token.ExpiresAt) {
		s.RevokeToken(tokenValue)
		return nil, false
	}

	return token, true
}

// RevokeToken invalidates a token.
func (s *Server) RevokeToken(tokenValue TokenValue) {
	s.tokenMu.Lock()
	delete(s.tokens, tokenValue)
	s.tokenMu.Unlock()
}

// RevokeTokensForCommand revokes all tokens for a specific command.
// This is useful for cleanup when a command execution completes.
// Currently exercised by tests; production callers will use this
// when command lifecycle management is fully integrated.
func (s *Server) RevokeTokensForCommand(commandID string) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	for tokenValue, token := range s.tokens {
		if token.CommandID == commandID {
			delete(s.tokens, tokenValue)
		}
	}
}

// GetConnectionInfo returns connection information for a command.
// Returns an error if the server is not running.
func (s *Server) GetConnectionInfo(commandID string) (*ConnectionInfo, error) {
	if !s.IsRunning() {
		return nil, fmt.Errorf("SSH server is not running (state: %s)", s.State())
	}

	token, err := s.GenerateToken(commandID)
	if err != nil {
		return nil, err
	}

	return &ConnectionInfo{
		Host:     s.cfg.Host,
		Port:     s.Port(),
		Token:    token.Value,
		User:     "invowk",
		ExpireAt: token.ExpiresAt,
	}, nil
}

// cleanupExpiredTokens periodically removes expired tokens.
func (s *Server) cleanupExpiredTokens() {
	defer s.DoneGoroutine()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	ctx := s.Context()
	if ctx == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tokenMu.Lock()
			now := s.clock.Now()
			for tokenValue, token := range s.tokens {
				if now.After(token.ExpiresAt) {
					delete(s.tokens, tokenValue)
				}
			}
			s.tokenMu.Unlock()
		}
	}
}

// passwordHandler handles password authentication using tokens.
func (s *Server) passwordHandler(ctx ssh.Context, password string) bool {
	token, valid := s.ValidateToken(TokenValue(password))
	if !valid {
		s.logger.Warn("Invalid token authentication attempt", "user", ctx.User())
		return false
	}

	// Store the token info in the context for later use
	ctx.SetValue("token", token)
	ctx.SetValue("commandID", token.CommandID)

	s.logger.Debug("Token authentication successful", "commandID", token.CommandID)
	return true
}

// publicKeyHandler rejects all public key authentication.
// We only want token-based authentication.
func (s *Server) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	// Reject public key auth - we only accept token-based password auth
	return false
}
