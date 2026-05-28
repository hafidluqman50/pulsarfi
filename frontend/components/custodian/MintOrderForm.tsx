'use client';

import { useState, useEffect } from 'react';
import { Icon } from '@/components/ui/Icon';
import { DetailRow } from '@/components/swap/SwapView';
import { PSTOCKS } from '@/lib/data';
import { useStockPrice } from '@/http/market/hooks';
import { useTerminalLog, useMintPipeline } from '@/http/custodian/pipelineHooks';
import { currentTimestamp } from '@/lib/terminal';

const IDR_PRICE_FALLBACK: Record<string, number> = { BUMI: 246, ENRG: 378, KIJA: 144, TLKM: 2976, BBRI: 4780, GOTO: 98, ASII: 5190, UNVR: 2402 };
const LOT_SIZE = 100;

function Field({ label, children }: { label: string; children: React.ReactNode }): React.ReactNode {
  return (
    <div>
      <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 8 }}>{label}</div>
      {children}
    </div>
  );
}

function Cursor(): React.ReactNode {
  const [isVisible, setIsVisible] = useState(true);
  useEffect(() => {
    const intervalId = setInterval(() => setIsVisible(prev => !prev), 500);
    return () => clearInterval(intervalId);
  }, []);
  return <span style={{ display: "inline-block", width: 7, height: 13, background: isVisible ? "#fff" : "transparent", verticalAlign: -2 }} />;
}

export function MintOrderForm(): React.ReactNode {
  const [selectedIpoTicker, setSelectedIpoTicker] = useState("BUMI");
  const [quantity, setQuantity] = useState("50000");
  const { log, appendLog } = useTerminalLog();
  const { run: runMint, running, isPending: isMintPending } = useMintPipeline(appendLog);

  const selectedStock = PSTOCKS.find(stock => stock.ipo === selectedIpoTicker);
  const { data: priceData } = useStockPrice(selectedIpoTicker, 'idx');
  const idrPrice = priceData?.price ?? IDR_PRICE_FALLBACK[selectedIpoTicker] ?? 250;
  const idrTotal = idrPrice * (parseInt(quantity) || 0) * LOT_SIZE;

  async function handleRunPipeline(): Promise<void> {
    if (!selectedStock || !parseInt(quantity)) return;
    await runMint({
      ticker: selectedStock.ticker,
      stockName: selectedStock.name,
      idxTicker: selectedIpoTicker,
      quantity,
      idrPrice,
      idrTotal,
    });
  }

  return (
    <div className="grid-2col-form" style={{ marginTop: 32 }}>
      {/* FORM */}
      <div className="card" style={{ padding: 0 }}>
        <div className="hairline" style={{ padding: "16px 20px" }}>
          <div className="eyebrow" style={{ color: "var(--body)" }}>01 · New tokenization order</div>
        </div>
        <div style={{ padding: 20, display: "flex", flexDirection: "column", gap: 18 }}>
          <Field label="IDX Ticker">
            <select className="input mono" value={selectedIpoTicker} onChange={event => setSelectedIpoTicker(event.target.value)}>
              {PSTOCKS.map(stock => <option key={stock.ipo} value={stock.ipo}>{stock.ipo} · {stock.name.replace("Pulsar ", "")}</option>)}
            </select>
          </Field>
          <Field label="Order quantity (lots to buy on IDX)">
            <input className="input mono" value={quantity} onChange={event => setQuantity(event.target.value.replace(/[^0-9]/g, ""))} />
          </Field>
        </div>

        <div className="hairline" style={{ padding: "16px 20px", background: "var(--canvas-soft)" }}>
          <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 10 }}>02 · Order preview</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <DetailRow k="IDR notional" v={`Rp ${idrTotal.toLocaleString("id-ID")}`} />
            <DetailRow
              k="Mint output"
              v={`${parseInt(quantity || "0").toLocaleString()} ${selectedStock?.ticker ?? selectedIpoTicker}`}
              hint={`1 token = ${LOT_SIZE} shares`}
            />
          </div>
        </div>

        <div style={{ padding: 20 }}>
          <button
            onClick={handleRunPipeline}
            disabled={running || isMintPending || !quantity}
            className="btn btn-merah"
            style={{ width: "100%", padding: 16, fontSize: 15, display: "inline-flex", justifyContent: "center", alignItems: "center", gap: 10 }}
          >
            {running ? <Icon name="loader" size={14} /> : <Icon name="play" size={14} />}
            {isMintPending ? "Sign in wallet…" : running ? "Executing pipeline…" : "Execute mint pipeline"}
          </button>
          <div style={{ marginTop: 10, fontSize: 11, color: "var(--body)", letterSpacing: "0.04em", textAlign: "center" }}>
            Operator role required · Multisig 3/5
          </div>
        </div>
      </div>

      {/* CONSOLE */}
      <div className="card-ink" style={{ display: "flex", flexDirection: "column", overflow: "hidden", minHeight: 540 }}>
        <div style={{ padding: "12px 18px", display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid rgba(255,255,255,0.16)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <Icon name="terminal" size={16} />
            <span className="mono" style={{ fontSize: 12, letterSpacing: "0.08em", textTransform: "uppercase" }}>horizon-bridge // ops.go</span>
          </div>
          <div className="mono" style={{ fontSize: 11, color: "rgba(255,255,255,0.55)" }}>
            <span style={{ display: "inline-block", width: 6, height: 6, borderRadius: 999, background: "#52ce7a", marginRight: 6, verticalAlign: 2 }} />
            streaming
          </div>
        </div>
        <div className="mono" style={{ padding: "16px 18px", fontSize: 12.5, lineHeight: 1.7, flex: 1, overflowY: "auto", color: "rgba(255,255,255,0.85)", maxHeight: 540 }}>
          {log.map((logLine, logIndex) => (
            <div key={logIndex} style={{ display: "flex", gap: 14 }}>
              <span style={{ color: "rgba(255,255,255,0.4)", flexShrink: 0 }}>{logLine.timestamp}</span>
              <span style={{ color: logLine.level === "OK" ? "#52ce7a" : logLine.level === "ERR" ? "#ff6a6a" : "rgba(255,255,255,0.55)", width: 32, flexShrink: 0 }}>{logLine.level}</span>
              <span style={{ flex: 1, whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{logLine.text}</span>
            </div>
          ))}
          {running && (
            <div style={{ display: "flex", gap: 14 }}>
              <span style={{ color: "rgba(255,255,255,0.4)" }}>{currentTimestamp()}</span>
              <span style={{ color: "rgba(255,255,255,0.55)", width: 32 }}>...</span>
              <span><Cursor /></span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
