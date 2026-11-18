# Change Flow

One Workflow per OpenSpec change, one agent per stage, run from the main session with the template `.agents/workflows/change-flow.js`. The floor every stage keeps is `.agents/rules/agent-workflows.md`; on conflict the rule wins over this runbook and over the template.

## Running it

Create the change worktree and branch first (`git worktree add <worktree> -b <branch> origin/main`), then call the Workflow tool with `scriptPath: ".agents/workflows/change-flow.js"` and `args`:

| Arg | Required | Meaning |
|-----|----------|---------|
| `mode` | yes | `new`, `revise`, `continue`, `finalize`, `fast-lane` or `dry-run` |
| `change` | yes (not dry-run) | OpenSpec change id (`openspec/changes/<change>/`) |
| `worktree` | yes | absolute path of the change worktree |
| `branch` | yes | the change branch (never `main`) |
| `primary` | yes | absolute path of the main checkout, for the guard |
| `pr` | no | the draft PR number, once it exists |
| `base` | no | default `origin/main` |
| `from` | with `continue` | `implement`, `simplify`, `review` or `fix` |
| `notes` | no | an extra brief, for example the path of a decisions file |

The script validates the args and throws on a bad one before it spawns any agent. Run `mode: "dry-run"` after editing the template: it runs the script's self-checks (arg validation, stage plans, lens-id normalisation, guard comparison, failed-review detection) and spawns no agent. `python3 scripts/test_change_flow_template.py` (part of `make test-scripts`) drives the same template under node with stub agents.

## Modes

- `new`: the change does not exist yet. Spec with `openspec-propose`, then the full flow.
- `revise`: the change exists and its artifacts need edits. Spec with `openspec-update-change`, then the full flow.
- `fast-lane`: docs, agent tooling or CI-only changes. One agent writes the spec and self-reviews it, then implementation onward. If the lens selector says the change is not fast-lane eligible (Go or non-testdata CUE files, or a formal, schema, security, supply-chain or architecture lens), the independent spec review runs before code review.
- `continue`: resume after a stopped run, at `from`. Use it when the earlier stages are done and committed.
- `finalize`: only the fix-and-gates stage, for example after the maintainer answered a `maintainer_asks` item.

## Stages and why this order

1. **Spec** (`openspec-propose` or `openspec-update-change`).
2. **Spec review**: an independent read-only agent. Blocking findings go to one spec-fix agent (`openspec-update-change`) and a re-review; if they remain, the run returns `blocked`.
3. **Implement** (`openspec-apply-change`): tests with the code, commit, push, draft PR, and the formal lane below.
4. **Simplify** (`simplify`, quality only, on the committed diff against the base). It runs only the tests of what it touched.
5. **Lenses**: `scripts/change-flow-lenses.py --base <base> --head HEAD` picks the domain lenses from the paths the change touches. Triggers are read live from the rules that own them (`paths:` frontmatter of formal-bindings, windows, cue-patterns, ci-workflows and version-pinning; the pgo-staleness hook regex) plus the security and module-security areas, which the script test keeps in sync with their agent and skill files. Malformed output selects every lens, and the script logs it.
6. **Review fan-out**, in parallel, findings only: the `base` lens (`.agents/agents/code-reviewer.md` over the diff, plus the delta specs) always; `code-review high <pr or branch>` always; and the selected domain lenses, each loading its skill or agent file by name.
7. **Fix and gates**: one independent agent rebases on the base, dedupes and verifies every finding, applies the confirmed ones, records the rejected ones with reasons, runs the full `.agents/rules/checklist.md` once on the final tree, archives the change (`openspec-archive-change`), runs `/learn` minimally, pushes, and watches `gh pr checks`. It returns `{status, base_sha, head_sha, gates, rejected, maintainer_asks}` and never marks the PR ready or merges.

`simplify` runs before review because it edits logic: the last pass that edits code must be the bug-hunting one, or refactored code would reach `main` with no correctness review. The gates run once, at the end, because every earlier stage can still change the tree.

## Formal lane

- Implementation runs `python3 scripts/formal.py affected --event pull_request --base <base> --head HEAD`. When it prints `run=true`, it also runs `python3 scripts/formal.py correspondence`, `python3 scripts/formal_mutation.py triage --check` and `make formal-reach`.
- `make formal` is left to the required `Models, golden vectors, and correspondence` CI check, unless Java is present locally and the lane is affected.
- The fix agent runs `make mutation-formal` once, report-only, in a throwaway worktree, only when a function body bound by `.agents/rules/formal-bindings.md` was rewritten: a green `triage --check` proves nothing about a rewritten body.

## Merging

The template ends at "ready for maintainer merge". The main session then checks that the returned `base_sha` equals the current `origin/main` (otherwise it reruns `finalize`), and squash-merges as `git-merge.md` describes once the maintainer approves. The main session also handles every `maintainer_asks` item: SonarCloud issue status, PR ready, labels and the like.

## Probe facts (2026-11-18)

Measured in this repository's sessions; re-check them after a Claude Code upgrade.

- P1. Workflow agents (the default type and `general-purpose`) have `Skill`, but not `Agent` or `Workflow`. They load `CLAUDE.md`, the always-loaded rules and the memory index, but not the conversation: decisions must live in repo rules.
- P2. `code-review` always runs as a fork in the session's primary directory (the main checkout), not in the agent's worktree. With no target it reviewed the main checkout. With an explicit branch or PR target it reviews the right diff, because refs are shared across worktrees. A positive control with seeded Go bugs returned 9 findings, both seeded bugs included, to an agent with no schema.
- P3. When the calling agent has a `schema`, the fork's output is lost: the agent sees only "Skill execution completed". A "(none)" result with no explanation counts as a failed lens, not as zero findings; the template logs it and the `base` lens keeps the floor.
- P4. `simplify` runs inline and single-pass ("Agent tool unavailable -> single-pass inline cleanup"). It follows the cwd only when the agent uses `cd <worktree> && ...` or `git -C`; Bash cwd resets to the primary directory between calls.
- P5. Checklist step 7 used to tell agents to resolve Sonar false positives in the SonarCloud UI, an outward write. Since 2026-11-18 the agent records the entry in both properties files and lists it for the maintainer.
