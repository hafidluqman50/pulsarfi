'use client';

import { Layout } from '@/components/layout/Layout';
import { SwapView } from '@/components/swap/SwapView';

const SWAP_HEADLINE = {
  eyebrow: 'The Trading Floor · Issue No. 0142',
  line1:   "Indonesia's market,",
  line2:   'unbound',
  line3:   'from its trading hours.',
};

export function SwapPage() {
  return (
    <Layout>
      <SwapView headline={SWAP_HEADLINE} />
    </Layout>
  );
}
