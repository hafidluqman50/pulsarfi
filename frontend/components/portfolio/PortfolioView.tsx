'use client';

import { useState, useMemo } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { toast } from 'sonner';
import { type Address } from 'viem';
import { sliceRange, fmtIDRX, fmtNum, fmtPct, shortAddr } from '@/lib/data';
import { useMarketStocks, useStockHistory, useStockTransactions } from '@/http/market/hooks';
import { useWalletTokenBalanceState } from '@/http/market/tokenHooks';
import { useTransferToken } from '@/http/market/transferHooks';
import { useRequestRedeem } from '@/http/market/redeemHooks';
import { toMarketToken, unitsFromDecimalInput, unitsFromNumber } from '@/lib/swap';
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
import { ISO_PALETTES } from '@/components/ui/IsoBar';
import { AreaChart } from '@/components/charts/AreaChart';
import { AllocationBars } from '@/components/portfolio/AllocationBars';
import { SwapModal } from '@/components/ui/SwapModal';
import { TransferModal, type TransferToken } from '@/components/ui/TransferModal';
import { RedeemModal, type RedeemToken } from '@/components/ui/RedeemModal';

const POSITION_GRID = 'grid grid-cols-[2fr_1fr_1fr_1fr_1fr_1fr_240px] gap-[16px]';
const SECTION_LABEL = 'text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]';
const PAGE_WRAPPER = 'mx-auto w-full max-w-[1440px] px-[32px] pb-[64px] pt-[32px] max-[719px]:px-[16px] max-[719px]:pb-[48px] max-[719px]:pt-[24px]';

const ACTIVITY_COLORS: Record<string, string> = {
  Buy: 'var(--positive)',
  Sell: 'var(--negative)',
  Received: '#1f4d8a',
  Sent: '#1f4d8a',
};
const ACTIVITY_DEFAULT_COLOR = '#5a4a3a';

function PortfolioSkeleton(): React.ReactNode {
  return (
    <div className={PAGE_WRAPPER}>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))] items-center gap-[48px]">
        <div>
          <div className="h-[13px] w-[180px] bg-[var(--canvas-soft)]" />
          <div className="mt-[22px] h-[66px] w-[420px] max-w-full bg-[var(--canvas-soft)]" />
          <div className="mt-[20px] flex flex-wrap gap-[28px]">
            <div className="h-[40px] w-[150px] bg-[var(--canvas-soft)]" />
            <div className="h-[40px] w-[190px] bg-[var(--canvas-soft)]" />
            <div className="h-[40px] w-[150px] bg-[var(--canvas-soft)]" />
          </div>
        </div>
        <div className="h-[250px] w-full bg-[var(--canvas-soft)]" />
      </div>
      <div className="mt-[32px] border-t border-[var(--hairline)] pt-[24px]">
        <div className="mb-[18px] flex items-center justify-between">
          <div className="h-[42px] w-[180px] bg-[var(--canvas-soft)]" />
          <div className="h-[34px] w-[260px] bg-[var(--canvas-soft)]" />
        </div>
        <div className="paper-stack px-[16px] pb-[0] pt-[12px]">
          <div className="skeleton h-[260px] w-full" />
        </div>
      </div>
      <div className="mt-[44px]">
        <div className="h-[38px] w-[180px] bg-[var(--canvas-soft)]" />
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="border-b border-[var(--hairline)] py-[18px]">
            <div className="h-[34px] w-full bg-[var(--canvas-soft)]" />
          </div>
        ))}
      </div>
    </div>
  );
}

