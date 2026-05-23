'use client';

import { useState, useCallback } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { toast } from 'sonner';
import { PSTOCKS, ALL_TOKENS, tokenByTicker, DEFAULT_PORTFOLIO, fmtNum, fmtIDRX, fmtPct, seriesFor, shortAddr, Balances, Token } from '@/lib/data';
import { Icon } from '@/components/ui/Icon';
import { PStockMark } from '@/components/ui/PStockMark';
import { Sparkline } from '@/components/ui/Sparkline';
import { Accordion } from '@/components/ui/Accordion';
import { TokenSelectModal } from '@/components/ui/TokenSelectModal';

interface HeadlineConfig {
  eyebrow: string;
  line1: string;
  line2: string;
  line3: string;
}

interface SwapViewProps {
  balances: Balances;
  setBalances: React.Dispatch<React.SetStateAction<Balances>>;
  buyAdjustCostBasis: (ticker: string, qtyAdded: number, pricePaid: number) => void;
  headline: HeadlineConfig;
}

export function SwapView({ balances, setBalances, buyAdjustCostBasis, headline }: SwapViewProps) {
  const { address, isConnected } = useAccount();

  const [inTok, setInTok]         = useState<Token>(tokenByTicker("IDRX"));
  const [outTok, setOutTok]       = useState<Token>(tokenByTicker("BUMIP"));
  const [amount, setAmount]       = useState("");
  const [pickerFor, setPickerFor] = useState<"in" | "out" | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [slippage, setSlippage]   = useState(0.5);
  const [approved, setApproved]   = useState(false);
  const [busy, setBusy]           = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  const num      = parseFloat(amount) || 0;
  const inPrice  = inTok.price;
  const outPrice = outTok.price;
  const rate     = inPrice / outPrice;
  const outAmt   = num * rate * (1 - 0.003);
  const minReceived = outAmt * (1 - slippage / 100);
  const inBal    = balances[inTok.ticker] ?? 0;
  const insufficient = isConnected && num > inBal;
  const networkFee   = 0.21;

  let ctaText: string, ctaAction: (() => void) | undefined, ctaDisabled = false, ctaKind = "merah";
  if (!isConnected) {
    ctaText = "Connect Wallet"; ctaAction = undefined;
  } else if (!num) {
    ctaText = "Enter an amount"; ctaDisabled = true;
  } else if (insufficient) {
    ctaText = `Insufficient ${inTok.ticker} balance`; ctaDisabled = true;
  } else if (!approved && inTok.isStable) {
    ctaText = busy ? "Approving…" : `Approve ${inTok.ticker}`;
    ctaAction = handleApprove; ctaKind = "primary";
  } else {
    ctaText = busy ? "Swapping…" : "Execute Swap";
    ctaAction = handleSwap;
  }
  if (busy) ctaDisabled = true;

  function handleApprove() {
    setBusy(true);
    const toastId = toast.loading(`Approving ${inTok.ticker}…`, { description: "Awaiting wallet signature" });
    setTimeout(() => {
      toast.success("Approval confirmed", { id: toastId, description: `${inTok.ticker} spending enabled · 0x7f3a…b821`, duration: 4000 });
      setApproved(true);
      setBusy(false);
    }, 1400);
  }

  function handleSwap() {
    setBusy(true);
    const toastId = toast.loading("Transaction pending…", { description: `Swapping ${fmtNum(num)} ${inTok.ticker} → ${fmtNum(outAmt, 2)} ${outTok.ticker}` });
    setTimeout(() => {
      const ok = Math.random() < 0.92;
      if (ok) {
        setBalances(prev => ({
          ...prev,
          [inTok.ticker]: (prev[inTok.ticker] || 0) - num,
          [outTok.ticker]: (prev[outTok.ticker] || 0) + outAmt,
        }));
        if (!outTok.isStable) buyAdjustCostBasis(outTok.ticker, outAmt, outTok.price);
        toast.success("Swap executed", { id: toastId, description: `Received ${fmtNum(outAmt, 2)} ${outTok.ticker} · View on Arbiscan`, duration: 5000 });
        setAmount("");
      } else {
        toast.error("Swap execution failed", { id: toastId, description: "Insufficient liquidity for this trade size. Try a smaller amount or adjust slippage.", duration: 6000 });
      }
      setBusy(false);
    }, 1800);
  }

  function flip() {
    setInTok(outTok); setOutTok(inTok); setAmount(""); setApproved(false);
  }

  const pctButton = (pct: number) => (
    <button key={pct} onClick={() => setAmount(((inBal * pct) || 0).toString())}
      style={{ appearance: "none", border: "1px solid var(--hairline-strong)", background: "transparent", padding: "4px 10px", fontSize: 11, fontWeight: 600, fontFamily: "Inter", cursor: "pointer", letterSpacing: "0.08em", textTransform: "uppercase" }}>
      {pct === 1 ? "Max" : `${pct * 100}%`}
    </button>
  );

  return (
    <div className="grid-2col pad-x" style={{ padding: "40px 24px" }}>
      {/* LEFT — Editorial */}
      <div>
        <div className="eyebrow" style={{ color: "var(--merah)", marginBottom: 16 }}>{headline.eyebrow}</div>
        <h1 className="display hero-display" style={{ fontSize: 80, lineHeight: 0.96, letterSpacing: "-0.03em", margin: 0, fontWeight: 400 }}>
          {headline.line1}<br />
          <span className="display-it">{headline.line2}</span><br />
          {headline.line3}
        </h1>
        <p style={{ marginTop: 28, fontSize: 18, lineHeight: 1.55, color: "var(--ink-soft)", maxWidth: 540, fontFamily: '"Fraunces", serif', fontWeight: 300 }}>
          Eight blue-chip equities from the Indonesia Stock Exchange, tokenized 1:1 on Arbitrum.
          Trade <em className="display-it">BUMIP, TLKMP, GOTOP</em> and others at any hour, settle in seconds,
          and exit gap risk on the weekend. A custodian holds the underlying; arbitrageurs maintain the peg.
        </p>

        <div className="hairline-top grid-3col" style={{ marginTop: 36, paddingTop: 24 }}>
          <Stat label="24h Volume" value="77.8M IDRX" sub="across 8 pairs" />
          <Stat label="Total Value Locked" value="458M IDRX" sub="↑ 12.3% week" />
          <Stat label="Peg Deviation" value="0.03%" sub="within tolerance" />
        </div>

        <div style={{ marginTop: 36 }}>
          <div className="eyebrow" style={{ marginBottom: 14, color: "var(--body)" }}>Top movers · 24h</div>
          <MoversList />
        </div>
      </div>

      {/* RIGHT — Swap card */}
      <div className="swap-sticky" style={{ position: "sticky", top: 24 }}>
        <div className="card swap-card-shadow" style={{ padding: 0, boxShadow: "12px 12px 0 0 rgba(22,17,14,0.08)" }}>
          <div className="hairline" style={{ padding: "16px 20px", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div className="display" style={{ fontSize: 22, fontWeight: 500 }}>Swap</div>
            <div style={{ display: "flex", gap: 14, alignItems: "center" }}>
              <div className="eyebrow" style={{ color: "var(--body)" }}>v2 ROUTER</div>
              <button className="btn btn-ghost" style={{ padding: 4 }} onClick={() => setShowSettings(s => !s)} aria-label="Settings">
                <Icon name="settings" size={16} />
              </button>
            </div>
          </div>

          {showSettings && (
            <div className="hairline" style={{ padding: "14px 20px", background: "var(--canvas)" }}>
              <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 8 }}>Slippage Tolerance</div>
              <div style={{ display: "flex", gap: 8 }}>
                {[0.1, 0.5, 1.0].map(s => (
                  <button key={s} onClick={() => setSlippage(s)}
                    style={{ appearance: "none", padding: "8px 14px", border: "1px solid var(--ink)", background: slippage === s ? "var(--ink)" : "transparent", color: slippage === s ? "var(--putih)" : "var(--ink)", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "Inter" }}>
                    {s}%
                  </button>
                ))}
                <div style={{ flex: 1 }} />
                <input type="number" step="0.1" value={slippage} onChange={e => setSlippage(parseFloat(e.target.value) || 0)}
                  className="input mono" style={{ width: 90, textAlign: "right" }} />
              </div>
            </div>
          )}

          <SwapField label="You pay" token={inTok} balance={balances[inTok.ticker]} amount={amount} onAmount={setAmount} onSelect={() => setPickerFor("in")}
            actions={isConnected ? (
              <div style={{ display: "flex", gap: 6 }}>{[0.25, 0.5, 1].map(pctButton)}</div>
            ) : undefined}
          />

          <div style={{ position: "relative", height: 0 }}>
            <button onClick={flip} aria-label="Flip" style={{
              position: "absolute", left: "50%", top: -18, transform: "translateX(-50%)",
              width: 36, height: 36, background: "var(--canvas)", border: "1px solid var(--ink)",
              cursor: "pointer", color: "var(--ink)", display: "flex", alignItems: "center", justifyContent: "center", borderRadius: 0,
            }}>
              <Icon name="swap" size={14} />
            </button>
          </div>

          <SwapField label="You receive" token={outTok} balance={balances[outTok.ticker]}
            amount={outAmt ? outAmt.toFixed(outTok.isStable ? 2 : 4) : ""} readOnly onSelect={() => setPickerFor("out")}
            actions={<div className="mono" style={{ fontSize: 11, color: "var(--body)" }}>≈ {fmtIDRX(num * inPrice)}</div>}
          />

          <div style={{ padding: "4px 20px 16px" }}>
            <Accordion open={detailsOpen} onToggle={() => setDetailsOpen(o => !o)}
              summary={
                <span>
                  1 {inTok.ticker} = <span className="mono">{rate.toFixed(4)}</span> {outTok.ticker}
                  <span style={{ color: "var(--body)", marginLeft: 8 }}>· {fmtIDRX(inPrice)}</span>
                </span>
              }>
              <DetailRow k="Expected output"  v={`${fmtNum(outAmt, 4)} ${outTok.ticker}`} />
              <DetailRow k="Minimum received" v={`${fmtNum(minReceived, 4)} ${outTok.ticker}`} hint={`slippage ${slippage}%`} />
              <DetailRow k="LP fee"           v={`${fmtNum(num * 0.003, 4)} ${inTok.ticker}`} hint="0.30%" />
              <DetailRow k="Price impact"     v={<span style={{ color: "var(--positive)" }}>{`< 0.01%`}</span>} />
              <DetailRow k="Network fee"      v="~$0.21" hint="Arbitrum Sepolia" />
              <DetailRow k="Route"            v={
                <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                  <span className="mono">{inTok.ticker}</span>
                  <Icon name="chevron-right" size={11} />
                  <span className="mono" style={{ color: "var(--merah)" }}>{outTok.ticker}</span>
                  <span style={{ color: "var(--body)", marginLeft: 6 }}>· Uniswap V2</span>
                </span>
              } />
            </Accordion>
          </div>

          <div style={{ padding: "0 20px 20px" }}>
            {!isConnected ? (
              <ConnectButton.Custom>
                {({ openConnectModal }) => (
                  <button onClick={openConnectModal} className="btn btn-merah"
                    style={{ width: "100%", padding: "16px 20px", fontSize: 15, letterSpacing: "0.03em" }}>
                    Connect Wallet
                  </button>
                )}
              </ConnectButton.Custom>
            ) : (
              <button onClick={ctaAction} disabled={ctaDisabled}
                className={`btn ${ctaKind === "merah" ? "btn-merah" : "btn-primary"}`}
                style={{ width: "100%", padding: "16px 20px", fontSize: 15, letterSpacing: "0.03em" }}>
                {busy && <span style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 8 }}><Icon name="loader" size={14} /></span>}
                {ctaText}
              </button>
            )}
            {isConnected && !insufficient && num > 0 && (
              <div style={{ marginTop: 12, fontSize: 11, color: "var(--body)", textAlign: "center", letterSpacing: "0.04em" }}>
                Settles instantly on-chain · Custody peg 1:1 verified 8 min ago
              </div>
            )}
          </div>
        </div>

        <div style={{ marginTop: 18, fontSize: 12, color: "var(--body)", lineHeight: 1.6, fontFamily: '"Fraunces", serif' }}>
          By trading, you affirm you are not a resident of restricted jurisdictions and that pStocks are synthetic exposures
          backed 1:1 by IDX-listed equities held by <em>PT Horizon Kustodian Indonesia</em>.
        </div>
      </div>

      <TokenSelectModal open={!!pickerFor} tokens={pickerFor === "in" ? ALL_TOKENS : PSTOCKS} balances={balances}
        title={pickerFor === "in" ? "Pay with" : "Receive"} excludeTicker={pickerFor === "in" ? outTok.ticker : inTok.ticker}
        onSelect={t => { if (pickerFor === "in") { setInTok(t); setApproved(false); } else setOutTok(t); setPickerFor(null); }}
        onClose={() => setPickerFor(null)}
      />
    </div>
  );
}

