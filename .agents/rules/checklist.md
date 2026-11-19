# Agent Checklist

**CRITICAL: Work is NOT complete until `make test` passes.** Never stop at `make test-short`: short mode skips the integration, container and full CLI tests that CI runs. If any step below changes code, restart from step 1 (a change-flow fix-and-gates agent restarts from step 2: `.agents/rules/agent-workflows.md`).

1. Review the changed code for reuse, simplification and efficiency, and fix what you find (Claude Code: `/simplify`).
2. `make tidy`; for new Go files, `make license-check`.
3. `make lint`: fix ALL issues, including pre-existing ones.
4. `make test`: the full suite. It already runs `make test-cli`; run `test-cli` alone only while iterating on CLI tests.
5. `make check-baseline`, then triage every new goplint finding, whatever the task scope: decide whether the fix belongs in Invowk or in the analyzer (`github.com/invowk/goplint`). If both are plausible, stop and ask the user. Protocol inconclusives are always blocking: never baseline, except or inline-ignore them.
6. `make check-file-length`: every Go file, production and test, stays under 1000 lines.
7. `make sonar-local`: fix real bugs and vulnerabilities. For a false positive, record it in BOTH `sonar-project.properties` and `.sonarcloud.properties`, keeping multicriteria IDs gapless, and list it for the maintainer, who resolves the issue in the SonarCloud UI (False Positive or Accepted, with a comment naming the multicriteria entry). An agent never changes SonarCloud issue status itself. Automatic Analysis ignores file-based issue suppressions, so the file entries alone change nothing.
8. Conditional checks:

   | When | Run |
   |------|-----|
   | `scripts/`, `tools/mutation/` or `formal/` changed | `make test-scripts` |
   | Code bound by formal correspondence tables changed (`.agents/rules/formal-bindings.md`) | `python3 scripts/formal.py correspondence`; `make formal` when Java is available |
   | Hot-path files changed and the `pgo-staleness` pre-commit hook warns | Prefer `make pgo-profile-parse-discovery` and commit `default.pgo`; advisory, since CI `pgo-sanity` regenerates it otherwise |
   | A `*_schema.cue`, CUE-backed Go struct or `internal/config/types.go` changed | `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/` |
   | Website changed | `cd website && npm run build` |
   | Module-related changes | `for m in samples/invowkmods/*.invowkmod; do go run . validate "$m"; done` |
   | `AGENTS.md`, `.claude/`, `.agents/` or `scripts/check-agent-docs.sh` changed | `make check-agent-docs` |

9. Docs and diagrams. User-facing behavior, configuration, CLI/API or schema changes update the README and website docs (sync map: `.agents/skills/docs/SKILL.md`). Changes to package boundaries, execution flow, runtime selection, discovery or major integrations re-check `docs/architecture/`. New diagrams use D2 (`.agents/skills/d2-diagrams/`); run `make render-diagrams` after D2 edits. The task is not complete until docs and diagrams match behavior.
10. Runtime mirrors: every `virtual_<feature>.txtar` needs a `native_<feature>.txtar` mirror (enforced by `TestShRuntimeMirrorCoverage` and `TestVirtualNativeCommandPathAlignment`). Exceptions for virtual files live only in `tests/cli/runtime_mirror_exemptions.json`; non-virtual tests (`container_*`, `dogfooding_*`, built-in command tests) need no mirror.
11. Last step: `/learn` (`.agents/skills/learn/`).

Pre-commit runs lint, the routed goplint gate, the Windows cross-build (`make check-windows-build`), `sonar-local` and a PGO staleness advisory. Everything else here is manual. Details: `.agents/runbooks/quality-gates.md`.
