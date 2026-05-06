#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export const BENCHMARK_REPORT_SCHEMA_VERSION = '1.0';
export const BENCHMARK_HISTORY_SCHEMA_VERSION = '1.0';

const currentFile = fileURLToPath(import.meta.url);
const repoRoot = path.resolve(path.dirname(currentFile), '..');
const releaseSuffixes = [
  '_bench-report.md',
  '_bench-report.json',
  '_bench-summary.svg',
  '_bench-raw.txt',
];

const userMetricIds = [
  'startup:version',
  'startup:help',
  'startup:cmd_help',
  'startup:cmd_list',
  'go:BenchmarkFullPipeline:mean_ns_op',
];

const developerMetricIds = [
  'go:BenchmarkCUEParsing:mean_ns_op',
  'go:BenchmarkCUEParsingComplex:mean_ns_op',
  'go:BenchmarkInvowkmodParsing:mean_ns_op',
  'go:BenchmarkDiscovery:mean_ns_op',
  'go:BenchmarkModuleValidation:mean_ns_op',
  'go:BenchmarkRuntimeVirtual:mean_ns_op',
];

function main() {
  const [command, ...argv] = process.argv.slice(2);
  const args = parseArgs(argv);

  try {
    switch (command) {
      case 'render':
        commandRender(args);
        break;
      case 'history':
        commandHistory(args);
        break;
      case 'validate-assets':
        commandValidateAssets(args);
        break;
      case 'validate-history':
        commandValidateHistory(args);
        break;
      default:
        usage();
        process.exit(command ? 1 : 0);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(`Error: ${message}`);
    process.exit(1);
  }
}

function usage() {
  console.log(`Usage:
  node scripts/benchmark-report.mjs render --out-dir <dir> --stamp <stamp> [inputs...]
  node scripts/benchmark-report.mjs history --input-dir <dir> --output <file> [--allow-partial]
  node scripts/benchmark-report.mjs history --tag <release-tag> --output <file> [--allow-partial]
  node scripts/benchmark-report.mjs validate-assets --dir <dir> --layout generated|release [--tag <tag>]
  node scripts/benchmark-report.mjs validate-history --input <file>`);
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
    } else {
      args[key] = next;
      i += 1;
    }
  }
  return args;
}

function requireArg(args, key) {
  const value = args[key];
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`--${key.replaceAll('_', '-')} is required`);
  }
  return value;
}

function optionalString(args, key, fallback = '') {
  const value = args[key];
  return typeof value === 'string' ? value : fallback;
}

function optionalNumber(args, key) {
  const value = args[key];
  if (value === undefined || value === true || value === '' || value === 'unknown') {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return null;
  }
  return parsed;
}

function commandRender(args) {
  const outDir = requireArg(args, 'out_dir');
  const stamp = requireArg(args, 'stamp');
  const startupRows = readStartupRows(requireArg(args, 'startup_tsv'));
  const goBenchmarks = readGoBenchmarkSummary(requireArg(args, 'go_summary_tsv'));
  const rawGoOutput = fs.readFileSync(requireArg(args, 'go_raw'), 'utf8');
  const rawStartupRows = fs.readFileSync(requireArg(args, 'startup_tsv'), 'utf8');
  const tag = optionalString(args, 'tag', process.env.TAG || '');
  const version = tag.startsWith('v') ? tag.slice(1) : tag;
  const generatedAt = optionalString(args, 'generated_at', new Date().toISOString());
  const report = buildReport({
    tag,
    version,
    generatedAt,
    mode: requireArg(args, 'mode'),
    startupSamples: Number(requireArg(args, 'startup_samples')),
    goBenchmarkCount: Number(requireArg(args, 'go_bench_count')),
    goBenchmarkStatus: requireArg(args, 'go_bench_status'),
    goBenchmarkWallSeconds: optionalNumber(args, 'go_bench_wall'),
    goBenchmarkEstimatedLoopSeconds: optionalNumber(args, 'go_bench_est_total'),
    goBenchmarkOverheadSeconds: optionalNumber(args, 'go_bench_overhead'),
    goBenchmarkTimingScope: requireArg(args, 'go_bench_timing_scope'),
    branch: requireArg(args, 'branch'),
    commit: requireArg(args, 'commit'),
    platform: requireArg(args, 'platform'),
    goVersion: requireArg(args, 'go_version'),
    binary: requireArg(args, 'binary'),
    goBenchmarkCommand: optionalString(args, 'go_bench_command'),
    startupRows,
    goBenchmarks,
    rawGoOutput,
    rawStartupRows,
  });

  const history = loadHistory(optionalString(args, 'history'));
  const historyWithCurrent = buildAggregateHistory([...history.records, report], {
    generatedAt,
    source: {
      kind: 'render',
      path: optionalString(args, 'history'),
    },
  });

  validateBenchmarkReport(report);

  fs.mkdirSync(outDir, { recursive: true });
  const mdPath = path.join(outDir, `${stamp}.md`);
  const jsonPath = path.join(outDir, `${stamp}.json`);
  const svgPath = path.join(outDir, `${stamp}_summary.svg`);
  const rawPath = path.join(outDir, `${stamp}_raw.txt`);

  report.artifacts = {
    markdown: path.basename(mdPath),
    json: path.basename(jsonPath),
    svg: path.basename(svgPath),
    raw: path.basename(rawPath),
  };

  const markdown = renderMarkdownReport(report, historyWithCurrent);
  const svg = renderSvgSummary(report, historyWithCurrent);

  validateSvg(svg);
  fs.writeFileSync(jsonPath, `${JSON.stringify(report, null, 2)}\n`);
  fs.writeFileSync(mdPath, markdown);
  fs.writeFileSync(svgPath, svg);
  fs.writeFileSync(rawPath, rawGoOutput);

  console.log(`Benchmark report written to: ${mdPath}`);
  console.log(`Benchmark JSON written to: ${jsonPath}`);
  console.log(`Benchmark SVG written to: ${svgPath}`);
  console.log(`Benchmark raw output written to: ${rawPath}`);
}

