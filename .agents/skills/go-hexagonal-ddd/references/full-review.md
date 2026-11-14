# Manual Full-Codebase Architecture Review

Procedure for an explicitly requested full, repo-wide, or codebase architecture
review with the `go-hexagonal-ddd` skill. Never run it for ordinary package
design or implementation tasks. `SKILL.md` § "Manual Full-Codebase Review"
holds the one-line coordinator directives; this file is the full procedure.

## Coordinator Duties

1. Read `AGENTS.md`, `.agents/rules/package-design.md`, and
   `.agents/skills/go-hexagonal-ddd/references/source-guide.md`.
2. Capture a stable baseline: branch/HEAD, `git status --short`,
   `go list ./cmd/... ./internal/... ./pkg/...`, and
   `go list ./...` (root module).
   Also capture a package inventory for drift checks:
   `find cmd internal pkg -maxdepth 2 -type d | sort`. Every package in that
   inventory must belong to one surface below; report any package that does
   not (surface drift) instead of silently skipping it.
3. Create a task list and launch subagents for the review surfaces below. Use no
   more than six live subagents. If fewer slots are available, queue the
   remaining surfaces and launch them only as slots free up.
4. Tell every subagent this is a read-only review. They must not edit files, run
   broad formatters, or propose speculative rewrites.
5. Do not substitute coordinator analysis for a queued surface. The coordinator
   may inspect shared metadata, deduplicate results, and verify reported
   evidence, but each owned surface should be reviewed by its assigned subagent.
6. Merge subagent reports by evidence quality, not by volume. Prefer no finding
   over a low-confidence architecture opinion.

## Review Surfaces

Use these deterministic surfaces unless the user narrows scope. Test-only
helpers (`internal/testutil/`) and `internal/benchmark/` are reviewed only as
evidence for the surface whose code they exercise.

| Surface | Primary paths | Focus |
| --- | --- | --- |
| SA-1 CLI and app services | `cmd/invowk/`, `internal/app/commandsvc/`, `internal/app/execute/`, `internal/app/deps/`, `internal/app/commandadapters/` | Driving adapter boundaries, use-case orchestration, app-service/adapters split, domain policy placement |
| SA-2 Discovery and modules | `internal/discovery/`, `pkg/invowkmod/`, `internal/app/modulecache/`, `internal/app/moduleops/`, `internal/app/modulesync/`, `samples/invowkmods/`, `tests/cli/testdata/*module*.txtar`, `*.invowkmod` and `invowk_modules/` fixtures | Dependency graph semantics, scope rules, module aggregate boundaries, module cache/sync/lock use cases |
| SA-3 Runtime and outside devices | `internal/runtime/`, `internal/container/`, `internal/containerplan/`, `internal/provision/`, `internal/provisionenv/`, `pkg/containerargs/`, `internal/watch/`, `internal/uroot/` | Ports/adapters, host process/container/filesystem boundaries, persistent/ephemeral container target policy, provisioning environment contract, deterministic test seams |
| SA-4 Schemas and value types | `pkg/invowkfile/`, `pkg/types/`, `pkg/fspath/`, `pkg/platform/`, `internal/config/`, `pkg/cueutil/` | Invariants, value-object placement, schema/domain language drift, path and platform foundations |
| SA-5 Audit, security, and LLM domains | `internal/audit/`, `internal/auditllm/`, `internal/issue/`, `internal/llm/`, `internal/agentcmd/`, `internal/acpclient/`, `internal/app/llmconfig/`, lock-file and module-security call sites | Finding model, trust boundaries, policy services, error/diagnostic ownership, LLM contract and adapter boundaries |
| SA-6 UI/server adapters and tools | `internal/tui/`, `internal/tuiserver/`, `internal/tuiclient/`, `internal/tuiwire/`, `internal/sshserver/`, `internal/core/serverbase/` | Adapter leakage, server lifecycle boundaries, TUI delegation wire contract |

## Subagent Prompt Shape

Give each subagent the same constraints plus its assigned surface. Load the
skill with the harness's own mechanism: Codex uses `$go-hexagonal-ddd`; Claude
Code loads it with the Skill tool (`go-hexagonal-ddd`) or `/go-hexagonal-ddd`.

```text
Load the go-hexagonal-ddd skill (Codex: $go-hexagonal-ddd; Claude Code: Skill
tool or /go-hexagonal-ddd) from the repository root for a read-only
architecture review of <surface>. Read AGENTS.md, the relevant .agents/rules
files, and .agents/skills/go-hexagonal-ddd/references/source-guide.md.
Inspect current code, imports, call sites, and tests for this surface only.
Do not edit files or run broad formatters.
Return only high-confidence Hexagonal Architecture or DDD findings. For each
finding include: evidence files/symbols, why the current boundary causes real
design pressure, the smallest proposed design adjustment, and verification to
run. If nothing meets that bar, say so.
```

## High-Confidence Gate

Report a finding only when all are true:

- It is grounded in current files, imports, call sites, or tests with concrete
  symbols and paths.
- It names the Invowk capability or policy being misplaced, duplicated, hidden
  in an adapter, or coupled to an outside device.
- It explains real pressure: blocked deterministic tests, repeated policy
  logic, dependency direction mismatch, schema/domain language drift,
  nondeterministic outside actor coupling, or a security/trust-boundary leak.
- It proposes the smallest design adjustment that would improve ownership,
  coupling, invariants, or testability without adding ceremony.
- It includes a verification path such as narrow Go tests, CLI testscript,
  schema sync checks, or package import checks.

Omit items based only on naming taste, preferred layering style, hypothetical
future adapters, generic "clean architecture" rules, or value-object wrapping
with no validation/normalization leverage.

## Final Report Shape

Lead with findings, ordered by impact and confidence. For each finding use this
shape:

- **Finding:** concise title with affected surface.
- **Evidence:** concrete files/symbols/call sites.
- **Why it matters:** the current design pressure in Invowk terms.
- **Design adjustment:** the smallest proposed package/type/port movement.
- **Verification:** exact checks or tests that would confirm the adjustment.

After findings, add "No High-Confidence Finding" surfaces so the user can see
which areas were reviewed without noise. Keep speculative follow-ups separate
and clearly labeled, or omit them if the user asked for findings only.
