'use client';

import { Layout } from '@/components/layout/Layout';
import { StocksListView } from '@/components/stocks/StocksListView';

export function StocksListPage(): React.ReactNode {
  return (
    <Layout>
      <StocksListView />
    </Layout>
  );
}
