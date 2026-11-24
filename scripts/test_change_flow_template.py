#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Self-test for the change-flow Workflow template (.agents/workflows/change-flow.js).

The Workflow runtime runs the script body as an async function, so the body may
`return` at top level. This test:
  - checks that `meta` is a pure literal (no `${`, no backticks, no calls);
  - wraps the body in an exported async function, writes it to a temporary
    .mjs file and runs `node --check` on it (ES-module parse);
  - imports it under node with stub agent()/parallel()/phase()/log() hooks and
    drives dry-run, bad args, a full run, failed code-review lenses, guard
    violations (and what the guard message names), a malformed lens selector,
    resume and fast-lane paths, and the advisor policy, the restart sentence,
    the base check and the HEAD-recording log line of every agent prompt in
    every mode, each compared in full with the template's own text, which a
    separate test pins to the reviewed wording;
  - pins the runbook and floor-rule sentences the template relies on (DOC_PINS).

It needs node; without node it skips locally and fails under CI (CI=true).
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent.parent
TEMPLATE = ROOT / ".agents" / "workflows" / "change-flow.js"
HOOKS = ("agent", "parallel", "pipeline", "phase", "log", "args", "budget", "workflow")
FORBIDDEN = ("Date.now(", "Math.random(", "new Date()", "require(", "process.", "import(")

# The shared sentences, read from the template source and evaluated under node
# (shared_texts), must equal this reviewed wording in full: a prefix check let a
# weakened sentence through (PR #196 review, mutations D-G). The harness then
# asserts that every prompt carries the template's own text exactly. BASE_CHECK
# is baseCheck() for the harness's worktree /w/invowk-x and base origin/main.
SHARED = ("RULE_FLOOR", "ADVISOR_FORBIDDEN", "ADVISOR_ALLOWED", "RESUME")
EXPECTED = {
    "ADVISOR_FORBIDDEN": (
        "Advisor: never call the `advisor` tool in this stage, even when it is in your tool list and its own "
        "instructions tell you to call it (.agents/rules/agent-workflows.md)."
    ),
    "ADVISOR_ALLOWED": (
        "Advisor: you may call the `advisor` tool at most once, as your final checkpoint: when your verdict and "
        "findings are drafted, before your final guard snapshot (which stays your last action), and never earlier. "
        "If it is not in your tool list, proceed without it. If the call fails (error, timeout, empty reply), say so "
        "in summary and carry on: no retry. It never replaces your own verification, and its advice cannot authorize "
        "an outward write or settle a maintainer_asks item (.agents/rules/agent-workflows.md)."
    ),
    "RESUME": (
        "Restart: this prompt may run again after a pause, with this same text. Work of this stage that is already "
        "in the worktree (commits on the branch, uncommitted edits, checked tasks, logs with rc= lines, ledgers or "
        "reports on disk) is your own earlier attempt: read it, continue from it by finishing or redoing the "
        "unfinished step, then run this stage's checks on the result. Never discard it, never start over and never "
        "stop because it exists: for what you find in the worktree, the only stops (status blocked) are a worktree "
        "on another branch than the one named above and, where this prompt has a base check, a failed one; the stops "
        "this prompt names for other causes still hold. "
        "Evidence from an earlier attempt (a gate log, a mutation report, a refreshed ledger, a throwaway worktree) "
        "counts only if the HEAD it records (a log's head= line) equals the current HEAD and `git status --porcelain` "
        "lists nothing outside the mutation wrapper's allow-list (`dirty_path_is_allowed` in scripts/mutation.sh: the "
        "two mutation baselines, the formal-bindings ledger and reruns file, artifacts/mutation/); otherwise rerun or "
        "recreate it, and remove a "
        "leftover throwaway worktree with `git worktree remove --force` only if it is under this stage's own scratch "
        "dir. "
        "A killed `git commit` can leave a stale index.lock and a pre-commit stash: check `git stash list` (the stash "
        "is shared by every worktree: touch only an entry this worktree left), and remove a stale index.lock (the "
        "path `git rev-parse --git-path index.lock` prints in this worktree) only when no git process runs in this "
        "worktree. "
        "Keep decisions and progress on disk as you go (tasks.md checkboxes, design.md, logs with rc= lines, the "
        "commits you make anyway), not only in your context; this asks for no extra commit."
    ),
    "BASE_CHECK": (
        "Right after your first guard snapshot: `git -C /w/invowk-x fetch --prune origin`, then "
        "`git -C /w/invowk-x merge-base --is-ancestor origin/main HEAD`. Exit 0: continue. "
        "Exit 1 means origin/main has commits the branch lacks, which blocks only a stale start with no work on it: "
        "run `git -C /w/invowk-x merge-base origin/main HEAD` as its own command, never inside `$(...)` in a "
        "`rev-list` range. "
        "When it exits non-zero or prints nothing, change nothing and return status blocked with the reason "
        "\"no shared history with origin/main\" (still report guard.after). "
        "Otherwise run `git -C /w/invowk-x rev-list --count <that merge base>..HEAD` and "
        "`git -C /w/invowk-x status --porcelain`. "
        "When the count is 0 and the status prints nothing, change nothing and return status blocked with the reason "
        "\"branch not rebased on origin/main\" (still report guard.after). "
        "Otherwise (commits beyond the merge base, or uncommitted edits) continue: the fix stage rebases on "
        "origin/main before its gates. "
        "Any other non-zero exit of `--is-ancestor` (for example 128, a missing ref) and any failure of the later "
        "commands is a failure: return status blocked with its stderr. "
    ),
}

