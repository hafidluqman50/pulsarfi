'use client';

import { useState } from 'react';
import { Token, Balances, fmtNum, fmtIDRX } from '@/lib/data';
import { Icon } from './Icon';
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
      position: "fixed", inset: 0, background: "rgba(22,17,14,0.45)", zIndex: 400,
      display: "flex", alignItems: "center", justifyContent: "center",
    }} onClick={close}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{
        background: "var(--putih)", width: 460, maxWidth: "92vw", maxHeight: "82vh",
        display: "flex", flexDirection: "column",
        border: "1px solid var(--ink)",
        boxShadow: "8px 8px 0 0 rgba(22,17,14,0.10)",
      }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "16px 20px 12px" }}>
          <div className="display" style={{ fontSize: 22 }}>{title}</div>
          <button className="btn-ghost btn" onClick={close} aria-label="Close" style={{ padding: 4 }}><Icon name="x" /></button>
        </div>
        <div className="hairline-strong" style={{ padding: "0 20px 14px" }}>
          <div style={{ position: "relative" }}>
            <div style={{ position: "absolute", top: "50%", left: 12, transform: "translateY(-50%)", color: "var(--body)" }}>
              <Icon name="search" size={16} />
            </div>
            <input
              autoFocus value={q} onChange={e => setQ(e.target.value)}
              placeholder="Search by name, ticker, or sector"
              className="input" style={{ paddingLeft: 36 }}
            />
          </div>
        </div>
        <div style={{ overflowY: "auto", flex: 1 }}>
          {groupOrder.length === 0 && (
            <div style={{ padding: "24px 20px", color: "var(--body)", fontSize: 14 }}>No tokens match.</div>
          )}
          {groupOrder.map(g => (
            <div key={g}>
              <div className="eyebrow hairline" style={{ padding: "10px 20px", color: "var(--body)", background: "var(--canvas)" }}>{g}</div>
              {groups[g].map(tok => {
                const bal = balances?.[tok.ticker] ?? 0;
                return (
                  <div key={tok.ticker} className="row-hover" onClick={() => select(tok)} style={{
                    display: "flex", alignItems: "center", gap: 14,
                    padding: "12px 20px", cursor: "pointer", borderBottom: "1px solid var(--hairline)",
                  }}>
                    <PStockMark ticker={tok.ticker} size={32} />
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
  );
}
