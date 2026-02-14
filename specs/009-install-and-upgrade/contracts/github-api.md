# Contract: GitHub Releases API Interaction

**Phase 1 Output** | **Date**: 2026-02-13

## Overview

The self-upgrade command and install script interact with the GitHub Releases API to discover versions, download assets, and verify integrity. This document defines the exact API interactions.

## API Base URL

```
https://api.github.com
```

## Authentication

- **Default**: Unauthenticated (60 req/hour rate limit)
- **Authenticated**: Via `GITHUB_TOKEN` env var (5,000 req/hour)
- **Header**: `Authorization: Bearer {token}` (when token is set)

## Common Headers (all requests)

```
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
User-Agent: invowk/{version}
```

## Endpoints

### 1. List Releases

**Purpose**: Find the latest stable release or check for available upgrades.

```
GET /repos/invowk/invowk/releases?per_page=30&page=1
```

**Response** (simplified — only fields we parse):

```json
[
  {
    "tag_name": "v1.1.0",
    "name": "v1.1.0",
    "draft": false,
    "prerelease": false,
    "html_url": "https://github.com/invowk/invowk/releases/tag/v1.1.0",
    "created_at": "2026-02-10T12:00:00Z",
    "assets": [
      {
        "name": "invowk_1.1.0_linux_amd64.tar.gz",
        "browser_download_url": "https://github.com/invowk/invowk/releases/download/v1.1.0/invowk_1.1.0_linux_amd64.tar.gz",
        "size": 5242880,
        "content_type": "application/gzip"
      },
      {
        "name": "checksums.txt",
        "browser_download_url": "https://github.com/invowk/invowk/releases/download/v1.1.0/checksums.txt",
        "size": 1024,
        "content_type": "text/plain"
      }
    ]
  }
]
```

**Filtering logic (client-side)**:
1. Skip entries where `draft == true` or `prerelease == true`
2. Parse `tag_name` with `semver.IsValid()`
3. Sort by semver descending
4. First entry = latest stable

**Pagination**: Follow `Link` header for `rel="next"` if first page has no stable release. In practice, the first page (30 releases) almost always contains the latest stable.

### 2. Get Release by Tag

**Purpose**: Fetch a specific release when user targets a version.

```
GET /repos/invowk/invowk/releases/tags/v1.2.0
```

**Response**: Same shape as a single element of the list response.

**Error responses**:
- `404 Not Found`: Version does not exist
- `403 Forbidden`: Rate limited

### 3. Download Asset

**Purpose**: Download the binary archive for the current platform.

```
GET {asset.browser_download_url}
```

This is a direct download URL from `github.com` (not `api.github.com`). It returns the binary content directly (no JSON wrapping).

**Asset selection logic**:
```
os   = runtime.GOOS    # "linux", "darwin", "windows"
arch = runtime.GOARCH   # "amd64", "arm64"
ext  = "tar.gz"         # ".zip" for windows

assetName = fmt.Sprintf("invowk_%s_%s_%s.%s", version, os, arch, ext)
```

Where `version` is the tag without the `v` prefix (GoReleaser convention: `invowk_1.1.0_linux_amd64.tar.gz`, not `invowk_v1.1.0_...`).

### 4. Download Checksums

**Purpose**: Verify integrity of downloaded asset.

```
GET {checksums_asset.browser_download_url}
```

**Response** (text/plain):

```
a1b2c3d4e5f6...  invowk_1.1.0_linux_amd64.tar.gz
f7g8h9i0j1k2...  invowk_1.1.0_darwin_amd64.tar.gz
m3n4o5p6q7r8...  invowk_1.1.0_darwin_arm64.tar.gz
...
```

**Format**: `{sha256_hex}  {filename}` (two spaces between hash and filename — standard `sha256sum` output format).

**Verification logic**:
1. Download `checksums.txt`
2. Find line matching the asset filename
3. Compare SHA256 of downloaded archive with the hash from `checksums.txt`
4. If no matching line found → error (asset not in checksums)
5. If hash mismatch → error (corrupted download)

## Rate Limit Handling

**Response headers** (present on all API responses):

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 42
X-RateLimit-Reset: 1707836400
```

**Handling strategy**:
1. If `X-RateLimit-Remaining` is `0`, extract reset time from `X-RateLimit-Reset`
2. Format as human-readable time: "resets at 14:30 UTC"
3. Suggest setting `GITHUB_TOKEN` for higher limits

## Error Response Shape

```json
{
  "message": "API rate limit exceeded for ...",
  "documentation_url": "https://docs.github.com/..."
}
```

**HTTP status codes to handle**:
| Status | Meaning | Action |
|--------|---------|--------|
| 200 | Success | Parse response |
| 304 | Not Modified | Use cached response (if implementing caching) |
| 403 | Rate limited | Show rate limit message |
| 404 | Not found | Version does not exist |
| 5xx | Server error | Retry once, then fail with message |

## Install Script API Usage

The install script uses the same endpoints but via `curl`/`wget` instead of Go's `net/http`:

```sh
# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/invowk/invowk/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

# Download asset
curl -fsSL -o "$TMPDIR/invowk.tar.gz" "https://github.com/invowk/invowk/releases/download/${VERSION}/invowk_${VERSION#v}_${OS}_${ARCH}.tar.gz"

# Download checksums
curl -fsSL -o "$TMPDIR/checksums.txt" "https://github.com/invowk/invowk/releases/download/${VERSION}/checksums.txt"
```

Note: The install script uses the `/releases/latest` endpoint (simpler, always returns latest non-prerelease) while the Go upgrade command uses `/releases` (list all, for more control over filtering and pagination).
