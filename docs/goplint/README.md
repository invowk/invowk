# goplint in invowk (consumer guide)

Invowk enforces its DDD Value Type conventions with
[goplint](https://github.com/invowk/goplint), pinned as a Go tool dependency
in the root `go.mod` (see `.agents/rules/version-pinning.md` for the current
version). Everything analyzer-internal — the soundness-assurance harness,
semantic evidence, completion records, fuzzing, mutation kernels, and their
documentation — lives in that repository and gates every release invowk can
pin.

## What runs here

| Gate | Command | What it proves |
|------|---------|----------------|
| Type checks | `make check-types` / `make check-types-all` | No new bare primitives / full DDD compliance against `.goplint/exceptions.toml` |
| Baseline | `make check-baseline` | No new findings beyond `.goplint/baseline.toml` |
| Exceptions | `make check-goplint-exceptions` | No stale or overdue exception entries |
| Full scan | `make check-goplint-full-scan` | The blocking canonical production scan is clean |
| Repository audit | `make check-goplint-repository-audit` | Produces the canonical audit artifact other gates reuse |
| Performance smoke | `make check-goplint-performance-smoke` | One live-tree scan stays under catastrophic wall/RSS limits (not certification) |
| Routed gate | `make check-goplint-consumer-routed` | Pre-commit entry: documentation-only diffs skip analysis; everything else runs the consumer tier |

All gates run the analyzer built by `scripts/goplint.sh build`, which verifies
the embedded module version against the pin. Set
`GOPLINT_REPOSITORY_AUDIT_PATH` to share one canonical scan across the audit,
baseline, exception, and full-scan verdicts (the CI `goplint consumer gates`
job does this).

## Invowk-owned configuration (`.goplint/`)

- `baseline.toml` — accepted findings; regenerate with `make update-baseline`.
- `exceptions.toml` — intentional primitive-usage exceptions with reasons and
  review dates.
- `consumer-smoke.github-ubuntu-x64-4cpu.toml` — catastrophic-regression
  limits for the live-tree smoke scan, calibrated for this codebase.
- `consumer-gate.v1.json` — consumer-gate descriptor digested into every
  repository-audit input binding.
- `ownership.v1.json` — two-class routing manifest (documentation vs
  consumer, fail-closed to consumer) used by the pre-commit routed gate.

## Where the rest went

Analyzer semantics, `//goplint:` directive reference, diagnostic categories,
soundness profiles, evidence and completion-record machinery are documented in
the [goplint repository](https://github.com/invowk/goplint) under
`docs/goplint/` and `AGENTS.md`, guarded there by its `docs-guard` gate.
Upgrading the pin follows `.agents/rules/version-pinning.md`; dependabot
proposes bumps through the root `gomod` ecosystem entry.
