'use client';

import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { Layout } from '@/components/layout/Layout';
import { SwapModal } from '@/components/ui/SwapModal';
import { PStockMark } from '@/components/ui/PStockMark';
import { AreaChart } from '@/components/charts/AreaChart';
import { useStockPrice } from '@/http/market/hooks';
import { useReserves } from '@/http/custodian/hooks';
import {
  PSTOCKS, sliceRange,
  fmtIDRX, fmtPct, fmtNum,
} from '@/lib/data';
import {
  buildMinuteTimeSeries,
  buildStockTimeSeries,
  rawTokenToNumber,
  STOCK_LOT_SIZE,
  STOCK_NEWS,
  STOCK_TIMEFRAME_OPTIONS,
} from '@/lib/stockDetail';

export function StockDetailPage() {
  const params = useParams();
  const router = useRouter();
  const ticker = typeof params.ticker === 'string' ? params.ticker : '';

  const stock = PSTOCKS.find(stockItem => stockItem.ticker === ticker);
  const { data: idxPrice } = useStockPrice(stock?.ipo ?? '', 'idx');
  const { data: poolPrice } = useStockPrice(stock?.ipo ?? '');
  const { data: reserves = [] } = useReserves();

  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const [tradeOpen, setTradeOpen]                 = useState(false);

  const reserveEntry   = reserves.find(entry => entry.stock.ticker === stock?.ticker);
  const tokenSupply    = rawTokenToNumber(reserveEntry?.on_chain_supply) ?? stock?.supply ?? 0;
  const displayPrice  = idxPrice?.price ?? stock?.price ?? 0;
  const displayChange = idxPrice?.change_24h ?? stock?.change24h ?? 0;
  const pricedStock = stock ? { ...stock, price: displayPrice, change24h: displayChange } : null;

  const fullSeries = stock ? buildStockTimeSeries({ ...stock, price: displayPrice, change24h: displayChange }, 365) : [];
  const minuteSeries = buildMinuteTimeSeries(idxPrice?.sparkline_1d ?? []);
  const chartData = selectedTimeframe === '1D' && minuteSeries.length > 0
    ? minuteSeries
    : sliceRange(fullSeries, selectedTimeframe);

  if (!stock) {
    return (
      <Layout>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 1, padding: '80px 24px' }}>
          <div style={{ textAlign: 'center' }}>
            <div className="display" style={{ fontSize: 32, marginBottom: 12 }}>Not found</div>
            <div style={{ color: 'var(--body)', marginBottom: 24 }}>No pStock with ticker &quot;{ticker}&quot;</div>
            <button className="btn btn-primary" onClick={() => router.push('/stocks')}>Back to Markets</button>
          </div>
        </div>
      </Layout>
    );
  }

  const isPositive  = displayChange >= 0;
  const newsItems   = STOCK_NEWS[stock.ticker] ?? [];

  return (
    <Layout>
      <div className="container pad-x" style={{ paddingTop: 28, paddingBottom: 64 }}>

        {/* Breadcrumb */}
        <nav style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 24 }}>
          <button
            onClick={() => router.push('/stocks')}
            style={{ appearance: 'none', border: 0, background: 'transparent', padding: 0, cursor: 'pointer', fontSize: 13, color: 'var(--body)', fontFamily: 'inherit' }}
          >
            Markets
          </button>
          <span style={{ color: 'var(--hairline-strong)', fontSize: 13 }}>/</span>
          <span className="mono" style={{ fontSize: 13, fontWeight: 700, color: 'var(--ink)' }}>{stock.ticker}</span>
        </nav>

        {/* Stock header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 28, gap: 16, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
            <PStockMark ticker={stock.ticker} size={52} />
            <div>
              <div className="eyebrow" style={{ color: 'var(--body)', marginBottom: 4 }}>
                {stock.sector} · IDX: {stock.ipo} · Tokenized on Arbitrum
              </div>
              <div className="display" style={{ fontSize: 28, lineHeight: 1.15 }}>{stock.name}</div>
              <div style={{ marginTop: 10, display: 'flex', alignItems: 'baseline', gap: 16, flexWrap: 'wrap' }}>
                <span className="mono" style={{ fontSize: 28, fontWeight: 700, letterSpacing: '-0.02em' }}>
                  {fmtIDRX(displayPrice)}
                </span>
                <span className="mono" style={{ fontSize: 15, color: isPositive ? 'var(--positive)' : 'var(--negative)' }}>
                  {fmtPct(displayChange)} 24h
                </span>
              </div>
            </div>
          </div>
          <button
            className="btn btn-merah"
            onClick={() => setTradeOpen(true)}
            style={{ padding: '13px 28px', fontSize: 14, flexShrink: 0 }}
          >
            Trade {stock.ticker}
          </button>
        </div>

        {/* Chart */}
        <div style={{ marginBottom: 20 }}>
          <div className="stock-chart-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
            <div className="range-pills">
              {STOCK_TIMEFRAME_OPTIONS.map(timeframe => (
                <button
                  key={timeframe}
                  className={selectedTimeframe === timeframe ? 'active' : ''}
                  onClick={() => setSelectedTimeframe(timeframe)}
                >
                  {timeframe}
                </button>
              ))}
            </div>
            <span className="mono stock-supply-info" style={{ fontSize: 11, color: 'var(--body)' }}>
              Total supply · {fmtNum(tokenSupply, 0)} {stock.ticker}
            </span>
          </div>
          <div style={{ border: '1px solid var(--hairline)', background: 'var(--putih)', padding: '12px 0 4px' }}>
            <AreaChart
              data={chartData}
              height={340}
              valueFormatter={value => fmtIDRX(value)}
            />
          </div>
        </div>

        {/* Stats row */}
        <div className="stock-stats-grid">
          {[
            { label: 'IDX Ticker', value: stock.ipo },
            { label: 'Sector',     value: stock.sector },
            { label: 'Total Supply', value: fmtNum(tokenSupply, 0) },
            { label: 'IDX Mkt Cap', value: fmtIDRX(displayPrice * tokenSupply * STOCK_LOT_SIZE) },
            { label: 'Pool Price', value: poolPrice?.price ? fmtIDRX(poolPrice.price) : '—' },
          ].map(statItem => (
            <div key={statItem.label} className="stock-stats-cell">
              <div className="eyebrow" style={{ fontSize: 9, marginBottom: 5, color: 'var(--body)' }}>{statItem.label}</div>
              <div className="mono" style={{ fontSize: 14, fontWeight: 700 }}>{statItem.value}</div>
            </div>
          ))}
        </div>

        {/* News */}
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 14 }}>
            <span className="eyebrow">Market Intelligence</span>
            <span className="mono" style={{ fontSize: 10, color: 'var(--body)' }}>IDX · Realtime Feed</span>
          </div>
          <div style={{ border: '1px solid var(--hairline)', background: 'var(--putih)' }}>
            {newsItems.map((newsItem, newsIndex) => (
              <div
                key={newsIndex}
                style={{ padding: '18px 20px', borderBottom: newsIndex < newsItems.length - 1 ? '1px solid var(--hairline)' : 'none' }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: 14, lineHeight: 1.45, marginBottom: 8 }}>{newsItem.headline}</div>
                    <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                      <span className="eyebrow" style={{ fontSize: 9, color: 'var(--body)' }}>{newsItem.source}</span>
                      <span style={{ fontSize: 11, color: 'var(--body)' }}>{newsItem.time}</span>
                    </div>
                  </div>
                  <span
                    className="eyebrow"
                    style={{ fontSize: 9, padding: '3px 8px', border: '1px solid var(--hairline)', color: 'var(--body)', flexShrink: 0, marginTop: 2 }}
                  >
                    {newsItem.tag}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

      </div>

      {tradeOpen && pricedStock && (
        <SwapModal
          defaultOut={pricedStock}
          onClose={() => setTradeOpen(false)}
        />
      )}
    </Layout>
  );
}
