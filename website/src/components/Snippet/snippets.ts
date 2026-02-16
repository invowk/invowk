/**
 * Centralized code snippets for documentation.
 *
 * These snippets are shared across all translations to:
 * 1. Avoid duplication of code blocks in translation files
 * 2. Ensure consistency when code examples are updated
 * 3. Reduce translation maintenance burden
 */

import { gettingStartedSnippets } from './data/getting-started';
import { cliSnippets } from './data/cli';
import { coreConceptsSnippets } from './data/core-concepts';
import { runtimeModesSnippets } from './data/runtime-modes';
import { dependenciesSnippets } from './data/dependencies';
import { environmentSnippets } from './data/environment';
import { flagsArgsSnippets } from './data/flags-args';
import { advancedSnippets } from './data/advanced';
import { modulesSnippets } from './data/modules';
import { tuiSnippets } from './data/tui';
import { configSnippets } from './data/config';

export interface Snippet {
  /** The programming language for syntax highlighting */
  language: string;
  /** The code content */
  code: string;
}

/**
 * All available snippets aggregated from individual section files.
 */
export const snippets = {
  ...gettingStartedSnippets,
  ...cliSnippets,
  ...coreConceptsSnippets,
  ...runtimeModesSnippets,
  ...dependenciesSnippets,
  ...environmentSnippets,
  ...flagsArgsSnippets,
  ...advancedSnippets,
  ...modulesSnippets,
  ...tuiSnippets,
  ...configSnippets,
} as const;

// Type-safe snippet IDs
export type SnippetId = keyof typeof snippets;

// Helper to get all snippet IDs for documentation/tooling
export const snippetIds = Object.keys(snippets) as SnippetId[];
