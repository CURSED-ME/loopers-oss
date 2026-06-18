// @ts-check
import {themes as prismThemes} from 'prism-react-renderer';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Loopers',
  tagline: 'Break the loop before it breaks your budget.',
  favicon: 'img/favicon.ico',

  future: { v4: true },

  url: 'https://docs.tryloopers.com',
  baseUrl: '/',

  organizationName: 'CURSED-ME',
  projectName: 'loopers-oss',

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/CURSED-ME/loopers-oss/edit/main/Documentation/',
        },
        blog: {
          showReadingTime: true,
          feedOptions: { type: ['rss', 'atom'], xslt: true },
          editUrl: 'https://github.com/CURSED-ME/loopers-oss/edit/main/Documentation/',
          onInlineTags: 'warn',
          onInlineAuthors: 'warn',
          onUntruncatedBlogPosts: 'warn',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/loopers-social-card.png',
      colorMode: {
        defaultMode: 'light',
        disableSwitch: false,
        respectPrefersColorScheme: false,
      },
      navbar: {
        title: 'Loopers',
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docsSidebar',
            position: 'left',
            label: 'Docs',
          },
          { to: '/docs/reference/cli', label: 'CLI Reference', position: 'left' },
          { to: '/docs/architecture', label: 'Architecture', position: 'left' },
          { to: '/blog', label: 'Blog', position: 'left' },
          { href: 'https://tryloopers.com', label: 'Cloud ↗', position: 'right' },
          {
            href: 'https://github.com/CURSED-ME/loopers-oss',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Documentation',
            items: [
              { label: 'Getting Started', to: '/docs/getting-started' },
              { label: 'Architecture', to: '/docs/architecture' },
              { label: 'CLI Reference', to: '/docs/reference/cli' },
              { label: 'API Headers', to: '/docs/reference/headers' },
            ],
          },
          {
            title: 'Concepts',
            items: [
              { label: 'Budget Windows', to: '/docs/concepts/budget-windows' },
              { label: 'Session Budgets', to: '/docs/concepts/session-budgets' },
              { label: 'Concurrency & Correctness', to: '/docs/concepts/concurrency-correctness' },
            ],
          },
          {
            title: 'SDKs',
            items: [
              { label: 'Python SDK', to: '/docs/sdks/python' },
              { label: 'TypeScript SDK', to: '/docs/sdks/typescript' },
            ],
          },
          {
            title: 'Community',
            items: [
              { label: 'GitHub', href: 'https://github.com/CURSED-ME/loopers-oss' },
              { label: 'Loopers Cloud', href: 'https://tryloopers.com' },
              { label: 'Contributing', to: '/docs/contributing/guide' },
              { label: 'Security Policy', to: '/docs/security' },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Loopers. MIT License.`,
      },
      prism: {
        theme: prismThemes.vsDark,
        darkTheme: prismThemes.vsDark,
        additionalLanguages: ['bash', 'yaml', 'lua', 'go', 'python', 'typescript'],
      },
    }),
};

export default config;
