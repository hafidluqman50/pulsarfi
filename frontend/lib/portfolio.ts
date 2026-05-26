import { formatUnits, type Address } from 'viem';
import type { MarketStock } from '@/http/market/priceApi';
import type { StockTransaction } from '@/http/market/transactionApi';
import type { Balances, TimePoint } from '@/lib/data';

export const PORTFOLIO_LOT_SIZE = 100;
const PORTFOLIO_CHART_ANCHOR = new Date('2026-05-26T14:00:00+08:00').getTime();

export type PortfolioPosition = {
  ticker: string;
  name: string;
  sector: string;
  ipo: string;
  contractAddress?: Address;
  qty: number;
  avg: number;
  price: number;
  poolPrice: number;
  value: number;
  cost: number;
  pnl: number;
  pnlPct: number;
  dayPnl: number;
  change24h: number;
};

export type StablePosition = {
  ticker: string;
  name: string;
  qty: number;
  value: number;
};

export type ActivityRow = {
  kind: 'swap';
  when: string;
  text: string;
  a: string;
  b: string;
  status: 'Confirmed';
  hash: string;
  txHash: string;
};

type CostLot = {
  qty: number;
  cost: number;
};

function rawAmount(raw: string, decimals: number): number {
  const value = BigInt(raw.replace(/[^0-9]/g, '') || '0');
  return Number(formatUnits(value, decimals));
}

export function stockLots(raw: string): number {
  return rawAmount(raw, 18);
}

export function idrxAmount(raw: string): number {
  return rawAmount(raw, 2);
}

export function buildCostBasis(transactions: StockTransaction[]): Record<string, number> {
  const lots: Record<string, CostLot> = {};
  const sorted = [...transactions].sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));

  for (const tx of sorted) {
    const ticker = tx.ticker;
    const stockQty = stockLots(tx.stock_amount);
    const idrx = idrxAmount(tx.idrx_amount);
    const current = lots[ticker] ?? { qty: 0, cost: 0 };

    if (tx.side === 'buy') {
      current.qty += stockQty;
      current.cost += idrx;
    } else if (current.qty > 0) {
      const avg = current.cost / current.qty;
      const removedQty = Math.min(stockQty, current.qty);
      current.qty -= removedQty;
      current.cost = Math.max(0, current.cost - avg * removedQty);
    }

    lots[ticker] = current;
  }

  return Object.fromEntries(
    Object.entries(lots)
      .filter(([, lot]) => lot.qty > 0)
      .map(([ticker, lot]) => [ticker, lot.cost / lot.qty]),
  );
}

export function buildPositions(
  balances: Balances,
  marketStocks: MarketStock[],
  transactions: StockTransaction[],
): PortfolioPosition[] {
  const costBasis = buildCostBasis(transactions);

  return marketStocks
    .map((stock) => {
      const qty = balances[stock.ticker] ?? 0;
      const last = stock.price * PORTFOLIO_LOT_SIZE;
      const avg = costBasis[stock.ticker] ?? last;
      const value = qty * last;
      const cost = qty * avg;
      const previousLast = stock.change_24h === -100
        ? last
        : (stock.price / (1 + stock.change_24h / 100)) * PORTFOLIO_LOT_SIZE;
      const dayPnl = qty * (last - previousLast);

      return {
        ticker: stock.ticker,
        name: stock.stock_name,
        sector: stock.sector ?? 'Other',
        ipo: stock.idx_ticker,
        contractAddress: stock.contract_address as Address | undefined,
        qty,
        avg,
        price: last,
        poolPrice: stock.pool_price,
        value,
        cost,
        pnl: value - cost,
        pnlPct: avg ? ((last - avg) / avg) * 100 : 0,
        dayPnl,
        change24h: stock.change_24h,
      };
    })
    .filter(position => position.qty > 0)
    .sort((a, b) => b.value - a.value);
}

export function buildStables(balances: Balances): StablePosition[] {
  const qty = balances.IDRX ?? 0;
  return qty > 0 ? [{ ticker: 'IDRX', name: 'Indonesian Rupiah X', qty, value: qty }] : [];
}

export function buildPortfolioSeries(currentValue: number, transactions: StockTransaction[]): TimePoint[] {
  const now = PORTFOLIO_CHART_ANCHOR;
  if (currentValue <= 0) {
    return [
      { timestamp: now - 60_000, value: 0 },
      { timestamp: now, value: 0 },
    ];
  }

  const firstTxTime = transactions.length > 0
    ? Math.min(...transactions.map(tx => Date.parse(tx.created_at)).filter(Number.isFinite))
    : now - 60 * 60_000;
  const start = Number.isFinite(firstTxTime) ? firstTxTime : now - 60 * 60_000;

  return [
    { timestamp: Math.min(start, now - 60_000), value: currentValue },
    { timestamp: now, value: currentValue },
  ];
}

export function buildActivityRows(transactions: StockTransaction[]): ActivityRow[] {
  return transactions.map((tx) => {
    const stockQty = stockLots(tx.stock_amount);
    const idrx = idrxAmount(tx.idrx_amount);
    const isBuy = tx.side === 'buy';
    return {
      kind: 'swap',
      when: relativeTime(tx.created_at),
      text: isBuy ? 'Buy' : 'Sell',
      a: isBuy ? `${formatAmount(idrx)} IDRX` : `${formatAmount(stockQty)} ${tx.ticker}`,
      b: isBuy ? `${formatAmount(stockQty)} ${tx.ticker}` : `${formatAmount(idrx)} IDRX`,
      status: 'Confirmed',
      hash: `${tx.tx_hash.slice(0, 6)}...${tx.tx_hash.slice(-4)}`,
      txHash: tx.tx_hash,
    };
  });
}

export function formatAmount(value: number): string {
  return value.toLocaleString('id-ID', { maximumFractionDigits: 4 });
}

function relativeTime(value: string): string {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return 'recently';

  const diffSeconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (diffSeconds < 60) return `${diffSeconds}s ago`;
  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) return `${diffMinutes}m ago`;
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  return `${Math.floor(diffHours / 24)}d ago`;
}