export function PortfolioView() {
  const { address, isConnected } = useAccount();
  const idrxAddress = process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined;
  const { data: marketStocks = [], isLoading: isMarketLoading } = useMarketStocks();
  const marketTokens = useMemo(() => marketStocks.map(toMarketToken), [marketStocks]);
  const { balances, isLoading: isBalancesLoading } = useWalletTokenBalanceState(marketTokens);
  const { data: transactions = [], isLoading: isTransactionsLoading } = useStockTransactions(address);
  const transferToken = useTransferToken();
  const requestRedeem = useRequestRedeem();

  const [range, setRange]         = useState("1D");
  const [expanded, setExpanded]   = useState<string | null>(null);
  const [transferOpen, setTransferOpen] = useState<TransferToken | null>(null);
  const [redeemOpen, setRedeemOpen]     = useState<RedeemToken | null>(null);
  const [tradeToken, setTradeToken]     = useState<PortfolioPosition | null>(null);

  const tradeOutput = tradeToken ? marketTokens.find(token => token.ticker === tradeToken.ticker) : undefined;
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
  const allocationItems = useMemo(() => [
    ...[...positions].sort((first, second) => second.value - first.value).map(position => ({ label: position.ticker, value: position.value })),
    ...(stables.length > 0 ? [{ label: 'IDRX', value: stableValue }] : []),
  ], [positions, stables, stableValue]);
  const accentByTicker = useMemo(
    () => Object.fromEntries(allocationItems.map((item, index) => [item.label, ISO_PALETTES[index % ISO_PALETTES.length].top])),
    [allocationItems],
  );
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

  if (isMarketLoading || isBalancesLoading || isTransactionsLoading) {
    return <PortfolioSkeleton />;
  }

  async function handleRedeem(opts: { token: typeof redeemOpen; amount: string }) {
    if (!opts.token || !address) return;
    const toastId = toast.loading('Submitting redeem request…');
    try {
      const tokenAmount = unitsFromDecimalInput(opts.amount, 18);
      const txHash = await requestRedeem.mutateAsync({
        ticker: opts.token.ticker,
        tokenAmount,
        walletAddress: address,
        stockContractAddress: opts.token.address,
      });
      toast.success('Redeem request submitted', {
        id: toastId,
        description: `Tx ${txHash.slice(0, 10)}...${txHash.slice(-6)}`,
        duration: 6000,
      });
      setRedeemOpen(null);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Redeem failed';
      toast.error('Redeem failed', { id: toastId, description: message.slice(0, 160), duration: 8000 });
    }
  }

  async function handleTransfer(opts: { token: TransferToken; to: Address; amount: string }) {
    const toastId = toast.loading("Broadcasting transfer...");
    try {
      const txHash = await transferToken.mutateAsync({
        token_address: opts.token.address,
        ticker: opts.token.ticker,
        from: address!,
        to: opts.to,
        amount: unitsFromDecimalInput(opts.amount, opts.token.isStable ? 2 : 18),
        estimated_idrx_amount: opts.token.isStable ? undefined : unitsFromNumber((parseFloat(opts.amount) || 0) * opts.token.price, 2),
        is_stable: Boolean(opts.token.isStable),
      });
      toast.success("Transfer sent", { id: toastId, description: `Tx ${txHash.slice(0, 10)}...${txHash.slice(-6)}`, duration: 4500 });
      setTransferOpen(null);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Transfer failed";
      const recordFailed = message.startsWith("Transfer confirmed");
      toast.error(recordFailed ? "Portfolio record failed" : "Transfer failed", { id: toastId, description: message.slice(0, 160), duration: 8000 });
    }
  }

  return (
    <div className={PAGE_WRAPPER}>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))] items-center gap-[48px]">
        <div>
          <div className="mb-[12px] text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--merah)]">Portfolio · {shortAddr(address!)}</div>
          <div className={`mb-[6px] ${SECTION_LABEL}`}>Total Net Worth</div>
          <div className="display tnum !text-[length:clamp(44px,5.4vw,72px)] !leading-[0.92] !tracking-[-0.035em]">{fmtIDRX(totalValue)}</div>
          <div className="mt-[18px] flex flex-wrap gap-[28px]">
            <PnLChip label="Today"               amount={dayPnl}      pct={dayPnlPct} />
            <PnLChip label="All-time unrealized" amount={allTimePnl}  pct={allTimePnlPct} />
            <PnLChip label="Cash · stables"      amount={stableValue} muted />
          </div>
        </div>
        <AllocationBars items={allocationItems} total={totalValue} />
      </div>

      <div className="mt-[32px] border-t border-[var(--hairline)] pt-[24px]">
        <div className="mb-[18px] flex flex-wrap items-center justify-between gap-[12px]">
          <div>
            <div className={SECTION_LABEL}>Portfolio value</div>
            <div className="display mt-[2px] !text-[20px] !leading-[normal]">{range === "1D" ? "Today" : range === "ALL" ? "All time" : `Last ${range}`}</div>
          </div>
          <div className="range-pills range-pills-full">
            {["1D","1W","1M","3M","1Y","ALL"].map(timeframeOption => (
              <button key={timeframeOption} className={range === timeframeOption ? "active" : ""} onClick={() => setRange(timeframeOption)}>{timeframeOption}</button>
            ))}
          </div>
        </div>
        <div className="paper-stack px-[16px] pb-[0] pt-[12px]">
          <AreaChart data={ranged} height={260} paper valueFormatter={value => `${(value / 1_000_000).toFixed(1)}M IDRX`} />
        </div>
      </div>

      <div className="mt-[44px]">
        <div className="flex flex-wrap items-baseline justify-between gap-[8px] border-b border-[var(--ink)] pb-[10px]">
          <h2 className="display m-[0] !text-[32px] !leading-[normal] !tracking-[-0.02em]">Positions</h2>
          <span className={SECTION_LABEL}>{positions.length} stocks · {stables.length} stable</span>
        </div>
        <PositionsList positions={positions} stables={stables} idrxAddress={idrxAddress} accentByTicker={accentByTicker} expanded={expanded} setExpanded={setExpanded} onTrade={t => setTradeToken(t)} onTransfer={t => setTransferOpen(t)} onRedeem={t => setRedeemOpen(t)} />
      </div>

      <div className="mt-[56px]">
        <div className="flex flex-wrap items-baseline justify-between border-b border-[var(--ink)] pb-[10px]">
          <h2 className="display m-[0] !text-[32px] !leading-[normal] !tracking-[-0.02em]">Recent activity</h2>
          <span className={SECTION_LABEL}>wallet swaps</span>
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

      {redeemOpen && (
        <RedeemModal
          token={redeemOpen}
          balance={balances[redeemOpen.ticker] ?? 0}
          onClose={() => setRedeemOpen(null)}
          onSubmit={(opts) => handleRedeem({ token: redeemOpen, amount: opts.amount })}
          busy={requestRedeem.isPending}
        />
      )}

      {tradeToken && tradeOutput && (
        <SwapModal
          defaultOut={tradeOutput}
          onClose={() => setTradeToken(null)}
        />
      )}
    </div>
  );
}

