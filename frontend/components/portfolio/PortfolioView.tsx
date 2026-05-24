'use client';

import { useState, useMemo } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { toast } from 'sonner';
import { PSTOCKS, tokenByTicker, portfolioTimeSeries, sliceRange, fmtIDRX, fmtNum, fmtPct, fmtAxisDate, shortAddr, Balances } from '@/lib/data';
import { Icon } from '@/components/ui/Icon';
import { PStockMark } from '@/components/ui/PStockMark';
import { AreaChart } from '@/components/charts/AreaChart';
import { Donut } from '@/components/charts/Donut';
import { SwapModal } from '@/components/ui/SwapModal';
import { DetailRow } from '@/components/swap/SwapView';

const PALETTE = ["#c8102e", "#16110e", "#1f4d8a", "#5a4a3a", "#9a0c24", "#2a231e", "#6f2da8", "#2c5e2e"];

interface PortfolioViewProps {
  balances: Balances;
  setBalances: React.Dispatch<React.SetStateAction<Balances>>;
  costBasis: Record<string, number>;
}

export function PortfolioView({ balances, setBalances, costBasis }: PortfolioViewProps) {
  const { address, isConnected } = useAccount();
  const [range, setRange]         = useState("1M");
  const [expanded, setExpanded]   = useState<string | null>(null);
  const [transferOpen, setTransferOpen] = useState<{ ticker: string; name: string; price: number } | null>(null);
  const [tradeToken, setTradeToken] = useState<{ ticker: string; name: string; price: number } | null>(null);

  const series = useMemo(() => portfolioTimeSeries("PFV", 365, 18400 * 16142, 26840 * 16142), []);
  const ranged = useMemo(() => sliceRange(series, range), [series, range]);

  if (!isConnected) {
    return (
      <div className="pad-x" style={{ padding: "80px 24px", display: "flex", flexDirection: "column", alignItems: "center", gap: 20 }}>
        <div className="eyebrow" style={{ color: "var(--merah)" }}>Portfolio · No wallet connected</div>
        <h1 className="display hero-display" style={{ margin: 0, fontSize: 56, textAlign: "center", maxWidth: 720, lineHeight: 1, letterSpacing: "-0.025em" }}>
          Connect to view your <span className="display-it">cosmos</span> of holdings.
        </h1>
        <p style={{ marginTop: 8, color: "var(--body)", textAlign: "center", maxWidth: 520 }}>
          Your tokenized equity positions, avg buy price, unrealized P&L and 24-hour change will appear once you sign in with a wallet on Arbitrum Sepolia.
        </p>
        <ConnectButton.Custom>
          {({ openConnectModal }) => (
            <button className="btn btn-merah" onClick={openConnectModal} style={{ marginTop: 12 }}>Connect Wallet</button>
          )}
        </ConnectButton.Custom>
      </div>
    );
  }

  const positions = Object.entries(balances)
    .filter(([k, v]) => v > 0 && k !== "IDRX")
    .map(([ticker, qty]) => {
      const tok = tokenByTicker(ticker);
      const avg   = costBasis[ticker] ?? tok.price;
      const value = qty * tok.price;
      const cost  = qty * avg;
      const pnl   = value - cost;
      const pnlPct = avg ? ((tok.price - avg) / avg) * 100 : 0;
      const dayPrev = tok.price / (1 + ((tok as { change24h?: number }).change24h ?? 0) / 100);
      const dayPnl  = qty * (tok.price - dayPrev);
      return { ...tok, qty, avg, value, cost, pnl, pnlPct, dayPnl };
    })
    .sort((a, b) => b.value - a.value);

  const stables = Object.entries(balances)
    .filter(([k]) => k === "IDRX")
    .map(([ticker, qty]) => ({ ...tokenByTicker(ticker), qty, value: qty }));

  const stockValue   = positions.reduce((s, p) => s + p.value, 0);
  const stockCost    = positions.reduce((s, p) => s + p.cost,  0);
  const stableValue  = stables.reduce((s, p) => s + p.value,   0);
  const totalValue   = stockValue + stableValue;
  const allTimePnl   = stockValue - stockCost;
  const allTimePnlPct = stockCost ? (allTimePnl / stockCost) * 100 : 0;
  const dayPnl       = positions.reduce((s, p) => s + p.dayPnl, 0);
  const dayPnlPct    = stockValue - dayPnl ? (dayPnl / (stockValue - dayPnl)) * 100 : 0;

  const donutData = positions.map(position => ({ label: position.ticker, value: position.value }));

  function handleTransfer(opts: { token: { ticker: string }; to: string; amt: number }) {
    setTransferOpen(null);
    const toastId = toast.loading("Broadcasting transfer…");
    setTimeout(() => {
      toast.success("Transfer sent", { id: toastId, description: `${fmtNum(opts.amt, 2)} ${opts.token.ticker} → ${shortAddr(opts.to)}`, duration: 4500 });
    }, 1200);
  }

  return (
    <div className="pad-x" style={{ padding: "32px 24px 16px" }}>
      {/* HERO */}
      <div className="grid-2col-balanced">
        <div>
          <div className="eyebrow" style={{ color: "var(--merah)", marginBottom: 12 }}>Portfolio · {shortAddr(address!)}</div>
          <div style={{ display: "flex", alignItems: "baseline", gap: 12, flexWrap: "wrap", marginBottom: 6 }}>
            <span className="eyebrow" style={{ color: "var(--body)" }}>Total Net Worth</span>
          </div>
          <div className="display tnum total-display" style={{ fontSize: 72, lineHeight: 0.92, letterSpacing: "-0.035em" }}>{fmtIDRX(totalValue)}</div>
          <div style={{ display: "flex", gap: 28, flexWrap: "wrap", marginTop: 18 }}>
            <PnLChip label="Today"              v={dayPnl}     pct={dayPnlPct} />
            <PnLChip label="All-time unrealized" v={allTimePnl} pct={allTimePnlPct} />
            <PnLChip label="Cash · stables"     v={stableValue} muted />
          </div>
        </div>
        <SplitRow stockValue={stockValue} stableValue={stableValue} />
      </div>

      {/* CHART */}
      <div className="hairline-top" style={{ marginTop: 32, paddingTop: 24 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 12, marginBottom: 18 }}>
          <div>
            <div className="eyebrow" style={{ color: "var(--body)" }}>Portfolio value</div>
            <div className="display" style={{ fontSize: 20, marginTop: 2 }}>{range === "1D" ? "Today" : range === "ALL" ? "All time" : `Last ${range}`}</div>
          </div>
          <div className="range-pills">
            {["1D","1W","1M","3M","1Y","ALL"].map(timeframeOption => (
              <button key={timeframeOption} className={range === timeframeOption ? "active" : ""} onClick={() => setRange(timeframeOption)}>{timeframeOption}</button>
            ))}
          </div>
        </div>
        <AreaChart data={ranged} height={280} valueFormatter={value => `${(value / 1_000_000).toFixed(1)}M IDRX`} labelFormatter={fmtAxisDate} range={range} />
      </div>

      {/* ALLOCATION */}
      {donutData.length > 0 && (
        <div className="hairline-top" style={{ marginTop: 36, paddingTop: 24 }}>
          <div className="eyebrow" style={{ marginBottom: 16, color: "var(--body)" }}>Allocation</div>
          <div style={{ display: "flex", gap: 32, alignItems: "center", flexWrap: "wrap" }}>
            <Donut data={donutData} size={180} thickness={22} palette={PALETTE} />
            <div style={{ flex: 1, minWidth: 240, display: "grid", gridTemplateColumns: "1fr 1fr", gap: "10px 24px" }}>
              {donutData.map((donutSegment, segmentIndex) => (
                <div key={donutSegment.label} style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ width: 10, height: 10, background: PALETTE[segmentIndex % PALETTE.length], flexShrink: 0 }} />
                  <span style={{ fontWeight: 600, fontSize: 13 }}>{donutSegment.label}</span>
                  <span className="mono" style={{ fontSize: 12, color: "var(--body)", marginLeft: "auto" }}>{((donutSegment.value / stockValue) * 100).toFixed(1)}%</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* POSITIONS */}
      <div style={{ marginTop: 40 }}>
        <div className="hairline-strong" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 10, flexWrap: "wrap", gap: 8 }}>
          <h2 className="display section-title" style={{ fontSize: 32, margin: 0, letterSpacing: "-0.02em" }}>Positions</h2>
          <div className="eyebrow" style={{ color: "var(--body)" }}>{positions.length} stocks · {stables.length} stable</div>
        </div>
        <PositionsList positions={positions} stables={stables} expanded={expanded} setExpanded={setExpanded} onTrade={t => setTradeToken(t)} onTransfer={t => setTransferOpen(t)} />
      </div>

      {/* ACTIVITY */}
      <div style={{ marginTop: 56 }}>
        <div className="hairline-strong" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 10, flexWrap: "wrap" }}>
          <h2 className="display section-title" style={{ fontSize: 32, margin: 0, letterSpacing: "-0.02em" }}>Recent activity</h2>
          <span className="eyebrow" style={{ color: "var(--body)" }}>last 7 days</span>
        </div>
        <ActivityList />
      </div>

      {transferOpen && (
        <TransferModal
          token={transferOpen}
          balance={balances[transferOpen.ticker] ?? 0}
          onClose={() => setTransferOpen(null)}
          onSubmit={handleTransfer}
        />
      )}

      {tradeToken && (
        <SwapModal
          defaultOut={tokenByTicker(tradeToken.ticker)}
          balances={balances}
          setBalances={setBalances}
          onClose={() => setTradeToken(null)}
        />
      )}
    </div>
  );
}

