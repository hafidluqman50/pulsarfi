'use client';

import { useRouter } from 'next/navigation';
import { PStockMark } from '@/components/ui/PStockMark';
import { Sparkline } from '@/components/ui/Sparkline';
import { SplitFlap } from '@/components/ui/SplitFlap';
import type { MarketStock } from '@/http/market/priceApi';
import { fmtPct } from '@/lib/data';

export const BOARD_GRID = 'grid grid-cols-[48px_150px_minmax(0,1fr)_200px_150px_80px] gap-[16px]';

const FLAP_WIDTH = 7;

export function StockRow({ stock, sparkline }: { stock: MarketStock; sparkline: number[] }): React.ReactNode {
  const router = useRouter();

  const price = stock.price ?? 0;
  const change24h = stock.change_24h ?? 0;
  const isPositive = change24h >= 0;
  const changeColor = isPositive ? 'var(--positive)' : 'var(--negative)';
  const sector = stock.sector ?? '—';
  const priceText = price.toLocaleString('id-ID', { maximumFractionDigits: 0 }).padStart(FLAP_WIDTH, ' ');
  const changeText = fmtPct(change24h).padStart(FLAP_WIDTH, ' ');

  return (
    <div
      className={`${BOARD_GRID} cursor-pointer items-center border-t border-[#efebe3] px-[6px] py-[9px] transition-colors duration-150 hover:bg-[#f7f4ee]`}
      onClick={() => router.push(`/stocks/${stock.ticker}`)}
    >
      <span className="grid h-[34px] w-[34px] place-items-center bg-[var(--canvas-soft)]">
        <PStockMark ticker={stock.ticker} size={26} />
      </span>
      <span>
        <SplitFlap text={stock.ticker} size={18} />
      </span>
      <div className="min-w-0">
        <div className="text-[14px] font-[600] text-[var(--ink)]">
          {stock.stock_name.replace(/^Pulsar /, '')}
        </div>
        <div className="text-[12px] text-[var(--body)]">{sector}</div>
      </div>
      <span>
        <SplitFlap text={priceText} size={18} />
      </span>
      <span>
        <SplitFlap text={changeText} size={18} color={changeColor} />
      </span>
      <div className="flex justify-end">
        <Sparkline data={sparkline} positive={isPositive} width={72} height={28} strokeWidth={1.4} />
      </div>
    </div>
  );
}

export function StockRowSkeleton(): React.ReactNode {
  return (
    <div className={`${BOARD_GRID} items-center border-t border-[#efebe3] px-[6px] py-[9px]`}>
      <div className="h-[34px] w-[34px] bg-[var(--canvas-soft)]" />
      <div className="h-[23px] w-[80px] bg-[var(--canvas-soft)]" />
      <div className="min-w-0">
        <div className="h-[16px] w-[140px] max-w-full bg-[var(--canvas-soft)]" />
        <div className="mt-[6px] h-[12px] w-[90px] max-w-full bg-[var(--canvas-soft)]" />
      </div>
      <div className="h-[23px] w-[110px] bg-[var(--canvas-soft)]" />
      <div className="h-[23px] w-[90px] bg-[var(--canvas-soft)]" />
      <div className="ml-auto h-[28px] w-[72px] bg-[var(--canvas-soft)]" />
    </div>
  );
}
