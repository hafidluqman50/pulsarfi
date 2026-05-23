'use client';

import { useState } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { toast } from 'sonner';
import { Token, ALL_TOKENS, PSTOCKS, STABLES, tokenByTicker, fmtNum, fmtIDRX, Balances } from '@/lib/data';
import { Icon } from './Icon';
import { PStockMark } from './PStockMark';
import { Accordion } from './Accordion';
import { TokenSelectModal } from './TokenSelectModal';

interface SwapModalProps {
  defaultOut: Token;
  balances: Balances;
  setBalances: React.Dispatch<React.SetStateAction<Balances>>;
  onClose: () => void;
}

export function SwapModal({ defaultOut, balances, setBalances, onClose }: SwapModalProps) {
  const { isConnected } = useAccount();

  const [inTok, setInTok]         = useState<Token>(tokenByTicker("IDRX"));
  const [outTok, setOutTok]       = useState<Token>(defaultOut);
  const [amount, setAmount]       = useState("");
  const [pickerFor, setPickerFor] = useState<"in" | "out" | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [slippage]                = useState(0.5);
  const [approved, setApproved]   = useState(false);
  const [busy, setBusy]           = useState(false);

  const num       = parseFloat(amount) || 0;
  const rate      = inTok.price / outTok.price;
  const outAmt    = num * rate * (1 - 0.003);
  const minReceived = outAmt * (1 - slippage / 100);
  const inBal     = balances[inTok.ticker] ?? 0;
  const insufficient = isConnected && num > inBal;

  let ctaText: string, ctaAction: (() => void) | undefined, ctaDisabled = false;
  if (!isConnected) {
    ctaText = "Connect Wallet";
  } else if (!num) {
    ctaText = "Enter an amount"; ctaDisabled = true;
  } else if (insufficient) {
    ctaText = `Insufficient ${inTok.ticker}`; ctaDisabled = true;
  } else if (!approved && inTok.isStable) {
    ctaText = busy ? "Approving…" : `Approve ${inTok.ticker}`;
    ctaAction = handleApprove;
  } else {
    ctaText = busy ? "Swapping…" : "Execute Swap";
    ctaAction = handleSwap;
  }
  if (busy) ctaDisabled = true;

  function handleApprove() {
    setBusy(true);
    const id = toast.loading(`Approving ${inTok.ticker}…`, { description: "Awaiting wallet signature" });
    setTimeout(() => {
      toast.success("Approval confirmed", { id, description: `${inTok.ticker} spending enabled`, duration: 4000 });
      setApproved(true); setBusy(false);
    }, 1400);
  }

  function handleSwap() {
    setBusy(true);
    const id = toast.loading("Transaction pending…", { description: `${fmtNum(num)} ${inTok.ticker} → ${fmtNum(outAmt, 2)} ${outTok.ticker}` });
    setTimeout(() => {
      if (Math.random() < 0.92) {
        setBalances(prev => ({
          ...prev,
          [inTok.ticker]:  (prev[inTok.ticker]  || 0) - num,
          [outTok.ticker]: (prev[outTok.ticker] || 0) + outAmt,
        }));
        toast.success("Swap executed", { id, description: `Received ${fmtNum(outAmt, 2)} ${outTok.ticker} · View on Arbiscan`, duration: 5000 });
        setAmount(""); onClose();
      } else {
        toast.error("Swap execution failed", { id, description: "Insufficient liquidity. Try a smaller amount.", duration: 6000 });
      }
      setBusy(false);
    }, 1800);
  }

  function flip() { setInTok(outTok); setOutTok(inTok); setAmount(""); setApproved(false); }

  return (
    <>
      <div className="overlay" onClick={onClose} style={{ position: "fixed", inset: 0, background: "rgba(22,17,14,0.45)", zIndex: 300, display: "flex", alignItems: "center", justifyContent: "center", padding: 16 }}>
        <div className="modal" onClick={e => e.stopPropagation()} style={{ background: "var(--putih)", width: 440, maxWidth: "100%", border: "1px solid var(--ink)", boxShadow: "12px 12px 0 0 rgba(22,17,14,0.10)" }}>
          {/* Header */}
          <div className="hairline" style={{ padding: "16px 20px", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div className="display" style={{ fontSize: 22, fontWeight: 500 }}>Trade {outTok.ticker}</div>
            <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
              <div className="eyebrow" style={{ color: "var(--body)" }}>v2 ROUTER</div>
              <button className="btn btn-ghost" onClick={onClose} style={{ padding: 4 }}><Icon name="x" size={16} /></button>
            </div>
          </div>

          {/* PAY */}
          <ModalField label="You pay" token={inTok} balance={inBal} amount={amount} onAmount={setAmount} onSelect={() => setPickerFor("in")} />

          {/* FLIP */}
          <div style={{ position: "relative", height: 0 }}>
            <button onClick={flip} aria-label="Flip" style={{ position: "absolute", left: "50%", top: -18, transform: "translateX(-50%)", width: 36, height: 36, background: "var(--canvas)", border: "1px solid var(--ink)", cursor: "pointer", color: "var(--ink)", display: "flex", alignItems: "center", justifyContent: "center", borderRadius: 0 }}>
              <Icon name="swap" size={14} />
            </button>
          </div>

          {/* RECEIVE */}
          <ModalField label="You receive" token={outTok} balance={balances[outTok.ticker] ?? 0}
            amount={outAmt ? outAmt.toFixed(outTok.isStable ? 2 : 4) : ""} readOnly onSelect={() => setPickerFor("out")}
            hint={num ? `≈ ${fmtIDRX(num * inTok.price)}` : undefined}
          />

          {/* DETAILS */}
          <div style={{ padding: "4px 20px 4px" }}>
            <Accordion open={detailsOpen} onToggle={() => setDetailsOpen(o => !o)}
              summary={<span>1 {inTok.ticker} = <span className="mono">{rate.toFixed(4)}</span> {outTok.ticker}</span>}>
              <DetailRow k="Min received"  v={`${fmtNum(minReceived, 4)} ${outTok.ticker}`} hint={`${slippage}% slippage`} />
              <DetailRow k="LP fee"        v={`${fmtNum(num * 0.003, 4)} ${inTok.ticker}`} hint="0.30%" />
              <DetailRow k="Network fee"   v="~$0.21" hint="Arbitrum Sepolia" />
            </Accordion>
          </div>

          {/* CTA */}
          <div style={{ padding: "8px 20px 20px" }}>
            {!isConnected ? (
              <ConnectButton.Custom>
                {({ openConnectModal }) => (
                  <button className="btn btn-merah" onClick={openConnectModal} style={{ width: "100%", padding: "14px 20px", fontSize: 15 }}>Connect Wallet</button>
                )}
              </ConnectButton.Custom>
            ) : (
              <button onClick={ctaAction} disabled={ctaDisabled} className="btn btn-merah"
                style={{ width: "100%", padding: "14px 20px", fontSize: 15 }}>
                {busy && <span style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 8 }}><Icon name="loader" size={14} /></span>}
                {ctaText}
              </button>
            )}
          </div>
        </div>
      </div>

      <TokenSelectModal open={!!pickerFor} tokens={pickerFor === "in" ? [...STABLES] : PSTOCKS} balances={balances}
        title={pickerFor === "in" ? "Pay with" : "Receive"} excludeTicker={pickerFor === "in" ? outTok.ticker : inTok.ticker}
        onSelect={t => { if (pickerFor === "in") { setInTok(t); setApproved(false); } else setOutTok(t); setPickerFor(null); }}
        onClose={() => setPickerFor(null)}
      />
    </>
  );
}

