# Invowk

Invowk is a command runner like `just`, `task` and `mise`, written in Go (version: the `go` directive in `go.mod`). Users define commands (`cmds`) in `invowkfile.cue` and run them under the reserved `invowk cmd` namespace. Modules are directories named `<module-id>.invowkmod` (RDNS ids preferred) holding an `invowkmod.cue` and an `invowkfile.cue`; they can require other modules and bundle scripts and files. Runtimes: `native` (host shell), `virtual-sh` (mvdan/sh plus u-root built-ins), `virtual-lua` (embedded golua) and `container` (Docker/Podman).

## Hard invariants

- **Virtual runtimes are not a sandbox.** Never describe `virtual-sh` or `virtual-lua` as sandboxed or isolated.
  - Host binaries are denied unless listed in `allowed_binaries` (`["*"]` allows all); allowed binaries run as native host processes with host access.
  - The shared harness gates resolution and path access; it is not a kernel jail, and mvdan/sh has no sandbox API.
  - The host environment, including `PATH`, is inherited by default (`env_inherit_mode: "all"`).
  - u-root built-in errors and host-binary policy denials never fall back silently to host execution.
  - In docs and examples: "no shell dependency" only means the interpreter is built in. Show `allowed_binaries` whenever a virtual example launches a host binary, and point to the `container` runtime for isolation.
- **The container runtime runs Linux containers only. Never use Alpine or Windows container images anywhere.** In tests, docs, CUE snippets and examples:
  - use `debian:stable-slim`; never `ubuntu:*`, `debian:<codename>` or any other base image;
  - language images (`golang:1.N`, `python:3-slim`, `node:22-slim`) only to demonstrate language-specific runtimes, with stable tags.
  - CI and build images follow the version-pinning rule. Why: `.agents/memorials/container-linux-only.md`.
- **Module dependencies are explicit only** (like Go modules): the root `invowkmod.cue` declares every module in the tree, and transitive dependencies are never resolved. `invowk module tidy` adds missing ones; `invowk module sync` fails with actionable errors. Lock file v2.0 stores SHA-256 content hashes.
- **Command scope is a static gate.** `CheckCommandDependenciesExist()` (`internal/app/deps/deps.go`) checks `depends_on.cmds` against the same module, global modules (`~/.invowk/cmds`) and first-level `requires`. It does not intercept scripts that call `invowk cmd` at run time; for isolation, use the container runtime.
- Pin every dependency, tool, action, image and MCP server in CI, build config, scripts and `.mcp.json` per the version-pinning rule, never `@latest` or unpinned; its documented exceptions apply. User-facing `go install …@latest` instructions are intentional.
- Sign every commit and merge into `main` only by squash merge (git rule). Work is not done until `make test` passes (checklist rule).

## Agent operation

- **CRITICAL:** track all plans and non-trivial work in the harness task tracker (Claude Code: `TaskCreate`/`TaskUpdate`): create tasks at the start, keep statuses current, and remove stale ones.
- **CRITICAL:** whenever possible and appropriate, use multiple tasks, teammates and subagents.
- **CRITICAL:** teammates must always get plan approval before they change anything.
- On compaction, keep file paths, symbol names, open issues and goals, architectural decisions, learnings, next steps and the last ~5 tool outputs. Never compact this file or rule, skill and agent definitions.

## Governance precedence

System and user instructions come first. For repository guidance, this order wins:

1. `AGENTS.md` (this file): indexes, scope maps and governance.
2. `.agents/rules/*.md`: normative rules.
3. `.agents/skills/*/SKILL.md`: procedures. Runbooks (`.agents/runbooks/`) have the same standing.
4. Package-level `AGENTS.md` files: scoped, explicit exceptions only.

A rule beats a skill or runbook unless the rule grants an exception. Memorials (`.agents/memorials/`) are history and never normative. `.claude/{rules,skills,agents,commands,workflows}` are compatibility symlinks; docs reference `.agents/...`.

## Where things live

- Schemas: `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkmod/invowkmod_schema.cue`, `internal/config/config_schema.cue`. All runtimes implement the interface in `internal/runtime/`.
- Code: `cmd/invowk/` (thin Cobra adapters), `internal/` (private packages; services under `internal/app/`), `pkg/` (public packages), `tests/cli/` (testscript), `samples/invowkmods/` (sample modules). Package map and code-area routing: `.agents/runbooks/repo-map.md`.
- Docs: `website/` (Docusaurus), `docs/architecture/` (diagrams), `formal/` (Alloy/TLA+ models), `openspec/` and `specs/` (specifications), `tasks/next/` (pending decisions), `.goplint/` (goplint consumer config; guide `docs/goplint/README.md`).
- Agent docs: `.agents/{rules,skills,agents,commands,workflows}/`. On-demand procedures are indexed in `.agents/runbooks/README.md`, history and past decisions in `.agents/memorials/README.md`; neither is auto-loaded.

## Quick commands

`make build`, `make test`, `make lint`, `make tidy`, `make check-goplint-consumer-routed`, `make check-baseline`. Every target: the commands rule.

## Rules index

