import { useTaskTrades } from '@/http/agent/hooks';

type TradeLedgerProps = {
  taskId: number;
  onChainTaskId: number;
};

// Title reads "trades", not "tasks" — one on-chain Task id, many Trades,
// each with its own id sequence (Trade 1, Trade 2, ...) distinct from the
// Task id.
export function TradeLedger({ taskId, onChainTaskId }: TradeLedgerProps) {
  const { data: trades = [] } = useTaskTrades(taskId);

  return (
    <div className="trade-ledger hairline p-[16px]">
      <p className="text-[13px]">
        On-chain Task #{onChainTaskId} · {trades.length} trades
      </p>
      {trades.map((trade, index) => (
        <div key={trade.id} className="hairline-top flex items-center justify-between py-[8px] text-[13px]">
          <span>Trade {index + 1}</span>
          <span>
            {trade.side} {trade.ticker} · {trade.amount}
          </span>
          <span className="mono text-[11px] text-[var(--body)]">{trade.tx_hash ? `${trade.tx_hash.slice(0, 10)}…` : '—'}</span>
        </div>
      ))}
      <p className="mt-[8px] text-[12px] text-[var(--body)]">
        One on-chain Task id, many Trades. #{onChainTaskId} stays the same for the life of the rule; each fill is its
        own TradeExecuted event with its own hash, so nothing overwrites and nothing replays.
      </p>
    </div>
  );
}
