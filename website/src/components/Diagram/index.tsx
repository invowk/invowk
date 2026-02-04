import React from 'react';
import Mermaid from '@theme/Mermaid';
import { diagrams, type DiagramId } from './diagrams';

export interface DiagramProps {
  /** The unique identifier of the diagram to render */
  id: DiagramId;
}

/**
 * Diagram component for rendering Mermaid diagrams from a centralized registry.
 *
 * Usage in MDX:
 * ```mdx
 * import Diagram from '@site/src/components/Diagram';
 *
 * <Diagram id="architecture/c4-context" />
 * ```
 *
 * This component mirrors the Snippet pattern to:
 * 1. Avoid duplication of diagram code across translation files
 * 2. Ensure consistency when diagrams are updated
 * 3. Keep docs/architecture/*.md as the authoritative source for GitHub rendering
 */
export default function Diagram({ id }: DiagramProps): React.ReactElement {
  const diagram = diagrams[id];

  if (!diagram) {
    console.error(`Diagram with id "${id}" not found`);
    return (
      <div style={{ color: 'red', padding: '1rem', border: '1px solid red' }}>
        Diagram not found: {id}
      </div>
    );
  }

  return <Mermaid value={diagram.code} />;
}

// Re-export types for convenience
export type { DiagramId } from './diagrams';