function commandHistory(args) {
  const tag = optionalString(args, 'tag');
  const tempDir = tag ? fs.mkdtempSync(path.join(os.tmpdir(), 'invowk-benchmark-assets-')) : '';
  const inputDir = tag ? tempDir : requireArg(args, 'input_dir');
  const output = requireArg(args, 'output');
  const allowPartial = args.allow_partial === true;
  const records = [];
  const warnings = [];

  try {
    if (tag) {
      downloadReleaseBenchmarkAssets(tag, inputDir);
    }
    const entries = fs.existsSync(inputDir) ? fs.readdirSync(inputDir).sort() : [];
    const jsonFiles = new Set(entries.filter((entry) => entry.endsWith('.json')));

    for (const entry of entries) {
      const full = path.join(inputDir, entry);
      if (entry.endsWith('.json')) {
        try {
          const record = JSON.parse(fs.readFileSync(full, 'utf8'));
          validateBenchmarkReport(record);
          records.push(record);
        } catch (error) {
          const message = `invalid JSON asset ${entry}: ${error.message}`;
          if (!allowPartial) {
            throw new Error(message);
          }
          warnings.push(message);
        }
        continue;
      }

      if (!entry.endsWith('.md')) {
        continue;
      }
      const adjacentJson = entry.replace(/\.md$/, '.json');
      if (jsonFiles.has(adjacentJson)) {
        continue;
      }
      try {
        const record = parseLegacyMarkdownReport(fs.readFileSync(full, 'utf8'), {
          sourcePath: full,
        });
        records.push(record);
      } catch (error) {
        const message = `could not parse legacy Markdown ${entry}: ${error.message}`;
        if (!allowPartial) {
          throw new Error(message);
        }
        warnings.push(message);
      }
    }
  } finally {
    if (tempDir) {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  }

  const history = buildAggregateHistory(records, {
    generatedAt: optionalString(args, 'generated_at', ''),
    source: {
      kind: tag ? 'release-assets' : 'local-assets',
      path: tag || inputDir,
    },
    warnings,
  });
  validateBenchmarkHistory(history);
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, `${JSON.stringify(history, null, 2)}\n`);
  console.log(`Benchmark history written to: ${output}`);
}

function downloadReleaseBenchmarkAssets(tag, outputDir) {
  fs.mkdirSync(outputDir, { recursive: true });
  execFileSync('gh', ['release', 'download', tag, '--pattern', '*bench*', '--dir', outputDir], {
    cwd: repoRoot,
    stdio: 'pipe',
  });
}

function commandValidateAssets(args) {
  const dir = requireArg(args, 'dir');
  const layout = optionalString(args, 'layout', 'generated');
  const tag = optionalString(args, 'tag');
  validateAssetDirectory(dir, { layout, tag });
  console.log(`Benchmark assets are valid: ${dir}`);
}

function commandValidateHistory(args) {
  const input = requireArg(args, 'input');
  const history = JSON.parse(fs.readFileSync(input, 'utf8'));
  validateBenchmarkHistory(history);
  console.log(`Benchmark history is valid: ${input}`);
}

export function buildReport(input) {
  const cpuModel = parseCpuModel(input.rawGoOutput);
  const platform = parsePlatform(input.platform);
  const warnings = [];

  if (input.goBenchmarkStatus !== 'ok') {
    warnings.push({
      code: 'go-benchmark-partial',
      message: input.goBenchmarkStatus,
    });
  }

  return {
    schema_version: BENCHMARK_REPORT_SCHEMA_VERSION,
    source_kind: 'json',
    release: {
      tag: input.tag,
      version: input.version,
      prerelease: input.version.includes('-'),
      generated_at: normalizeTimestamp(input.generatedAt),
      commit: input.commit,
      branch: input.branch,
    },
    run: {
      mode: input.mode,
      startup_samples: input.startupSamples,
      go_benchmark_count: input.goBenchmarkCount,
      go_benchmark_status: input.goBenchmarkStatus,
      go_benchmark_wall_seconds: input.goBenchmarkWallSeconds,
      go_benchmark_estimated_loop_seconds: input.goBenchmarkEstimatedLoopSeconds,
      go_benchmark_overhead_seconds: input.goBenchmarkOverheadSeconds,
      go_benchmark_timing_scope: input.goBenchmarkTimingScope,
      binary: input.binary,
      commands: {
        go_benchmark: input.goBenchmarkCommand,
      },
    },
    environment: {
      platform: input.platform,
      os: platform.os,
      kernel: platform.kernel,
      arch: platform.arch,
      cpu_model: cpuModel,
      logical_cpu_count: os.cpus().length,
      go_version: input.goVersion,
      runner: process.env.RUNNER_NAME || process.env.GITHUB_RUNNER_NAME || '',
    },
    startup: input.startupRows,
    go_benchmarks: input.goBenchmarks,
    raw: {
      startup_rows: input.rawStartupRows,
      go_benchmark_output: input.rawGoOutput,
    },
    warnings,
    artifacts: {},
  };
}

export function readStartupRows(file) {
  const ids = new Map([
    ['Version (--version)', 'version'],
    ['Help (--help)', 'help'],
    ['Cmd Help (cmd --help)', 'cmd_help'],
    ['Cmd List (cmd)', 'cmd_list'],
  ]);
  const lines = readNonEmptyLines(file);
  return lines.map((line) => {
    const [label, mean, min, max, samples] = line.split('\t');
    return {
      id: ids.get(label) || slug(label),
      label,
      mean_ms: parseRequiredNumber(mean, `startup ${label} mean_ms`),
      min_ms: parseRequiredNumber(min, `startup ${label} min_ms`),
      max_ms: parseRequiredNumber(max, `startup ${label} max_ms`),
      samples: parseRequiredInteger(samples, `startup ${label} samples`),
    };
  });
}

