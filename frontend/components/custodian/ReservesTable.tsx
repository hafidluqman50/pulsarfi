'use client';

import type { ReserveEntry } from '@/http/custodian/custodianApi';
import { PStockMark } from '@/components/ui/PStockMark';
import { formatRawToken, relativeAge } from './utils';

function EmptyReserveRow(): React.ReactNode {
  return (
    <div className="hairline py-[18px] text-[13px] text-[var(--body)]">
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
    <div className="reserves-table">
      <div className="hairline table-head-desktop grid grid-cols-[1.6fr_1.2fr_1.2fr_1fr_1fr_1fr] gap-[16px] py-[14px]">
        {["Asset", "Custodian holdings", "On-chain supply", "Peg ratio", "Last mint", "Status"].map((heading, columnIndex) => (
          <div key={columnIndex} className={`eyebrow !text-[var(--body)] ${columnIndex >= 1 && columnIndex <= 5 ? "text-right" : "text-left"}`}>{heading}</div>
        ))}
      </div>
      {isLoading && Array.from({ length: 3 }, (_, skeletonIndex) => (
        <div key={skeletonIndex} className="hairline table-row-stack grid grid-cols-[1.6fr_1.2fr_1.2fr_1fr_1fr_1fr] items-center gap-[16px] py-[16px]">
          <div className="flex items-center gap-[12px]">
            <div className="skeleton h-[32px] w-[32px] shrink-0" />
            <div>
              <div className="skeleton mb-[4px] h-[14px] w-[60px]" />
              <div className="skeleton h-[11px] w-[90px]" />
            </div>
          </div>
          {["w-[100px]", "w-[100px]", "w-[60px]", "w-[80px]", "w-[60px]"].map((cellWidthClass, cellIndex) => (
            <div key={cellIndex} className="text-right">
              <div className={`skeleton ml-auto h-[13px] ${cellWidthClass}`} />
            </div>
          ))}
        </div>
      ))}
      {!isLoading && entries.length === 0 && <EmptyReserveRow />}
      {entries.map(entry => {
        const stock = entry.stock;
        const custodyBalance = formatRawToken(entry.custodian_holdings);
        const onChainSupply = formatRawToken(entry.on_chain_supply);
        const isPegged = entry.peg_status === "pegged";
        return (
          <div key={stock.ticker} className="hairline table-row-stack grid grid-cols-[1.6fr_1.2fr_1.2fr_1fr_1fr_1fr] items-center gap-[16px] py-[16px]">
            <div className="col-asset flex items-center gap-[12px]">
              <PStockMark ticker={stock.ticker} size={32} />
              <div>
                <div className="text-[14px] font-semibold">{stock.ticker}</div>
                <div className="text-[11px] text-[var(--body)]">{stock.idx_ticker} · {stock.sector ?? "—"}</div>
              </div>
            </div>
            <div className="row-cell text-right"><span className="row-cell-label">Custodian holdings</span><div className="row-cell-value"><span className="mono text-[13px]">{custodyBalance}</span></div></div>
            <div className="row-cell text-right"><span className="row-cell-label">On-chain supply</span><div className="row-cell-value"><span className="mono text-[13px]">{onChainSupply}</span></div></div>
            <div className="row-cell text-right"><span className="row-cell-label">Peg ratio</span><div className="row-cell-value"><span className={`mono text-[13px] ${isPegged ? "text-[var(--positive)]" : "text-[var(--negative)]"}`}>{entry.peg_ratio}×</span></div></div>
            <div className="row-cell text-right"><span className="row-cell-label">Last mint</span><div className="row-cell-value"><span className="mono text-[12px] text-[var(--body)]">{relativeAge(entry.last_attested_at)}</span></div></div>
            <div className="row-cell text-right"><span className="row-cell-label">Status</span><div className="row-cell-value">
              <span className={`inline-flex items-center gap-[6px] text-[11px] font-bold uppercase tracking-[0.08em] ${isPegged ? "text-[var(--positive)]" : "text-[var(--negative)]"}`}>
                <span className={`h-[6px] w-[6px] rounded-[999px] ${isPegged ? "bg-[var(--positive)]" : "bg-[var(--negative)]"}`} />
                {isPegged ? "Pegged" : "Off-peg"}
              </span>
            </div></div>
          </div>
        );
      })}
    </div>
  );
}