function PnLChip({ label, v, pct, muted }: { label: string; v: number; pct?: number; muted?: boolean }) {
  const pos = v >= 0;
  const color = muted ? "var(--ink)" : pos ? "var(--positive)" : "var(--negative)";
  return (
    <div>
      <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 4 }}>{label}</div>
      <div className="mono" style={{ fontSize: 19, fontWeight: 500, color, lineHeight: 1.1 }}>
        {!muted && (v >= 0 ? "+" : "−")}{fmtIDRX(Math.abs(v))}
      </div>
      {pct != null && <div className="mono" style={{ fontSize: 12, color, marginTop: 2 }}>{fmtPct(pct)}</div>}
    </div>
  );
}

function SplitRow({ stockValue, stableValue }: { stockValue: number; stableValue: number }) {
  const total = stockValue + stableValue;
  const pStockPct = total ? (stockValue / total) * 100 : 0;
  return (
    <div>
      <div style={{ display: "flex", height: 6, marginBottom: 14, background: "var(--canvas-soft)" }}>
        <div style={{ width: `${pStockPct}%`, background: "var(--merah)" }} />
        <div style={{ flex: 1, background: "var(--ink)" }} />
      </div>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 24, fontSize: 13 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ width: 8, height: 8, background: "var(--merah)" }} />
            <span className="eyebrow" style={{ color: "var(--body)" }}>pStocks · {pStockPct.toFixed(1)}%</span>
          </div>
          <div className="mono" style={{ fontSize: 18, marginTop: 4 }}>{fmtIDRX(stockValue)}</div>
        </div>
        <div style={{ textAlign: "right" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, justifyContent: "flex-end" }}>
            <span style={{ width: 8, height: 8, background: "var(--ink)" }} />
            <span className="eyebrow" style={{ color: "var(--body)" }}>IDRX · {(100 - pStockPct).toFixed(1)}%</span>
          </div>
          <div className="mono" style={{ fontSize: 18, marginTop: 4 }}>{fmtIDRX(stableValue)}</div>
        </div>
      </div>
    </div>
  );
}

