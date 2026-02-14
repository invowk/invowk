// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestListReleases_FiltersStableOnly(t *testing.T) {
	t.Parallel()

	releases := []githubRelease{
		{TagName: "v1.2.0", Name: "Stable 1.2.0", Draft: false, Prerelease: false},
		{TagName: "v1.3.0-alpha.1", Name: "Alpha", Draft: false, Prerelease: true},
		{TagName: "v1.1.0", Name: "Stable 1.1.0", Draft: false, Prerelease: false},
		{TagName: "v2.0.0", Name: "Draft 2.0", Draft: true, Prerelease: false},
		{TagName: "v1.0.0", Name: "Stable 1.0.0", Draft: false, Prerelease: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("encoding releases: %v", err)
		}
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	got, err := client.ListReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out the draft and prerelease, leaving 3 stable releases.
	if len(got) != 3 {
		t.Fatalf("expected 3 stable releases, got %d", len(got))
	}

	// Should be sorted by semver descending: v1.2.0, v1.1.0, v1.0.0
	wantOrder := []string{"v1.2.0", "v1.1.0", "v1.0.0"}
	for i, want := range wantOrder {
		if got[i].TagName != want {
			t.Errorf("release[%d]: got tag %q, want %q", i, got[i].TagName, want)
		}
	}
}

func TestListReleases_Pagination(t *testing.T) {
	t.Parallel()

	page1 := []githubRelease{
		{TagName: "v2.0.0", Name: "Stable 2.0.0", Draft: false, Prerelease: false},
	}
	page2 := []githubRelease{
		{TagName: "v1.0.0", Name: "Stable 1.0.0", Draft: false, Prerelease: false},
	}

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		pageParam := r.URL.Query().Get("page")
		if pageParam == "2" {
			// Second page: no Link header (last page).
			if err := json.NewEncoder(w).Encode(page2); err != nil {
				t.Errorf("encoding page 2: %v", err)
			}
			return
		}

		// First page: include Link header pointing to page 2.
		nextURL := fmt.Sprintf("%s/repos/invowk/invowk/releases?per_page=30&page=2", srvURL)
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
		if err := json.NewEncoder(w).Encode(page1); err != nil {
			t.Errorf("encoding page 1: %v", err)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := NewGitHubClient(WithBaseURL(srv.URL))
	got, err := client.ListReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 releases across 2 pages, got %d", len(got))
	}

	// Sorted descending: v2.0.0 first, v1.0.0 second.
	if got[0].TagName != "v2.0.0" {
		t.Errorf("release[0]: got tag %q, want %q", got[0].TagName, "v2.0.0")
	}
	if got[1].TagName != "v1.0.0" {
		t.Errorf("release[1]: got tag %q, want %q", got[1].TagName, "v1.0.0")
	}
}

func TestGetReleaseByTag_Success(t *testing.T) {
	t.Parallel()

	release := githubRelease{
		TagName:    "v1.5.0",
		Name:       "Release 1.5.0",
		Draft:      false,
		Prerelease: false,
		HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.5.0",
		CreatedAt:  "2025-06-15T10:30:00Z",
		Assets: []githubAsset{
			{
				Name:               "invowk_1.5.0_linux_amd64.tar.gz",
				BrowserDownloadURL: "https://github.com/invowk/invowk/releases/download/v1.5.0/invowk_1.5.0_linux_amd64.tar.gz",
				Size:               5242880,
				ContentType:        "application/gzip",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/tags/v1.5.0") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encoding release: %v", err)
		}
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	got, err := client.GetReleaseByTag(context.Background(), "v1.5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.TagName != "v1.5.0" {
		t.Errorf("got tag %q, want %q", got.TagName, "v1.5.0")
	}
	if got.Name != "Release 1.5.0" {
		t.Errorf("got name %q, want %q", got.Name, "Release 1.5.0")
	}
	if got.HTMLURL != "https://github.com/invowk/invowk/releases/tag/v1.5.0" {
		t.Errorf("got HTML URL %q, want %q", got.HTMLURL, "https://github.com/invowk/invowk/releases/tag/v1.5.0")
	}
	if len(got.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(got.Assets))
	}
	if got.Assets[0].Name != "invowk_1.5.0_linux_amd64.tar.gz" {
		t.Errorf("got asset name %q, want %q", got.Assets[0].Name, "invowk_1.5.0_linux_amd64.tar.gz")
	}
	if got.Assets[0].Size != 5242880 {
		t.Errorf("got asset size %d, want %d", got.Assets[0].Size, 5242880)
	}
}

