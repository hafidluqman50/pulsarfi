'use client';

import { useState } from 'react';
import type { AgentTask } from '@/http/agent/taskApi';
import { useTaskTrades, useDisarmTask, usePauseTask, useResumeTask } from '@/http/agent/hooks';

type TradeLedgerProps = {
  task: AgentTask;
};

// Pixel-matched to Agent Chat.dc.html's post-arm states: the "Armed"
// banner, "On-chain Task #N" ledger, "Loop paused by you" banner,
// "Disarmed" banner, and the disarm control row. One on-chain Task id,
// many Trades — #N stays the same for the life of the rule; each fill is
// its own TradeExecuted event with its own hash, so nothing overwrites and
// nothing replays.
export function TradeLedger({ task }: TradeLedgerProps) {
  const { data: trades = [] } = useTaskTrades(task.id);
  const [open, setOpen] = useState(false);
  const disarmTask = useDisarmTask(task.id);
  const pauseTask = usePauseTask(task.id);
  const resumeTask = useResumeTask(task.id);

  const onChainTaskId = task.on_chain_task_id;
  const isCancelled = task.status === 'cancelled';

  return (
    <>
      {task.paused && !isCancelled && (
        <div className="rise" style={{ border: '1px solid var(--warn)', background: 'var(--warn-soft)', padding: '13px 15px' }}>
          <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--warn-deep)', marginBottom: 8 }}>
            Loop paused by you
          </div>
          <div style={{ fontSize: 13.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
            I have stopped evaluating. Sources keep arriving and are still logged, but nothing is scored and no proposal can reach you until you resume.
            This Task keeps its signature — this is a pause, not a disarm.
          </div>
          {task.paused_at && (
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', marginTop: 10 }}>paused at {new Date(task.paused_at).toLocaleString()}</div>
          )}
        </div>
      )}

      {isCancelled ? (
        <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
          <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--ink)', marginBottom: 8 }}>
            Disarmed · Task T-{task.id}
          </div>
          <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
            On-chain Task cancelled and the allowance is back to zero. My autonomy ended the moment you signed this — and nothing had to be wound down,
            because nothing was ever transferred beyond what already filled.
          </div>
        </div>
      ) : (
        !task.paused && (
          <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--positive)', background: 'var(--putih)', padding: '13px 15px' }}>
            <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--positive)', marginBottom: 8 }}>
              Armed · Task T-{task.id} · on-chain #{onChainTaskId}
            </div>
            <div style={{ fontSize: 14.5, lineHeight: 1.55 }}>{task.trigger_description ?? task.summary ?? 'Executing inside the caps you signed.'}</div>
          </div>
        )
      )}

      <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
        <button
          onClick={() => setOpen((v) => !v)}
          style={{ width: '100%', appearance: 'none', border: 0, cursor: 'pointer', background: 'var(--canvas-soft)', padding: '11px 13px', textAlign: 'left', display: 'flex', alignItems: 'center', gap: 9 }}
        >
          <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
            On-chain Task #{onChainTaskId} · {trades.length} trades
          </span>
          <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{open ? 'hide' : 'show'}</span>
        </button>
        {open && (
          <div>
            {trades.map((trade, index) => (
              <div key={trade.id} style={{ borderTop: '1px solid var(--hairline)', padding: '11px 13px', display: 'flex', gap: 10, alignItems: 'baseline', flexWrap: 'wrap' }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, flex: 'none' }}>Trade {index + 1}</span>
                <span style={{ flex: '1 1 130px', minWidth: 0, fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', lineHeight: 1.45 }}>
                  {trade.side} {trade.ticker} · {trade.amount}
                </span>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)', flex: 'none' }}>
                  {trade.tx_hash ? `${trade.tx_hash.slice(0, 10)}…` : '—'}
                </span>
              </div>
            ))}
            {trades.length === 0 && <div style={{ borderTop: '1px solid var(--hairline)', padding: '11px 13px', fontSize: 12, color: 'var(--body)' }}>No trades yet.</div>}
            <div style={{ borderTop: '1px solid var(--hairline)', padding: '11px 13px', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.5 }}>
              One on-chain Task id, many Trades. <span style={{ fontFamily: 'var(--font-mono)' }}>#{onChainTaskId}</span> stays the same for the life of the
              rule; each fill is its own <span style={{ fontFamily: 'var(--font-mono)' }}>TradeExecuted</span> event with its own hash, so nothing
              overwrites and nothing replays.
            </div>
          </div>
        )}
      </div>

      {!isCancelled && (
        <div className="rise" style={{ border: '1px solid var(--hairline-strong)', background: 'var(--canvas-soft)', padding: '12px 13px', display: 'flex', gap: 11, alignItems: 'center', flexWrap: 'wrap' }}>
          <span style={{ flex: '1 1 160px', minWidth: 0, fontSize: 12, color: 'var(--ink-soft)', lineHeight: 1.5 }}>
            Disarm cancels on-chain Task #{onChainTaskId} and sets the allowance to zero. It is the only switch you need, and it needs nobody&apos;s agreement.
          </span>
          <button
            onClick={() => (task.paused ? resumeTask.mutate() : pauseTask.mutate())}
            style={{ appearance: 'none', cursor: 'pointer', border: '1px solid var(--ink)', background: 'transparent', color: 'var(--ink)', font: '600 11.5px/1 var(--font-sans)', padding: '10px 12px', flex: 'none' }}
          >
            {task.paused ? 'Resume' : 'Pause'}
          </button>
          <button
            onClick={() => disarmTask.mutate()}
            style={{ appearance: 'none', cursor: 'pointer', border: '1px solid var(--merah)', background: 'transparent', color: 'var(--merah)', font: '600 11.5px/1 var(--font-sans)', padding: '10px 12px', flex: 'none' }}
          >
            Disarm
          </button>
        </div>
      )}
    </>
  );
}
