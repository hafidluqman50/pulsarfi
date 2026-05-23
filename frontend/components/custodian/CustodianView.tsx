'use client';

import { useState, useEffect } from 'react';
import { useAccount } from 'wagmi';
import { toast } from 'sonner';
import { PSTOCKS, fmtUSD, fmtNum, fmtIDR, shortAddr, Balances } from '@/lib/data';
import { Icon } from '@/components/ui/Icon';
import { PStockMark } from '@/components/ui/PStockMark';
import { DetailRow } from '@/components/swap/SwapView';

interface LogLine { timestamp: string; level: string; text: string; }

const DEFAULT_LOG: LogLine[] = [
  { timestamp: "14:02:11.482", level: "INFO", text: "[boot] horizon-bridge v0.4.1 starting · operator=msig-3/5" },
  { timestamp: "14:02:11.501", level: "INFO", text: "[rpc]  connected → arbitrum-sepolia · chain=421614 · height=98,341,008" },
  { timestamp: "14:02:11.518", level: "INFO", text: "[idx]  connected → broker.mirae.co.id · session=valid" },
  { timestamp: "14:02:12.044", level: "OK",   text: "[oracle] price feed warm · 8 markets · staleness=2.1s" },
  { timestamp: "14:02:12.231", level: "OK",   text: "[attest] last proof-of-reserves verified · 8 min ago" },
  { timestamp: "14:02:12.402", level: "INFO", text: "[ready] awaiting operator command…" },
];

function padTimeComponent(timeComponent: number) { return timeComponent < 10 ? "0" + timeComponent : "" + timeComponent; }
function currentTimestamp() {
  const now = new Date();
  return `${padTimeComponent(now.getHours())}:${padTimeComponent(now.getMinutes())}:${padTimeComponent(now.getSeconds())}.${("00" + now.getMilliseconds()).slice(-3)}`;
}
function randomHexString(length: number) { return Array.from({ length }, () => Math.floor(Math.random() * 16).toString(16)).join(""); }
function randomDigitString(length: number) { return Array.from({ length }, () => Math.floor(Math.random() * 10)).join(""); }
const delay = (milliseconds: number) => new Promise(resolve => setTimeout(resolve, milliseconds));

const IDR_PRICE_MAP: Record<string, number> = { BUMI: 246, ENRG: 378, KIJA: 144, TLKM: 2976, BBRI: 4780, GOTO: 98, ASII: 5190, UNVR: 2402 };

interface CustodianViewProps {
  balances: Balances;
  setBalances: React.Dispatch<React.SetStateAction<Balances>>;
}

