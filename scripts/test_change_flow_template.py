#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Self-test for the change-flow Workflow template (.agents/workflows/change-flow.js).

The Workflow runtime runs the script body as an async function, so the body may
`return` at top level. This test:
  - checks that `meta` is a pure literal (no `${`, no backticks, no calls);
  - wraps the body in an exported async function, writes it to a temporary
    .mjs file and runs `node --check` on it (ES-module parse);
  - imports it under node with stub agent()/parallel()/phase()/log() hooks and
    drives dry-run, bad args, a full run, a failed code-review lens, a guard
    violation and a malformed lens selector.

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

function stubs(overrides) {
  const calls = []
  const phases = []
  const logs = []
  const agent = async (prompt, opts) => {
    const label = (opts && opts.label) || '?'
    calls.push({ label, prompt, schema: !!(opts && opts.schema), agentType: opts && opts.agentType })
    if (overrides[label]) return overrides[label](prompt, opts)
    if (label === 'review:code-review') return 'CODE-REVIEW-BEGIN\n1. a.go:3 nil deref\nCODE-REVIEW-END\nGUARD-JSON: ' + JSON.stringify(guard)
    if (label === 'lenses') return { exit_code: 0, stdout: '{"lenses":["ci"],"fast_lane_eligible":true,"reasons":{"ci":["Makefile"]}}', guard }
    if (label.startsWith('review:') || label === 'spec-review' || label === 'spec-review-2') return { blocking: false, findings: [], summary: 'ok', guard }
    if (label === 'fix') return { status: 'done', base_sha: 'b1', head_sha: 'h2', pr: '7', gates: ['make test'], rejected: [], maintainer_asks: [], summary: 'ok', guard }
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
  assert.deepEqual(result.failed_lenses, [])
  assert.ok(result.next.startsWith('ready for maintainer merge'))
}

// a "(none)" code-review result with no explanation is a failed lens, not zero findings.
{
  const { result, logs } = await go(good, {
    'review:code-review': () => 'CODE-REVIEW-BEGIN\nSkill execution completed\nCODE-REVIEW-END\nGUARD-JSON: ' + JSON.stringify(guard),
  })
  assert.equal(result.status, 'done')
  assert.deepEqual(result.failed_lenses, ['code-review'])
  assert.ok(logs.some((l) => l.includes('code-review lens FAILED')))
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

// finalize runs the fix stage only.
{
  const { calls } = await go({ ...good, mode: 'finalize', pr: 9 })
  assert.deepEqual(calls.map((c) => c.label), ['fix'])
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
