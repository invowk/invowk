import React from 'react';
import CodeBlock from '@theme/CodeBlock';
import { snippets, type SnippetId } from './snippets';

export interface SnippetProps {
  /** The unique identifier of the snippet to render */
  id: SnippetId;
  /** Optional title to display above the code block */
  title?: string;
  /** Whether to show line numbers (default: false for short snippets, true for long ones) */
  showLineNumbers?: boolean;
}

/**
 * Snippet component for rendering reusable code blocks across translations.
 * 
 * Usage in MDX:
 * ```mdx
 * import Snippet from '@site/src/components/Snippet';
 * 
 * <Snippet id="invkfile-basic-structure" />
 * <Snippet id="cli-list-commands" title="List all commands" />
 * ```
 */
export default function Snippet({ id, title, showLineNumbers }: SnippetProps): React.ReactElement {
  const snippet = snippets[id];

  if (!snippet) {
    console.error(`Snippet with id "${id}" not found`);
    return (
      <div style={{ color: 'red', padding: '1rem', border: '1px solid red' }}>
        Snippet not found: {id}
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
