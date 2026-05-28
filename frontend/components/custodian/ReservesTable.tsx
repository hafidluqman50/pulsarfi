'use client';

import type { ReserveEntry } from '@/http/custodian/custodianApi';
import { PStockMark } from '@/components/ui/PStockMark';
import { formatRawToken, relativeAge } from './utils';

function EmptyReserveRow(): React.ReactNode {
  return (
    <div className="hairline" style={{ padding: "18px 0", color: "var(--body)", fontSize: 13 }}>
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
      <div className="hairline table-head-desktop" style={{ display: "grid", gridTemplateColumns: "1.6fr 1.2fr 1.2fr 1fr 1fr 1fr", gap: 16, padding: "14px 0" }}>
        {["Asset", "Custodian holdings", "On-chain supply", "Peg ratio", "Last mint", "Status"].map((heading, columnIndex) => (
          <div key={columnIndex} className="eyebrow" style={{ color: "var(--body)", textAlign: columnIndex >= 1 && columnIndex <= 5 ? "right" : "left" }}>{heading}</div>
        ))}
      </div>
      {isLoading && Array.from({ length: 3 }, (_, skeletonIndex) => (
        <div key={skeletonIndex} className="hairline table-row-stack" style={{ display: "grid", gridTemplateColumns: "1.6fr 1.2fr 1.2fr 1fr 1fr 1fr", gap: 16, padding: "16px 0", alignItems: "center" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div className="skeleton" style={{ width: 32, height: 32, flexShrink: 0 }} />
            <div>
              <div className="skeleton" style={{ height: 14, width: 60, marginBottom: 4 }} />
              <div className="skeleton" style={{ height: 11, width: 90 }} />
            </div>
          </div>
          {[100, 100, 60, 80, 60].map((cellWidth, cellIndex) => (
            <div key={cellIndex} style={{ textAlign: "right" }}>
              <div className="skeleton" style={{ height: 13, width: cellWidth, marginLeft: "auto" }} />
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
          <div key={stock.ticker} className="hairline table-row-stack" style={{ display: "grid", gridTemplateColumns: "1.6fr 1.2fr 1.2fr 1fr 1fr 1fr", gap: 16, padding: "16px 0", alignItems: "center" }}>
            <div className="col-asset" style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <PStockMark ticker={stock.ticker} size={32} />
              <div>
                <div style={{ fontWeight: 600, fontSize: 14 }}>{stock.ticker}</div>
                <div style={{ fontSize: 11, color: "var(--body)" }}>{stock.idx_ticker} · {stock.sector ?? "—"}</div>
              </div>
            </div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Custodian holdings</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{custodyBalance}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">On-chain supply</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{onChainSupply}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Peg ratio</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13, color: isPegged ? "var(--positive)" : "var(--negative)" }}>{entry.peg_ratio}×</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Last mint</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>{relativeAge(entry.last_attested_at)}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Status</span><div className="row-cell-value">
              <span style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11, fontWeight: 700, letterSpacing: "0.08em", textTransform: "uppercase", color: isPegged ? "var(--positive)" : "var(--negative)" }}>
                <span style={{ width: 6, height: 6, borderRadius: 999, background: isPegged ? "var(--positive)" : "var(--negative)" }} />
                {isPegged ? "Pegged" : "Off-peg"}
              </span>
            </div></div>
          </div>
        );
      })}
    </div>
  );
}
