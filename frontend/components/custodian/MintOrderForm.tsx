'use client';

import { useState, useEffect, useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import { DetailRow } from '@/components/swap/SwapView';
import { useStockPrice } from '@/http/market/hooks';
import { useCustodianStocks } from '@/http/custodian/hooks';
import { useTerminalLog, useMintPipeline } from '@/http/custodian/pipelineHooks';
import { currentTimestamp } from '@/lib/terminal';

const LOT_SIZE = 100;

function Field({ label, children }: { label: string; children: React.ReactNode }): React.ReactNode {
  return (
    <div>
      <div className="mb-[8px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">{label}</div>
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
  return <span className={`mt-[6px] inline-block h-[13px] w-[7px] ${isVisible ? "bg-[var(--ink)]" : "bg-transparent"}`} />;
}

export function MintOrderForm(): React.ReactNode {
  const { data: custodianStocks = [], isLoading: isStocksLoading } = useCustodianStocks();
  const stockOptions = useMemo(
    () => custodianStocks.filter(stock => stock.idx_ticker && stock.ticker),
    [custodianStocks],
  );

  const [selectedIpoTicker, setSelectedIpoTicker] = useState("");
  const [quantity, setQuantity] = useState("50000");
  const { log, appendLog } = useTerminalLog();
  const { run: runMint, running, isPending: isMintPending } = useMintPipeline(appendLog);

  const activeIpoTicker = selectedIpoTicker || stockOptions[0]?.idx_ticker || "";
  const selectedStock = stockOptions.find(stock => stock.idx_ticker === activeIpoTicker);
  const { data: priceData, isLoading: isPriceLoading } = useStockPrice(activeIpoTicker, 'idx');
  const idrPrice = priceData?.price;
  const idrTotal = (idrPrice ?? 0) * (parseInt(quantity) || 0) * LOT_SIZE;

  async function handleRunPipeline(): Promise<void> {
    if (!selectedStock || !parseInt(quantity) || !idrPrice) return;
    await runMint({
      ticker: selectedStock.ticker,
      stockName: selectedStock.stock_name,
      idxTicker: activeIpoTicker,
      quantity,
      idrTotal,
    });
  }

  return (
    <div className="mt-[44px] grid grid-cols-[repeat(auto-fit,minmax(min(100%,400px),1fr))] items-stretch gap-[28px]">
      <div className="paper-stack flex flex-col">
        <div className="flex flex-wrap items-baseline justify-between gap-[12px] border-b border-[var(--hairline)] px-[20px] py-[16px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">
          <span>01 · New tokenization order</span>
          <span>Mint / burn on fill</span>
        </div>
        <div className="flex flex-col gap-[18px] p-[20px]">
          <Field label="IDX Ticker">
            {isStocksLoading ? (
              <div className="h-[46px] w-full bg-[var(--canvas-soft)]" />
            ) : (
              <select
                className="input mono !bg-[var(--canvas)] !p-[12px] !text-[14px] !font-[500]"
                value={activeIpoTicker}
                onChange={event => setSelectedIpoTicker(event.target.value)}
                disabled={stockOptions.length === 0}
              >
                {stockOptions.map(stock => (
                  <option key={stock.idx_ticker} value={stock.idx_ticker}>
                    {stock.idx_ticker} · {stock.stock_name.replace("Pulsar ", "")}
                  </option>
                ))}
              </select>
            )}
          </Field>
          <Field label="Order quantity (lots to buy on IDX)">
            <input
              className="input mono !bg-[var(--canvas)] !p-[12px] !text-[14px] !font-[500]"
              value={quantity}
              onChange={event => setQuantity(event.target.value.replace(/[^0-9]/g, ""))}
            />
          </Field>
        </div>

        <div className="border-y border-[var(--hairline)] bg-[var(--canvas-soft)] px-[20px] py-[16px]">
          <div className="mb-[10px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">02 · Order preview</div>
          {isPriceLoading || !idrPrice ? (
            <div className="flex flex-col gap-[8px]">
              <div className="h-[20px] w-full bg-[var(--hairline)]" />
              <div className="h-[20px] w-[80%] bg-[var(--hairline)]" />
            </div>
          ) : (
            <div className="flex flex-col gap-[8px]">
              <DetailRow k="IDR notional" v={`Rp ${idrTotal.toLocaleString("id-ID")}`} />
              <DetailRow
                k="Mint output"
                v={`${parseInt(quantity || "0").toLocaleString()} ${selectedStock?.ticker ?? activeIpoTicker}`}
                hint={`1 token = ${LOT_SIZE} shares`}
              />
            </div>
          )}
        </div>

        <div className="mt-auto p-[20px]">
          <button
            onClick={handleRunPipeline}
            disabled={running || isMintPending || !quantity || !selectedStock || !idrPrice}
            className={`btn btn-merah !inline-flex !w-full !items-center !justify-center !gap-[10px] !p-[16px] !text-[15px] ${running || isMintPending ? "is-busy" : ""}`}
          >
            {running ? <Icon name="loader" size={14} /> : <Icon name="play" size={14} />}
            {isMintPending ? "Sign in wallet…" : running ? "Executing pipeline…" : "Execute mint pipeline"}
          </button>
          <div className="mt-[10px] text-center text-[11px] tracking-[0.04em] text-[var(--body)]">
            Operator role required · Multisig 3/5
          </div>
        </div>
      </div>

      <div className="relative min-h-[480px]">
        <div className="absolute bottom-[-10px] left-[10px] right-[-6px] top-[14px] rotate-[0.8deg] border border-[var(--hairline)] bg-[#f3eedf]" />
        <div className="relative flex h-full min-h-[480px] border border-[var(--hairline)] bg-[#fdfbf4] shadow-[0_22px_30px_-16px_rgba(22,17,14,0.25)]">
          <div className="tractor-margin w-[22px] flex-none border-r border-dashed border-[#d9d1c4]" />
          <div className="flex min-w-0 flex-1 flex-col">
            <div className="flex items-center justify-between gap-[10px] border-b border-[var(--ink)] px-[16px] py-[12px]">
              <span className="mono text-[12px] font-[600] uppercase leading-[normal] tracking-[0.08em]">horizon-bridge // ops.go</span>
              <span className="mono flex items-center gap-[6px] text-[11px] leading-[normal] text-[var(--positive)]">
                <span className="h-[6px] w-[6px] rounded-full bg-[var(--positive)]" />
                streaming
              </span>
            </div>
            <div className="tractor-lines mono max-h-[520px] flex-1 overflow-y-auto px-[16px] text-[12.5px] leading-[26px] text-[var(--ink-soft)]">
              {log.map((logLine, logIndex) => (
                <div key={logIndex} className="flex gap-[14px]">
                  <span className="shrink-0 text-[#9a9286]">{logLine.timestamp}</span>
                  <span className={`w-[34px] shrink-0 ${logLine.level === "OK" ? "font-[600] text-[var(--positive)]" : logLine.level === "ERR" ? "font-[600] text-[var(--merah)]" : "text-[var(--body)]"}`}>{logLine.level}</span>
                  <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{logLine.text}</span>
                </div>
              ))}
              {running && (
                <div className="flex gap-[14px]">
                  <span className="text-[#9a9286]">{currentTimestamp()}</span>
                  <span className="w-[34px] text-[var(--body)]">...</span>
                  <span><Cursor /></span>
                </div>
              )}
            </div>
          </div>
          <div className="tractor-margin w-[22px] flex-none border-l border-dashed border-[#d9d1c4]" />
        </div>
      </div>
    </div>
  );
}
