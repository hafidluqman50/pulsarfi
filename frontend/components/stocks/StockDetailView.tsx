'use client';

import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { SwapModal } from '@/components/ui/SwapModal';
import { useReserves } from '@/http/custodian/hooks';
import { useMarketStocks, useStockHistory, useStockPrice } from '@/http/market/hooks';
import { STOCK_NEWS, rawTokenToNumber } from '@/lib/stockDetail';
import { toMarketToken } from '@/lib/swap';
import { StockDetailChart } from './StockDetailChart';
import { StockDetailHeader } from './StockDetailHeader';
import { StockDetailSkeleton } from './StockDetailSkeleton';
import { StockDetailStats } from './StockDetailStats';
import { StockNewsList } from './StockNewsList';

export function StockDetailView(): React.ReactNode {
  const params = useParams();
  const router = useRouter();
  const ticker = typeof params.ticker === 'string' ? params.ticker : '';

  const { data: marketStocks = [], isLoading: isMarketLoading } = useMarketStocks();
  const stock = marketStocks.find(stockItem => stockItem.ticker === ticker);
  const { data: idxPrice } = useStockPrice(stock?.idx_ticker ?? '', 'idx');
  const { data: poolPrice } = useStockPrice(stock?.idx_ticker ?? '');
  const { data: reserves = [], isLoading: isReservesLoading } = useReserves();

  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const [tradeOpen, setTradeOpen] = useState(false);
  const { data: chartData = [], isLoading: isHistoryLoading } = useStockHistory(stock?.idx_ticker ?? '', selectedTimeframe, 'idx');

  if (isMarketLoading) {
    return <StockDetailSkeleton />;
  }

  if (!stock) {
    return (
      <div className="flex flex-1 items-center justify-center px-[24px] py-[80px]">
        <div className="text-center">
          <div className="display mb-[12px] !text-[32px]">Not found</div>
          <div className="mb-[24px] text-[var(--body)]">No pStock with ticker &quot;{ticker}&quot;</div>
          <button className="btn btn-primary" onClick={() => router.push('/stocks')}>Back to Markets</button>
        </div>
      </div>
    );
  }

  const reserveEntry = reserves.find(entry => entry.stock.ticker === stock.ticker);
  const tokenSupply = rawTokenToNumber(reserveEntry?.on_chain_supply);
  const displayPrice = idxPrice?.price ?? stock.price ?? 0;
  const displayChange = idxPrice?.change_24h ?? stock.change_24h ?? 0;
  const pricedStock = { ...toMarketToken(stock), price: displayPrice, change24h: displayChange };
  const newsItems = STOCK_NEWS[stock.ticker] ?? [];

  return (
    <div className="container pad-x !pb-[64px] !pt-[28px]">
      <StockDetailHeader
        stock={stock}
        displayPrice={displayPrice}
        displayChange={displayChange}
        onTrade={() => setTradeOpen(true)}
      />

      <StockDetailChart
        chartData={chartData}
        isHistoryLoading={isHistoryLoading}
        isReservesLoading={isReservesLoading}
        selectedTimeframe={selectedTimeframe}
        stockTicker={stock.ticker}
        tokenSupply={tokenSupply}
        onTimeframeChange={setSelectedTimeframe}
      />

      <StockDetailStats
        stock={stock}
        displayPrice={displayPrice}
        poolPrice={poolPrice?.price}
        tokenSupply={tokenSupply}
      />

      <StockNewsList newsItems={newsItems} />

      {tradeOpen && (
        <SwapModal
          defaultOut={pricedStock}
          onClose={() => setTradeOpen(false)}
        />
      )}
    </div>
  );
}