export function readGoBenchmarkSummary(file) {
  return readNonEmptyLines(file).map((line) => {
    const fields = line.split('\t');
    if (fields.length !== 11) {
      throw new Error(`invalid Go benchmark summary row: ${line}`);
    }
    const [
      rawName,
      samples,
      meanNs,
      minNs,
      maxNs,
      meanMs,
      meanIterations,
      estimatedRunMs,
      estimatedTotalSeconds,
      meanBytes,
      meanAllocs,
    ] = fields;
    const identity = normalizeBenchmarkName(rawName);
    return {
      id: identity.id,
      raw_name: rawName,
      cpu_suffix: identity.cpuSuffix,
      samples: parseRequiredInteger(samples, `${rawName} samples`),
      mean_ns_op: parseNullableNumber(meanNs),
      min_ns_op: parseNullableNumber(minNs),
      max_ns_op: parseNullableNumber(maxNs),
      mean_ms_op: parseNullableNumber(meanMs),
      mean_iterations_per_run: parseNullableNumber(meanIterations),
      estimated_run_ms: parseNullableNumber(estimatedRunMs),
      estimated_total_seconds: parseNullableNumber(estimatedTotalSeconds),
      mean_bytes_op: parseNullableNumber(meanBytes),
      mean_allocs_op: parseNullableNumber(meanAllocs),
    };
  });
}

export function normalizeBenchmarkName(rawName) {
  const match = rawName.match(/^(Benchmark.+?)-([0-9]+)$/);
  if (!match) {
    return {
      id: rawName,
      cpuSuffix: '',
    };
  }
  return {
    id: match[1],
    cpuSuffix: match[2],
  };
}

export function validateBenchmarkReport(report) {
  assertObject(report, 'report');
  if (report.schema_version !== BENCHMARK_REPORT_SCHEMA_VERSION) {
    throw new Error(`unsupported schema_version: ${report.schema_version}`);
  }
  assertObject(report.release, 'release');
  assertString(report.release.generated_at, 'release.generated_at');
  assertString(report.release.commit, 'release.commit');
  assertObject(report.run, 'run');
  assertString(report.run.mode, 'run.mode');
  assertInteger(report.run.startup_samples, 'run.startup_samples');
  assertInteger(report.run.go_benchmark_count, 'run.go_benchmark_count');
  assertObject(report.environment, 'environment');
  assertString(report.environment.platform, 'environment.platform');
  assertString(report.environment.go_version, 'environment.go_version');
  assertArray(report.startup, 'startup');
  assertArray(report.go_benchmarks, 'go_benchmarks');
  if (report.startup.length === 0) {
    throw new Error('startup must contain at least one scenario');
  }
  if (report.go_benchmarks.length === 0) {
    throw new Error('go_benchmarks must contain at least one benchmark');
  }
  for (const row of report.startup) {
    assertString(row.id, 'startup[].id');
    assertString(row.label, 'startup[].label');
    assertNumber(row.mean_ms, `startup.${row.id}.mean_ms`);
    assertNumber(row.min_ms, `startup.${row.id}.min_ms`);
    assertNumber(row.max_ms, `startup.${row.id}.max_ms`);
    assertInteger(row.samples, `startup.${row.id}.samples`);
  }
  for (const row of report.go_benchmarks) {
    assertString(row.id, 'go_benchmarks[].id');
    assertString(row.raw_name, 'go_benchmarks[].raw_name');
    assertInteger(row.samples, `go_benchmarks.${row.id}.samples`);
    assertNullableNumber(row.mean_ns_op, `go_benchmarks.${row.id}.mean_ns_op`);
    assertNullableNumber(row.mean_bytes_op, `go_benchmarks.${row.id}.mean_bytes_op`);
    assertNullableNumber(row.mean_allocs_op, `go_benchmarks.${row.id}.mean_allocs_op`);
  }
  assertObject(report.raw, 'raw');
  assertString(report.raw.go_benchmark_output, 'raw.go_benchmark_output');
}

