import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'category',
      label: 'Product',
      items: ['overview', 'problem', 'why-idrx', 'business-flow'],
    },
    {
      type: 'category',
      label: 'System Design',
      items: ['architecture', 'protocol-design', 'app-flow', 'data-api'],
    },
    {
      type: 'category',
      label: 'Operations',
      items: ['getting-started', 'contracts-faucet'],
    },
  ],
};

export default sidebars;