function SwapField({ label, token, balance, amount, onAmount, onSelect, readOnly, actions }: {
  label: string; token: Token; balance?: number; amount: string;
  onAmount?: (v: string) => void; onSelect: () => void; readOnly?: boolean; actions?: React.ReactNode;
}) {
  return (
    <div className="hairline" style={{ padding: "20px 20px 16px" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", marginBottom: 12 }}>
        <span className="eyebrow" style={{ color: "var(--body)" }}>{label}</span>
        <span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>Bal {balance != null ? fmtNum(balance, 4) : "—"}</span>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
        <input inputMode="decimal" placeholder="0.00" value={amount} readOnly={readOnly}
          onChange={onAmount ? (e) => { const v = e.target.value.replace(/[^0-9.]/g, ""); onAmount(v); } : undefined}
          className="mono swap-input-amt"
          style={{ appearance: "none", border: 0, outline: 0, flex: 1, minWidth: 0, background: "transparent", fontSize: 36, fontWeight: 400, color: "var(--ink)", padding: 0, fontFamily: '"Fraunces", serif', letterSpacing: "-0.02em", width: "100%" }}
        />
        <button onClick={onSelect} style={{ appearance: "none", display: "flex", alignItems: "center", gap: 10, padding: "8px 12px 8px 8px", border: "1px solid var(--ink)", background: "var(--canvas)", cursor: "pointer", color: "var(--ink)", font: "inherit" }}>
          <PStockMark ticker={token.ticker} size={26} />
          <span style={{ fontWeight: 600, fontSize: 14 }}>{token.ticker}</span>
          <Icon name="chevron-down" size={14} />
        </button>
      </div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 10, minHeight: 22 }}>
        <span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>{token.name}</span>
        <div>{actions}</div>
      </div>
    </div>
  );
}