export function CustodianView({ balances, setBalances }: CustodianViewProps) {
  const { address, isConnected } = useAccount();
  const [selectedIpoTicker, setSelectedIpoTicker] = useState("BUMI");
  const [quantity,          setQuantity]          = useState("50000");
  const [destination,       setDestination]       = useState("liquidity");
  const [side,              setSide]              = useState<"mint" | "burn">("mint");
  const [running,           setRunning]           = useState(false);
  const [log,               setLog]               = useState<LogLine[]>(DEFAULT_LOG);

  const selectedStock = PSTOCKS.find(stockItem => stockItem.ipo === selectedIpoTicker);
  const idrPrice = IDR_PRICE_MAP[selectedIpoTicker] || 250;
  const idrTotal = idrPrice * (parseInt(quantity) || 0);
  const usdRate  = 16142;
  const usdTotal = idrTotal / usdRate;

  function appendLog(line: Omit<LogLine, "timestamp">) {
    setLog(previousLog => [...previousLog, { timestamp: currentTimestamp(), ...line }]);
  }

  async function runPipeline() {
    if (!selectedStock || !parseInt(quantity)) return;
    setRunning(true);
    const verb = side === "mint" ? "BUY" : "SELL";
    appendLog({ level: "INFO", text: `[idx-broker] order received → ${verb} ${quantity} ${selectedIpoTicker} @ market` });
    const toastId = toast.loading(
      side === "mint" ? `Sourcing ${quantity} ${selectedIpoTicker} on IDX…` : `Redeeming ${quantity} ${selectedIpoTicker} on IDX…`,
      { description: side === "mint" ? "Awaiting fill from broker desk" : "Releasing custody, settling IDR" }
    );
    await delay(800);
    appendLog({ level: "INFO", text: `[idx-broker] partial fill ${Math.floor(parseInt(quantity) * 0.6)}/${quantity} @ Rp ${idrPrice}` });
    await delay(700);
    appendLog({ level: "INFO", text: `[idx-broker] order filled · avg ${idrPrice} IDR · ref IDX-${randomDigitString(8)}` });
    appendLog({ level: "INFO", text: `[custody] ${side === "mint" ? "settlement T+2 scheduled" : "release confirmed"} · custodian: PT HORIZON KUSTODIAN` });
    await delay(800);
    toast.loading(side === "mint" ? `Calling mint() on ${selectedStock.ticker}…` : `Calling burn() on ${selectedStock.ticker}…`, { id: toastId, description: "Posting attestation to Arbitrum Sepolia" });
    appendLog({ level: "INFO", text: `[bridge] attestation hash 0x${randomHexString(8)}…${randomHexString(8)} signed (3/5 multisig)` });
    await delay(900);
    const txHash = `0x${randomHexString(8)}${randomHexString(8)}…${randomHexString(4)}`;
    appendLog({ level: "OK", text: `[evm] ${side}(${quantity} ${selectedStock.ticker}) → tx ${txHash} mined in block #98341${randomDigitString(3)}` });
    const newSupply = side === "mint" ? selectedStock.supply + parseInt(quantity) : selectedStock.supply - parseInt(quantity);
    appendLog({ level: "OK", text: `[supply] new circulating: ${newSupply.toLocaleString()} ${selectedStock.ticker}` });
    if (side === "mint" && destination === "wallet" && isConnected) {
      setBalances(previousBalances => ({ ...previousBalances, [selectedStock.ticker]: (previousBalances[selectedStock.ticker] || 0) + parseInt(quantity) }));
      appendLog({ level: "OK", text: `[evm] transferred to ${shortAddr(address!)}` });
    } else if (side === "mint") {
      appendLog({ level: "OK", text: `[lp] deposited to Uniswap V2 pool 0x${randomHexString(8)}…${randomHexString(4)}` });
    } else {
      appendLog({ level: "OK", text: `[idr-vault] +Rp ${idrTotal.toLocaleString("id-ID")} settled to escrow` });
    }
    toast.success(side === "mint" ? "Mint successful" : "Burn successful", { id: toastId, description: `${quantity} ${selectedStock.ticker} ${side === "mint" ? "minted" : "burned"} · 1:1 peg verified`, duration: 4500 });
    setRunning(false);
  }

  return (
    <div className="pad-x" style={{ padding: "32px 24px 16px" }}>
      {/* Section head */}
      <div className="hairline-strong" style={{ paddingBottom: 18 }}>
        <div className="eyebrow" style={{ color: "var(--merah)", marginBottom: 12 }}>Horizon Labs · Custodian Console · {isConnected ? shortAddr(address!) : "OPS-MASTER"}</div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", gap: 24, flexWrap: "wrap" }}>
          <div>
            <h1 className="display hero-display" style={{ fontSize: 56, margin: 0, lineHeight: 1, letterSpacing: "-0.028em" }}>
              The <span className="display-it">custodian bridge</span>.
            </h1>
            <p style={{ marginTop: 12, color: "var(--body)", maxWidth: 580, fontFamily: '"Fraunces", serif', fontSize: 17, fontWeight: 300 }}>
              Trigger institutional buy & sell orders on IDX. Each fill is attested by the custodian and triggers 1:1 mint or burn of pStock supply on Arbitrum.
            </p>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
            <Pill label="Custodian" value="ONLINE"    tone="green" />
            <Pill label="Oracle"    value="FRESH 12s" tone="green" />
            <Pill label="Bridge"    value="READY"     tone="green" />
          </div>
        </div>
      </div>

      {/* TOP METRICS */}
      <div className="grid-4col" style={{ marginTop: 24 }}>
        <Metric label="Assets Under Custody"   value="$28.4M"    sub="Rp 458.4 B equivalent"    tone="ink" />
        <Metric label="24h Mint Volume"         value="$1.24M"    sub="6 mints · 0 burns"         tone="merah" />
        <Metric label="Pending requests"        value="4"         sub="2 mint · 2 redeem" />
        <Metric label="IDR settlement vault"    value="Rp 12.8 B" sub="@ Bank Mandiri 1900" />
      </div>

      {/* MAIN: form + console */}
      <div className="grid-2col-form" style={{ marginTop: 32 }}>
        {/* FORM */}
        <div className="card" style={{ padding: 0 }}>
          <div className="hairline" style={{ padding: "16px 20px", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div className="eyebrow" style={{ color: "var(--body)" }}>01 · New tokenization order</div>
            <div style={{ display: "flex", border: "1px solid var(--ink)" }}>
              {[{ id: "mint" as const, label: "Mint" }, { id: "burn" as const, label: "Redeem" }].map((option, optionIndex) => (
                <button key={option.id} onClick={() => setSide(option.id)} style={{
                  appearance: "none", padding: "6px 14px", border: 0, borderLeft: optionIndex === 0 ? 0 : "1px solid var(--ink)",
                  background: side === option.id ? "var(--ink)" : "transparent", color: side === option.id ? "var(--putih)" : "var(--ink)",
                  fontSize: 12, fontWeight: 600, cursor: "pointer", fontFamily: "Inter", letterSpacing: "0.06em", textTransform: "uppercase",
                }}>{option.label}</button>
              ))}
            </div>
          </div>
          <div style={{ padding: 20, display: "flex", flexDirection: "column", gap: 18 }}>
            <Field label="IDX Ticker">
              <select className="input mono" value={selectedIpoTicker} onChange={event => setSelectedIpoTicker(event.target.value)}>
                {PSTOCKS.map(stock => <option key={stock.ipo} value={stock.ipo}>{stock.ipo} · {stock.name.replace("Pulsar ", "")}</option>)}
              </select>
            </Field>
            <Field label={side === "mint" ? "Order quantity (lots to buy on IDX)" : "Quantity to redeem (lots to sell on IDX)"}>
              <input className="input mono" value={quantity} onChange={event => setQuantity(event.target.value.replace(/[^0-9]/g, ""))} />
            </Field>
            {side === "mint" && (
              <Field label="Destination of minted p-tokens">
                <div style={{ display: "flex", gap: 0 }}>
                  {[{ id: "liquidity", label: "Liquidity Pool" }, { id: "wallet", label: "Operator Wallet" }].map((option, optionIndex) => (
                    <button key={option.id} onClick={() => setDestination(option.id)} style={{
                      flex: 1, padding: "12px", appearance: "none", border: "1px solid var(--ink)", borderLeftWidth: optionIndex === 0 ? 1 : 0,
                      background: destination === option.id ? "var(--ink)" : "var(--canvas)", color: destination === option.id ? "var(--putih)" : "var(--ink)",
                      fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "Inter",
                    }}>{option.label}</button>
                  ))}
                </div>
              </Field>
            )}
          </div>

          {/* ORDER PREVIEW */}
          <div className="hairline" style={{ padding: "16px 20px", background: "var(--canvas-soft)" }}>
            <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 10 }}>02 · Order preview</div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              <DetailRow k="IDR notional"   v={`Rp ${idrTotal.toLocaleString("id-ID")}`} />
              <DetailRow k="USD equivalent" v={fmtUSD(usdTotal)} hint="@ 16,142 IDR/USD" />
              <DetailRow k={side === "mint" ? "Broker fee" : "Exit fee"} v={`${fmtUSD(usdTotal * 0.0015)}`} hint="0.15%" />
              <DetailRow k="Gas (Arb Sepolia)" v="~$0.18" />
              <DetailRow k={side === "mint" ? "Mint output" : "IDR returned"} v={side === "mint" ? `${parseInt(quantity || "0").toLocaleString()} ${selectedStock?.ticker ?? selectedIpoTicker}` : `Rp ${idrTotal.toLocaleString("id-ID")}`} hint="1:1 peg" />
            </div>
          </div>

          <div style={{ padding: 20 }}>
            <button onClick={runPipeline} disabled={running || !quantity} className={`btn ${side === "mint" ? "btn-merah" : "btn-primary"}`}
              style={{ width: "100%", padding: 16, fontSize: 15, display: "inline-flex", justifyContent: "center", alignItems: "center", gap: 10 }}>
              {running ? <Icon name="loader" size={14} /> : <Icon name="play" size={14} />}
              {running ? "Executing pipeline…" : `Execute ${side === "mint" ? "mint" : "redeem"} pipeline`}
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

      {/* REQUEST QUEUE */}
      <div style={{ marginTop: 56 }}>
        <div className="hairline-strong" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 12, flexWrap: "wrap", gap: 8 }}>
          <h2 className="display section-title" style={{ fontSize: 32, margin: 0, letterSpacing: "-0.02em" }}>Request queue</h2>
          <div className="eyebrow" style={{ color: "var(--body)" }}>4 pending · 2 retail · 2 institutional</div>
        </div>
        <RequestQueue />
      </div>

      {/* VAULT INVENTORY */}
      <div style={{ marginTop: 56 }}>
        <div className="hairline-strong" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 12, flexWrap: "wrap", gap: 8 }}>
          <h2 className="display section-title" style={{ fontSize: 32, margin: 0, letterSpacing: "-0.02em" }}>Physical vault & proof of reserves</h2>
          <div className="eyebrow" style={{ color: "var(--body)" }}>last attestation · 8 min ago</div>
        </div>
        <ReservesTable />
      </div>
    </div>
  );
}

