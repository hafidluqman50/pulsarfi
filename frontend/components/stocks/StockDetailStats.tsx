'use client';

import type { MarketStock } from '@/http/market/priceApi';
import { fmtIDRX, fmtNum } from '@/lib/data';
import { STOCK_LOT_SIZE } from '@/lib/stockDetail';

type StockDetailStatsProps = {
  stock: MarketStock;
  displayPrice: number;
  poolPrice?: number;
  tokenSupply: number | null;
};

export function StockDetailStats({
  stock,
  displayPrice,
  poolPrice,
  tokenSupply,
}: StockDetailStatsProps): React.ReactNode {
  const stats = [
    { label: 'IDX Ticker', value: stock.idx_ticker },
    { label: 'Sector', value: stock.sector ?? '—' },
    { label: 'Total Supply', value: tokenSupply == null ? '—' : fmtNum(tokenSupply, 0) },
    { label: 'IDX Mkt Cap', value: tokenSupply == null ? '—' : fmtIDRX(displayPrice * tokenSupply * STOCK_LOT_SIZE) },
    { label: 'Pool Price', value: poolPrice ? fmtIDRX(poolPrice) : '—' },
  ];

  return (
    <div className="stock-stats-grid">
      {stats.map(statItem => (
        <div key={statItem.label} className="stock-stats-cell">
          <div className="eyebrow mb-[5px] !text-[9px] !text-[var(--body)]">{statItem.label}</div>
          <div className="mono text-[14px] font-bold">{statItem.value}</div>
        </div>
      ))}
    </div>
  );
}
