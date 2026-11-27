# Commands

Run targets from the repository root. Go comes from the `go` directive in `go.mod`. Optional tools: Docker or Podman (container tests), UPX (`build-upx`), gotestsum (`make test` uses it when present), govulncheck (`vulncheck`), Node (website; CI uses the workflows' `node-version`, `website/package.json` sets `engines`), and Java for the formal targets only (JDK version: version-pinning "Formal-Verification Tools"; `scripts/formal.py fetch` downloads and verifies the jars). Tool versions live in `.agents/rules/version-pinning.md`; never install a pinned tool with `@latest`. Run go-mutesting only through the Make targets or `go tool go-mutesting`.

| Group | Targets (`make <target>` unless shown) |
|-------|----------------------------------------|
| Build | `build` (x86-64 defaults to `GOAMD64=v3`; `GOAMD64=v1` for baseline CPUs), `build-dev`, `build-upx`, `build-all`, `build-cross`, `install`, `clean` |
| Test | `test` (full suite; already runs `test-cli`), `test-short`, `test-integration`, `test-cli`, `test-cli-cover`, `test-scripts` (guarded; `test-scripts-unguarded` runs the same list in place, fingerprinted only); one test: `go test -v -run TestName ./path/to/pkg/` |
| Lint & gates | `lint` (runs `lint-config-verify` and `lint-fmt`, then golangci-lint), `lint-darwin` (golangci-lint with `GOOS=darwin GOARCH=arm64` on Darwin-tagged packages), `tidy`, `license-check`, `check-file-length`, `check-agent-docs`, `check-windows-build`, `vulncheck`, `sonar-local`, `render-diagrams`, `install-hooks` |
| goplint | `check-goplint-consumer-routed`, `check-baseline`, `update-baseline`, `check-goplint-exceptions`, `check-goplint-full-scan`, `check-goplint-repository-audit`, `check-goplint-performance-smoke` (single-sample smoke, not certification), `check-types[-all][-json]`; details: `docs/goplint/README.md` |
| Formal | `formal` (needs Java), `formal-alloy`, `formal-tla`, `formal-traces`, `formal-golden`, `formal-rapid-deep`, `formal-reach` (binding reach gate, no Java), `formal-promotion-gate` (read-only GitHub API); `python3 scripts/formal.py correspondence`; `python3 scripts/formal.py snapshot [--out FILE] [--compare FILE]` |
| Mutation (manual, advisory) | `mutation-dry-run`, `mutation-pr`, `mutation-full`, `mutation-baseline-update`, `mutation-rerun MUTATION_MUTANT_ID=<id>`, `mutation-formal[-dry-run\|-rerun\|-baseline-update]` (supports `MUTATION_FORMAL_SYMBOL=<[file:]sym>[,...]` or `MUTATION_FORMAL_STALE=1` scoping, and `MUTATION_FORMAL_JOBS=<n>` parallel workers), `mutation-formal-refresh MUTATION_FORMAL_SYMBOL=<[file:]sym>[,...]` or `MUTATION_FORMAL_STALE=1` (same-change ledger refresh from the last report-only run); reports under `artifacts/mutation/`; procedures: `.agents/runbooks/mutation-testing.md` |
| PGO & bench | `pgo-profile[-short]`, `pgo-profile-parse-discovery`, `pgo-audit`, `bench-bmf[-full]`; procedures: `.agents/runbooks/pgo-and-benchmarks.md` |
| Release | `release VERSION=vX.Y.Z RELEASE_NOTES_FILE=notes.md [YES=1] [DRY_RUN=1]`; `release-bump TYPE=major\|minor\|patch RELEASE_NOTES_FILE=notes.md [PRERELEASE=alpha\|beta\|rc] [PROMOTE=1] [YES=1] [DRY_RUN=1]`; `version-docs VERSION=X.Y.Z`; `goreleaser check`; `goreleaser release --snapshot --clean`; procedure: publish-release skill |
| Website | `cd website && npm start`, `cd website && npm run build` |

**Uncommitted work is protected mechanically; keep it that way.**

- Mutation targets: the wrapper reverts tracked files under `cmd/`, `internal/` and `pkg/` and deletes new untracked files there when it exits. It refuses to start while tracked work outside its allow-list (`dirty_path_is_allowed` in `scripts/mutation.sh`) is uncommitted, so commit regenerated artifacts first. Its restore helpers act only on the checkout that `ensure_clean_tracked_worktree_for_mutation` verified clean when the run started. That check is not repeated, so do not edit `cmd/`, `internal/` or `pkg/` in that checkout while a run is in progress: the exit restore reverts those edits too. An unarmed direct call exits non-zero; from the exit trap it logs a no-op and keeps the exit status.
- Script tests: `make test-scripts` (and CI) runs the single list in `scripts/script-tests.sh` inside a throwaway repository with canaries (`scripts/check-script-tests-worktree-safety.sh`). It exits 3 if any test changed a checkout or its git config, refs or stash, and prints recovery commands without restoring anything. A test that drives a destructive helper must use a temporary repository (`make_fixture_repo` in `scripts/test_mutation.sh`). Every new script test file (`test_*` or `*_test` shell, Python, JS or PowerShell, anywhere outside `tests/cli/` and `testdata/`) goes into that list; the run fails otherwise.

## Internal Commands (Hidden)

- All `invowk internal *` commands and subcommands MUST remain hidden.
- Do NOT document internal commands in website docs; only mention them in `README.md` and agent-facing docs under `.agents/`.
