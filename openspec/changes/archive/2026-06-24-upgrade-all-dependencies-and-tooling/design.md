## Context

The current repo has multiple dependency surfaces with different risk profiles:

- Root Go module direct updates: `charm.land/lipgloss/v2`, `github.com/openai/openai-go/v3`, and `github.com/sahilm/fuzzy`.
- Nested `tools/goplint` module direct update: `golang.org/x/tools`.
- Website npm graph: direct dependencies are current, but `npm audit` reports transitive advisories that a lockfile refresh can partially address.
- Pinned tooling and CI infrastructure: `govulncheck`, `go-mutesting`, Cosign, UPX, Context7 MCP, selected GitHub Actions majors, and release workflow pins.
- Existing policy surfaces: `.agents/rules/version-pinning.md`, `.agents/rules/commands.md`, CI workflows, mutation testing, and release tooling maintenance specs.

The previous dependency work established two important constraints: audit first, and avoid broad unreviewable transitive churn. The user's current direction is to upgrade everything, so this design treats "everything" as every eligible dependency and tooling pin discovered by a fresh inventory, applied in controlled phases with explicit verification and documented deferrals for upstream-blocked leftovers.

## Goals / Non-Goals

**Goals:**

- Refresh all currently eligible direct Go module dependencies in the root and nested modules.
- Apply transitive Go updates only when they are required for selected direct/tool updates, security fixes, or clearly justified cleanup.
- Refresh the website npm lockfile to eliminate fixable advisories without replacing tested direct packages unnecessarily.
- Update repository-pinned tools, MCP servers, and CI action pins where current policy allows.
- Update mutation tooling beyond the current `v2.7.1` label track while preserving baseline/report behavior.
- Update release tooling pins and validation for Cosign, UPX, GoReleaser track consistency, and release dry-run confidence.
- Keep all version-pinning guidance, wrapper version checks, workflows, and docs synchronized.
- Preserve current product behavior unless an upstream dependency forces a minimal compatibility adjustment.

**Non-Goals:**

- Migrate Invowk's OpenAI-compatible LLM client from Chat Completions to Responses.
- Adopt React 19 app patterns, Docusaurus feature changes, or UI redesigns just because the packages are current.
- Move CI from Node.js 24 LTS to a non-LTS or not-yet-policy-approved Node major.
- Recompute mutation baselines or accept new mutation survivors as part of the tool-version bump.
- Remove every deprecated transitive module if upstream dependency trees still require them.
- Silence `npm audit` through unsafe overrides, direct package downgrades, or unreviewed major replacements.

## Decisions

### Upgrade as one OpenSpec change with phased implementation checkpoints

This change owns the full refresh surface, but implementation should be staged by risk and verification surface:

1. Reconfirm inventory and clean baseline.
2. Website lockfile and npm advisory refresh.
3. Root and nested Go module direct updates.
4. Mutation tooling update.
5. CI, MCP, release, and workflow pin update.
6. Full verification and deferred-finding report.

Alternative considered: create separate OpenSpec changes for each track. That remains the cleanest review shape for unrelated changes, but the user explicitly asked to upgrade everything after the exploration. Phasing inside one change keeps that intent while preserving review boundaries.

### Treat website advisories as lockfile/security maintenance first

The website direct dependencies are current. The first implementation attempt should use an npm lockfile/security refresh that moves vulnerable transitive packages inside existing semver ranges. If `npm audit fix` proposes a direct major replacement or a downgrade such as replacing the current image zoom plugin with an older incompatible release, pause and choose a targeted lockfile or override strategy with explicit evidence.

Alternative considered: accept every `npm audit fix` suggestion. That can swap or downgrade tested direct packages and would be too risky for a documentation site whose direct stack is already current.

### Keep OpenAI SDK on Chat Completions

`openai-go/v3` patch updates should be applied as SDK maintenance only. Invowk's LLM client is intentionally OpenAI-compatible and local-endpoint friendly, so it should preserve `/v1/chat/completions`, `/v1/models`, optional API key handling, structured-output gating, and fake-server tests.

Alternative considered: migrate to Responses API while updating `openai-go/v3`. That is a product behavior migration with local-provider compatibility risk, not a dependency refresh.

### Update Go dependencies explicitly instead of sweeping all transitive modules

