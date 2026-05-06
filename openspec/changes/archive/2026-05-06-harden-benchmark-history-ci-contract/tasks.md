## 1. Benchmark Contract Job

- [x] 1.1 Add a dedicated Ubuntu `benchmark-history-contract` job to `.github/workflows/ci.yml` with Go and Node setup.
- [x] 1.2 Run `node scripts/test_benchmark_report.mjs` in the contract job.
- [x] 1.3 Run committed history validation with `node scripts/benchmark-report.mjs validate-history --input website/static/benchmarks/history.json`.
- [x] 1.4 Run history-aware release staging in the contract job with `TAG=v0.0.0-ci`, `STARTUP_SAMPLES=3`, `BENCH_COUNT=1`, `BENCH_REPORT_OUT_DIR=$RUNNER_TEMP/release-benchmarks`, and `BENCH_HISTORY_JSON=website/static/benchmarks/history.json`.
- [x] 1.5 Add contract assertions that release-named Markdown, JSON, SVG, and raw benchmark assets exist in `release-assets/`.
- [x] 1.6 Add a contract assertion that the generated Markdown includes the history/evolution section when history input is present.

## 2. Release Workflow Alignment

- [x] 2.1 Pass `BENCH_HISTORY_JSON=website/static/benchmarks/history.json` in the `Release Dry Run` benchmark staging step.
- [x] 2.2 Pass `BENCH_HISTORY_JSON=website/static/benchmarks/history.json` in the main release workflow benchmark staging step.
- [x] 2.3 Pass `BENCH_HISTORY_JSON=website/static/benchmarks/history.json` in the manual release benchmark fallback workflow.
- [x] 2.4 Keep local staging compatible with omitted `BENCH_HISTORY_JSON` for first-report or alternate-history local runs.

## 3. Staged Asset Validation

- [x] 3.1 Ensure staged release JSON assets are validated with the benchmark report schema.
- [x] 3.2 Ensure staged release SVG assets are validated as non-empty SVG summaries.
- [x] 3.3 Ensure staged release raw output assets are non-empty and release-named consistently.
- [x] 3.4 Keep missing, duplicate, and unexpected release asset failures actionable by listing discovered assets.

## 4. Website Static Output Checks

- [x] 4.1 Add a post-build check in `.github/workflows/test-website.yml` for the built performance page output.
- [x] 4.2 Add a post-build check in `.github/workflows/test-website.yml` for the built static benchmark history JSON.
- [x] 4.3 Keep existing docs parity, version asset, benchmark history validation, and website build checks in place.

## 5. Verification

- [x] 5.1 Run `node scripts/test_benchmark_report.mjs`.
- [x] 5.2 Run `node scripts/benchmark-report.mjs validate-history --input website/static/benchmarks/history.json`.
- [x] 5.3 Run a local history-aware staging dry-run with low sample counts and verify `release-assets/` contents.
- [x] 5.4 Run website validation/build commands that cover the performance page and static benchmark history data.
- [x] 5.5 Run workflow or lint checks available locally for changed GitHub Actions YAML and shell scripts.
- [x] 5.6 Run `openspec status --change harden-benchmark-history-ci-contract` and confirm the change is apply-ready.
