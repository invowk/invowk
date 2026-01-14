import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'Invowk™',
  tagline: 'A dynamically extensible command runner. Like `just`, but with superpowers.',
  favicon: 'img/favicon.ico',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true,
  },

  // Set the production url of your site here
  url: 'https://invowk.dev',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For custom domains: use '/'
  // For username.github.io/project-name: use '/project-name/'
  baseUrl: '/',

  // GitHub pages deployment config.
  organizationName: 'invowk',
  projectName: 'invowk',
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  // Markdown configuration
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  // Internationalization
  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'pt-BR'],
    localeConfigs: {
      en: {
        htmlLang: 'en-US',
        label: 'English',
      },
      'pt-BR': {
        htmlLang: 'pt-BR',
        label: 'Portugues (Brasil)',
      },
    },
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/invowk/invowk/tree/main/website/',
          // Versioning
          lastVersion: 'current',
          versions: {
            current: {
              label: 'Next',
              path: '',
            },
          },
        },
        blog: false, // Blog disabled until content is ready
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    [
      require.resolve('@easyops-cn/docusaurus-search-local'),
      /** @type {import("@easyops-cn/docusaurus-search-local").PluginOptions} */
      ({
        hashed: true,
        language: ['en', 'pt'],
        indexDocs: true,
        indexBlog: false,
        indexPages: false,
        docsRouteBasePath: '/docs',
        highlightSearchTermsOnTargetPage: true,
        searchResultLimits: 8,
        searchBarShortcut: true,
        searchBarShortcutHint: true,
      }),
    ],
  ],

  themeConfig: {
    image: 'img/invowk-social-card.png',
    colorMode: {
      defaultMode: 'dark',
      respectPrefersColorScheme: true,
    },
    announcementBar: {
      id: 'alpha_warning',
      content: '⚠️ <strong>Alpha Software</strong> — Invowk is under active development. The invkfile format and features may change between releases. <a target="_blank" rel="noopener noreferrer" href="https://github.com/invowk/invowk">Star us on GitHub</a> to follow progress!',
      backgroundColor: '#f59e0b',
      textColor: '#000000',
      isCloseable: false,
    },
    navbar: {
      title: 'Invowk',
      logo: {
        alt: 'Invowk Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        // {to: '/blog', label: 'Blog', position: 'left'}, // TODO: Enable when blog content is ready
        {
          type: 'docsVersionDropdown',
          position: 'right',
          dropdownActiveClassDisabled: true,
        },
        {
          type: 'localeDropdown',
          position: 'right',
        },
        {
          href: 'https://github.com/invowk/invowk',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/getting-started/installation',
            },
            {
              label: 'Core Concepts',
              to: '/docs/core-concepts/invkfile-format',
            },
            {
              label: 'CLI Reference',
              to: '/docs/reference/cli',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/invowk/invowk/discussions',
            },
            {
              label: 'Issues',
              href: 'https://github.com/invowk/invowk/issues',
            },
          ],
        },
        {
          title: 'More',
          items: [
            // {
            //   label: 'Blog',
            //   to: '/blog',
            // }, // TODO: Enable when blog content is ready
            {
              label: 'GitHub',
              href: 'https://github.com/invowk/invowk',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Danilo Cominotti Marques. invowk™ is a trademark of Danilo Cominotti Marques. Licensed under EPL-2.0.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'json', 'yaml', 'powershell', 'python', 'ruby', 'cue'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