export function renderMarkdownReport(report, history) {
  const lines = [];
  lines.push('# Invowk Benchmark Report');
  lines.push('');
  lines.push(`> Generated at: ${formatGeneratedAt(report.release.generated_at)}`);
  lines.push('');
  lines.push('## Performance Evolution');
  lines.push('');
  lines.push(...renderEvolutionMarkdown(report, history));
  lines.push('');
  lines.push('## Run Metadata');
  lines.push('');
  lines.push('| Field | Value |');
  lines.push('|---|---|');
  lines.push(`| Mode | \`${report.run.mode}\` |`);
  lines.push(`| Startup Samples | \`${report.run.startup_samples}\` per scenario |`);
  lines.push(`| Go Benchmark Count | \`${report.run.go_benchmark_count}\` |`);
  lines.push(`| Go Benchmark Status | \`${report.run.go_benchmark_status}\` |`);
  lines.push(`| Go Benchmark Wall Time (s) | \`${formatNullable(report.run.go_benchmark_wall_seconds)}\` |`);
  lines.push(`| Go Benchmark Estimated Loop Time (s) | \`${formatNullable(report.run.go_benchmark_estimated_loop_seconds)}\` |`);
  lines.push(`| Go Benchmark Harness/Overhead (s) | \`${formatNullable(report.run.go_benchmark_overhead_seconds)}\` |`);
  lines.push(`| Go Benchmark Timing Scope | \`${report.run.go_benchmark_timing_scope}\` |`);
  lines.push(`| Branch | \`${report.release.branch}\` |`);
  lines.push(`| Commit | \`${report.release.commit}\` |`);
  lines.push(`| Platform | \`${report.environment.platform}\` |`);
  lines.push(`| CPU | \`${report.environment.cpu_model || 'unknown'}\` |`);
  lines.push(`| Logical CPUs | \`${report.environment.logical_cpu_count}\` |`);
  lines.push(`| Go Version | \`${report.environment.go_version}\` |`);
  lines.push(`| Binary | \`${report.run.binary}\` |`);
  if (report.artifacts?.json || report.artifacts?.svg || report.artifacts?.raw) {
    lines.push(`| JSON Asset | \`${report.artifacts.json || 'not staged'}\` |`);
    lines.push(`| SVG Asset | \`${report.artifacts.svg || 'not staged'}\` |`);
    lines.push(`| Raw Asset | \`${report.artifacts.raw || 'not staged'}\` |`);
  }
  lines.push('');
  lines.push('## Startup Timings');
  lines.push('');
  lines.push('| Scenario | Mean (ms) | Min (ms) | Max (ms) | Samples |');
  lines.push('|---|---:|---:|---:|---:|');
  for (const row of report.startup) {
    lines.push(`| ${row.label} | ${fixed(row.mean_ms, 2)} | ${fixed(row.min_ms, 2)} | ${fixed(row.max_ms, 2)} | ${row.samples} |`);
  }
  lines.push('');
  lines.push('## Go Benchmarks (`internal/benchmark`)');
  lines.push('');
  lines.push('| Benchmark | Samples | Mean ns/op | Min ns/op | Max ns/op | Mean ms/op | Mean iters/run | Est run (ms) | Est total (s) | Mean B/op | Mean allocs/op |');
  lines.push('|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|');
  for (const row of report.go_benchmarks) {
    lines.push(`| \`${row.raw_name}\` | ${row.samples} | ${formatNullable(row.mean_ns_op)} | ${formatNullable(row.min_ns_op)} | ${formatNullable(row.max_ns_op)} | ${formatNullable(row.mean_ms_op)} | ${formatNullable(row.mean_iterations_per_run)} | ${formatNullable(row.estimated_run_ms)} | ${formatNullable(row.estimated_total_seconds)} | ${formatNullable(row.mean_bytes_op)} | ${formatNullable(row.mean_allocs_op)} |`);
  }
  if (report.warnings.length > 0) {
    lines.push('');
    for (const warning of report.warnings) {
      lines.push(`> Note: ${warning.message}`);
    }
  }
  lines.push('');
  lines.push('## Raw Startup Timing Data');
  lines.push('');
  lines.push('```text');
  lines.push(report.raw.startup_rows.trimEnd());
  lines.push('```');
  lines.push('');
  lines.push('## Raw Go Benchmark Output');
  lines.push('');
  lines.push('```text');
  lines.push(report.raw.go_benchmark_output.trimEnd());
  lines.push('```');
  lines.push('');
  return `${lines.join('\n')}\n`;
}

function renderEvolutionMarkdown(report, history) {
  const lines = [];
  const currentKey = recordKey(report);
  const records = history.records.filter((record) => recordKey(record) !== currentKey);
  const previous = records.at(-1);

  lines.push('Benchmark trends use lower-is-better semantics for time, allocation, and memory metrics.');
  lines.push('');
  renderComparisonSection(lines, 'Previous Release', report, previous);
  renderWindowSection(lines, 'Last 3 Months', report, history.windows.last_3_months);
  renderWindowSection(lines, 'Last 1 Year', report, history.windows.last_1_year);
  renderWindowSection(lines, 'All History', report, history.windows.all);
  renderEnvironmentNotes(lines, report, previous);
  return lines;
}

function renderComparisonSection(lines, heading, report, previous) {
  lines.push(`### ${heading}`);
  lines.push('');
  if (!previous) {
    lines.push('- Insufficient history for comparison.');
    lines.push('');
    return;
  }
  const comparisons = compareRecords(report, previous)
    .filter((item) => isCuratedMetric(item.metric_id))
    .slice(0, 8);
  if (comparisons.length === 0) {
    lines.push('- No comparable metrics were found.');
    lines.push('');
    return;
  }
  for (const item of comparisons) {
    lines.push(`- ${item.label}: ${formatValue(item.previous, item.unit)} -> ${formatValue(item.current, item.unit)}, ${formatPercentChange(item.percent_change)} ${item.direction}.`);
  }
  lines.push('');
}

function renderWindowSection(lines, heading, report, windowData) {
  lines.push(`### ${heading}`);
  lines.push('');
  if (!windowData || windowData.record_count < 2) {
    lines.push('- Insufficient history for this window.');
    lines.push('');
    return;
  }
  const currentMetrics = flattenRecordMetrics(report);
  const firstByMetric = new Map();
  for (const point of windowData.points) {
    if (!firstByMetric.has(point.metric_id) && currentMetrics.has(point.metric_id)) {
      firstByMetric.set(point.metric_id, point);
    }
  }
  const comparisons = [];
  for (const [metricId, current] of currentMetrics) {
    const first = firstByMetric.get(metricId);
    if (!first || !isCuratedMetric(metricId) || first.value === 0) {
      continue;
    }
    comparisons.push(buildComparison(metricId, current, first.value));
  }
  if (comparisons.length === 0) {
    lines.push('- No comparable metrics were found in this window.');
    lines.push('');
    return;
  }
  const ranked = comparisons.toSorted((a, b) => Math.abs(b.percent_change) - Math.abs(a.percent_change)).slice(0, 6);
  for (const item of ranked) {
    lines.push(`- ${item.label}: ${formatValue(item.previous, item.unit)} -> ${formatValue(item.current, item.unit)}, ${formatPercentChange(item.percent_change)} ${item.direction}.`);
  }
  lines.push('');
}

