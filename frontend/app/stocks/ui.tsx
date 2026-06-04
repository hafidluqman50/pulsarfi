'use client';

import { useState, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import { Layout } from '@/components/layout/Layout';
import { PStockMark } from '@/components/ui/PStockMark';
import { Sparkline } from '@/components/ui/Sparkline';
import { AreaChart } from '@/components/charts/AreaChart';
import { useMarketStocks, useStockHistory, useStockPrice } from '@/http/market/hooks';
import type { MarketStock } from '@/http/market/priceApi';
import {
  fmtIDRX, fmtPct,
} from '@/lib/data';

const TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;

function StockRow({ stock, sparkline }: { stock: MarketStock; sparkline: number[] }): React.ReactNode {
  const router = useRouter();

  const price = stock.price ?? 0;
  const change24h = stock.change_24h ?? 0;
  const isPositive = change24h >= 0;
  const sector = stock.sector ?? '—';

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

function ChartSkeleton(): React.ReactNode {
  return <div className="skeleton h-[220px] w-full" />;
}

function StockRowSkeleton(): React.ReactNode {
  return (
    <div className="hairline stock-list-row">
      <div className="flex min-w-0 items-center gap-[12px]">
        <div className="skeleton h-[34px] w-[34px]" />
        <div className="min-w-0 flex-1">
          <div className="skeleton h-[16px] w-[72px]" />
          <div className="skeleton mt-[6px] h-[12px] w-[180px] max-w-full" />
        </div>
      </div>
      <div className="stock-sector"><div className="skeleton h-[22px] w-[96px]" /></div>
      <div className="ml-auto"><div className="skeleton h-[16px] w-[92px]" /></div>
      <div className="ml-auto"><div className="skeleton h-[16px] w-[58px]" /></div>
      <div className="stock-sparkline ml-auto"><div className="skeleton h-[28px] w-[72px]" /></div>
    </div>
  );
}

export function StocksListPage(): React.ReactNode {
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const { data: marketStocksData = [], isLoading } = useMarketStocks();
  const marketStocks = useMemo(
    () => Array.isArray(marketStocksData) ? marketStocksData : [],
    [marketStocksData],
  );

  const { data: ihsgData } = useStockPrice('IHSG');
  const { data: usdIdrData } = useStockPrice('USDIDR');
  const { data: ihsgHistory = [], isLoading: isIhsgHistoryLoading } = useStockHistory('IHSG', selectedTimeframe);
  const ihsgValue  = ihsgData?.price ?? ihsgHistory[ihsgHistory.length - 1]?.value;
  const ihsgChange = ihsgData?.change_24h ?? 0;
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
    <Layout>
      <div className="container pad-x !pb-[64px] !pt-[36px]">

        {/* ── IHSG Section ── */}
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
                    {fmtPct(ihsgChange)} 24h
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
    </Layout>
  );
}
