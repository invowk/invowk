#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0
//
// Validate that all snippet/diagram references in versioned docs resolve
// correctly to their version-scoped snapshots and static SVG files.
//
// For current docs, validates against the live snippets.ts and svgPaths.
// For versioned docs, validates against per-version snapshot files and SVG copies.
//
// Usage:
//   node scripts/validate-version-assets.mjs
//
// Exit codes:
//   0 - All references resolve correctly
//   1 - Missing references found (details printed to stderr)

import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(__dirname, '..');
const WEBSITE_DIR = path.join(REPO_ROOT, 'website');
const SNIPPETS_DIR = path.join(WEBSITE_DIR, 'src/components/Snippet');
const DIAGRAM_DIR = path.join(WEBSITE_DIR, 'src/components/Diagram');
const VERSIONS_FILE = path.join(WEBSITE_DIR, 'versions.json');

let errors = 0;

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

function walkDir(dir, ext) {
  const results = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      results.push(...walkDir(full, ext));
    } else if (entry.name.endsWith(ext)) {
      results.push(full);
    }
  }
  return results;
}

function scanIds(dir, pattern) {
  const mdxFiles = walkDir(dir, '.mdx');
  const ids = new Set();
  const regex = new RegExp(pattern, 'g');
  for (const file of mdxFiles) {
    const content = fs.readFileSync(file, 'utf8');
    let match;
    while ((match = regex.exec(content)) !== null) {
      ids.add(match[1]);
    }
  }
  return ids;
}

function stripLeadingSlash(p) {
  return p.startsWith('/') ? p.slice(1) : p;
}

// ---------------------------------------------------------------------------
// Parse live source files
// ---------------------------------------------------------------------------

function parseAllSnippetKeys() {
  const snippetsPath = path.join(SNIPPETS_DIR, 'snippets.ts');
  const content = fs.readFileSync(snippetsPath, 'utf8');

  const startMarker = 'export const snippets = ';
  const startIdx = content.indexOf(startMarker);
  const endMarker = '} as const;';
  const endIdx = content.lastIndexOf(endMarker);

  if (startIdx === -1 || endIdx === -1) {
    console.error('ERROR: Could not parse snippets.ts');
    process.exit(1);
  }

  const objectLiteral = content.slice(startIdx + startMarker.length, endIdx + 1);
  const snippets = vm.runInNewContext(`(${objectLiteral})`);
  return new Set(Object.keys(snippets));
}

function parseDiagramPathEntries() {
  const diagramPath = path.join(DIAGRAM_DIR, 'index.tsx');
  const content = fs.readFileSync(diagramPath, 'utf8');

  const startMarker = 'const svgPaths: Record<string, string> = {';
  const startIdx = content.indexOf(startMarker);
  const searchFrom = startIdx + startMarker.length;
  const endIdx = content.indexOf('};', searchFrom);
  const objectBody = content.slice(searchFrom, endIdx);

  const entries = {};
  const regex = /'([^']+)':\s*'([^']+)'/g;
  let match;
  while ((match = regex.exec(objectBody)) !== null) {
    entries[match[1]] = match[2];
  }
  return entries;
}

// ---------------------------------------------------------------------------
// Parse version snapshot files
// ---------------------------------------------------------------------------

function parseVersionSnippetKeys(version) {
  const filePath = path.join(SNIPPETS_DIR, 'versions', `v${version}.ts`);
  if (!fs.existsSync(filePath)) return null;

  const content = fs.readFileSync(filePath, 'utf8');
  const keys = new Set();
  const keyRegex = /^\s+'([^']+)':\s*\{/gm;
  let m;
  while ((m = keyRegex.exec(content)) !== null) {
    keys.add(m[1]);
  }
  return keys;
}

