'use client';

import { useRouter } from 'next/navigation';
import { PStockMark } from '@/components/ui/PStockMark';
import { Sparkline } from '@/components/ui/Sparkline';
import type { MarketStock } from '@/http/market/priceApi';
import { fmtIDRX, fmtPct } from '@/lib/data';

export function StockRow({ stock, sparkline }: { stock: MarketStock; sparkline: number[] }): React.ReactNode {
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

export function StockRowSkeleton(): React.ReactNode {
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
