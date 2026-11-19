export const meta = {
  name: 'invowk-change-flow',
  description: 'One OpenSpec change through spec, implementation, simplify, review fan-out and an independent fix-and-gates pass',
  whenToUse: 'A change in the invowk repository that needs the full per-change flow; procedure and modes in .agents/runbooks/change-flow.md, floor in .agents/rules/agent-workflows.md',
  phases: [
    { title: 'Validate', detail: 'check args; dry-run runs the self-checks and spawns no agent' },
    { title: 'Spec', detail: 'openspec-propose or openspec-update-change' },
    { title: 'Spec review', detail: 'independent read-only review, at most one fix round' },
    { title: 'Implement', detail: 'openspec-apply-change, tests with the code, draft PR' },
    { title: 'Simplify', detail: 'simplify skill, quality only' },
    { title: 'Lenses', detail: 'scripts/change-flow-lenses.py picks the domain lenses' },
    { title: 'Review', detail: 'base, code-review and domain lenses in parallel, findings only' },
    { title: 'Fix and gates', detail: 'independent agent: verify findings, full checklist once, archive' },
  ],
}

// Template contract (see .agents/runbooks/change-flow.md):
// - no session paths, no secrets, no copied rule text: prompts name rule files;
// - no clock or random calls (the Workflow runtime forbids them: they break resume);
// - every agent returns a guard of the primary checkout, compared by guardChanged().

const MODES = ['new', 'revise', 'continue', 'finalize', 'fast-lane', 'dry-run']
const CONTINUE_FROM = ['implement', 'simplify', 'review', 'fix']
// Keep in sync with LENS_IDS in scripts/change-flow-lenses.py (a script test checks it).
const LENS_IDS = ['formal', 'windows', 'schema', 'ci', 'security', 'supply-chain', 'perf', 'architecture']

const RULE_FLOOR = '.agents/rules/agent-workflows.md'
const RUNBOOK = '.agents/runbooks/change-flow.md'
const CHECKLIST = '.agents/rules/checklist.md'

// What each domain lens loads. Agent files in .agents/agents/ are not registered
// agent types: they are named in the prompt, never passed as agentType.
const LENS_BRIEFS = {
  formal: { skills: ['formal-verification'], files: ['.agents/rules/formal-bindings.md'], focus: 'model-to-code correspondence tables, golden vectors and rapid properties for every bound symbol the diff renames, moves or changes' },
  windows: { skills: ['windows-testing'], files: ['.agents/rules/windows.md'], focus: 'host path handling, process lifecycle and platform-split tests on Windows' },
  schema: { skills: ['cue', 'schema-sync-check'], files: ['.agents/agents/cue-schema-agent.md', '.agents/rules/cue-patterns.md'], focus: 'CUE schema and Go struct sync, behavioural sync tests and validation constraints' },
  ci: { skills: ['ci-update'], files: ['.agents/rules/ci-workflows.md', '.agents/rules/version-pinning.md'], focus: 'pinning and CI workflow rules for the changed files only (audit; change nothing)' },
  security: { skills: [], files: ['.agents/agents/security-reviewer.md'], focus: 'the high-risk code areas that file names' },
  'supply-chain': { skills: ['module-security'], files: ['.agents/agents/supply-chain-reviewer.md'], focus: 'module-system attack surfaces SC-01..SC-10 touched by the diff' },
  perf: { skills: [], files: ['.agents/agents/performance-analyzer.md'], focus: 'hot paths, allocations and PGO profile staleness' },
  architecture: { skills: ['go-hexagonal-ddd'], files: ['.agents/rules/package-design.md'], focus: 'package boundaries, ports and adapters for the added, removed or renamed packages' },
}

const SNAPSHOT_SCHEMA = {
  type: 'object',
  properties: { status: { type: 'string' }, branch: { type: 'string' }, head: { type: 'string' } },
  required: ['status', 'branch', 'head'],
}
const GUARD_SCHEMA = {
  type: 'object',
  properties: { before: SNAPSHOT_SCHEMA, after: SNAPSHOT_SCHEMA },
  required: ['before', 'after'],
}
const STATUS = { type: 'string', enum: ['done', 'blocked'] }

function withGuard(properties, required) {
  return { type: 'object', properties: { ...properties, guard: GUARD_SCHEMA }, required: [...required, 'guard'] }
}

const STAGE_SCHEMA = withGuard(
  { status: STATUS, summary: { type: 'string' }, head_sha: { type: 'string' }, pr: { type: 'string' }, formal_affected: { type: 'boolean' } },
  ['status', 'summary'],
)
const FINDING = {
  type: 'object',
  properties: {
    file: { type: 'string' },
    line: { type: 'integer' },
    severity: { type: 'string', enum: ['blocking', 'major', 'minor', 'nit'] },
    summary: { type: 'string' },
    evidence: { type: 'string' },
  },
  required: ['file', 'severity', 'summary', 'evidence'],
}
const FINDINGS_SCHEMA = withGuard({ findings: { type: 'array', items: FINDING }, summary: { type: 'string' } }, ['findings', 'summary'])
const SPEC_REVIEW_SCHEMA = withGuard(
  { blocking: { type: 'boolean' }, findings: { type: 'array', items: FINDING }, summary: { type: 'string' } },
  ['blocking', 'findings', 'summary'],
)
const SELECTOR_SCHEMA = withGuard({ exit_code: { type: 'integer' }, stdout: { type: 'string' } }, ['exit_code', 'stdout'])
const FIX_SCHEMA = withGuard(
  {
    status: STATUS,
    base_sha: { type: 'string' },
    head_sha: { type: 'string' },
    pr: { type: 'string' },
    gates: { type: 'array', items: { type: 'string' } },
    rejected: { type: 'array', items: { type: 'string' } },
    maintainer_asks: { type: 'array', items: { type: 'string' } },
    summary: { type: 'string' },
    changed_files: { type: 'array', items: { type: 'string' } },
  },
  ['status', 'base_sha', 'head_sha', 'gates', 'rejected', 'maintainer_asks', 'summary', 'changed_files'],
)

