#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0

import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const currentFile = fileURLToPath(import.meta.url);
const repoRoot = path.resolve(path.dirname(currentFile), '..');

export const SHORT_BENCH_REGEX = '^Benchmark(CUEParsing|CUEParsingComplex|InvowkmodParsing|Discovery.*|ModuleValidation|FullPipeline)$';

const startupScenarios = [
  { id: 'version', name: 'startup/version', args: ['--version'] },
  { id: 'help', name: 'startup/help', args: ['--help'] },
  { id: 'cmd-help', name: 'startup/cmd-help', args: ['cmd', '--help'] },
  { id: 'cmd-list', name: 'startup/cmd-list', args: ['cmd'] },
];

function main() {
  try {
    const args = parseArgs(process.argv.slice(2));
    const output = optionalString(args, 'output', 'artifacts/benchmarks/invowk.bmf.json');
    const mode = optionalString(args, 'mode', 'short');
    const binary = resolveRepoPath(optionalString(args, 'binary', './bin/invowk'));
    const rawGoOutput = optionalString(args, 'raw_go_output');
    const goRawInput = optionalString(args, 'go_raw_input');
    const benchRegex = optionalString(args, 'bench_regex', mode === 'short' ? SHORT_BENCH_REGEX : '.');
    const benchCount = positiveInt(args, 'bench_count', 5);
    const benchTime = optionalString(args, 'bench_time', '1s');
    const startupSamples = positiveInt(args, 'startup_samples', 40);
    const goCmd = optionalString(args, 'go_cmd', process.env.GOCMD || 'go');
    const skipBuild = args.build === false || args.no_build === true;
    const skipStartup = args.skip_startup === true;

    if (mode !== 'short' && mode !== 'full') {
      throw new Error(`--mode must be "short" or "full", got: ${mode}`);
    }

    const bmf = {};

    if (!skipBuild) {
      const buildSeconds = runBuild({ goCmd, binary });
      bmf['build/invowk'] = {
        'build-time': {
          value: round(buildSeconds, 2),
        },
      };
    }

    assertExecutable(binary);

    const stat = fs.statSync(binary);
    bmf['binary/bin-invowk'] = {
      'file-size': {
        value: stat.size,
      },
    };

    if (!skipStartup) {
      for (const scenario of startupScenarios) {
        const summary = measureStartup(binary, scenario.args, startupSamples);
        bmf[scenario.name] = {
          latency: toMetric(summary.mean, summary.min, summary.max),
        };
      }
    }

    const goOutput = goRawInput
      ? fs.readFileSync(resolveRepoPath(goRawInput), 'utf8')
      : runGoBenchmarks({ goCmd, benchRegex, benchCount, benchTime });

    if (rawGoOutput) {
      const rawPath = resolveRepoPath(rawGoOutput);
      fs.mkdirSync(path.dirname(rawPath), { recursive: true });
      fs.writeFileSync(rawPath, goOutput);
    }

    const goMetrics = parseGoBenchOutput(goOutput);
    Object.assign(bmf, goMetricsToBmf(goMetrics));

    validateBmf(bmf);

    const outputPath = resolveRepoPath(output);
    fs.mkdirSync(path.dirname(outputPath), { recursive: true });
    fs.writeFileSync(outputPath, `${JSON.stringify(bmf, null, 2)}\n`);

    console.log(`BMF benchmark report written to: ${path.relative(repoRoot, outputPath)}`);
    console.log(`Benchmarks: ${Object.keys(bmf).length}`);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(`Error: ${message}`);
    process.exit(1);
  }
}

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith('--')) {
      throw new Error(`unexpected argument: ${token}`);
    }

    const key = token.slice(2).replaceAll('-', '_');
    const next = argv[i + 1];
    if (next === undefined || next.startsWith('--')) {
      args[key] = true;
      continue;
    }

    args[key] = next;
    i += 1;
  }
  return args;
}

function optionalString(args, key, fallback = '') {
  const value = args[key];
  return typeof value === 'string' ? value : fallback;
}