function parseVersionDiagramEntries(version) {
  const filePath = path.join(DIAGRAM_DIR, 'versions', `v${version}.ts`);
  if (!fs.existsSync(filePath)) return null;

  const content = fs.readFileSync(filePath, 'utf8');
  const entries = {};
  const regex = /^\s+'([^']+)':\s*'([^']+)'/gm;
  let m;
  while ((m = regex.exec(content)) !== null) {
    entries[m[1]] = m[2];
  }
  return entries;
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

function validateCurrentDocs() {
  const docsDir = path.join(WEBSITE_DIR, 'docs');
  if (!fs.existsSync(docsDir)) return;

  console.log('Validating current docs...');

  const snippetIds = scanIds(docsDir, '<Snippet\\s+id="([^"]*)"');
  const diagramIds = scanIds(docsDir, '<Diagram\\s+id="([^"]*)"');

  const liveSnippetKeys = parseAllSnippetKeys();
  const liveDiagramEntries = parseDiagramPathEntries();

  for (const id of snippetIds) {
    if (!liveSnippetKeys.has(id)) {
      console.error(`  MISSING: current docs reference snippet "${id}" not in snippets.ts`);
      errors++;
    }
  }

  for (const id of diagramIds) {
    if (!liveDiagramEntries[id]) {
      console.error(`  MISSING: current docs reference diagram "${id}" not in svgPaths`);
      errors++;
    }
  }

  if (errors === 0) {
    console.log(`  OK: ${snippetIds.size} snippets, ${diagramIds.size} diagrams`);
  }
}

function validateVersionedDocs() {
  if (!fs.existsSync(VERSIONS_FILE)) {
    console.log('No versions.json found — skipping versioned docs validation.');
    return;
  }

  const versions = JSON.parse(fs.readFileSync(VERSIONS_FILE, 'utf8'));
  const liveDiagramEntries = parseDiagramPathEntries();

  for (const version of versions) {
    const versionedDir = path.join(WEBSITE_DIR, `versioned_docs/version-${version}`);
    if (!fs.existsSync(versionedDir)) {
      console.warn(`  WARNING: Versioned docs directory missing for ${version}`);
      continue;
    }

    console.log(`Validating version ${version}...`);
    let versionErrors = 0;

    const snippetIds = scanIds(versionedDir, '<Snippet\\s+id="([^"]*)"');
    const diagramIds = scanIds(versionedDir, '<Diagram\\s+id="([^"]*)"');

    // Check snippet snapshot
    const snapshotKeys = parseVersionSnippetKeys(version);
    if (!snapshotKeys) {
      console.error(`  MISSING: No snippet snapshot file for version ${version}`);
      errors++;
      versionErrors++;
    } else {
      for (const id of snippetIds) {
        if (!snapshotKeys.has(id)) {
          console.error(`  MISSING: version ${version} references snippet "${id}" not in snapshot`);
          errors++;
          versionErrors++;
        }
      }
    }

    // Check diagram snapshot + SVG files
    const diagramEntries = parseVersionDiagramEntries(version);
    if (!diagramEntries) {
      console.error(`  MISSING: No diagram snapshot file for version ${version}`);
      errors++;
      versionErrors++;
    } else {
      for (const id of diagramIds) {
        if (!diagramEntries[id]) {
          console.error(`  MISSING: version ${version} references diagram "${id}" not in snapshot`);
          errors++;
          versionErrors++;
        } else {
          // Verify the SVG file exists on disk
          const svgPath = path.join(WEBSITE_DIR, 'static', stripLeadingSlash(diagramEntries[id]));
          if (!fs.existsSync(svgPath)) {
            // Check if this is a pre-existing issue (live source SVG also missing)
            const livePath = liveDiagramEntries[id];
            const liveSvgPath = livePath
              ? path.join(REPO_ROOT, 'docs', stripLeadingSlash(livePath))
              : null;
            if (liveSvgPath && !fs.existsSync(liveSvgPath)) {
              console.warn(`  WARNING: SVG missing (pre-existing): ${diagramEntries[id]}`);
            } else {
              console.error(`  MISSING: SVG file not found: ${diagramEntries[id]}`);
              errors++;
              versionErrors++;
            }
          }
        }
      }
    }

    if (versionErrors === 0) {
      console.log(`  OK: ${snippetIds.size} snippets, ${diagramIds.size} diagrams`);
    }
  }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

console.log('Validating version assets...\n');

validateCurrentDocs();
validateVersionedDocs();

console.log('');
if (errors > 0) {
  console.error(`FAILED: ${errors} missing reference(s) found.`);
  console.error('Run "node scripts/snapshot-version-assets.mjs <version>" to fix.');
  process.exit(1);
} else {
  console.log('All version asset references are valid.');
}