// ---------------------------------------------------------------- pure helpers

const SAFE_PATH = /^\/[A-Za-z0-9._@%+=,:\/-]*$/
const SAFE_REF = /^[A-Za-z0-9][A-Za-z0-9._\/-]*$/
const CHANGE_ID = /^[a-z0-9][a-z0-9-]*$/
// Names that are, or resolve to, something other than a change branch.
const NOT_A_CHANGE_BRANCH = /^(?:main|HEAD|(?:refs|heads|tags|origin|remotes)\/.*)$/

function fail(message) {
  throw new Error('change-flow: ' + message)
}

// plainPath returns p without trailing slashes when it is an absolute path with
// no empty, "." or ".." segment (so two spellings of one directory cannot
// differ), or null.
function plainPath(p) {
  if (typeof p !== 'string' || !SAFE_PATH.test(p)) return null
  const out = p.replace(/\/+$/, '')
  if (out === '' || out.includes('//') || /\/\.\.?(?:\/|$)/.test(out)) return null
  return out
}

// validateArgs returns normalised args or throws on the first bad one.
function validateArgs(raw) {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) fail('args must be an object')
  const known = ['mode', 'change', 'worktree', 'branch', 'primary', 'pr', 'base', 'notes', 'from', 'lane', 'review_file']
  for (const key of Object.keys(raw)) if (!known.includes(key)) fail('unknown arg ' + JSON.stringify(key))
  if (!MODES.includes(raw.mode)) fail('mode must be one of ' + MODES.join(', '))
  const out = { mode: raw.mode, base: raw.base === undefined ? 'origin/main' : raw.base }
  if (raw.mode === 'dry-run') return out
  for (const key of ['change', 'worktree', 'branch', 'primary']) {
    if (typeof raw[key] !== 'string' || raw[key] === '') fail(key + ' is required')
  }
  if (!CHANGE_ID.test(raw.change)) fail('change must be an OpenSpec change id (kebab-case)')
  const worktree = plainPath(raw.worktree)
  const primary = plainPath(raw.primary)
  if (!worktree) fail('worktree must be a plain absolute path')
  if (!primary) fail('primary must be a plain absolute path')
  if (worktree === primary) fail('worktree must not be the primary checkout')
  if (!SAFE_REF.test(raw.branch) || raw.branch.includes('..') || NOT_A_CHANGE_BRANCH.test(raw.branch)) fail('branch must be a change branch, not main or a ref alias')
  if (typeof out.base !== 'string' || !SAFE_REF.test(out.base) || out.base.includes('..')) fail('base must be a git ref')
  if (raw.pr !== undefined) {
    const pr = String(raw.pr)
    if (!/^[1-9][0-9]*$/.test(pr)) fail('pr must be a positive integer')
    out.pr = pr
  }
  if (raw.notes !== undefined && (typeof raw.notes !== 'string' || raw.notes.length > 4000)) fail('notes must be a string of at most 4000 characters')
  if (raw.mode === 'continue') {
    if (!CONTINUE_FROM.includes(raw.from)) fail('continue needs from: ' + CONTINUE_FROM.join(', '))
    out.from = raw.from
  } else if (raw.from !== undefined) {
    fail('from is only valid with mode continue')
  }
  // lane "fast": the run being continued was a fast-lane run, so its spec has had
  // only a self-review and the post-selector independent spec review still applies.
  if (raw.lane !== undefined) {
    if (raw.mode !== 'continue' || raw.lane !== 'fast') fail('lane must be "fast", and only with mode continue')
    out.lane = 'fast'
  }
  // review_file: the review of an earlier run (its result's `review` field), for
  // a fix stage that runs without a review fan-out of its own.
  const fixOnly = raw.mode === 'finalize' || (raw.mode === 'continue' && raw.from === 'fix')
  if (raw.review_file !== undefined) {
    if (!fixOnly) fail('review_file is only valid with finalize or continue from fix')
    out.review_file = plainPath(raw.review_file)
    if (!out.review_file) fail('review_file must be a plain absolute path')
  } else if (raw.mode === 'continue' && raw.from === 'fix') {
    fail('continue from fix needs review_file: the earlier review is not carried between runs')
  }
  return { ...out, change: raw.change, worktree, primary, branch: raw.branch, notes: raw.notes }
}

// planStages lists the stages a mode runs, in order.
function planStages(mode, from) {
  const tail = ['implement', 'simplify', 'lenses', 'review', 'fix']
  if (mode === 'new' || mode === 'revise') return ['spec', 'spec-review', ...tail]
  if (mode === 'fast-lane') return ['spec-fast', ...tail]
  if (mode === 'finalize') return ['fix']
  if (mode === 'continue') {
    const start = { implement: 'implement', simplify: 'simplify', review: 'lenses', fix: 'fix' }[from]
    if (!start) fail('unknown continue point ' + JSON.stringify(from))
    return tail.slice(tail.indexOf(start))
  }
  if (mode === 'dry-run') return []
  fail('unknown mode ' + JSON.stringify(mode))
}

