'use client';

import { useState } from 'react';
import type { AgentTask } from '@/http/agent/taskApi';
import { useTaskTrades, useDisarmTask, usePauseTask, useResumeTask } from '@/http/agent/hooks';
import { type CardContract, parseCardContract } from './cardContract';

type TradeLedgerProps = {
  task: AgentTask;
  contract?: CardContract;
};

function formatTriggerDisplay(task: AgentTask): string {
  if (task.summary && task.summary.trim() && !task.summary.startsWith('{')) {
    return task.summary;
  }
  if (task.trigger_description) {
    try {
      const parsed = JSON.parse(task.trigger_description);
      const rawSide = String(parsed.side || '').toUpperCase();
      const ticker = parsed.ticker ? String(parsed.ticker).toUpperCase() : '';
      const rawBudget = parsed.budget_idrx || parsed.budget;
      const budget = rawBudget && Number(rawBudget) > 0 ? Number(rawBudget).toLocaleString() : '';

      if (rawSide || ticker || budget) {
        const parts: string[] = [];
        if (rawSide && ticker) parts.push(`${rawSide} ${ticker}`);
        else if (ticker) parts.push(ticker);
        if (budget) parts.push(`(${budget} IDRX)`);
        return parts.join(' ');
      }
    } catch {
      if (!task.trigger_description.startsWith('{')) {
        return task.trigger_description;
      }
    }
  }
  return '';
}

export function TradeLedger({ task, contract: passedContract }: TradeLedgerProps) {
  const { data: trades = [] } = useTaskTrades(task.id);
  const [open, setOpen] = useState(false);
  const disarmTask = useDisarmTask(task.id);
  const pauseTask = usePauseTask(task.id);
  const resumeTask = useResumeTask(task.id);

  const contract = passedContract ?? parseCardContract(task);
  const onChainTaskId = task.on_chain_task_id;
  const isCancelled = task.status === 'cancelled';
  const isExecuted = task.status === 'executed';

  return (
    <>
      {task.paused && !isCancelled && !isExecuted && (
        <div className="rise" style={{ border: '1px solid var(--warn)', background: 'var(--warn-soft)', padding: '13px 15px' }}>
          <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--warn-deep)', marginBottom: 8 }}>
            {contract.ledger.paused_title}
          </div>
          <div style={{ fontSize: 13.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
            {contract.ledger.paused_desc}
          </div>
          {task.paused_at && (
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', marginTop: 10 }}>
              {new Date(task.paused_at).toLocaleString()}
            </div>
          )}
        </div>
      )}

      {isCancelled ? (
        <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
          <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--ink)', marginBottom: 8 }}>
            {contract.ledger.disarmed_title.replace('{id}', String(task.id))}
          </div>
          <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
            {contract.ledger.disarmed_desc}
          </div>
        </div>
      ) : isExecuted ? (
        <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--positive)', background: 'var(--putih)', padding: '13px 15px' }}>
          <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--positive)', marginBottom: 8 }}>
            {contract.ledger.executed_title.replace('{id}', String(task.id)).replace('{onChainTaskId}', String(onChainTaskId))}
          </div>
          <div style={{ fontSize: 14.5, lineHeight: 1.55 }}>{formatTriggerDisplay(task) || contract.ledger.executed_desc}</div>
        </div>
      ) : (
        !task.paused && (
          <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--positive)', background: 'var(--putih)', padding: '13px 15px' }}>
            <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--positive)', marginBottom: 8 }}>
              {contract.ledger.armed_title.replace('{id}', String(task.id)).replace('{onChainTaskId}', String(onChainTaskId))}
            </div>
            <div style={{ fontSize: 14.5, lineHeight: 1.55 }}>{formatTriggerDisplay(task)}</div>
          </div>
        )
      )}

      <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
        <button
          onClick={() => setOpen((v) => !v)}
          style={{ width: '100%', appearance: 'none', border: 0, cursor: 'pointer', background: 'var(--canvas-soft)', padding: '11px 13px', textAlign: 'left', display: 'flex', alignItems: 'center', gap: 9 }}
        >
          <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
            {contract.ledger.trades_header.replace('{onChainTaskId}', String(onChainTaskId)).replace('{count}', String(trades.length))}
          </span>
          <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>
            {open ? contract.ledger.toggle_hide : contract.ledger.toggle_show}
          </span>
        </button>
        {open && (
          <div>
            {trades.map((trade, index) => (
              <div key={trade.id} style={{ borderTop: '1px solid var(--hairline)', padding: '11px 13px', display: 'flex', gap: 10, alignItems: 'baseline', flexWrap: 'wrap' }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, flex: 'none' }}>
                  #{index + 1}
                </span>
                <span style={{ flex: '1 1 130px', minWidth: 0, fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', lineHeight: 1.45 }}>
                  {trade.side} {trade.ticker} · {trade.amount}
                </span>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, flex: 'none' }}>
                  {trade.tx_hash ? (
                    <a
                      href={`https://sepolia.arbiscan.io/tx/${trade.tx_hash}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ color: 'var(--merah)', textDecoration: 'underline', textUnderlineOffset: '2px' }}
                    >
                      {`${trade.tx_hash.slice(0, 10)}…`}
                    </a>
                  ) : (
                    <span style={{ color: 'var(--ticker)' }}>—</span>
                  )}
                </span>
              </div>
            ))}
            {trades.length === 0 && (
              <div style={{ borderTop: '1px solid var(--hairline)', padding: '11px 13px', fontSize: 12, color: 'var(--body)' }}>
                {contract.ledger.no_trades_yet}
              </div>
            )}
            <div style={{ borderTop: '1px solid var(--hairline)', padding: '11px 13px', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.5 }}>
              {contract.ledger.multi_trade_notice.replace('{onChainTaskId}', String(onChainTaskId))}
            </div>
          </div>
        )}
      </div>

      {!isCancelled && !isExecuted && (
        <div className="rise" style={{ border: '1px solid var(--hairline-strong)', background: 'var(--canvas-soft)', padding: '12px 13px', display: 'flex', gap: 11, alignItems: 'center', flexWrap: 'wrap' }}>
          <span style={{ flex: '1 1 160px', minWidth: 0, fontSize: 12, color: 'var(--ink-soft)', lineHeight: 1.5 }}>
            {contract.ledger.disarm_notice.replace('{onChainTaskId}', String(onChainTaskId))}
          </span>
          <button
            onClick={() => (task.paused ? resumeTask.mutate() : pauseTask.mutate())}
            style={{ appearance: 'none', cursor: 'pointer', border: '1px solid var(--ink)', background: 'transparent', color: 'var(--ink)', font: '600 11.5px/1 var(--font-sans)', padding: '10px 12px', flex: 'none' }}
          >
            {task.paused ? contract.ledger.resume_button : contract.ledger.pause_button}
          </button>
          <button
            onClick={() => disarmTask.mutate()}
            style={{ appearance: 'none', cursor: 'pointer', border: '1px solid var(--merah)', background: 'transparent', color: 'var(--merah)', font: '600 11.5px/1 var(--font-sans)', padding: '10px 12px', flex: 'none' }}
          >
            {contract.ledger.disarm_button}
          </button>
        </div>
      )}

      {isExecuted && (
        <div className="rise" style={{ border: '1px solid var(--hairline)', background: 'var(--canvas-soft)', padding: '12px 13px', display: 'flex', alignItems: 'center' }}>
          <span style={{ fontSize: 12, color: 'var(--ink-soft)', lineHeight: 1.5 }}>
            {contract.ledger.executed_desc}
          </span>
        </div>
      )}
    </>
  );
}
