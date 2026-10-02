'use client';

import { useState } from 'react';
import { Token, Balances, fmtNum, fmtIDRX } from '@/lib/data';
import { PStockMark } from './PStockMark';

interface TokenSelectModalProps {
  open: boolean;
  tokens: Token[];
  balances: Balances;
  onSelect: (t: Token) => void;
  onClose: () => void;
  title?: string;
  excludeTicker?: string;
}

export function TokenSelectModal({ open, tokens, balances, onSelect, onClose, title = "Select a token", excludeTicker }: TokenSelectModalProps) {
  const [q, setQ] = useState("");
  if (!open) return null;

  function close() {
    setQ("");
    onClose();
  }

  function select(token: Token) {
    setQ("");
    onSelect(token);
  }

  const list = tokens
    .filter(t => t.ticker !== excludeTicker)
    .filter(t => {
      if (!q) return true;
      const s = q.toLowerCase();
      return t.ticker.toLowerCase().includes(s) || t.name.toLowerCase().includes(s) || ((t as { sector?: string }).sector || "").toLowerCase().includes(s);
    });

  const groups: Record<string, Token[]> = {};
  for (const t of list) {
    const g = t.isStable ? "Stablecoins" : ((t as { sector?: string }).sector || "Other");
    (groups[g] = groups[g] || []).push(t);
  }
  const groupOrder = ["Stablecoins", "Energy", "Infrastructure", "Telecom", "Financial", "Technology", "Industrials", "Consumer", "Other"]
    .filter(g => groups[g]);

  return (
    <div className="overlay" style={{
      position: "fixed", inset: 0, background: "rgba(22,17,14,0.32)", zIndex: 400,
      display: "flex", alignItems: "center", justifyContent: "center",
    }} onClick={close}>
      <div className="rise paper-sheaf" onClick={e => e.stopPropagation()} style={{ width: 460, maxWidth: "92vw" }}>
        <div className="sheet" style={{ maxHeight: "82vh", display: "flex", flexDirection: "column" }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "16px 20px 12px" }}>
            <div className="display" style={{ fontSize: 22, fontWeight: 500 }}>{title}</div>
            <button onClick={close} aria-label="Close" style={{
              appearance: "none", border: "1px solid var(--hairline)", background: "#fff", width: 30, height: 30,
              cursor: "pointer", fontSize: 13, color: "var(--ink)",
            }}>✕</button>
          </div>
          <div className="hairline-strong" style={{ padding: "0 20px 14px" }}>
            <div style={{ position: "relative" }}>
              <div style={{ position: "absolute", top: "50%", left: 12, transform: "translateY(-50%)", color: "var(--body)", fontSize: 13 }}>⌕</div>
              <input
                autoFocus value={q} onChange={e => setQ(e.target.value)}
                placeholder="Search by name, ticker, or sector"
                className="input" style={{ background: "var(--canvas)", padding: "11px 12px 11px 34px", fontSize: 14 }}
              />
            </div>
          </div>
          <div style={{ overflowY: "auto", flex: 1 }}>
            {groupOrder.length === 0 && (
              <div style={{ margin: "14px 20px", padding: "24px 20px", background: "#fff", border: "1px dashed var(--hairline-strong)", color: "var(--body)", fontSize: 14 }}>No tokens match.</div>
            )}
            {groupOrder.map(g => (
              <div key={g}>
                <div style={{
                  padding: "9px 20px", background: "var(--canvas-soft)", borderBottom: "1px solid var(--hairline)",
                  fontSize: 10, fontWeight: 600, letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--body)",
                }}>{g}</div>
                {groups[g].map(tok => {
                  const bal = balances?.[tok.ticker] ?? 0;
                  return (
                    <div key={tok.ticker} className="transition-colors hover:bg-[#fbfaf7]" onClick={() => select(tok)} style={{
                      display: "flex", alignItems: "center", gap: 14,
                      padding: "12px 20px", cursor: "pointer", borderBottom: "1px solid var(--hairline)",
                    }}>
                      <span style={{ width: 34, height: 34, background: "var(--canvas-soft)", display: "grid", placeItems: "center", flex: "none" }}>
                        <PStockMark ticker={tok.ticker} size={26} />
                      </span>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontWeight: 600, fontSize: 14 }}>{tok.ticker}</div>
                        <div style={{ fontSize: 12, color: "var(--body)" }}>{tok.name}</div>
                      </div>
                      <div style={{ textAlign: "right" }}>
                        <div className="mono" style={{ fontSize: 13 }}>{fmtNum(bal)}</div>
                        <div className="mono" style={{ fontSize: 11, color: "var(--body)" }}>{tok.isStable ? "1 IDRX" : fmtIDRX(tok.price)}</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
