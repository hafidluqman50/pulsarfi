'use client';

import { useState, useMemo } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { toast } from 'sonner';
import { type Address } from 'viem';
import { tokenByTicker, sliceRange, fmtIDRX, fmtNum, fmtPct, shortAddr } from '@/lib/data';
import { useMarketStocks, useStockTransactions } from '@/http/market/hooks';
import { useWalletTokenBalances } from '@/http/market/tokenHooks';
import { useTransferToken } from '@/http/market/transferHooks';
import { toMarketToken, unitsFromDecimalInput } from '@/lib/swap';
import {
  ActivityRow,
  PortfolioPosition,
  StablePosition,
  buildActivityRows,
  buildPortfolioSeries,
  buildPositions,
  buildStables,
} from '@/lib/portfolio';
import { Icon } from '@/components/ui/Icon';
import { PStockMark } from '@/components/ui/PStockMark';
import { AreaChart } from '@/components/charts/AreaChart';
import { Donut } from '@/components/charts/Donut';
import { SwapModal } from '@/components/ui/SwapModal';
import { DetailRow } from '@/components/swap/SwapView';

const PALETTE = ["#c8102e", "#16110e", "#1f4d8a", "#5a4a3a", "#9a0c24", "#2a231e", "#6f2da8", "#2c5e2e"];
const PALETTE_CLASSES = [
  "bg-[#c8102e]",
  "bg-[#16110e]",
  "bg-[#1f4d8a]",
  "bg-[#5a4a3a]",
  "bg-[#9a0c24]",
  "bg-[#2a231e]",
  "bg-[#6f2da8]",
  "bg-[#2c5e2e]",
];
const POSITION_CHART_ANCHOR = new Date('2026-05-26T14:00:00+08:00').getTime();

type TransferToken = {
  ticker: string;
  name: string;
  price: number;
  address: Address;
  isStable: boolean;
};

