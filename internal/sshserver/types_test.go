// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestHostAddress_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		addr    HostAddress
		want    bool
		wantErr bool
	}{
		{"localhost", HostAddress("localhost"), true, false},
		{"ipv4", HostAddress("127.0.0.1"), true, false},
		{"ipv6 loopback", HostAddress("::1"), true, false},
		{"hostname", HostAddress("myhost.local"), true, false},
		{"all interfaces", HostAddress("0.0.0.0"), true, false},
		{"empty", HostAddress(""), false, true},
		{"whitespace only", HostAddress("   "), false, true},
		{"tabs only", HostAddress("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.addr.IsValid()
			if isValid != tt.want {
				t.Errorf("HostAddress(%q).IsValid() = %v, want %v", tt.addr, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("HostAddress(%q).IsValid() returned no errors, want error", tt.addr)
				}
				if !errors.Is(errs[0], ErrInvalidHostAddress) {
					t.Errorf("error should wrap ErrInvalidHostAddress, got: %v", errs[0])
				}
				var addrErr *InvalidHostAddressError
				if !errors.As(errs[0], &addrErr) {
					t.Errorf("error should be *InvalidHostAddressError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("HostAddress(%q).IsValid() returned unexpected errors: %v", tt.addr, errs)
			}
		})
	}
}

func TestHostAddress_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr HostAddress
		want string
	}{
		{HostAddress("127.0.0.1"), "127.0.0.1"},
		{HostAddress("localhost"), "localhost"},
		{HostAddress(""), ""},
	}

	for _, tt := range tests {
		if got := tt.addr.String(); got != tt.want {
			t.Errorf("HostAddress(%q).String() = %q, want %q", string(tt.addr), got, tt.want)
		}
	}
}

func TestTokenValue_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   TokenValue
		want    bool
		wantErr bool
	}{
		{"valid token", TokenValue("abc123def456"), true, false},
		{"single char", TokenValue("x"), true, false},
		{"with special chars", TokenValue("token-with_special.chars"), true, false},
		{"empty", TokenValue(""), false, true},
		{"whitespace only", TokenValue("   "), false, true},
		{"tabs only", TokenValue("\t\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.token.IsValid()
			if isValid != tt.want {
				t.Errorf("TokenValue(%q).IsValid() = %v, want %v", tt.token, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("TokenValue(%q).IsValid() returned no errors, want error", tt.token)
				}
				if !errors.Is(errs[0], ErrInvalidTokenValue) {
					t.Errorf("error should wrap ErrInvalidTokenValue, got: %v", errs[0])
				}
				var tokenErr *InvalidTokenValueError
				if !errors.As(errs[0], &tokenErr) {
					t.Errorf("error should be *InvalidTokenValueError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("TokenValue(%q).IsValid() returned unexpected errors: %v", tt.token, errs)
			}
		})
	}
}

func TestTokenValue_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		token TokenValue
		want  string
	}{
		{TokenValue("abc123"), "abc123"},
		{TokenValue(""), ""},
	}

	for _, tt := range tests {
		if got := tt.token.String(); got != tt.want {
			t.Errorf("TokenValue(%q).String() = %q, want %q", string(tt.token), got, tt.want)
		}
	}
}

func TestListenPort_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    ListenPort
		want    bool
		wantErr bool
	}{
		{"zero auto-select", ListenPort(0), true, false},
		{"port 1", ListenPort(1), true, false},
		{"standard SSH", ListenPort(22), true, false},
		{"standard HTTP", ListenPort(80), true, false},
		{"high port", ListenPort(8080), true, false},
		{"max port", ListenPort(65535), true, false},
		{"negative", ListenPort(-1), false, true},
		{"too large", ListenPort(65536), false, true},
		{"very negative", ListenPort(-1000), false, true},
		{"very large", ListenPort(100000), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.port.IsValid()
			if isValid != tt.want {
				t.Errorf("ListenPort(%d).IsValid() = %v, want %v", tt.port, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ListenPort(%d).IsValid() returned no errors, want error", tt.port)
				}
				if !errors.Is(errs[0], ErrInvalidListenPort) {
					t.Errorf("error should wrap ErrInvalidListenPort, got: %v", errs[0])
				}
				var portErr *InvalidListenPortError
				if !errors.As(errs[0], &portErr) {
					t.Errorf("error should be *InvalidListenPortError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ListenPort(%d).IsValid() returned unexpected errors: %v", tt.port, errs)
			}
		})
	}
}

func TestListenPort_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		port ListenPort
		want string
	}{
		{ListenPort(0), "0"},
		{ListenPort(8080), "8080"},
		{ListenPort(65535), "65535"},
	}

	for _, tt := range tests {
		if got := tt.port.String(); got != tt.want {
			t.Errorf("ListenPort(%d).String() = %q, want %q", int(tt.port), got, tt.want)
		}
	}
}

func TestSSHConfig_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       Config
		want      bool
		wantErr   bool
		wantCount int // expected number of field errors
	}{
		{
			"all valid",
			Config{
				Host:         HostAddress("127.0.0.1"),
				Port:         ListenPort(2222),
				DefaultShell: invowkfile.ShellPath("/bin/sh"),
			},
			true, false, 0,
		},
		{
			"valid with zero port (auto-select)",
			Config{
				Host:         HostAddress("localhost"),
				Port:         ListenPort(0),
				DefaultShell: invowkfile.ShellPath("/bin/bash"),
			},
			true, false, 0,
		},
		{
			"invalid host (empty)",
			Config{
				Host:         HostAddress(""),
				Port:         ListenPort(22),
				DefaultShell: invowkfile.ShellPath("/bin/sh"),
			},
			false, true, 1,
		},
		{
			"invalid port (negative)",
			Config{
				Host:         HostAddress("127.0.0.1"),
				Port:         ListenPort(-1),
				DefaultShell: invowkfile.ShellPath("/bin/sh"),
			},
			false, true, 1,
		},
		{
			"invalid default shell (whitespace-only)",
			Config{
				Host:         HostAddress("127.0.0.1"),
				Port:         ListenPort(22),
				DefaultShell: invowkfile.ShellPath("   "),
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			Config{
				Host:         HostAddress(""),
				Port:         ListenPort(70000),
				DefaultShell: invowkfile.ShellPath("  "),
			},
			false, true, 3,
		},
		{
			"zero value struct",
			Config{},
			false, true, 1, // empty Host is invalid; Port 0 and empty ShellPath are valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cfg.IsValid()
			if isValid != tt.want {
				t.Errorf("Config.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Config.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidSSHConfig) {
					t.Errorf("error should wrap ErrInvalidSSHConfig, got: %v", errs[0])
				}
				var cfgErr *InvalidSSHConfigError
				if !errors.As(errs[0], &cfgErr) {
					t.Fatalf("error should be *InvalidSSHConfigError, got: %T", errs[0])
				}
				if len(cfgErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(cfgErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("Config.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}
