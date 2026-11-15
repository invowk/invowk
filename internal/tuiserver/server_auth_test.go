// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/testutil"
)

// getWithToken sends GET path to the server, with the bearer token when token
// is non-empty, and returns the status code and body.
func getWithToken(t *testing.T, base URL, path string, token AuthToken) (code int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, string(base)+path, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest(%s): %v", path, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+string(token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body of %s: %v", path, err)
	}
	return resp.StatusCode, string(raw)
}

// TestServerAuthCoversEveryPath pins that the token check wraps the whole mux
// and runs before routing and the method check.
func TestServerAuthCoversEveryPath(t *testing.T) {
	t.Parallel()

	server, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if startErr := server.Start(t.Context()); startErr != nil {
		t.Fatalf("Start(): %v", startErr)
	}
	t.Cleanup(func() { testutil.MustStop(t, server) })

	tests := []struct {
		name     string
		path     string
		token    AuthToken
		wantCode int
		wantBody string
	}{
		{name: "health without token", path: "/health", wantCode: http.StatusUnauthorized, wantBody: "unauthorized"},
		{name: "health with wrong token", path: "/health", token: AuthToken("wrong"), wantCode: http.StatusUnauthorized, wantBody: "unauthorized"},
		{name: "health with token", path: "/health", token: server.Token(), wantCode: http.StatusOK, wantBody: "ok"},
		{name: "tui GET without token is 401 not 405", path: "/tui", wantCode: http.StatusUnauthorized, wantBody: "unauthorized"},
		{name: "root without token", path: "/", wantCode: http.StatusUnauthorized, wantBody: "unauthorized"},
		{name: "unknown path without token", path: "/nope", wantCode: http.StatusUnauthorized, wantBody: "unauthorized"},
		{name: "unknown path with token", path: "/nope", token: server.Token(), wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := getWithToken(t, server.URL(), tt.path, tt.token)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d (body %q)", code, tt.wantCode, body)
			}
			if tt.wantBody != "" && body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

// TestServerAuthCoversEveryMethod pins that the token check also sees methods
// net/http could answer before the handler. "OPTIONS *" goes over a raw
// connection, because http.Client cannot send an asterisk target; without
// DisableGeneralOptionsHandler net/http answers it with 200 itself.
func TestServerAuthCoversEveryMethod(t *testing.T) {
	t.Parallel()

	server, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if startErr := server.Start(t.Context()); startErr != nil {
		t.Fatalf("Start(): %v", startErr)
	}
	t.Cleanup(func() { testutil.MustStop(t, server) })
	hostPort := strings.TrimPrefix(string(server.URL()), "http://")

	tests := []struct {
		name   string
		method string
		target string
	}{
		{name: "OPTIONS asterisk", method: http.MethodOptions, target: "*"},
		{name: "HEAD health", method: http.MethodHead, target: "/health"},
		{name: "OPTIONS health", method: http.MethodOptions, target: "/health"},
		{name: "OPTIONS tui", method: http.MethodOptions, target: "/tui"},
		{name: "POST tui", method: http.MethodPost, target: "/tui"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if code := rawStatus(t, hostPort, tt.method, tt.target); code != http.StatusUnauthorized {
				t.Errorf("%s %s without token: status = %d, want 401", tt.method, tt.target, code)
			}
		})
	}
}

// rawStatus sends one request line over a raw TCP connection and returns the
// response status code.
func rawStatus(t *testing.T, hostPort, method, target string) int {
	t.Helper()
	var dialer net.Dialer
	conn, err := dialer.DialContext(t.Context(), "tcp", hostPort)
	if err != nil {
		t.Fatalf("dial %s: %v", hostPort, err)
	}
	defer func() { _ = conn.Close() }()
	if deadlineErr := conn.SetDeadline(time.Now().Add(10 * time.Second)); deadlineErr != nil {
		t.Fatalf("SetDeadline: %v", deadlineErr)
	}
	body := ""
	if method == http.MethodPost {
		body = `{"component":"confirm","options":{"title":"x"}}`
	}
	request := method + " " + target + " HTTP/1.1\r\nHost: " + hostPort +
		"\r\nContent-Type: application/json\r\nContent-Length: " + strconv.Itoa(len(body)) +
		"\r\nConnection: close\r\n\r\n" + body
	if _, writeErr := io.WriteString(conn, request); writeErr != nil {
		t.Fatalf("write request: %v", writeErr)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
	if err != nil {
		t.Fatalf("read response to %s %s: %v", method, target, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// TestServerStopDuringStartDoesNotBlock pins that a Stop that wins the race
// against Start makes Start return at once and close the bound listeners,
// instead of leaving the readiness channel open.
func TestServerStopDuringStartDoesNotBlock(t *testing.T) {
	t.Parallel()

	server, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if startErr := server.base.TransitionToStarting(t.Context()); startErr != nil {
		t.Fatalf("TransitionToStarting(): %v", startErr)
	}
	if !server.base.TransitionToStopping() {
		t.Fatal("TransitionToStopping() = false, want true")
	}
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(): %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- server.serveListeners([]net.Listener{listener}) }()
	select {
	case serveErr := <-done:
		if !errors.Is(serveErr, errServerStoppedDuringStart) {
			t.Fatalf("serveListeners() = %v, want errServerStoppedDuringStart", serveErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveListeners() blocked after a Stop that raced Start")
	}
	if _, acceptErr := listener.Accept(); acceptErr == nil {
		t.Fatal("listener still accepts connections, want it closed")
	}
	server.base.TransitionToStopped()
}
