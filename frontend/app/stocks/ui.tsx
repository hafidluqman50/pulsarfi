'use client';

import { useState, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import { Layout } from '@/components/layout/Layout';
import { PStockMark } from '@/components/ui/PStockMark';
import { Sparkline } from '@/components/ui/Sparkline';
import { AreaChart } from '@/components/charts/AreaChart';
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

const IHSG_FULL_SERIES   = ihsgTimeSeries(365);
const IHSG_LAST          = IHSG_FULL_SERIES[IHSG_FULL_SERIES.length - 1].value;
const IHSG_PREVIOUS      = IHSG_FULL_SERIES[IHSG_FULL_SERIES.length - 2].value;
const IHSG_CHANGE_PCT    = ((IHSG_LAST - IHSG_PREVIOUS) / IHSG_PREVIOUS) * 100;

const TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;

export function StocksListPage() {
  const router = useRouter();
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');

  const chartData = useMemo(() => sliceRange(IHSG_FULL_SERIES, selectedTimeframe), [selectedTimeframe]);

  const sparklineData = useMemo(() =>
    PSTOCKS.reduce<Record<string, number[]>>((accumulator, stock) => {
      accumulator[stock.ticker] = seriesFor(stock.ticker, 28);
      return accumulator;
    }, {}),
  []);

  const isIhsgPositive = IHSG_CHANGE_PCT >= 0;

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
                {IHSG_LAST.toLocaleString('en-US', { minimumFractionDigits: 2 })}
              </span>
              <span className="mono" style={{ fontSize: 18, color: isIhsgPositive ? 'var(--positive)' : 'var(--negative)' }}>
                {fmtPct(IHSG_CHANGE_PCT)} 24h
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
            <div className="eyebrow" style={{ color: 'var(--body)', marginTop: 4 }}>8 tokenized equities · Arbitrum</div>
          </div>

          {/* Table header */}
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

          {PSTOCKS.map(stock => {
            const isPositive = stock.change24h >= 0;
            return (
              <div
                key={stock.ticker}
                className="hairline stock-list-row"
                onClick={() => router.push(`/stocks/${stock.ticker}`)}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
                  <PStockMark ticker={stock.ticker} size={34} />
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontWeight: 700, fontSize: 14 }}>{stock.ticker}</div>
                    <div style={{ fontSize: 12, color: 'var(--body)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {stock.name}
                    </div>
                    <div className="only-mobile eyebrow" style={{ fontSize: 10, color: 'var(--body)', marginTop: 3 }}>
                      {stock.sector}
                    </div>
                  </div>
                </div>

                <div className="stock-sector">
                  <span style={{ fontSize: 12, color: 'var(--body)', border: '1px solid var(--hairline)', padding: '2px 8px' }}>
                    {stock.sector}
                  </span>
                </div>

                <div className="mono" style={{ textAlign: 'right', fontSize: 14, fontWeight: 600 }}>
                  {fmtIDRX(stock.price)}
                </div>

                <div
                  className="mono"
                  style={{ textAlign: 'right', fontSize: 13, color: isPositive ? 'var(--positive)' : 'var(--negative)' }}
                >
                  {fmtPct(stock.change24h)}
                </div>

                <div className="stock-sparkline" style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <Sparkline data={sparklineData[stock.ticker]} positive={isPositive} width={72} height={28} />
                </div>
              </div>
            );
          })}
        </div>

      </div>
    </Layout>
  );
}
