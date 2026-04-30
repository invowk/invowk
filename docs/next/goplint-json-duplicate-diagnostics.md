# Goplint Backlog: Duplicate JSON Diagnostics for Test Variants

## Status

Backlog.

When goplint runs in JSON mode over packages that also have test variants, the same diagnostic object can appear twice: once under the regular package key and once under the synthetic test package key (`package [package.test]`). This makes machine-readable backlog counts look twice as large unless the caller deduplicates by diagnostic identity.

Observed during full backlog triage on 2026-04-30:

```sh
./bin/goplint -check-all -check-enum-sync -json \
  -config=tools/goplint/exceptions.toml \
  ./cmd/... ./internal/... ./pkg/... > /tmp/invowk-goplint.json

jq '[.[]?.goplint[]?] | length' /tmp/invowk-goplint.json
# 246

jq -r '.[]?.goplint[]? | [.posn,.category,.message] | @tsv' \
  /tmp/invowk-goplint.json | sort -u | wc -l
# 123
```

The duplicated package keys had the same per-package diagnostic counts:

```text
github.com/invowk/invowk/cmd/invowk                                  24
github.com/invowk/invowk/cmd/invowk [github.com/invowk/invowk/cmd/invowk.test] 24
github.com/invowk/invowk/internal/audit                               93
github.com/invowk/invowk/internal/audit [github.com/invowk/invowk/internal/audit.test] 93
github.com/invowk/invowk/pkg/invowkmod                                  6
github.com/invowk/invowk/pkg/invowkmod [github.com/invowk/invowk/pkg/invowkmod.test] 6
```

## Why It Matters

Backlog automation, baselines, and one-shot remediation planning should not have to know about Go analysis package/test variant duplication. A JSON consumer should be able to trust that each emitted diagnostic is a unique finding unless goplint explicitly documents otherwise.

The current behavior is especially noisy for `make check-types-all` style audits because high-signal category counts appear inflated and can make resolved work look incomplete.

## Desired Fix

goplint should emit each logical diagnostic once in JSON output.

Acceptable implementation paths:

- Deduplicate diagnostics before JSON encoding by stable identity: package import path without test variant suffix, position, category, and message.
- Suppress test-variant package entries when the diagnostic position belongs to non-test source and an identical regular-package diagnostic already exists.
- If retaining package/test separation is important for other analyzers, add a documented `--dedupe-json-diagnostics` behavior and enable it for goplint CLI output by default.

## Acceptance Criteria

- Running goplint JSON mode over `./cmd/... ./internal/... ./pkg/...` produces equal direct and unique counts for identical diagnostic identity:

  ```sh
  jq '[.[]?.goplint[]?] | length' out.json
  jq -r '.[]?.goplint[]? | [.posn,.category,.message] | @tsv' out.json | sort -u | wc -l
  ```

- Text output and baseline matching remain unchanged unless they currently depend on duplicated package/test diagnostics.
- Tests cover at least one package with a non-test source diagnostic and a test variant, proving the JSON output contains one logical diagnostic.
