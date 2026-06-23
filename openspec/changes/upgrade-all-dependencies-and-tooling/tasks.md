## 1. Inventory Refresh

- [x] 1.1 Re-run `openspec status --change upgrade-all-dependencies-and-tooling --json` and confirm the proposal, design, and specs are the active implementation source.
- [x] 1.2 Record the starting worktree state and preserve unrelated dirty files from the existing `update-go-mutesting-output-labels` work.
- [x] 1.3 Re-run Go module update inventory for `.` and `tools/goplint` with `go list -m -u -retracted -json all`.
- [x] 1.4 Re-run deprecated/retracted-module inventory for both Go modules and capture deferred findings.
- [x] 1.5 Re-run `go mod tidy -diff` in both Go modules and confirm no pre-existing tidy drift.
- [x] 1.6 Re-run `make vulncheck` and capture the all-module vulnerability baseline.
- [x] 1.7 Re-run website `npm --prefix website outdated --json`, `npm --prefix website audit --omit=optional --json`, and `npm audit fix --dry-run` evidence.
- [x] 1.8 Re-run CI/tooling inventory for workflow `uses:` pins, `go install` tools, MCP server versions, UPX, Cosign, GoReleaser track, Node.js LTS status, and Bencher policy exceptions.

## 2. Website Dependency Security Refresh

- [x] 2.1 Choose a website update strategy that prioritizes lockfile/transitive fixes and rejects unsafe direct package downgrades or unreviewed major replacements.
- [x] 2.2 Refresh fixable website transitive dependencies in `website/package-lock.json`.
- [x] 2.3 Update `website/package.json` only if a compatible direct dependency change is necessary and explicitly justified.
- [x] 2.4 Run `npm --prefix website ci` from the refreshed lockfile.
- [x] 2.5 Run `npm --prefix website run typecheck`.
- [x] 2.6 Run `npm --prefix website run build`.
- [x] 2.7 Re-run `npm --prefix website audit --omit=optional --json` and document fixed and remaining advisories.

## 3. Go Dependency Refresh

- [x] 3.1 Update root direct Go dependencies selected by the refreshed inventory, including `github.com/openai/openai-go/v3`, `charm.land/lipgloss/v2`, and `github.com/sahilm/fuzzy` when still current.
- [x] 3.2 Update `tools/goplint` direct Go dependencies selected by the refreshed inventory, including `golang.org/x/tools` when still current.
- [x] 3.3 Accept transitive Go changes selected by the Go toolchain for the explicit direct/tool updates.
- [x] 3.4 Avoid broad `go get -u all` unless implementation records the justification and affected verification surface.
- [x] 3.5 Run root `go mod tidy` and nested `tools/goplint` `go mod tidy`.
- [x] 3.6 Confirm `go mod tidy -diff` is clean in both Go modules.
- [x] 3.7 Run targeted LLM/audit tests to prove `openai-go/v3` still uses the existing Chat Completions and model-listing contract.
- [x] 3.8 Run targeted TUI/fuzzy tests covering Lip Gloss and fuzzy matching surfaces.
- [x] 3.9 Run targeted `tools/goplint` analyzer tests and semantic gates affected by `golang.org/x/tools`.

## 4. Mutation Tool Refresh

- [x] 4.1 Re-check the latest `github.com/jonbaldie/go-mutesting/v2` version and select the update target.
- [x] 4.2 Update the root Go tool dependency for `github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting`.
- [x] 4.3 Update `scripts/mutation.sh`, `scripts/test_mutation.sh`, `.agents/rules/version-pinning.md`, `.agents/rules/commands.md`, and mutation docs for the selected tool version where needed.
- [x] 4.4 Preserve mutation baseline files and survivor counts unless a separate intentional baseline-update step is added.
- [x] 4.5 Verify the resolved `go-mutesting` binary version with `go version -m "$(go tool -n go-mutesting)"`.
- [x] 4.6 Run `bash scripts/test_mutation.sh`.
- [x] 4.7 Run a lightweight mutation dry-run or focused command path that proves changed-line/base-ref behavior without a full mutation scan.

## 5. CI, MCP, and Release Tooling Refresh

- [x] 5.1 Update `govulncheck` workflow installs and documented pins to the selected current version.
- [x] 5.2 Update MCP server pins in `.mcp.json`, including Context7, and update `.agents/rules/version-pinning.md` if the documented pin changes.
- [x] 5.3 Update GitHub Actions major pins that policy allows, keeping shared action majors consistent across workflows.
- [x] 5.4 Review `bencherdev/bencher@main` as a policy exception or pinning risk and document the decision.
- [x] 5.5 Update Cosign installer/action and `cosign-release` pins together in CI and release workflows.
- [x] 5.6 Update UPX inline install versions together in CI and release workflows.
- [x] 5.7 Review the GoReleaser v2 track and update only if a tagged compatible track is available and validated.
- [x] 5.8 Keep Node.js workflow pins on the approved active LTS major unless a policy-approved migration is documented.
- [x] 5.9 Update cache keys, wrapper version checks, `.agents/rules/version-pinning.md`, and `.agents/rules/commands.md` for every changed tooling pin.

## 6. Validation

- [x] 6.1 Run `make vulncheck`.
- [x] 6.2 Run `make test`.
- [x] 6.3 Run `make lint`.
- [x] 6.4 Run `make check-agent-docs` because agent rules or guidance are expected to change.
- [x] 6.5 Run focused goplint gates affected by `golang.org/x/tools`, including semantic spec, IFDS compatibility, refinement, alias, and baseline checks as applicable.
- [x] 6.6 Run GoReleaser configuration checks with the selected release tooling track.
- [x] 6.7 Run a snapshot release dry run that covers checksums, signing setup, compression, Homebrew metadata, WinGet metadata, and release notes handling.
- [x] 6.8 Run relevant script tests for changed workflow/tool wrappers.
- [x] 6.9 If changes are pushed, verify remote CI on the exact pushed SHA and record final status.

## 7. Review and Closeout

- [x] 7.1 Review `git diff` by phase and confirm unrelated dirty files were not modified.
- [x] 7.2 Record the final upgraded versions, unchanged policy pins, and version tracks in an implementation summary.
- [x] 7.3 Record remaining npm advisories, deprecated Go modules, branch-pinned actions, or upstream-blocked findings with follow-up recommendations.
- [x] 7.4 Confirm no user-facing CLI, CUE schema, runtime, module, audit, TUI, or website-content behavior changed unintentionally.
- [x] 7.5 Confirm mutation baselines and survivor counts were not recomputed during routine tool refresh.
- [x] 7.6 Run `openspec validate upgrade-all-dependencies-and-tooling --strict`.
- [x] 7.7 Run `openspec status --change upgrade-all-dependencies-and-tooling` and confirm the change is ready for implementation completion.
