'use client';

import { useState } from 'react';
import { Layout } from '@/components/layout/Layout';
import { CustodianView } from '@/components/custodian/CustodianView';
import { DEFAULT_PORTFOLIO, Balances } from '@/lib/data';

export function CustodianPage() {
  const [balances, setBalances] = useState<Balances>(DEFAULT_PORTFOLIO);

  return (
    <Layout>
      <CustodianView
        balances={balances}
        setBalances={setBalances}
      />
    </Layout>
  );
}
