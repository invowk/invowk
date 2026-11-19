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
    violations, a malformed lens selector, resume and fast-lane paths.

It needs node; without node it skips locally and fails under CI (CI=true).
"""

from __future__ import annotations

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

HARNESS = r"""
import { strict as assert } from 'node:assert'
const mod = await import(process.argv[2])
const run = mod.default
const meta = mod.meta

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

async function go(args, overrides) {
  const s = stubs(overrides || {})
  const result = await run(...s.hooks, args, { total: null }, async () => { throw new Error('no nested workflows') })
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
  for (const label of ['spec', 'implement']) {
    const p = calls.find((c) => c.label === label).prompt
    assert.ok(p.includes('merge-base --is-ancestor origin/main HEAD'), label + ' refuses a stale base')
  }
  const fixP = calls.find((c) => c.label === 'fix').prompt
  assert.ok(fixP.includes('/w/invowk/scripts/change-flow-foreign-commits.sh /w/invowk-x'), 'the helper comes from the primary checkout')
  assert.ok(fixP.includes('Skip checklist step 1'))
  assert.ok(fixP.includes('the change is already archived'))
  assert.ok(fixP.includes('never edit the change-flow tooling'))
  assert.ok(fixP.includes('Return changed_files'))
  assert.ok(!fixP.includes('in this finalize run'))
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
  assert.ok(!fix.prompt.includes("own commits"), 'default base adds no base-commit note')
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

// a guard difference stops the run at that stage.
{
  const { result, calls } = await go(good, {
    simplify: () => ({ status: 'done', summary: 'x', guard: { before: snap, after: { ...snap, status: ' M internal/x.go' } } }),
  })
  assert.equal(result.status, 'blocked')
  assert.match(result.reason, /primary checkout changed \(status\)/)
  assert.equal(calls[calls.length - 1].label, 'simplify')
}

// a stale base: the spec or implement stage returns blocked and no later stage runs.
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
  assert.ok(calls[0].prompt.includes('in this finalize run, only if the rebase changed such a body compared with the lease from step 1'))
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
console.log('change-flow template harness: ok')
"""


def split_template(text: str) -> tuple[str, str]:
    """Return (meta block, body); meta ends at the first line that is exactly '}'."""
    lines = text.splitlines()
    if not lines or lines[0] != "export const meta = {":
        raise AssertionError("the template must start with `export const meta = {`")
    end = lines.index("}")
    return "\n".join(lines[: end + 1]), "\n".join(lines[end + 1 :])


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

    def test_node_parse_and_harness(self) -> None:
        node = shutil.which("node")
        if node is None:
            if os.environ.get("CI"):
                self.fail("node is required in CI for the change-flow template self-test")
            self.skipTest("node not found locally: skipping the ES-module parse and harness (CI runs them)")
        meta, body = split_template(self.text)
        module = meta + "\nexport default async function changeFlow(" + ", ".join(HOOKS) + ") {\n" + body + "\n}\n"
        with tempfile.TemporaryDirectory() as tmp:
            mjs = Path(tmp) / "change-flow.mjs"
            mjs.write_text(module, encoding="utf-8")
            harness = Path(tmp) / "harness.mjs"
            harness.write_text(HARNESS, encoding="utf-8")
            check = subprocess.run([node, "--check", str(mjs)], capture_output=True, text=True)
            self.assertEqual(check.returncode, 0, check.stderr)
            ran = subprocess.run([node, str(harness), mjs.as_uri()], capture_output=True, text=True, cwd=tmp)
            self.assertEqual(ran.returncode, 0, ran.stdout + ran.stderr)
            self.assertIn("harness: ok", ran.stdout)


if __name__ == "__main__":
    unittest.main()