function PnLChip({ label, amount, pct, muted }: { label: string; amount: number; pct?: number; muted?: boolean }) {
  const isPositive = amount >= 0;
  const color = muted ? 'var(--body)' : isPositive ? 'var(--positive)' : 'var(--negative)';
  const sign = muted ? '' : isPositive ? '+' : '−';
  return (
    <div>
      <div className={SECTION_LABEL}>{label}</div>
      <div className="mono mt-[4px] text-[15px] font-[400] leading-[normal]" style={{ color }}>
        {sign}{fmtIDRX(Math.abs(amount))}{pct != null && <> · {fmtPct(pct)}</>}
      </div>
    </div>
  );
}

const ACTION_BASE = 'cursor-pointer appearance-none px-[12px] py-[7px] text-[12px] font-[600] disabled:cursor-not-allowed disabled:opacity-50';
const TRADE_BUTTON = `${ACTION_BASE} border border-[var(--ink)] bg-[var(--ink)] text-[var(--putih)] hover:bg-black`;
const SEND_BUTTON = `${ACTION_BASE} border border-[var(--hairline-strong)] bg-[var(--putih)] text-[var(--ink)] hover:border-[var(--ink)]`;
const REDEEM_BUTTON = `${ACTION_BASE} border border-[var(--ink)] bg-transparent text-[var(--ink)] hover:bg-[var(--ink)] hover:text-[var(--putih)]`;
const NUMBER_CELL = 'mono text-right text-[13px] leading-[normal]';

