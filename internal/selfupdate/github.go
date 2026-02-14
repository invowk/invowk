// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// defaultPerPage is the number of releases fetched per API page.
	defaultPerPage = 30

	// maxPages is the upper bound on pagination to avoid runaway requests.
	maxPages = 3

	// maxJSONResponseBytes is the upper bound on JSON API response size (10 MB).
	// Prevents unbounded memory consumption from malicious or malformed responses.
	maxJSONResponseBytes = 10 << 20
)

// ErrReleaseNotFound is returned when a requested release tag does not exist.
var ErrReleaseNotFound = errors.New("release not found")

type (
	// RateLimitError is returned when the GitHub API rate limit is exceeded.
	RateLimitError struct {
		Limit     int
		Remaining int
		ResetAt   time.Time
	}

	// Release represents a GitHub Release with its assets.
	Release struct {
		TagName    string  // Semantic version tag, e.g., "v1.0.0"
		Name       string  // Human-readable release name
		Prerelease bool    // True for alpha/beta/RC releases
		Draft      bool    // True for unpublished drafts
		Assets     []Asset // Downloadable artifacts
		HTMLURL    string  // Browser URL for the release page
		CreatedAt  string  // ISO 8601 timestamp
	}

	// Asset represents a single downloadable file in a GitHub Release.
	Asset struct {
		Name               string // Filename, e.g., "invowk_1.0.0_linux_amd64.tar.gz"
		BrowserDownloadURL string // Direct download URL
		Size               int64  // File size in bytes
		ContentType        string // MIME type
	}

	// githubRelease is the JSON wire format for a GitHub Release API response.
	githubRelease struct {
		TagName    string        `json:"tag_name"`
		Name       string        `json:"name"`
		Prerelease bool          `json:"prerelease"`
		Draft      bool          `json:"draft"`
		HTMLURL    string        `json:"html_url"`
		CreatedAt  string        `json:"created_at"`
		Assets     []githubAsset `json:"assets"`
	}

	// githubAsset is the JSON wire format for a GitHub Release asset.
	githubAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
		ContentType        string `json:"content_type"`
	}

	// GitHubClient queries the GitHub Releases API for version information and asset downloads.
	GitHubClient struct {
		httpClient *http.Client
		owner      string // Repository owner (default: "invowk")
		repo       string // Repository name (default: "invowk")
		baseURL    string // API base URL (default: "https://api.github.com", overridable for tests)
		token      string // Optional GITHUB_TOKEN for authenticated requests
		userAgent  string // User-Agent header value
	}

	// ClientOption configures a GitHubClient during construction.
	ClientOption func(*GitHubClient)
)

// Error formats the rate limit details as a human-readable message.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit exceeded (%d remaining, resets at %s)",
		e.Remaining, e.ResetAt.UTC().Format("15:04 UTC"))
}

// WithHTTPClient sets a custom HTTP client, useful for tests or proxy configurations.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(g *GitHubClient) {
		g.httpClient = c
	}
}

// WithBaseURL overrides the GitHub API base URL, primarily for test servers.
func WithBaseURL(base string) ClientOption {
	return func(g *GitHubClient) {
		g.baseURL = strings.TrimRight(base, "/")
	}
}

// WithToken sets a GitHub personal access token for authenticated requests.
// Authenticated requests have a higher rate limit (5000/hour vs 60/hour).
func WithToken(token string) ClientOption {
	return func(g *GitHubClient) {
		g.token = token
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) ClientOption {
	return func(g *GitHubClient) {
		g.userAgent = ua
	}
}

// WithRepo overrides the default repository owner and name.
func WithRepo(owner, repo string) ClientOption {
	return func(g *GitHubClient) {
		g.owner = owner
		g.repo = repo
	}
}

// NewGitHubClient creates a GitHubClient with sensible defaults.
// Defaults: owner="invowk", repo="invowk", baseURL="https://api.github.com",
// userAgent="invowk/dev", httpClient=http.DefaultClient.
func NewGitHubClient(opts ...ClientOption) *GitHubClient {
	c := &GitHubClient{
		httpClient: http.DefaultClient,
		owner:      "invowk",
		repo:       "invowk",
		baseURL:    "https://api.github.com",
		userAgent:  "invowk/dev",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ListReleases fetches stable (non-draft, non-prerelease) releases, sorted by
// semantic version in descending order. Pagination is followed up to maxPages.
func (c *GitHubClient) ListReleases(ctx context.Context) ([]Release, error) {
	pageURL := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d",
		c.baseURL, c.owner, c.repo, defaultPerPage)

	var all []Release

	for page := 0; page < maxPages && pageURL != ""; page++ {
		resp, reqErr := c.doRequest(ctx, http.MethodGet, pageURL)
		if reqErr != nil {
			return nil, fmt.Errorf("listing releases: %w", reqErr)
		}

		if rlErr := checkRateLimit(resp); rlErr != nil {
			resp.Body.Close()
			return nil, rlErr
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("listing releases: unexpected status %d", resp.StatusCode)
		}

		releases, parseErr := parseReleases(io.LimitReader(resp.Body, maxJSONResponseBytes))
		resp.Body.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("listing releases: %w", parseErr)
		}

		// Filter client-side: keep only stable releases (non-draft, non-prerelease).
		for i := range releases {
			if !releases[i].Draft && !releases[i].Prerelease {
				all = append(all, releases[i])
			}
		}

		pageURL = parseLinkHeader(resp.Header.Get("Link"))
	}

	// Sort by semantic version descending. Releases without valid semver tags
	// are sorted to the end.
	sortReleasesBySemverDesc(all)

	return all, nil
}

// GetReleaseByTag fetches a single release by its Git tag (e.g., "v1.0.0").
// Returns ErrReleaseNotFound if the tag does not correspond to a release.
func (c *GitHubClient) GetReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	tagURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s",
		c.baseURL, c.owner, c.repo, tag)

	resp, err := c.doRequest(ctx, http.MethodGet, tagURL)
	if err != nil {
		return nil, fmt.Errorf("getting release %s: %w", tag, err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body

	if err := checkRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrReleaseNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting release %s: unexpected status %d", tag, resp.StatusCode)
	}

	var gr githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONResponseBytes)).Decode(&gr); err != nil {
		return nil, fmt.Errorf("getting release %s: decoding response: %w", tag, err)
	}

	r := toRelease(gr)
	return &r, nil
}

