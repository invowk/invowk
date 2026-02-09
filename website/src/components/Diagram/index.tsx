import React from 'react';

/**
 * Mapping of diagram IDs to their SVG file paths.
 *
 * SVG files are pre-rendered from D2 sources and committed to the repository.
 * This eliminates runtime rendering and ensures consistent diagrams across
 * all platforms (GitHub, Docusaurus, local preview).
 *
 * The paths are relative to the static directory configured in docusaurus.config.ts.
 */
const svgPaths: Record<string, string> = {
  // C4 Diagrams
  'architecture/c4-context': '/diagrams/rendered/c4/context.svg',
  'architecture/c4-container': '/diagrams/rendered/c4/container.svg',
  'architecture/c4-component-runtime': '/diagrams/rendered/c4/component-runtime.svg',
  'architecture/c4-component-container': '/diagrams/rendered/c4/component-container.svg',

  // Execution Sequence Diagrams
  'architecture/execution-main': '/diagrams/rendered/sequences/execution-main.svg',
  'architecture/execution-container': '/diagrams/rendered/sequences/execution-container.svg',
  'architecture/execution-virtual': '/diagrams/rendered/sequences/execution-virtual.svg',

  // Error Flowchart
  'architecture/execution-errors': '/diagrams/rendered/flowcharts/execution-errors.svg',

  // Runtime Selection Flowcharts
  'architecture/runtime-decision': '/diagrams/rendered/flowcharts/runtime-decision.svg',
  'architecture/runtime-platform': '/diagrams/rendered/flowcharts/runtime-platform.svg',
  'architecture/runtime-native-check': '/diagrams/rendered/flowcharts/runtime-native-check.svg',
  'architecture/runtime-virtual-check': '/diagrams/rendered/flowcharts/runtime-virtual-check.svg',
  'architecture/runtime-container-check': '/diagrams/rendered/flowcharts/runtime-container-check.svg',
  'architecture/runtime-provision': '/diagrams/rendered/flowcharts/runtime-provision.svg',
  'architecture/runtime-ssh': '/diagrams/rendered/flowcharts/runtime-ssh.svg',

  // Discovery Flowcharts
  'architecture/discovery-flow': '/diagrams/rendered/flowcharts/discovery-flow.svg',
  'architecture/discovery-conflict': '/diagrams/rendered/flowcharts/discovery-conflict.svg',
  'architecture/discovery-module-structure': '/diagrams/rendered/flowcharts/discovery-module-structure.svg',
  'architecture/discovery-deps': '/diagrams/rendered/flowcharts/discovery-deps.svg',
  'architecture/discovery-search-paths': '/diagrams/rendered/flowcharts/discovery-search-paths.svg',
  'architecture/discovery-cache': '/diagrams/rendered/flowcharts/discovery-cache.svg',
  'architecture/discovery-not-found': '/diagrams/rendered/flowcharts/discovery-not-found.svg',
  'architecture/discovery-wrong-version': '/diagrams/rendered/flowcharts/discovery-wrong-version.svg',
};

export type DiagramId = keyof typeof svgPaths;

export interface DiagramProps {
  /** The unique identifier of the diagram to render */
  id: DiagramId;
  /** Optional alt text for accessibility (defaults to diagram ID) */
  alt?: string;
}

/**
 * Diagram component for rendering pre-rendered SVG diagrams.
 *
 * Diagrams are rendered from D2 sources using TALA layout engine and
 * committed as SVG files. This approach:
 *
 * 1. Eliminates runtime Mermaid rendering overhead
 * 2. Ensures identical diagrams on GitHub and Docusaurus
 * 3. Provides production-quality layouts via TALA
 * 4. Supports deterministic, reproducible diagram generation
 *
 * Usage in MDX:
 * ```mdx
 * import Diagram from '@site/src/components/Diagram';
 *
 * <Diagram id="architecture/c4-context" />
 * ```
 */
export default function Diagram({ id, alt }: DiagramProps): React.ReactElement {
  const path = svgPaths[id];

  if (!path) {
    console.error(`Diagram with id "${id}" not found. Available IDs:`, Object.keys(svgPaths));
    return (
      <div
        style={{
          color: 'red',
          padding: '1rem',
          border: '1px solid red',
          borderRadius: '4px',
          backgroundColor: 'rgba(255, 0, 0, 0.1)',
        }}
      >
        <strong>Diagram not found:</strong> {id}
        <br />
        <small>
          Run <code>make render-diagrams</code> to generate SVG files from D2 sources.
        </small>
      </div>
    );
  }

  return (
    <div className="diagram-wrapper">
      <img
        className="diagram-img"
        src={path}
        alt={alt || `Diagram: ${id}`}
      />
    </div>
  );
}

// Re-export types for convenience
export type { DiagramId };
