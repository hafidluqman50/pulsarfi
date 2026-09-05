'use client';

import { Layout } from '@/components/layout/Layout';
import { PortfolioView } from '@/components/portfolio/PortfolioView';
import { QuasarPanel } from '@/components/agent/QuasarPanel';

export function PortfolioPage() {
  return (
    <Layout>
      <PortfolioView />
      <QuasarPanel />
    </Layout>
  );
}
