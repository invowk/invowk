// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/selfupdate"
)

type (
	// upgradeTestRelease is the JSON wire format for a GitHub Release API response,
	// matching the structure expected by the selfupdate.GitHubClient. This is the
	// same shape as the unexported githubRelease type in the selfupdate package.
	upgradeTestRelease struct {
		TagName    string             `json:"tag_name"`
		Name       string             `json:"name"`
		Prerelease bool               `json:"prerelease"`
		Draft      bool               `json:"draft"`
		HTMLURL    string             `json:"html_url"`
		CreatedAt  string             `json:"created_at"`
		Assets     []upgradeTestAsset `json:"assets"`
	}

	// upgradeTestAsset is the JSON wire format for a GitHub Release asset.
	upgradeTestAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
		ContentType        string `json:"content_type"`
	}
)

// setupUpgradeTestServer creates an httptest server that serves GitHub Releases
// API responses from the given release list. The server handles:
//   - GET /repos/invowk/invowk/releases -> JSON array of releases
//   - GET /repos/invowk/invowk/releases/tags/{tag} -> single release or 404
//
// Returns the server (automatically closed via t.Cleanup) and a configured
// Updater pointing at the test server.
func setupUpgradeTestServer(t *testing.T, currentVersion string, releases []upgradeTestRelease) (*selfupdate.Updater, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle release list endpoint.
		if strings.HasSuffix(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/tags/") {
			if err := json.NewEncoder(w).Encode(releases); err != nil {
				t.Errorf("encoding releases: %v", err)
			}
			return
		}

		// Handle release by tag endpoint.
		if strings.Contains(r.URL.Path, "/releases/tags/") {
			tag := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			for _, rel := range releases {
				if rel.TagName == tag {
					if err := json.NewEncoder(w).Encode(rel); err != nil {
						t.Errorf("encoding release: %v", err)
					}
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}

		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"message":"Not Found","path":%q}`, r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	client := selfupdate.NewGitHubClient(selfupdate.WithBaseURL(srv.URL))
	updater := selfupdate.NewUpdater(currentVersion, selfupdate.WithGitHubClient(client))

	return updater, srv
}

func TestRunUpgrade_UpgradeAvailable_CheckMode(t *testing.T) {
	t.Parallel()

	releases := []upgradeTestRelease{
		{
			TagName:    "v1.1.0",
			Name:       "v1.1.0",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.1.0",
			CreatedAt:  "2026-01-01T00:00:00Z",
		},
	}

	updater, _ := setupUpgradeTestServer(t, "v1.0.0", releases)

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		check:   true,
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	wantTokens := []string{
		"Current version: v1.0.0",
		"Latest version:  v1.1.0",
		"An upgrade is available",
	}
	for _, token := range wantTokens {
		if !strings.Contains(out, token) {
			t.Errorf("stdout %q does not contain expected token %q", out, token)
		}
	}

	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunUpgrade_AlreadyUpToDate(t *testing.T) {
	t.Parallel()

	releases := []upgradeTestRelease{
		{
			TagName:    "v1.0.0",
			Name:       "v1.0.0",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.0.0",
			CreatedAt:  "2026-01-01T00:00:00Z",
		},
	}

	updater, _ := setupUpgradeTestServer(t, "v1.0.0", releases)

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Already up to date") {
		t.Errorf("stdout %q does not contain 'Already up to date'", out)
	}
}

func TestRunUpgrade_PreReleaseAhead(t *testing.T) {
	t.Parallel()

	releases := []upgradeTestRelease{
		{
			TagName:    "v1.0.0",
			Name:       "v1.0.0",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.0.0",
			CreatedAt:  "2026-01-01T00:00:00Z",
		},
	}

	updater, _ := setupUpgradeTestServer(t, "v1.1.0-alpha.1", releases)

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(strings.ToLower(out), "pre-release") {
		t.Errorf("stdout %q does not contain 'pre-release' (case-insensitive)", out)
	}
}

// TestRunUpgrade_HomebrewDetected and TestRunUpgrade_GoInstallDetected are
// intentionally omitted here. The install method detection is driven by
// unexported test seams in the selfupdate package (osExecutable, evalSymlinks,
// installMethodHint, readBuildInfo) that cannot be overridden from package cmd.
// The Homebrew and GoInstall routing paths are thoroughly tested in
// internal/selfupdate/selfupdate_test.go (TestUpdater_Check_HomebrewDetected
// and TestUpdater_Check_GoInstallDetected).

func TestRunUpgrade_NetworkError(t *testing.T) {
	t.Parallel()

	// Server returns 500 for all requests, simulating a network/server failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := selfupdate.NewGitHubClient(selfupdate.WithBaseURL(srv.URL))
	updater := selfupdate.NewUpdater("v1.0.0", selfupdate.WithGitHubClient(client))

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err == nil {
		t.Fatal("expected error for server failure, got nil")
	}

	if got := classifyUpgradeExitCode(err); got != 2 {
		t.Errorf("classifyUpgradeExitCode() = %d, want 2 for network error", got)
	}
}

func TestRunUpgrade_RateLimited(t *testing.T) {
	t.Parallel()

	resetTime := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))
	t.Cleanup(srv.Close)

	client := selfupdate.NewGitHubClient(selfupdate.WithBaseURL(srv.URL))
	updater := selfupdate.NewUpdater("v1.0.0", selfupdate.WithGitHubClient(client))

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err == nil {
		t.Fatal("expected error for rate limit, got nil")
	}

	formatted := formatUpgradeError(err)
	wantTokens := []string{"rate limit", "GITHUB_TOKEN"}
	for _, token := range wantTokens {
		if !strings.Contains(strings.ToLower(formatted), strings.ToLower(token)) {
			t.Errorf("formatUpgradeError() %q does not contain %q", formatted, token)
		}
	}
}

func TestRunUpgrade_SpecificVersion(t *testing.T) {
	t.Parallel()

	releases := []upgradeTestRelease{
		{
			TagName:    "v1.0.5",
			Name:       "v1.0.5",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.0.5",
			CreatedAt:  "2026-01-15T00:00:00Z",
		},
	}

	updater, _ := setupUpgradeTestServer(t, "v1.0.0", releases)

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		target:  "v1.0.5",
		check:   true,
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "v1.0.5") {
		t.Errorf("stdout %q does not contain target version 'v1.0.5'", out)
	}
	if !strings.Contains(out, "An upgrade is available") {
		t.Errorf("stdout %q does not contain 'An upgrade is available'", out)
	}
}

func TestRunUpgrade_ReleaseNotFound(t *testing.T) {
	t.Parallel()

	// Server with no releases matching v9.9.9 — tag lookup returns 404.
	releases := []upgradeTestRelease{
		{
			TagName:    "v1.0.0",
			Name:       "v1.0.0",
			Prerelease: false,
			Draft:      false,
			CreatedAt:  "2026-01-01T00:00:00Z",
		},
	}

	updater, _ := setupUpgradeTestServer(t, "v1.0.0", releases)

	var stdout, stderr bytes.Buffer
	p := upgradeParams{
		stdout:  &stdout,
		stderr:  &stderr,
		updater: updater,
		target:  "v9.9.9",
		yes:     true,
	}

	err := runUpgrade(context.Background(), p)
	if err == nil {
		t.Fatal("expected error for missing release, got nil")
	}

	if !errors.Is(err, selfupdate.ErrReleaseNotFound) {
		t.Errorf("expected error wrapping ErrReleaseNotFound, got: %v", err)
	}

	if got := classifyUpgradeExitCode(err); got != 1 {
		t.Errorf("classifyUpgradeExitCode() = %d, want 1 for ErrReleaseNotFound", got)
	}
}

func TestClassifyUpgradeExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "permission error returns 1",
			err:      os.ErrPermission,
			wantCode: 1,
		},
		{
			name:     "wrapped permission error returns 1",
			err:      fmt.Errorf("cannot replace binary: %w", os.ErrPermission),
			wantCode: 1,
		},
		{
			name:     "release not found returns 1",
			err:      selfupdate.ErrReleaseNotFound,
			wantCode: 1,
		},
		{
			name:     "wrapped release not found returns 1",
			err:      fmt.Errorf("fetching release v9.9.9: %w", selfupdate.ErrReleaseNotFound),
			wantCode: 1,
		},
		{
			name:     "generic error returns 2",
			err:      errors.New("connection refused"),
			wantCode: 2,
		},
		{
			name:     "nil-safe generic error returns 2",
			err:      fmt.Errorf("unexpected: %w", errors.New("boom")),
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyUpgradeExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("classifyUpgradeExitCode(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}

func TestFormatUpgradeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantTokens []string
	}{
		{
			name: "rate limit error mentions token guidance",
			err: &selfupdate.RateLimitError{
				Limit:     60,
				Remaining: 0,
				ResetAt:   time.Date(2026, 2, 15, 14, 30, 0, 0, time.UTC),
			},
			wantTokens: []string{"rate limit", "GITHUB_TOKEN"},
		},
		{
			name: "checksum error mentions verification failure",
			err: &selfupdate.ChecksumError{
				Filename: "invowk_1.0.0_linux_amd64.tar.gz",
				Expected: "aaaa",
				Got:      "bbbb",
			},
			wantTokens: []string{"checksum verification failed", "aaaa", "bbbb"},
		},
		{
			name:       "permission error suggests elevated privileges",
			err:        fmt.Errorf("replacing binary: %w", os.ErrPermission),
			wantTokens: []string{"permissions", "sudo"},
		},
		{
			name:       "generic error suggests network check",
			err:        errors.New("connection refused"),
			wantTokens: []string{"network connection", "connection refused"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatUpgradeError(tt.err)
			for _, token := range tt.wantTokens {
				if !strings.Contains(strings.ToLower(got), strings.ToLower(token)) {
					t.Errorf("formatUpgradeError() = %q, missing token %q", got, token)
				}
			}
		})
	}
}
