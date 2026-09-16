'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { useSettleHorizonTask, useSendChatMessage } from '@/http/agent/hooks';

type HorizonNoticeCardProps = {
  uiProps: unknown;
  taskId?: number;
  chatId?: string;
  onSendPrompt?: (text: string) => void;
};

type HorizonCardLabels = {
  badge?: string;
  time_remaining_label?: string;
  headline?: string;
  custodial_notice?: string;
  close_button?: string;
  leave_button?: string;
  processing_button?: string;
  close_settled_text?: string;
  leave_settled_text?: string;
  close_toast_title?: string;
  close_toast_desc?: string;
  leave_toast_title?: string;
  leave_toast_desc?: string;
  close_prompt?: string;
};

type HorizonData = {
  task_id?: number;
  ticker?: string;
  side?: string;
  time_remaining?: string;
  expires_at?: string;
  card?: HorizonCardLabels;
};

export function HorizonNoticeCard({ uiProps, taskId: directTaskId, chatId, onSendPrompt }: HorizonNoticeCardProps) {
  const data = (uiProps as HorizonData) || {};
  const taskId = directTaskId ?? data.task_id;
  const ticker = data.ticker || 'IDX';
  const labels = data.card || {};
  const timeRemaining = labels.time_remaining_label || data.time_remaining || '';

  const [settledPolicy, setSettledPolicy] = useState<'leave_open' | 'close_position' | null>(null);
  const [isPending, setIsPending] = useState(false);

  const settleMutation = useSettleHorizonTask(taskId || 0);
  const sendMessage = useSendChatMessage(chatId);

  if (!taskId) return null;

  async function handleClosePosition() {
    if (isPending) return;
    setIsPending(true);
    try {
      await settleMutation.mutateAsync('close_position');
      setSettledPolicy('close_position');
      toast.success(labels.close_toast_title || 'Position exit initiated', {
        description: labels.close_toast_desc || `Initiating sell order for ${ticker} to IDRX...`,
      });

      const promptText = labels.close_prompt || `Sell all ${ticker} tokens to IDRX`;
      if (onSendPrompt) {
        onSendPrompt(promptText);
      } else if (chatId) {
        sendMessage.mutate(promptText);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Action failed';
      toast.error(msg);
    } finally {
      setIsPending(false);
    }
  }

  async function handleLeaveOpen() {
    if (isPending) return;
    setIsPending(true);
    try {
      await settleMutation.mutateAsync('leave_open');
      setSettledPolicy('leave_open');
      toast.success(labels.leave_toast_title || 'Position retained', {
        description: labels.leave_toast_desc || `Position ${ticker} remains safely stored in your wallet.`,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Action failed';
      toast.error(msg);
    } finally {
      setIsPending(false);
    }
  }

  return (
    <div className="rise" style={{ border: '1px solid var(--merah)', background: 'var(--putih)' }}>
      {/* Header */}
      <div
        style={{
          background: 'var(--ink)',
          color: 'var(--canvas)',
          padding: '10px 13px',
          display: 'flex',
          alignItems: 'center',
          gap: 9,
        }}
      >
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
          {labels.badge || 'SWING HORIZON ALERT · H-1'}
        </span>
        <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--merah)' }}>
          {ticker}
        </span>
      </div>

      {/* Body */}
      <div style={{ padding: '13px 15px', borderBottom: '1px solid var(--hairline)' }}>
        <p style={{ fontSize: 13, lineHeight: 1.55, color: 'var(--ink)' }}>
          {labels.headline || (
            <>
              Swing position <strong>{ticker}</strong> is nearing horizon expiration (
              <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--merah)', fontWeight: 600 }}>{timeRemaining}</span> remaining).
            </>
          )}
        </p>
        <p style={{ fontSize: 12, lineHeight: 1.5, color: 'var(--body)', marginTop: 6 }}>
          {labels.custodial_notice ||
            'As a self-custodial protocol, PulsarFi will never force-sell or transfer your assets without explicit permission. Choose your action for this position:'}
        </p>
      </div>

      {/* Action / Result */}
      <div style={{ padding: '12px 14px' }}>
        {settledPolicy === 'leave_open' && (
          <div
            style={{
              padding: '8px 12px',
              background: 'var(--canvas-soft)',
              border: '1px solid var(--positive)',
              color: 'var(--positive)',
              fontSize: 12.5,
              fontWeight: 500,
            }}
          >
            {labels.leave_settled_text || `✓ Position ${ticker} retained in your portfolio. Horizon task completed.`}
          </div>
        )}

        {settledPolicy === 'close_position' && (
          <div
            style={{
              padding: '8px 12px',
              background: 'var(--canvas-soft)',
              border: '1px solid var(--merah)',
              color: 'var(--ink)',
              fontSize: 12.5,
              fontWeight: 500,
            }}
          >
            {labels.close_settled_text || `✓ Sell order requested. Review and confirm the PlanCard in chat to execute swap to IDRX.`}
          </div>
        )}

        {!settledPolicy && (
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
            <button
              type="button"
              disabled={isPending}
              onClick={handleClosePosition}
              style={{
                appearance: 'none',
                border: '1px solid var(--merah)',
                cursor: isPending ? 'not-allowed' : 'pointer',
                background: 'var(--merah)',
                color: 'var(--putih)',
                font: '600 12.5px/1 var(--font-sans)',
                padding: '10px 14px',
                flex: '1 1 180px',
                textAlign: 'center',
              }}
            >
              {isPending ? (labels.processing_button || 'Processing...') : (labels.close_button || 'Exit Position (Sell to IDRX)')}
            </button>

            <button
              type="button"
              disabled={isPending}
              onClick={handleLeaveOpen}
              style={{
                appearance: 'none',
                border: '1px solid var(--ink)',
                cursor: isPending ? 'not-allowed' : 'pointer',
                background: 'transparent',
                color: 'var(--ink)',
                font: '600 12.5px/1 var(--font-sans)',
                padding: '10px 14px',
                flex: '1 1 180px',
                textAlign: 'center',
              }}
            >
              {isPending ? (labels.processing_button || 'Processing...') : (labels.leave_button || 'Keep in Portfolio')}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
