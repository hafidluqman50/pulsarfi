'use client';

import { useMemo, useState, useSyncExternalStore } from 'react';
import { AreaChart } from '@/components/charts/AreaChart';
import { useMarketStocks, useStockHistory, useStockPrice } from '@/http/market/hooks';
import { SplitFlap } from '@/components/ui/SplitFlap';
import { fmtPct } from '@/lib/data';
import { useViewportWidth } from '@/lib/useViewportWidth';
import { ChartSkeleton } from './ChartSkeleton';
import { BOARD_GRID, StockRow, StockRowSkeleton } from './StockRow';

const TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;
const CLOCK_REFRESH_MS = 15_000;
const MOBILE_BREAKPOINT = 720;

const jakartaClock = new Intl.DateTimeFormat('en-GB', {
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
  timeZone: 'Asia/Jakarta',
});

function subscribeToClock(onChange: () => void) {
  const intervalId = setInterval(onChange, CLOCK_REFRESH_MS);
  return () => clearInterval(intervalId);
}

function useJakartaClock() {
  return useSyncExternalStore(
    subscribeToClock,
    () => jakartaClock.format(new Date()),
    () => '00:00',
  );
}

export function StocksListView(): React.ReactNode {
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const clock = useJakartaClock();
  const isMobile = useViewportWidth() < MOBILE_BREAKPOINT;
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
    <div className="mx-auto w-full max-w-[1440px] px-[32px] pb-[64px] pt-[32px] max-[719px]:px-[16px] max-[719px]:pb-[48px] max-[719px]:pt-[24px]">
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))] items-end gap-[36px]">
        <div>
          <div className="eyebrow mb-[10px] !leading-[normal] !text-[var(--body)]">
            Indeks Harga Saham Gabungan · IDX Composite
          </div>
          <div className="flex flex-wrap items-center gap-[16px]">
            {ihsgValue == null ? (
              <>
                <span className="skeleton h-[44px] w-[220px]" />
                <span className="skeleton h-[22px] w-[92px]" />
              </>
            ) : (
              <>
                <SplitFlap
                  text={ihsgValue.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  size={isMobile ? 24 : 34}
                />
                <span className={`mono text-[18px] leading-[normal] ${isIhsgPositive ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}>
                  {fmtPct(ihsgChange)} {selectedTimeframe}
                </span>
              </>
            )}
          </div>
          <div className="mono mt-[10px] text-[13px] leading-[normal] text-[var(--body)]">
            {usdIdrData?.price ? (
              <>IDR/USD {usdIdrData.price.toLocaleString('en-US', { maximumFractionDigits: 0 })}</>
            ) : (
              <span className="skeleton block h-[18px] w-[130px]" />
            )}
          </div>
        </div>

        <div className="flex justify-end max-[719px]:justify-start">
          <div className="range-pills range-pills-full">
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
      </div>

      <div className="paper-stack mt-[16px] px-[16px] pb-[0] pt-[12px]">
        {isIhsgHistoryLoading || ihsgHistory.length === 0 ? (
          <ChartSkeleton />
        ) : (
          <AreaChart
            data={ihsgHistory}
            height={220}
            paper
            valueFormatter={value => value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          />
        )}
      </div>

      <div className="mt-[56px] flex flex-wrap items-baseline justify-between gap-[8px] border-b border-[var(--ink)] pb-[12px]">
        <div>
          <div className="display !text-[26px] !leading-[normal]">pStocks</div>
          <div className="eyebrow mt-[4px] !leading-[normal] !text-[var(--body)]">
            {isLoading ? 'Loading market-ready equities' : `${marketStocks.length} market-ready equities · Arbitrum`}
          </div>
        </div>
        <span className="eyebrow !leading-[normal] !text-[var(--body)]">
          Live board · updates every few seconds
        </span>
      </div>

      <div className="paper-stack mt-[20px] overflow-x-auto px-[clamp(12px,2vw,24px)] pb-[10px] pt-[18px]">
        <div className="min-w-[760px]">
          <div className="flex items-center justify-between border-b border-[var(--ink)] pb-[12px] text-[11px] font-[600] uppercase leading-[normal] tracking-[0.2em] text-[var(--body)]">
            <span className="flex items-center gap-[10px]">
              <span className="board-dot inline-block h-[8px] w-[8px] rounded-full bg-[var(--merah)]" />
              PulsarFi Board · 24/7
            </span>
            <span className="flex items-center gap-[10px]">
              WIB
              <SplitFlap text={clock} size={14} />
            </span>
          </div>

          <div className={`${BOARD_GRID} px-[6px] py-[10px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]`}>
            <span />
            <span>Ticker</span>
            <span>Company · Sector</span>
            <span>Price IDRX</span>
            <span>24h</span>
            <span className="text-right">7d</span>
          </div>

          {!isLoading && marketStocks.length === 0 ? (
            <div className="border-t border-[#efebe3] px-[6px] py-[18px] text-[var(--body)]">
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
    </div>
  );
}
