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
  fmtIDRX, fmtPct, fmtAxisDate,
  TimePoint,
} from '@/lib/data';

const IDR_PER_USD = 16142;

function ihsgTimeSeries(days = 365): TimePoint[] {
  let seed = 0x4948_5347;
  const rng = () => (seed = (seed * 1103515245 + 12345) >>> 0) / 0xffffffff;

  const endValue   = 7_234.56;
  const startValue = 6_510;
  const outputPoints: TimePoint[] = [];
  const trend = (endValue - startValue) / days;
  let currentValue = startValue;
  const today = new Date(2026, 4, 23);

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

const IHSG_FULL_SERIES = ihsgTimeSeries(365);

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
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
        <PStockMark ticker={stock.ticker} size={34} />
        <div style={{ minWidth: 0 }}>
          <div style={{ fontWeight: 700, fontSize: 14 }}>{stock.ticker}</div>
          <div style={{ fontSize: 12, color: 'var(--body)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {stock.stock_name}
          </div>
          <div className="only-mobile eyebrow" style={{ fontSize: 10, color: 'var(--body)', marginTop: 3 }}>
            {sector}
          </div>
        </div>
      </div>

      <div className="stock-sector">
        <span style={{ fontSize: 12, color: 'var(--body)', border: '1px solid var(--hairline)', padding: '2px 8px' }}>
          {sector}
        </span>
      </div>

      <div className="mono" style={{ textAlign: 'right', fontSize: 14, fontWeight: 600 }}>
        {fmtIDRX(price)}
      </div>

      <div
        className="mono"
        style={{ textAlign: 'right', fontSize: 13, color: isPositive ? 'var(--positive)' : 'var(--negative)' }}
      >
        {fmtPct(change24h)}
      </div>

      <div className="stock-sparkline" style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Sparkline data={sparkline} positive={isPositive} width={72} height={28} />
      </div>
    </div>
  );
}

export function StocksListPage(): React.ReactNode {
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const { data: marketStocksData = [], isLoading } = useMarketStocks();
  const marketStocks = Array.isArray(marketStocksData) ? marketStocksData : [];

  const chartData = useMemo(() => sliceRange(IHSG_FULL_SERIES, selectedTimeframe), [selectedTimeframe]);

  const sparklineData = useMemo(() =>
    marketStocks.reduce<Record<string, number[]>>((acc, stock) => {
      if (stock.ticker) {
        acc[stock.ticker] = stock.sparkline_7d?.length ? stock.sparkline_7d : seriesFor(stock.ticker, 28);
      }
      return acc;
    }, {}),
  [marketStocks]);

  const { data: ihsgData } = useStockPrice('IHSG');
  const ihsgValue = ihsgData?.price ?? IHSG_FULL_SERIES[IHSG_FULL_SERIES.length - 1].value;
  const ihsgChange = ihsgData?.change_24h ?? 0;
  const isIhsgPositive = ihsgChange >= 0;

  return (
    <Layout>
      <div className="container pad-x" style={{ paddingTop: 36, paddingBottom: 64 }}>

        {/* ── IHSG Section ── */}
        <div style={{ marginBottom: 48 }}>
          <div style={{ marginBottom: 20 }}>
            <div className="eyebrow" style={{ color: 'var(--body)', marginBottom: 8 }}>
              Indeks Harga Saham Gabungan · IDX Composite
            </div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 20, flexWrap: 'wrap' }}>
              <span className="display" style={{ fontSize: 42, letterSpacing: '-0.02em' }}>
                {ihsgValue.toLocaleString('en-US', { minimumFractionDigits: 2 })}
              </span>
              <span className="mono" style={{ fontSize: 18, color: isIhsgPositive ? 'var(--positive)' : 'var(--negative)' }}>
                {fmtPct(ihsgChange)} 24h
              </span>
              <span className="mono" style={{ fontSize: 13, color: 'var(--body)' }}>
                IDR/USD {IDR_PER_USD.toLocaleString()}
              </span>
            </div>
          </div>

          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
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

          <div style={{ border: '1px solid var(--hairline)', background: 'var(--putih)', padding: '8px 0 0' }}>
            <AreaChart
              data={chartData}
              height={220}
              valueFormatter={value => value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
              labelFormatter={(timestamp, range) => fmtAxisDate(timestamp, range)}
              range={selectedTimeframe}
            />
          </div>
        </div>

        {/* ── Stock List ── */}
        <div>
          <div className="hairline-strong" style={{ paddingBottom: 12, marginBottom: 0 }}>
            <span className="display" style={{ fontSize: 26 }}>pStocks</span>
            <div className="eyebrow" style={{ color: 'var(--body)', marginTop: 4 }}>
              {isLoading ? 'Loading market-ready equities' : `${marketStocks.length} market-ready equities · Arbitrum`}
            </div>
          </div>

          <div
            className="table-head-desktop hairline"
            style={{ display: 'grid', gridTemplateColumns: '2fr 1.2fr 1fr 1fr 80px', gap: 16, padding: '12px 16px', alignItems: 'center' }}
          >
            {['Stock', 'Sector', 'Price', '24h', '7d'].map((heading, columnIndex) => (
              <div
                key={heading}
                className="eyebrow"
                style={{ color: 'var(--body)', textAlign: columnIndex >= 2 ? 'right' : 'left' }}
              >
                {heading}
              </div>
            ))}
          </div>

          {!isLoading && marketStocks.length === 0 ? (
            <div className="hairline" style={{ padding: '18px 16px', color: 'var(--body)' }}>
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
