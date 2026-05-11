#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0

import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

import {
  SHORT_BENCH_REGEX,
  TRACKED_GO_BENCHMARKS,
  goMetricsToBmf,
  parseGoBenchOutput,
  validateBmf,
} from './bench-bmf.mjs';

const currentFile = fileURLToPath(import.meta.url);
const repoRoot = path.resolve(path.dirname(currentFile), '..');

const sampleGoBenchOutput = `goos: linux
goarch: amd64
pkg: github.com/invowk/invowk/internal/benchmark
cpu: fixture
BenchmarkCUEParsing-24        10 100 ns/op 20 B/op 2 allocs/op
BenchmarkCUEParsing-24        12 120 ns/op 24 B/op 4 allocs/op
BenchmarkFullPipeline-24       5 500 ns/op 80 B/op 8 allocs/op
PASS
ok   github.com/invowk/invowk/internal/benchmark 0.01s
`;

function testParseGoBenchOutput() {
  const parsed = parseGoBenchOutput(sampleGoBenchOutput);
  assert.equal(parsed.size, 2);

  const cue = parsed.get('BenchmarkCUEParsing');
  assert.deepEqual(cue.ns, [100, 120]);
  assert.deepEqual(cue.bytes, [20, 24]);
  assert.deepEqual(cue.allocs, [2, 4]);

  const bmf = goMetricsToBmf(parsed);
  assert.equal(bmf['cue/parse-invowkfile-basic'].latency.value, 110);
  assert.equal(bmf['cue/parse-invowkfile-basic'].latency.lower_value, 100);
  assert.equal(bmf['cue/parse-invowkfile-basic'].latency.upper_value, 120);
  assert.equal(bmf['cue/parse-invowkfile-basic'].memory.value, 22);
  assert.equal(bmf['cue/parse-invowkfile-basic'].allocations.value, 3);
  assert.equal(bmf['command/execute-virtual-end-to-end-basic'].latency.value, 500);
  validateBmf(bmf);
}

function testValidateBmfRejectsInvalidPayload() {
  assert.throws(
    () => validateBmf({ 'go/BenchmarkBad': { latency: { lower_value: 1 } } }),
    /value is required/,
  );
  assert.throws(
    () => validateBmf({}),
    /at least one benchmark/,
  );
}

function testTrackedRegexUsesExplicitBenchmarks() {
  assert.ok(SHORT_BENCH_REGEX.startsWith('^('));
  assert.ok(!SHORT_BENCH_REGEX.includes('.*'));
  for (const name of TRACKED_GO_BENCHMARKS) {
    assert.match(name, /^Benchmark/);
    assert.match(name, new RegExp(SHORT_BENCH_REGEX));
  }
}

function testEmitterEndToEndWithFixtures() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'invowk-bmf-'));
  try {
    const stubBinary = path.join(tmp, 'invowk');
    const rawGo = path.join(tmp, 'go.txt');
    const output = path.join(tmp, 'invowk.bmf.json');

    fs.writeFileSync(stubBinary, '#!/usr/bin/env sh\nexit 0\n');
    fs.chmodSync(stubBinary, 0o755);
    fs.writeFileSync(rawGo, sampleGoBenchOutput);

    execFileSync(
      'node',
      [
        'scripts/bench-bmf.mjs',
        '--no-build',
        '--binary',
        stubBinary,
        '--startup-samples',
        '2',
        '--go-raw-input',
        rawGo,
        '--output',
        output,
      ],
      {
        cwd: repoRoot,
        stdio: 'pipe',
      },
    );

    const bmf = JSON.parse(fs.readFileSync(output, 'utf8'));
    validateBmf(bmf);
    assert.ok(bmf['binary/bin-invowk']['file-size'].value > 0);
    assert.ok(bmf['startup/version'].latency.value > 0);
    assert.ok(bmf['command/execute-cli-native-basic'].latency.value > 0);
    assert.ok(bmf['command/execute-cli-virtual-basic'].latency.value > 0);
    assert.equal(bmf['command/execute-virtual-end-to-end-basic'].latency.value, 500);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

testParseGoBenchOutput();
testValidateBmfRejectsInvalidPayload();
testTrackedRegexUsesExplicitBenchmarks();
testEmitterEndToEndWithFixtures();

console.log('bench-bmf tests passed');