# The runbook and floor-rule facts the template relies on (PR #196 review, second
# round): each present sentence must appear verbatim, each removed one never.
DOC_PINS = {
    ".agents/runbooks/change-flow.md": (
        (
            "1. No rebase is in progress: neither `git -C <worktree> rev-parse --path-format=absolute --git-path "
            "rebase-merge` nor the same with `rebase-apply` prints a path that exists. Otherwise finish the rebase "
            "(`git -C <worktree> rebase --continue`) or abort it first; `--abbrev-ref HEAD` printing `HEAD` is not the "
            "test, since a detached HEAD prints it too.",
            "3. Once 1 and 2 pass, update the branch at launch only in the one case the stages' base check blocks "
            "(stage 1 below): no commit beyond `git merge-base <base> HEAD` and an empty `git status --porcelain`. That "
            "is a fast-forward: `git -C <worktree> fetch --prune origin`, then `git -C <worktree> merge --ff-only "
            "<base>`. A branch with work is not rebased at launch: the fix stage rebases it under its lease.",
            "`blocked_at` is `\"after <stage>\"`, naming the last stage that passed its guard, or `\"start\"` when none "
            "did,",
            "- `spec-review`, `spec-review-2`, `spec-fix` in `new` or `revise`: `revise`, with the review's findings in "
            "`notes`; the result carries no findings list, only the reviewer's summary (in `reason` and `stages`).",
            "- `spec-review`, `spec-review-2`, `spec-fix` in a `fast-lane` run (or a `continue` with `lane: \"fast\"`), "
            "where they run after `lenses`: `continue` from `review` with `lane: \"fast\"`, which runs that spec review "
            "again while the change is not fast-lane eligible; when the reason asks for `continue` from `implement`, "
            "use that, with `lane: \"fast\"`.",
            "When the stopped run was itself `finalize` or `continue` from `fix`, its `review` field is null: pass the "
            "original `review_file` again.",
            "The spec stage refuses two cases on the base: the change exists there (`blocked`, \"change exists on the "
            "base: use revise\"), since a colliding id is someone else's change, never an earlier attempt; or the "
            "base's `openspec/changes/archive/` has an entry named exactly `YYYY-MM-DD-<change>` (`blocked`, \"change "
            "already archived on the base: pick a new id\"), since a leftover branch was already squash-merged.",
            "a later run that reads `reviewed_head` from `review_file` takes the whole change when it is none, empty or "
            "not a hex SHA",
            "counts only if the HEAD it records is the current one and `git status --porcelain` lists nothing outside "
            "the mutation wrapper's allow-list (`dirty_path_is_allowed` in `scripts/mutation.sh`)",
            "names that commit by its full 40-hex SHA in `notes` (the prompt's \"Extra brief from the maintainer "
            "session\" line)",
        ),
        (
            "--autostash",
            "During an interrupted rebase it prints",
            "names the stage before it",
            "the maintainer brief names",
            "the main session rebases at launch",
            "no uncommitted edit besides",
        ),
    ),
    ".agents/rules/agent-workflows.md": (
        (
            "counts only at the HEAD it recorded, on a tree whose `git status --porcelain` lists nothing outside the "
            "mutation wrapper's allow-list (`dirty_path_is_allowed` in `scripts/mutation.sh`)",
        ),
        ("no other uncommitted edit",),
    ),
}

