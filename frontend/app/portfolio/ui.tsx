'use client';

import { useState } from 'react';
import { Layout } from '@/components/layout/Layout';
import { PortfolioView } from '@/components/portfolio/PortfolioView';
import { DEFAULT_PORTFOLIO, DEFAULT_COST_BASIS, Balances } from '@/lib/data';

export function PortfolioPage() {
  const [balances, setBalances] = useState<Balances>(DEFAULT_PORTFOLIO);
  const [costBasis]             = useState<Record<string, number>>(DEFAULT_COST_BASIS);

  return (
    <Layout>
      <PortfolioView
        balances={balances}
        setBalances={setBalances}
        costBasis={costBasis}
      />
    </Layout>
  );
}
