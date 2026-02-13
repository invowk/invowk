import React from 'react';
import CodeBlock from '@theme/CodeBlock';
import { useActiveDocContext } from '@docusaurus/plugin-content-docs/client';
import { snippets, type SnippetId } from './snippets';
import type { Snippet as SnippetType } from './snippets';
import { resolveVersionSnippets } from './versions';

export interface SnippetProps {
  /** The unique identifier of the snippet to render */
  id: SnippetId;
  /** Optional title to display above the code block */
  title?: string;
  /** Whether to show line numbers (default: false for short snippets, true for long ones) */
  showLineNumbers?: boolean;
}

/**
 * Resolves a snippet by ID, checking version-scoped snapshots first.
 *
 * For versioned docs (non-'current'), looks up the per-version snapshot
 * created at release time. Falls back to the live snippets map so that
 * current/next docs always render from the latest data.
 */
function resolveSnippet(id: string, versionName: string | undefined): SnippetType | undefined {
  if (versionName && versionName !== 'current') {
    const versionSnippets = resolveVersionSnippets(versionName);
    if (versionSnippets?.[id]) {
      return versionSnippets[id];
    }
  }
  return snippets[id as SnippetId];
}

/**
 * Snippet component for rendering reusable code blocks across translations.
 *
 * For versioned docs, snippets are resolved from immutable per-version
 * snapshots so that updates to current/next never affect released docs.
 *
 * Usage in MDX:
 * ```mdx
 * import Snippet from '@site/src/components/Snippet';
 *
 * <Snippet id="getting-started/invowkfile-basic-structure" />
 * <Snippet id="cli-list-commands" title="List all commands" />
 * ```
 */
export default function Snippet({ id, title, showLineNumbers }: SnippetProps): React.ReactElement {
  const versionName = useActiveDocContext('default')?.activeVersion?.name;
  const snippet = resolveSnippet(id, versionName);

  if (!snippet) {
    const versionInfo = versionName ? ` (version: ${versionName})` : '';
    console.error(`Snippet with id "${id}" not found${versionInfo}`);
    return (
      <div style={{ color: 'red', padding: '1rem', border: '1px solid red' }}>
        Snippet not found: {id}{versionInfo}
      </div>
    );
  }

  // Auto-detect line numbers: show for code with more than 10 lines
  const autoShowLineNumbers = showLineNumbers ?? snippet.code.split('\n').length > 10;

  return (
    <CodeBlock
      language={snippet.language}
      title={title}
      showLineNumbers={autoShowLineNumbers}
    >
      {snippet.code}
    </CodeBlock>
  );
}

// Re-export types for convenience
export type { SnippetId } from './snippets';
