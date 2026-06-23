# Version Pinning

## Policy

All external dependencies, tools, and images MUST use pinned versions. The use of `latest`,
unpinned tags, or unversioned install scripts is prohibited in CI workflows, Dockerfiles, and
build configuration.

## Rules by Category

### Go Dependencies
- Pinned automatically via `go.mod` + `go.sum` (exact versions enforced by Go toolchain).
- CUE library has a documented 6-step upgrade process (see `.agents/skills/cue/SKILL.md`).

### Go Tool Dependencies
- Pinned through `go.mod` `tool` directives plus exact `require` versions.
- Add or upgrade tools with `go get -tool <module>/cmd/<tool>@vX.Y.Z` from the root module.
- Remove tools with `go get -tool <module>/cmd/<tool>@none`.
- Verify tools with `go version -m "$(go tool -n <tool>)"` when the tool does not provide a reliable `--version` flag.
- **Current pinned versions:**
  - `go-mutesting`: `v2.7.5` (`github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting`)
  - `golangci-lint`: `v2.12.2` (`github.com/golangci/golangci-lint/v2/cmd/golangci-lint`, resolved and verified by `scripts/golangci-lint.sh`)

### CI Tool Installs (`go install`, `curl | sh`, etc.)
- MUST pin to an exact version: `go install tool@vX.Y.Z` (never `@latest`).
- MUST verify after install: `tool --version` or equivalent.
- When upgrading, update the version in ALL workflow files that reference it.
- **Current pinned versions:**
  - `gotestsum`: `v1.13.0`
  - `govulncheck`: `v1.4.0`
  - `cosign`: `v3.1.1` (via `cosign-release` input in `.github/workflows/ci.yml` and `.github/workflows/release.yml`)
  - UPX: `5.2.0`
  - D2: `v0.7.1`

### MCP Servers (`.mcp.json`)
- MUST pin to an exact version: `@upstash/context7-mcp@X.Y.Z` (never `@latest`).
- **Current pinned versions:**
  - `@upstash/context7-mcp`: `3.2.2`
  - `@modelcontextprotocol/server-github`: `2025.4.8`

### GitHub Actions
- Pin to major version tags (e.g., `@v6`). Dependabot manages minor/patch bumps weekly.
- All workflows MUST use the same major version for a given action (no stale pins like `@v4`
  when others use `@v6`).
- **Exception**: `sigstore/cosign-installer` is pinned to `@v4.1.2` (exact version) because
  the floating `@v4` major tag has not been published yet. Switch to `@v4` when available.
- **Exception**: `bencherdev/bencher` is pinned to the exact release tag `@v0.6.8`
  with CLI `version: 0.6.8` because upstream has no stable major tag for the action.

### Container Images
- Production/CI base images: `debian:stable-slim` (rolling tag — intentional exception for
  automatic security patches). Document this exception where the image is referenced.
- **`debian:stable-slim` is the ONLY Debian/base image allowed in ALL documentation examples,
  CUE snippets, and tests.** No `ubuntu:*`, no `debian:bookworm`, no other base images.
  Language-specific images (e.g., `golang:1.26`, `python:3-slim`, `node:22-slim`) are allowed
  when demonstrating language-specific runtimes, but must use stable tags (never `latest`).
- NEVER use Alpine or Windows container images (see `AGENTS.md` "Container Runtime Limitations").

### GoReleaser
- Pin via semver range in the goreleaser-action `version` input (current track: `~> v2.16`).

### npm Dependencies (website)
- Pinned via `package-lock.json`. Core Docusaurus packages use exact versions;
  others may use caret/tilde ranges (lockfile provides determinism).

## When Upgrading Tool Versions
1. Search all workflow files, wrapper scripts, `go.mod`, `.pre-commit-config.yaml`, and agent rules for the tool name to find every reference.
2. Update the version source and every enforcing wrapper/check simultaneously.
3. Update the "Current pinned versions" list in this rule.
4. Update `.agents/rules/commands.md` if the tool appears in Prerequisites or examples.
5. Run affected CI workflows to verify.
