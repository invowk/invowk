# Version Pinning

## Policy

All external dependencies, tools, and images MUST use pinned versions. The use of `latest`,
unpinned tags, or unversioned install scripts is prohibited in CI workflows, Dockerfiles, and
build configuration.

## Rules by Category

### Go Dependencies
- Pinned automatically via `go.mod` + `go.sum` (exact versions enforced by Go toolchain).
- CUE library has a documented 6-step upgrade process (see `.claude/skills/cue/SKILL.md`).

### CI Tool Installs (`go install`, `curl | sh`, etc.)
- MUST pin to an exact version: `go install tool@vX.Y.Z` (never `@latest`).
- MUST verify after install: `tool --version` or equivalent.
- When upgrading, update the version in ALL workflow files that reference it.
- **Current pinned versions:**
  - `gotestsum`: `v1.13.0`
  - `golangci-lint`: `v2.9.0` (via golangci-lint-action `version` input)
  - UPX: `5.1.0`
  - D2: `v0.7.1`

### GitHub Actions
- Pin to major version tags (e.g., `@v6`). Dependabot manages minor/patch bumps weekly.
- All workflows MUST use the same major version for a given action (no stale pins like `@v4`
  when others use `@v6`).

### Container Images
- Production/CI base images: `debian:stable-slim` (rolling tag — intentional exception for
  automatic security patches). Document this exception where the image is referenced.
- **`debian:stable-slim` is the ONLY Debian/base image allowed in ALL documentation examples,
  CUE snippets, and tests.** No `ubuntu:*`, no `debian:bookworm`, no other base images.
  Language-specific images (e.g., `golang:1.26`, `python:3-slim`) are allowed when
  demonstrating language-specific runtimes, but must use stable tags (never `latest`).
- NEVER use Alpine or Windows container images (see CLAUDE.md Container Runtime Limitations).

### GoReleaser
- Pin via semver range (`~> v2`) in the goreleaser-action `version` input.

### npm Dependencies (website)
- Pinned via `package-lock.json`. Core Docusaurus packages use exact versions;
  others may use caret/tilde ranges (lockfile provides determinism).

## When Upgrading Tool Versions
1. Search all workflow files for the tool name to find every reference.
2. Update the version in all locations simultaneously.
3. Update the "Current pinned versions" list in this rule.
4. Update `.agents/rules/commands.md` if the tool appears in Prerequisites or examples.
5. Run affected CI workflows to verify.
