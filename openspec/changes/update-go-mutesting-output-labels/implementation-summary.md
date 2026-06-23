## Implementation Summary

Selected tool version: `github.com/jonbaldie/go-mutesting/v2 v2.7.1`.

`go list -m -versions github.com/jonbaldie/go-mutesting/v2` showed `v2.7.1`
as the newest tagged v2 module version. `go list -m -json
github.com/jonbaldie/go-mutesting/v2@v2.7.1` resolved the upstream tag
`refs/tags/v2.7.1`, dated `2026-06-02T20:26:26Z`.

The root Go tool dependency was updated through the Go tool dependency workflow:

- `go get -tool github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting@v2.7.1`
- `go mod tidy`

The wrapper expectation now matches the tool pin:

- `scripts/mutation.sh` expects `GO_MUTESTING_VERSION="v2.7.1"`.
- `.agents/rules/version-pinning.md` lists `go-mutesting v2.7.1`.
- `go version -m "$(go tool -n go-mutesting)"` reports embedded module
  `github.com/jonbaldie/go-mutesting/v2 v2.7.1`.

Current terminal label semantics are documented as:

- `KILLED`: tests caught the mutant.
- `ESCAPED`: the mutant survived.

The downloaded `v2.7.1` module source confirms these labels in the README,
console constants, and command output construction. Current Invowk docs and
agent guidance now use `KILLED` / `ESCAPED`, while historical triage notes keep
old `go-mutesting v2.7.0` `PASS` / `FAIL` interpretations explicitly
version-scoped.

Automation continues to avoid terminal-label scraping. The wrapper still selects
machine-readable reports and stable IDs through flags such as
`--logger-summary-json`, `--logger-agentic-json`, `--baseline`, and
`--run-mutant-id`; the rerun profile uses `--output-statuses=e` as a status
selector, not a parsed terminal label.

Verification completed:

- `go mod tidy`
- `go mod tidy -diff`
- `go version -m "$(go tool -n go-mutesting)"`
- `bash scripts/test_mutation.sh` (`52 passed, 0 failed`)
- `make test-scripts`
- `MUTATION_REPORT_DIR="$(mktemp -d)" make mutation-dry-run MUTATION_MODULE=root`
- `make lint`
- `make check-agent-docs`
- `make check-baseline`
- `govulncheck ./...`
- `make test`
- `make check-file-length`
- `git diff --check`
- `openspec validate update-go-mutesting-output-labels --type change --strict --json`

Mutation baselines and survivor counts were not recomputed for this label/tool
update. `git diff -- tools/mutation/baselines/root-baseline.json
tools/mutation/baselines/goplint-baseline.json` was empty; current baseline
counts remain root `2`, goplint `0`.