When a file in `.agents/rules/` is added, renamed or removed, update this index in the same change; `make check-agent-docs` verifies it. Every rule, skill, agent, workflow and command appears here exactly once in bracketed form.

- Always loaded:
  - [`.agents/rules/agent-workflows.md`]: Workflow-agent floor (review, guard, outward writes, lock, restart, advisor)
  - [`.agents/rules/checklist.md`]: completion gates
  - [`.agents/rules/commands.md`]: make targets, hidden internal commands
  - [`.agents/rules/general-rules.md`]: sources, code quality, scope
  - [`.agents/rules/git.md`]: signing, squash merges, commit messages
- Loaded when touching matching files (`paths:` frontmatter); read them directly if your tool does not auto-load:
  - [`.agents/rules/ci-workflows.md`]: `.github/`, `scripts/`, `Makefile`, `tools/`, pre-commit config
  - [`.agents/rules/version-pinning.md`]: workflows, `go.mod`, `Makefile`, scripts, Dockerfiles, `build/`, `.mcp.json`, `formal/manifest.toml`, GoReleaser, `website/package.json`
  - [`.agents/rules/formal-bindings.md`]: code named by formal correspondence tables
  - [`.agents/rules/cue-patterns.md`]: `*.cue`, invowkfile, invowkmod, config, cueutil
  - [`.agents/rules/licensing.md`]: `*.go`
  - [`.agents/rules/package-design.md`]: `cmd/`, `internal/`, `pkg/`
  - [`.agents/rules/testing.md`]: `*_test.go`, `tests/cli/`, testdata, `internal/testutil/`
  - [`.agents/rules/windows.md`]: path-handling packages (runtime, container, provision, config, app, modulelock, audit, uroot, watch, testutil, platform, fspath, types, invowkmod, invowkfile), `tests/cli/*.go`, `native_*.txtar`

## Skills index

- Go and architecture: [`.agents/skills/go/`] [`.agents/skills/go-hexagonal-ddd/`] [`.agents/skills/invowk-typesystem/`] [`.agents/skills/cli/`] [`.agents/skills/discovery/`] [`.agents/skills/server/`]
- Schema: [`.agents/skills/cue/`] [`.agents/skills/invowk-schema/`] [`.agents/skills/schema-sync-check/`]
- Runtimes: [`.agents/skills/shell/`] [`.agents/skills/virtual-lua/`] [`.agents/skills/uroot/`] [`.agents/skills/container/`]
- Testing: [`.agents/skills/go-testing/`] [`.agents/skills/testing/`] [`.agents/skills/native-mirror/`] [`.agents/skills/windows-testing/`] [`.agents/skills/macos-testing/`] [`.agents/skills/linux-testing/`] [`.agents/skills/tmux-testing/`] [`.agents/skills/tui-testing/`] [`.agents/skills/fixer/`] [`.agents/skills/review-tests/`]
- Docs and diagrams: [`.agents/skills/docs/`] (editing), [`.agents/skills/review-docs/`] (audits), [`.agents/skills/d2-diagrams/`] (default for new diagrams)
- Release, CI and dependencies: [`.agents/skills/publish-release/`] [`.agents/skills/changelog/`] [`.agents/skills/ci-update/`] [`.agents/skills/dep-audit/`] [`.agents/skills/bencher/`]
- Security and formal methods: [`.agents/skills/module-security/`] [`.agents/skills/formal-verification/`]
- Meta: [`.agents/skills/learn/`]
- User-only, never auto-invoke:
  - [`.agents/skills/speckit.specify/`] [`.agents/skills/speckit.clarify/`] [`.agents/skills/speckit.plan/`] [`.agents/skills/speckit.tasks/`] [`.agents/skills/speckit.taskstoissues/`] [`.agents/skills/speckit.implement/`] [`.agents/skills/speckit.analyze/`] [`.agents/skills/speckit.checklist/`] [`.agents/skills/speckit.constitution/`]
  - [`.agents/skills/openspec-apply-change/`] [`.agents/skills/openspec-archive-change/`] [`.agents/skills/openspec-explore/`] [`.agents/skills/openspec-propose/`] [`.agents/skills/openspec-sync-specs/`] [`.agents/skills/openspec-update-change/`]

## Agents and commands

- Agents (spawnable reviewers and generators): [`.agents/agents/code-reviewer.md`] [`.agents/agents/cue-schema-agent.md`] [`.agents/agents/performance-analyzer.md`] [`.agents/agents/security-reviewer.md`] [`.agents/agents/supply-chain-reviewer.md`] [`.agents/agents/ci-readiness.md`] [`.agents/agents/test-writer.md`]
- Workflow templates (Workflow tool scripts; floor: `.agents/rules/agent-workflows.md`): [`.agents/workflows/change-flow.js`]
- Slash commands: [`.agents/commands/fix-it.md`] [`.agents/commands/fix-it-simple.md`] [`.agents/commands/improve-type-system.md`] [`.agents/commands/review-docs.md`] [`.agents/commands/review-rules.md`] [`.agents/commands/review-tests.md`] [`.agents/commands/review-type-system.md`] [`.agents/commands/rust-alt.md`]