type Position = {
  ticker: string; name: string; price: number; change24h?: number; sector?: string; ipo?: string; supply?: number;
  qty: number; avg: number; value: number; cost: number; pnl: number; pnlPct: number; dayPnl: number;
};

function PositionsList({ positions, stables, expanded, setExpanded, onTrade, onTransfer }: {
  positions: Position[];
  stables: { ticker: string; name: string; price: number; qty: number; value: number }[];
  expanded: string | null;
  setExpanded: (t: string | null) => void;
  onTrade: (t: { ticker: string; name: string; price: number }) => void;
  onTransfer: (t: { ticker: string; name: string; price: number }) => void;
}) {
  return (
    <div>
      <div className="hairline table-head-desktop" style={{ display: "grid", gridTemplateColumns: "auto 2fr 1fr 1fr 1fr 1fr 1fr 156px", gap: 16, padding: "14px 0" }}>
        {["", "Stock", "Lot", "Avg buy", "Last", "Market value", "Unrealized P&L", ""].map((h, i) => (
          <div key={i} className="eyebrow" style={{ color: "var(--body)", textAlign: i >= 2 && i <= 6 ? "right" : "left" }}>{h}</div>
        ))}
      </div>
      {positions.map(p => {
        const pos    = p.pnl >= 0;
        const isOpen = expanded === p.ticker;
        return (
          <div key={p.ticker} className="hairline">
            <div className="row-hover position-row" onClick={() => setExpanded(isOpen ? null : p.ticker)} style={{ cursor: "pointer" }}>
              <div className="pos-head">
                <PStockMark ticker={p.ticker} size={40} />
                <div className="col-asset" style={{ minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <span style={{ fontWeight: 600, fontSize: 15 }}>{p.ticker}</span>
                    {p.ipo && <span style={{ fontSize: 11, color: "var(--body)", border: "1px solid var(--hairline-strong)", padding: "1px 6px" }}>{p.ipo}</span>}
                    <span style={{ fontSize: 11, color: (p.change24h ?? 0) >= 0 ? "var(--positive)" : "var(--negative)", fontWeight: 600 }} className="mono">
                      {fmtPct(p.change24h ?? 0)} today
                    </span>
                  </div>
                  <div style={{ fontSize: 12, color: "var(--body)", marginTop: 2 }}>{p.name}</div>
                </div>
              </div>
              <div className="pos-data">
                <RowCell label="Lot" align="right"><span className="mono" style={{ fontSize: 14 }}>{fmtNum(p.qty, 2)}</span></RowCell>
                <RowCell label="Avg buy" align="right"><span className="mono" style={{ fontSize: 14 }}>{fmtIDRX(p.avg)}</span></RowCell>
                <RowCell label="Last" align="right"><span className="mono" style={{ fontSize: 14 }}>{fmtIDRX(p.price)}</span></RowCell>
                <RowCell label="Market value" align="right"><span className="mono" style={{ fontSize: 15, fontWeight: 500 }}>{fmtIDRX(p.value)}</span></RowCell>
                <RowCell label="Unrealized P&L" align="right">
                  <div className="mono" style={{ fontSize: 14, color: pos ? "var(--positive)" : "var(--negative)", fontWeight: 500 }}>
                    {pos ? "+" : "−"}{fmtIDRX(Math.abs(p.pnl))}
                  </div>
                  <div className="mono" style={{ fontSize: 11, color: pos ? "var(--positive)" : "var(--negative)" }}>{fmtPct(p.pnlPct)}</div>
                </RowCell>
              </div>
              <div className="pos-actions">
                <button className="btn btn-ghost" onClick={e => { e.stopPropagation(); onTrade(p); }} style={{ padding: "6px 12px", fontSize: 13, border: "1px solid var(--ink)" }}>Trade</button>
                <button className="btn btn-ghost" onClick={e => { e.stopPropagation(); onTransfer(p); }} style={{ padding: "6px 12px", fontSize: 13, border: "1px solid var(--ink)" }}>Send</button>
              </div>
            </div>
            {isOpen && <PositionDetail position={p} />}
          </div>
        );
      })}

      {stables.map(s => (
        <div key={s.ticker} className="hairline row-hover position-row">
          <div className="pos-head">
            <PStockMark ticker={s.ticker} size={40} />
            <div className="col-asset" style={{ minWidth: 0 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ fontWeight: 600, fontSize: 15 }}>{s.ticker}</span>
                <span style={{ fontSize: 11, color: "var(--body)", border: "1px solid var(--hairline-strong)", padding: "1px 6px" }}>STABLE</span>
              </div>
              <div style={{ fontSize: 12, color: "var(--body)", marginTop: 2 }}>{s.name}</div>
            </div>
          </div>
          <div className="pos-data">
            <RowCell label="Lot" align="right"><span className="mono" style={{ fontSize: 14 }}>{fmtNum(s.qty, 2)}</span></RowCell>
            <RowCell label="Avg buy" align="right"><span className="mono" style={{ fontSize: 14 }}>1 IDRX</span></RowCell>
            <RowCell label="Last" align="right"><span className="mono" style={{ fontSize: 14 }}>1 IDRX</span></RowCell>
            <RowCell label="Market value" align="right"><span className="mono" style={{ fontSize: 15 }}>{fmtIDRX(s.value)}</span></RowCell>
            <RowCell label="Unrealized P&L" align="right"><span className="mono" style={{ fontSize: 13, color: "var(--body)" }}>—</span></RowCell>
          </div>
          <div className="pos-actions">
            <button className="btn btn-ghost" style={{ padding: "6px 12px", fontSize: 13, border: "1px solid var(--ink)" }}>Trade</button>
            <button className="btn btn-ghost" style={{ padding: "6px 12px", fontSize: 13, border: "1px solid var(--ink)" }}>Send</button>
          </div>
        </div>
      ))}
    </div>
  );
}

function RowCell({ label, align = "left", children }: { label: string; align?: "left" | "right"; children: React.ReactNode }) {
  return (
    <div className="row-cell" style={{ textAlign: align, minWidth: 0 }}>
      <span className="row-cell-label">{label}</span>
      <div className="row-cell-value">{children}</div>
    </div>
  );
}

function PositionDetail({ position }: { position: Position }) {
  const [range, setRange] = useState("1M");
  const series = useMemo(() => {
    const raw = portfolioTimeSeries(position.ticker, 365, position.price * 600, position.price * 1100);
    const lastValue = raw[raw.length - 1].value;
    return raw.map(dataPoint => ({ timestamp: dataPoint.timestamp, value: (dataPoint.value / lastValue) * position.price }));
  }, [position.ticker, position.price]);
  const ranged = useMemo(() => sliceRange(series, range), [series, range]);

  return (
    <div style={{ background: "var(--canvas-soft)", padding: 20, marginTop: -1, borderTop: "1px solid var(--hairline)" }}>
      <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 2fr) minmax(0, 1fr)", gap: 24, alignItems: "flex-start" }} className="grid-2col-form">
        <div>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14, flexWrap: "wrap", gap: 12 }}>
            <div>
              <div className="eyebrow" style={{ color: "var(--body)" }}>{position.ticker} · {position.name}</div>
              <div className="display" style={{ fontSize: 24, marginTop: 2 }}>{fmtIDRX(position.price)}</div>
            </div>
            <div className="range-pills">
              {["1D","1W","1M","3M","1Y"].map(r => (
                <button key={r} className={range === r ? "active" : ""} onClick={() => setRange(r)}>{r}</button>
              ))}
            </div>
          </div>
          <AreaChart data={ranged} height={200} valueFormatter={v => fmtIDRX(v)} labelFormatter={fmtAxisDate} range={range} />
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 12, paddingTop: 4 }}>
          <KV k="Lot owned"     v={`${fmtNum(position.qty, 2)}`} />
          <KV k="Avg buy price" v={fmtIDRX(position.avg)} />
          <KV k="Current price" v={fmtIDRX(position.price)} />
          <KV k="Total cost"    v={fmtIDRX(position.cost)} />
          <KV k="Market value"  v={fmtIDRX(position.value)} highlight />
          <KV k="Unrealized P&L" v={
            <span style={{ color: position.pnl >= 0 ? "var(--positive)" : "var(--negative)", fontWeight: 600 }}>
              {position.pnl >= 0 ? "+" : "−"}{fmtIDRX(Math.abs(position.pnl))} ({fmtPct(position.pnlPct)})
            </span>
          } />
          {position.sector && <KV k="Sector" v={position.sector} />}
          {position.ipo    && <KV k="IDX ticker" v={position.ipo} />}
          {position.supply && <KV k="Token supply" v={fmtNum(position.supply)} />}
        </div>
      </div>
    </div>
  );
}