function renderEnvironmentNotes(lines, report, previous) {
  lines.push('### Environment Notes');
  lines.push('');
  if (!previous) {
    lines.push('- No prior environment available for compatibility comparison.');
    return;
  }
  const changed = [];
  for (const [label, key] of [
    ['Go version', 'go_version'],
    ['CPU', 'cpu_model'],
    ['OS', 'os'],
    ['Architecture', 'arch'],
    ['Runner', 'runner'],
  ]) {
    if ((report.environment[key] || '') !== (previous.environment[key] || '')) {
      changed.push(`${label}: \`${previous.environment[key] || 'unknown'}\` -> \`${report.environment[key] || 'unknown'}\``);
    }
  }
  if (report.run.mode !== previous.run.mode) {
    changed.push(`Benchmark mode: \`${previous.run.mode}\` -> \`${report.run.mode}\``);
  }
  if (report.run.startup_samples !== previous.run.startup_samples) {
    changed.push(`Startup samples: \`${previous.run.startup_samples}\` -> \`${report.run.startup_samples}\``);
  }
  if (changed.length === 0) {
    lines.push('- Current and previous benchmark environments are compatible based on recorded metadata.');
    return;
  }
  for (const note of changed) {
    lines.push(`- Reduced confidence: ${note}.`);
  }
}

