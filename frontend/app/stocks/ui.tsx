'use client';

import { useState, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import { Layout } from '@/components/layout/Layout';
import { PStockMark } from '@/components/ui/PStockMark';
import { Sparkline } from '@/components/ui/Sparkline';
import { AreaChart } from '@/components/charts/AreaChart';
import { useMarketStocks, useStockPrice } from '@/http/market/hooks';
import type { MarketStock } from '@/http/market/priceApi';
import {
  PSTOCKS, seriesFor, sliceRange,
  fmtIDRX, fmtPct,
  TimePoint,
} from '@/lib/data';

const IDR_PER_USD = 16142;

function ihsgTimeSeries(days: number, endValue: number): TimePoint[] {
  let seed = 0x4948_5347;
  const rng = () => (seed = (seed * 1103515245 + 12345) >>> 0) / 0xffffffff;

  const startValue = endValue * 0.92;
  const outputPoints: TimePoint[] = [];
  const trend = (endValue - startValue) / days;
  let currentValue = startValue;
  const today = new Date();

  for (let dayIndex = 0; dayIndex < days; dayIndex++) {
    const noise = (rng() - 0.5) * currentValue * 0.012;
    const event = rng() < 0.02 ? (rng() - 0.4) * currentValue * 0.04 : 0;
    currentValue = Math.max(startValue * 0.8, currentValue + trend + noise + event);
    const date = new Date(today);
    date.setDate(date.getDate() - (days - 1 - dayIndex));
    outputPoints.push({ timestamp: date.getTime(), value: currentValue });
  }
  outputPoints[outputPoints.length - 1].value = endValue;
  return outputPoints;
}

const TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;

function StockRow({ stock, sparkline }: { stock: MarketStock; sparkline: number[] }): React.ReactNode {
  const router = useRouter();
  const fallback = PSTOCKS.find(item => item.ticker === stock.ticker);

  const price = stock.price ?? fallback?.price ?? 0;
  const change24h = stock.change_24h ?? fallback?.change24h ?? 0;
  const isPositive = change24h >= 0;
  const sector = stock.sector ?? fallback?.sector ?? 'Unclassified';

  return (
    <div
      className="hairline stock-list-row"
      onClick={() => router.push(`/stocks/${stock.ticker}`)}
    >
      <div className="flex min-w-0 items-center gap-[12px]">
        <PStockMark ticker={stock.ticker} size={34} />
        <div className="min-w-0">
          <div className="text-[14px] font-bold">{stock.ticker}</div>
          <div className="truncate text-[12px] text-[var(--body)]">
            {stock.stock_name}
          </div>
          <div className="only-mobile eyebrow mt-[3px] !text-[10px] !text-[var(--body)]">
            {sector}
          </div>
        </div>
      </div>

      <div className="stock-sector">
        <span className="border border-[var(--hairline)] px-[8px] py-[2px] text-[12px] text-[var(--body)]">
          {sector}
        </span>
      </div>

      <div className="mono text-right text-[14px] font-semibold">
        {fmtIDRX(price)}
      </div>

      <div
        className={`mono text-right text-[13px] ${isPositive ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}
      >
        {fmtPct(change24h)}
      </div>

      <div className="stock-sparkline flex justify-end">
        <Sparkline data={sparkline} positive={isPositive} width={72} height={28} />
      </div>
    </div>
  );
}

const IHSG_FALLBACK = 6_170;

export function StocksListPage(): React.ReactNode {
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const { data: marketStocksData = [], isLoading } = useMarketStocks();
  const marketStocks = useMemo(
    () => Array.isArray(marketStocksData) ? marketStocksData : [],
    [marketStocksData],
  );

  const { data: ihsgData } = useStockPrice('IHSG');
  const ihsgValue  = ihsgData?.price ?? IHSG_FALLBACK;
  const ihsgChange = ihsgData?.change_24h ?? 0;
  const isIhsgPositive = ihsgChange >= 0;

  // Series generated from the real fetched end value so chart and headline always match
  const ihsgFullSeries = useMemo(() => ihsgTimeSeries(365, ihsgValue), [ihsgValue]);
  const chartData      = useMemo(() => sliceRange(ihsgFullSeries, selectedTimeframe), [ihsgFullSeries, selectedTimeframe]);

  const sparklineData = useMemo(() =>
    marketStocks.reduce<Record<string, number[]>>((acc, stock) => {
      if (stock.ticker) {
        acc[stock.ticker] = stock.sparkline_7d?.length ? stock.sparkline_7d : seriesFor(stock.ticker, 28);
      }
      return acc;
    }, {}),
  [marketStocks]);

  return (
    <Layout>
      <div className="container pad-x !pb-[64px] !pt-[36px]">

        {/* ── IHSG Section ── */}
        <div className="mb-[48px]">
          <div className="mb-[20px]">
            <div className="eyebrow mb-[8px] !text-[var(--body)]">
              Indeks Harga Saham Gabungan · IDX Composite
            </div>
            <div className="flex flex-wrap items-baseline gap-[20px]">
              <span className="display !text-[42px] !tracking-[-0.02em]">
                {ihsgValue.toLocaleString('en-US', { minimumFractionDigits: 2 })}
              </span>
              <span className={`mono text-[18px] ${isIhsgPositive ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}>
                {fmtPct(ihsgChange)} 24h
              </span>
              <span className="mono text-[13px] text-[var(--body)]">
                IDR/USD {IDR_PER_USD.toLocaleString()}
              </span>
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
            <AreaChart
              data={chartData}
              height={220}
              valueFormatter={value => value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            />
          </div>
        </div>

        {/* ── Stock List ── */}
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

          {marketStocks.map(stock => (
            <StockRow
              key={stock.ticker}
              stock={stock}
              sparkline={sparklineData[stock.ticker] ?? []}
            />
          ))}
        </div>

      </div>
    </Layout>
  );
}