export function DetailRow({ k, v, hint }: { k: string; v: React.ReactNode; hint?: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", gap: 12 }}>
      <span style={{ color: "var(--body)", fontSize: 13 }}>{k}</span>
      <span style={{ textAlign: "right", fontSize: 13, fontFamily: '"JetBrains Mono", monospace' }}>
        {v}
        {hint && <span style={{ marginLeft: 6, color: "var(--body)", fontFamily: "Inter", fontSize: 11 }}>· {hint}</span>}
      </span>
    </div>
  );
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div>
      <div className="eyebrow" style={{ color: "var(--body)" }}>{label}</div>
      <div className="display" style={{ fontSize: 32, marginTop: 6, lineHeight: 1, letterSpacing: "-0.02em" }}>{value}</div>
      <div style={{ marginTop: 6, fontSize: 12, color: "var(--body)" }}>{sub}</div>
    </div>
  );
}

function MoversList() {
  const movers = [...PSTOCKS].sort((a, b) => Math.abs(b.change24h) - Math.abs(a.change24h)).slice(0, 5);
  return (
    <div className="hairline-top">
      {movers.map(s => {
        const pos = s.change24h >= 0;
        return (
          <div key={s.ticker} className="hairline row-hover mover-row" style={{ display: "grid", gridTemplateColumns: "auto 1fr auto auto auto", alignItems: "center", gap: 16, padding: "14px 0" }}>
            <PStockMark ticker={s.ticker} size={32} />
            <div style={{ minWidth: 0 }}>
              <div style={{ fontWeight: 600, fontSize: 14 }}>{s.ticker}</div>
              <div style={{ fontSize: 12, color: "var(--body)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{s.name} · {s.sector}</div>
            </div>
            <div className="mover-spark"><Sparkline data={seriesFor(s.ticker)} positive={pos} /></div>
            <div className="mono" style={{ minWidth: 80, textAlign: "right", fontSize: 14 }}>{fmtIDRX(s.price)}</div>
            <div className="mono" style={{ minWidth: 76, textAlign: "right", fontSize: 13, color: pos ? "var(--positive)" : "var(--negative)" }}>{fmtPct(s.change24h)}</div>
          </div>
        );
      })}
    </div>
  );
}