function Metric({ label, value, sub, tone = "default" }: { label: string; value: string; sub: string; tone?: "ink" | "merah" | "default" }) {
  const isInk = tone === "ink", isMerah = tone === "merah";
  return (
    <div className="stat-card" style={{ padding: 22, background: isInk ? "var(--ink)" : isMerah ? "var(--merah)" : "var(--canvas)", color: (isInk || isMerah) ? "var(--putih)" : "var(--ink)", border: "1px solid " + (isInk ? "var(--ink)" : isMerah ? "var(--merah)" : "var(--hairline)") }}>
      <div className="eyebrow" style={{ color: (isInk || isMerah) ? "rgba(255,255,255,0.7)" : "var(--body)" }}>{label}</div>
      <div className="display tnum" style={{ fontSize: 32, marginTop: 8, lineHeight: 1, letterSpacing: "-0.02em" }}>{value}</div>
      <div style={{ marginTop: 8, fontSize: 12, color: (isInk || isMerah) ? "rgba(255,255,255,0.6)" : "var(--body)" }}>{sub}</div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 8 }}>{label}</div>
      {children}
    </div>
  );
}

function Pill({ label, value, tone = "neutral" }: { label: string; value: string; tone?: "green" | "red" | "neutral" }) {
  const dotColor = { green: "#52ce7a", red: "#ff6a6a", neutral: "var(--body)" }[tone];
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, border: "1px solid var(--hairline-strong)", padding: "6px 10px" }}>
      <span style={{ width: 7, height: 7, background: dotColor, borderRadius: 999 }} />
      <span className="eyebrow" style={{ color: "var(--body)" }}>{label}</span>
      <span className="mono" style={{ fontSize: 11, fontWeight: 600 }}>{value}</span>
    </div>
  );
}

