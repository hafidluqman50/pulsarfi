import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'PulsarFi Docs',
  tagline: 'Asset-backed IDX equities on Arbitrum Sepolia',
  favicon: 'img/logo-only-nobg.png',

  url: 'https://pulsarfi.local',
  baseUrl: '/',
  organizationName: 'horizonlabs',
  projectName: 'pulsarfi',

  onBrokenLinks: 'throw',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: 'content',
          routeBasePath: 'docs',
          sidebarPath: './sidebars.ts',
          editUrl: undefined,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/logo-nobg.png',
    navbar: {
      title: 'PulsarFi',
      logo: {
        alt: 'PulsarFi',
        src: 'img/logo-only-nobg.png',
        width: 40,
        height: 32,
      },
      items: [
        {to: '/docs/overview', label: 'Docs', position: 'left'},
        {to: '/docs/getting-started', label: 'Get Started', position: 'left'},
        {to: '/docs/contracts-faucet', label: 'Contracts & Faucet', position: 'left'},
        {
          href: 'https://sepolia.arbiscan.io/address/0x204488318C0E75978B3c851382Aa83f3065a8f5A',
          label: 'Arbiscan',
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
            {label: 'Overview', to: '/docs/overview'},
            {label: 'App Flow', to: '/docs/app-flow'},
            {label: 'Faucet IDRX', to: '/docs/contracts-faucet'},
          ],
        },
        {
          title: 'Network',
          items: [
            {
              label: 'Arbitrum Sepolia Explorer',
              href: 'https://sepolia.arbiscan.io',
            },
          ],
        },
      ],
      copyright: `Copyright ${new Date().getFullYear()} PulsarFi.`,
    },
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    prism: {
      theme: require('prism-react-renderer').themes.github,
      darkTheme: require('prism-react-renderer').themes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
