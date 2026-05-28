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
        <div className="flex flex-1 items-center justify-center px-[24px] py-[80px]">
          <div className="text-center">
            <div className="display mb-[12px] !text-[32px]">Not found</div>
            <div className="mb-[24px] text-[var(--body)]">No pStock with ticker &quot;{ticker}&quot;</div>
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
      <div className="container pad-x !pb-[64px] !pt-[28px]">

        {/* Breadcrumb */}
        <nav className="mb-[24px] flex items-center gap-[6px]">
          <button
            onClick={() => router.push('/stocks')}
            className="cursor-pointer appearance-none border-0 bg-transparent p-[0] text-[13px] text-[var(--body)] [font-family:inherit]"
          >
            Markets
          </button>
          <span className="text-[13px] text-[var(--hairline-strong)]">/</span>
          <span className="mono text-[13px] font-bold text-[var(--ink)]">{stock.ticker}</span>
        </nav>

        {/* Stock header */}
        <div className="mb-[28px] flex flex-wrap items-start justify-between gap-[16px]">
          <div className="flex items-start gap-[16px]">
            <PStockMark ticker={stock.ticker} size={52} />
            <div>
              <div className="eyebrow mb-[4px] !text-[var(--body)]">
                {stock.sector} · IDX: {stock.ipo} · Tokenized on Arbitrum
              </div>
              <div className="display !text-[28px] !leading-[1.15]">{stock.name}</div>
              <div className="mt-[10px] flex flex-wrap items-baseline gap-[16px]">
                <span className="mono text-[28px] font-bold tracking-[-0.02em]">
                  {fmtIDRX(displayPrice)}
                </span>
                <span className={`mono text-[15px] ${isPositive ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}>
                  {fmtPct(displayChange)} 24h
                </span>
              </div>
            </div>
          </div>
          <button
            onClick={() => setTradeOpen(true)}
            className="btn btn-merah shrink-0 !px-[28px] !py-[13px] !text-[14px]"
          >
            Trade {stock.ticker}
          </button>
        </div>

        {/* Chart */}
        <div className="mb-[20px]">
          <div className="stock-chart-header mb-[10px] flex items-center justify-between">
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
            <span className="mono stock-supply-info text-[11px] text-[var(--body)]">
              Total supply · {fmtNum(tokenSupply, 0)} {stock.ticker}
            </span>
          </div>
          <div className="border border-[var(--hairline)] bg-[var(--putih)] pb-[4px] pt-[12px]">
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
              <div className="eyebrow mb-[5px] !text-[9px] !text-[var(--body)]">{statItem.label}</div>
              <div className="mono text-[14px] font-bold">{statItem.value}</div>
            </div>
          ))}
        </div>

        {/* News */}
        <div>
          <div className="mb-[14px] flex items-baseline justify-between">
            <span className="eyebrow">Market Intelligence</span>
            <span className="mono text-[10px] text-[var(--body)]">IDX · Realtime Feed</span>
          </div>
          <div className="border border-[var(--hairline)] bg-[var(--putih)]">
            {newsItems.map((newsItem, newsIndex) => (
              <div
                key={newsIndex}
                className={`px-[20px] py-[18px] ${newsIndex < newsItems.length - 1 ? 'border-b border-[var(--hairline)]' : ''}`}
              >
                <div className="flex items-start justify-between gap-[16px]">
                  <div className="min-w-0 flex-1">
                    <div className="mb-[8px] text-[14px] font-semibold leading-[1.45]">{newsItem.headline}</div>
                    <div className="flex items-center gap-[12px]">
                      <span className="eyebrow !text-[9px] !text-[var(--body)]">{newsItem.source}</span>
                      <span className="text-[11px] text-[var(--body)]">{newsItem.time}</span>
                    </div>
                  </div>
                  <span
                    className="eyebrow mt-[2px] shrink-0 border border-[var(--hairline)] px-[8px] py-[3px] !text-[9px] !text-[var(--body)]"
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
