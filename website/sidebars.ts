import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

/**
 * Invowk documentation sidebars.
 *
 * The documentation follows a gentle discovery/progression pattern:
 * - Start with installation and quickstart
 * - Move to core concepts (invkfile format, commands, namespaces)
 * - Then dive into features (runtime modes, dependencies, etc.)
 * - Finally, reference materials (CLI, schema)
 */
const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/installation',
        'getting-started/quickstart',
        'getting-started/your-first-invkfile',
      ],
    },
    {
      type: 'category',
      label: 'Core Concepts',
      collapsed: false,
      items: [
        'core-concepts/invkfile-format',
        'core-concepts/commands-and-namespaces',
        'core-concepts/implementations',
      ],
    },
    {
      type: 'category',
      label: 'Runtime Modes',
      items: [
        'runtime-modes/overview',
        'runtime-modes/native',
        'runtime-modes/virtual',
        'runtime-modes/container',
      ],
    },
    {
      type: 'category',
      label: 'Dependencies',
      items: [
        'dependencies/overview',
        'dependencies/tools',
        'dependencies/filepaths',
        'dependencies/commands',
        'dependencies/capabilities',
        'dependencies/env-vars',
        'dependencies/custom-checks',
      ],
    },
    {
      type: 'category',
      label: 'Flags and Arguments',
      items: [
        'flags-and-arguments/overview',
        'flags-and-arguments/flags',
        'flags-and-arguments/positional-arguments',
      ],
    },
    {
      type: 'category',
      label: 'Environment Configuration',
      items: [
        'environment/overview',
        'environment/env-files',
        'environment/env-vars',
        'environment/precedence',
      ],
    },
    {
      type: 'category',
      label: 'Advanced Features',
      items: [
        'advanced/interpreters',
        'advanced/workdir',
        'advanced/platform-specific',
        'advanced/interactive-mode',
      ],
    },
    {
      type: 'category',
      label: 'Modules',
      items: [
        'modules/overview',
        'modules/creating-modules',
        'modules/validating',
        'modules/distributing',
        {
          type: 'category',
          label: 'Module Dependencies',
          items: [
            'modules/dependencies/overview',
            'modules/dependencies/declaring-dependencies',
            'modules/dependencies/cli-commands',
            'modules/dependencies/lock-file',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'TUI Components',
      items: [
        'tui/overview',
        'tui/input-and-write',
        'tui/choose-and-confirm',
        'tui/filter-and-file',
        'tui/table-and-spin',
        'tui/pager',
        'tui/format-and-style',
      ],
    },
    {
      type: 'category',
      label: 'Configuration',
      items: [
        'configuration/overview',
        'configuration/options',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/cli',
        'reference/invkfile-schema',
        'reference/invkmod-schema',
        'reference/config-schema',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/overview',
        'architecture/c4-context',
        'architecture/c4-container',
        'architecture/c4-component-runtime',
        'architecture/c4-component-container',
        'architecture/execution-flow',
        'architecture/runtime-selection',
        'architecture/discovery',
      ],
    },
  ],
};

export default sidebars;
