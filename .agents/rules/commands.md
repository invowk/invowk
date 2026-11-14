# Commands

Run targets from the repository root. Go comes from the `go` directive in `go.mod`. Optional tools: Docker or Podman (container tests), UPX (`build-upx`), gotestsum (`make test` uses it when present), govulncheck (`vulncheck`), Node (website; CI uses the workflows' `node-version`, `website/package.json` sets `engines`), and Java for the formal targets only (JDK version: version-pinning "Formal-Verification Tools"; `scripts/formal.py fetch` downloads and verifies the jars). Tool versions live in `.agents/rules/version-pinning.md`; never install a pinned tool with `@latest`. Run go-mutesting only through the Make targets or `go tool go-mutesting`.

| Group | Targets (`make <target>` unless shown) |
|-------|----------------------------------------|
| Build | `build` (x86-64 defaults to `GOAMD64=v3`; `GOAMD64=v1` for baseline CPUs), `build-dev`, `build-upx`, `build-all`, `build-cross`, `install`, `clean` |
| Test | `test` (full suite; already runs `test-cli`), `test-short`, `test-integration`, `test-cli`, `test-cli-cover`, `test-scripts`; one test: `go test -v -run TestName ./path/to/pkg/` |
| Lint & gates | `lint` (runs `lint-config-verify` and `lint-fmt`, then golangci-lint), `tidy`, `license-check`, `check-file-length`, `check-agent-docs`, `check-windows-build`, `vulncheck`, `sonar-local`, `render-diagrams`, `install-hooks` |
| goplint | `check-goplint-consumer-routed`, `check-baseline`, `update-baseline`, `check-goplint-exceptions`, `check-goplint-full-scan`, `check-goplint-repository-audit`, `check-goplint-performance-smoke` (single-sample smoke, not certification), `check-types[-all][-json]`; details: `docs/goplint/README.md` |
| Formal | `formal` (needs Java), `formal-alloy`, `formal-tla`, `formal-traces`, `formal-golden`, `formal-rapid-deep`, `formal-promotion-gate` (read-only GitHub API); `python3 scripts/formal.py correspondence`; `python3 scripts/formal.py snapshot [--out FILE] [--compare FILE]` |
| Mutation (manual, advisory) | `mutation-dry-run`, `mutation-pr`, `mutation-full`, `mutation-baseline-update`, `mutation-rerun MUTATION_MUTANT_ID=<id>`, `mutation-formal[-dry-run\|-rerun\|-baseline-update]`; reports under `artifacts/mutation/`; procedures: `.agents/runbooks/mutation-testing.md` |
| PGO & bench | `pgo-profile[-short]`, `pgo-profile-parse-discovery`, `pgo-audit`, `bench-bmf[-full]`; procedures: `.agents/runbooks/pgo-and-benchmarks.md` |
| Release | `release VERSION=vX.Y.Z RELEASE_NOTES_FILE=notes.md [YES=1] [DRY_RUN=1]`; `release-bump TYPE=major\|minor\|patch RELEASE_NOTES_FILE=notes.md [PRERELEASE=alpha\|beta\|rc] [PROMOTE=1] [YES=1] [DRY_RUN=1]`; `version-docs VERSION=X.Y.Z`; `goreleaser check`; `goreleaser release --snapshot --clean`; procedure: publish-release skill |
| Website | `cd website && npm start`, `cd website && npm run build` |

**Mutation warning:** commit regenerated artifacts before running any mutation target. The wrapper's tracked-source restore reverts every uncommitted tracked file when it exits, not only the mutated packages, so an uncommitted regenerated artifact is silently lost.

## Internal Commands (Hidden)

- All `invowk internal *` commands and subcommands MUST remain hidden.
- Do NOT document internal commands in website docs; only mention them in `README.md` and agent-facing docs under `.agents/`.