export function renderSvgSummary(report, history) {
  const metrics = [...flattenRecordMetrics(report).values()].filter((metric) => isCuratedMetric(metric.metric_id)).slice(0, 8);
  const width = 960;
  const height = 520;
  const chartX = 60;
  const chartY = 110;
  const rowHeight = 36;
  const chartWidth = 720;
  const maxValue = Math.max(...metrics.map((metric) => metric.value), 1);
  const previous = history.records.filter((record) => recordKey(record) !== recordKey(report)).at(-1);
  const comparisons = previous ? new Map(compareRecords(report, previous).map((item) => [item.metric_id, item])) : new Map();
  const rows = metrics.map((metric, index) => {
    const barWidth = Math.max(2, Math.round((metric.value / maxValue) * chartWidth));
    const y = chartY + index * rowHeight;
    const comparison = comparisons.get(metric.metric_id);
    const delta = comparison ? `${formatPercentChange(comparison.percent_change)} ${comparison.direction}` : 'new';
    const color = comparison?.direction === 'slower' || comparison?.direction === 'more' ? '#dc2626' : '#16a34a';
    return `
      <text x="${chartX}" y="${y + 18}" class="metric">${escapeXml(metric.label)}</text>
      <rect x="${chartX + 250}" y="${y}" width="${barWidth}" height="22" rx="3" fill="#4f46e5" />
      <text x="${chartX + 260 + barWidth}" y="${y + 17}" class="value">${escapeXml(formatValue(metric.value, metric.unit))}</text>
      <text x="${chartX + 810}" y="${y + 17}" class="delta" fill="${color}">${escapeXml(delta)}</text>`;
  }).join('\n');
  const emptyState = metrics.length === 0 ? '<text x="60" y="160" class="metric">Insufficient benchmark data for graphical summary.</text>' : '';
  const warnings = summarizeConfidence(report, previous).map((warning, index) =>
    `<text x="60" y="${height - 58 + index * 18}" class="note">${escapeXml(warning)}</text>`).join('\n');

  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="title desc">
  <title id="title">Invowk benchmark summary</title>
  <desc id="desc">Release benchmark summary with lower-is-better performance metrics.</desc>
  <style>
    .title { font: 700 28px sans-serif; fill: #111827; }
    .subtitle { font: 14px sans-serif; fill: #4b5563; }
    .metric { font: 13px sans-serif; fill: #111827; }
    .value { font: 12px monospace; fill: #111827; }
    .delta { font: 700 12px sans-serif; }
    .note { font: 12px sans-serif; fill: #6b7280; }
    .axis { stroke: #d1d5db; stroke-width: 1; }
  </style>
  <rect width="100%" height="100%" fill="#ffffff" />
  <text x="40" y="46" class="title">Invowk Benchmark Summary</text>
  <text x="40" y="72" class="subtitle">Lower is better. Generated ${escapeXml(formatDate(report.release.generated_at))}. ${escapeXml(report.release.tag || report.release.commit)}</text>
  <line x1="${chartX + 250}" y1="${chartY - 18}" x2="${chartX + 250}" y2="${chartY + rowHeight * Math.max(metrics.length, 1)}" class="axis" />
  ${rows}
  ${emptyState}
  ${warnings}
</svg>
`;
}

export function validateSvg(svg) {
  if (!svg.includes('<svg') || !svg.includes('</svg>')) {
    throw new Error('SVG output is not parseable as SVG text');
  }
  if (!svg.includes('<rect') && !svg.includes('Insufficient benchmark data')) {
    throw new Error('SVG output has no data series or explicit empty state');
  }
}

export function buildAggregateHistory(records, options = {}) {
  const sorted = [...records]
    .filter(Boolean)
    .toSorted((a, b) => Date.parse(a.release.generated_at) - Date.parse(b.release.generated_at));
  const maxDate = sorted.length > 0 ? new Date(sorted.at(-1).release.generated_at) : new Date();
  const generatedAt = options.generatedAt || maxDate.toISOString();
  const points = sorted.flatMap((record) => [...flattenRecordMetrics(record).values()]);
  return {
    schema_version: BENCHMARK_HISTORY_SCHEMA_VERSION,
    generated_at: normalizeTimestamp(generatedAt),
    source: options.source || { kind: 'unknown', path: '' },
    warnings: options.warnings || [],
    windows: {
      last_3_months: buildWindow(sorted, points, addMonths(maxDate, -3)),
      last_1_year: buildWindow(sorted, points, addMonths(maxDate, -12)),
      all: buildWindow(sorted, points, null),
    },
    records: sorted.map((record) => sanitizeHistoryRecord(record)),
    metrics: points,
  };
}

function buildWindow(records, points, since) {
  const filteredRecords = since
    ? records.filter((record) => Date.parse(record.release.generated_at) >= since.getTime())
    : records;
  const keys = new Set(filteredRecords.map(recordKey));
  return {
    since: since ? since.toISOString() : '',
    record_count: filteredRecords.length,
    points: points.filter((point) => keys.has(point.record_key)),
  };
}

export function validateBenchmarkHistory(history) {
  assertObject(history, 'history');
  if (history.schema_version !== BENCHMARK_HISTORY_SCHEMA_VERSION) {
    throw new Error(`unsupported history schema_version: ${history.schema_version}`);
  }
  assertObject(history.windows, 'windows');
  for (const key of ['last_3_months', 'last_1_year', 'all']) {
    assertObject(history.windows[key], `windows.${key}`);
    assertArray(history.windows[key].points, `windows.${key}.points`);
  }
  assertArray(history.records, 'records');
  assertArray(history.metrics, 'metrics');
}

export function parseLegacyMarkdownReport(markdown, options = {}) {
  const generatedMatch = markdown.match(/Generated at:\s*([^\n]+)/);
  const commitMatch = markdown.match(/\| Commit \| `([^`]+)` \|/);
  const branchMatch = markdown.match(/\| Branch \| `([^`]+)` \|/);
  const modeMatch = markdown.match(/\| Mode \| `([^`]+)` \|/);
  const startupSamplesMatch = markdown.match(/\| Startup Samples \| `([0-9]+)`/);
  const goCountMatch = markdown.match(/\| Go Benchmark Count \| `([0-9]+)` \|/);
  const goVersionMatch = markdown.match(/\| Go Version \| `([^`]+)` \|/);
  const platformMatch = markdown.match(/\| Platform \| `([^`]+)` \|/);
  const binaryMatch = markdown.match(/\| Binary \| `([^`]+)` \|/);
  const rawGo = extractCodeBlock(markdown, '## Raw Go Benchmark Output');
  const rawStartup = extractCodeBlock(markdown, '## Raw Startup Timing Data');
  const startup = parseMarkdownTable(markdown, '## Startup Timings').map((row) => ({
    id: startupIdFromLabel(row.Scenario),
    label: row.Scenario,
    mean_ms: parseRequiredNumber(row['Mean (ms)'], `${row.Scenario} mean_ms`),
    min_ms: parseRequiredNumber(row['Min (ms)'], `${row.Scenario} min_ms`),
    max_ms: parseRequiredNumber(row['Max (ms)'], `${row.Scenario} max_ms`),
    samples: parseRequiredInteger(row.Samples, `${row.Scenario} samples`),
  }));
  const goBenchmarks = parseMarkdownTable(markdown, '## Go Benchmarks').map((row) => {
    const rawName = row.Benchmark.replaceAll('`', '');
    const identity = normalizeBenchmarkName(rawName);
    return {
      id: identity.id,
      raw_name: rawName,
      cpu_suffix: identity.cpuSuffix,
      samples: parseRequiredInteger(row.Samples, `${rawName} samples`),
      mean_ns_op: parseNullableNumber(row['Mean ns/op']),
      min_ns_op: parseNullableNumber(row['Min ns/op']),
      max_ns_op: parseNullableNumber(row['Max ns/op']),
      mean_ms_op: parseNullableNumber(row['Mean ms/op']),
      mean_iterations_per_run: parseNullableNumber(row['Mean iters/run']),
      estimated_run_ms: parseNullableNumber(row['Est run (ms)']),
      estimated_total_seconds: parseNullableNumber(row['Est total (s)']),
      mean_bytes_op: parseNullableNumber(row['Mean B/op']),
      mean_allocs_op: parseNullableNumber(row['Mean allocs/op']),
    };
  });
  const report = buildReport({
    tag: inferTagFromPath(options.sourcePath || ''),
    version: inferTagFromPath(options.sourcePath || '').replace(/^v/, ''),
    generatedAt: generatedMatch ? generatedMatch[1].trim() : new Date(0).toISOString(),
    mode: modeMatch?.[1] || 'unknown',
    startupSamples: Number(startupSamplesMatch?.[1] || 0),
    goBenchmarkCount: Number(goCountMatch?.[1] || 0),
    goBenchmarkStatus: 'legacy',
    goBenchmarkWallSeconds: null,
    goBenchmarkEstimatedLoopSeconds: null,
    goBenchmarkOverheadSeconds: null,
    goBenchmarkTimingScope: 'legacy Markdown parse',
    branch: branchMatch?.[1] || 'unknown',
    commit: commitMatch?.[1] || 'unknown',
    platform: platformMatch?.[1] || 'unknown',
    goVersion: goVersionMatch?.[1] || 'unknown',
    binary: binaryMatch?.[1] || 'unknown',
    goBenchmarkCommand: '',
    startupRows: startup,
    goBenchmarks,
    rawGoOutput: rawGo,
    rawStartupRows: rawStartup,
  });
  report.source_kind = 'legacy-markdown';
  report.warnings.push({
    code: 'legacy-markdown',
    message: 'Parsed from a Markdown-only benchmark report; confidence is reduced.',
  });
  validateBenchmarkReport(report);
  return report;
}

function parseMarkdownTable(markdown, heading) {
  const start = markdown.indexOf(heading);
  if (start === -1) {
    throw new Error(`missing ${heading}`);
  }
  const rest = markdown.slice(start);
  const lines = rest.split('\n');
  const tableLines = [];
  let started = false;
  for (const line of lines) {
    if (line.startsWith('|')) {
      tableLines.push(line);
      started = true;
    } else if (started) {
      break;
    }
  }
  if (tableLines.length < 3) {
    throw new Error(`missing table rows for ${heading}`);
  }
  const headers = splitMarkdownRow(tableLines[0]);
  return tableLines.slice(2).map((line) => {
    const cells = splitMarkdownRow(line);
    return Object.fromEntries(headers.map((header, index) => [header, cells[index] || '']));
  });
}

function splitMarkdownRow(line) {
  return line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((cell) => cell.trim());
}

function extractCodeBlock(markdown, heading) {
  const start = markdown.indexOf(heading);
  if (start === -1) {
    return '';
  }
  const rest = markdown.slice(start);
  const match = rest.match(/```text\n([\s\S]*?)\n```/);
  return match ? match[1] : '';
}

function flattenRecordMetrics(record) {
  const metrics = new Map();
  const key = recordKey(record);
  for (const row of record.startup || []) {
    metrics.set(`startup:${row.id}`, {
      record_key: key,
      release_tag: record.release.tag,
      release_version: record.release.version,
      generated_at: record.release.generated_at,
      source_kind: record.source_kind || 'json',
      audience: 'users',
      metric_id: `startup:${row.id}`,
      label: row.label,
      unit: 'ms',
      value: row.mean_ms,
      lower_is_better: true,
      confidence: confidenceForRecord(record),
      environment: record.environment,
    });
  }
  for (const row of record.go_benchmarks || []) {
    for (const [field, unit, audience] of [
      ['mean_ns_op', 'ns/op', 'developers'],
      ['mean_bytes_op', 'B/op', 'developers'],
      ['mean_allocs_op', 'allocs/op', 'developers'],
    ]) {
      const value = row[field];
      if (value === null || value === undefined) {
        continue;
      }
      const metricId = `go:${row.id}:${field}`;
      metrics.set(metricId, {
        record_key: key,
        release_tag: record.release.tag,
        release_version: record.release.version,
        generated_at: record.release.generated_at,
        source_kind: record.source_kind || 'json',
        audience,
        metric_id: metricId,
        benchmark_id: row.id,
        label: `${row.id} ${unit}`,
        unit,
        value,
        lower_is_better: true,
        confidence: confidenceForRecord(record),
        environment: record.environment,
      });
    }
  }
  return metrics;
}

function compareRecords(currentRecord, previousRecord) {
  const current = flattenRecordMetrics(currentRecord);
  const previous = flattenRecordMetrics(previousRecord);
  const comparisons = [];
  for (const [metricId, metric] of current) {
    const old = previous.get(metricId);
    if (!old || old.value === 0) {
      continue;
    }
    comparisons.push(buildComparison(metricId, metric, old.value));
  }
  return comparisons;
}

function buildComparison(metricId, currentMetric, previousValue) {
  const percentChange = ((currentMetric.value - previousValue) / previousValue) * 100;
  const lowerIsBetter = currentMetric.lower_is_better !== false;
  let direction = 'unchanged';
  if (Math.abs(percentChange) >= 0.5) {
    const gotWorse = lowerIsBetter ? percentChange > 0 : percentChange < 0;
    direction = gotWorse ? worseWord(currentMetric.unit) : betterWord(currentMetric.unit);
  }
  return {
    metric_id: metricId,
    label: currentMetric.label,
    unit: currentMetric.unit,
    current: currentMetric.value,
    previous: previousValue,
    percent_change: percentChange,
    direction,
  };
}

function betterWord(unit) {
  return unit === 'allocs/op' || unit === 'B/op' ? 'less' : 'faster';
}

function worseWord(unit) {
  return unit === 'allocs/op' || unit === 'B/op' ? 'more' : 'slower';
}

function isCuratedMetric(metricId) {
  return userMetricIds.includes(metricId) || developerMetricIds.includes(metricId);
}

function summarizeConfidence(report, previous) {
  const notes = [];
  if (!previous) {
    notes.push('Insufficient previous-release history; this summary shows current values.');
  } else if (summarizeEnvironmentDelta(report, previous).length > 0) {
    notes.push('Reduced confidence: benchmark environment changed since the previous record.');
  }
  if ((report.warnings || []).length > 0) {
    notes.push(`Warnings: ${report.warnings.map((warning) => warning.code).join(', ')}`);
  }
  return notes.slice(0, 2);
}

function summarizeEnvironmentDelta(report, previous) {
  const keys = ['go_version', 'cpu_model', 'os', 'arch', 'runner'];
  return keys.filter((key) => (report.environment[key] || '') !== (previous.environment[key] || ''));
}

function validateAssetDirectory(dir, options) {
  if (!fs.existsSync(dir)) {
    throw new Error(`asset directory does not exist: ${dir}`);
  }
  const files = fs.readdirSync(dir).filter((entry) => fs.statSync(path.join(dir, entry)).isFile()).sort();
  if (options.layout === 'release') {
    const tag = options.tag || process.env.TAG || '';
    if (!tag) {
      throw new Error('--tag is required for release layout validation');
    }
    const version = tag.replace(/^v/, '');
    const expected = releaseSuffixes.map((suffix) => `invowk_${version}${suffix}`);
    const missing = expected.filter((file) => !files.includes(file));
    const unexpected = files.filter((file) => !expected.includes(file));
    if (missing.length > 0 || unexpected.length > 0) {
      throw new Error(`invalid release benchmark assets; missing=[${missing.join(', ')}] unexpected=[${unexpected.join(', ')}] found=[${files.join(', ')}]`);
    }
    for (const file of expected) {
      validateAssetFile(path.join(dir, file));
    }
    return;
  }

  const jsonFiles = files.filter((file) => file.endsWith('.json'));
  if (jsonFiles.length === 0) {
    throw new Error(`no generated benchmark JSON files found in ${dir}`);
  }
  for (const jsonFile of jsonFiles) {
    const stem = jsonFile.replace(/\.json$/, '');
    for (const file of [`${stem}.md`, `${stem}.json`, `${stem}_summary.svg`, `${stem}_raw.txt`]) {
      if (!files.includes(file)) {
        throw new Error(`missing generated benchmark asset: ${file}`);
      }
      validateAssetFile(path.join(dir, file));
    }
  }
}

function validateAssetFile(file) {
  const stat = fs.statSync(file);
  if (stat.size === 0) {
    throw new Error(`empty benchmark asset: ${file}`);
  }
  if (file.endsWith('.json')) {
    validateBenchmarkReport(JSON.parse(fs.readFileSync(file, 'utf8')));
  }
  if (file.endsWith('.svg')) {
    validateSvg(fs.readFileSync(file, 'utf8'));
  }
}

function sanitizeHistoryRecord(record) {
  return {
    schema_version: record.schema_version,
    source_kind: record.source_kind || 'json',
    release: record.release,
    run: record.run,
    environment: record.environment,
    startup: record.startup,
    go_benchmarks: record.go_benchmarks,
    warnings: record.warnings || [],
    artifacts: record.artifacts || {},
  };
}

function loadHistory(historyPath) {
  if (!historyPath || !fs.existsSync(historyPath)) {
    return buildAggregateHistory([], {
      source: { kind: 'none', path: '' },
    });
  }
  const history = JSON.parse(fs.readFileSync(historyPath, 'utf8'));
  validateBenchmarkHistory(history);
  return history;
}

function readNonEmptyLines(file) {
  return fs.readFileSync(file, 'utf8').split(/\r?\n/).filter((line) => line.length > 0);
}

function parseCpuModel(rawGoOutput) {
  const match = rawGoOutput.match(/^cpu:\s*(.+)$/m);
  return match ? match[1].trim() : '';
}

function parsePlatform(platform) {
  const fields = platform.split(/\s+/);
  return {
    os: fields[0] || '',
    kernel: fields[1] || '',
    arch: fields.slice(2).join(' '),
  };
}

function startupIdFromLabel(label) {
  const mapping = {
    'Version (--version)': 'version',
    'Help (--help)': 'help',
    'Cmd Help (cmd --help)': 'cmd_help',
    'Cmd List (cmd)': 'cmd_list',
  };
  return mapping[label] || slug(label);
}

function inferTagFromPath(sourcePath) {
  const base = path.basename(sourcePath);
  const match = base.match(/invowk_([^_]+)_bench-report\.md$/);
  return match ? `v${match[1]}` : '';
}

function recordKey(record) {
  return `${record.release.tag || record.release.commit || 'unknown'}@${record.release.generated_at}`;
}

function confidenceForRecord(record) {
  if (record.source_kind === 'legacy-markdown') {
    return 'reduced';
  }
  if ((record.warnings || []).length > 0) {
    return 'reduced';
  }
  return 'normal';
}

function addMonths(date, months) {
  const result = new Date(date);
  result.setUTCMonth(result.getUTCMonth() + months);
  return result;
}

function normalizeTimestamp(value) {
  if (!value) {
    return new Date().toISOString();
  }
  const normalized = value.endsWith(' UTC') ? value.replace(' UTC', 'Z').replace(' ', 'T') : value;
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) {
    throw new Error(`invalid timestamp: ${value}`);
  }
  return date.toISOString();
}

function formatGeneratedAt(value) {
  return `${formatDate(value)} UTC`;
}

function formatDate(value) {
  return new Date(value).toISOString().replace('T', ' ').replace(/\.\d{3}Z$/, '');
}

function formatNullable(value) {
  return value === null || value === undefined ? 'unknown' : String(value);
}

function formatValue(value, unit) {
  if (unit === 'ms') {
    return `${fixed(value, 2)} ms`;
  }
  if (unit === 'ns/op') {
    return `${fixed(value, 2)} ns/op`;
  }
  if (unit === 'B/op') {
    return `${fixed(value, 2)} B/op`;
  }
  if (unit === 'allocs/op') {
    return `${fixed(value, 2)} allocs/op`;
  }
  return `${fixed(value, 2)} ${unit}`;
}

function formatPercentChange(value) {
  const sign = value > 0 ? '+' : '';
  return `${sign}${fixed(value, 1)}%`;
}

function fixed(value, decimals) {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) {
    return 'unknown';
  }
  return Number(value).toFixed(decimals);
}

function parseNullableNumber(value) {
  if (value === undefined || value === '-' || value === 'unknown' || value === '') {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    throw new Error(`expected number, got: ${value}`);
  }
  return parsed;
}

function parseRequiredNumber(value, label) {
  const parsed = parseNullableNumber(value);
  if (parsed === null) {
    throw new Error(`${label} is required`);
  }
  return parsed;
}

function parseRequiredInteger(value, label) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error(`${label} must be a non-negative integer`);
  }
  return parsed;
}

function slug(value) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
}

function escapeXml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function assertObject(value, label) {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
}

function assertArray(value, label) {
  if (!Array.isArray(value)) {
    throw new Error(`${label} must be an array`);
  }
}

function assertString(value, label) {
  if (typeof value !== 'string') {
    throw new Error(`${label} must be a string`);
  }
}

function assertNumber(value, label) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`${label} must be a finite number`);
  }
}

function assertNullableNumber(value, label) {
  if (value !== null && value !== undefined) {
    assertNumber(value, label);
  }
}

function assertInteger(value, label) {
  if (!Number.isInteger(value) || value < 0) {
    throw new Error(`${label} must be a non-negative integer`);
  }
}

export function gitReleaseAssets(tag) {
  const output = execFileSync('gh', ['release', 'view', tag, '--json', 'assets', '--jq', '.assets[].name'], {
    cwd: repoRoot,
    encoding: 'utf8',
  });
  return output.split(/\r?\n/).filter(Boolean);
}

if (process.argv[1] && path.resolve(process.argv[1]) === currentFile) {
  main();
}
