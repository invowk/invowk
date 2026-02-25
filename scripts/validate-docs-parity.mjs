#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0
//
// Validate docs parity between English docs and i18n locales.
//
// Checks:
// - File path parity for current docs and all versioned docs
// - Snippet ID parity in mirrored files
// - Diagram ID parity in mirrored files
//
// Usage:
//   node scripts/validate-docs-parity.mjs [options]
//
// Options:
//   --mode strict|report          Validation mode (default: strict)
//   --format text|json            Output format (default: text)
//   --all-locales                 Validate all locales under website/i18n
//   --locale <locale>             Validate a specific locale (repeatable)
//   --exceptions <path>           Path to exceptions JSON file

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(__dirname, '..');
const WEBSITE_DIR = path.join(REPO_ROOT, 'website');
const DOCS_DIR = path.join(WEBSITE_DIR, 'docs');
const I18N_DIR = path.join(WEBSITE_DIR, 'i18n');
const VERSIONS_FILE = path.join(WEBSITE_DIR, 'versions.json');
const DEFAULT_EXCEPTIONS_FILE = path.join(WEBSITE_DIR, 'docs-parity-exceptions.json');

const SNIPPET_ID_REGEX = /<Snippet\b[^>]*\bid=(["'])([^"']+)\1/g;
const DIAGRAM_ID_REGEX = /<Diagram\b[^>]*\bid=(["'])([^"']+)\1/g;

function usage() {
  console.error(`Usage: node scripts/validate-docs-parity.mjs [options]

Options:
  --mode strict|report
  --format text|json
  --all-locales
  --locale <locale>
  --exceptions <path>`);
}

function parseArgs(argv) {
  let mode = 'strict';
  let format = 'text';
  let allLocales = false;
  const locales = [];
  let exceptionsPath = DEFAULT_EXCEPTIONS_FILE;

  function requireValue(flag, index) {
    const value = argv[index + 1];
    if (!value || value.startsWith('--')) {
      console.error(`Missing value for ${flag}`);
      usage();
      process.exit(1);
    }
    return value;
  }

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--mode') {
      mode = requireValue(arg, i);
      i += 1;
    } else if (arg === '--format') {
      format = requireValue(arg, i);
      i += 1;
    } else if (arg === '--all-locales') {
      allLocales = true;
    } else if (arg === '--locale') {
      locales.push(requireValue(arg, i));
      i += 1;
    } else if (arg === '--exceptions') {
      exceptionsPath = requireValue(arg, i);
      i += 1;
    } else if (arg === '--help' || arg === '-h') {
      usage();
      process.exit(0);
    } else {
      console.error(`Unknown argument: ${arg}`);
      usage();
      process.exit(1);
    }
  }

  if (!['strict', 'report'].includes(mode)) {
    console.error(`Invalid --mode value: ${mode}`);
    usage();
    process.exit(1);
  }
  if (!['text', 'json'].includes(format)) {
    console.error(`Invalid --format value: ${format}`);
    usage();
    process.exit(1);
  }

  return { mode, format, allLocales, locales, exceptionsPath };
}

function walkMdxFiles(dir) {
  const files = [];
  if (!fs.existsSync(dir)) {
    return files;
  }
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...walkMdxFiles(full));
    } else if (entry.name.endsWith('.mdx')) {
      files.push(full);
    }
  }
  return files;
}

function scanIds(content, regex) {
  const ids = new Set();
  const pattern = new RegExp(regex);
  let match;
  while ((match = pattern.exec(content)) !== null) {
    ids.add(match[2]);
  }
  return ids;
}

function setDifference(a, b) {
  return [...a].filter((x) => !b.has(x));
}

function sortObjectArray(arr) {
  return [...arr].sort((a, b) => {
    if (a.locale !== b.locale) return a.locale.localeCompare(b.locale);
    if (a.version !== b.version) return a.version.localeCompare(b.version);
    if (a.path !== b.path) return a.path.localeCompare(b.path);
    if (a.type !== b.type) return a.type.localeCompare(b.type);
    const aId = a.id ?? '';
    const bId = b.id ?? '';
    return aId.localeCompare(bId);
  });
}