HARNESS = r"""
import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
const mod = await import(process.argv[2])
const run = mod.default
const meta = mod.meta
// The shared sentences and baseCheck() text, read from the template source
// (shared_texts in the Python test, which also pins their wording).
const TEXT = JSON.parse(readFileSync(process.argv[3], 'utf8'))

const snap = { status: '?? a.log', branch: 'main', head: 'abc123' }
const guard = { before: snap, after: { ...snap } }
const good = { mode: 'new', change: 'add-thing', worktree: '/w/invowk-x', branch: 'feat/add-thing', primary: '/w/invowk' }

const codeReview = (body, status) => 'CODE-REVIEW-BEGIN\n' + body + '\nCODE-REVIEW-END\nCODE-REVIEW-STATUS: ' + status + '\nGUARD-JSON: ' + JSON.stringify(guard)

function stubs(overrides) {
  const calls = []
  const phases = []
  const logs = []
  const agent = async (prompt, opts) => {
    const label = (opts && opts.label) || '?'
    calls.push({ label, prompt, schema: !!(opts && opts.schema), agentType: opts && opts.agentType })
    if (overrides[label]) return overrides[label](prompt, opts)
    if (label === 'review:code-review') return codeReview('1. a.go:3 nil deref', 'ok')
    if (label === 'lenses') return { exit_code: 0, stdout: '{"lenses":["ci"],"fast_lane_eligible":true,"reasons":{"ci":["Makefile"]}}', guard }
    if (label.startsWith('review:') || label === 'spec-review' || label === 'spec-review-2') return { blocking: false, findings: [], summary: 'ok', guard }
    if (label === 'fix') return { status: 'done', base_sha: 'b1', head_sha: 'h2', pr: '7', gates: ['make test'], rejected: [], maintainer_asks: [], summary: 'ok', changed_files: ['internal/x.go'], guard }
    return { status: 'done', summary: label + ' ok', head_sha: 'h1', pr: '7', guard }
  }
  const parallel = async (thunks) => Promise.all(thunks.map((t) => t().catch(() => null)))
  const pipeline = async () => { throw new Error('pipeline is not used') }
  return { calls, phases, logs, hooks: [agent, parallel, pipeline, (t) => phases.push(t), (m) => logs.push(m)] }
}

// Advisor policy (.agents/rules/agent-workflows.md, "Advisor"): only the spec
// reviewer (every round) and the base reviewer may call the advisor, once;
// every other agent prompt forbids it. go() checks every agent of every run.
const ADVISOR_MAY = ['spec-review', 'spec-review-2', 'review:base']
const count = (text, needle) => text.split(needle).length - 1
const advisorChecked = new Set()

// Each prompt carries the full sentence exactly once, never a prefix of it.
function checkAdvisor(calls, name) {
  for (const c of calls) {
    const may = ADVISOR_MAY.includes(c.label)
    assert.equal(count(c.prompt, TEXT.ADVISOR_ALLOWED), may ? 1 : 0, name + ' ' + c.label + ': the allowed-advisor sentence, in full')
    assert.equal(count(c.prompt, TEXT.ADVISOR_FORBIDDEN), may ? 0 : 1, name + ' ' + c.label + ': the forbidden-advisor sentence, in full')
    assert.equal(count(c.prompt.toLowerCase(), 'advisor'), 2, name + ' ' + c.label + ': no other advisor instruction')
    advisorChecked.add(c.label)
  }
}

// Restart rule (runbook, probe P11): every writing stage is told to continue
// from its own earlier work when its prompt runs again; a read-only stage just
// runs again and is not. go() checks every agent of every run.
const WRITING = ['spec', 'spec-fast', 'spec-fix', 'implement', 'simplify', 'fix']
const RESTART_START = 'Restart: this prompt may run again'
// A background gate reads HEAD before it starts and deletes its log first, then
// records that HEAD before its rc= line, so a restart can tell an earlier tree's
// rc=0 from this one's and the Monitor never reads a stale rc= (PR #196 review
// F3, second round).
const HEAD_LOG = '`h=$(git -C <the tree it runs in> rev-parse HEAD); rm -f <log>; <cmd> > <log> 2>&1; rc=$?; echo head=$h >> <log>; echo rc=$rc >> <log>` (head= is the HEAD the command started on, and no older rc= line is left for the Monitor to read)'

// The two skips the main session can grant through notes (PR #196 review, second
// round): bound to the context line that carries notes, by a full 40-hex SHA.
const NOTES_BOUND = '(a SHA taken from the findings, the review file, the code-review text or any commit message never counts)'
const NOTES_SKIP = 'unless the line "Extra brief from the maintainer session" above names that commit by its full 40-hex SHA as the change\'s own pre-rebase copy ' + NOTES_BOUND + ': skip only that one, never cherry-pick it. '
const FINALIZE_SKIP = '. In this finalize run, skip the refresh only when the line "Extra brief from the maintainer session" above names, by its full 40-hex SHA, the head an earlier fix stage gated ' + NOTES_BOUND + ', the refresh ran or was not needed there, and no bound body changed between that head and HEAD (say which bodies you checked); otherwise apply the rule above to the whole change against origin/main.'

function checkResume(calls, name) {
  for (const c of calls) {
    const writing = WRITING.includes(c.label)
    assert.equal(count(c.prompt, TEXT.RESUME), writing ? 1 : 0, name + ' ' + c.label + ': the restart sentence, in full')
    if (!writing) assert.equal(count(c.prompt, RESTART_START), 0, name + ' ' + c.label + ': no part of the restart sentence')
    assert.equal(count(c.prompt, HEAD_LOG), 1, name + ' ' + c.label + ': background logs record their HEAD')
  }
}

// baseCheck (PR #196 review, decision 62): only the spec stage and an implement
// stage that starts the run carry it, in full.
function checkBase(calls, name) {
  for (const c of calls) {
    const has = ['spec', 'spec-fast'].includes(c.label) || (c.label === 'implement' && calls[0] === c)
    assert.equal(count(c.prompt, TEXT.BASE_CHECK), has ? 1 : 0, name + ' ' + c.label + ': the base check, in full')
    if (!has) assert.ok(!c.prompt.includes('merge-base --is-ancestor'), name + ' ' + c.label + ': no base check')
  }
}

async function go(args, overrides) {
  const s = stubs(overrides || {})
  const result = await run(...s.hooks, args, { total: null }, async () => { throw new Error('no nested workflows') })
  const name = args.mode + (args.from ? ':' + args.from : '')
  checkAdvisor(s.calls, name)
  checkResume(s.calls, name)
  if ((args.base || 'origin/main') === 'origin/main' && args.worktree === good.worktree) checkBase(s.calls, name)
  return { result, ...s }
}

const titles = meta.phases.map((p) => p.title)
assert.equal(meta.name, 'invowk-change-flow')

// dry-run: self-checks only, no agent.
{
  const { result, calls } = await go({ mode: 'dry-run' })
  assert.equal(result.status, 'done')
  assert.equal(calls.length, 0)
  assert.deepEqual(result.checks, ['args', 'plan', 'lenses', 'guard', 'review'])
}

// bad args throw before any agent.
for (const bad of [undefined, { mode: 'ship-it' }, { ...good, worktree: 'rel' }, { ...good, branch: 'main' }, { ...good, mode: 'continue' }]) {
  const s = stubs({})
  await assert.rejects(run(...s.hooks, bad, { total: null }, null))
  assert.equal(s.calls.length, 0)
}

// full new-mode run.
{
  const { result, calls, phases } = await go(good)
  assert.equal(result.status, 'done', JSON.stringify(result))
  const order = calls.map((c) => c.label)
  assert.deepEqual(order.slice(0, 5), ['spec', 'spec-review', 'implement', 'simplify', 'lenses'])
  assert.equal(order[order.length - 1], 'fix')
  assert.ok(order.includes('review:base') && order.includes('review:code-review') && order.includes('review:ci'))
  assert.ok(!order.includes('review:formal'))
  for (const p of phases) assert.ok(titles.includes(p), 'phase without meta entry: ' + p)
  const cr = calls.find((c) => c.label === 'review:code-review')
  assert.equal(cr.schema, false, 'code-review lens must run without a schema')
  assert.ok(cr.prompt.includes('"high 7"'), 'code-review gets an explicit target and level')
  assert.ok(!/--fix|--comment/.test(cr.prompt.replace(/never --fix, never --comment/, '')))
  assert.ok(calls.every((c) => c.agentType === undefined), 'agent files are named in prompts, never agentType')
  assert.ok(calls.every((c) => c.prompt.includes('/w/invowk status --porcelain')), 'every stage gets the guard')
  const fix = calls.find((c) => c.label === 'fix')
  assert.ok(fix.prompt.includes('a.go:3 nil deref'), 'code-review text reaches the fix agent')
  assert.equal(result.base_sha, 'b1')
  assert.deepEqual(result.changed_files, ['internal/x.go'])
  assert.ok(calls.find((c) => c.label === 'spec').prompt.includes(TEXT.BASE_CHECK), 'spec refuses a stale start')
  assert.ok(!calls.find((c) => c.label === 'implement').prompt.includes('merge-base --is-ancestor'), 'implement after spec in the same run does not re-check the base')
  const fixP = calls.find((c) => c.label === 'fix').prompt
  assert.ok(fixP.includes('/w/invowk/scripts/change-flow-foreign-commits.sh /w/invowk-x'), 'the helper comes from the primary checkout')
  assert.ok(fixP.includes('Run checklist steps 2-10') && fixP.includes('Skip step 1 (/simplify)') && fixP.includes('Step 11 (/learn) runs in step 4'))
  assert.ok(fixP.includes('the change is already archived'))
  assert.ok(fixP.includes('never edit the change-flow tooling'))
  assert.ok(fixP.includes('Return changed_files'))
  assert.ok(!fixP.includes('In this finalize run'), 'the finalize skip clause appears only in finalize runs')
  assert.ok(fixP.includes('make mutation-formal-refresh MUTATION_FORMAL_SYMBOL='), 'the fix stage refreshes the formal-bindings ledger')
  assert.ok(fixP.includes('or `MUTATION_FORMAL_STALE=1` for every symbol whose entries fail the anchor check'), 'the stale-symbol refresh is offered')
  assert.ok(fixP.includes('`FILE:OLD` (removal-only) and the new name in the same MUTATION_FORMAL_SYMBOL'), 'renamed or deleted bound functions')
  assert.ok(fixP.includes('The only classes this stage may write are `equivalent` and a deferred `binding-gap` or `model-gap` with a `follow_up`'), 'allowed triage classes')
  assert.ok(fixP.includes('(a closed gap, `abstraction`, `defect`) would change the plan-inputs digest after the review: put it in maintainer_asks and return status blocked'), 'digest-changing classes block')
  assert.ok(fixP.includes('commit the review fixes and the fixes of checklist steps 2-3 (tidy, lint) first, then run the mutation and the refresh'), 'refresh after the lint and tidy fixes')
  assert.ok(fixP.includes('if a later gate makes you edit a bound function body, redo both on the new HEAD'), 'a later bound-body edit redoes the refresh')
  assert.ok(fixP.includes('on the FINAL tree after the review fixes are committed'), 'the refresh runs on the final tree')
  assert.ok(fixP.includes('`make mutation-formal-rerun MUTATION_MUTANT_ID=...` line it printed twice'), 'two reruns per new escapee')
  assert.ok(fixP.includes("first commit every tracked edit outside the wrapper's allow-list") && !fixP.includes('commit first if it changed formal/manifest.toml'), 'the reruns need every tracked edit outside the allow-list committed')
  assert.ok(fixP.includes('commit the baseline, the ledger and the reruns'))
  assert.ok(fixP.includes('A survivor you cannot classify goes into maintainer_asks') && fixP.includes('return status blocked'))
  assert.ok(!fixP.includes('report-only, in a throwaway worktree, ONLY'), 'no report-only-only mutation run')
  {
    // lesson (#186): check the formal-bindings plan before the 35-minute run.
    const planAt = fixP.indexOf('python3 scripts/formal_mutation.py plan --out')
    const runAt = fixP.indexOf('Run `make mutation-formal` once')
    assert.ok(planAt >= 0 && runAt >= 0 && planAt < runAt, 'the plan check comes before make mutation-formal')
    const symbolsAt = fixP.indexOf('python3 scripts/formal_mutation.py symbols --plan')
    assert.ok(planAt < symbolsAt && symbolsAt < runAt, 'the symbols check reads the plan before make mutation-formal')
    assert.ok(!fixP.includes('jq -r'), 'no jq or by-eye matching of the plan')
    assert.ok(fixP.includes('never decide by eye or with jq'), 'the symbols subcommand decides')
    // a failed plan is not "nothing planned".
    assert.ok(fixP.includes('--out <scratch dir>` must exit 0') && fixP.includes('never means "nothing planned"'), 'the plan must exit 0')
    assert.ok(fixP.includes('put it in maintainer_asks and return status blocked. '), 'a plan failure blocks unless fixed')
    // renamed or deleted bound functions are removal-only, never a skip.
    assert.ok(fixP.includes('plus `FILE:OLD` for each renamed or deleted bound function'), 'renames and deletes are checked')
    assert.ok(fixP.includes('`removal-only` (a renamed or deleted function'), 'a rename or delete is removal-only')
    assert.ok(fixP.includes('Only when it prints `-> skip`, skip `make mutation-formal`'), 'only an all-unplanned list skips the run')
    assert.ok(fixP.includes('Otherwise refresh the planned and removal-only symbols'), 'removal-only symbols are refreshed')
    assert.ok(fixP.includes('name in summary every symbol you checked'), 'the skip names the symbols checked')
    // a failing triage --check after a skip goes back to the refresh or blocks.
    assert.ok(fixP.includes('if that `triage --check` fails, go back to the refresh path below, or put the failure in maintainer_asks and return status blocked'), 'a triage failure after a skip is handled')
  }
  // lesson: the pre-commit hooks take about 5 minutes; every committing stage says so.
  for (const label of ['spec', 'implement', 'simplify', 'fix']) {
    const p = calls.find((c) => c.label === label).prompt
    assert.ok(p.includes('Run every `git commit` under the lock in the background with an rc= line'), label + ' gets the commit timeout')
    assert.ok(p.includes('never in the foreground, not even with a 600000 ms timeout') && p.includes('waiting for the lock counts'), label + ' backgrounds a locked commit')
  }
  for (const label of ['spec-review', 'lenses', 'review:base', 'review:code-review']) {
    assert.ok(!calls.find((c) => c.label === label).prompt.includes('Run every `git commit`'), label + ' does not commit')
  }
  assert.deepEqual(result.failed_lenses, [])
  assert.ok(result.next.startsWith('ready for maintainer merge'))
}

// a failed code-review lens is logged and reported, never counted as zero findings:
// skill boilerplate, an explained failure, bare "(none)." / "No findings.", a
// missing status line, or an agent that returned nothing.
for (const [name, reply] of Object.entries({
  'skill-only': codeReview('Skill execution completed', 'ok'),
  'explained-failure': codeReview('The skill returned only "Skill execution completed"; nothing usable.', 'failed'),
  'explained-failure-no-phrase': codeReview('Nothing usable: the fork returned no output.', 'failed'),
  'none-period': codeReview('(none).', 'ok'),
  'no-findings': codeReview('No findings.', 'ok'),
  'no-status': 'CODE-REVIEW-BEGIN\n1. a.go:3 nil deref\nCODE-REVIEW-END\nGUARD-JSON: ' + JSON.stringify(guard),
  'agent-died': null,
})) {
  const { result, logs, calls } = await go(good, { 'review:code-review': () => reply })
  assert.equal(result.status, 'done', name + ': ' + JSON.stringify(result))
  assert.deepEqual(result.failed_lenses, ['code-review'], name)
  assert.ok(logs.some((l) => l.includes('code-review lens FAILED')), name)
  const fix = calls.find((c) => c.label === 'fix')
  assert.ok(fix.prompt.includes('the code-review lens FAILED'), name)
  assert.ok(!fix.prompt.includes('nothing usable'), name)
}

// every stage forbids ending a turn with work running; the fix agent commits
// confirmed fixes before the long gates and checks findings on the branch; a
// base other than origin/main makes it drop findings from the base's commits.
{
  const { calls } = await go(good)
  assert.ok(calls.every((c) => c.prompt.includes('Never end your turn')), 'every stage forbids returning with work running')
  const fix = calls.find((c) => c.label === 'fix')
  assert.ok(fix.prompt.includes('BEFORE the long gates'))
  assert.ok(fix.prompt.includes('show HEAD:<file>'))
  assert.ok(!fix.prompt.includes('drop findings that belong to'), 'default base adds no base-commit note')
  const other = await go({ ...good, base: 'feat/base-change' })
  const fix2 = other.calls.find((c) => c.label === 'fix')
  assert.ok(fix2.prompt.includes("drop findings that belong to feat/base-change's own commits"))
}

// fast-lane may propose a new change; an agent's pr URL becomes the number and a
// junk pr never becomes the code-review target; the fix agent integrates others'
// commits before using the fetched tip as its lease.
{
  const fast = await go({ ...good, mode: 'fast-lane' })
  const spec = fast.calls.find((c) => c.label === 'spec-fast')
  assert.ok(spec.prompt.includes('openspec-propose if openspec/changes/add-thing/ does not exist'))
  const url = await go(good, { implement: () => ({ status: 'done', summary: 'ok', head_sha: 'h1', pr: 'https://github.com/invowk/invowk/pull/176', guard }), simplify: () => ({ status: 'done', summary: 'ok', guard }) })
  assert.ok(url.calls.find((c) => c.label === 'review:code-review').prompt.includes('"high 176"'))
  const junk = await go(good, { spec: () => ({ status: 'done', summary: 'ok', guard }), implement: () => ({ status: 'done', summary: 'ok', head_sha: 'h1', pr: 'see above', guard }), simplify: () => ({ status: 'done', summary: 'ok', guard }) })
  assert.ok(junk.calls.find((c) => c.label === 'review:code-review').prompt.includes('"high feat/add-thing"'))
  const fix = url.calls.find((c) => c.label === 'fix')
  assert.ok(fix.prompt.includes('scripts/change-flow-foreign-commits.sh /w/invowk-x feat/add-thing origin/main'))
  assert.ok(fix.prompt.includes('fetch --prune origin'))
  assert.ok(fix.prompt.includes('If it exits 3'))
  assert.ok(fix.prompt.includes('Any exit other than 0 or 3 is a failure'))
  assert.ok(fix.prompt.includes('Ancestry is not the test'))
  assert.ok(fix.prompt.includes('Monitor tool'), 'long gates wait with Monitor')
  assert.ok(fix.prompt.includes('never a bare force'))
}

// a code-review reply without its guard still stops the run.
{
  const { result } = await go(good, { 'review:code-review': () => 'CODE-REVIEW-BEGIN\n1. x\nCODE-REVIEW-END\nCODE-REVIEW-STATUS: ok\n' })
  assert.equal(result.status, 'blocked')
  assert.match(result.reason, /review:code-review/)
}

// a guard difference stops the run at that stage, and the reason says what
// changed: the porcelain lines, HEAD and the branch, not only the field names.
{
  const { result, calls } = await go(good, {
    simplify: () => ({ status: 'done', summary: 'x', guard: { before: snap, after: { ...snap, status: ' M internal/x.go' } } }),
  })
  assert.equal(result.status, 'blocked')
  assert.match(result.reason, /primary checkout changed \(status\)/)
  assert.ok(result.reason.includes('simplify: primary checkout changed (status): appeared: " M internal/x.go"; disappeared: "?? a.log"'), result.reason)
  assert.equal(calls[calls.length - 1].label, 'simplify')
  const moved = await go(good, {
    implement: () => ({ status: 'done', summary: 'x', guard: { before: snap, after: { status: snap.status, branch: 'feat/other', head: 'def4567890123' } } }),
  })
  assert.ok(moved.result.reason.includes('implement: primary checkout changed (branch, head): HEAD abc123 -> def45678; branch main -> feat/other'), moved.result.reason)
  // two heads with the same 8-character prefix are printed in full.
  const twin = await go(good, {
    implement: () => ({ status: 'done', summary: 'x', guard: { before: { ...snap, head: 'abcdef0123' }, after: { ...snap, head: 'abcdef0199' } } }),
  })
  assert.ok(twin.result.reason.includes('implement: primary checkout changed (head): HEAD abcdef0123 -> abcdef0199'), twin.result.reason)
}

// a difference between one stage's last record and the next stage's first is
// logged with the same detail and does not stop the run.
{
  const pulled = { ...snap, head: 'fed9876543210' }
  const { result, logs } = await go(good, {
    simplify: () => ({ status: 'done', summary: 'x', guard: { before: pulled, after: { ...pulled } } }),
  })
  assert.equal(result.status, 'done', JSON.stringify(result))
  assert.ok(logs.includes('note: primary checkout changed between stages, before simplify, not by a stage (head): HEAD abc123 -> fed98765'), logs.join('\n'))
}

// a restarted spec stage finds the change its earlier attempt created: new and
// fast-lane continue it with openspec-update-change instead of proposing again.
{
  const specOf = async (mode, label) => (await go({ ...good, mode })).calls.find((c) => c.label === label).prompt
  const fresh = await specOf('new', 'spec')
  assert.ok(fresh.includes('Skill openspec-propose if openspec/changes/add-thing/ does not exist in the worktree yet, otherwise Skill openspec-update-change (an earlier attempt of this stage created it'))
  assert.ok((await specOf('fast-lane', 'spec-fast')).includes('does not exist in the worktree yet, otherwise Skill openspec-update-change,'))
  const revise = await specOf('revise', 'spec')
  assert.ok(revise.includes('Invoke Skill openspec-update-change for change') && !revise.includes('openspec-propose'))
}

// continue from implement: implement starts the run, so it checks the base.
// The check blocks only a stale start with no work (no commit beyond the merge
// base, clean status), so a restart after the base moved continues (decision 62).
{
  const { calls } = await go({ ...good, mode: 'continue', from: 'implement', pr: 9 })
  assert.equal(calls[0].label, 'implement')
  assert.equal(count(calls[0].prompt, TEXT.BASE_CHECK), 1)
  assert.ok(TEXT.BASE_CHECK.includes('rev-list --count <that merge base>..HEAD') && TEXT.BASE_CHECK.includes('When the count is 0 and the status prints nothing'))
  assert.ok(TEXT.BASE_CHECK.includes('Otherwise (commits beyond the merge base, or uncommitted edits) continue'))
  assert.ok(!TEXT.BASE_CHECK.includes('the branch is behind'), 'behind the base alone is no longer a stop')}

// new and fast-lane refuse a change that already exists on the base (a colliding
// id), before they treat the change directory as an earlier attempt; revise
// edits an existing change and does not test it (PR #196 review F1).
{
  const onBase = '`git -C /w/invowk-x cat-file -e origin/main:openspec/changes/add-thing`: exit 0 means the change already exists on origin/main'
  const archived = 'Then `git -C /w/invowk-x ls-tree --name-only origin/main openspec/changes/archive/`: when it lists `openspec/changes/archive/YYYY-MM-DD-add-thing` exactly (a date, a hyphen, then this id with nothing after it; a suffix match is not a match), the change was already merged and archived on origin/main: change nothing and return status blocked with the reason "change already archived on the base: pick a new id" (still report guard.after). '
  for (const [mode, label] of [['new', 'spec'], ['fast-lane', 'spec-fast']]) {
    const p = (await go({ ...good, mode })).calls.find((c) => c.label === label).prompt
    assert.equal(count(p, onBase), 1, mode)
    assert.ok(p.includes('return status blocked with the reason "change exists on the base: use revise"'), mode)
    assert.ok(p.indexOf(TEXT.BASE_CHECK) < p.indexOf(onBase) && p.indexOf(onBase) < p.indexOf('openspec-propose'), mode + ': base check, then the base test, then the skill')
    // a leftover branch already squash-merged: its archived copy on the base, matched exactly (second round).
    assert.equal(count(p, archived), 1, mode + ': the archived-copy test, in full')
    assert.ok(p.indexOf(onBase) < p.indexOf(archived) && p.indexOf(archived) < p.indexOf('openspec-propose'), mode + ': the archive test before the skill')
  }
  const revise = (await go({ ...good, mode: 'revise' })).calls.find((c) => c.label === 'spec').prompt
  assert.ok(!revise.includes('cat-file -e') && !revise.includes('ls-tree'))
}

// the fix stage's step-1 stop on a changed own commit holds on a restart too
// (PR #196 review F2); the main session resolves it (runbook).
{
  const fixP = (await go(good)).calls.find((c) => c.label === 'fix').prompt
  assert.ok(fixP.includes('That stop holds on a restart too: an earlier attempt\'s pre-rebase commit is listed the same way, only the main session can tell it from a foreign one, and the restart rule below does not lift it.'))
  // the way out is bound to the notes line, by a full SHA, never one from the findings or a commit (second round).
  assert.ok(fixP.includes(NOTES_SKIP), 'the main session\'s way out')
  assert.ok(!fixP.includes('the maintainer brief names'), 'no unbound "maintainer brief"')
}

// a restarted fix stage that cannot tell its commits apart returns a superset of
// changed_files, derived from the reviewed head when the run knows it (F4).
{
  const sha = 'abcdef0123456789abcdef0123456789abcdef01'
  const { result, calls } = await go(good, { simplify: () => ({ status: 'done', summary: 'ok', head_sha: sha, guard }) })
  const fixP = calls.find((c) => c.label === 'fix').prompt
  assert.ok(fixP.includes('If you cannot tell your own commits apart (a restart after an earlier attempt committed or rebased), return the superset instead, every file that differs from the reviewed head, `git -C /w/invowk-x diff --name-only ' + sha + ' HEAD`'), fixP)
  assert.ok(fixP.includes('say in summary that changed_files is that superset'))
  assert.equal(result.review.reviewed_head, sha)
  const junk = (await go(good, { simplify: () => ({ status: 'done', summary: 'ok', head_sha: 'see above', guard }), implement: () => ({ status: 'done', summary: 'ok', guard }) })).calls.find((c) => c.label === 'fix').prompt
  assert.ok(junk.includes('return the superset instead, every file of the change, `git -C /w/invowk-x diff --name-only origin/main...HEAD`'), 'a non-SHA head falls back to the whole change')
  const fin = (await go({ ...good, mode: 'finalize' })).calls[0].prompt
  assert.ok(fin.includes('return the superset instead, every file of the change, `git -C /w/invowk-x diff --name-only origin/main...HEAD`'))
  const file = (await go({ ...good, mode: 'finalize', review_file: '/w/review.json' })).calls[0].prompt
  // N3: a continue from review records reviewed_head '' (second round).
  assert.ok(file.includes('every file that differs from the `reviewed_head` of /w/review.json, `git -C /w/invowk-x diff --name-only <reviewed_head> HEAD`, or, when that field is none, empty or not a hex SHA, every file of the change, `git -C /w/invowk-x diff --name-only origin/main...HEAD`'))
  const fromReview = await go({ ...good, mode: 'continue', from: 'review' })
  assert.equal(fromReview.result.review.reviewed_head, '', 'a continue from review has no earlier head')
}

// blocked-stage plumbing: a spec or implement stage that returns blocked (for
// example on a stale base) ends the run there. The baseCheck prompt text itself
// is asserted in the main run above.
for (const stage of ['spec', 'implement']) {
  const { result, calls } = await go(good, {
    [stage]: () => ({ status: 'blocked', summary: 'branch not rebased on origin/main', guard: { before: snap, after: snap } }),
  })
  assert.equal(result.status, 'blocked')
  assert.equal(result.blocked_at, stage)
  assert.match(result.reason, /not rebased/)
  assert.equal(calls[calls.length - 1].label, stage, stage + ' is the last stage that ran')
}

// a missing guard also stops the run.
{
  const { result } = await go(good, { implement: () => ({ status: 'done', summary: 'x' }) })
  assert.equal(result.status, 'blocked')
  assert.match(result.reason, /missing or malformed guard/)
}

// a malformed selector selects every lens.
{
  const { result, calls, logs } = await go({ ...good, mode: 'continue', from: 'review' }, {
    lenses: () => ({ exit_code: 0, stdout: '{"lenses":["vibes"]}', guard }),
  })
  assert.equal(result.status, 'done')
  const labels = calls.map((c) => c.label)
  for (const id of ['formal', 'windows', 'schema', 'ci', 'security', 'supply-chain', 'perf', 'architecture']) assert.ok(labels.includes('review:' + id), id)
  assert.ok(logs.some((l) => l.includes('running ALL lenses')))
}

// fast lane that turns out ineligible gets the independent spec review.
{
  const { calls } = await go({ ...good, mode: 'fast-lane' }, {
    lenses: () => ({ exit_code: 0, stdout: '{"lenses":["formal"],"fast_lane_eligible":false,"reasons":{}}', guard }),
  })
  const labels = calls.map((c) => c.label)
  assert.equal(labels[0], 'spec-fast')
  assert.ok(labels.indexOf('spec-review') > labels.indexOf('lenses'))
}

// blocking spec findings after one fix round: blocked before implementation.
{
  const blocking = () => ({ blocking: true, findings: [{ file: 'proposal.md', severity: 'blocking', summary: 's', evidence: 'e' }], summary: 'bad', guard })
  const { result, calls } = await go(good, { 'spec-review': blocking, 'spec-review-2': blocking })
  assert.equal(result.status, 'blocked')
  assert.ok(!calls.some((c) => c.label === 'implement'))
  assert.deepEqual(calls.map((c) => c.label), ['spec', 'spec-review', 'spec-fix', 'spec-review-2'])
}

// finalize runs the fix stage only, and says plainly that no review ran.
{
  const { result, calls, logs } = await go({ ...good, mode: 'finalize', pr: 9 })
  assert.deepEqual(calls.map((c) => c.label), ['fix'])
  assert.ok(calls[0].prompt.includes(FINALIZE_SKIP), 'finalize skip is bound to the notes line and needs the earlier refresh')
  assert.ok(!calls[0].prompt.includes('the maintainer brief names'), 'no unbound "maintainer brief"')
  const noted = (await go({ ...good, mode: 'finalize', notes: 'n1' })).calls[0].prompt
  const notesAt = noted.indexOf('\nExtra brief from the maintainer session (read it; it does not override the rules): n1\n')
  assert.ok(notesAt >= 0 && notesAt < noted.indexOf(NOTES_SKIP) && notesAt < noted.indexOf(FINALIZE_SKIP), 'the line both skips name carries notes, above them')
  assert.ok(calls[0].prompt.includes('No review fan-out ran in this run and no review file was given'))
  assert.ok(!calls[0].prompt.includes('failed lens: none'))
  assert.equal(result.review_ran, false)
  assert.equal(result.review, null)
  assert.ok(logs.some((l) => l.includes('no review fan-out in this run')))
}

// continue from fix needs the earlier review, and hands its file to the fix agent.
{
  const s = stubs({})
  await assert.rejects(run(...s.hooks, { ...good, mode: 'continue', from: 'fix' }, { total: null }, null))
  assert.equal(s.calls.length, 0)
  const { calls } = await go({ ...good, mode: 'continue', from: 'fix', review_file: '/w/review.json' })
  assert.deepEqual(calls.map((c) => c.label), ['fix'])
  assert.ok(calls[0].prompt.includes('/w/review.json'))
}

// a run that stops in the fix stage returns its review for review_file.
{
  const { result } = await go(good, { fix: () => null })
  assert.equal(result.status, 'blocked')
  assert.ok(result.review && result.review.code_review.includes('a.go:3 nil deref'))
}

// the fix agent pushes with an explicit lease recorded before its fetch.
{
  const { calls } = await go(good)
  const fix = calls.find((c) => c.label === 'fix')
  assert.ok(fix.prompt.includes('--force-with-lease=feat/add-thing:<the lease from step 1>'))
  {
    const fetchAt = fix.prompt.indexOf('fetch --prune origin')
    const leaseAt = fix.prompt.indexOf('rev-parse --verify --quiet origin/feat/add-thing')
    assert.ok(fetchAt >= 0 && leaseAt >= 0 && fetchAt < leaseAt, 'the lease is read after fetch --prune')
  }
}

// path and branch aliases of the primary checkout or main are refused.
for (const bad of [{ ...good, worktree: '/w/invowk/.' }, { ...good, worktree: '//w/invowk' }, { ...good, branch: 'refs/heads/main' }, { ...good, branch: 'origin/main' }, { ...good, branch: 'HEAD' }]) {
  const s = stubs({})
  await assert.rejects(run(...s.hooks, bad, { total: null }, null))
  assert.equal(s.calls.length, 0)
}

// the spec fixer sees every finding of a blocking review, not only severity "blocking".
{
  let n = 0
  const review = () => (n++ === 0
    ? { blocking: true, findings: [{ file: 'design.md', severity: 'major', summary: 'untestable requirement', evidence: 'e' }], summary: 'one major gap', guard }
    : { blocking: false, findings: [], summary: 'ok', guard })
  const { calls } = await go(good, { 'spec-review': review, 'spec-review-2': review })
  const fix = calls.find((c) => c.label === 'spec-fix')
  assert.ok(fix.prompt.includes('untestable requirement') && fix.prompt.includes('one major gap'))
  assert.ok(fix.prompt.includes('Run every `git commit` under the lock in the background'), 'spec-fix gets the commit timeout')
}

// a fast-lane spec fixed after implementation stops for re-implementation.
{
  const blockingOnce = () => ({ blocking: true, findings: [{ file: 'spec.md', severity: 'blocking', summary: 's', evidence: 'e' }], summary: 'gap', guard })
  const { result, calls } = await go({ ...good, mode: 'fast-lane' }, {
    lenses: () => ({ exit_code: 0, stdout: '{"lenses":[],"fast_lane_eligible":false,"reasons":{}}', guard }),
    'spec-review': blockingOnce,
  })
  assert.equal(result.status, 'blocked')
  assert.match(result.reason, /from implement/)
  assert.ok(!calls.some((c) => c.label.startsWith('review:')))
}

// continuing a fast-lane run (lane fast) keeps the post-selector spec review.
{
  const ineligible = () => ({ exit_code: 0, stdout: '{"lenses":[],"fast_lane_eligible":false,"reasons":{}}', guard })
  const fast = await go({ ...good, mode: 'continue', from: 'simplify', lane: 'fast' }, { lenses: ineligible })
  assert.ok(fast.calls.some((c) => c.label === 'spec-review'))
  const plain = await go({ ...good, mode: 'continue', from: 'simplify' }, { lenses: ineligible })
  assert.ok(!plain.calls.some((c) => c.label === 'spec-review'))
}

// advisor policy, mode by mode (go() has checked each prompt): the stages each
// mode runs, which of them may call the advisor, and that every agent label the
// template can spawn was checked at least once.
{
  const ineligible = () => ({ exit_code: 0, stdout: '{"lenses":[],"fast_lane_eligible":false,"reasons":{}}', guard })
  const everyLens = () => ({ exit_code: 0, stdout: '{"lenses":["vibes"]}', guard })
  const blockingOnce = () => {
    let n = 0
    return () => ({ blocking: n++ === 0, findings: [{ file: 'spec.md', severity: 'blocking', summary: 's', evidence: 'e' }], summary: 'gap', guard })
  }
  const may = async (args, overrides) => (await go(args, overrides)).calls.map((c) => c.label).filter((l) => ADVISOR_MAY.includes(l))
  for (const mode of ['new', 'revise']) {
    assert.deepEqual(await may({ ...good, mode }), ['spec-review', 'review:base'], mode)
    const review = blockingOnce()
    assert.deepEqual(await may({ ...good, mode }, { 'spec-review': review, 'spec-review-2': review }), ['spec-review', 'spec-review-2', 'review:base'], mode + ' with a fix round')
  }
  // fast-lane: the spec agent self-reviews and may not call it; the independent
  // spec review that an ineligible change gets may.
  const fast = await go({ ...good, mode: 'fast-lane' })
  assert.ok(fast.calls.find((c) => c.label === 'spec-fast').prompt.includes('review them yourself'))
  assert.deepEqual(fast.calls.map((c) => c.label).filter((l) => ADVISOR_MAY.includes(l)), ['review:base'])
  assert.deepEqual(await may({ ...good, mode: 'fast-lane' }, { lenses: ineligible }), ['spec-review', 'review:base'])
  assert.deepEqual(await may({ ...good, mode: 'fast-lane' }, { lenses: ineligible, 'spec-review': blockingOnce() }), ['spec-review', 'spec-review-2'])
  assert.deepEqual(await may({ ...good, mode: 'continue', from: 'simplify', lane: 'fast' }, { lenses: ineligible }), ['spec-review', 'review:base'])
  for (const from of ['implement', 'simplify', 'review']) assert.deepEqual(await may({ ...good, mode: 'continue', from }, { lenses: everyLens }), ['review:base'], from)
  assert.deepEqual(await may({ ...good, mode: 'continue', from: 'fix', review_file: '/w/review.json' }), [])
  assert.deepEqual(await may({ ...good, mode: 'finalize' }), [])
  assert.deepEqual(await may({ ...good, mode: 'finalize', review_file: '/w/review.json' }), [])
  const every = ['spec', 'spec-fast', 'spec-review', 'spec-fix', 'spec-review-2', 'implement', 'simplify', 'lenses', 'review:base', 'review:code-review',
    ...['formal', 'windows', 'schema', 'ci', 'security', 'supply-chain', 'perf', 'architecture'].map((id) => 'review:' + id), 'fix']
  assert.deepEqual([...advisorChecked].sort(), every.sort(), 'every agent label was checked for the advisor policy')
}
console.log('change-flow template harness: ok')
"""


