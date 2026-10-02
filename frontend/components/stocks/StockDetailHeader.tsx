'use client';

import { useRouter } from 'next/navigation';
import { PStockMark } from '@/components/ui/PStockMark';
import { SplitFlap } from '@/components/ui/SplitFlap';
import type { MarketStock } from '@/http/market/priceApi';
import { fmtPct } from '@/lib/data';
import { useViewportWidth } from '@/lib/useViewportWidth';

const MOBILE_BREAKPOINT = 720;

type StockDetailHeaderProps = {
  stock: MarketStock;
  displayPrice: number;
  displayChange: number;
};

export function StockDetailHeader({
  stock,
  displayPrice,
  displayChange,
}: StockDetailHeaderProps): React.ReactNode {
  const router = useRouter();
  const isMobile = useViewportWidth() < MOBILE_BREAKPOINT;
  const isPositive = displayChange >= 0;
  const priceText = displayPrice.toLocaleString('id-ID', { maximumFractionDigits: 0 });

  return (
    <>
      <button
        onClick={() => router.push('/stocks')}
        className="cursor-pointer appearance-none border-0 bg-transparent p-[0] text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]"
      >
        ← Markets
      </button>

      <div className="mt-[18px] flex flex-wrap items-end justify-between gap-[24px] border-b border-[var(--ink)] pb-[20px]">
        <div className="flex items-center gap-[18px]">
          <div className="grid h-[72px] w-[72px] place-items-center border border-[var(--hairline)] bg-[var(--putih)] shadow-[0_4px_0_-2px_#f3f0ea,0_5px_0_-2px_#e3ddd2,0_18px_22px_-12px_rgba(22,17,14,0.3)]">
            <PStockMark ticker={stock.ticker} size={52} />
          </div>
          <div>
            <div className="flex items-center gap-[10px]">
              <span className="display !text-[40px] !leading-none !tracking-[-0.02em]">{stock.ticker}</span>
              <span className="border border-[var(--hairline)] px-[8px] py-[2px] text-[12px] leading-[normal] text-[var(--body)]">
                {stock.sector ?? 'Sector pending'}
              </span>
            </div>
            <div className="mt-[4px] text-[17px] font-[300] leading-[normal] text-[var(--ink-soft)] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
              {stock.stock_name} · <em className="display-it">IDX: {stock.idx_ticker}</em>
            </div>
          </div>
        </div>
        <div className="text-right">
          <div>
            <SplitFlap text={priceText} size={isMobile ? 26 : 40} />
          </div>
          <div className={`mono mt-[8px] text-[15px] leading-[normal] ${isPositive ? 'text-[var(--positive)]' : 'text-[var(--negative)]'}`}>
            {fmtPct(displayChange)} 24h · IDRX
          </div>
        </div>
      </div>
    </>
  );
}
