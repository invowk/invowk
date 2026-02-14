> **Status Update (2026-02-13):** User Story 3 (Self-Upgrade via CLI) has been removed
> from this branch. Research items R3 (atomic binary replacement), R4 (install method detection),
> and R5 (semver comparison) are archived — they informed the design but the feature was not shipped.
> This document is retained as design history. See the branch commit log for details.

# Research: Installation Methods & Self-Upgrade

**Phase 0 Output** | **Date**: 2026-02-13

## R1: Module Path Migration Strategy

**Decision**: Use `sed` for bulk import replacement, followed by `goimports` + `gofumpt` for formatting cleanup.

**Rationale**: The codebase has 217 import references to `invowk-cli/` across 111 Go files. A mechanical `sed` replacement is the safest approach for this scale — it's atomic, reviewable, and well-understood. The MEMORY.md "Large-scale sed rename safety" pattern applies directly.

**Alternatives Considered**:
- `gofmt -r` (Go's built-in rewrite tool): Cannot handle import path rewrites — it operates on AST expressions, not import strings.
- `gorename` / `gopls rename`: Designed for identifier renames within a module, not module path changes.
- Manual editing: Impractical at 111 files.

**Migration checklist**:
1. `go.mod` line 1: `module invowk-cli` → `module github.com/invowk/invowk`
2. All `.go` files: `"invowk-cli/` → `"github.com/invowk/invowk/`
3. `Makefile` ldflags: 3 `-X` entries
4. `.goreleaser.yaml` ldflags: 3 `-X` entries
5. `.claude/rules/*.md`, `.claude/agents/*.md`: Documentation references (non-breaking but should be consistent)
6. `specs/` markdown files: Documentation references

**Risk mitigation**: Run `go build ./...` immediately after sed + goimports. Compile errors mean missed references. Run `make test` for full verification.

## R2: GitHub Releases API for Version Checking

**Decision**: Use the GitHub REST API v3 (`/repos/{owner}/{repo}/releases`) with stdlib `net/http`. No third-party GitHub client library.

**Rationale**: The upgrade command needs exactly two API operations: (1) list releases to find the latest stable, and (2) download release assets. The GitHub REST API is stable, well-documented, and requires only simple HTTP GET requests. A full GitHub client library (e.g., `google/go-github`) would be overkill — it pulls in dozens of transitive dependencies for features we don't need.

**API endpoints used**:
- `GET /repos/invowk/invowk/releases` — List all releases (paginated, 30 per page)
  - Filter: exclude pre-releases (`prerelease: false`), exclude drafts (`draft: false`)
  - Sort: by `created_at` descending (default)
  - Extract: `tag_name`, `assets[].browser_download_url`, `assets[].name`
- `GET /repos/invowk/invowk/releases/tags/{tag}` — Get specific release by tag
  - Used when user specifies a target version: `invowk upgrade v1.2.0`

**Rate limits**:
- Unauthenticated: 60 requests/hour (sufficient for individual use)
- Authenticated: 5,000 requests/hour (via `GITHUB_TOKEN` env var)
- Implementation: Check `X-RateLimit-Remaining` header, provide clear error message when exhausted

**Asset naming convention** (from GoReleaser config):
- Pattern: `invowk_{Version}_{Os}_{Arch}.tar.gz` (Linux/macOS), `.zip` (Windows)
- Checksums: `checksums.txt` in release assets
- Example: `invowk_1.0.0_linux_amd64.tar.gz`

**Alternatives Considered**:
- `google/go-github`: Full-featured but heavy (~50 transitive deps). Rejected per Principle V (Simplicity).
- GraphQL API (v4): More efficient for complex queries but requires authentication. Overkill for listing releases.
- `go-selfupdate` library: Good patterns to reference but fights our detection/verification model. Used as design reference only.

## R3: Atomic Binary Replacement

**Decision**: Write-to-temp-file-then-rename pattern using `os.CreateTemp()` in the same directory as the target binary, followed by `os.Rename()`.

**Rationale**: `os.Rename()` is atomic on all major filesystems when source and target are on the same filesystem. By creating the temp file in the same directory as the target binary, we guarantee same-filesystem semantics.

**Implementation pattern**:
```
1. Resolve target path: os.Executable() → filepath.EvalSymlinks()
2. Create temp file in same directory: os.CreateTemp(targetDir, ".invowk-upgrade-*")
3. Download + verify new binary → write to temp file
4. Copy permissions from old binary to temp file: os.Chmod()
5. Rename temp to target: os.Rename(tempPath, targetPath)
6. On any error: os.Remove(tempPath) in defer
```

**Platform considerations**:
- **Linux**: Can replace a running binary (old inode stays open until process exits). No issues.
- **macOS**: Same behavior as Linux for rename. No issues.
- **Windows**: Cannot rename over a running binary. Workaround: rename current to `.old`, rename new to target, delete `.old` on next run. However, per spec, Windows upgrade is via `go install` — the upgrade command will suggest `go install` instead of direct replacement.

**Alternatives Considered**:
- Write directly to target path: Not atomic — interrupted write leaves corrupt binary.
- Copy to temp, delete old, rename temp: Race condition — deletion + rename is not atomic.
- Use `RENAME_EXCHANGE` (Linux `renameat2`): Not available on macOS, Go stdlib doesn't expose it.

## R4: Install Method Detection

**Decision**: Hybrid approach — path-based heuristics as primary, `debug.ReadBuildInfo()` as confirmation for go-install, optional build-time ldflags hint as override.

**Rationale**: No single detection method is reliable across all edge cases. Path heuristics cover 90% of cases, build info confirms go-install, and ldflags provide an escape hatch for unusual setups.

**Detection logic (in priority order)**:
1. **Build-time ldflags hint** (highest priority): If `-X installMethod=...` was set during build, use that value. This handles edge cases like custom Homebrew prefixes or non-standard GOPATH locations.
2. **Path heuristics**:
   - Homebrew: Binary path contains `/opt/homebrew/` (macOS ARM), `/usr/local/Cellar/` (macOS Intel), or `/home/linuxbrew/.linuxbrew/` (Linux)
   - Go install: Binary is in `$GOPATH/bin/` (default: `~/go/bin/`) AND `debug.ReadBuildInfo()` returns a module path matching `github.com/invowk/invowk`
   - Script install: Binary is in `~/.local/bin/` (default script install dir)
3. **Fallback**: If no heuristic matches → `Unknown` (treated same as script install — direct binary replacement is safe)

**Install method enum**:
```go
type InstallMethod int

const (
    InstallMethodUnknown  InstallMethod = iota
    InstallMethodScript                         // Shell script install (~/.local/bin)
    InstallMethodHomebrew                       // Homebrew (brew upgrade)
    InstallMethodGoInstall                      // go install (go install @latest)
)
```

**Behavior per method**:
| Method | `invowk upgrade` Behavior |
|--------|---------------------------|
| Script / Unknown | Direct binary replacement with SHA256 verification |
| Homebrew | Print: "Detected Homebrew installation. Run `brew upgrade invowk` instead." Exit 0. |
| GoInstall | Print: "Detected go install. Run `go install github.com/invowk/invowk@latest` instead." Exit 0. |

## R5: Semver Comparison for Upgrade Eligibility

**Decision**: Use `golang.org/x/mod/semver` (Go's official semver package) for version comparison.

**Rationale**: The project already uses Go modules with semver tags. `golang.org/x/mod/semver` is the official Go package for semantic versioning, maintained by the Go team, with zero transitive dependencies. It handles pre-release comparison correctly per the semver spec.

**Upgrade eligibility rules**:
1. **Latest stable**: Filter releases where `semver.Prerelease("v"+version) == ""` (no pre-release suffix)
2. **Current > latest stable**: If current version (including pre-release) is newer than latest stable → "already up to date" or "on pre-release ahead of stable" message
3. **Specific version target**: `invowk upgrade v1.2.0` — verify target is a valid release, compare with current
4. **Pre-release protection**: Never downgrade from a pre-release to an older stable (e.g., v1.1.0-alpha.1 → v1.0.0). User would need `--force` (not in initial scope per spec)

**Alternatives Considered**:
- `github.com/Masterminds/semver/v3`: More feature-rich (constraints, ranges) but those features aren't needed. Adds a dependency.
- Manual parsing: Error-prone, reinventing the wheel.

## R6: POSIX Shell Install Script Best Practices

**Decision**: Pure POSIX sh (no bashisms), wrapped in a `main()` function, with platform detection via `uname`.

**Rationale**: The install script must run on the widest possible range of Unix systems. POSIX sh is the lowest common denominator — it works on macOS (which ships zsh as default but has `/bin/sh`), all Linux distributions, and BSDs. Wrapping in `main()` prevents partial execution when the script is piped from `curl` (FR-009).

**Key patterns**:
1. **Function wrapping**: `main() { ... }; main "$@"` at the bottom of the script
2. **Platform detection**:
   ```sh
   OS=$(uname -s | tr '[:upper:]' '[:lower:]')    # linux, darwin
   ARCH=$(uname -m)                                 # x86_64, aarch64, arm64
   # Normalize: x86_64 → amd64, aarch64 → arm64
   ```
3. **Download tool detection**: Check for `curl`, fall back to `wget`
4. **Checksum verification**: `sha256sum` (Linux) or `shasum -a 256` (macOS)
5. **Temp directory**: Use `mktemp -d` for staging, clean up in trap
6. **No partial install**: Download → verify → install atomically. If any step fails, clean up and exit.

**Reference scripts studied**:
- Rust's `rustup-init.sh`: Gold standard for POSIX install scripts
- Go's `get.golang.org` installer: Simple platform detection
- Deno's `install.sh`: Modern, clean implementation

**Error handling**:
- `set -eu` (strict mode without pipefail — not POSIX)
- `trap cleanup EXIT` for temp file cleanup
- Each step checks return code and exits with clear message

## R7: GoReleaser Homebrew Integration

**Decision**: Use GoReleaser's built-in `brews` configuration (already scaffolded but commented out in `.goreleaser.yaml`).

**Rationale**: GoReleaser automates the entire Homebrew formula lifecycle — generating the Ruby formula file, computing checksums, and pushing to the tap repository. This eliminates manual formula maintenance and ensures the tap is always in sync with releases.

**Requirements**:
1. **Tap repository**: `invowk/homebrew-tap` on GitHub (public)
2. **Token**: `HOMEBREW_TAP_TOKEN` secret in main repo — a fine-grained PAT or GitHub App token with write access to the tap repo's contents
3. **GoReleaser config**: Uncomment existing `brews:` section, update if needed
4. **Formula features**: `bin.install "invowk"`, test block with `system "#{bin}/invowk", "--version"`, dependencies: none (static binary)

**Tap repository structure**:
```
homebrew-tap/
├── Formula/
│   └── invowk.rb    # Auto-generated by GoReleaser on each release
└── README.md
```

**Alternatives Considered**:
- Manual formula in this repo: Requires manual updates on each release. Rejected.
- Homebrew core: Requires significant adoption before acceptance. Future goal, not initial path.
- Custom GitHub Action for formula updates: Reinvents what GoReleaser already does. Rejected.
