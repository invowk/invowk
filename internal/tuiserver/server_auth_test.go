// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"io"
	"net/http"
	"testing"

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