// DownloadAsset downloads the file at the given URL and returns the response body
// as a streaming reader. The caller is responsible for closing the returned ReadCloser.
func (c *GitHubClient) DownloadAsset(ctx context.Context, assetURL string) (io.ReadCloser, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, assetURL)
	if err != nil {
		return nil, fmt.Errorf("downloading asset %s: %w", redactURL(assetURL), err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("downloading asset %s: unexpected status %d", redactURL(assetURL), resp.StatusCode)
	}

	return resp.Body, nil
}

// doRequest creates and executes an HTTP request with common GitHub API headers.
func (c *GitHubClient) doRequest(ctx context.Context, method, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.userAgent)

	// Only attach the auth token when the request targets a known GitHub host.
	// This prevents token leakage if a download URL redirects to a third-party CDN.
	if c.token != "" && isGitHubHost(req.URL, c.baseURL) {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

// checkRateLimit inspects the X-RateLimit-* response headers and returns a
// RateLimitError when the remaining quota is zero. It does not inspect the
// HTTP status code — only the header values are examined.
func checkRateLimit(resp *http.Response) error {
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "" {
		// No rate limit headers present; nothing to check.
		return nil
	}

	rem, err := strconv.Atoi(remaining)
	if err != nil {
		// Malformed header value; skip rate limit check.
		return nil //nolint:nilerr // Non-numeric header is non-fatal.
	}

	// Only treat it as a rate limit error when remaining is zero.
	if rem > 0 {
		return nil
	}

	// Parse companion headers for a richer error message.
	// Errors are intentionally ignored — malformed or missing values default to zero,
	// which is acceptable for a diagnostic error message.
	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))                 //nolint:errcheck // Best-effort header parsing.
	resetUnix, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64) //nolint:errcheck // Best-effort header parsing.
	resetAt := time.Unix(resetUnix, 0)

	return &RateLimitError{
		Limit:     limit,
		Remaining: 0,
		ResetAt:   resetAt,
	}
}

// parseReleases decodes a JSON array of GitHub releases from the response body.
func parseReleases(body io.Reader) ([]Release, error) {
	var raw []githubRelease
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}

	releases := make([]Release, 0, len(raw))
	for _, gr := range raw {
		releases = append(releases, toRelease(gr))
	}
	return releases, nil
}

// parseLinkHeader extracts the URL for the "next" page from a GitHub API Link header.
// Returns an empty string if no next page exists.
//
// Example header: <https://api.github.com/...?page=2>; rel="next", <...>; rel="last"
func parseLinkHeader(header string) string {
	if header == "" {
		return ""
	}

	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}

		// Extract URL between < and >
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}

	return ""
}

// toRelease converts the internal JSON wire type to the exported Release type.
// Asset fields are identical between githubAsset and Asset (ignoring struct tags),
// so Go permits direct type conversion.
func toRelease(gr githubRelease) Release {
	assets := make([]Asset, 0, len(gr.Assets))
	for _, ga := range gr.Assets {
		assets = append(assets, Asset(ga))
	}

	return Release{
		TagName:    gr.TagName,
		Name:       gr.Name,
		Prerelease: gr.Prerelease,
		Draft:      gr.Draft,
		Assets:     assets,
		HTMLURL:    gr.HTMLURL,
		CreatedAt:  gr.CreatedAt,
	}
}

// sortReleasesBySemverDesc sorts releases by semantic version in descending order.
// Releases with invalid semver tags are placed at the end. Uses a stable sort
// so releases with identical tags preserve their original ordering.
func sortReleasesBySemverDesc(releases []Release) {
	slices.SortStableFunc(releases, func(a, b Release) int {
		return semver.Compare(b.TagName, a.TagName)
	})
}

// isGitHubHost reports whether reqURL targets a known GitHub host, so the auth
// token can be safely attached. It matches the configured API base URL host and,
// when the base is api.github.com, also trusts github.com for asset downloads.
func isGitHubHost(reqURL *url.URL, baseURL string) bool {
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	// Match the configured API host (covers both production and test servers).
	if strings.EqualFold(reqURL.Host, base.Host) {
		return true
	}
	// When the API base is api.github.com, also trust github.com for asset downloads.
	if strings.EqualFold(base.Host, "api.github.com") && strings.EqualFold(reqURL.Host, "github.com") {
		return true
	}
	return false
}

// redactURL strips query parameters and fragments from a URL for safe inclusion
// in error messages, preventing accidental exposure of tokens or sensitive data.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid-url>"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