function KV({ k, v, highlight }: { k: string; v: React.ReactNode; highlight?: boolean }) {
  return (
    <div className="hairline" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 8, gap: 16 }}>
      <span style={{ fontSize: 12, color: "var(--body)" }}>{k}</span>
      <span className="mono" style={{ fontSize: highlight ? 15 : 13, fontWeight: highlight ? 600 : 400, textAlign: "right" }}>{v}</span>
    </div>
  );
}

const ACTIVITY = [
  { kind: "swap", when: "2h ago",  text: "Buy",    a: "78.400 IDRX",   b: "320 BUMIP",    fee: "$0.21", status: "Confirmed", hash: "0x7f3a…b821" },
  { kind: "mint", when: "8h ago",  text: "Mint",   a: "—",             b: "5.000 TLKMP",  fee: "$0.14", status: "Confirmed", hash: "0x4c2e…91a7" },
  { kind: "swap", when: "1d ago",  text: "Sell",   a: "84 ENRGP",      b: "31.752 IDRX",  fee: "$0.18", status: "Confirmed", hash: "0xa991…f0c4" },
  { kind: "send", when: "2d ago",  text: "Send",   a: "12.400 GOTOP",  b: "0x9c1f…ae42",  fee: "$0.12", status: "Confirmed", hash: "0xd44b…02f1" },
  { kind: "fail", when: "3d ago",  text: "Failed", a: "6.455.000 IDRX",b: "BBRIP swap",   fee: "$0.05", status: "Reverted",  hash: "0x6e1c…71b3" },
  { kind: "swap", when: "5d ago",  text: "Buy",    a: "3.234.000 IDRX",b: "32.980 GOTOP", fee: "$0.16", status: "Confirmed", hash: "0xfe21…44ab" },
];

