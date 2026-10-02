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
    <div className="mt-[28px] grid grid-cols-[repeat(auto-fit,minmax(130px,1fr))] border border-[var(--hairline)] bg-[var(--putih)]">
      {stats.map(statItem => (
        <div key={statItem.label} className="border-r border-[var(--hairline)] px-[18px] py-[14px]">
          <div className="mb-[5px] text-[9px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]">{statItem.label}</div>
          <div className="mono text-[14px] font-[700] leading-[normal] [overflow-wrap:anywhere]">{statItem.value}</div>
        </div>
      ))}
    </div>
  );
}