def split_template(text: str) -> tuple[str, str]:
    """Return (meta block, body); meta ends at the first line that is exactly '}'."""
    lines = text.splitlines()
    if not lines or lines[0] != "export const meta = {":
        raise AssertionError("the template must start with `export const meta = {`")
    end = lines.index("}")
    return "\n".join(lines[: end + 1]), "\n".join(lines[end + 1 :])


def shared_source(text: str) -> str:
    """The template's own declarations of the shared sentences and of baseCheck(),
    as a node module that prints their values (baseCheck for the harness args)."""
    lines = text.splitlines()
    out = []
    for name in SHARED:
        starts = [i for i, line in enumerate(lines) if line.startswith(f"const {name} = ")]
        if len(starts) != 1:
            raise AssertionError(f"the template must declare {name} exactly once")
        end = starts[0]
        while lines[end].rstrip().endswith("+"):
            end += 1
        out += lines[starts[0] : end + 1]
    base_check = re.findall(r"(?ms)^function baseCheck\(a\) \{.*?^\}$", text)
    if len(base_check) != 1:
        raise AssertionError("the template must declare function baseCheck(a) exactly once")
    out.append(base_check[0])
    names = ", ".join(n for n in SHARED if n != "RULE_FLOOR")
    out.append("console.log(JSON.stringify({ " + names + ", BASE_CHECK: baseCheck({ worktree: '/w/invowk-x', base: 'origin/main' }) }))")
    return "\n".join(out) + "\n"