Direct Go upgrades should be named explicitly. Transitive upgrades should be accepted when selected by the Go toolchain due to direct upgrades, tool upgrades, or security fixes. Broad `go get -u all` should be used only if the implementation records why it is needed and validates the resulting graph.

Alternative considered: run `go get -u ./...` or `go get -u all` at the start. The current graph includes many analyzer, cloud, and tooling transitives; a blanket sweep would make behavioral review noisy.

### Keep mutation reports and baselines stable across the tool update

The mutation tool update should preserve current wrapper behavior, report locations, stable mutant IDs, baseline filtering, advisory/blocking mode semantics, and terminal-label guidance. Any new changed-line behavior, such as merge-base diff selection, should improve PR targeting without recomputing accepted survivor baselines.

Alternative considered: update the tool and regenerate baselines in the same change. That would mix dependency maintenance with test-quality triage and make survivor changes hard to review.

### Update CI and release pins through sync pairs

CI and release tooling updates must follow `.agents/rules/version-pinning.md`: update every synced reference together, including workflow pins, wrapper expectations, rules, commands, and cache keys. GitHub Actions major bumps should be reviewed as action compatibility updates, while Dependabot-owned minor and patch bumps remain outside this manual change unless the major move requires them.

Alternative considered: update only the visible workflow `uses:` lines. That misses cache keys, agent docs, wrapper version checks, and repeated release workflow pins.

### Document remaining blockers instead of pretending the graph is perfect

If a deprecated transitive module, npm advisory, or policy exception remains after compatible upgrades, implementation must record it with the blocker and owner surface. Examples include upstream Docusaurus transitive advisories, branch-pinned actions such as `bencherdev/bencher@main`, or deprecated modules that only disappear after upstream migration.

Alternative considered: force overrides until audit output is empty. Overrides can create an untested dependency graph and should be used only with targeted justification and build/test evidence.

## Risks / Trade-offs

- Website lockfile refresh changes generated dependency graph while direct deps stay unchanged -> Mitigate with `npm --prefix website ci`, `npm --prefix website run typecheck`, `npm --prefix website run build`, and before/after `npm audit` evidence.
- GitHub Actions major updates can break CI syntax or runner assumptions -> Mitigate by grouping action updates by family, checking release notes for majors, and watching the exact pushed SHA.
- Cosign updates can affect signing bundle/default behavior -> Mitigate with GoReleaser check, snapshot dry run, and release signing verification in CI dry-run paths.
- UPX updates can affect binary size or decompression behavior -> Mitigate with release dry run and binary smoke checks.
- `openai-go/v3` patch updates can alter generated types or request payloads -> Mitigate with fake-server `internal/auditllm` tests and preserving Chat Completions path assertions.
- `golang.org/x/tools` updates can alter analyzer behavior -> Mitigate with full `tools/goplint` tests, semantic spec gates, and baseline checks.
- Broad upgrade scope increases review load -> Mitigate with phased commits or at least phased validation notes inside one OpenSpec change.

## Migration Plan

1. Re-run dependency and tooling inventory: Go modules, npm outdated/audit, tool versions, MCP versions, workflow action majors, Node LTS status, deprecated/retracted modules, and `go mod tidy -diff`.
2. Refresh website lockfile/security advisories first; verify website install, typecheck, build, and audit delta.
3. Refresh root and nested Go direct dependencies; tidy both modules; run focused tests for LLM, TUI, fuzzy filter, goplint analyzer, and all-module vulnerability scanning.
4. Update `go-mutesting`; verify wrapper version checks, command construction, dry-run/focused path, and docs/spec deltas without baseline regeneration.
5. Update CI/MCP/release tooling pins through sync pairs; verify agent docs, workflow syntax where possible, GoReleaser checks, release dry run, and relevant script tests.
6. Run aggregate gates appropriate for the final changed surface: `make test`, `make lint`, `make vulncheck`, `make check-agent-docs`, website build/typecheck, and targeted release/mutation checks.
7. Record final upgraded versions, remaining deferred findings, and verification results in the implementation summary.

Rollback is by phase: revert the most recent dependency/tooling phase and its generated lockfile/checksum changes together. If CI action majors fail remotely, revert the action-major phase while retaining completed package/security updates that passed local verification.

## Open Questions

None. Implementation should pause only if an upgrade requires a product behavior migration, unsafe npm override, non-LTS Node major, mutation baseline recomputation, or release signing model change beyond routine pin refresh.
