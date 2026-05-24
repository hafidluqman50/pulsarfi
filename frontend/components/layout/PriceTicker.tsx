'use client';

import { PSTOCKS, fmtPct } from '@/lib/data';

export function PriceTicker() {
  const tickerItems = PSTOCKS.map(stock => ({
    ticker:        stock.ticker,
    price:         stock.price,
    changePercent: stock.change24h,
  }));

  const renderTickerRow = (animationKey: string) => (
    <div key={animationKey} className="flex gap-[48px]">
      {tickerItems.map(stockItem => (
        <div key={animationKey + stockItem.ticker} className="flex items-baseline gap-[8px] whitespace-nowrap">
          <span className="font-[600] text-[12px]">{stockItem.ticker}</span>
          <span className="mono text-[12px] text-[var(--ink)]">${stockItem.price.toFixed(4)}</span>
          <span className={`mono text-[11px] ${stockItem.changePercent >= 0 ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}>
            {fmtPct(stockItem.changePercent)}
          </span>
        </div>
      ))}
    </div>
  );

  return (
    <div className="hairline py-[10px] bg-[var(--canvas)] overflow-hidden">
      <div className="ticker-track">
        {renderTickerRow("first")}{renderTickerRow("second")}
      </div>
    </div>
  );
}