function Cursor() {
  const [isVisible, setIsVisible] = useState(true);
  useEffect(() => {
    const intervalId = setInterval(() => setIsVisible(previousVisible => !previousVisible), 500);
    return () => clearInterval(intervalId);
  }, []);
  return <span style={{ display: "inline-block", width: 7, height: 13, background: isVisible ? "#fff" : "transparent", verticalAlign: -2 }} />;
}

const REQUESTS = [
  { id: "REQ-9821", kind: "Mint",   ticker: "BUMIP", qty: 250_000, who: "0x7a31…3c2f", waited: "12 min", tier: "Retail",        idr: 61_500_000 },
  { id: "REQ-9820", kind: "Mint",   ticker: "TLKMP", qty: 8_400,   who: "Mirae IB",    waited: "38 min", tier: "Institutional", idr: 24_998_400 },
  { id: "REQ-9819", kind: "Redeem", ticker: "GOTOP", qty: 120_000, who: "0x9c1f…ae42", waited: "1 h 4 m",tier: "Retail",        idr: 11_760_000 },
  { id: "REQ-9818", kind: "Redeem", ticker: "ENRGP", qty: 18_200,  who: "Mandiri TM",  waited: "2 h 1 m",tier: "Institutional", idr: 6_879_600 },
];

function RequestQueue() {
  const [completedRequests, setCompletedRequests] = useState<Record<string, string>>({});

  function approve(request: typeof REQUESTS[0]) {
    setCompletedRequests(previous => ({ ...previous, [request.id]: "approved" }));
    const toastId = toast.loading(`Approving ${request.id}`, { description: `${request.kind} ${fmtNum(request.qty)} ${request.ticker}` });
    setTimeout(() => toast.success(`${request.id} approved`, { id: toastId, description: "Pipeline queued · ETA ~30s", duration: 3500 }), 1000);
  }
  function reject(request: typeof REQUESTS[0]) {
    setCompletedRequests(previous => ({ ...previous, [request.id]: "rejected" }));
    toast.info(`${request.id} rejected`, { description: "Reason logged · operator memo required", duration: 3000 });
  }

  return (
    <div>
      <div className="hairline table-head-desktop" style={{ display: "grid", gridTemplateColumns: "auto 1fr 1fr 1fr 1fr 1fr 1fr 1.6fr", gap: 12, padding: "12px 0" }}>
        {["", "ID", "Type", "Asset", "Quantity", "IDR notional", "Waited", ""].map((heading, columnIndex) => (
          <div key={columnIndex} className="eyebrow" style={{ color: "var(--body)", textAlign: columnIndex >= 4 && columnIndex <= 6 ? "right" : "left" }}>{heading}</div>
        ))}
      </div>
      {REQUESTS.map(request => {
        const status = completedRequests[request.id];
        const isMint = request.kind === "Mint";
        return (
          <div key={request.id} className="hairline table-row-stack" style={{ display: "grid", gridTemplateColumns: "auto 1fr 1fr 1fr 1fr 1fr 1fr 1.6fr", gap: 12, padding: "16px 0", alignItems: "center", opacity: status ? 0.55 : 1 }}>
            <div className="col-asset" style={{ width: 32, height: 32, border: "1px solid var(--ink)", display: "flex", alignItems: "center", justifyContent: "center", background: isMint ? "var(--merah)" : "var(--ink)", color: "var(--putih)" }}>
              <Icon name={isMint ? "arrow-up" : "arrow-down"} size={14} />
            </div>
            <div className="row-cell"><span className="row-cell-label">ID</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{request.id}</span></div></div>
            <div className="row-cell"><span className="row-cell-label">Type</span><div className="row-cell-value"><span style={{ fontSize: 11, fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: isMint ? "var(--merah)" : "var(--ink)" }}>{request.kind} · {request.tier}</span></div></div>
            <div className="row-cell"><span className="row-cell-label">Asset</span><div className="row-cell-value"><div style={{ display: "flex", alignItems: "center", gap: 8 }}><PStockMark ticker={request.ticker} size={22} /><span style={{ fontWeight: 600, fontSize: 13 }}>{request.ticker}</span></div></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Quantity</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{fmtNum(request.qty)}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">IDR notional</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{fmtIDR(request.idr)}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Waited</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>{request.waited}</span></div></div>
            <div className="col-actions pos-actions" style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
              {!status && (
                <>
                  <button className="btn btn-ghost" onClick={() => reject(request)} style={{ padding: "6px 12px", border: "1px solid var(--ink)" }}>Reject</button>
                  <button className="btn btn-primary" onClick={() => approve(request)} style={{ padding: "6px 14px" }}>Approve</button>
                </>
              )}
              {status && (
                <span style={{ fontSize: 11, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: status === "approved" ? "var(--positive)" : "var(--body)" }}>
                  {status === "approved" ? "Approved · pipeline queued" : "Rejected"}
                </span>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function ReservesTable() {
  return (
    <div>
      <div className="hairline table-head-desktop" style={{ display: "grid", gridTemplateColumns: "1.6fr 1.2fr 1.2fr 1fr 1fr 1fr", gap: 16, padding: "14px 0" }}>
        {["Asset", "Custodian holdings", "On-chain supply", "Peg ratio", "Last mint", "Status"].map((heading, columnIndex) => (
          <div key={columnIndex} className="eyebrow" style={{ color: "var(--body)", textAlign: columnIndex >= 1 && columnIndex <= 5 ? "right" : "left" }}>{heading}</div>
        ))}
      </div>
      {PSTOCKS.map((stock, rowIndex) => {
        const onChainSupply  = stock.supply;
        const custodyBalance = onChainSupply + (stock.ticker === "GOTOP" ? 1 : 0);
        const pegRatio       = (custodyBalance / onChainSupply).toFixed(4);
        const isPegged       = parseFloat(pegRatio) >= 1;
        return (
          <div key={stock.ticker} className="hairline table-row-stack" style={{ display: "grid", gridTemplateColumns: "1.6fr 1.2fr 1.2fr 1fr 1fr 1fr", gap: 16, padding: "16px 0", alignItems: "center" }}>
            <div className="col-asset" style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <PStockMark ticker={stock.ticker} size={32} />
              <div>
                <div style={{ fontWeight: 600, fontSize: 14 }}>{stock.ticker}</div>
                <div style={{ fontSize: 11, color: "var(--body)" }}>{stock.ipo} · {stock.sector}</div>
              </div>
            </div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Custodian holdings</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{custodyBalance.toLocaleString()}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">On-chain supply</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{onChainSupply.toLocaleString()}</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Peg ratio</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13, color: isPegged ? "var(--positive)" : "var(--negative)" }}>{pegRatio}×</span></div></div>
            <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Last mint</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>{["8m","1h","3h","6h","12h","1d","1d","2d"][rowIndex]} ago</span></div></div>
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