function ActivityList() {
  return (
    <div>
      {ACTIVITY.map((row, i) => (
        <div key={i} className="hairline row-hover activity-row" style={{ display: "grid", gridTemplateColumns: "auto 1fr auto auto auto", gap: 20, padding: "16px 0", alignItems: "center" }}>
          <div style={{ width: 36, height: 36, border: "1px solid var(--ink)", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0, background: row.kind === "fail" ? "var(--merah-soft)" : "var(--canvas)" }}>
            <Icon name={row.kind === "send" ? "send" : row.kind === "fail" ? "x" : "swap"} size={14} />
          </div>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 14, fontWeight: 600 }}>{row.text} <span style={{ fontWeight: 400, color: "var(--body)" }}>· {row.a}{row.b && " → "}{row.b}</span></div>
            <div style={{ fontSize: 12, color: "var(--body)", marginTop: 3 }}>
              <span className="mono">{row.hash}</span> · {row.when}
            </div>
          </div>
          <div style={{ fontSize: 11, color: row.status === "Reverted" ? "var(--negative)" : "var(--positive)", fontWeight: 600, letterSpacing: "0.06em", textTransform: "uppercase" }}>{row.status}</div>
          <div className="mono only-desktop" style={{ fontSize: 12, color: "var(--body)" }}>{row.fee}</div>
          <a className="btn-ghost btn only-desktop" href="#" onClick={e => e.preventDefault()} style={{ display: "inline-flex", alignItems: "center", gap: 6, padding: 4 }}>
            <Icon name="external" size={13} />
          </a>
        </div>
      ))}
    </div>
  );
}

