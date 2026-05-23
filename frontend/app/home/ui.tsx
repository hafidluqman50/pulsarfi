'use client';

import { useState, useCallback } from 'react';
import { Layout } from '@/components/layout/Layout';
import { Masthead } from '@/components/layout/Masthead';
import { SwapView } from '@/components/swap/SwapView';
import { DEFAULT_PORTFOLIO, DEFAULT_COST_BASIS, Balances } from '@/lib/data';

const SWAP_HEADLINE = {
  eyebrow: 'The Trading Floor · Issue No. 0142',
  line1:   "Indonesia's market,",
  line2:   'unbound',
  line3:   'from its trading hours.',
};

export function SwapPage() {
  const [balances, setBalances]     = useState<Balances>(DEFAULT_PORTFOLIO);
  const [costBasis, setCostBasis]   = useState<Record<string, number>>(DEFAULT_COST_BASIS);

  const adjustCostBasisOnBuy = useCallback((
    ticker: string,
    quantityAdded: number,
    pricePaid: number,
  ) => {
    setCostBasis(previous => {
      const previousQuantity = balances[ticker] || 0;
      const previousAverage  = previous[ticker] ?? pricePaid;
      const newQuantity       = previousQuantity + quantityAdded;
      const newAverage        = newQuantity > 0
        ? (previousQuantity * previousAverage + quantityAdded * pricePaid) / newQuantity
        : pricePaid;
      return { ...previous, [ticker]: newAverage };
    });
  }, [balances]);

  return (
    <Layout>
      <Masthead />
      <SwapView
        balances={balances}
        setBalances={setBalances}
        buyAdjustCostBasis={adjustCostBasisOnBuy}
        headline={SWAP_HEADLINE}
      />
    </Layout>
  );
}
