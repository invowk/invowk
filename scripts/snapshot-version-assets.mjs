#!/usr/bin/env node
// SPDX-License-Identifier: MPL-2.0
//
// Snapshot version assets (snippets + diagrams) for immutable versioned docs.
//
// This script is called by version-docs.sh after creating a docs version snapshot.
// It scans the versioned MDX files, extracts referenced snippet/diagram IDs,
// and creates per-version snapshot files that the components resolve at runtime.
//
// Usage:
//   node scripts/snapshot-version-assets.mjs <version> [--update]
//
// Arguments:
//   version   - Semver version WITHOUT 'v' prefix (e.g., 0.1.0-alpha.1)
//   --update  - Add missing entries without overwriting existing ones (for backports)

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

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

const version = process.argv[2];
const updateMode = process.argv.includes('--update');

if (!version) {
  console.error('Usage: node scripts/snapshot-version-assets.mjs <version> [--update]');
  process.exit(1);
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

/** Recursively collect files with a given extension. */
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

/** Convert a semver version string to a valid JS variable name. */
function versionToVarName(v) {
  return 'v' + v.replace(/[.\-]/g, '_');
}

/** Escape a string for safe embedding inside a template literal. */
function escapeForTemplateLiteral(str) {
  return str
    .replace(/\\/g, '\\\\')
    .replace(/`/g, '\\`')
    .replace(/\$\{/g, '\\${');
}

/** Strip leading slash from a URL path for safe path.join usage. */
function stripLeadingSlash(p) {
  return p.startsWith('/') ? p.slice(1) : p;
}

// ---------------------------------------------------------------------------
// Scanning: extract referenced IDs from versioned MDX files
// ---------------------------------------------------------------------------

function scanIds(version, pattern) {
  const versionedDir = path.join(WEBSITE_DIR, `versioned_docs/version-${version}`);
  if (!fs.existsSync(versionedDir)) {
    console.error(`ERROR: Versioned docs not found: ${versionedDir}`);
    process.exit(1);
  }

  const mdxFiles = walkDir(versionedDir, '.mdx');
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

function scanSnippetIds(version) {
  return scanIds(version, '<Snippet\\s+id="([^"]*)"');
}

function scanDiagramIds(version) {
  return scanIds(version, '<Diagram\\s+id="([^"]*)"');
}

// ---------------------------------------------------------------------------
// Parsing: extract current snippet and diagram data from source files
// ---------------------------------------------------------------------------

/**
 * Parse all snippet files in the data directory and return a combined snippets object.
 *
 * Each file is parsed using vm.runInNewContext to safely evaluate the object literal.
 */
function parseAllSnippets() {
  const dataDir = path.join(SNIPPETS_DIR, 'data');
  const snippetFiles = fs.readdirSync(dataDir).filter((f) => f.endsWith('.ts'));
  const allSnippets = {};

  for (const file of snippetFiles) {
    const filePath = path.join(dataDir, file);
    const content = fs.readFileSync(filePath, 'utf8');

    // Each file exports a named object: export const name = { ... };
    const startRegex = /export const \w+ = /;
    const match = content.match(startRegex);
    if (!match) {
      console.warn(`WARNING: Could not find export in ${file} — skipping.`);
      continue;
    }

    const startIdx = match.index + match[0].length;
    // Handle both `};` and `} satisfies Record<string, Snippet>;`
    const endIdx = Math.max(content.lastIndexOf('};'), content.lastIndexOf('} satisfies'));

    if (endIdx === -1) {
      console.error(`ERROR: Could not find object closing in ${file}`);
      process.exit(1);
    }

    const objectLiteral = content.slice(startIdx, endIdx + 1);
    try {
      const snippets = vm.runInNewContext(`(${objectLiteral})`);
      Object.assign(allSnippets, snippets);
    } catch (err) {
      console.error(`ERROR: Failed to parse snippets in ${file}:`, err);
      process.exit(1);
    }
  }

  return allSnippets;
}

/** Parse the Diagram/index.tsx svgPaths map. */
function parseDiagramPaths() {
  const diagramPath = path.join(DIAGRAM_DIR, 'index.tsx');
  const content = fs.readFileSync(diagramPath, 'utf8');

  const startMarker = 'const svgPaths: Record<string, string> = {';
  const startIdx = content.indexOf(startMarker);
  if (startIdx === -1) {
    console.error('ERROR: Could not find svgPaths in Diagram/index.tsx');
    process.exit(1);
  }

  const searchFrom = startIdx + startMarker.length;
  const endIdx = content.indexOf('};', searchFrom);
  const objectBody = content.slice(searchFrom, endIdx);

  const pathMap = {};
  const regex = /'([^']+)':\s*'([^']+)'/g;
  let match;
  while ((match = regex.exec(objectBody)) !== null) {
    pathMap[match[1]] = match[2];
  }
  return pathMap;
}

// ---------------------------------------------------------------------------
// Writing: generate per-version snapshot files
// ---------------------------------------------------------------------------

function writeVersionSnippets(version, snippetIds, allSnippets) {
  const versionsDir = path.join(SNIPPETS_DIR, 'versions');
  fs.mkdirSync(versionsDir, { recursive: true });
  const filePath = path.join(versionsDir, `v${version}.ts`);

  // --update mode: only add entries for IDs not already in the snapshot
  if (updateMode && fs.existsSync(filePath)) {
    const existing = fs.readFileSync(filePath, 'utf8');
    const existingKeys = new Set();
    const keyRegex = /^\s+'([^']+)':\s*\{/gm;
    let m;
    while ((m = keyRegex.exec(existing)) !== null) {
      existingKeys.add(m[1]);
    }

    const newIds = [...snippetIds].filter((id) => !existingKeys.has(id)).sort();
    if (newIds.length === 0) {
      console.log(`  No new snippet entries needed for v${version}.`);
      return;
    }

    const missing = newIds.filter((id) => !allSnippets[id]);
    if (missing.length > 0) {
      console.error(`ERROR: Missing snippets for version ${version}:`);
      for (const id of missing) console.error(`  - ${id}`);
      process.exit(1);
    }

    const newEntries = newIds
      .map((id) => {
        const s = allSnippets[id];
        return `  '${id}': {\n    language: '${s.language}',\n    code: \`${escapeForTemplateLiteral(s.code)}\`,\n  },`;
      })
      .join('\n');

    const insertIdx = existing.lastIndexOf('};');
    const updated = existing.slice(0, insertIdx) + newEntries + '\n' + existing.slice(insertIdx);
    fs.writeFileSync(filePath, updated);
    console.log(`  Added ${newIds.length} new snippet entries to v${version}.`);
    return;
  }

  // Normal mode: generate the full snapshot from current snippet data files
  const missing = [...snippetIds].filter((id) => !allSnippets[id]);
  if (missing.length > 0) {
    console.error(`ERROR: Missing snippets for version ${version}:`);
    for (const id of missing) console.error(`  - ${id}`);
    process.exit(1);
  }

  const snippetEntries = [...snippetIds]
    .sort()
    .map((id) => {
      const s = allSnippets[id];
      return `  '${id}': {\n    language: '${s.language}',\n    code: \`${escapeForTemplateLiteral(s.code)}\`,\n  },`;
    })
    .join('\n');

  const content = `// AUTO-GENERATED snippet snapshot for v${version} — do not edit manually.
// Snippet IDs extracted from versioned_docs/version-${version}/**/*.mdx
import type { Snippet } from '../snippets';

const snippets: Record<string, Snippet> = {
${snippetEntries}
};

export default snippets;
`;

  fs.writeFileSync(filePath, content);
}

function writeVersionDiagrams(version, diagramIds, pathMap) {
  const versionsDir = path.join(DIAGRAM_DIR, 'versions');
  fs.mkdirSync(versionsDir, { recursive: true });
  const filePath = path.join(versionsDir, `v${version}.ts`);

  const missing = [];
  const entries = [];

  for (const id of [...diagramIds].sort()) {
    const originalPath = pathMap[id];
    if (!originalPath) {
      missing.push(id);
      continue;
    }

    // Source: docs/ directory (served via staticDirectories: ['static', '../docs'])
    const srcFile = path.join(REPO_ROOT, 'docs', stripLeadingSlash(originalPath));

    // Version-specific URL path: /diagrams/rendered/... -> /diagrams/v{VERSION}/...
    const versionPath = originalPath.replace('/diagrams/rendered/', `/diagrams/v${version}/`);

    // Destination: website/static/ + version path
    const destFile = path.join(WEBSITE_DIR, 'static', stripLeadingSlash(versionPath));

    // Copy SVG (skip in update mode if destination already exists)
    if (!(updateMode && fs.existsSync(destFile))) {
      if (fs.existsSync(srcFile)) {
        fs.mkdirSync(path.dirname(destFile), { recursive: true });
        fs.copyFileSync(srcFile, destFile);
      } else {
        console.warn(`  WARNING: Source SVG not found (pre-existing): ${srcFile}`);
      }
    }

    entries.push({ id, path: versionPath });
  }

  if (missing.length > 0) {
    console.error(`ERROR: Missing diagrams for version ${version}:`);
    for (const id of missing) console.error(`  - ${id}`);
    process.exit(1);
  }

  // In update mode with existing file, merge new entries
  if (updateMode && fs.existsSync(filePath)) {
    const existing = fs.readFileSync(filePath, 'utf8');
    const existingKeys = new Set();
    const keyRegex = /^\s+'([^']+)':/gm;
    let m;
    while ((m = keyRegex.exec(existing)) !== null) {
      existingKeys.add(m[1]);
    }

    const newEntries = entries.filter((e) => !existingKeys.has(e.id));
    if (newEntries.length === 0) {
      console.log(`  No new diagram entries needed for v${version}.`);
      return;
    }

    const newLines = newEntries.map((e) => `  '${e.id}': '${e.path}',`).join('\n');
    const insertIdx = existing.lastIndexOf('};');
    const updated = existing.slice(0, insertIdx) + newLines + '\n' + existing.slice(insertIdx);
    fs.writeFileSync(filePath, updated);
    console.log(`  Added ${newEntries.length} new diagram entries to v${version}.`);
    return;
  }

  // Normal mode: generate full file
  const diagramEntries = entries.map((e) => `  '${e.id}': '${e.path}',`).join('\n');

  const content = `// AUTO-GENERATED diagram snapshot for v${version} — do not edit manually.
const diagramPaths: Record<string, string> = {
${diagramEntries}
};

export default diagramPaths;
`;

  fs.writeFileSync(filePath, content);
}

