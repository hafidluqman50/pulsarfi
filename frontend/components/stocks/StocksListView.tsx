'use client';

import { useMemo, useState } from 'react';
import { AreaChart } from '@/components/charts/AreaChart';
import { useMarketStocks, useStockHistory, useStockPrice } from '@/http/market/hooks';
import { fmtPct } from '@/lib/data';
import { ChartSkeleton } from './ChartSkeleton';
import { StockRow, StockRowSkeleton } from './StockRow';

const TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;

export function StocksListView(): React.ReactNode {
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const { data: marketStocksData = [], isLoading } = useMarketStocks();
  const marketStocks = useMemo(
    () => Array.isArray(marketStocksData) ? marketStocksData : [],
    [marketStocksData],
  );

  const { data: ihsgData } = useStockPrice('IHSG');
  const { data: usdIdrData } = useStockPrice('USDIDR');
  const { data: ihsgHistory = [], isLoading: isIhsgHistoryLoading } = useStockHistory('IHSG', selectedTimeframe);
  const ihsgStats = useMemo(() => {
    const firstPoint = ihsgHistory[0];
    const lastPoint = ihsgHistory[ihsgHistory.length - 1];
    if (firstPoint && lastPoint && firstPoint.value > 0) {
      return {
        value: lastPoint.value,
        change: ((lastPoint.value - firstPoint.value) / firstPoint.value) * 100,
      };
    }
    return {
      value: ihsgData?.price,
      change: ihsgData?.change_24h ?? 0,
    };
  }, [ihsgData?.change_24h, ihsgData?.price, ihsgHistory]);
  const ihsgValue = ihsgStats.value;
  const ihsgChange = ihsgStats.change;
  const isIhsgPositive = ihsgChange >= 0;

  const sparklineData = useMemo(() =>
    marketStocks.reduce<Record<string, number[]>>((acc, stock) => {
      if (stock.ticker) {
        acc[stock.ticker] = stock.sparkline_7d ?? [];
      }
      return acc;
    }, {}),
  [marketStocks]);

  return (
    <div className="container pad-x !pb-[64px] !pt-[36px]">
      <div className="mb-[48px]">
        <div className="mb-[20px]">
          <div className="eyebrow mb-[8px] !text-[var(--body)]">
            Indeks Harga Saham Gabungan · IDX Composite
          </div>
          <div className="flex flex-wrap items-baseline gap-[20px]">
            {ihsgValue == null ? (
              <>
                <span className="skeleton h-[48px] w-[220px]" />
                <span className="skeleton h-[22px] w-[92px]" />
              </>
            ) : (
              <>
                <span className="display !text-[42px] !tracking-[-0.02em]">
                  {ihsgValue.toLocaleString('en-US', { minimumFractionDigits: 2 })}
                </span>
                <span className={`mono text-[18px] ${isIhsgPositive ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}>
                  {fmtPct(ihsgChange)} {selectedTimeframe}
                </span>
              </>
            )}
            {usdIdrData?.price ? (
              <span className="mono text-[13px] text-[var(--body)]">
                IDR/USD {usdIdrData.price.toLocaleString('en-US', { maximumFractionDigits: 0 })}
              </span>
            ) : (
              <span className="skeleton h-[18px] w-[130px]" />
            )}
          </div>
        </div>

        <div className="mb-[10px] flex items-center justify-between">
          <div className="range-pills">
            {TIMEFRAME_OPTIONS.map(timeframe => (
              <button
                key={timeframe}
                className={selectedTimeframe === timeframe ? 'active' : ''}
                onClick={() => setSelectedTimeframe(timeframe)}
              >
                {timeframe}
              </button>
            ))}
          </div>
        </div>

        <div className="border border-[var(--hairline)] bg-[var(--putih)] pb-[0] pt-[8px]">
          {isIhsgHistoryLoading || ihsgHistory.length === 0 ? (
            <ChartSkeleton />
          ) : (
            <AreaChart
              data={ihsgHistory}
              height={220}
              valueFormatter={value => value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            />
          )}
        </div>
      </div>

      <div>
        <div className="hairline-strong mb-[0] pb-[12px]">
          <span className="display !text-[26px]">pStocks</span>
          <div className="eyebrow mt-[4px] !text-[var(--body)]">
            {isLoading ? 'Loading market-ready equities' : `${marketStocks.length} market-ready equities · Arbitrum`}
          </div>
        </div>

        <div
          className="table-head-desktop hairline grid grid-cols-[2fr_1.2fr_1fr_1fr_80px] items-center gap-[16px] px-[16px] py-[12px]"
        >
          {['Stock', 'Sector', 'Price', '24h', '7d'].map((heading, columnIndex) => (
            <div
              key={heading}
              className={`eyebrow !text-[var(--body)] ${columnIndex >= 2 ? 'text-right' : 'text-left'}`}
            >
              {heading}
            </div>
          ))}
        </div>

        {!isLoading && marketStocks.length === 0 ? (
          <div className="hairline px-[16px] py-[18px] text-[var(--body)]">
            No pStocks have an active liquidity pool yet.
          </div>
        ) : null}

        {isLoading
          ? Array.from({ length: 6 }, (_, index) => <StockRowSkeleton key={index} />)
          : marketStocks.map(stock => (
            <StockRow
              key={stock.ticker}
              stock={stock}
              sparkline={sparklineData[stock.ticker] ?? []}
            />
          ))}
      </div>
    </div>
  );
}
