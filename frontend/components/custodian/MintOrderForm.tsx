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
      <div className="eyebrow mb-[8px] !text-[var(--body)]">{label}</div>
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
  return <span className={`inline-block h-[13px] w-[7px] align-[-2px] ${isVisible ? "bg-[#fff]" : "bg-transparent"}`} />;
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
    <div className="grid-2col-form mt-[32px]">
      {/* FORM */}
      <div className="card p-[0]">
        <div className="hairline px-[20px] py-[16px]">
          <div className="eyebrow !text-[var(--body)]">01 · New tokenization order</div>
        </div>
        <div className="flex flex-col gap-[18px] p-[20px]">
          <Field label="IDX Ticker">
            <select className="input mono" value={selectedIpoTicker} onChange={event => setSelectedIpoTicker(event.target.value)}>
              {PSTOCKS.map(stock => <option key={stock.ipo} value={stock.ipo}>{stock.ipo} · {stock.name.replace("Pulsar ", "")}</option>)}
            </select>
          </Field>
          <Field label="Order quantity (lots to buy on IDX)">
            <input className="input mono" value={quantity} onChange={event => setQuantity(event.target.value.replace(/[^0-9]/g, ""))} />
          </Field>
        </div>

        <div className="hairline bg-[var(--canvas-soft)] px-[20px] py-[16px]">
          <div className="eyebrow mb-[10px] !text-[var(--body)]">02 · Order preview</div>
          <div className="flex flex-col gap-[8px]">
            <DetailRow k="IDR notional" v={`Rp ${idrTotal.toLocaleString("id-ID")}`} />
            <DetailRow
              k="Mint output"
              v={`${parseInt(quantity || "0").toLocaleString()} ${selectedStock?.ticker ?? selectedIpoTicker}`}
              hint={`1 token = ${LOT_SIZE} shares`}
            />
          </div>
        </div>

        <div className="p-[20px]">
          <button
            onClick={handleRunPipeline}
            disabled={running || isMintPending || !quantity}
            className="btn btn-merah !inline-flex !w-full !items-center !justify-center !gap-[10px] !p-[16px] !text-[15px]"
          >
            {running ? <Icon name="loader" size={14} /> : <Icon name="play" size={14} />}
            {isMintPending ? "Sign in wallet…" : running ? "Executing pipeline…" : "Execute mint pipeline"}
          </button>
          <div className="mt-[10px] text-center text-[11px] tracking-[0.04em] text-[var(--body)]">
            Operator role required · Multisig 3/5
          </div>
        </div>
      </div>

      {/* CONSOLE */}
      <div className="card-ink flex min-h-[540px] flex-col overflow-hidden">
        <div className="flex items-center justify-between border-b border-[rgba(255,255,255,0.16)] px-[18px] py-[12px]">
          <div className="flex items-center gap-[12px]">
            <Icon name="terminal" size={16} />
            <span className="mono text-[12px] uppercase tracking-[0.08em]">horizon-bridge // ops.go</span>
          </div>
          <div className="mono text-[11px] text-[rgba(255,255,255,0.55)]">
            <span className="mr-[6px] inline-block h-[6px] w-[6px] rounded-[999px] bg-[#52ce7a] align-[2px]" />
            streaming
          </div>
        </div>
        <div className="mono max-h-[540px] flex-1 overflow-y-auto px-[18px] py-[16px] text-[12.5px] leading-[1.7] text-[rgba(255,255,255,0.85)]">
          {log.map((logLine, logIndex) => (
            <div key={logIndex} className="flex gap-[14px]">
              <span className="shrink-0 text-[rgba(255,255,255,0.4)]">{logLine.timestamp}</span>
              <span className={`w-[32px] shrink-0 ${logLine.level === "OK" ? "text-[#52ce7a]" : logLine.level === "ERR" ? "text-[#ff6a6a]" : "text-[rgba(255,255,255,0.55)]"}`}>{logLine.level}</span>
              <span className="flex-1 whitespace-pre-wrap break-words">{logLine.text}</span>
            </div>
          ))}
          {running && (
            <div className="flex gap-[14px]">
              <span className="text-[rgba(255,255,255,0.4)]">{currentTimestamp()}</span>
              <span className="w-[32px] text-[rgba(255,255,255,0.55)]">...</span>
              <span><Cursor /></span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