function positiveInt(args, key, fallback) {
  const value = args[key] ?? fallback;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`--${key.replaceAll('_', '-')} must be a positive integer, got: ${value}`);
  }
  return parsed;
}

function resolveRepoPath(value) {
  if (path.isAbsolute(value)) {
    return value;
  }
  return path.resolve(repoRoot, value);
}

function runBuild({ goCmd, binary }) {
  fs.mkdirSync(path.dirname(binary), { recursive: true });

  const version = commandOutput('git', ['describe', '--tags', '--always', '--dirty'], 'dev');
  const commit = commandOutput('git', ['rev-parse', '--short', 'HEAD'], 'unknown');
  const buildDate = new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
  const ldflags = [
    '-s',
    '-w',
    `-X github.com/invowk/invowk/cmd/invowk.Version=${version}`,
    `-X github.com/invowk/invowk/cmd/invowk.Commit=${commit}`,
    `-X github.com/invowk/invowk/cmd/invowk.BuildDate=${buildDate}`,
  ].join(' ');
  const env = { ...process.env };
  if (!env.GOAMD64 && commandOutput(goCmd, ['env', 'GOARCH'], '') === 'amd64') {
    env.GOAMD64 = 'v3';
  }

  const started = process.hrtime.bigint();
  const result = spawnSync(goCmd, ['build', '-trimpath', `-ldflags=${ldflags}`, '-o', binary, '.'], {
    cwd: repoRoot,
    encoding: 'utf8',
    env,
    stdio: 'inherit',
  });
  if (result.status !== 0) {
    throw new Error(`${goCmd} build failed with exit code ${result.status ?? 'unknown'}`);
  }
  const elapsed = Number(process.hrtime.bigint() - started) / 1_000_000_000;
  return elapsed;
}

function commandOutput(command, args, fallback) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  if (result.status !== 0) {
    return fallback;
  }
  const value = result.stdout.trim();
  return value || fallback;
}

function assertExecutable(binary) {
  if (!fs.existsSync(binary)) {
    throw new Error(`binary does not exist: ${binary}`);
  }
  try {
    fs.accessSync(binary, fs.constants.X_OK);
  } catch {
    throw new Error(`binary is not executable: ${binary}`);
  }
}

function measureStartup(binary, args, samples) {
  runChecked(binary, args);

  const values = [];
  for (let i = 0; i < samples; i += 1) {
    const started = process.hrtime.bigint();
    runChecked(binary, args);
    const elapsedNs = Number(process.hrtime.bigint() - started);
    values.push(elapsedNs);
  }

  return summarize(values);
}