func TestGetReleaseByTag_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	got, err := client.GetReleaseByTag(context.Background(), "v99.99.99")

	if got != nil {
		t.Errorf("expected nil release, got %+v", got)
	}
	if !errors.Is(err, ErrReleaseNotFound) {
		t.Errorf("expected ErrReleaseNotFound, got %v", err)
	}
}

func TestRateLimitError(t *testing.T) {
	t.Parallel()

	resetTime := time.Date(2025, 7, 1, 14, 30, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	_, err := client.GetReleaseByTag(context.Background(), "v1.0.0")

	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}

	if rle.Limit != 60 {
		t.Errorf("got limit %d, want 60", rle.Limit)
	}
	if rle.Remaining != 0 {
		t.Errorf("got remaining %d, want 0", rle.Remaining)
	}
	if !rle.ResetAt.Equal(resetTime) {
		t.Errorf("got reset time %v, want %v", rle.ResetAt, resetTime)
	}

	wantMsg := "GitHub API rate limit exceeded (0 remaining, resets at 14:30 UTC)"
	if rle.Error() != wantMsg {
		t.Errorf("got error message %q, want %q", rle.Error(), wantMsg)
	}
}

func TestAuthenticatedRequest(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		// Return a valid release so the request completes normally.
		release := githubRelease{TagName: "v1.0.0", Name: "Test"}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encoding release: %v", err)
		}
	}))
	defer srv.Close()

	token := "ghp_test_token_12345" //nolint:gosec // Fake token for testing only.
	client := NewGitHubClient(
		WithBaseURL(srv.URL),
		WithToken(token),
	)

	_, err := client.GetReleaseByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantAuth := "Bearer " + token
	if gotAuth != wantAuth {
		t.Errorf("got Authorization header %q, want %q", gotAuth, wantAuth)
	}
}

func TestUserAgentHeader(t *testing.T) {
	t.Parallel()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		release := githubRelease{TagName: "v1.0.0", Name: "Test"}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encoding release: %v", err)
		}
	}))
	defer srv.Close()

	customUA := "invowk/1.2.3"
	client := NewGitHubClient(
		WithBaseURL(srv.URL),
		WithUserAgent(customUA),
	)

	_, err := client.GetReleaseByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotUA != customUA {
		t.Errorf("got User-Agent %q, want %q", gotUA, customUA)
	}
}

func TestDownloadAsset(t *testing.T) {
	t.Parallel()

	assetContent := "binary-content-placeholder-for-test"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		fmt.Fprint(w, assetContent)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	body, err := client.DownloadAsset(context.Background(), srv.URL+"/download/asset.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = body.Close() }() // test code: read-only response body

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	if string(data) != assetContent {
		t.Errorf("got body %q, want %q", string(data), assetContent)
	}
}

func TestListReleases_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	_, err := client.ListReleases(context.Background())

	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Errorf("expected error mentioning status 500, got %q", err.Error())
	}
}