function PositionsList({ positions, stables, idrxAddress, accentByTicker, expanded, setExpanded, onTrade, onTransfer, onRedeem }: {
  positions: PortfolioPosition[];
  stables: StablePosition[];
  idrxAddress?: Address;
  accentByTicker: Record<string, string>;
  expanded: string | null;
  setExpanded: (t: string | null) => void;
  onTrade: (t: PortfolioPosition) => void;
  onTransfer: (t: TransferToken) => void;
  onRedeem: (t: RedeemToken) => void;
}) {
  return (
    <div className="overflow-x-auto">
      <div className="min-w-[860px]">
        <div className={`${POSITION_GRID} border-b border-[var(--hairline)] px-[8px] py-[12px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]`}>
          <span>Asset</span>
          <span className="text-right">Balance</span>
          <span className="text-right">Avg buy</span>
          <span className="text-right">Price</span>
          <span className="text-right">Value</span>
          <span className="text-right">Unrealized</span>
          <span />
        </div>
        {positions.map(p => {
          const isProfit = p.pnl >= 0;
          const isOpen = expanded === p.ticker;
          return (
            <div key={p.ticker}>
              <div
                className={`${POSITION_GRID} mover-paper cursor-pointer items-center border-b border-[var(--hairline)] px-[8px] py-[18px]`}
                onClick={() => setExpanded(isOpen ? null : p.ticker)}
              >
                <div className="flex items-center gap-[12px]">
                  <span className="h-[34px] w-[4px] shrink-0" style={{ background: accentByTicker[p.ticker] }} />
                  <PStockMark ticker={p.ticker} size={32} />
                  <div className="min-w-0">
                    <div className="text-[14px] font-[700] leading-[normal]">{p.ticker}</div>
                    <div className="text-[12px] leading-[normal] text-[var(--body)]">{p.name}</div>
                  </div>
                </div>
                <span className={NUMBER_CELL}>{fmtNum(p.qty, 2)}</span>
                <span className={`${NUMBER_CELL} text-[var(--body)]`}>{fmtIDRX(p.avg)}</span>
                <span className={NUMBER_CELL}>{fmtIDRX(p.price)}</span>
                <span className={`${NUMBER_CELL} font-[600]`}>{fmtIDRX(p.value)}</span>
                <span className={NUMBER_CELL} style={{ color: isProfit ? 'var(--positive)' : 'var(--negative)' }}>{fmtPct(p.pnlPct)}</span>
                <div className="flex justify-end gap-[8px]">
                  <button className={TRADE_BUTTON} onClick={e => { e.stopPropagation(); onTrade(p); }}>Trade</button>
                  <button
                    className={SEND_BUTTON}
                    disabled={!p.contractAddress}
                    onClick={e => {
                      e.stopPropagation();
                      if (p.contractAddress) onTransfer({ ticker: p.ticker, name: p.name, price: p.price, address: p.contractAddress, isStable: false });
                    }}
                  >
                    Send
                  </button>
                  <button
                    className={REDEEM_BUTTON}
                    disabled={!p.contractAddress}
                    onClick={e => {
                      e.stopPropagation();
                      if (p.contractAddress) onRedeem({ ticker: p.ticker, name: p.name, price: p.price, address: p.contractAddress });
                    }}
                  >
                    Redeem
                  </button>
                </div>
              </div>
              {isOpen && <PositionDetail position={p} />}
            </div>
          );
        })}

        {stables.map(s => (
          <div key={s.ticker} className={`${POSITION_GRID} mover-paper items-center border-b border-[var(--hairline)] px-[8px] py-[18px]`}>
            <div className="flex items-center gap-[12px]">
              <span className="h-[34px] w-[4px] shrink-0" style={{ background: accentByTicker[s.ticker] }} />
              <PStockMark ticker={s.ticker} size={32} />
              <div className="min-w-0">
                <div className="text-[14px] font-[700] leading-[normal]">{s.ticker}</div>
                <div className="text-[12px] leading-[normal] text-[var(--body)]">{s.name}</div>
              </div>
            </div>
            <span className={NUMBER_CELL}>{fmtNum(s.qty, 2)}</span>
            <span className={`${NUMBER_CELL} text-[var(--body)]`}>1 IDRX</span>
            <span className={NUMBER_CELL}>1 IDRX</span>
            <span className={`${NUMBER_CELL} font-[600]`}>{fmtIDRX(s.value)}</span>
            <span className={`${NUMBER_CELL} text-[var(--body)]`}>—</span>
            <div className="flex justify-end gap-[8px]">
              <button className={TRADE_BUTTON} disabled>Trade</button>
              <button
                className={REDEEM_BUTTON}
                disabled={!idrxAddress}
                onClick={() => {
                  if (idrxAddress) onTransfer({ ticker: s.ticker, name: s.name, price: 1, address: idrxAddress, isStable: true });
                }}
              >
                Transfer
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function PositionDetail({ position }: { position: PortfolioPosition }) {
  const [range, setRange] = useState("1M");
  const { data: series = [], isLoading } = useStockHistory(position.ipo, range, 'idx');

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
          {isLoading || series.length === 0 ? (
            <div className="skeleton h-[200px] w-full" />
          ) : (
            <AreaChart data={series} height={200} paper valueFormatter={v => fmtIDRX(v)} />
          )}
        </div>
        <div className="flex flex-col gap-[12px] pt-[4px]">
          <KV k="Holdings"      v={`${fmtNum(position.qty, 2)}`} />
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
      <div className="border-b border-[var(--hairline)] py-[18px] text-[var(--body)]">
        No swap activity recorded for this wallet yet.
      </div>
    );
  }

  return (
    <div>
      {rows.map((row) => (
        <div key={row.txHash} className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-[16px] border-b border-[var(--hairline)] px-[4px] py-[14px]">
          <span
            className="min-w-[64px] px-[8px] py-[4px] text-center text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--putih)]"
            style={{ background: ACTIVITY_COLORS[row.text] ?? ACTIVITY_DEFAULT_COLOR }}
          >
            {row.text}
          </span>
          <div className="min-w-0">
            <div className="text-[14px] font-[600] leading-[normal]">{row.a}{row.b && " → "}{row.b}</div>
            <div className="mono text-[11px] leading-[normal] text-[var(--body)]">{row.when} · {row.hash}</div>
          </div>
          <div className="flex items-center gap-[12px]">
            <span className="text-[11px] font-[600] uppercase tracking-[0.06em] text-[var(--positive)]">{row.status}</span>
            <a className="btn-ghost btn only-desktop !inline-flex !items-center !gap-[6px] !p-[4px]" href={`https://sepolia.arbiscan.io/tx/${row.txHash}`} target="_blank" rel="noreferrer">
              <Icon name="external" size={13} />
            </a>
          </div>
        </div>
      ))}
    </div>
  );
}
