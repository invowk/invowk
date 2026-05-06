#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import {
  buildAggregateHistory,
  normalizeBenchmarkName,
  parseLegacyMarkdownReport,
  renderMarkdownReport,
  renderSvgSummary,
  validateBenchmarkHistory,
  validateBenchmarkReport,
  validateSvg,
} from './benchmark-report.mjs';

const repoRoot = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..');
const fixtureDir = path.join(repoRoot, 'scripts/fixtures/benchmark-history');
let pass = 0;
let fail = 0;

function assert(name, condition) {
  if (condition) {
    pass += 1;
    return;
  }
  fail += 1;
  console.error(`FAIL: ${name}`);
}

function assertThrows(name, fn) {
  try {
    fn();
    fail += 1;
    console.error(`FAIL: ${name}\n  expected an exception`);
  } catch {
    pass += 1;
  }
}

function readJson(name) {
  return JSON.parse(fs.readFileSync(path.join(fixtureDir, name), 'utf8'));
}

function main() {
  const current = readJson('current.json');
  const previous = readJson('previous-compatible.json');
  const incompatible = readJson('previous-incompatible.json');
  const partial = readJson('partial-run.json');
  const first = readJson('first-release.json');

  for (const [name, fixture] of [
    ['current', current],
    ['previous', previous],
    ['incompatible', incompatible],
    ['partial', partial],
    ['first', first],
  ]) {
    validateBenchmarkReport(fixture);
    assert(`${name} fixture has benchmark data`, fixture.go_benchmarks.length > 0);
  }

  const identity = normalizeBenchmarkName('BenchmarkCUEParsing-24');
  assert('normalizes benchmark suffix', identity.id === 'BenchmarkCUEParsing' && identity.cpuSuffix === '24');

  assertThrows('rejects unsupported report schema', () => {
    validateBenchmarkReport({ ...current, schema_version: '99.0' });
  });

  assertThrows('rejects empty benchmark rows', () => {
    validateBenchmarkReport({ ...current, go_benchmarks: [] });
  });

  const legacy = parseLegacyMarkdownReport(fs.readFileSync(path.join(fixtureDir, 'legacy-report.md'), 'utf8'), {
    sourcePath: 'invowk_0.8.0_bench-report.md',
  });
  assert('legacy parser marks reduced confidence source', legacy.source_kind === 'legacy-markdown');
  assert('legacy parser keeps stable benchmark id', legacy.go_benchmarks[0].id === 'BenchmarkCUEParsing');

  const history = buildAggregateHistory([legacy, previous, incompatible, current, partial, first], {
    generatedAt: '2026-05-05T00:00:00Z',
    source: { kind: 'fixture', path: fixtureDir },
  });
  validateBenchmarkHistory(history);
  assert('history includes all records', history.records.length === 6);
  assert('history has all-history points', history.windows.all.points.length > 0);
  assert('history has one-year window', history.windows.last_1_year.record_count > 0);
  const deterministicHistory = buildAggregateHistory([legacy, previous]);
  assert('history generated_at defaults to newest source record', deterministicHistory.generated_at === previous.release.generated_at);

  const markdown = renderMarkdownReport(current, history);
  assert('markdown includes performance evolution', markdown.includes('## Performance Evolution'));
  assert('markdown includes previous release comparison', markdown.includes('### Previous Release'));
  assert('markdown preserves raw Go output', markdown.includes('## Raw Go Benchmark Output'));

  const svg = renderSvgSummary(current, history);
  validateSvg(svg);
  assert('svg includes lower-is-better description', svg.includes('Lower is better'));

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'invowk-benchmark-report-'));
  try {
    const startupTsv = path.join(tmp, 'startup.tsv');
    const goSummaryTsv = path.join(tmp, 'go.tsv');
    const goRaw = path.join(tmp, 'go_raw.txt');
    fs.writeFileSync(startupTsv, 'Version (--version)\t10.00\t9.00\t11.00\t3\nCmd List (cmd)\t42.00\t40.00\t44.00\t3\n');
    fs.writeFileSync(goSummaryTsv, 'BenchmarkCUEParsing-24\t3\t100.00\t90.00\t110.00\t0.000100\t1000.00\t1.000\t3.000000\t20.00\t2.00\n');
    fs.writeFileSync(goRaw, 'goos: linux\ngoarch: amd64\ncpu: Test CPU\nBenchmarkCUEParsing-24 3 100 ns/op 20 B/op 2 allocs/op\nPASS\nok test 1.0s\n');

    execFileSync('node', [
      'scripts/benchmark-report.mjs',
      'render',
      '--out-dir', tmp,
      '--stamp', '2026-05-05_00-00-00',
      '--mode', 'short',
      '--startup-samples', '3',
      '--go-bench-count', '1',
      '--go-bench-status', 'ok',
      '--go-bench-wall', '1.0',
      '--go-bench-est-total', '1.0',
      '--go-bench-overhead', '0',
      '--go-bench-timing-scope', 'all parsed benchmark rows',
      '--startup-tsv', startupTsv,
      '--go-summary-tsv', goSummaryTsv,
      '--go-raw', goRaw,
      '--branch', 'main',
      '--commit', 'abcdef0',
      '--platform', 'Linux 6.0 x86_64',
      '--go-version', 'go version go1.26.0 linux/amd64',
      '--binary', './bin/invowk',
      '--go-bench-command', 'go test ./internal/benchmark',
      '--generated-at', '2026-05-05 00:00:00 UTC',
      '--tag', 'v1.0.0',
    ], { cwd: repoRoot, stdio: 'pipe' });

    execFileSync('node', ['scripts/benchmark-report.mjs', 'validate-assets', '--dir', tmp, '--layout', 'generated'], {
      cwd: repoRoot,
      stdio: 'pipe',
    });
    assert('render command produced JSON', fs.existsSync(path.join(tmp, '2026-05-05_00-00-00.json')));
    assert('render command produced Markdown', fs.existsSync(path.join(tmp, '2026-05-05_00-00-00.md')));
    assert('render command produced SVG', fs.existsSync(path.join(tmp, '2026-05-05_00-00-00_summary.svg')));
    assert('render command produced raw output', fs.existsSync(path.join(tmp, '2026-05-05_00-00-00_raw.txt')));
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }

  console.log(`${pass} passed, ${fail} failed`);
  process.exit(fail === 0 ? 0 : 1);
}

main();
