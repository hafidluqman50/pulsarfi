'use client';

import { useMemo } from 'react';
import { PSTOCKS, fmtPct, fmtIDRX } from '@/lib/data';
import { useMarketStocks } from '@/http/market/hooks';

const SKELETON_WIDTHS = [72, 88, 64, 96, 80, 76, 84, 68];

export function PriceTicker() {
  const { data: marketStocks, isLoading } = useMarketStocks();

  const tickerItems = useMemo(() => {
    if (marketStocks && marketStocks.length > 0) {
      return marketStocks.map(stock => ({
        ticker:        stock.ticker,
        price:         stock.price,
        changePercent: stock.change_24h,
      }));
    }
    return PSTOCKS.map(stock => ({
      ticker:        stock.ticker,
      price:         stock.price,
      changePercent: stock.change24h,
    }));
  }, [marketStocks]);

  if (isLoading && !marketStocks) {
    return (
      <div className="hairline py-[10px] bg-[var(--canvas)] overflow-hidden">
        <div className="flex gap-[48px] px-[24px]">
          {SKELETON_WIDTHS.map((width, index) => (
            <div key={index} className="flex items-baseline gap-[8px]">
              <span className="skeleton h-[12px] rounded-[2px]" style={{ width: 36 }} />
              <span className="skeleton h-[12px] rounded-[2px]" style={{ width }} />
              <span className="skeleton h-[11px] rounded-[2px]" style={{ width: 42 }} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  const renderTickerRow = (animationKey: string) => (
    <div key={animationKey} className="flex gap-[48px]">
      {tickerItems.map(stockItem => (
        <div key={animationKey + stockItem.ticker} className="flex items-baseline gap-[8px] whitespace-nowrap">
          <span className="font-[600] text-[12px]">{stockItem.ticker}</span>
          <span className="mono text-[12px] text-[var(--ink)]">{fmtIDRX(stockItem.price * 100)}</span>
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
