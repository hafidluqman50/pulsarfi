'use client';

import { PSTOCKS, fmtPct } from '@/lib/data';

export function PriceTicker() {
  const tickerItems = PSTOCKS.map(stock => ({
    ticker:        stock.ticker,
    price:         stock.price,
    changePercent: stock.change24h,
  }));

  const renderTickerRow = (animationKey: string) => (
    <div key={animationKey} style={{ display: "flex", gap: 48 }}>
      {tickerItems.map(stockItem => (
        <div key={animationKey + stockItem.ticker} style={{ display: "flex", alignItems: "baseline", gap: 8, whiteSpace: "nowrap" }}>
          <span style={{ fontWeight: 600, fontSize: 12 }}>{stockItem.ticker}</span>
          <span className="mono" style={{ fontSize: 12, color: "var(--ink)" }}>${stockItem.price.toFixed(4)}</span>
          <span className="mono" style={{ fontSize: 11, color: stockItem.changePercent >= 0 ? "var(--positive)" : "var(--negative)" }}>
            {fmtPct(stockItem.changePercent)}
          </span>
        </div>
      ))}
    </div>
  );

  return (
    <div className="hairline" style={{ padding: "10px 0", background: "var(--canvas)", overflow: "hidden" }}>
      <div className="ticker-track">
        {renderTickerRow("first")}{renderTickerRow("second")}
      </div>
    </div>
  );
}