// ---------------------------------------------------------------------------
// Barrel files: auto-generated index.ts that maps versions to snapshots
// ---------------------------------------------------------------------------

function regenerateBarrels() {
  if (!fs.existsSync(VERSIONS_FILE)) {
    console.warn('WARNING: versions.json not found — skipping barrel generation.');
    return;
  }
  const versions = JSON.parse(fs.readFileSync(VERSIONS_FILE, 'utf8'));

  const snippetVersionsDir = path.join(SNIPPETS_DIR, 'versions');
  const diagramVersionsDir = path.join(DIAGRAM_DIR, 'versions');

  // Snippet barrel
  if (fs.existsSync(snippetVersionsDir)) {
    const snapshotVersions = versions.filter((v) =>
      fs.existsSync(path.join(snippetVersionsDir, `v${v}.ts`)),
    );

    const imports = snapshotVersions
      .map((v) => `import ${versionToVarName(v)} from './v${v}';`)
      .join('\n');

    const mapEntries = snapshotVersions
      .map((v) => `  '${v}': ${versionToVarName(v)},`)
      .join('\n');

    const content = `// AUTO-GENERATED by scripts/snapshot-version-assets.mjs — do not edit manually.
import type { Snippet } from '../snippets';
${imports}

const versionMap: Record<string, Record<string, Snippet>> = {
${mapEntries}
};

export function resolveVersionSnippets(version: string): Record<string, Snippet> | undefined {
  return versionMap[version];
}
`;
    fs.writeFileSync(path.join(snippetVersionsDir, 'index.ts'), content);
  }

  // Diagram barrel
  if (fs.existsSync(diagramVersionsDir)) {
    const snapshotVersions = versions.filter((v) =>
      fs.existsSync(path.join(diagramVersionsDir, `v${v}.ts`)),
    );

    const imports = snapshotVersions
      .map((v) => `import ${versionToVarName(v)} from './v${v}';`)
      .join('\n');

    const mapEntries = snapshotVersions
      .map((v) => `  '${v}': ${versionToVarName(v)},`)
      .join('\n');

    const content = `// AUTO-GENERATED by scripts/snapshot-version-assets.mjs — do not edit manually.
${imports}

const versionMap: Record<string, Record<string, string>> = {
${mapEntries}
};

export function resolveVersionDiagramPaths(version: string): Record<string, string> | undefined {
  return versionMap[version];
}
`;
    fs.writeFileSync(path.join(diagramVersionsDir, 'index.ts'), content);
  }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

function main() {
  console.log(`Snapshotting assets for version ${version}${updateMode ? ' (update mode)' : ''}...`);

  const snippetIds = scanSnippetIds(version);
  const diagramIds = scanDiagramIds(version);
  console.log(`  Found ${snippetIds.size} snippet IDs and ${diagramIds.size} diagram IDs.`);

  const allSnippets = parseAllSnippets();
  const pathMap = parseDiagramPaths();

  writeVersionSnippets(version, snippetIds, allSnippets);
  console.log(`  Wrote snippet snapshot: Snippet/versions/v${version}.ts`);

  writeVersionDiagrams(version, diagramIds, pathMap);
  console.log(`  Wrote diagram snapshot: Diagram/versions/v${version}.ts`);
  console.log(`  Copied ${diagramIds.size} SVGs to static/diagrams/v${version}/`);

  regenerateBarrels();
  console.log('  Regenerated barrel files.');

  console.log(`Asset snapshot complete for v${version}.`);
}

main();
