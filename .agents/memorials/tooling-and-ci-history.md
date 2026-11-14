# Tooling and CI history

Historical notes moved out of the rules on 2026-11-14. None of this is normative.

- **Benchmark reports.** The former `bench-report`/`bench-report-full` Make targets, the `invowk_<version>_bench-report.md` release asset and the `release-benchmark-asset.yml` fallback workflow were replaced by BMF generation (`make bench-bmf[-full]`) and Bencher. None of them exists any more.
- **go-mutesting labels.** `go-mutesting v2.7.0` printed inverted `PASS`/`FAIL` labels. Current versions print `KILLED`/`ESCAPED`; keep any old example version-scoped.
- **context7-mcp v4.** The v4 breaking changes affected only the HTTP transport; the stdio invocation in `.mcp.json` did not change.
- **golua fork.** `github.com/invowk/golua` is invowk's maintained fork of `arnodel/golua`, patched for the Go 1.27 `//go:linkname` allowlist removal via `hash/maphash.Comparable`.
- **goplint extraction.** goplint moved from the in-tree `tools/goplint` to the standalone `github.com/invowk/goplint` repository (relocation noted 2026-08-26 in `docs/next/mutation-testing.md`). Its analyzer-soundness assurance and its own test-parallelism policy are governed there; invowk only consumes released versions.
- **CodeQL default setup.** GitHub's default CodeQL setup was disabled because it cannot coexist with advanced-setup uploads and its runners pin their own Go toolchain, which cannot follow `go.mod` bumps.
- **Release docs.** Older agent docs described workflow-dispatch release tags as lightweight and tag pushes with `git tag -s -m "Release vX"`. The release workflow now creates annotated tags whose message is the release notes (`RELEASE_NOTES_FILE`); see the publish-release skill.