function ModalField({ label, token, balance, amount, onAmount, onSelect, readOnly, hint }: {
  label: string; token: Token; balance: number; amount: string;
  onAmount?: (v: string) => void; onSelect: () => void; readOnly?: boolean; hint?: string;
}) {
  return (
    <div className="hairline" style={{ padding: "16px 20px 14px" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", marginBottom: 10 }}>
        <span className="eyebrow" style={{ color: "var(--body)" }}>{label}</span>
        <span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>Bal {fmtNum(balance, 4)}</span>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <input inputMode="decimal" placeholder="0.00" value={amount} readOnly={readOnly}
          onChange={onAmount ? e => onAmount(e.target.value.replace(/[^0-9.]/g, "")) : undefined}
          className="mono"
          style={{ appearance: "none", border: 0, outline: 0, flex: 1, background: "transparent", fontSize: 32, color: "var(--ink)", padding: 0, fontFamily: '"Fraunces", serif', letterSpacing: "-0.02em" }}
        />
        <button onClick={onSelect} style={{ appearance: "none", display: "flex", alignItems: "center", gap: 8, padding: "6px 10px 6px 8px", border: "1px solid var(--ink)", background: "var(--canvas)", cursor: "pointer", color: "var(--ink)", font: "inherit" }}>
          <PStockMark ticker={token.ticker} size={22} />
          <span style={{ fontWeight: 600, fontSize: 13 }}>{token.ticker}</span>
          <Icon name="chevron-down" size={13} />
        </button>
      </div>
      {hint && <div className="mono" style={{ fontSize: 11, color: "var(--body)", marginTop: 6 }}>{hint}</div>}
    </div>
  );
}

function DetailRow({ k, v, hint }: { k: string; v: string; hint?: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", gap: 12 }}>
      <span style={{ color: "var(--body)", fontSize: 13 }}>{k}</span>
      <span className="mono" style={{ fontSize: 13 }}>
        {v}{hint && <span style={{ marginLeft: 6, color: "var(--body)", fontFamily: "Inter", fontSize: 11 }}>· {hint}</span>}
      </span>
    </div>
  );
}