function runChecked(command, args) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: 'utf8',
    stdio: 'ignore',
  });
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(' ')} failed with exit code ${result.status ?? 'unknown'}`);
  }
}

function runGoBenchmarks({ goCmd, benchRegex, benchCount, benchTime }) {
  const goArgs = [
    'test',
    './internal/benchmark',
    '-run=^$',
    `-bench=${benchRegex}`,
    '-benchmem',
    `-benchtime=${benchTime}`,
    `-count=${benchCount}`,
  ];
  const result = spawnSync(goCmd, goArgs, {
    cwd: repoRoot,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 100,
  });
  const output = `${result.stdout || ''}${result.stderr || ''}`;
  if (result.status !== 0) {
    process.stderr.write(output);
    throw new Error(`${goCmd} ${goArgs.join(' ')} failed with exit code ${result.status ?? 'unknown'}`);
  }
  return output;
}

export function parseGoBenchOutput(output) {
  const metrics = new Map();
  for (const line of output.split(/\r?\n/)) {
    if (!line.startsWith('Benchmark')) {
      continue;
    }

    const fields = line.trim().split(/\s+/);
    if (fields.length < 4) {
      continue;
    }

    const name = normalizeBenchmarkName(fields[0]);
    const row = metrics.get(name) || { ns: [], bytes: [], allocs: [] };

    for (let i = 2; i < fields.length; i += 1) {
      const unit = fields[i];
      const previous = Number(fields[i - 1]);
      if (!Number.isFinite(previous)) {
        continue;
      }
      if (unit === 'ns/op') {
        row.ns.push(previous);
      } else if (unit === 'B/op') {
        row.bytes.push(previous);
      } else if (unit === 'allocs/op') {
        row.allocs.push(previous);
      }
    }

    metrics.set(name, row);
  }

  if (metrics.size === 0) {
    throw new Error('no Go benchmark rows were parsed');
  }

  return metrics;
}

function normalizeBenchmarkName(name) {
  return name.replace(/-\d+$/, '');
}

export function goMetricsToBmf(metrics) {
  const bmf = {};
  for (const [name, values] of metrics) {
    if (values.ns.length === 0) {
      continue;
    }

    const benchmark = {
      latency: summaryToMetric(summarize(values.ns)),
    };

    if (values.bytes.length > 0) {
      benchmark.memory = summaryToMetric(summarize(values.bytes));
    }
    if (values.allocs.length > 0) {
      benchmark.allocations = summaryToMetric(summarize(values.allocs));
    }

    bmf[`go/${name}`] = benchmark;
  }

  if (Object.keys(bmf).length === 0) {
    throw new Error('no Go benchmark latency metrics were parsed');
  }

  return bmf;
}

function summarize(values) {
  if (values.length === 0) {
    throw new Error('cannot summarize an empty metric series');
  }
  let min = values[0];
  let max = values[0];
  let sum = 0;
  for (const value of values) {
    if (!Number.isFinite(value)) {
      throw new Error(`metric value is not finite: ${value}`);
    }
    min = Math.min(min, value);
    max = Math.max(max, value);
    sum += value;
  }
  return {
    mean: sum / values.length,
    min,
    max,
  };
}

function summaryToMetric(summary) {
  return toMetric(summary.mean, summary.min, summary.max);
}

function toMetric(value, lowerValue, upperValue) {
  return {
    value: round(value, 2),
    lower_value: round(lowerValue, 2),
    upper_value: round(upperValue, 2),
  };
}

function round(value, places) {
  const scale = 10 ** places;
  return Math.round(value * scale) / scale;
}

export function validateBmf(bmf) {
  if (!bmf || typeof bmf !== 'object' || Array.isArray(bmf)) {
    throw new Error('BMF root must be an object');
  }

  const benchmarkNames = Object.keys(bmf);
  if (benchmarkNames.length === 0) {
    throw new Error('BMF root must contain at least one benchmark');
  }

  for (const benchmarkName of benchmarkNames) {
    if (!benchmarkName || benchmarkName.length > 1024) {
      throw new Error(`invalid benchmark name: ${benchmarkName}`);
    }
    const benchmark = bmf[benchmarkName];
    if (!benchmark || typeof benchmark !== 'object' || Array.isArray(benchmark)) {
      throw new Error(`benchmark ${benchmarkName} must be an object`);
    }
    const measureNames = Object.keys(benchmark);
    if (measureNames.length === 0) {
      throw new Error(`benchmark ${benchmarkName} must contain at least one measure`);
    }
    for (const measureName of measureNames) {
      if (!measureName || measureName.length > 64) {
        throw new Error(`invalid measure name for ${benchmarkName}: ${measureName}`);
      }
      const metric = benchmark[measureName];
      if (!metric || typeof metric !== 'object' || Array.isArray(metric)) {
        throw new Error(`metric ${benchmarkName}/${measureName} must be an object`);
      }
      for (const key of ['value', 'lower_value', 'upper_value']) {
        if (metric[key] === undefined) {
          continue;
        }
        if (typeof metric[key] !== 'number' || !Number.isFinite(metric[key])) {
          throw new Error(`metric ${benchmarkName}/${measureName}.${key} must be a finite number`);
        }
      }
      if (metric.value === undefined) {
        throw new Error(`metric ${benchmarkName}/${measureName}.value is required`);
      }
    }
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === currentFile) {
  main();
}