function listAvailableLocales() {
  if (!fs.existsSync(I18N_DIR)) {
    return [];
  }
  return fs
    .readdirSync(I18N_DIR, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
}

function normalizeException(raw) {
  return {
    locale: raw.locale,
    version: raw.version ?? 'current',
    path: raw.path ?? '',
    type: raw.type,
    id: raw.id ?? null,
    reason: raw.reason ?? 'No reason provided.',
  };
}

function loadExceptions(exceptionsPath) {
  if (!exceptionsPath) {
    return [];
  }
  const resolvedPath = path.resolve(exceptionsPath);
  if (!fs.existsSync(resolvedPath)) {
    console.warn(`WARNING: Exceptions file not found: ${resolvedPath}`);
    return [];
  }

  const parsed = JSON.parse(fs.readFileSync(resolvedPath, 'utf8'));
  const list = Array.isArray(parsed) ? parsed : parsed.exceptions;

  if (!Array.isArray(list)) {
    console.error(`ERROR: Invalid exceptions format in ${resolvedPath}.`);
    process.exit(1);
  }

  const normalized = list.map(normalizeException);
  for (const e of normalized) {
    if (!e.locale || !e.type) {
      console.error(`ERROR: Invalid exception entry. "locale" and "type" are required.`);
      process.exit(1);
    }
  }
  return normalized;
}

function isExcepted(finding, exceptions) {
  return exceptions.find((e) => {
    if (e.locale !== finding.locale) return false;
    if (e.version !== finding.version) return false;
    if (e.type !== finding.type) return false;
    if (e.path && e.path !== finding.path) return false;
    if (e.id && e.id !== finding.id) return false;
    return true;
  });
}

function relativeMdxMap(rootDir) {
  const map = new Map();
  const files = walkMdxFiles(rootDir);
  for (const file of files) {
    const rel = path.relative(rootDir, file).replaceAll(path.sep, '/');
    const content = fs.readFileSync(file, 'utf8');
    map.set(rel, {
      snippets: scanIds(content, SNIPPET_ID_REGEX),
      diagrams: scanIds(content, DIAGRAM_ID_REGEX),
    });
  }
  return map;
}

function compareTree({ locale, version, sourceDir, localeDir, findings }) {
  if (!fs.existsSync(sourceDir)) {
    findings.push({
      locale,
      version,
      path: '',
      type: 'missing-source-dir',
      id: null,
      message: `Source docs directory missing: ${sourceDir}`,
    });
    return;
  }

  if (!fs.existsSync(localeDir)) {
    findings.push({
      locale,
      version,
      path: '',
      type: 'missing-locale-dir',
      id: null,
      message: `Locale docs directory missing: ${localeDir}`,
    });
    return;
  }

  const sourceMap = relativeMdxMap(sourceDir);
  const localeMap = relativeMdxMap(localeDir);
  const sourceFiles = new Set(sourceMap.keys());
  const localeFiles = new Set(localeMap.keys());

  for (const rel of setDifference(sourceFiles, localeFiles).sort()) {
    findings.push({
      locale,
      version,
      path: rel,
      type: 'missing-file',
      id: null,
      message: `Missing translated file: ${rel}`,
    });
  }

  for (const rel of setDifference(localeFiles, sourceFiles).sort()) {
    findings.push({
      locale,
      version,
      path: rel,
      type: 'extra-file',
      id: null,
      message: `Extra translated file not present in English docs: ${rel}`,
    });
  }

  for (const rel of [...sourceFiles].sort()) {
    if (!localeFiles.has(rel)) {
      continue;
    }

    const sourceIds = sourceMap.get(rel);
    const localeIds = localeMap.get(rel);

    const missingSnippetIds = setDifference(sourceIds.snippets, localeIds.snippets).sort();
    const extraSnippetIds = setDifference(localeIds.snippets, sourceIds.snippets).sort();
    const missingDiagramIds = setDifference(sourceIds.diagrams, localeIds.diagrams).sort();
    const extraDiagramIds = setDifference(localeIds.diagrams, sourceIds.diagrams).sort();

    for (const id of missingSnippetIds) {
      findings.push({
        locale,
        version,
        path: rel,
        type: 'missing-snippet-id',
        id,
        message: `Missing Snippet id "${id}" in translated file ${rel}`,
      });
    }
    for (const id of extraSnippetIds) {
      findings.push({
        locale,
        version,
        path: rel,
        type: 'extra-snippet-id',
        id,
        message: `Extra Snippet id "${id}" in translated file ${rel}`,
      });
    }
    for (const id of missingDiagramIds) {
      findings.push({
        locale,
        version,
        path: rel,
        type: 'missing-diagram-id',
        id,
        message: `Missing Diagram id "${id}" in translated file ${rel}`,
      });
    }
    for (const id of extraDiagramIds) {
      findings.push({
        locale,
        version,
        path: rel,
        type: 'extra-diagram-id',
        id,
        message: `Extra Diagram id "${id}" in translated file ${rel}`,
      });
    }
  }
}

function main() {
  const { mode, format, allLocales, locales, exceptionsPath } = parseArgs(process.argv.slice(2));
  const availableLocales = listAvailableLocales();
  const selectedLocales = allLocales || locales.length === 0
    ? availableLocales
    : [...new Set(locales)].sort();
  const versions = fs.existsSync(VERSIONS_FILE)
    ? JSON.parse(fs.readFileSync(VERSIONS_FILE, 'utf8'))
    : [];
  const exceptions = loadExceptions(exceptionsPath);
  const findings = [];

  for (const locale of selectedLocales) {
    const localeRoot = path.join(I18N_DIR, locale, 'docusaurus-plugin-content-docs');
    const localeCurrentDir = path.join(localeRoot, 'current');
    compareTree({
      locale,
      version: 'current',
      sourceDir: DOCS_DIR,
      localeDir: localeCurrentDir,
      findings,
    });

    for (const version of versions) {
      compareTree({
        locale,
        version,
        sourceDir: path.join(WEBSITE_DIR, `versioned_docs/version-${version}`),
        localeDir: path.join(localeRoot, `version-${version}`),
        findings,
      });
    }
  }

  const sortedFindings = sortObjectArray(findings);
  const active = [];
  const excepted = [];
  for (const finding of sortedFindings) {
    const exception = isExcepted(finding, exceptions);
    if (exception) {
      excepted.push({ ...finding, exceptionReason: exception.reason });
    } else {
      active.push(finding);
    }
  }

  const summary = {
    mode,
    format,
    localesChecked: selectedLocales,
    versionsChecked: ['current', ...versions],
    findingsTotal: sortedFindings.length,
    findingsActive: active.length,
    findingsExcepted: excepted.length,
  };

  if (format === 'json') {
    console.log(JSON.stringify({ summary, active, excepted }, null, 2));
  } else {
    console.log('Validating docs parity...\n');
    console.log(`Locales: ${selectedLocales.join(', ') || '(none)'}`);
    console.log(`Versions: current${versions.length ? `, ${versions.join(', ')}` : ''}`);
    console.log('');

    if (active.length === 0) {
      console.log('Active parity findings: none');
    } else {
      console.log(`Active parity findings: ${active.length}`);
      for (const finding of active) {
        const idPart = finding.id ? ` [id=${finding.id}]` : '';
        console.log(
          `  - [${finding.locale}] [${finding.version}] [${finding.type}] ${finding.path}${idPart}`,
        );
      }
    }

    if (excepted.length > 0) {
      console.log(`\nExcepted findings: ${excepted.length}`);
      for (const finding of excepted) {
        console.log(
          `  - [${finding.locale}] [${finding.version}] [${finding.type}] ${finding.path} (${finding.exceptionReason})`,
        );
      }
    }
  }

  if (mode === 'strict' && active.length > 0) {
    process.exit(1);
  }
}

main();
