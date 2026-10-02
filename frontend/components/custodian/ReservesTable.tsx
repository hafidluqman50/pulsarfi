'use client';

import type { ReserveEntry } from '@/http/custodian/custodianApi';
import { PStockMark } from '@/components/ui/PStockMark';
import { formatRawToken } from './utils';

const RESERVE_GRID = 'grid grid-cols-[1.4fr_1fr_1fr_auto] gap-[12px]';

function EmptyReserveRow(): React.ReactNode {
  return (
    <div className="border-b border-[var(--hairline)] px-[4px] py-[18px] text-[13px] text-[var(--body)]">
      No reserve attestations submitted yet
    </div>
  );
}

interface ReservesTableProps {
  entries: ReserveEntry[];
  isLoading?: boolean;
}

export function ReservesTable({ entries, isLoading }: ReservesTableProps): React.ReactNode {
  return (
    <div>
      <div className={`${RESERVE_GRID} border-b border-[var(--hairline)] px-[4px] py-[10px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]`}>
        <span>Stock</span>
        <span className="text-right">In custody</span>
        <span className="text-right">On-chain</span>
        <span className="text-right">Ratio</span>
      </div>
      {isLoading && Array.from({ length: 3 }, (_, skeletonIndex) => (
        <div key={skeletonIndex} className={`${RESERVE_GRID} items-center border-b border-[var(--hairline)] px-[4px] py-[11px]`}>
          <div className="flex items-center gap-[10px]">
            <div className="h-[22px] w-[22px] shrink-0 bg-[var(--canvas-soft)]" />
            <div className="h-[13px] w-[60px] bg-[var(--canvas-soft)]" />
          </div>
          <div className="ml-auto h-[13px] w-[80px] bg-[var(--canvas-soft)]" />
          <div className="ml-auto h-[13px] w-[80px] bg-[var(--canvas-soft)]" />
          <div className="ml-auto h-[13px] w-[56px] bg-[var(--canvas-soft)]" />
        </div>
      ))}
      {!isLoading && entries.length === 0 && <EmptyReserveRow />}
      {entries.map(entry => {
        const stock = entry.stock;
        const isPegged = entry.peg_status === "pegged";
        const statusColor = isPegged ? "var(--positive)" : "var(--negative)";
        return (
          <div
            key={stock.ticker}
            className={`${RESERVE_GRID} items-center border-b border-[var(--hairline)] px-[4px] py-[11px] ${isPegged ? "" : "bg-[#fdf3f4]"}`}
          >
            <span className="flex items-center gap-[10px]">
              <PStockMark ticker={stock.ticker} size={22} />
              <span className="text-[13px] font-[600]">{stock.ticker}</span>
            </span>
            <span className="mono text-right text-[13px] leading-[normal]">{formatRawToken(entry.custodian_holdings)}</span>
            <span className="mono text-right text-[13px] leading-[normal]">{formatRawToken(entry.on_chain_supply)}</span>
            <span className="mono inline-flex items-center justify-end gap-[6px] text-[12px] font-[600] leading-[normal]" style={{ color: statusColor }}>
              <span className="h-[6px] w-[6px] rounded-full" style={{ background: statusColor }} />
              {entry.peg_ratio}×
            </span>
          </div>
        );
      })}
    </div>
  );
}
