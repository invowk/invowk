package sshserver

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	srv, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

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
	srv, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

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
	srv, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

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
	srv, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

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
	cfg := DefaultConfig()
	cfg.Port = 0 // Auto-select port

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if srv.IsRunning() {
		t.Error("Server should not be running before Start()")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	if !srv.IsRunning() {
		t.Error("Server should be running after Start()")
	}

	if srv.Port() == 0 {
		t.Error("Server port should be assigned")
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	if srv.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}
}

func TestGetConnectionInfo(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = 0

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Should fail before server starts
	_, err = srv.GetConnectionInfo("test")
	if err == nil {
		t.Error("GetConnectionInfo should fail when server is not running")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

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
	cfg := DefaultConfig()
	cfg.TokenTTL = 1 * time.Millisecond // Very short TTL

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	token, err := srv.GenerateToken("test-command")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Token should be expired
	_, ok := srv.ValidateToken(token.Value)
	if ok {
		t.Error("Expired token should not be valid")
	}
}
