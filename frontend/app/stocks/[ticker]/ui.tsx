'use client';

import { Layout } from '@/components/layout/Layout';
import { StockDetailView } from '@/components/stocks/StockDetailView';

export function StockDetailPage(): React.ReactNode {
  return (
    <Layout>
      <StockDetailView />
    </Layout>
  );
}
