'use client';

import { useRouter } from 'next/navigation';
import { PStockMark } from '@/components/ui/PStockMark';
import type { MarketStock } from '@/http/market/priceApi';
import { fmtIDRX, fmtPct } from '@/lib/data';

type StockDetailHeaderProps = {
  stock: MarketStock;
  displayPrice: number;
  displayChange: number;
  onTrade: () => void;
};

export function StockDetailHeader({
  stock,
  displayPrice,
  displayChange,
  onTrade,
}: StockDetailHeaderProps): React.ReactNode {
  const router = useRouter();
  const isPositive = displayChange >= 0;

  return (
    <>
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

      <div className="mb-[28px] flex flex-wrap items-start justify-between gap-[16px]">
        <div className="flex items-start gap-[16px]">
          <PStockMark ticker={stock.ticker} size={52} />
          <div>
            <div className="eyebrow mb-[4px] !text-[var(--body)]">
              {stock.sector ?? 'Sector pending'} · IDX: {stock.idx_ticker} · Tokenized on Arbitrum
            </div>
            <div className="display !text-[28px] !leading-[1.15]">{stock.stock_name}</div>
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
          onClick={onTrade}
          className="btn btn-merah shrink-0 !px-[28px] !py-[13px] !text-[14px]"
        >
          Trade {stock.ticker}
        </button>
      </div>
    </>
  );
}