export function PortfolioView() {
  const { address, isConnected } = useAccount();
  const idrxAddress = process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined;
  const { data: marketStocks = [] } = useMarketStocks();
  const marketTokens = useMemo(() => marketStocks.map(toMarketToken), [marketStocks]);
  const balances = useWalletTokenBalances(marketTokens);
  const { data: transactions = [] } = useStockTransactions(address);
  const transferToken = useTransferToken();

  const [range, setRange]         = useState("1D");
  const [expanded, setExpanded]   = useState<string | null>(null);
  const [transferOpen, setTransferOpen] = useState<TransferToken | null>(null);
  const [tradeToken, setTradeToken] = useState<PortfolioPosition | null>(null);

  const positions = useMemo(() => buildPositions(balances, marketStocks, transactions), [balances, marketStocks, transactions]);
  const stables = useMemo(() => buildStables(balances), [balances]);
  const stockValue = positions.reduce((sum, position) => sum + position.value, 0);
  const stockCost = positions.reduce((sum, position) => sum + position.cost, 0);
  const stableValue = stables.reduce((sum, stable) => sum + stable.value, 0);
  const totalValue = stockValue + stableValue;
  const allTimePnl = stockValue - stockCost;
  const allTimePnlPct = stockCost ? (allTimePnl / stockCost) * 100 : 0;
  const dayPnl = positions.reduce((sum, position) => sum + position.dayPnl, 0);
  const dayPnlPct = stockValue - dayPnl ? (dayPnl / (stockValue - dayPnl)) * 100 : 0;
  const donutData = positions.map(position => ({ label: position.ticker, value: position.value }));
  const activityRows = useMemo(() => buildActivityRows(transactions), [transactions]);
  const series = useMemo(() => buildPortfolioSeries(totalValue, transactions), [totalValue, transactions]);
  const ranged = useMemo(() => sliceRange(series, range), [series, range]);

  if (!isConnected) {
    return (
      <div className="pad-x flex flex-col items-center gap-[20px] !px-[24px] !py-[80px]">
        <div className="eyebrow !text-[var(--merah)]">Portfolio · No wallet connected</div>
        <h1 className="display hero-display !m-[0] max-w-[720px] text-center !text-[56px] !leading-none !tracking-[-0.025em]">
          Connect to view your <span className="display-it">cosmos</span> of holdings.
        </h1>
        <p className="mt-[8px] max-w-[520px] text-center text-[var(--body)]">
          Your tokenized equity positions, avg buy price, unrealized P&L and 24-hour change will appear once you sign in with a wallet on Arbitrum Sepolia.
        </p>
        <ConnectButton.Custom>
          {({ openConnectModal }) => (
            <button className="btn btn-merah mt-[12px]" onClick={openConnectModal}>Connect Wallet</button>
          )}
        </ConnectButton.Custom>
      </div>
    );
  }

  async function handleTransfer(opts: { token: TransferToken; to: Address; amount: string }) {
    const toastId = toast.loading("Broadcasting transfer...");
    try {
      const txHash = await transferToken.mutateAsync({
        token_address: opts.token.address,
        from: address!,
        to: opts.to,
        amount: unitsFromDecimalInput(opts.amount, opts.token.isStable ? 2 : 18),
        is_stable: Boolean(opts.token.isStable),
      });
      toast.success("Transfer sent", { id: toastId, description: `Tx ${txHash.slice(0, 10)}...${txHash.slice(-6)}`, duration: 4500 });
      setTransferOpen(null);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Transfer failed";
      toast.error("Transfer failed", { id: toastId, description: message.slice(0, 120), duration: 7000 });
    }
  }

  return (
    <div className="pad-x !px-[24px] !pb-[16px] !pt-[32px]">
      {/* HERO */}
      <div className="grid-2col-balanced">
        <div>
          <div className="eyebrow mb-[12px] !text-[var(--merah)]">Portfolio · {shortAddr(address!)}</div>
          <div className="mb-[6px] flex flex-wrap items-baseline gap-[12px]">
            <span className="eyebrow !text-[var(--body)]">Total Net Worth</span>
          </div>
          <div className="display tnum total-display !text-[72px] !leading-[0.92] !tracking-[-0.035em]">{fmtIDRX(totalValue)}</div>
          <div className="mt-[18px] flex flex-wrap gap-[28px]">
            <PnLChip label="Today"              v={dayPnl}     pct={dayPnlPct} />
            <PnLChip label="All-time unrealized" v={allTimePnl} pct={allTimePnlPct} />
            <PnLChip label="Cash · stables"     v={stableValue} muted />
          </div>
        </div>
        <SplitRow stockValue={stockValue} stableValue={stableValue} />
      </div>

      {/* CHART */}
      <div className="hairline-top mt-[32px] pt-[24px]">
        <div className="mb-[18px] flex flex-wrap items-center justify-between gap-[12px]">
          <div>
            <div className="eyebrow !text-[var(--body)]">Portfolio value</div>
            <div className="display mt-[2px] !text-[20px]">{range === "1D" ? "Today" : range === "ALL" ? "All time" : `Last ${range}`}</div>
          </div>
          <div className="range-pills">
            {["1D","1W","1M","3M","1Y","ALL"].map(timeframeOption => (
              <button key={timeframeOption} className={range === timeframeOption ? "active" : ""} onClick={() => setRange(timeframeOption)}>{timeframeOption}</button>
            ))}
          </div>
        </div>
        <AreaChart data={ranged} height={280} valueFormatter={value => `${(value / 1_000_000).toFixed(1)}M IDRX`} />
      </div>

      {/* ALLOCATION */}
      {donutData.length > 0 && (
        <div className="hairline-top mt-[36px] pt-[24px]">
          <div className="eyebrow mb-[16px] !text-[var(--body)]">Allocation</div>
          <div className="flex flex-wrap items-center gap-[32px]">
            <Donut data={donutData} size={180} thickness={22} palette={PALETTE} />
            <div className="grid min-w-[240px] flex-1 grid-cols-[1fr_1fr] gap-x-[24px] gap-y-[10px]">
              {donutData.map((donutSegment, segmentIndex) => (
                <div key={donutSegment.label} className="flex items-center gap-[8px]">
                  <span className={`h-[10px] w-[10px] shrink-0 ${PALETTE_CLASSES[segmentIndex % PALETTE_CLASSES.length]}`} />
                  <span className="text-[13px] font-semibold">{donutSegment.label}</span>
                  <span className="mono ml-auto text-[12px] text-[var(--body)]">{((donutSegment.value / stockValue) * 100).toFixed(1)}%</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* POSITIONS */}
      <div className="mt-[40px]">
        <div className="hairline-strong flex flex-wrap items-baseline justify-between gap-[8px] pb-[10px]">
          <h2 className="display section-title !m-[0] !text-[32px] !tracking-[-0.02em]">Positions</h2>
          <div className="eyebrow !text-[var(--body)]">{positions.length} stocks · {stables.length} stable</div>
        </div>
        <PositionsList positions={positions} stables={stables} idrxAddress={idrxAddress} expanded={expanded} setExpanded={setExpanded} onTrade={t => setTradeToken(t)} onTransfer={t => setTransferOpen(t)} />
      </div>

      {/* ACTIVITY */}
      <div className="mt-[56px]">
        <div className="hairline-strong flex flex-wrap items-baseline justify-between pb-[10px]">
          <h2 className="display section-title !m-[0] !text-[32px] !tracking-[-0.02em]">Recent activity</h2>
          <span className="eyebrow !text-[var(--body)]">wallet swaps</span>
        </div>
        <ActivityList rows={activityRows} />
      </div>

      {transferOpen && (
        <TransferModal
          token={transferOpen}
          balance={balances[transferOpen.ticker] ?? 0}
          onClose={() => setTransferOpen(null)}
          onSubmit={handleTransfer}
          busy={transferToken.isPending}
        />
      )}

      {tradeToken && (
        <SwapModal
          defaultOut={tokenByTicker(tradeToken.ticker)}
          onClose={() => setTradeToken(null)}
        />
      )}
    </div>
  );
}

function PnLChip({ label, v, pct, muted }: { label: string; v: number; pct?: number; muted?: boolean }) {
  const pos = v >= 0;
  const colorClass = muted ? "text-[var(--ink)]" : pos ? "text-[var(--positive)]" : "text-[var(--negative)]";
  return (
    <div>
      <div className="eyebrow mb-[4px] !text-[var(--body)]">{label}</div>
      <div className={`mono text-[19px] font-medium leading-[1.1] ${colorClass}`}>
        {!muted && (v >= 0 ? "+" : "−")}{fmtIDRX(Math.abs(v))}
      </div>
      {pct != null && <div className={`mono mt-[2px] text-[12px] ${colorClass}`}>{fmtPct(pct)}</div>}
    </div>
  );
}

function SplitRow({ stockValue, stableValue }: { stockValue: number; stableValue: number }) {
  const total = stockValue + stableValue;
  const pStockPct = total ? (stockValue / total) * 100 : 0;
  return (
    <div>
      <svg className="mb-[14px] block h-[6px] w-full bg-[var(--canvas-soft)]" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
        <rect x="0" y="0" width={pStockPct} height="6" fill="var(--merah)" />
        <rect x={pStockPct} y="0" width={100 - pStockPct} height="6" fill="var(--ink)" />
      </svg>
      <div className="flex justify-between gap-[24px] text-[13px]">
        <div>
          <div className="flex items-center gap-[8px]">
            <span className="h-[8px] w-[8px] bg-[var(--merah)]" />
            <span className="eyebrow !text-[var(--body)]">pStocks · {pStockPct.toFixed(1)}%</span>
          </div>
          <div className="mono mt-[4px] text-[18px]">{fmtIDRX(stockValue)}</div>
        </div>
        <div className="text-right">
          <div className="flex items-center justify-end gap-[8px]">
            <span className="h-[8px] w-[8px] bg-[var(--ink)]" />
            <span className="eyebrow !text-[var(--body)]">IDRX · {(100 - pStockPct).toFixed(1)}%</span>
          </div>
          <div className="mono mt-[4px] text-[18px]">{fmtIDRX(stableValue)}</div>
        </div>
      </div>
    </div>
  );
}

function PositionsList({ positions, stables, idrxAddress, expanded, setExpanded, onTrade, onTransfer }: {
  positions: PortfolioPosition[];
  stables: StablePosition[];
  idrxAddress?: Address;
  expanded: string | null;
  setExpanded: (t: string | null) => void;
  onTrade: (t: PortfolioPosition) => void;
  onTransfer: (t: TransferToken) => void;
}) {
  return (
    <div>
      <div className="hairline table-head-desktop grid grid-cols-[auto_2fr_1fr_1fr_1fr_1fr_1fr_156px] gap-[16px] py-[14px]">
        {["", "Stock", "Lot", "Avg buy", "IDX lot", "Market value", "Unrealized P&L", ""].map((h, i) => (
          <div key={i} className={`eyebrow !text-[var(--body)] ${i >= 2 && i <= 6 ? "text-right" : "text-left"}`}>{h}</div>
        ))}
      </div>
      {positions.map(p => {
        const pos    = p.pnl >= 0;
        const isOpen = expanded === p.ticker;
        return (
          <div key={p.ticker} className="hairline">
            <div className="row-hover position-row cursor-pointer" onClick={() => setExpanded(isOpen ? null : p.ticker)}>
              <div className="pos-head">
                <PStockMark ticker={p.ticker} size={40} />
                <div className="col-asset min-w-0">
                  <div className="flex flex-wrap items-center gap-[8px]">
                    <span className="text-[15px] font-semibold">{p.ticker}</span>
                    {p.ipo && <span className="border border-[var(--hairline-strong)] px-[6px] py-[1px] text-[11px] text-[var(--body)]">{p.ipo}</span>}
                    <span className={`mono text-[11px] font-semibold ${(p.change24h ?? 0) >= 0 ? "text-[var(--positive)]" : "text-[var(--negative)]"}`}>
                      {fmtPct(p.change24h ?? 0)} today
                    </span>
                  </div>
                  <div className="mt-[2px] text-[12px] text-[var(--body)]">{p.name}</div>
                </div>
              </div>
              <div className="pos-data">
                <RowCell label="Lot" align="right"><span className="mono text-[14px]">{fmtNum(p.qty, 2)}</span></RowCell>
                <RowCell label="Avg buy" align="right"><span className="mono text-[14px]">{fmtIDRX(p.avg)}</span></RowCell>
                <RowCell label="Last" align="right"><span className="mono text-[14px]">{fmtIDRX(p.price)}</span></RowCell>
                <RowCell label="Market value" align="right"><span className="mono text-[15px] font-medium">{fmtIDRX(p.value)}</span></RowCell>
                <RowCell label="Unrealized P&L" align="right">
                  <div className={`mono text-[14px] font-medium ${pos ? "text-[var(--positive)]" : "text-[var(--negative)]"}`}>
                    {pos ? "+" : "−"}{fmtIDRX(Math.abs(p.pnl))}
                  </div>
                  <div className={`mono text-[11px] ${pos ? "text-[var(--positive)]" : "text-[var(--negative)]"}`}>{fmtPct(p.pnlPct)}</div>
                </RowCell>
              </div>
              <div className="pos-actions">
                <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]" onClick={e => { e.stopPropagation(); onTrade(p); }}>Trade</button>
                <button
                  className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]"
                  disabled={!p.contractAddress}
                  onClick={e => {
                    e.stopPropagation();
                    if (p.contractAddress) onTransfer({ ticker: p.ticker, name: p.name, price: p.price, address: p.contractAddress, isStable: false });
                  }}
                >
                  Send
                </button>
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
            <div className="col-asset min-w-0">
              <div className="flex items-center gap-[8px]">
                <span className="text-[15px] font-semibold">{s.ticker}</span>
                <span className="border border-[var(--hairline-strong)] px-[6px] py-[1px] text-[11px] text-[var(--body)]">STABLE</span>
              </div>
              <div className="mt-[2px] text-[12px] text-[var(--body)]">{s.name}</div>
            </div>
          </div>
          <div className="pos-data">
            <RowCell label="Lot" align="right"><span className="mono text-[14px]">{fmtNum(s.qty, 2)}</span></RowCell>
            <RowCell label="Avg buy" align="right"><span className="mono text-[14px]">1 IDRX</span></RowCell>
            <RowCell label="Last" align="right"><span className="mono text-[14px]">1 IDRX</span></RowCell>
            <RowCell label="Market value" align="right"><span className="mono text-[15px]">{fmtIDRX(s.value)}</span></RowCell>
            <RowCell label="Unrealized P&L" align="right"><span className="mono text-[13px] text-[var(--body)]">—</span></RowCell>
          </div>
          <div className="pos-actions">
            <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]" disabled>Trade</button>
            <button
              className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px] !text-[13px]"
              disabled={!idrxAddress}
              onClick={() => {
                if (idrxAddress) onTransfer({ ticker: s.ticker, name: s.name, price: 1, address: idrxAddress, isStable: true });
              }}
            >
              Send
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}

function RowCell({ label, align = "left", children }: { label: string; align?: "left" | "right"; children: React.ReactNode }) {
  return (
    <div className={`row-cell min-w-0 ${align === "right" ? "text-right" : "text-left"}`}>
      <span className="row-cell-label">{label}</span>
      <div className="row-cell-value">{children}</div>
    </div>
  );
}

function PositionDetail({ position }: { position: PortfolioPosition }) {
  const [range, setRange] = useState("1M");
  const series = useMemo(() => {
    return [
      { timestamp: POSITION_CHART_ANCHOR - 60 * 60_000, value: position.price },
      { timestamp: POSITION_CHART_ANCHOR, value: position.price },
    ];
  }, [position.price]);
  const ranged = useMemo(() => sliceRange(series, range), [series, range]);

  return (
    <div className="mt-[-1px] border-t border-[var(--hairline)] bg-[var(--canvas-soft)] p-[20px]">
      <div className="grid-2col-form grid grid-cols-[minmax(0,2fr)_minmax(0,1fr)] items-start gap-[24px]">
        <div>
          <div className="mb-[14px] flex flex-wrap items-center justify-between gap-[12px]">
            <div>
              <div className="eyebrow !text-[var(--body)]">{position.ticker} · {position.name}</div>
              <div className="display mt-[2px] !text-[24px]">{fmtIDRX(position.price)}</div>
            </div>
            <div className="range-pills">
              {["1D","1W","1M","3M","1Y"].map(r => (
                <button key={r} className={range === r ? "active" : ""} onClick={() => setRange(r)}>{r}</button>
              ))}
            </div>
          </div>
          <AreaChart data={ranged} height={200} valueFormatter={v => fmtIDRX(v)} />
        </div>
        <div className="flex flex-col gap-[12px] pt-[4px]">
          <KV k="Lot owned"     v={`${fmtNum(position.qty, 2)}`} />
          <KV k="Avg buy price" v={fmtIDRX(position.avg)} />
          <KV k="IDX lot price" v={fmtIDRX(position.price)} />
          <KV k="Pool price" v={fmtIDRX(position.poolPrice)} />
          <KV k="Total cost"    v={fmtIDRX(position.cost)} />
          <KV k="Market value"  v={fmtIDRX(position.value)} highlight />
          <KV k="Unrealized P&L" v={
            <span className={`font-semibold ${position.pnl >= 0 ? "text-[var(--positive)]" : "text-[var(--negative)]"}`}>
              {position.pnl >= 0 ? "+" : "−"}{fmtIDRX(Math.abs(position.pnl))} ({fmtPct(position.pnlPct)})
            </span>
          } />
          {position.sector && <KV k="Sector" v={position.sector} />}
          {position.ipo    && <KV k="IDX ticker" v={position.ipo} />}
        </div>
      </div>
    </div>
  );
}

function KV({ k, v, highlight }: { k: string; v: React.ReactNode; highlight?: boolean }) {
  return (
    <div className="hairline flex items-baseline justify-between gap-[16px] pb-[8px]">
      <span className="text-[12px] text-[var(--body)]">{k}</span>
      <span className={`mono text-right ${highlight ? "text-[15px] font-semibold" : "text-[13px] font-normal"}`}>{v}</span>
    </div>
  );
}

function ActivityList({ rows }: { rows: ActivityRow[] }) {
  if (rows.length === 0) {
    return (
      <div className="hairline py-[18px] text-[var(--body)]">
        No swap activity recorded for this wallet yet.
      </div>
    );
  }

  return (
    <div>
      {rows.map((row) => (
        <div key={row.txHash} className="hairline row-hover activity-row grid grid-cols-[auto_1fr_auto_auto] items-center gap-[20px] py-[16px]">
          <div className="flex h-[36px] w-[36px] shrink-0 items-center justify-center border border-[var(--ink)] bg-[var(--canvas)]">
            <Icon name="swap" size={14} />
          </div>
          <div className="min-w-0">
            <div className="text-[14px] font-semibold">{row.text} <span className="font-normal text-[var(--body)]">· {row.a}{row.b && " → "}{row.b}</span></div>
            <div className="mt-[3px] text-[12px] text-[var(--body)]">
              <span className="mono">{row.hash}</span> · {row.when}
            </div>
          </div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--positive)]">{row.status}</div>
          <a className="btn-ghost btn only-desktop !inline-flex !items-center !gap-[6px] !p-[4px]" href={`https://sepolia.arbiscan.io/tx/${row.txHash}`} target="_blank" rel="noreferrer">
            <Icon name="external" size={13} />
          </a>
        </div>
      ))}
    </div>
  );
}

function TransferModal({ token, balance, onClose, onSubmit, busy }: {
  token: TransferToken;
  balance: number;
  onClose: () => void;
  onSubmit: (opts: { token: TransferToken; to: Address; amount: string }) => void;
  busy: boolean;
}) {
  const [to, setTo]   = useState("");
  const [amt, setAmt] = useState("");
  const num = parseFloat(amt) || 0;
  const ok  = /^0x[a-fA-F0-9]{40}$/.test(to) && num > 0 && num <= balance && !busy;

  return (
    <div className="overlay fixed inset-[0] z-[200] flex items-center justify-center bg-[rgba(22,17,14,0.45)] p-[16px]" onClick={onClose}>
      <div className="modal w-[440px] max-w-full border border-[var(--ink)] bg-[var(--putih)] shadow-[8px_8px_0_0_rgba(22,17,14,0.10)]" onClick={e => e.stopPropagation()}>
        <div className="hairline flex items-center justify-between px-[20px] py-[16px]">
          <div className="display !text-[22px]">Send {token.ticker}</div>
          <button className="btn-ghost btn !p-[4px]" onClick={onClose}><Icon name="x" /></button>
        </div>
        <div className="flex flex-col gap-[14px] p-[20px]">
          <div>
            <div className="eyebrow mb-[6px] !text-[var(--body)]">Recipient address</div>
            <input className="input mono" placeholder="0x… or .arb name" value={to} onChange={e => setTo(e.target.value)} />
          </div>
          <div>
            <div className="mb-[6px] flex justify-between">
              <div className="eyebrow !text-[var(--body)]">Amount</div>
              <span className="mono text-[12px] text-[var(--body)]">Balance {fmtNum(balance, 4)}</span>
            </div>
            <div className="relative">
              <input className="input mono !pr-[60px]" placeholder="0.00" value={amt} onChange={e => setAmt(e.target.value.replace(/[^0-9.]/g, ""))} />
              <button onClick={() => setAmt(balance.toString())} className="absolute right-[6px] top-1/2 -translate-y-1/2 cursor-pointer appearance-none border border-[var(--hairline-strong)] bg-[var(--canvas)] px-[8px] py-[4px] text-[11px] font-semibold [font-family:var(--font-inter,_Inter,_sans-serif)]">MAX</button>
            </div>
            {num > balance && <div className="mt-[6px] text-[12px] text-[var(--negative)]">Exceeds available balance</div>}
          </div>
          <div className="hairline-top flex flex-col gap-[6px] pt-[14px] text-[13px]">
            <DetailRow k="Network"     v="Arbitrum Sepolia" />
            <DetailRow k="Network fee" v="~$0.12" />
          </div>
          <button className="btn btn-primary !w-full !p-[14px]" disabled={!ok} onClick={() => onSubmit({ token, to: to as Address, amount: amt })}>
            {busy ? "Sending..." : `Send ${token.ticker}`}
          </button>
        </div>
      </div>
    </div>
  );
}