// normaliseLenses validates the selector's stdout; anything malformed selects every lens.
function normaliseLenses(stdout) {
  const all = (why) => ({ lenses: [...LENS_IDS], fast_lane_eligible: false, reasons: {}, fallback: why })
  let parsed
  try {
    parsed = JSON.parse(stdout)
  } catch (err) {
    return all('selector output is not JSON')
  }
  if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.lenses)) return all('selector output has no lenses array')
  if (typeof parsed.fast_lane_eligible !== 'boolean') return all('selector output has no fast_lane_eligible boolean')
  for (const id of parsed.lenses) if (!LENS_IDS.includes(id)) return all('unknown lens id ' + JSON.stringify(id))
  const reasons = parsed.reasons && typeof parsed.reasons === 'object' && !Array.isArray(parsed.reasons) ? parsed.reasons : {}
  return { lenses: LENS_IDS.filter((id) => parsed.lenses.includes(id)), fast_lane_eligible: parsed.fast_lane_eligible, reasons, fallback: null }
}

function sameSnapshot(a, b) {
  return a.status === b.status && a.branch === b.branch && a.head === b.head
}

function validSnapshot(s) {
  return !!s && typeof s === 'object' && ['status', 'branch', 'head'].every((k) => typeof s[k] === 'string') && s.head !== ''
}

// guardChanged returns null when the primary checkout did not change during a
// stage, or a description of the problem (missing guard counts as changed).
function guardChanged(guard) {
  if (!guard || !validSnapshot(guard.before) || !validSnapshot(guard.after)) return 'missing or malformed guard'
  if (sameSnapshot(guard.before, guard.after)) return null
  const diff = ['status', 'branch', 'head'].filter((k) => guard.before[k] !== guard.after[k])
  return 'primary checkout changed (' + diff.join(', ') + ')'
}

// parseTextGuard reads the last "GUARD-JSON: {...}" line of a schemaless agent's text.
function parseTextGuard(text) {
  const lines = String(text || '').split('\n').filter((l) => l.startsWith('GUARD-JSON:'))
  if (!lines.length) return null
  try {
    return JSON.parse(lines[lines.length - 1].slice('GUARD-JSON:'.length))
  } catch (err) {
    return null
  }
}

// A line that, alone, reports no findings without saying what was reviewed, or
// carries no content: a bare bracket, a heading word, an empty JSON array.
const BARE_NO_FINDINGS = /^(?:\(none\)|none|nothing|no (?:findings?|issues?|problems?|bugs?)(?: found)?|nothing (?:found|usable|to report)|\[\]|\{\}|[\[\]{},]+|summary|result|results|findings|review)$/i

