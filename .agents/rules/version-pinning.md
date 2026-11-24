---
paths:
  - ".github/**"
  - "go.mod"
  - "**/go.mod"
  - "Makefile"
  - "scripts/**"
  - "Dockerfile"
  - "**/Dockerfile"
  - "**/Containerfile"
  - "build/**"
  - ".mcp.json"
  - "formal/manifest.toml"
  - ".pre-commit-config.yaml"
  - ".goreleaser.yaml"
  - "website/package.json"
  - "openspec/config.yaml"
  - ".agents/skills/openspec-*/**"
---

# Version Pinning

## Policy

All external dependencies, tools, and images MUST use pinned versions. The use of `latest`,
unpinned tags, or unversioned install scripts is prohibited in CI workflows, Dockerfiles, and
build configuration. The exceptions below are deliberate. User-facing install instructions
(`go install github.com/invowk/invowk@latest` in `README.md`, website docs and
`scripts/install.sh`) are intentional and out of scope.

## Rules by Category

### Go Dependencies
- Pinned automatically via `go.mod` + `go.sum`.
- CUE library upgrades follow the 6-step process in `.agents/skills/cue/SKILL.md` ("Upgrade Process").

### Go Tool Dependencies
- Pinned through `go.mod` `tool` directives plus exact `require` versions.
- Add or upgrade with `go get -tool <module>/cmd/<tool>@vX.Y.Z` from the root module; remove with `go get -tool <module>/cmd/<tool>@none`.
- Verify with `go version -m "$(go tool -n <tool>)"` when the tool has no reliable `--version` flag.
- **Current pinned versions:**
  - `go-mutesting`: `v2.8.3` (`github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting`, also pinned and verified by `scripts/mutation.sh`; update both together, and re-check that `mutant_id` in `scripts/formal_mutation.py` still matches its `internal/baseline` `MutantID`, or `triage --check` fails on every anchored survivor)
  - `golangci-lint`: `v2.13.1` (`github.com/golangci/golangci-lint/v2/cmd/golangci-lint`, resolved and verified by `scripts/golangci-lint.sh`)
  - `goplint`: `v0.2.0` (`github.com/invowk/goplint` plus its `cmd/repository-audit` and `cmd/benchmark-policy` commands, built and version-verified by `scripts/goplint.sh`; Dependabot bumps the pin through the root `gomod` ecosystem)

### Go Toolchain
- Version source: the `go` directive in the root `go.mod` (no `toolchain` directive). CI follows via `go-version-file:`. Check that a compatible goplint release exists (or is cut) when bumping Go here.
- On a Go minor-version bump, ALL of these must move together:
  - `build/bencher/Dockerfile` base image (`GOTOOLCHAIN=local` — hard-fails if lagging)
  - `default.pgo` (regenerate)
  - `golang.org/x/tools`, golangci-lint, go-mutesting, and the goplint pin (releases built for the new Go version)
  - language-image examples such as `golang:1.N` in docs and tests
- Check that the `github.com/invowk/golua` fork builds on the new Go version before bumping.

### CI Tool Installs (`go install`, `curl | sh`, etc.)
- MUST pin to an exact version: `go install tool@vX.Y.Z` (never `@latest`).
- MUST verify after install: `tool --version` or equivalent.
- When upgrading, update the version in ALL workflow files that reference it.
- **Current pinned versions:**
  - `gotestsum`: `v1.13.0` (`.github/workflows/ci.yml`, `.github/workflows/release.yml`)
  - `govulncheck`: `v1.7.0` (`.github/workflows/ci.yml`)
  - `cosign`: `v3.1.3` (`cosign-release` input in `.github/workflows/release-signing.yml` and `.github/workflows/release.yml`)
  - UPX: `5.2.0` (`UPX_VERSION` in `.github/workflows/ci.yml`, `.github/workflows/release-signing.yml` and `.github/workflows/release.yml`)
  - D2: `v0.7.1` (`.github/workflows/validate-diagrams.yml`)

### Formal-Verification Tools (`formal/manifest.toml`)
- Jars are downloaded by `scripts/formal.py` into `bin/formal/` and SHA-256 verified before every use; the manifest records version, URL, and checksum.
- Never pin prerelease builds.
- `pgregory.net/rapid` is a test-only Go dependency; `go.mod` is its single source of truth.
- **Current pinned versions:**
  - Alloy Analyzer: `6.2.0` (`org.alloytools.alloy.dist.jar`)
  - TLA+ tools: `1.7.4` (`tla2tools.jar`)
  - JDK: Temurin `jdk-25.0.4.1+1` in CI (written as its semver `25.0.4+101.0.LTS`) (`FORMAL_JAVA_VERSION` in `.github/workflows/formal-verification.yml`, installed with `actions/setup-java@v6` and checked with `java -version`); any Java 25 locally

### MCP Servers (`.mcp.json`)
- MUST pin to an exact version: `@upstash/context7-mcp@X.Y.Z` (never `@latest`).
- **Current pinned versions:**
  - `@upstash/context7-mcp`: `4.0.3`
  - `@modelcontextprotocol/server-github`: `2025.4.8`

### GitHub Actions
- Pin to major version tags (e.g., `@v6`). Dependabot manages minor/patch bumps weekly.
- All workflows MUST use the same major version for a given action (no stale pins like `@v4` when others use `@v6`).
- **Exception**: `sigstore/cosign-installer` is pinned to `@v4.1.2` (exact version) because the floating `@v4` major tag has not been published yet. Switch to `@v4` when available.
- **Exception**: `bencherdev/bencher` is pinned to the exact release tag `@v0.6.12` with CLI `version: 0.6.12` because upstream has no stable major tag for the action.

### Container Images
- NEVER use Alpine or Windows container images.
- `debian:stable-slim` is a rolling tag on purpose (automatic security patches). Document this exception where the image is referenced in production or CI files.
- `build/bencher/Dockerfile` uses `golang:1.27-bookworm` and `node:24-bookworm-slim`: a documented language-image exception for the benchmark image.
- Language images use stable tags, never `latest`.
- The image policy for docs, tests, CUE snippets and examples (`debian:stable-slim` only; no Alpine, Windows or `ubuntu:*` images) lives in `AGENTS.md` "Hard invariants".

### GoReleaser
- Pin via semver range in the goreleaser-action `version` input (current track: `~> v2.16`).

### OpenSpec CLI
- `@fission-ai/openspec` is a local developer tool (no CI use). Current version: `2.0.0`, recorded as `generatedBy` in `.agents/skills/openspec-*/SKILL.md`.
- Upgrade with `npm install -g @fission-ai/openspec@X.Y.Z`, then `openspec update --dry-run` and `openspec update` in the repo, and commit the regenerated skills and `.agents/skills/.openspec-target`. Never hand-edit `openspec-*` skills: `openspec update` overwrites them. Customise through `openspec/config.yaml`.

### npm Dependencies (website)
- Pinned via `package-lock.json`. Core Docusaurus packages use exact versions; others may use caret/tilde ranges (the lockfile provides determinism).

## When Upgrading Tool Versions
1. Search all workflow files, wrapper scripts, `go.mod`, `.pre-commit-config.yaml`, agent rules, and documentation (`docs/`, `website/`) for the tool name to find every reference. `.agents/skills/ci-update/SKILL.md` ("Required Sync Pairs") lists the known pairs.
2. Update the version source and every enforcing wrapper/check simultaneously.
3. Update the "Current pinned versions" list in this rule.
4. Update the prerequisites line in `.agents/rules/commands.md` only if a tool is added or removed.
5. Run affected CI workflows to verify.