function TransferModal({ token, balance, onClose, onSubmit }: {
  token: { ticker: string; name: string; price: number };
  balance: number;
  onClose: () => void;
  onSubmit: (opts: { token: { ticker: string }; to: string; amt: number }) => void;
}) {
  const [to, setTo]   = useState("");
  const [amt, setAmt] = useState("");
  const num = parseFloat(amt) || 0;
  const ok  = to.length >= 6 && num > 0 && num <= balance;

  return (
    <div className="overlay" style={{ position: "fixed", inset: 0, background: "rgba(22,17,14,0.45)", zIndex: 200, display: "flex", alignItems: "center", justifyContent: "center", padding: 16 }} onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ background: "var(--putih)", width: 440, maxWidth: "100%", border: "1px solid var(--ink)", boxShadow: "8px 8px 0 0 rgba(22,17,14,0.10)" }}>
        <div className="hairline" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "16px 20px" }}>
          <div className="display" style={{ fontSize: 22 }}>Send {token.ticker}</div>
          <button className="btn-ghost btn" onClick={onClose} style={{ padding: 4 }}><Icon name="x" /></button>
        </div>
        <div style={{ padding: 20, display: "flex", flexDirection: "column", gap: 14 }}>
          <div>
            <div className="eyebrow" style={{ color: "var(--body)", marginBottom: 6 }}>Recipient address</div>
            <input className="input mono" placeholder="0x… or .arb name" value={to} onChange={e => setTo(e.target.value)} />
          </div>
          <div>
            <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 6 }}>
              <div className="eyebrow" style={{ color: "var(--body)" }}>Amount</div>
              <span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>Balance {fmtNum(balance, 4)}</span>
            </div>
            <div style={{ position: "relative" }}>
              <input className="input mono" placeholder="0.00" value={amt} onChange={e => setAmt(e.target.value.replace(/[^0-9.]/g, ""))} style={{ paddingRight: 60 }} />
              <button onClick={() => setAmt(balance.toString())} style={{ position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)", appearance: "none", border: "1px solid var(--hairline-strong)", background: "var(--canvas)", padding: "4px 8px", fontSize: 11, fontWeight: 600, fontFamily: "Inter", cursor: "pointer" }}>MAX</button>
            </div>
            {num > balance && <div style={{ fontSize: 12, color: "var(--negative)", marginTop: 6 }}>Exceeds available balance</div>}
          </div>
          <div className="hairline-top" style={{ paddingTop: 14, display: "flex", flexDirection: "column", gap: 6, fontSize: 13 }}>
            <DetailRow k="Network"     v="Arbitrum Sepolia" />
            <DetailRow k="Network fee" v="~$0.12" />
          </div>
          <button className="btn btn-primary" disabled={!ok} onClick={() => onSubmit({ token, to, amt: num })} style={{ width: "100%", padding: 14 }}>
            Send {token.ticker}
          </button>
        </div>
      </div>
    </div>
  );
}