// prNumber returns the PR number in an agent's pr field ("7", "#7", or a pull
// URL of this repository), or null, so a free-text value or another
// repository's PR never becomes the code-review target.
function prNumber(value) {
  const m = /^(?:#?([1-9][0-9]*)|https:\/\/github\.com\/invowk\/invowk\/pull\/([1-9][0-9]*)\/?)$/.exec(String(value || '').trim())
  return m ? m[1] || m[2] : null
}

// reviewBody extracts the verbatim code-review text, or null when the lens failed.
// It fails unless the agent's status line after the markers says ok AND the body
// holds something besides skill boilerplate and bare "no findings" lines: no
// markers, no or a non-ok status, an empty body, only "Skill execution
// completed", or a "(none)" / "No findings." / "[]" result with no explanation
// (probe P3 in the runbook).
function reviewBody(text) {
  const match = /CODE-REVIEW-BEGIN\n([\s\S]*?)\nCODE-REVIEW-END[ \t]*\n+CODE-REVIEW-STATUS:[ \t]*([A-Za-z]+)/.exec(String(text || ''))
  if (!match || match[2].toLowerCase() !== 'ok') return null
  const substantive = match[1]
    .replace(/^.*skill execution completed.*$/gim, '')
    .replace(/^\s*skill\s+"?[\w:.-]+"?\s+completed\b.*$/gim, '')
    .replace(/^\s*```\w*\s*$/gm, '')
    .split('\n')
    .map((l) => l.replace(/^\s*#+\s*/, '').replace(/^\s*(?:findings|results?|summary)\s*:?/i, '').trim().replace(/[.!:]+$/, '').trim())
    .filter((l) => l !== '' && !BARE_NO_FINDINGS.test(l))
  return substantive.length ? match[1].trim() : null
}

// --------------------------------------------------------------- self-checks

function expectThrow(name, fn) {
  let threw = false
  try {
    fn()
  } catch (err) {
    threw = true
  }
  if (!threw) fail('self-check ' + name + ': expected an error')
}

function expectEqual(name, got, want) {
  if (JSON.stringify(got) !== JSON.stringify(want)) fail('self-check ' + name + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want))
}

function selfChecks() {
  const good = { mode: 'new', change: 'add-thing', worktree: '/w/invowk-x', branch: 'feat/add-thing', primary: '/w/invowk' }
  expectEqual('args.default-base', validateArgs(good).base, 'origin/main')
  expectEqual('args.pr-number', validateArgs({ ...good, pr: 12 }).pr, '12')
  expectEqual('args.dry-run-alone', validateArgs({ mode: 'dry-run' }).mode, 'dry-run')
  expectEqual('args.continue', validateArgs({ ...good, mode: 'continue', from: 'review' }).from, 'review')
  const bad = {
    'not-object': null,
    'unknown-mode': { ...good, mode: 'ship-it' },
    'unknown-key': { ...good, force: true },
    'missing-change': { ...good, change: undefined },
    'bad-change': { ...good, change: 'Add Thing' },
    'relative-worktree': { ...good, worktree: 'invowk-x' },
    'dotdot-worktree': { ...good, worktree: '/w/../etc' },
    'shell-worktree': { ...good, worktree: '/w/x;rm' },
    'same-checkout': { ...good, worktree: '/w/invowk/' },
    'same-checkout-dot': { ...good, worktree: '/w/invowk/.' },
    'same-checkout-slashes': { ...good, worktree: '//w/invowk' },
    'main-branch': { ...good, branch: 'main' },
    'main-ref': { ...good, branch: 'refs/heads/main' },
    'main-heads': { ...good, branch: 'heads/main' },
    'tag-ref': { ...good, branch: 'tags/v1' },
    'remote-main': { ...good, branch: 'origin/main' },
    'head-branch': { ...good, branch: 'HEAD' },
    'bad-base': { ...good, base: '-x' },
    'bad-pr': { ...good, pr: '12a' },
    'continue-no-from': { ...good, mode: 'continue' },
    'from-without-continue': { ...good, from: 'fix' },
    'continue-fix-no-review': { ...good, mode: 'continue', from: 'fix' },
    'review-file-not-fix': { ...good, review_file: '/w/review.json' },
    'review-file-relative': { ...good, mode: 'finalize', review_file: 'review.json' },
    'lane-without-continue': { ...good, mode: 'fast-lane', lane: 'fast' },
    'lane-unknown': { ...good, mode: 'continue', from: 'review', lane: 'slow' },
  }
  for (const [name, value] of Object.entries(bad)) expectThrow('args.' + name, () => validateArgs(value))
  expectEqual('args.continue-fix', validateArgs({ ...good, mode: 'continue', from: 'fix', review_file: '/w/r.json' }).review_file, '/w/r.json')
  expectEqual('args.continue-lane', validateArgs({ ...good, mode: 'continue', from: 'simplify', lane: 'fast' }).lane, 'fast')
  expectEqual('args.feature-branch-named-main', validateArgs({ ...good, branch: 'feat/main' }).branch, 'feat/main')
  for (const id of LENS_IDS) expectEqual('lenses.brief-' + id, !!LENS_BRIEFS[id], true)

  expectEqual('plan.new', planStages('new'), ['spec', 'spec-review', 'implement', 'simplify', 'lenses', 'review', 'fix'])
  expectEqual('plan.fast-lane', planStages('fast-lane'), ['spec-fast', 'implement', 'simplify', 'lenses', 'review', 'fix'])
  expectEqual('plan.continue-review', planStages('continue', 'review'), ['lenses', 'review', 'fix'])
  expectEqual('plan.finalize', planStages('finalize'), ['fix'])
  expectEqual('plan.simplify-before-review', planStages('revise').indexOf('simplify') < planStages('revise').indexOf('review'), true)

  expectEqual('lenses.ok', normaliseLenses('{"lenses":["perf","ci"],"fast_lane_eligible":true,"reasons":{}}').lenses, ['ci', 'perf'])
  expectEqual('lenses.empty', normaliseLenses('{"lenses":[],"fast_lane_eligible":true}').lenses, [])
  for (const [name, stdout] of Object.entries({
    'not-json': 'Traceback (most recent call last)',
    'no-array': '{"lenses":"ci","fast_lane_eligible":false}',
    'unknown-id': '{"lenses":["ci","vibes"],"fast_lane_eligible":false}',
    'no-flag': '{"lenses":["ci"]}',
    'null': 'null',
  })) {
    const got = normaliseLenses(stdout)
    expectEqual('lenses.' + name, [got.lenses, got.fast_lane_eligible, got.fallback !== null], [LENS_IDS, false, true])
  }

  const snap = { status: '?? a.log', branch: 'main', head: 'abc' }
  expectEqual('guard.same', guardChanged({ before: snap, after: { ...snap } }), null)
  expectEqual('guard.status', guardChanged({ before: snap, after: { ...snap, status: ' M x.go\n?? a.log' } }), 'primary checkout changed (status)')
  expectEqual('guard.head', guardChanged({ before: snap, after: { ...snap, head: 'def' } }), 'primary checkout changed (head)')
  expectEqual('guard.missing', guardChanged(undefined), 'missing or malformed guard')
  expectEqual('guard.malformed', guardChanged({ before: snap, after: { status: '' } }), 'missing or malformed guard')
  const line = 'GUARD-JSON: ' + JSON.stringify({ before: snap, after: snap })
  expectEqual('guard.text', guardChanged(parseTextGuard('x\n' + line + '\n')), null)
  expectEqual('guard.text-missing', parseTextGuard('no guard here'), null)

  const wrap = (body, status) => 'CODE-REVIEW-BEGIN\n' + body + '\nCODE-REVIEW-END\nCODE-REVIEW-STATUS: ' + (status || 'ok') + '\n'
  expectEqual('review.findings', reviewBody(wrap('1. a.go:3 nil deref')), '1. a.go:3 nil deref')
  expectEqual('review.explained-none', reviewBody(wrap('Result: (none). Reviewed 4 files; the retry loop is bounded.')) !== null, true)
  expectEqual('review.bare-none', reviewBody(wrap('## Result\n(none)')), null)
  expectEqual('review.none-period', reviewBody(wrap('(none).')), null)
  expectEqual('review.findings-none', reviewBody(wrap('Findings: (none).')), null)
  expectEqual('review.no-findings', reviewBody(wrap('No findings.')), null)
  expectEqual('review.fork-empty-json', reviewBody(wrap('Skill "code-review" completed (forked execution).\n\nResult:\n```json\n[]\n```')), null)
  expectEqual('review.skill-only', reviewBody(wrap('Skill execution completed')), null)
  expectEqual('review.split-empty-json', reviewBody(wrap('Result:\n```json\n[\n]\n```')), null)
  expectEqual('review.heading-no-findings', reviewBody(wrap('## Summary\nNo findings.')), null)
  expectEqual('pr.number', [prNumber('7'), prNumber('#7'), prNumber('https://github.com/invowk/invowk/pull/176')], ['7', '7', '176'])
  expectEqual('pr.junk', [prNumber('seven'), prNumber(''), prNumber('https://example.com/x/7'), prNumber('https://github.com/other/repo/pull/5')], [null, null, null, null])
  expectEqual('review.explained-failure', reviewBody(wrap('The skill returned nothing usable.', 'failed')), null)
  expectEqual('review.no-status', reviewBody('CODE-REVIEW-BEGIN\n1. a.go:3 nil deref\nCODE-REVIEW-END\n'), null)
  expectEqual('review.no-markers', reviewBody('Skill execution completed'), null)
  return ['args', 'plan', 'lenses', 'guard', 'review']
}

// ------------------------------------------------------------------ prompts

function context(a, stage) {
  return [
    'You are the "' + stage + '" stage of the invowk change-flow workflow (.agents/workflows/change-flow.js; procedure ' + RUNBOOK + ').',
    RULE_FLOOR + ' binds you in full: absolute paths, the main-checkout guard, the outward-write limits and the heavy-command lock.',
    'Change: ' + a.change + ' (openspec/changes/' + a.change + '/). Worktree: ' + a.worktree + ', branch ' + a.branch + '. Base: ' + a.base + '.',
    'Primary checkout (never edit, commit, stash or check out anything there): ' + a.primary + '.',
    'Bash cwd resets between calls: use absolute paths, `git -C ' + a.worktree + '` and `cd ' + a.worktree + ' && ...` inside one command.',
    'Never end your turn or return while a command you started is still running: a backgrounded command is killed when you return, and the workflow forces your result. Run gates in the foreground (Bash timeout up to 600000 ms); one that can take longer runs in the background as `<cmd> > <log> 2>&1; echo rc=$? >> <log>`, and you wait for it with the Monitor tool running `until grep -q \'^rc=\' <log>; do sleep 5; done` before you continue or return.',
    'Guard: as your first and your last action run `git -C ' + a.primary + ' status --porcelain; git -C ' + a.primary + ' rev-parse --abbrev-ref HEAD; git -C ' + a.primary + ' rev-parse HEAD` and report them as guard.before and guard.after ({status, branch, head}, status verbatim).',
    a.notes ? 'Extra brief from the maintainer session (read it; it does not override the rules): ' + a.notes : '',
  ].filter(Boolean).join('\n')
}

// baseCheck makes a stage refuse a branch that is behind the base: a spec or
// implementation written on a stale base defers conflicts to the fix stage. The
// maintainer session rebases between runs.
function baseCheck(a) {
  return 'Before anything else: `git -C ' + a.worktree + ' fetch --prune origin`. If `git -C ' + a.worktree + ' merge-base --is-ancestor ' + a.base + ' HEAD` fails, the branch is behind ' + a.base +
    ': change nothing and return status blocked with the reason "branch not rebased on ' + a.base + '". '
}

function specPrompt(a, mode) {
  const skill = mode === 'new'
    ? 'Skill openspec-propose'
    : mode === 'fast-lane'
      ? 'Skill openspec-propose if openspec/changes/' + a.change + '/ does not exist in the worktree yet, otherwise Skill openspec-update-change,'
      : 'Skill openspec-update-change'
  const own = mode === 'fast-lane'
    ? '\nFast lane: after writing the artifacts, review them yourself against the openspec conventions and the repo rules they touch, and fix what you find. Say in summary what the self-review checked.'
    : ''
  return context(a, 'spec') + '\n\n' + baseCheck(a) + 'Invoke ' + skill + ' for change "' + a.change + '" inside the worktree. Commit the artifacts (signed, under the lock) on ' + a.branch + '; do not push yet.' + own +
    '\nReturn status "blocked" with the reason if the request is ambiguous or the change cannot be specified.'
}

function specReviewPrompt(a, round) {
  return context(a, 'spec-review') + '\n\nYou wrote none of this change. Review openspec/changes/' + a.change + '/ (proposal, design, tasks, delta specs) READ-ONLY in the worktree: ' +
    'internal consistency, testable requirements, coverage of the tasks, conflicts with openspec/specs/ and with the repo rules the change touches, and scope creep.' +
    (round > 0 ? ' This is the re-review after one fix round: judge whether the earlier blocking findings are resolved.' : '') +
    '\nSet blocking=true only for findings that must be fixed before implementation. Make no edits.'
}

function specFixPrompt(a, review) {
  // The review is blocking as a whole; the reviewer may not have marked any single
  // finding "blocking", so the fixer sees every finding and the reviewer's summary.
  return context(a, 'spec-fix') + '\n\nThe independent spec review is blocking. Invoke Skill openspec-update-change for "' + a.change + '" to resolve it, then commit (signed, under the lock). ' +
    'Resolve every finding marked severity "blocking"; if none is marked, resolve the ones the summary names as blocking, or else every "major" one.\n' +
    'Reviewer summary: ' + review.summary + '\nFindings:\n' + JSON.stringify(review.findings, null, 1)
}

function implementPrompt(a) {
  return context(a, 'implement') + '\n\n' + baseCheck(a) + 'Invoke Skill openspec-apply-change for "' + a.change + '" in the worktree. Write tests with the code and run the tests of what you changed. ' +
    'Commit (signed, under the lock; messages per .agents/rules/git.md), push ' + a.branch + ' and open or update its DRAFT pull request (never ready, never merge).\n' +
    'Formal lane (' + RUNBOOK + ', "Formal lane"): run `cd ' + a.worktree + ' && python3 scripts/formal.py affected --event pull_request --base ' + a.base + ' --head HEAD`; when it prints run=true, also run ' +
    '`python3 scripts/formal.py correspondence`, `python3 scripts/formal_mutation.py triage --check` and `make formal-reach` there, and fix what fails. Leave `make formal` to CI unless Java is present.\n' +
    'Return head_sha, pr (the number) and formal_affected.'
}

function simplifyPrompt(a) {
  return context(a, 'simplify') + '\n\nInvoke Skill simplify with args "the committed diff `git -C ' + a.worktree + ' diff ' + a.base + '...HEAD`" and apply its cleanups inside the worktree only. ' +
    'Quality only: no behaviour changes. It runs single-pass here; that is expected. Run only the tests of the code you touched, then commit (signed, under the lock) and push ' + a.branch + '. Return head_sha.'
}

function selectorPrompt(a) {
  return context(a, 'lenses') + '\n\nRun exactly `cd ' + a.worktree + ' && python3 scripts/change-flow-lenses.py --base ' + a.base + ' --head HEAD` and return its exit code and its stdout verbatim. Change nothing.'
}

function baseLensPrompt(a) {
  return context(a, 'review:base') + '\n\nYou wrote none of this code. Follow .agents/agents/code-reviewer.md over `git -C ' + a.worktree + ' diff ' + a.base + '...HEAD`, ' +
    'and check the implementation against the delta specs in openspec/changes/' + a.change + '/specs/ and its tasks.md. READ-ONLY: findings only, no edits, no commits.'
}

function codeReviewPrompt(a, target) {
  return context(a, 'review:code-review') + '\n\nInvoke Skill code-review with args exactly "high ' + target + '" (never --fix, never --comment). It runs as a fork; that is expected. ' +
    'Then answer with the fork\'s full text verbatim between a line CODE-REVIEW-BEGIN and a line CODE-REVIEW-END, then one line `CODE-REVIEW-STATUS: ok` or `CODE-REVIEW-STATUS: failed`, ' +
    'then one line `GUARD-JSON: {"before":{...},"after":{...}}`. ' +
    'Status ok only when the fork returned review results: findings, or a statement of what it reviewed and why it found nothing. ' +
    'Status failed when it returned nothing, only "Skill execution completed", or "(none)" / "No findings" / [] with no explanation; you may say why between the markers. Make no edits.'
}

function domainLensPrompt(a, lens, reasons) {
  const brief = LENS_BRIEFS[lens]
  const loads = [
    ...brief.skills.map((s) => 'Skill ' + s),
    ...brief.files.map((f) => 'read ' + f),
  ].join(', ')
  return context(a, 'review:' + lens) + '\n\nYou wrote none of this code. Load: ' + loads + '. Review `git -C ' + a.worktree + ' diff ' + a.base + '...HEAD` for ' + brief.focus + '.' +
    '\nPaths that selected this lens: ' + JSON.stringify((reasons || []).slice(0, 40)) + '\nREAD-ONLY: findings only, no edits, no commits, no outward writes.'
}

// reviewSection tells the fix agent what review it acts on: this run's fan-out,
// an earlier run's review file, or, explicitly, none.
function reviewSection(a, review) {
  if (review.ran) {
    return 'Failed lenses (re-check their angle yourself): ' + JSON.stringify(review.failed) + '\n' +
      'Structured findings: ' + JSON.stringify(review.findings) + '\n' +
      'code-review text:\n' + (review.failed.includes('code-review') ? '(the code-review lens FAILED; it is in the failed lenses above)' : review.codeReview)
  }
  if (a.review_file) {
    return 'No review fan-out ran in this run. The review to act on is the earlier run\'s, in ' + a.review_file + ' (a change-flow result\'s `review` field: findings, failed lenses, code-review text). ' +
      'Read it and treat it exactly as this run\'s findings; if it is missing or unreadable, return status blocked.'
  }
  return 'No review fan-out ran in this run and no review file was given, so there are no review findings to apply. ' +
    'Do not describe the change as reviewed by this run; say in summary that this pass applied no review findings.'
}

function fixPrompt(a, review) {
  return context(a, 'fix-and-gates') + '\n\nYou are the independent fix-and-gates agent: you wrote none of this change. In the worktree:\n' +
    '1. `git -C ' + a.worktree + ' fetch --prune origin` (--prune, so a deleted remote branch leaves no stale ref and lease), then `' + a.primary + '/scripts/change-flow-foreign-commits.sh ' + a.worktree + ' ' + a.branch + ' ' + a.base + '`: it lists the commits on origin/' + a.branch + ' with no patch-equivalent locally and not on ' + a.base + ', which someone else pushed (for example the CI PGO bot). ' +
    'If it exits 3, origin/' + a.branch + ' has a merge commit (GitHub "Update branch" or a web conflict resolution) that a force push would drop: return status blocked and list the printed merges in maintainer_asks. Any exit other than 0 or 3 is a failure, not an empty list: return status blocked with its stderr. ' +
    'Ancestry is not the test: a stopped run that rebased leaves pre-rebase copies there, and those must not be replayed. Cherry-pick each listed commit, oldest first, never drop one; if one is your own commit changed by a conflict resolution, or a cherry-pick conflicts, return status blocked and list it in maintainer_asks. ' +
    'Record the fetched tip (`git -C ' + a.worktree + ' rev-parse --verify --quiet origin/' + a.branch + '`, empty when never pushed) as the lease. ' +
    'Then rebase ' + a.branch + ' on ' + a.base + '; report the base SHA you rebased on.\n' +
    '2. Dedupe and verify every finding below against the branch\'s code (`git -C ' + a.worktree + ' show HEAD:<file>`: the code-review fork reads surrounding files from the primary checkout' +
    (a.base === 'origin/main' ? '' : ', and it diffs against main, so drop findings that belong to ' + a.base + '\'s own commits') +
    '); apply the confirmed ones; list each rejected one with its reason. Commit them locally (signed, under the lock) BEFORE the long gates, so a stopped run leaves them committed; push only after step 3.\n' +
    '3. Run the FULL checklist (' + CHECKLIST + ') ONCE on the final tree, plus the formal lane of ' + RUNBOOK + '. Skip checklist step 1 (/simplify): it belongs to the simplify stage, and a cleanup here would land after the review. ' +
    'Run `make mutation-formal` once, report-only, in a throwaway worktree, ONLY if this change rewrote a function body bound by .agents/rules/formal-bindings.md' +
    (a.mode === 'finalize' ? '; in this finalize run, only if the rebase changed such a body compared with the head the earlier gates ran on (say which you checked)' : '') + '.\n' +
    '4. Invoke Skill openspec-archive-change for "' + a.change + '" (skip it when openspec/changes/' + a.change + '/ is gone and openspec/changes/archive/*-' + a.change + '/ exists: the change is already archived), ' +
    'then Skill learn (minimal; never edit the change-flow tooling, i.e. .agents/workflows/, ' + RUNBOOK + ', ' + RULE_FLOOR + ' and scripts/change-flow-*, inside a change\'s PR: list proposed edits to it in maintainer_asks), commit (signed, under the lock), and push with ' +
    '`git -C ' + a.worktree + ' push --force-with-lease=' + a.branch + ':<the lease from step 1> origin ' + a.branch + '` (a bare --force-with-lease after a fetch protects nothing). ' +
    'If the lease is rejected, someone pushed meanwhile: repeat step 1 (fetch --prune, the helper, cherry-picks), re-run the gates their commits touch, and push again with the new lease; never a bare force. Never approve held workflow runs. ' +
    'update the DRAFT PR and watch `gh pr checks`; fix failures. Never mark it ready, never merge.\n' +
    'Anything that needs an outward write the floor rule forbids goes into maintainer_asks. So does every point where a rule says to stop and ask the user ' +
    '(for example checklist step 5, or a scope question under .agents/rules/general-rules.md): you cannot ask, so list the question with your evidence, ' +
    'and return status blocked when the answer decides whether the change is correct.\n' +
    'Return changed_files: every non-test file you changed in this stage (paths relative to the worktree), so the maintainer session can review post-review edits.\n\n' +
    reviewSection(a, review)
}

// ---------------------------------------------------------------- execution

const a = validateArgs(args)
phase('Validate')
const checks = selfChecks()
if (a.mode === 'dry-run') {
  log('dry-run: self-checks passed (' + checks.join(', ') + '); no agent spawned')
  return { status: 'done', mode: 'dry-run', checks, plans: Object.fromEntries(['new', 'revise', 'fast-lane', 'finalize'].map((m) => [m, planStages(m)])) }
}

const stages = planStages(a.mode, a.from)
log('plan: ' + stages.join(' -> '))
const record = []
let baseline = null
let pr = a.pr
let headSha = ''
// review.ran is false until this run's fan-out finishes.
let review = { ran: false, findings: [], failed: [], codeReview: '' }

// reviewOut is the review as a result field, so a later finalize or
// continue-from-fix run can be given it as review_file.
function reviewOut() {
  return review.ran ? { findings: review.findings, failed_lenses: review.failed, code_review: review.codeReview } : null
}

function blocked(stage, why, extra) {
  return { status: 'blocked', mode: a.mode, change: a.change, branch: a.branch, pr: pr || '', blocked_at: stage, reason: why, stages: record, review: reviewOut(), ...(extra || {}) }
}

// check enforces the guard and records the stage; it returns an error string or null.
function check(stage, result) {
  if (!result) return 'agent returned nothing (skipped or died)'
  const changed = guardChanged(result.guard)
  if (changed) return changed
  if (baseline && !sameSnapshot(baseline, result.guard.before)) log('note: primary checkout differs from the previous stage (' + stage + '); not caused by a stage')
  baseline = result.guard.after
  if (result.pr) {
    const n = prNumber(result.pr)
    if (n) pr = n
    else log('note: ' + stage + ' returned pr ' + JSON.stringify(String(result.pr)) + ', not a PR number; kept ' + (pr || 'the branch as the target'))
  }
  if (result.head_sha) headSha = result.head_sha
  record.push({ stage, status: result.status || 'done', summary: result.summary || '' })
  return null
}

async function stageAgent(stage, title, prompt, schema, opts) {
  phase(title)
  const result = await agent(prompt, { label: stage, phase: title, schema, ...(opts || {}) })
  const problem = check(stage, result)
  if (problem) throw new Error(stage + ': ' + problem)
  return result
}

// specReviewLoop returns {why, fixed}: why is the blocking reason or null, and
// fixed says whether a spec-fix round changed the spec.
async function specReviewLoop() {
  let specReview = await stageAgent('spec-review', 'Spec review', specReviewPrompt(a, 0), SPEC_REVIEW_SCHEMA)
  if (!specReview.blocking) return { why: null, fixed: false }
  const fixed = await stageAgent('spec-fix', 'Spec review', specFixPrompt(a, specReview), STAGE_SCHEMA)
  if (fixed.status !== 'done') return { why: 'spec fix blocked: ' + fixed.summary, fixed: true }
  specReview = await stageAgent('spec-review-2', 'Spec review', specReviewPrompt(a, 1), SPEC_REVIEW_SCHEMA)
  return { why: specReview.blocking ? 'blocking spec findings remain after one fix round: ' + specReview.summary : null, fixed: true }
}

async function reviewFanOut(selection) {
  const target = pr || a.branch
  const lenses = [
    { id: 'base', run: () => agent(baseLensPrompt(a), { label: 'review:base', phase: 'Review', schema: FINDINGS_SCHEMA }) },
    { id: 'code-review', run: () => agent(codeReviewPrompt(a, target), { label: 'review:code-review', phase: 'Review' }) },
    ...selection.lenses.map((id) => ({
      id,
      run: () => agent(domainLensPrompt(a, id, selection.reasons[id]), { label: 'review:' + id, phase: 'Review', schema: FINDINGS_SCHEMA }),
    })),
  ]
  phase('Review')
  const results = await parallel(lenses.map((l) => l.run))
  const out = { ran: true, findings: [], failed: [], codeReview: '' }
  for (let i = 0; i < lenses.length; i++) {
    const id = lenses[i].id
    const result = results[i]
    if (id === 'code-review') {
      if (result === null || result === undefined) {
        // Skipped or died: a failed lens, like a dead domain lens. A reply with a
        // missing or changed guard still stops the run below.
        out.failed.push(id)
        log('code-review lens FAILED: agent returned nothing (skipped or died); the base lens keeps the floor')
        continue
      }
      const body = reviewBody(result)
      const guard = parseTextGuard(result)
      const problem = check('review:code-review', guard ? { guard, summary: body ? 'ok' : 'failed lens' } : null)
      if (problem) throw new Error('review:code-review: ' + problem)
      if (body) out.codeReview = body
      else {
        out.failed.push(id)
        log('code-review lens FAILED (status not ok, no output, or "(none)" with no explanation); the base lens keeps the floor')
      }
      continue
    }
    const problem = check('review:' + id, result)
    if (problem === 'agent returned nothing (skipped or died)') {
      out.failed.push(id)
      log('lens ' + id + ' FAILED: ' + problem)
      continue
    }
    if (problem) throw new Error('review:' + id + ': ' + problem)
    out.findings.push(...result.findings.map((f) => ({ lens: id, ...f })))
  }
  if (out.failed.includes('base')) throw new Error('review:base failed; the floor needs an independent reviewer')
  log('review: ' + out.findings.length + ' structured findings, failed lenses: ' + (out.failed.join(', ') || 'none'))
  return out
}

let selection = { lenses: [...LENS_IDS], fast_lane_eligible: false, reasons: {}, fallback: 'not selected' }
let final = null
const fastLaneRun = a.mode === 'fast-lane' || a.lane === 'fast'

try {
  for (const stage of stages) {
    if (stage === 'spec' || stage === 'spec-fast') {
      const r = await stageAgent(stage, 'Spec', specPrompt(a, stage === 'spec-fast' ? 'fast-lane' : a.mode), STAGE_SCHEMA)
      if (r.status !== 'done') return blocked(stage, r.summary)
    } else if (stage === 'spec-review') {
      const { why } = await specReviewLoop()
      if (why) return blocked(stage, why)
    } else if (stage === 'implement') {
      const r = await stageAgent(stage, 'Implement', implementPrompt(a), STAGE_SCHEMA)
      if (r.status !== 'done') return blocked(stage, r.summary)
    } else if (stage === 'simplify') {
      const r = await stageAgent(stage, 'Simplify', simplifyPrompt(a), STAGE_SCHEMA)
      if (r.status !== 'done') return blocked(stage, r.summary)
    } else if (stage === 'lenses') {
      const r = await stageAgent(stage, 'Lenses', selectorPrompt(a), SELECTOR_SCHEMA, { effort: 'low' })
      selection = r.exit_code === 0 ? normaliseLenses(r.stdout) : normaliseLenses('')
      if (selection.fallback) log('lens selector output unusable (' + selection.fallback + '); running ALL lenses')
      log('domain lenses: ' + (selection.lenses.join(', ') || 'none'))
      if (fastLaneRun && !selection.fast_lane_eligible) {
        log('change is not fast-lane eligible: running the independent spec review before code review')
        const { why, fixed } = await specReviewLoop()
        if (why) return blocked('spec-review', why)
        // The code was written against the spec before this fix: implement again.
        if (fixed) return blocked('spec-review', 'the independent spec review changed the spec after implementation; re-run with mode continue, from implement')
      }
    } else if (stage === 'review') {
      review = await reviewFanOut(selection)
    } else if (stage === 'fix') {
      if (!review.ran) log(a.review_file ? 'no review fan-out in this run: the fix agent reads ' + a.review_file : 'no review fan-out in this run and no review_file: the fix agent applies no review findings')
      final = await stageAgent(stage, 'Fix and gates', fixPrompt(a, review), FIX_SCHEMA)
    }
  }
} catch (err) {
  return blocked(record.length ? 'after ' + record[record.length - 1].stage : 'start', String(err && err.message ? err.message : err))
}

return {
  status: final.status,
  mode: a.mode,
  change: a.change,
  branch: a.branch,
  pr: final.pr || pr || '',
  base_sha: final.base_sha,
  head_sha: final.head_sha || headSha,
  gates: final.gates,
  rejected: final.rejected,
  maintainer_asks: final.maintainer_asks,
  changed_files: final.changed_files,
  lenses: review.ran ? selection.lenses : [],
  review_ran: review.ran,
  failed_lenses: review.failed,
  review: reviewOut(),
  stages: record,
  next: final.status === 'done'
    ? 'ready for maintainer merge: check base_sha == origin/main, then squash-merge per .agents/runbooks/git-merge.md'
    : 'blocked: ' + final.summary,
}