func TestDownloadAsset_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	body, err := client.DownloadAsset(context.Background(), srv.URL+"/not-found")

	if body != nil {
		body.Close()
		t.Error("expected nil body for 404 response")
	}
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestNewGitHubClient_Defaults(t *testing.T) {
	t.Parallel()

	client := NewGitHubClient()

	if client.owner != "invowk" {
		t.Errorf("got owner %q, want %q", client.owner, "invowk")
	}
	if client.repo != "invowk" {
		t.Errorf("got repo %q, want %q", client.repo, "invowk")
	}
	if client.baseURL != "https://api.github.com" {
		t.Errorf("got baseURL %q, want %q", client.baseURL, "https://api.github.com")
	}
	if client.userAgent != "invowk/dev" {
		t.Errorf("got userAgent %q, want %q", client.userAgent, "invowk/dev")
	}
	if client.token != "" {
		t.Errorf("got non-empty token %q, want empty", client.token)
	}
}

func TestNewGitHubClient_WithRepo(t *testing.T) {
	t.Parallel()

	client := NewGitHubClient(WithRepo("myorg", "myrepo"))

	if client.owner != "myorg" {
		t.Errorf("got owner %q, want %q", client.owner, "myorg")
	}
	if client.repo != "myrepo" {
		t.Errorf("got repo %q, want %q", client.repo, "myrepo")
	}
}

func TestParseLinkHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "next link present",
			header: `<https://api.github.com/repos/invowk/invowk/releases?page=2>; rel="next", <https://api.github.com/repos/invowk/invowk/releases?page=5>; rel="last"`,
			want:   "https://api.github.com/repos/invowk/invowk/releases?page=2",
		},
		{
			name:   "no next link",
			header: `<https://api.github.com/repos/invowk/invowk/releases?page=1>; rel="prev", <https://api.github.com/repos/invowk/invowk/releases?page=5>; rel="last"`,
			want:   "",
		},
		{
			name:   "next link only",
			header: `<https://api.github.com/repos/invowk/invowk/releases?page=3>; rel="next"`,
			want:   "https://api.github.com/repos/invowk/invowk/releases?page=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseLinkHeader(tt.header)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListReleases_ContextCanceled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Delay long enough that the context cancellation takes effect.
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	client := NewGitHubClient(WithBaseURL(srv.URL))
	_, err := client.ListReleases(ctx)

	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestRateLimitError_NonRateLimit403(t *testing.T) {
	t.Parallel()

	// A 403 without rate limit headers should produce a generic error, not a RateLimitError.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// No rate limit headers set at all.
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"Forbidden"}`)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	_, err := client.GetReleaseByTag(context.Background(), "v1.0.0")

	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}

	// Should NOT be a RateLimitError since there are no rate limit headers.
	var rle *RateLimitError
	if errors.As(err, &rle) {
		t.Errorf("expected non-RateLimitError for 403 without rate limit headers, got %+v", rle)
	}
}

func TestGitHubAPIHeaders(t *testing.T) {
	t.Parallel()

	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		release := githubRelease{TagName: "v1.0.0", Name: "Test"}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encoding release: %v", err)
		}
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	_, err := client.GetReleaseByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotHeaders.Get("Accept"); got != "application/vnd.github+json" {
		t.Errorf("got Accept header %q, want %q", got, "application/vnd.github+json")
	}
	if got := gotHeaders.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
		t.Errorf("got X-GitHub-Api-Version header %q, want %q", got, "2022-11-28")
	}
}

func TestListReleases_SemverSort(t *testing.T) {
	t.Parallel()

	// Return releases in random order to verify sorting.
	releases := []githubRelease{
		{TagName: "v0.1.0", Name: "v0.1.0", Draft: false, Prerelease: false},
		{TagName: "v10.0.0", Name: "v10.0.0", Draft: false, Prerelease: false},
		{TagName: "v2.0.0", Name: "v2.0.0", Draft: false, Prerelease: false},
		{TagName: "v1.9.0", Name: "v1.9.0", Draft: false, Prerelease: false},
		{TagName: "v1.10.0", Name: "v1.10.0", Draft: false, Prerelease: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("encoding releases: %v", err)
		}
	}))
	defer srv.Close()

	client := NewGitHubClient(WithBaseURL(srv.URL))
	got, err := client.ListReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Semver descending: v10.0.0, v2.0.0, v1.10.0, v1.9.0, v0.1.0
	wantOrder := []string{"v10.0.0", "v2.0.0", "v1.10.0", "v1.9.0", "v0.1.0"}
	if len(got) != len(wantOrder) {
		t.Fatalf("expected %d releases, got %d", len(wantOrder), len(got))
	}
	for i, want := range wantOrder {
		if got[i].TagName != want {
			t.Errorf("release[%d]: got tag %q, want %q", i, got[i].TagName, want)
		}
	}
}

func TestSortReleasesBySemverDesc_DuplicateTags(t *testing.T) {
	t.Parallel()

	// Two releases with the same tag — the old map-based sort would lose one.
	// SortStableFunc preserves both and maintains original relative order.
	releases := []Release{
		{TagName: "v1.0.0", Name: "First"},
		{TagName: "v2.0.0", Name: "Latest"},
		{TagName: "v1.0.0", Name: "Second"},
	}

	sortReleasesBySemverDesc(releases)

	if len(releases) != 3 {
		t.Fatalf("expected 3 releases after sort, got %d", len(releases))
	}

	// v2.0.0 first, then both v1.0.0 entries preserved in original relative order.
	if releases[0].TagName != "v2.0.0" {
		t.Errorf("release[0]: got tag %q, want %q", releases[0].TagName, "v2.0.0")
	}
	if releases[1].TagName != "v1.0.0" || releases[1].Name != "First" {
		t.Errorf("release[1]: got tag=%q name=%q, want tag=%q name=%q",
			releases[1].TagName, releases[1].Name, "v1.0.0", "First")
	}
	if releases[2].TagName != "v1.0.0" || releases[2].Name != "Second" {
		t.Errorf("release[2]: got tag=%q name=%q, want tag=%q name=%q",
			releases[2].TagName, releases[2].Name, "v1.0.0", "Second")
	}
}

func TestIsGitHubHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reqURL  string
		baseURL string
		want    bool
	}{
		{
			name:    "API host matches base URL",
			reqURL:  "https://api.github.com/repos/invowk/invowk/releases",
			baseURL: "https://api.github.com",
			want:    true,
		},
		{
			name:    "github.com trusted when base is api.github.com",
			reqURL:  "https://github.com/invowk/invowk/releases/download/v1.0.0/archive.tar.gz",
			baseURL: "https://api.github.com",
			want:    true,
		},
		{
			name:    "third-party CDN rejected",
			reqURL:  "https://cdn.example.com/releases/archive.tar.gz",
			baseURL: "https://api.github.com",
			want:    false,
		},
		{
			name:    "test server matches own base URL",
			reqURL:  "http://127.0.0.1:8080/repos/invowk/invowk/releases",
			baseURL: "http://127.0.0.1:8080",
			want:    true,
		},
		{
			name:    "test server rejects different host",
			reqURL:  "http://evil.example.com/steal-token",
			baseURL: "http://127.0.0.1:8080",
			want:    false,
		},
		{
			name:    "github.com NOT trusted when base is custom GHE",
			reqURL:  "https://github.com/something",
			baseURL: "https://github.mycompany.com/api/v3",
			want:    false,
		},
		{
			name:    "case-insensitive host matching",
			reqURL:  "https://API.GITHUB.COM/repos",
			baseURL: "https://api.github.com",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := parseTestURL(tt.reqURL)
			if err != nil {
				t.Fatalf("parsing test URL %q: %v", tt.reqURL, err)
			}

			got := isGitHubHost(u, tt.baseURL)
			if got != tt.want {
				t.Errorf("isGitHubHost(%q, %q) = %v, want %v", tt.reqURL, tt.baseURL, got, tt.want)
			}
		})
	}
}

// parseTestURL is a test helper that parses a URL string. We use a helper
// instead of url.Parse directly so test cases remain declarative strings.
func parseTestURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL %q: %w", rawURL, err)
	}
	return u, nil
}