def shared_texts(node: str, text: str, tmp: str) -> tuple[dict[str, str], Path]:
    """Evaluate shared_source under node; return the texts and the JSON file holding them."""
    src = Path(tmp) / "shared.mjs"
    src.write_text(shared_source(text), encoding="utf-8")
    ran = subprocess.run([node, str(src)], capture_output=True, text=True, cwd=tmp)
    if ran.returncode != 0:
        raise AssertionError("evaluating the shared sentences failed: " + ran.stderr)
    out = Path(tmp) / "shared.json"
    out.write_text(ran.stdout, encoding="utf-8")
    return json.loads(ran.stdout), out


def need_node(test: unittest.TestCase) -> str:
    node = shutil.which("node")
    if node is None:
        if os.environ.get("CI"):
            test.fail("node is required in CI for the change-flow template self-test")
        test.skipTest("node not found locally: skipping the ES-module parse and harness (CI runs them)")
    return node


class TemplateTest(unittest.TestCase):
    text = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.text = TEMPLATE.read_text(encoding="utf-8")

    def test_meta_is_a_pure_literal(self) -> None:
        meta, _ = split_template(self.text)
        self.assertNotIn("${", meta)
        self.assertNotIn("`", meta)
        self.assertNotIn("...", meta)
        # With string literals removed, only keys, braces, brackets, commas and colons remain.
        bare = re.sub(r"'(?:[^'\\\n]|\\.)*'", "''", meta).replace("export const meta =", "", 1)
        self.assertNotIn("(", bare)
        self.assertNotIn('"', bare)
        for match in re.finditer(r"[A-Za-z_$][\w$]*", bare):
            following = bare[match.end() :].lstrip()
            self.assertTrue(following.startswith(":"), f"meta uses {match.group(0)!r} as a value")

    def test_body_avoids_nondeterminism_and_host_apis(self) -> None:
        _, body = split_template(self.text)
        for token in FORBIDDEN:
            self.assertNotIn(token, body, f"template body uses {token}")
        self.assertNotRegex(body, r"(?m)^\s*import\s")

    def test_no_session_paths(self) -> None:
        self.assertNotRegex(self.text, r"/var/home/|/home/[a-z]|/tmp/claude|/Users/")

    def test_every_prompt_builder_decides_the_advisor(self) -> None:
        # A builder added later is forbidden by default: it must carry one of the
        # two constants, and only the spec reviewer and the base reviewer may
        # carry the allowed one (.agents/rules/agent-workflows.md, "Advisor").
        builders = dict(re.findall(r"(?ms)^function (\w+Prompt)\((.*?)^\}$", self.text))
        self.assertGreaterEqual(len(builders), 10, sorted(builders))
        allowed = {"specReviewPrompt", "baseLensPrompt"}
        self.assertLessEqual(allowed, set(builders))
        for name, body in builders.items():
            with self.subTest(builder=name):
                self.assertEqual(body.count("ADVISOR_ALLOWED"), 1 if name in allowed else 0)
                self.assertEqual(body.count("ADVISOR_FORBIDDEN"), 0 if name in allowed else 1)
        self.assertEqual(self.text.count("const ADVISOR_ALLOWED = "), 1)
        self.assertEqual(self.text.count("const ADVISOR_FORBIDDEN = "), 1)

    def test_writing_prompt_builders_carry_the_restart_sentence(self) -> None:
        # The stages that write get RESUME once; a read-only builder never does.
        builders = dict(re.findall(r"(?ms)^function (\w+Prompt)\((.*?)^\}$", self.text))
        writing = {"specPrompt", "specFixPrompt", "implementPrompt", "simplifyPrompt", "fixPrompt"}
        self.assertLessEqual(writing, set(builders))
        for name, body in builders.items():
            with self.subTest(builder=name):
                self.assertEqual(body.count("RESUME"), 1 if name in writing else 0)
        self.assertEqual(self.text.count("const RESUME = "), 1)

    def test_shared_sentences_keep_the_reviewed_wording(self) -> None:
        # Full text, not a prefix: an appended "unless you are stuck", a dropped
        # "run this stage's checks" or "still hold", or "retry until it answers"
        # each fails here.
        node = need_node(self)
        with tempfile.TemporaryDirectory() as tmp:
            got, _ = shared_texts(node, self.text, tmp)
        for name, want in EXPECTED.items():
            with self.subTest(sentence=name):
                self.assertEqual(got[name], want)

    def test_docs_keep_the_reviewed_facts(self) -> None:
        for rel, (present, absent) in DOC_PINS.items():
            doc = (ROOT / rel).read_text(encoding="utf-8")
            for text in present:
                with self.subTest(doc=rel, present=text[:60]):
                    self.assertEqual(doc.count(text), 1)
            for text in absent:
                with self.subTest(doc=rel, absent=text):
                    self.assertNotIn(text, doc)

    def test_node_parse_and_harness(self) -> None:
        node = need_node(self)
        meta, body = split_template(self.text)
        module = meta + "\nexport default async function changeFlow(" + ", ".join(HOOKS) + ") {\n" + body + "\n}\n"
        with tempfile.TemporaryDirectory() as tmp:
            mjs = Path(tmp) / "change-flow.mjs"
            mjs.write_text(module, encoding="utf-8")
            harness = Path(tmp) / "harness.mjs"
            harness.write_text(HARNESS, encoding="utf-8")
            _, shared = shared_texts(node, self.text, tmp)
            check = subprocess.run([node, "--check", str(mjs)], capture_output=True, text=True)
            self.assertEqual(check.returncode, 0, check.stderr)
            ran = subprocess.run([node, str(harness), mjs.as_uri(), str(shared)], capture_output=True, text=True, cwd=tmp)
            self.assertEqual(ran.returncode, 0, ran.stdout + ran.stderr)
            self.assertIn("harness: ok", ran.stdout)


if __name__ == "__main__":
    unittest.main()
